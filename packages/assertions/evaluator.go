package assertions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/conv"
	"github.com/abdul-hamid-achik/hitspec/v2/internal/pathutil"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/jsonbody"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/snapshot"
	"github.com/xeipuuv/gojsonschema"
)

type Result struct {
	Passed   bool
	Message  string
	Expected any
	Actual   any
	Subject  string
	Operator string
	Line     int
}

type Evaluator struct {
	response    *http.Response
	body        *jsonbody.Body
	wholeBody   bool                // the subject being evaluated is the whole body
	baseDir     string              // Base directory for resolving schema file paths
	testFile    string              // Path to the test file (for snapshots)
	requestName string              // Name of the current request (for snapshots)
	resolver    func(string) string // Optional resolver for variable interpolation
}

// EvaluatorOption is a functional option for configuring an Evaluator.
type EvaluatorOption func(*Evaluator)

// WithTestFile sets the test file path for snapshot testing.
func WithTestFile(path string) EvaluatorOption {
	return func(e *Evaluator) {
		e.testFile = path
	}
}

// WithRequestName sets the request name for snapshot testing.
func WithRequestName(name string) EvaluatorOption {
	return func(e *Evaluator) {
		e.requestName = name
	}
}

// WithResolver sets a resolver function for variable interpolation in expected values.
func WithResolver(resolver func(string) string) EvaluatorOption {
	return func(e *Evaluator) {
		e.resolver = resolver
	}
}

func NewEvaluator(resp *http.Response) *Evaluator {
	return NewEvaluatorWithBaseDir(resp, "")
}

func NewEvaluatorWithBaseDir(resp *http.Response, baseDir string, opts ...EvaluatorOption) *Evaluator {
	e := &Evaluator{
		response: resp,
		baseDir:  baseDir,
	}
	e.body = jsonbody.New(resp.ContentType(), resp.Body)
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// resolveExpected applies variable interpolation to expected values.
// It handles strings, arrays, and maps recursively.
func (e *Evaluator) resolveExpected(expected any) any {
	if e.resolver == nil {
		return expected
	}

	switch v := expected.(type) {
	case string:
		return e.resolver(v)
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = e.resolveExpected(item)
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			result[key] = e.resolveExpected(val)
		}
		return result
	case *parser.EachSpec:
		return &parser.EachSpec{Path: v.Path, Operator: v.Operator, Expected: e.resolveExpected(v.Expected)}
	default:
		return expected
	}
}

// absentValue marks a path that matches nothing, as opposed to a JSON null.
type absentValue struct{}

// absent is the value of a missing path. It is distinct from nil (JSON null):
// exists is false for both, but == null only matches a real null.
var absent any = absentValue{}

func isAbsent(v any) bool {
	_, ok := v.(absentValue)
	return ok
}

func (e *Evaluator) Evaluate(assertion *parser.Assertion) *Result {
	// Resolve any variables in the expected value
	resolvedExpected := e.resolveExpected(assertion.Expected)

	result := &Result{
		Subject:  assertion.Subject,
		Operator: assertion.Operator.String(),
		Expected: resolvedExpected,
		Line:     assertion.Line,
	}

	e.wholeBody = assertion.Subject == "body"
	actual, err := e.getActualValue(assertion.Subject)
	if err == nil && isExistenceOp(assertion.Operator) {
		// A missing header reads as "" which is non-nil, so the generic exists
		// check would treat every header as present. Resolve presence explicitly.
		if name, ok := headerName(assertion.Subject); ok {
			actual = e.headerValueOrNil(name)
		}
	}
	if err != nil {
		result.Passed = false
		result.Message = err.Error()
		return result
	}
	if !isAbsent(actual) {
		result.Actual = actual
	}

	passed, msg := e.compare(actual, assertion.Operator, resolvedExpected)
	if !passed && isAbsent(actual) && msg == "" {
		msg = fmt.Sprintf("%s does not exist", assertion.Subject)
	}
	result.Passed = passed
	result.Message = msg

	// For length operators, show the computed length as the actual value
	switch assertion.Operator {
	case parser.OpLength, parser.OpLengthGt, parser.OpLengthGte, parser.OpLengthLt, parser.OpLengthLte:
		if !isAbsent(actual) {
			result.Actual = computeLength(actual)
		}
	}

	return result
}

