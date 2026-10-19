package env

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/builtin"
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
	// shadowed keeps the values a self-referencing definition (e.g.
	// `@baseUrl = {{baseUrl}}`) replaced in variables, newest last, so the
	// self-reference can be resolved against the lower-precedence layers.
	shadowed map[string][]any
	captures map[string]any
	dotenv   map[string]string
	funcs    *builtin.Registry
	warnFunc WarnFunc
}

func NewResolver() *Resolver {
	return &Resolver{
		variables: make(map[string]any),
		shadowed:  make(map[string][]any),
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
		delete(r.shadowed, k)
	}
}

// SetVariable defines name, replacing any earlier definition. When the new
// value references name itself (`@baseUrl = {{baseUrl}}`, the REST Client
// idiom for "default to whatever the environment says"), the replaced value is
// kept as a lower layer that the self-reference resolves against.
func (r *Resolver) SetVariable(name string, value any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, had := r.variables[name]
	if had && referencesVariable(value, name) {
		r.shadowed[name] = append(r.shadowed[name], old)
	} else {
		delete(r.shadowed, name)
	}
	r.variables[name] = value
}

// referencesVariable reports whether a value is a string containing {{name}}.
func referencesVariable(value any, name string) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	for _, m := range variablePattern.FindAllStringSubmatch(s, -1) {
		if strings.TrimSpace(m[1]) == name {
			return true
		}
	}
	return false
}

// FormatValue renders a variable or captured value for interpolation into a
// request. Strings are inserted verbatim; numbers keep their exact JSON text
// (1000000, never 1e+06); booleans are true/false; nil is null; maps and
// slices become compact JSON.
func FormatValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		return strconv.FormatBool(x)
	case []byte:
		return string(x)
	case float64, float32:
		if b, ok := marshalCompact(x); ok {
			return b
		}
		return fmt.Sprintf("%v", x)
	}
	switch reflect.ValueOf(v).Kind() {
	case reflect.Map, reflect.Slice, reflect.Array, reflect.Struct:
		if b, ok := marshalCompact(v); ok {
			return b
		}
	}
	return fmt.Sprintf("%v", v)
}

// marshalCompact encodes v as JSON without HTML escaping or a trailing newline.
func marshalCompact(v any) (string, bool) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", false
	}
	return strings.TrimRight(buf.String(), "\n"), true
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

// Resolve expands {{variable}}, {{$function()}}, {{capture}}, and ${VAR}
// references in input. Resolution is recursive: a variable's value may itself
// contain {{...}} references (e.g. @apiUrl = {{base}}/v1 where @base =
// {{scheme}}://api.example.com). A single pass left nested references literal,
// so {{apiUrl}} resolved to "{{base}}/v1" instead of "https://api.example.com/v1".
//
// Precedence, highest first: captures, variables (the hitspec.yaml environment,
// overridden by the file's own @variables), dotenv, the OS environment.
//
// A definition that references its own name (@baseUrl = {{baseUrl}}) resolves
// that reference against the next lower layer, so a file can default to the
// environment. If no lower layer defines it, the reference stays literal.
// Loops between distinct variables (a -> b -> a) are broken by a visited set
// and also stay literal.
//
// Captured values come from server responses, so they are inserted literally
// and never scanned again: a response containing "{{$env(SECRET)}}" must not
// make hitspec read local state.
func (r *Resolver) Resolve(input string) string {
	return r.resolve(input, &resolveState{seen: map[string]bool{}, skip: map[string]int{}})
}

// resolveState tracks one top-level Resolve call.
type resolveState struct {
	seen map[string]bool // variables being expanded (cycle guard)
	skip map[string]int  // per name: how many upper layers a self-reference skips
}

func (r *Resolver) resolve(input string, st *resolveState) string {
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

		// Cycle guard: a variable that (transitively) references itself must not
		// recurse forever. Leave the literal {{expr}} in place for this branch.
		if st.seen[expr] {
			r.warn("circular variable reference: %s", expr)
			return match
		}

		r.mu.RLock()
		capVal, isCapture := r.captures[expr]
		layers := r.layersLocked(expr)
		r.mu.RUnlock()

		// Captures hold response data: literal, never re-expanded.
		if isCapture && st.skip[expr] == 0 {
			return FormatValue(capVal)
		}

		// The first layer not skipped by an enclosing self-reference decides.
		if i := st.skip[expr]; i < len(layers) {
			replaced := FormatValue(layers[i])
			if !variablePattern.MatchString(replaced) {
				return replaced
			}
			// The value itself contains {{...}}: expand nested references. A
			// self-reference continues with the layers below this one.
			if referencesVariable(replaced, expr) {
				prev := st.skip[expr]
				st.skip[expr] = i + 1
				resolved := r.resolve(replaced, st)
				st.skip[expr] = prev
				return resolved
			}
			st.seen[expr] = true
			resolved := r.resolve(replaced, st)
			delete(st.seen, expr)
			return resolved
		}

		// Fallback to OS environment variable
		if val := os.Getenv(expr); val != "" {
			return val
		}

		if !strings.Contains(expr, ".") {
			r.warn("unresolved variable: %s", expr)
		}
		return match
	})
}

// layersLocked returns the definitions of name below the captures, highest
// precedence first: the current variable, any definitions it shadowed, dotenv.
// Callers must hold r.mu.
func (r *Resolver) layersLocked(name string) []any {
	var layers []any
	if v, ok := r.variables[name]; ok {
		layers = append(layers, v)
		sh := r.shadowed[name]
		for i := len(sh) - 1; i >= 0; i-- {
			layers = append(layers, sh[i])
		}
	}
	if v, ok := r.dotenv[name]; ok {
		layers = append(layers, v)
	}
	return layers
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
	for k, v := range r.shadowed {
		clone.shadowed[k] = append([]any(nil), v...)
	}
	for k, v := range r.captures {
		clone.captures[k] = v
	}
	for k, v := range r.dotenv {
		clone.dotenv[k] = v
	}
	return clone
}

// HasUnresolvedVariables checks if the resolved string still contains unresolved {{...}} variables
func (r *Resolver) HasUnresolvedVariables(input string) bool {
	resolved := r.Resolve(input)
	return variablePattern.MatchString(resolved)
}

// GetUnresolvedVariables returns a list of variable names that could not be resolved
func (r *Resolver) GetUnresolvedVariables(input string) []string {
	resolved := r.Resolve(input)
	matches := variablePattern.FindAllStringSubmatch(resolved, -1)
	if len(matches) == 0 {
		return nil
	}
	vars := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			vars = append(vars, match[1])
		}
	}
	return vars
}
