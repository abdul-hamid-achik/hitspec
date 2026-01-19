package env

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/abdul-hamid-achik/hitspec/packages/builtin"
)

var variablePattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// WarnFunc is a function type for handling warnings
type WarnFunc func(format string, args ...any)

// Resolver handles variable resolution with thread-safe access to variables and captures.
// It supports environment variables, built-in functions, captures from previous requests,
// and user-defined variables.
type Resolver struct {
	mu        sync.RWMutex
	variables map[string]any
	captures  map[string]any
	dotenv    map[string]string
	funcs     *builtin.Registry
	warnFunc  WarnFunc
}

func NewResolver() *Resolver {
	return &Resolver{
		variables: make(map[string]any),
		captures:  make(map[string]any),
		dotenv:    make(map[string]string),
		funcs:     builtin.NewRegistry(),
	}
}

// LoadDotEnv loads variables from a .env file for variable interpolation.
// Variables are also exported to the OS environment so ${VAR} syntax works
// in config files loaded after the .env file.
func (r *Resolver) LoadDotEnv(path string) error {
	vars, err := LoadAndExportDotEnv(path)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, v := range vars {
		r.dotenv[k] = v
	}
	return nil
}

// SetWarnFunc sets a function to be called when warnings occur (e.g., unresolved variables)
func (r *Resolver) SetWarnFunc(fn WarnFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.warnFunc = fn
}

func (r *Resolver) warn(format string, args ...any) {
	r.mu.RLock()
	fn := r.warnFunc
	r.mu.RUnlock()
	if fn != nil {
		fn(format, args...)
	}
}

func (r *Resolver) SetVariables(vars map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, v := range vars {
		r.variables[k] = v
	}
}

func (r *Resolver) SetVariable(name string, value any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.variables[name] = value
}

func (r *Resolver) SetCapture(requestName, captureName string, value any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := requestName + "." + captureName
	r.captures[key] = value
	r.captures[captureName] = value
}

func (r *Resolver) GetCapture(name string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.captures[name]
	return v, ok
}

func (r *Resolver) Resolve(input string) string {
	return variablePattern.ReplaceAllStringFunc(input, func(match string) string {
		expr := match[2 : len(match)-2]
		expr = strings.TrimSpace(expr)

		if strings.HasPrefix(expr, "$") {
			funcExpr := expr[1:] // strip the $
			// Check if it's a function call (has parentheses)
			if strings.Contains(funcExpr, "(") {
				if result, ok := r.funcs.Call(funcExpr); ok {
					return fmt.Sprintf("%v", result)
				}
				r.warn("unresolved function call: %s", expr)
				return match
			}
			// Otherwise treat as environment variable
			// Check dotenv first, then OS env
			r.mu.RLock()
			if val, ok := r.dotenv[funcExpr]; ok {
				r.mu.RUnlock()
				return val
			}
			r.mu.RUnlock()
			if val := os.Getenv(funcExpr); val != "" {
				return val
			}
			r.warn("unresolved environment variable: $%s", funcExpr)
			return match
		}

		// Keep for backward compatibility with non-$ function calls
		if strings.Contains(expr, "(") {
			if result, ok := r.funcs.Call(expr); ok {
				return fmt.Sprintf("%v", result)
			}
			r.warn("unresolved function call: %s", expr)
			return match
		}

		r.mu.RLock()
		if val, ok := r.captures[expr]; ok {
			r.mu.RUnlock()
			return fmt.Sprintf("%v", val)
		}

		if val, ok := r.variables[expr]; ok {
			r.mu.RUnlock()
			return fmt.Sprintf("%v", val)
		}

		// Check dotenv file
		if val, ok := r.dotenv[expr]; ok {
			r.mu.RUnlock()
			return val
		}
		r.mu.RUnlock()

		// Fallback to OS environment variable
		if val := os.Getenv(expr); val != "" {
			return val
		}

		r.warn("unresolved variable: %s", expr)
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
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.captures[name]; ok {
		return true
	}
	if _, ok := r.variables[name]; ok {
		return true
	}
	return false
}

func (r *Resolver) GetVariable(name string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if v, ok := r.captures[name]; ok {
		return v, true
	}
	if v, ok := r.variables[name]; ok {
		return v, true
	}
	return nil, false
}

func (r *Resolver) Clone() *Resolver {
	r.mu.RLock()
	defer r.mu.RUnlock()
	clone := NewResolver()
	for k, v := range r.variables {
		clone.variables[k] = v
	}
	for k, v := range r.captures {
		clone.captures[k] = v
	}
	for k, v := range r.dotenv {
		clone.dotenv[k] = v
	}
	return clone
}
