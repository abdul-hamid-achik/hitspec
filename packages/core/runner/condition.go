package runner

import (
	"strings"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

// evaluateCondition checks if a request's condition is met.
// For @if, the request runs only if the expression resolves to a truthy value.
// For @unless, the request runs only if the expression resolves to a falsy value.
func (r *Runner) evaluateCondition(cond *parser.Condition) bool {
	if cond == nil {
		return true // no condition means always run
	}

	resolved := r.resolver.Resolve(cond.Expression)
	truthy := isTruthy(resolved)

	switch cond.Type {
	case parser.ConditionIf:
		return truthy
	case parser.ConditionUnless:
		return !truthy
	default:
		return true
	}
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