// subjectWord reports whether subject is the keyword alone or the keyword
// followed by the given separator (so a body field named "headers" or
// "bodyText" is not mistaken for the header or body subject).
func subjectIs(subject, keyword string, seps ...string) bool {
	if subject == keyword {
		return true
	}
	for _, sep := range seps {
		if strings.HasPrefix(subject, keyword+sep) {
			return true
		}
	}
	return false
}

func (e *Evaluator) getActualValue(subject string) (any, error) {
	switch {
	case subject == "status":
		return e.response.StatusCode, nil
	case subject == "duration":
		return e.response.DurationMs(), nil
	// Percentile assertions - for single requests, all percentiles equal duration
	// In stress testing mode, these would be calculated from aggregated metrics
	case subject == "p50", subject == "p95", subject == "p99":
		return e.response.DurationMs(), nil
	case subjectIs(subject, "header", " "):
		name, _ := headerName(subject)
		if name == "" {
			return e.response.Headers, nil
		}
		return e.response.Header(name), nil
	case subjectIs(subject, "body", ".", "["):
		return e.getBodyValue(subject)
	case subjectIs(subject, "jsonpath", " "):
		path := strings.TrimSpace(strings.TrimPrefix(subject, "jsonpath"))
		return e.getJSONPathValue(path)
	default:
		return e.getBodyValue("body." + subject)
	}
}

func isExistenceOp(op parser.AssertionOperator) bool {
	return op == parser.OpExists || op == parser.OpNotExists
}

// headerName extracts the header name from a "header <name>" subject. ok is
// false when the subject is not a named header subject.
func headerName(subject string) (string, bool) {
	if !subjectIs(subject, "header", " ") {
		return "", false
	}
	name := strings.TrimSpace(strings.TrimPrefix(subject, "header"))
	return name, name != ""
}

// headerValueOrNil returns the header value (case-insensitive name), or nil when
// the response has no such header. A present header with an empty value is
// returned as "" and therefore counts as existing.
func (e *Evaluator) headerValueOrNil(name string) any {
	for k, v := range e.response.Headers {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return nil
}

// getBodyValue reads "body" (the whole body) or "body.<path>". On a JSON
// response the path is a gjson path (bracket notation is converted); on any
// other response only the whole body can be read, a path is an error instead of
// quietly matching.
func (e *Evaluator) getBodyValue(subject string) (any, error) {
	path := strings.TrimPrefix(subject, "body")
	if path == "" {
		if e.body.Err() != nil {
			return e.response.BodyString(), nil
		}
		return e.body.Root(), nil
	}
	if err := e.body.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", subject, err)
	}
	gpath := parser.BodyPathToGJSON(strings.TrimPrefix(path, "."))
	return e.lookup(gpath), nil
}

func (e *Evaluator) lookup(gpath string) any {
	result := e.body.Get(gpath)
	if !result.Exists() {
		return absent
	}
	return result.Value()
}

func (e *Evaluator) getJSONPathValue(path string) (any, error) {
	if err := e.body.Err(); err != nil {
		return nil, fmt.Errorf("jsonpath %s: %w", path, err)
	}
	gpath, err := parser.JSONPathToGJSON(path)
	if err != nil {
		return nil, err
	}
	return e.lookup(gpath), nil
}

