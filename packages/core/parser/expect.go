package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file is the single grammar for an "expect <subject> <operator> <value>"
// line. It is shared by >>> assertion blocks, >>>db blocks and the nested
// assertion of "each".
//
// A line is read as raw text, never as lexer tokens: the generic tokenizer was
// built for annotations and silently dropped, re-spelled or swallowed parts of
// an assertion (a "#" in a path became a comment, "Jane Doe" lost "Doe", an
// operator typo became "== typo", an operator with no value swallowed the next
// line).
//
// Expected values follow ONE rule:
//
//   - a JSON literal: "string" (also 'string'), number, true/false/null,
//     [list] (elements follow the same rule), {object};
//   - for matches: a /regex/flags literal;
//   - otherwise the rest of the line, trimmed, verbatim, as a string
//     ("expect body.name == Jane Doe" compares with "Jane Doe").
//
// A trailing "# comment" (whitespace, '#', whitespace or end of line) is
// stripped from the value.

// symbolOperators and wordOperators are the one operator table. Lookups are
// case-insensitive.
var symbolOperators = map[string]AssertionOperator{
	"==": OpEquals,
	"!=": OpNotEquals,
	">":  OpGreaterThan,
	">=": OpGreaterOrEqual,
	"<":  OpLessThan,
	"<=": OpLessOrEqual,
}

var wordOperators = map[string]AssertionOperator{
	"contains":   OpContains,
	"!contains":  OpNotContains,
	"startswith": OpStartsWith,
	"endswith":   OpEndsWith,
	"matches":    OpMatches,
	"exists":     OpExists,
	"!exists":    OpNotExists,
	"length":     OpLength,
	"includes":   OpIncludes,
	"!includes":  OpNotIncludes,
	"in":         OpIn,
	"!in":        OpNotIn,
	"type":       OpType,
	"each":       OpEach,
	"schema":     OpSchema,
	"snapshot":   OpSnapshot,
}

// operatorAliases maps common wrong spellings to the operator they were meant to be.
var operatorAliases = map[string]string{
	"=": "==", "===": "==", "equals": "==", "equal": "==", "eq": "==", "is": "==",
	"!==": "!=", "<>": "!=", "ne": "!=", "neq": "!=", "notequals": "!=", "isnot": "!=",
	"=>": ">=", "=<": "<=",
	"gt": ">", "gte": ">=", "ge": ">=", "lt": "<", "lte": "<=", "le": "<=",
	"=~": "matches", "match": "matches", "regex": "matches", "like": "matches",
	"contain": "contains", "has": "includes", "include": "includes",
	"startwith": "startsWith", "endwith": "endsWith", "starts": "startsWith", "ends": "endsWith",
	"exist": "exists", "notexists": "!exists", "missing": "!exists",
	"notcontains": "!contains", "notin": "!in", "notincludes": "!includes",
	"len": "length", "size": "length", "count": "length",
}

// LookupOperator resolves an operator spelling ("==", "startsWith", "!in") to
// its AssertionOperator. It is case-insensitive. "length" resolves to OpLength;
// the compound forms ("length >") are only recognized inside an assertion line.
func LookupOperator(name string) (AssertionOperator, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if op, ok := symbolOperators[name]; ok {
		return op, true
	}
	op, ok := wordOperators[name]
	return op, ok
}

// operatorSpellings lists the operators in documentation order, for messages.
var operatorSpellings = []string{
	"==", "!=", ">", ">=", "<", "<=", "contains", "!contains", "startsWith", "endsWith",
	"matches", "exists", "!exists", "length", "includes", "!includes", "in", "!in",
	"type", "each", "schema", "snapshot",
}

// typeNames are the values the type operator accepts.
var typeNames = []string{"null", "boolean", "number", "string", "array", "object"}

// suggestOperator returns the closest known operator spelling for a wrong one,
// or "" when nothing is close.
func suggestOperator(word string) string {
	key := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(word))
	if alias, ok := operatorAliases[key]; ok {
		return alias
	}
	return closest(key, operatorSpellings, true)
}

