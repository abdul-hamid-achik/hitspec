package env

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/packages/builtin"
)

var variablePattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

type Resolver struct {
	variables map[string]any
	captures  map[string]any
	funcs     *builtin.Registry
}

func NewResolver() *Resolver {
	return &Resolver{
		variables: make(map[string]any),
		captures:  make(map[string]any),
		funcs:     builtin.NewRegistry(),
	}
}

func (r *Resolver) SetVariables(vars map[string]any) {
	for k, v := range vars {
		r.variables[k] = v
	}
}

func (r *Resolver) SetVariable(name string, value any) {
	r.variables[name] = value
}

func (r *Resolver) SetCapture(requestName, captureName string, value any) {
	key := requestName + "." + captureName
	r.captures[key] = value
	r.captures[captureName] = value
}

func (r *Resolver) GetCapture(name string) (any, bool) {
	v, ok := r.captures[name]
	return v, ok
}

func (r *Resolver) Resolve(input string) string {
	return variablePattern.ReplaceAllStringFunc(input, func(match string) string {
		expr := match[2 : len(match)-2]
		expr = strings.TrimSpace(expr)

		if strings.HasPrefix(expr, "$") {
			envVar := expr[1:]
			if val := os.Getenv(envVar); val != "" {
				return val
			}
			return match
		}

		if strings.Contains(expr, "(") {
			if result, ok := r.funcs.Call(expr); ok {
				return fmt.Sprintf("%v", result)
			}
			return match
		}

		if val, ok := r.captures[expr]; ok {
			return fmt.Sprintf("%v", val)
		}

		if val, ok := r.variables[expr]; ok {
			return fmt.Sprintf("%v", val)
		}

		return match
	})
}

func (r *Resolver) ResolveAll(values map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range values {
		result[k] = r.Resolve(v)
	}
	return result
}

func (r *Resolver) HasVariable(name string) bool {
	if _, ok := r.captures[name]; ok {
		return true
	}
	if _, ok := r.variables[name]; ok {
		return true
	}
	return false
}

func (r *Resolver) GetVariable(name string) (any, bool) {
	if v, ok := r.captures[name]; ok {
		return v, true
	}
	if v, ok := r.variables[name]; ok {
		return v, true
	}
	return nil, false
}

func (r *Resolver) Clone() *Resolver {
	clone := NewResolver()
	for k, v := range r.variables {
		clone.variables[k] = v
	}
	for k, v := range r.captures {
		clone.captures[k] = v
	}
	return clone
}