// compare applies one operator. A missing path (absent) fails every operator
// except exists / !exists and the negated ones (!=, !contains, !includes, !in),
// which hold trivially; the null comparisons are strict: == null needs a JSON
// null and != null needs a present non-null value, so a misspelt path cannot
// pass them. Use !exists to accept "absent or null".
func (e *Evaluator) compare(actual any, op parser.AssertionOperator, expected any) (bool, string) {
	if isAbsent(actual) {
		switch op {
		case parser.OpExists:
			return false, "expected to exist"
		case parser.OpNotExists:
			return true, ""
		case parser.OpNotEquals:
			if expected == nil {
				return false, "expected a value that is not null, but the path does not exist (use !exists to allow absent)"
			}
			return true, ""
		case parser.OpNotContains, parser.OpNotIncludes, parser.OpNotIn:
			return true, ""
		case parser.OpEquals:
			if expected == nil {
				return false, "expected null, but the path does not exist (null and absent are different; use !exists to allow both)"
			}
		}
		return false, "does not exist"
	}

	switch op {
	case parser.OpEquals:
		return e.equals(actual, expected)
	case parser.OpNotEquals:
		passed, _ := e.equals(actual, expected)
		if passed {
			return false, fmt.Sprintf("expected not to equal %v", display(expected))
		}
		return true, ""
	case parser.OpGreaterThan:
		return e.compareNumeric(actual, expected, ">")
	case parser.OpGreaterOrEqual:
		return e.compareNumeric(actual, expected, ">=")
	case parser.OpLessThan:
		return e.compareNumeric(actual, expected, "<")
	case parser.OpLessOrEqual:
		return e.compareNumeric(actual, expected, "<=")
	case parser.OpContains:
		return e.contains(actual, expected)
	case parser.OpNotContains:
		passed, _ := e.contains(actual, expected)
		if passed {
			return false, fmt.Sprintf("expected not to contain %v", display(expected))
		}
		return true, ""
	case parser.OpStartsWith:
		return e.startsWith(actual, expected)
	case parser.OpEndsWith:
		return e.endsWith(actual, expected)
	case parser.OpMatches:
		return e.matches(actual, expected)
	case parser.OpExists:
		return e.exists(actual)
	case parser.OpNotExists:
		passed, _ := e.exists(actual)
		if passed {
			return false, "expected not to exist"
		}
		return true, ""
	case parser.OpLength:
		return e.length(actual, expected)
	case parser.OpLengthGt:
		return e.lengthCompare(actual, expected, ">")
	case parser.OpLengthGte:
		return e.lengthCompare(actual, expected, ">=")
	case parser.OpLengthLt:
		return e.lengthCompare(actual, expected, "<")
	case parser.OpLengthLte:
		return e.lengthCompare(actual, expected, "<=")
	case parser.OpIncludes:
		return e.includes(actual, expected)
	case parser.OpNotIncludes:
		passed, _ := e.includes(actual, expected)
		if passed {
			return false, fmt.Sprintf("expected not to include %v", display(expected))
		}
		return true, ""
	case parser.OpIn:
		return e.in(actual, expected)
	case parser.OpNotIn:
		passed, _ := e.in(actual, expected)
		if passed {
			return false, fmt.Sprintf("expected not to be in %v", display(expected))
		}
		return true, ""
	case parser.OpType:
		return e.typeCheck(actual, expected)
	case parser.OpSchema:
		return e.schema(actual, expected)
	case parser.OpEach:
		return e.each(actual, expected)
	case parser.OpSnapshot:
		return e.snapshot(actual, expected)
	default:
		return false, fmt.Sprintf("unknown operator: %v", op)
	}
}

// display renders a value for a message: strings as they are, composites as JSON.
func display(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	case []any, map[string]any:
		return jsonText(t)
	}
	return fmt.Sprintf("%v", v)
}

// jsonText encodes a value as compact JSON without HTML escaping.
func jsonText(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Sprintf("%v", v)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// text is the string form string operators work on: strings as they are,
// numbers without exponent, objects and arrays as JSON text (never Go's
// map[...] formatting), null as "null". The whole JSON body is the raw text of
// the response.
func (e *Evaluator) text(v any) string {
	if e.wholeBody && e.body.Err() == nil {
		return string(e.body.Raw())
	}
	return valueText(v)
}

func valueText(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case json.Number:
		return t.String()
	case []any, map[string]any:
		return jsonText(t)
	}
	return fmt.Sprintf("%v", v)
}

func isComposite(v any) bool {
	switch v.(type) {
	case []any, map[string]any:
		return true
	}
	return false
}