// closest returns the candidate with the smallest edit distance to word (case
// folded), when that distance is small enough to be a plausible typo.
func closest(word string, candidates []string, foldCase bool) string {
	best, bestDist := "", 1<<30
	w := word
	if foldCase {
		w = strings.ToLower(w)
	}
	for _, c := range candidates {
		cc := c
		if foldCase {
			cc = strings.ToLower(c)
		}
		if d := editDistance(w, cc); d < bestDist {
			best, bestDist = c, d
		}
	}
	limit := 1
	if utf8.RuneCountInString(word) >= 5 {
		limit = 2
	}
	if bestDist <= limit {
		return best
	}
	return ""
}

// editDistance is the optimal string alignment distance: insertions, deletions,
// substitutions and swaps of adjacent characters each cost 1 (so a transposed
// "nmae" is one edit from "name").
func editDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	d := make([][]int, len(ar)+1)
	for i := range d {
		d[i] = make([]int, len(br)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(ar); i++ {
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && ar[i-1] == br[j-2] && ar[i-2] == br[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(ar)][len(br)]
}

// exprScanner reads one assertion line.
type exprScanner struct {
	p    *Parser
	line int    // 1-based line number
	text string // the raw line, indentation included
	pos  int
}

func (s *exprScanner) fail(off int, format string, args ...any) error {
	if off > len(s.text) {
		off = len(s.text)
	}
	if off < 0 {
		off = 0
	}
	return &ParseError{
		File:    s.p.file,
		Line:    s.line,
		Column:  utf8.RuneCountInString(s.text[:off]) + 1,
		Message: fmt.Sprintf(format, args...),
		Snippet: s.text,
	}
}

func (s *exprScanner) skipSpace() {
	for s.pos < len(s.text) && (s.text[s.pos] == ' ' || s.text[s.pos] == '\t') {
		s.pos++
	}
}

func (s *exprScanner) atEnd() bool { return s.pos >= len(s.text) }

// scanSubject reads the subject up to whitespace outside (), [] and {{ }} so
// that a gjson query such as items.#(name=="Jane Doe").id stays one subject. A
// subject also ends where an operator begins without a space (status==200).
func (s *exprScanner) scanSubject() string {
	start := s.pos
	depth := 0
	var quote byte
	for s.pos < len(s.text) {
		c := s.text[s.pos]
		switch {
		case quote != 0:
			if c == '\\' && s.pos+1 < len(s.text) {
				s.pos++
			} else if c == quote {
				quote = 0
			}
		case strings.HasPrefix(s.text[s.pos:], "{{"):
			depth++
			s.pos++
		case depth > 0 && strings.HasPrefix(s.text[s.pos:], "}}"):
			depth--
			s.pos++
		case c == '(' || c == '[':
			depth++
		case (c == ')' || c == ']') && depth > 0:
			depth--
		case depth > 0 && (c == '"' || c == '\''):
			quote = c
		case depth == 0 && (c == ' ' || c == '\t'):
			return s.text[start:s.pos]
		case depth == 0 && (c == '=' || c == '<' || c == '>' || c == '!'):
			return s.text[start:s.pos]
		}
		s.pos++
	}
	return s.text[start:s.pos]
}

// peekWord returns the whitespace-delimited word at the cursor without moving it.
func (s *exprScanner) peekWord() string {
	i := s.pos
	for i < len(s.text) && s.text[i] != ' ' && s.text[i] != '\t' {
		i++
	}
	return s.text[s.pos:i]
}

// isOperatorWord reports whether w is a known word operator or starts with an
// operator symbol.
func isOperatorWord(w string) bool {
	if w == "" {
		return false
	}
	if strings.ContainsRune("=<>~", rune(w[0])) {
		return true
	}
	if w[0] == '!' {
		return len(w) > 1 && (w[1] == '=' || unicode.IsLetter(rune(w[1])))
	}
	_, ok := wordOperators[strings.ToLower(w)]
	return ok
}

// scanFullSubject reads the subject of an assertion, including the argument of
// "header <name>" and "jsonpath <path>". When special is false only the plain
// subject is read (>>>db columns, nested each subjects).
func (s *exprScanner) scanFullSubject(special bool) string {
	subject := s.scanSubject()
	if !special {
		return subject
	}
	switch strings.ToLower(subject) {
	case "header":
		save := s.pos
		s.skipSpace()
		if w := s.peekWord(); w != "" && !isOperatorWord(w) {
			s.pos += len(w)
			return subject + " " + w
		}
		s.pos = save
	case "jsonpath":
		s.skipSpace()
		start := s.pos
		path := s.scanSubject()
		if path != "" {
			return subject + " " + path
		}
		s.pos = start
	}
	return subject
}

// scanOperator reads the operator. A value that starts with a literal (number,
// quote, [, {, true/false/null) with no operator before it is the "==" shorthand
// (shorthand is true then and the cursor stays on the value). A bare word in
// operator position is an error: it is nearly always a typo.
func (s *exprScanner) scanOperator() (op AssertionOperator, shorthand bool, err error) {
	s.skipSpace()
	if s.atEnd() {
		return 0, false, s.fail(len(s.text), "missing operator and value (expected e.g. \"== 200\", \"exists\", \"contains \\\"x\\\"\")")
	}
	start := s.pos
	c := s.text[s.pos]

	isSymbol := strings.ContainsRune("=<>~", rune(c)) ||
		(c == '!' && (s.pos+1 >= len(s.text) || !unicode.IsLetter(rune(s.text[s.pos+1]))))
	if isSymbol {
		for s.pos < len(s.text) && strings.ContainsRune("=!<>~", rune(s.text[s.pos])) {
			s.pos++
		}
		sym := s.text[start:s.pos]
		if op, ok := symbolOperators[sym]; ok {
			return op, false, nil
		}
		msg := fmt.Sprintf("unknown operator %q", sym)
		if hint := suggestOperator(sym); hint != "" {
			msg += fmt.Sprintf(" (did you mean %q?)", hint)
		}
		return 0, false, s.fail(start, "%s", msg)
	}

	if unicode.IsLetter(rune(c)) || c == '_' || c == '!' {
		end := s.pos
		if c == '!' {
			end++
		}
		for end < len(s.text) && (unicode.IsLetter(rune(s.text[end])) || unicode.IsDigit(rune(s.text[end])) || s.text[end] == '_') {
			end++
		}
		word := s.text[s.pos:end]
		next := byte(' ')
		if end < len(s.text) {
			next = s.text[end]
		}
		if next == ' ' || next == '\t' || next == '"' || next == '\'' || next == '[' || next == '{' || next == '/' || next == '.' && word == "each" {
			lower := strings.ToLower(word)
			if lower == "true" || lower == "false" || lower == "null" {
				return OpEquals, true, nil
			}
			if op, ok := wordOperators[lower]; ok {
				s.pos = end
				if op == OpLength {
					op, err = s.scanLengthOperator()
					if err != nil {
						return 0, false, err
					}
				}
				return op, false, nil
			}
		}
		// Unknown: report the whole whitespace-delimited word.
		for end < len(s.text) && s.text[end] != ' ' && s.text[end] != '\t' {
			end++
		}
		word = s.text[start:end]
		msg := fmt.Sprintf("unknown operator %q", word)
		if hint := suggestOperator(word); hint != "" {
			msg += fmt.Sprintf(" (did you mean %q?)", hint)
		} else {
			msg += " (to compare with a string, quote it: == \"" + word + "\")"
		}
		return 0, false, s.fail(start, "%s", msg)
	}

	if c == '"' || c == '\'' || c == '[' || c == '{' || c == '-' || c == '+' || c == '.' || (c >= '0' && c <= '9') {
		return OpEquals, true, nil
	}
	return 0, false, s.fail(start, "expected an operator (%s), got %q", strings.Join(operatorSpellings, ", "), string(c))
}

// scanLengthOperator resolves the compound length forms ("length >", "length
// <="). "length == 3" is plain length. The cursor is just after "length".
func (s *exprScanner) scanLengthOperator() (AssertionOperator, error) {
	save := s.pos
	s.skipSpace()
	start := s.pos
	for s.pos < len(s.text) && strings.ContainsRune("=!<>", rune(s.text[s.pos])) {
		s.pos++
	}
	sym := s.text[start:s.pos]
	switch sym {
	case ">":
		return OpLengthGt, nil
	case ">=":
		return OpLengthGte, nil
	case "<":
		return OpLengthLt, nil
	case "<=":
		return OpLengthLte, nil
	case "==", "=":
		return OpLength, nil
	case "":
		s.pos = save
		return OpLength, nil
	}
	return 0, s.fail(start, "length supports ==, >, >=, < and <= (got %q)", sym)
}

// valueText returns the rest of the line without its trailing comment, trimmed,
// and the byte offset of its first character.
func (s *exprScanner) valueText() (string, int) {
	s.skipSpace()
	off := s.pos
	return stripTrailingComment(s.text[s.pos:]), off
}

var commentRe = regexp.MustCompile(`\s#(\s|$)`)

// stripTrailingComment removes a trailing "# comment": a '#' that follows
// whitespace and is itself followed by whitespace or the end of the line. A
// quoted string, list or object is delimited first, so a '#' inside it is data.
func stripTrailingComment(v string) string {
	v = strings.TrimSpace(v)
	end := -1
	switch {
	case v == "":
		return ""
	case v[0] == '"' || v[0] == '\'':
		end = closingQuote(v)
	case v[0] == '[':
		end = closingBracket(v)
	case v[0] == '{' && !strings.HasPrefix(v, "{{"):
		end = closingBracket(v)
	}
	if end >= 0 {
		rest := strings.TrimSpace(v[end+1:])
		if strings.HasPrefix(rest, "#") {
			return strings.TrimSpace(v[:end+1])
		}
		return v
	}
	if end == -1 && (v[0] == '"' || v[0] == '\'' || v[0] == '[' || (v[0] == '{' && !strings.HasPrefix(v, "{{"))) {
		return v // unterminated: let the value parser report it
	}
	if loc := commentRe.FindStringIndex(v); loc != nil {
		return strings.TrimSpace(v[:loc[0]])
	}
	if v[0] == '#' {
		// "expect status 200 #note" style is not a comment (no whitespace before
		// '#'), but a value that is only a comment is empty.
		if len(v) == 1 || v[1] == ' ' || v[1] == '\t' {
			return ""
		}
	}
	return v
}

// closingQuote returns the index of the quote that closes the string starting
// at v[0], or -1.
func closingQuote(v string) int {
	q := v[0]
	for i := 1; i < len(v); i++ {
		switch v[i] {
		case '\\':
			i++
		case q:
			return i
		}
	}
	return -1
}

// closingBracket returns the index of the bracket that closes the [ or { at
// v[0], honoring nesting, strings and {{ }}, or -1.
func closingBracket(v string) int {
	depth := 0
	for i := 0; i < len(v); i++ {
		switch c := v[i]; {
		case c == '"' || c == '\'':
			if j := closingQuote(v[i:]); j >= 0 {
				i += j
			} else {
				return -1
			}
		case strings.HasPrefix(v[i:], "{{"):
			if j := strings.Index(v[i:], "}}"); j >= 0 {
				i += j + 1
			}
		case c == '[' || c == '{':
			depth++
		case c == ']' || c == '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

type valueKind int

const (
	kindLiteral  valueKind = iota // number, bool, null, list, object
	kindQuoted                    // a quoted string
	kindVerbatim                  // the rest of the line, as written
)

var numberRe = regexp.MustCompile(`^-?\d+(\.\d+)?([eE][+-]?\d+)?$`)

// parseGeneral applies the expected-value grammar to v (already comment-free
// and trimmed). off is the byte offset of v in the line, for error columns.
func (s *exprScanner) parseGeneral(v string, off int) (any, valueKind, error) {
	switch {
	case v[0] == '"' || v[0] == '\'':
		end := closingQuote(v)
		if end < 0 {
			return nil, 0, s.fail(off, "unterminated string %s (a closing %c is missing)", v, v[0])
		}
		if rest := strings.TrimSpace(v[end+1:]); rest != "" {
			return nil, 0, s.fail(off+end+1, "unexpected text %q after the string %s (quote the whole value or remove the quotes)", rest, v[:end+1])
		}
		return decodeQuoted(v[:end+1]), kindQuoted, nil
	case v[0] == '[':
		end := closingBracket(v)
		if end < 0 {
			return nil, 0, s.fail(off, "unterminated list %s (a closing ] is missing)", v)
		}
		if rest := strings.TrimSpace(v[end+1:]); rest != "" {
			return nil, 0, s.fail(off+end+1, "unexpected text %q after the list", rest)
		}
		list, err := s.parseList(v[1:end], off+1)
		return list, kindLiteral, err
	case v[0] == '{' && !strings.HasPrefix(v, "{{"):
		end := closingBracket(v)
		if end < 0 {
			return nil, 0, s.fail(off, "unterminated JSON object %s (a closing } is missing)", v)
		}
		if rest := strings.TrimSpace(v[end+1:]); rest != "" {
			return nil, 0, s.fail(off+end+1, "unexpected text %q after the JSON object", rest)
		}
		obj, err := decodeJSONValue(v)
		if err != nil {
			return nil, 0, s.fail(off, "invalid JSON object: %v", err)
		}
		return obj, kindLiteral, nil
	}
	switch strings.ToLower(v) {
	case "true":
		return true, kindLiteral, nil
	case "false":
		return false, kindLiteral, nil
	case "null":
		return nil, kindLiteral, nil
	}
	if numberRe.MatchString(v) {
		return parseNumber(v), kindLiteral, nil
	}
	return v, kindVerbatim, nil
}

// parseNumber converts numeric text to int when it is integral and fits,
// otherwise float64.
func parseNumber(v string) any {
	if !strings.ContainsAny(v, ".eE") {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return v
}

// decodeQuoted decodes a quoted string literal (quotes included). Double-quoted
// strings are JSON-decoded (\n, \", \\, \uXXXX); when the text is not valid JSON
// (a regex such as "\d+") its contents are kept as written. Single-quoted
// strings keep their contents as written except \'.
func decodeQuoted(lit string) string {
	inner := lit[1 : len(lit)-1]
	if lit[0] == '"' {
		var out string
		if err := json.Unmarshal([]byte(lit), &out); err == nil {
			return out
		}
		return strings.ReplaceAll(inner, `\"`, `"`)
	}
	return strings.ReplaceAll(inner, `\'`, `'`)
}

// decodeJSONValue decodes one JSON value, mapping numbers to int or float64.
func decodeJSONValue(text string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected trailing data")
	}
	return normalizeJSONNumbers(v), nil
}

func normalizeJSONNumbers(v any) any {
	switch t := v.(type) {
	case json.Number:
		return parseNumber(t.String())
	case []any:
		for i := range t {
			t[i] = normalizeJSONNumbers(t[i])
		}
		return t
	case map[string]any:
		for k := range t {
			t[k] = normalizeJSONNumbers(t[k])
		}
		return t
	}
	return v
}

// parseList parses the inside of [ ... ]. Elements are separated by commas
// outside strings, nested brackets and {{ }}; each follows the value grammar.
func (s *exprScanner) parseList(inner string, off int) ([]any, error) {
	list := []any{}
	start := 0
	depth := 0
	flush := func(end int) error {
		raw := inner[start:end]
		elem := strings.TrimSpace(raw)
		if elem == "" {
			return nil
		}
		lead := len(raw) - len(strings.TrimLeft(raw, " \t"))
		v, _, err := s.parseGeneral(elem, off+start+lead)
		if err != nil {
			return err
		}
		list = append(list, v)
		return nil
	}
	for i := 0; i < len(inner); i++ {
		switch c := inner[i]; {
		case c == '"' || c == '\'':
			if j := closingQuote(inner[i:]); j >= 0 {
				i += j
			} else {
				return nil, s.fail(off+i, "unterminated string in list")
			}
		case strings.HasPrefix(inner[i:], "{{"):
			if j := strings.Index(inner[i:], "}}"); j >= 0 {
				i += j + 1
			}
		case c == '[' || c == '{':
			depth++
		case c == ']' || c == '}':
			depth--
		case c == ',' && depth == 0:
			if err := flush(i); err != nil {
				return nil, err
			}
			start = i + 1
		}
	}
	if err := flush(len(inner)); err != nil {
		return nil, err
	}
	return list, nil
}

// numericSubjects are subjects whose value is always a number.
func isNumericSubject(subject string) bool {
	switch strings.ToLower(subject) {
	case "status", "duration", "p50", "p95", "p99":
		return true
	}
	return false
}

func hasTemplate(v string) bool { return strings.Contains(v, "{{") }

// parseExpected parses the value text for operator op.
func (s *exprScanner) parseExpected(subject string, op AssertionOperator, shorthand bool, v string, off int) (any, error) {
	name := op.String()
	if shorthand {
		name = "=="
	}
	if v == "" {
		switch op {
		case OpExists, OpNotExists:
			return nil, nil
		case OpSnapshot:
			return nil, nil
		}
		return nil, s.fail(len(s.text), "operator %s needs a value", name)
	}

	switch op {
	case OpExists, OpNotExists:
		return nil, s.fail(off, "unexpected text %q after %s (it takes no value)", v, name)

	case OpType:
		w := strings.ToLower(v)
		if hasTemplate(v) {
			return v, nil
		}
		for _, t := range typeNames {
			if w == t {
				return t, nil
			}
		}
		msg := fmt.Sprintf("unknown type %q (expected %s)", v, strings.Join(typeNames, ", "))
		if hint := closest(w, typeNames, true); hint != "" {
			msg = fmt.Sprintf("unknown type %q (did you mean %q?)", v, hint)
		}
		return nil, s.fail(off, "%s", msg)

	case OpSchema, OpSnapshot:
		if v[0] == '"' || v[0] == '\'' {
			val, _, err := s.parseGeneral(v, off)
			return val, err
		}
		return v, nil

	case OpMatches:
		return s.parseRegex(v, off)

	case OpIn, OpNotIn:
		if v[0] != '[' && !(strings.HasPrefix(v, "{{") && strings.HasSuffix(v, "}}")) {
			return nil, s.fail(off, "operator %s needs a list, e.g. %s [200, 201]", name, name)
		}
		val, _, err := s.parseGeneral(v, off)
		return val, err

	case OpGreaterThan, OpGreaterOrEqual, OpLessThan, OpLessOrEqual,
		OpLength, OpLengthGt, OpLengthGte, OpLengthLt, OpLengthLte:
		val, kind, err := s.parseGeneral(v, off)
		if err != nil {
			return nil, err
		}
		if !numericValue(val) && !(kind == kindVerbatim && hasTemplate(v)) {
			if str, ok := val.(string); ok && kind == kindQuoted && (hasTemplate(str) || isNumberText(str)) {
				return val, nil
			}
			return nil, s.fail(off, "operator %s needs a number, got %q", name, v)
		}
		return val, nil
	}

	// Substring tests take text: a quoted value is decoded, anything else is
	// the text as written ("startsWith {", "contains 1.50", "contains [\"a\"").
	switch op {
	case OpContains, OpNotContains, OpStartsWith, OpEndsWith:
		if (v[0] == '"' || v[0] == '\'') && closingQuote(v) == len(v)-1 {
			return decodeQuoted(v), nil
		}
		return v, nil
	}

	// ==, != and the array operators.
	val, kind, err := s.parseGeneral(v, off)
	if err != nil {
		return nil, err
	}
	if kind == kindVerbatim && isNumericSubject(subject) && (op == OpEquals || op == OpNotEquals) && !hasTemplate(v) {
		msg := fmt.Sprintf("%s is a number but %q is not", subject, v)
		if fields := strings.Fields(v); len(fields) > 1 && allNumbers(fields) {
			msg += fmt.Sprintf(" (to accept several values write: expect %s in [%s])", subject, strings.Join(fields, ", "))
		}
		return nil, s.fail(off, "%s", msg)
	}
	return val, nil
}

func numericValue(v any) bool {
	switch v.(type) {
	case int, int64, float64:
		return true
	}
	return false
}

func isNumberText(v string) bool { return numberRe.MatchString(strings.TrimSpace(v)) }

func allNumbers(fields []string) bool {
	for _, f := range fields {
		if !numberRe.MatchString(f) {
			return false
		}
	}
	return true
}

var regexFlagsRe = regexp.MustCompile(`^[gimsuyUd]*$`)

// parseRegex parses the value of matches: a /pattern/flags literal, converted
// to a Go pattern ("(?i)pattern"), or any other value used as the pattern as is.
func (s *exprScanner) parseRegex(v string, off int) (any, error) {
	pattern := ""
	if v[0] == '/' {
		if j := strings.LastIndex(v, "/"); j > 0 && regexFlagsRe.MatchString(v[j+1:]) {
			flags := ""
			for _, f := range v[j+1:] {
				switch f {
				case 'i', 'm', 's', 'U':
					if !strings.ContainsRune(flags, f) {
						flags += string(f)
					}
				case 'g':
					// every match is always tested; accepted for JS familiarity
				default:
					return nil, s.fail(off+j+1, "unsupported regex flag %q (supported: i, m, s, U)", string(f))
				}
			}
			pattern = v[1:j]
			if pattern == "" {
				return nil, s.fail(off, "empty regular expression")
			}
			if flags != "" {
				pattern = "(?" + flags + ")" + pattern
			}
			return s.checkRegex(pattern, off)
		}
	}
	val, _, err := s.parseGeneral(v, off)
	if err != nil {
		return nil, err
	}
	pattern = fmt.Sprint(val)
	return s.checkRegex(pattern, off)
}

func (s *exprScanner) checkRegex(pattern string, off int) (any, error) {
	if !hasTemplate(pattern) {
		if _, err := regexp.Compile(pattern); err != nil {
			return nil, s.fail(off, "invalid regular expression: %v", err)
		}
	}
	return pattern, nil
}

// parseCondition scans an operator and its value from the cursor to the end of
// the line. subject is only used to judge numeric subjects. allowEach is false
// for the operator nested inside "each".
func (s *exprScanner) parseCondition(subject string, allowEach bool) (AssertionOperator, any, error) {
	opStart := s.pos
	op, shorthand, err := s.scanOperator()
	if err != nil {
		return 0, nil, err
	}
	if op == OpEach {
		if !allowEach {
			return 0, nil, s.fail(opStart, "each cannot be nested")
		}
		spec, err := s.parseEach()
		return op, spec, err
	}
	v, off := s.valueText()
	expected, err := s.parseExpected(subject, op, shorthand, v, off)
	return op, expected, err
}

// parseEach parses what follows "each": an operator and value applied to every
// element ("each type object", "each > 0"), an element path before the operator
// ("each body.id exists", "each .email matches /@/"), or an operator/value map
// ({"operator": "type", "value": "object"}).
func (s *exprScanner) parseEach() (*EachSpec, error) {
	spec := &EachSpec{}
	// "each.id exists": the path is glued to the keyword.
	if !s.atEnd() && s.text[s.pos] == '.' {
		spec.Path = strings.TrimPrefix(s.scanSubject(), ".")
	}
	s.skipSpace()
	if s.atEnd() {
		return nil, s.fail(len(s.text), "each needs an assertion to apply to every element, e.g. each type object")
	}

	if s.text[s.pos] == '{' && !strings.HasPrefix(s.text[s.pos:], "{{") {
		v, off := s.valueText()
		raw, err := decodeJSONValue(v)
		if err != nil {
			return nil, s.fail(off, "invalid JSON object: %v", err)
		}
		m, ok := raw.(map[string]any)
		opName, hasOp := m["operator"]
		val, hasVal := m["value"]
		if !ok || !hasOp || !hasVal {
			return nil, s.fail(off, `each takes {"operator": ..., "value": ...}`)
		}
		name := fmt.Sprint(opName)
		switch name {
		case "equals":
			name = "=="
		case "notEquals":
			name = "!="
		}
		op, found := LookupOperator(name)
		if !found || op == OpEach {
			return nil, s.fail(off, "unknown operator %q in each", name)
		}
		spec.Operator, spec.Expected = op, val
		return spec, nil
	}

	// Is the next word an element path, or already the operator/value?
	if spec.Path == "" {
		if w := s.peekWord(); !isOperatorWord(w) && !looksLikeLiteral(w) {
			path := strings.TrimPrefix(s.scanSubject(), "body")
			spec.Path = strings.TrimPrefix(path, ".")
		}
	}
	op, expected, err := s.parseCondition(spec.Path, false)
	if err != nil {
		return nil, err
	}
	spec.Operator, spec.Expected = op, expected
	return spec, nil
}

// looksLikeLiteral reports whether w starts like a value (the == shorthand).
func looksLikeLiteral(w string) bool {
	if w == "" {
		return false
	}
	switch c := w[0]; {
	case c == '"' || c == '\'' || c == '[' || (c == '{' && !strings.HasPrefix(w, "{{")):
		return true
	case c == '-' || c == '+' || c == '.' || (c >= '0' && c <= '9'):
		return true
	}
	switch strings.ToLower(w) {
	case "true", "false", "null":
		return true
	}
	return false
}
