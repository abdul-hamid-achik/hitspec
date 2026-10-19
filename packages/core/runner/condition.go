package runner

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
)

// evaluateCondition checks if a request's condition is met.
// For @if, the request runs only if the expression is true.
// For @unless, the request runs only if the expression is false.
//
// An expression is a value ({{runSlow}}), a comparison ({{env}} == "prod",
// {{env}} != staging), or those combined with ! && || (&& binds tighter than
// ||). A bare value is true unless it is empty, false, 0, no, null or an
// unresolved variable. Each side of a comparison is resolved on its own, then
// compared as text (or as numbers when both are numbers); quotes around either
// side are ignored.
func (r *Runner) evaluateCondition(cond *parser.Condition) bool {
	if cond == nil {
		return true // no condition means always run
	}

	truthy := r.evalOr(cond.Expression)

	switch cond.Type {
	case parser.ConditionIf:
		return truthy
	case parser.ConditionUnless:
		return !truthy
	default:
		return true
	}
}

func (r *Runner) evalOr(expr string) bool {
	for _, part := range splitCondition(expr, "||") {
		if r.evalAnd(part) {
			return true
		}
	}
	return false
}

func (r *Runner) evalAnd(expr string) bool {
	for _, part := range splitCondition(expr, "&&") {
		if !r.evalTerm(part) {
			return false
		}
	}
	return true
}

func (r *Runner) evalTerm(term string) bool {
	term = strings.TrimSpace(term)
	if strings.HasPrefix(term, "!") && !strings.HasPrefix(term, "!=") {
		return !r.evalTerm(term[1:])
	}
	if i, op := findComparison(term); i >= 0 {
		left := r.operand(term[:i])
		right := r.operand(term[i+len(op):])
		equal := conditionEqual(left, right)
		if op == "!=" {
			return !equal
		}
		return equal
	}
	return isTruthy(r.resolver.Resolve(term))
}

// operand resolves one side of a comparison. A variable that does not resolve
// counts as empty, so {{missing}} == "prod" is false and {{missing}} != "prod"
// is true.
func (r *Runner) operand(s string) string {
	resolved := strings.TrimSpace(r.resolver.Resolve(strings.TrimSpace(s)))
	resolved = unresolvedRef.ReplaceAllString(resolved, "")
	return unquoteCondition(strings.TrimSpace(resolved))
}

var unresolvedRef = regexp.MustCompile(`\{\{[^}]*\}\}`)

func unquoteCondition(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

func conditionEqual(a, b string) bool {
	if af, err := strconv.ParseFloat(a, 64); err == nil {
		if bf, err := strconv.ParseFloat(b, 64); err == nil {
			return af == bf
		}
	}
	return a == b
}

// findComparison returns the index of the first == or != outside quotes and
// {{ }}, and which one it is.
func findComparison(s string) (int, string) {
	var quote byte
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case strings.HasPrefix(s[i:], "{{"):
			depth++
			i++
		case depth > 0 && strings.HasPrefix(s[i:], "}}"):
			depth--
			i++
		case depth > 0:
		case c == '"' || c == '\'':
			quote = c
		case strings.HasPrefix(s[i:], "=="):
			return i, "=="
		case strings.HasPrefix(s[i:], "!="):
			return i, "!="
		}
	}
	return -1, ""
}

// splitCondition splits s on sep outside quotes and {{ }}.
func splitCondition(s, sep string) []string {
	var parts []string
	var quote byte
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case strings.HasPrefix(s[i:], "{{"):
			depth++
			i++
		case depth > 0 && strings.HasPrefix(s[i:], "}}"):
			depth--
			i++
		case depth > 0:
		case c == '"' || c == '\'':
			quote = c
		case strings.HasPrefix(s[i:], sep):
			parts = append(parts, s[start:i])
			i += len(sep) - 1
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

// isTruthy determines if a resolved string value is considered "true".
// Empty strings, "false", "0", "no", "null", and unresolved variables are falsy.
func isTruthy(value string) bool {
	v := strings.TrimSpace(strings.ToLower(value))
	if v == "" || v == "false" || v == "0" || v == "no" || v == "null" {
		return false
	}
	// Unresolved variable references (still contains {{...}}) are falsy
	if strings.Contains(v, "{{") && strings.Contains(v, "}}") {
		return false
	}
	return true
}