// jsonEqual compares decoded JSON values deeply. Numbers are compared as
// numbers (1 equals 1.0), objects by key, arrays by position.
func jsonEqual(a, b any) bool {
	switch x := a.(type) {
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !jsonEqual(x[i], y[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, xv := range x {
			yv, ok := y[k]
			if !ok || !jsonEqual(xv, yv) {
				return false
			}
		}
		return true
	case nil:
		return b == nil
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	}
	xn, xok := conv.ToFloat64(a)
	yn, yok := conv.ToFloat64(b)
	if xok && yok {
		if _, isStr := b.(string); !isStr {
			return xn == yn
		}
	}
	return false
}

func (e *Evaluator) equals(actual, expected any) (bool, string) {
	// Objects and arrays compare deeply as JSON; the loose string and number
	// coercion below is for scalars only (it would make ["a b"] equal ["a","b"]).
	if isComposite(actual) || isComposite(expected) {
		if jsonEqual(actual, expected) {
			return true, ""
		}
		return false, fmt.Sprintf("expected %s, got %s", display(expected), display(actual))
	}

	if reflect.DeepEqual(actual, expected) {
		return true, ""
	}

	actualNum, aOk := conv.ToFloat64(actual)
	expectedNum, eOk := conv.ToFloat64(expected)
	if aOk && eOk && actualNum == expectedNum {
		return true, ""
	}

	// null only equals null: "<nil>" must not match anything by accident.
	if actual == nil || expected == nil {
		return false, fmt.Sprintf("expected %v, got %v", display(expected), display(actual))
	}

	actualStr := valueText(actual)
	expectedStr := valueText(expected)
	if actualStr == expectedStr {
		return true, ""
	}

	return false, fmt.Sprintf("expected %v, got %v", display(expected), display(actual))
}

func (e *Evaluator) compareNumeric(actual, expected any, op string) (bool, string) {
	actualNum, aOk := conv.ToFloat64(actual)
	expectedNum, eOk := conv.ToFloat64(expected)

	if !aOk || !eOk {
		return false, fmt.Sprintf("cannot compare non-numeric values: %v %s %v", display(actual), op, display(expected))
	}

	var passed bool
	switch op {
	case ">":
		passed = actualNum > expectedNum
	case ">=":
		passed = actualNum >= expectedNum
	case "<":
		passed = actualNum < expectedNum
	case "<=":
		passed = actualNum <= expectedNum
	}

	if passed {
		return true, ""
	}
	return false, fmt.Sprintf("expected %v %s %v", display(actual), op, display(expected))
}

func (e *Evaluator) contains(actual, expected any) (bool, string) {
	actualStr := e.text(actual)
	expectedStr := valueText(expected)
	if strings.Contains(actualStr, expectedStr) {
		return true, ""
	}
	return false, fmt.Sprintf("expected %s to contain '%s'", quoteShort(actualStr), expectedStr)
}

func (e *Evaluator) startsWith(actual, expected any) (bool, string) {
	actualStr := e.text(actual)
	expectedStr := valueText(expected)
	if strings.HasPrefix(actualStr, expectedStr) {
		return true, ""
	}
	return false, fmt.Sprintf("expected %s to start with '%s'", quoteShort(actualStr), expectedStr)
}

func (e *Evaluator) endsWith(actual, expected any) (bool, string) {
	actualStr := e.text(actual)
	expectedStr := valueText(expected)
	if strings.HasSuffix(actualStr, expectedStr) {
		return true, ""
	}
	return false, fmt.Sprintf("expected %s to end with '%s'", quoteShort(actualStr), expectedStr)
}

// quoteShort quotes a value for a failure message, cutting very long ones.
func quoteShort(s string) string {
	const max = 200
	if utf8.RuneCountInString(s) > max {
		s = string([]rune(s)[:max]) + "..."
	}
	return "'" + s + "'"
}

func (e *Evaluator) matches(actual, expected any) (bool, string) {
	actualStr := e.text(actual)
	pattern := valueText(expected)
	// The parser turns an unquoted /re/flags literal into a Go pattern. A
	// quoted "/re/" or a {{variable}} holding /re/ reaches here with its
	// slashes; earlier releases stripped them, and suites rely on that.
	if len(pattern) > 2 && strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") {
		pattern = pattern[1 : len(pattern)-1]
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Sprintf("invalid regex pattern: %v", err)
	}

	if re.MatchString(actualStr) {
		return true, ""
	}
	return false, fmt.Sprintf("expected %s to match /%v/", quoteShort(actualStr), pattern)
}

// exists is true for a present, non-null value (absent never reaches here).
func (e *Evaluator) exists(actual any) (bool, string) {
	if actual == nil || isAbsent(actual) {
		return false, "expected to exist"
	}
	return true, ""
}

// computeLength returns the length of a value, or -1 if length cannot be
// computed. Strings count characters (runes), not bytes.
func computeLength(actual any) int {
	switch v := actual.(type) {
	case string:
		return utf8.RuneCountInString(v)
	case []any:
		return len(v)
	case map[string]any:
		return len(v)
	case nil:
		return -1
	default:
		rv := reflect.ValueOf(actual)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			return rv.Len()
		case reflect.String:
			return utf8.RuneCountInString(rv.String())
		default:
			return -1
		}
	}
}

