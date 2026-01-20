package assertions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/http"
	"github.com/abdul-hamid-achik/hitspec/packages/snapshot"
	"github.com/tidwall/gjson"
	"github.com/xeipuuv/gojsonschema"
)

type Result struct {
	Passed   bool
	Message  string
	Expected any
	Actual   any
	Subject  string
	Operator string
}

type Evaluator struct {
	response    *http.Response
	bodyJSON    gjson.Result
	baseDir     string // Base directory for resolving schema file paths
	testFile    string // Path to the test file (for snapshots)
	requestName string // Name of the current request (for snapshots)
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

func NewEvaluator(resp *http.Response) *Evaluator {
	return NewEvaluatorWithBaseDir(resp, "")
}

func NewEvaluatorWithBaseDir(resp *http.Response, baseDir string, opts ...EvaluatorOption) *Evaluator {
	e := &Evaluator{
		response: resp,
		baseDir:  baseDir,
	}
	if resp.IsJSON() {
		e.bodyJSON = gjson.ParseBytes(resp.Body)
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Evaluator) Evaluate(assertion *parser.Assertion) *Result {
	result := &Result{
		Subject:  assertion.Subject,
		Operator: assertion.Operator.String(),
		Expected: assertion.Expected,
	}

	actual, err := e.getActualValue(assertion.Subject)
	if err != nil {
		result.Passed = false
		result.Message = err.Error()
		return result
	}
	result.Actual = actual

	passed, msg := e.compare(actual, assertion.Operator, assertion.Expected)
	result.Passed = passed
	result.Message = msg

	// For length operator, show the computed length as the actual value
	if assertion.Operator == parser.OpLength {
		result.Actual = computeLength(actual)
	}

	return result
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
	case strings.HasPrefix(subject, "header"):
		headerName := strings.TrimPrefix(subject, "header")
		headerName = strings.TrimSpace(headerName)
		if headerName == "" {
			return e.response.Headers, nil
		}
		return e.response.Header(headerName), nil
	case strings.HasPrefix(subject, "body"):
		return e.getBodyValue(subject)
	case strings.HasPrefix(subject, "jsonpath"):
		path := strings.TrimPrefix(subject, "jsonpath")
		path = strings.TrimSpace(path)
		return e.getJSONPathValue(path)
	default:
		return e.getBodyValue("body." + subject)
	}
}

// convertBracketNotation converts array bracket notation to gjson dot notation
// e.g., "[0].id" -> "0.id", "items[0].tags[1]" -> "items.0.tags.1"
func convertBracketNotation(path string) string {
	// Replace [N] with .N
	result := regexp.MustCompile(`\[(\d+)\]`).ReplaceAllString(path, ".$1")
	// Remove leading dot if present (from converting [0] at start)
	result = strings.TrimPrefix(result, ".")
	return result
}

func (e *Evaluator) getBodyValue(subject string) (any, error) {
	if !e.bodyJSON.Exists() {
		return e.response.BodyString(), nil
	}

	path := strings.TrimPrefix(subject, "body")
	if path == "" {
		return e.bodyJSON.Value(), nil
	}
	path = strings.TrimPrefix(path, ".")

	// Convert bracket notation to gjson dot notation
	path = convertBracketNotation(path)

	result := e.bodyJSON.Get(path)
	if !result.Exists() {
		return nil, nil
	}
	return result.Value(), nil
}

func (e *Evaluator) getJSONPathValue(path string) (any, error) {
	if !e.bodyJSON.Exists() {
		return nil, fmt.Errorf("response body is not JSON")
	}
	// Convert bracket notation to gjson dot notation
	path = convertBracketNotation(path)
	result := e.bodyJSON.Get(path)
	if !result.Exists() {
		return nil, nil
	}
	return result.Value(), nil
}

func (e *Evaluator) compare(actual any, op parser.AssertionOperator, expected any) (bool, string) {
	switch op {
	case parser.OpEquals:
		return e.equals(actual, expected)
	case parser.OpNotEquals:
		passed, _ := e.equals(actual, expected)
		if passed {
			return false, fmt.Sprintf("expected not to equal %v", expected)
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
			return false, fmt.Sprintf("expected not to contain %v", expected)
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
	case parser.OpIncludes:
		return e.includes(actual, expected)
	case parser.OpNotIncludes:
		passed, _ := e.includes(actual, expected)
		if passed {
			return false, fmt.Sprintf("expected not to include %v", expected)
		}
		return true, ""
	case parser.OpIn:
		return e.in(actual, expected)
	case parser.OpNotIn:
		passed, _ := e.in(actual, expected)
		if passed {
			return false, fmt.Sprintf("expected not to be in %v", expected)
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

func (e *Evaluator) equals(actual, expected any) (bool, string) {
	if reflect.DeepEqual(actual, expected) {
		return true, ""
	}

	actualNum, aOk := toFloat64(actual)
	expectedNum, eOk := toFloat64(expected)
	if aOk && eOk && actualNum == expectedNum {
		return true, ""
	}

	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	if actualStr == expectedStr {
		return true, ""
	}

	return false, fmt.Sprintf("expected %v, got %v", expected, actual)
}

func (e *Evaluator) compareNumeric(actual, expected any, op string) (bool, string) {
	actualNum, aOk := toFloat64(actual)
	expectedNum, eOk := toFloat64(expected)

	if !aOk || !eOk {
		return false, fmt.Sprintf("cannot compare non-numeric values: %v %s %v", actual, op, expected)
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
	return false, fmt.Sprintf("expected %v %s %v", actual, op, expected)
}

func (e *Evaluator) contains(actual, expected any) (bool, string) {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	if strings.Contains(actualStr, expectedStr) {
		return true, ""
	}
	return false, fmt.Sprintf("expected '%v' to contain '%v'", actual, expected)
}

func (e *Evaluator) startsWith(actual, expected any) (bool, string) {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	if strings.HasPrefix(actualStr, expectedStr) {
		return true, ""
	}
	return false, fmt.Sprintf("expected '%v' to start with '%v'", actual, expected)
}

func (e *Evaluator) endsWith(actual, expected any) (bool, string) {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	if strings.HasSuffix(actualStr, expectedStr) {
		return true, ""
	}
	return false, fmt.Sprintf("expected '%v' to end with '%v'", actual, expected)
}

func (e *Evaluator) matches(actual, expected any) (bool, string) {
	actualStr := fmt.Sprintf("%v", actual)
	pattern := fmt.Sprintf("%v", expected)

	pattern = strings.TrimPrefix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Sprintf("invalid regex pattern: %v", err)
	}

	if re.MatchString(actualStr) {
		return true, ""
	}
	return false, fmt.Sprintf("expected '%v' to match /%v/", actual, pattern)
}

func (e *Evaluator) exists(actual any) (bool, string) {
	if actual == nil {
		return false, "expected to exist"
	}
	return true, ""
}

// computeLength returns the length of a value, or -1 if length cannot be computed
func computeLength(actual any) int {
	switch v := actual.(type) {
	case string:
		return len(v)
	case []any:
		return len(v)
	case map[string]any:
		return len(v)
	default:
		rv := reflect.ValueOf(actual)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
			return rv.Len()
		default:
			return -1
		}
	}
}

func (e *Evaluator) length(actual, expected any) (bool, string) {
	expectedLen, ok := toInt(expected)
	if !ok {
		return false, fmt.Sprintf("expected length must be a number, got %v", expected)
	}

	actualLen := computeLength(actual)
	if actualLen == -1 {
		return false, fmt.Sprintf("cannot get length of %T", actual)
	}

	if actualLen == expectedLen {
		return true, ""
	}
	return false, fmt.Sprintf("expected length %d, got %d", expectedLen, actualLen)
}

func (e *Evaluator) includes(actual, expected any) (bool, string) {
	arr, ok := actual.([]any)
	if !ok {
		return false, fmt.Sprintf("expected array, got %T", actual)
	}

	for _, item := range arr {
		if passed, _ := e.equals(item, expected); passed {
			return true, ""
		}
	}
	return false, fmt.Sprintf("expected array to include %v", expected)
}

func (e *Evaluator) in(actual, expected any) (bool, string) {
	arr, ok := expected.([]any)
	if !ok {
		return false, fmt.Sprintf("expected array for 'in' operator, got %T", expected)
	}

	for _, item := range arr {
		if passed, _ := e.equals(actual, item); passed {
			return true, ""
		}
	}
	return false, fmt.Sprintf("expected %v to be in %v", actual, expected)
}

func (e *Evaluator) typeCheck(actual, expected any) (bool, string) {
	expectedType := fmt.Sprintf("%v", expected)
	var actualType string

	switch actual.(type) {
	case nil:
		actualType = "null"
	case bool:
		actualType = "boolean"
	case float64, float32, int, int64, int32:
		actualType = "number"
	case string:
		actualType = "string"
	case []any:
		actualType = "array"
	case map[string]any:
		actualType = "object"
	default:
		actualType = reflect.TypeOf(actual).String()
	}

	if actualType == expectedType {
		return true, ""
	}
	return false, fmt.Sprintf("expected type %s, got %s", expectedType, actualType)
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case string:
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return f, true
		}
	}
	return 0, false
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

// validatePathWithinBase checks that the resolved path stays within the base directory
// to prevent path traversal attacks
func validatePathWithinBase(path, baseDir string) error {
	if baseDir == "" {
		return nil
	}

	// Clean and resolve both paths
	cleanBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("failed to resolve base directory: %v", err)
	}

	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %v", err)
	}

	// Ensure the path starts with the base directory
	if !strings.HasPrefix(cleanPath, cleanBase+string(filepath.Separator)) && cleanPath != cleanBase {
		return fmt.Errorf("path traversal detected: %s is outside allowed directory %s", path, baseDir)
	}

	return nil
}

func (e *Evaluator) schema(actual, expected any) (bool, string) {
	schemaPath := fmt.Sprintf("%v", expected)

	// Resolve schema path relative to base directory
	if !filepath.IsAbs(schemaPath) && e.baseDir != "" {
		schemaPath = filepath.Join(e.baseDir, schemaPath)
	}

	// Validate path doesn't escape base directory (prevent path traversal)
	if err := validatePathWithinBase(schemaPath, e.baseDir); err != nil {
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

func (e *Evaluator) each(actual, expected any) (bool, string) {
	arr, ok := actual.([]any)
	if !ok {
		return false, fmt.Sprintf("expected array for 'each' operator, got %T", actual)
	}

	if len(arr) == 0 {
		return true, "" // Empty array passes all conditions
	}

	// Expected should be an assertion-like structure or a simple value check
	// For simplicity, we check if each element equals the expected value
	// or if expected is a map with operator/value, we apply it to each element

	expectedMap, isMap := expected.(map[string]any)
	if isMap {
		// Check if it has operator and value fields
		op, hasOp := expectedMap["operator"]
		val, hasVal := expectedMap["value"]

		if hasOp && hasVal {
			opStr := fmt.Sprintf("%v", op)
			for i, item := range arr {
				passed, msg := e.applyOperator(item, opStr, val)
				if !passed {
					return false, fmt.Sprintf("item[%d]: %s", i, msg)
				}
			}
			return true, ""
		}
	}

	// Default: check each element equals expected
	for i, item := range arr {
		passed, msg := e.equals(item, expected)
		if !passed {
			return false, fmt.Sprintf("item[%d]: %s", i, msg)
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

func (e *Evaluator) applyOperator(actual any, op string, expected any) (bool, string) {
	switch op {
	case "==", "equals":
		return e.equals(actual, expected)
	case "!=", "notEquals":
		passed, _ := e.equals(actual, expected)
		if passed {
			return false, fmt.Sprintf("expected not to equal %v", expected)
		}
		return true, ""
	case ">":
		return e.compareNumeric(actual, expected, ">")
	case ">=":
		return e.compareNumeric(actual, expected, ">=")
	case "<":
		return e.compareNumeric(actual, expected, "<")
	case "<=":
		return e.compareNumeric(actual, expected, "<=")
	case "contains":
		return e.contains(actual, expected)
	case "startsWith":
		return e.startsWith(actual, expected)
	case "endsWith":
		return e.endsWith(actual, expected)
	case "matches":
		return e.matches(actual, expected)
	case "exists":
		return e.exists(actual)
	case "type":
		return e.typeCheck(actual, expected)
	default:
		return false, fmt.Sprintf("unknown operator in each: %s", op)
	}
}