func (e *Evaluator) length(actual, expected any) (bool, string) {
	expectedLen, ok := toInt(expected)
	if !ok {
		return false, fmt.Sprintf("expected length must be a number, got %v", display(expected))
	}

	actualLen := computeLength(actual)
	if actualLen == -1 {
		return false, fmt.Sprintf("cannot get length of %s", typeName(actual))
	}

	if actualLen == expectedLen {
		return true, ""
	}
	return false, fmt.Sprintf("expected length %d, got %d", expectedLen, actualLen)
}

func (e *Evaluator) lengthCompare(actual, expected any, op string) (bool, string) {
	expectedLen, ok := toInt(expected)
	if !ok {
		return false, fmt.Sprintf("expected length must be a number, got %v", display(expected))
	}

	actualLen := computeLength(actual)
	if actualLen == -1 {
		return false, fmt.Sprintf("cannot get length of %s", typeName(actual))
	}

	var passed bool
	switch op {
	case ">":
		passed = actualLen > expectedLen
	case ">=":
		passed = actualLen >= expectedLen
	case "<":
		passed = actualLen < expectedLen
	case "<=":
		passed = actualLen <= expectedLen
	}

	if passed {
		return true, ""
	}
	return false, fmt.Sprintf("expected length %s %d, got %d", op, expectedLen, actualLen)
}

func (e *Evaluator) includes(actual, expected any) (bool, string) {
	arr, ok := actual.([]any)
	if !ok {
		return false, fmt.Sprintf("expected array, got %s", typeName(actual))
	}

	for _, item := range arr {
		if passed, _ := e.equals(item, expected); passed {
			return true, ""
		}
	}
	return false, fmt.Sprintf("expected array to include %v", display(expected))
}

func (e *Evaluator) in(actual, expected any) (bool, string) {
	arr, ok := expected.([]any)
	if !ok {
		return false, fmt.Sprintf("expected a list for 'in' operator, got %s", typeName(expected))
	}

	for _, item := range arr {
		if passed, _ := e.equals(actual, item); passed {
			return true, ""
		}
	}
	return false, fmt.Sprintf("expected %v to be in %v", display(actual), display(expected))
}

// typeName is the JSON type name of a decoded value.
func typeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64, float32, int, int64, int32, json.Number:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return reflect.TypeOf(v).String()
	}
}

func (e *Evaluator) typeCheck(actual, expected any) (bool, string) {
	expectedType := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", expected)))
	actualType := typeName(actual)

	if actualType == expectedType {
		return true, ""
	}
	return false, fmt.Sprintf("expected type %s, got %s", expectedType, actualType)
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case int32:
		return int(n), true
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i, true
		}
	}
	return 0, false
}

func EvaluateAll(resp *http.Response, assertions []*parser.Assertion) []*Result {
	return EvaluateAllWithBaseDir(resp, assertions, "")
}

func EvaluateAllWithBaseDir(resp *http.Response, assertions []*parser.Assertion, baseDir string, opts ...EvaluatorOption) []*Result {
	evaluator := NewEvaluatorWithBaseDir(resp, baseDir, opts...)
	results := make([]*Result, len(assertions))
	for i, a := range assertions {
		results[i] = evaluator.Evaluate(a)
	}
	return results
}

func (e *Evaluator) schema(actual, expected any) (bool, string) {
	schemaPath := fmt.Sprintf("%v", expected)

	// Resolve schema path relative to base directory
	if !filepath.IsAbs(schemaPath) && e.baseDir != "" {
		schemaPath = filepath.Join(e.baseDir, schemaPath)
	}

	// Validate path doesn't escape base directory (prevent path traversal)
	if err := pathutil.ValidateWithinBase(schemaPath, e.baseDir); err != nil {
		return false, err.Error()
	}

	// Read schema file
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return false, fmt.Sprintf("failed to read schema file: %v", err)
	}

	// Convert actual value to JSON
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		return false, fmt.Sprintf("failed to marshal actual value: %v", err)
	}

	// Create schema and document loaders
	schemaLoader := gojsonschema.NewBytesLoader(schemaData)
	documentLoader := gojsonschema.NewBytesLoader(actualJSON)

	// Validate
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return false, fmt.Sprintf("schema validation error: %v", err)
	}

	if result.Valid() {
		return true, ""
	}

	// Collect validation errors
	var errors []string
	for _, desc := range result.Errors() {
		errors = append(errors, desc.String())
	}
	return false, fmt.Sprintf("schema validation failed: %s", strings.Join(errors, "; "))
}

// eachSpecFromValue builds the spec for an Expected that is not an EachSpec
// (assertions built in code): {"operator": ..., "value": ...} or a bare value
// every element must equal. It returns nil for an unknown operator name.
func eachSpecFromValue(expected any) *parser.EachSpec {
	if m, ok := expected.(map[string]any); ok {
		opName, hasOp := m["operator"]
		val, hasVal := m["value"]
		if hasOp && hasVal {
			name := fmt.Sprintf("%v", opName)
			switch name {
			case "equals":
				name = "=="
			case "notEquals":
				name = "!="
			}
			op, found := parser.LookupOperator(name)
			if !found || op == parser.OpEach {
				return nil
			}
			return &parser.EachSpec{Operator: op, Expected: val}
		}
	}
	return &parser.EachSpec{Operator: parser.OpEquals, Expected: expected}
}

// each applies the spec's operator to every element of the array (or to a path
// inside every element). An empty array passes: combine with length > 0 to
// require elements.
func (e *Evaluator) each(actual, expected any) (bool, string) {
	arr, ok := actual.([]any)
	if !ok {
		return false, fmt.Sprintf("expected array for 'each' operator, got %s", typeName(actual))
	}

	spec, ok := expected.(*parser.EachSpec)
	if !ok {
		spec = eachSpecFromValue(expected)
	}
	if spec == nil {
		return false, fmt.Sprintf("each: unknown operator in %v", display(expected))
	}

	// "body" inside the element refers to the element, never the response.
	saved := e.wholeBody
	e.wholeBody = false
	defer func() { e.wholeBody = saved }()

	for i, item := range arr {
		value, found := jsonbody.Lookup(item, spec.Path)
		if !found {
			value = absent
		}
		passed, msg := e.compare(value, spec.Operator, spec.Expected)
		if !passed {
			if msg == "" || msg == "does not exist" {
				msg = fmt.Sprintf("%s does not exist", spec.Path)
			}
			where := fmt.Sprintf("item[%d]", i)
			if spec.Path != "" && !strings.HasPrefix(msg, spec.Path) {
				where += "." + spec.Path
			}
			return false, fmt.Sprintf("%s: %s", where, msg)
		}
	}
	return true, ""
}

func (e *Evaluator) snapshot(actual, expected any) (bool, string) {
	// Get snapshot name from expected value (optional)
	snapshotName := ""
	if expected != nil {
		snapshotName = fmt.Sprintf("%v", expected)
	}

	// Use global snapshot manager
	manager := snapshot.GetGlobalManager()
	if manager == nil {
		return false, "snapshot manager not initialized"
	}

	result := manager.Compare(e.testFile, e.requestName, snapshotName, actual)
	if result.Passed {
		if result.IsNew {
			return true, "new snapshot created"
		}
		if result.WasUpdated {
			return true, "snapshot updated"
		}
		return true, ""
	}
	return false, result.Message
}
