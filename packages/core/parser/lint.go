package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// LintResult holds the whole-file findings of Lint.
type LintResult struct {
	// Errors are problems that make `hitspec run` refuse the file (duplicate
	// request names, dependency cycles).
	Errors []Warning
	// Warnings are probable mistakes (a @depends on a request that does not
	// exist, a {{variable}} nothing defines).
	Warnings []Warning
}

var varRefRe = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

// Lint checks relations between the requests of a parsed file. defined reports
// whether a variable name is provided from outside the file (an environment,
// .env, the process); it may be nil, in which case undefined variables are not
// reported (a variable can come from anywhere at run time, so this check only
// makes sense when the caller knows the environments).
func Lint(f *File, defined func(name string) bool) LintResult {
	var res LintResult
	at := func(line int, format string, args ...any) Warning {
		return Warning{File: f.Path, Line: line, Column: 1, Message: fmt.Sprintf(format, args...)}
	}

	byName := map[string]*Request{}
	for _, r := range f.Requests {
		if r.Name == "" {
			continue
		}
		if first, dup := byName[r.Name]; dup {
			res.Errors = append(res.Errors, at(r.Line, "duplicate request name %q (first defined at line %d); request names must be unique", r.Name, first.Line))
			continue
		}
		byName[r.Name] = r
	}

	// @depends: unknown targets and cycles.
	for _, r := range f.Requests {
		if r.Metadata == nil {
			continue
		}
		for _, dep := range r.Metadata.Depends {
			if hasTemplate(dep) {
				continue
			}
			if _, ok := byName[dep]; !ok {
				res.Warnings = append(res.Warnings, at(r.Line, "request %q depends on %q, which does not exist in this file", requestLabel(r), dep))
			}
		}
	}
	if cycle := findDependencyCycle(f, byName); cycle != "" {
		line := 1
		if len(f.Requests) > 0 {
			line = f.Requests[0].Line
		}
		res.Errors = append(res.Errors, at(line, "circular @depends: %s", cycle))
	}

	if defined == nil {
		return res
	}

	// Variables: collect every name the file itself provides.
	local := map[string]bool{}
	for _, v := range f.Variables {
		local[v.Name] = true
	}
	for _, r := range f.Requests {
		for _, c := range r.Captures {
			local[c.Name] = true
		}
	}
	reachable := transitiveDeps(f, byName)

	check := func(r *Request, line int, text string) {
		for _, m := range varRefRe.FindAllStringSubmatch(text, -1) {
			ref := m[1]
			if strings.HasPrefix(ref, "$") || strings.Contains(ref, "(") {
				continue // built-in function or environment variable
			}
			if prefix, name, dotted := strings.Cut(ref, "."); dotted {
				src, isReq := byName[prefix]
				if !isReq {
					continue // a nested value from an environment
				}
				if !hasCapture(src, name) {
					res.Warnings = append(res.Warnings, at(line, "{{%s}}: request %q has no capture named %q", ref, prefix, name))
				} else if r != nil && r != src && !reachable[r][prefix] {
					res.Warnings = append(res.Warnings, at(line, "{{%s}} is used without \"# @depends %s\", so %s may not have run yet", ref, prefix, prefix))
				}
				continue
			}
			if local[ref] || defined(ref) {
				continue
			}
			res.Warnings = append(res.Warnings, at(line, "variable {{%s}} is not defined in the file, a capture, the environments or the process environment", ref))
		}
	}
	for _, v := range f.Variables {
		check(nil, v.Line, v.Value)
	}
	for _, r := range f.Requests {
		forEachTemplateText(r, func(line int, text string) { check(r, line, text) })
	}
	return res
}

func hasCapture(r *Request, name string) bool {
	for _, c := range r.Captures {
		if c.Name == name {
			return true
		}
	}
	return false
}

func requestLabel(r *Request) string {
	if r.Name != "" {
		return r.Name
	}
	return r.Method + " " + r.URL
}

// transitiveDeps maps each request to the set of request names it depends on,
// directly or through other requests.
func transitiveDeps(f *File, byName map[string]*Request) map[*Request]map[string]bool {
	out := map[*Request]map[string]bool{}
	var walk func(r *Request, set map[string]bool, seen map[*Request]bool)
	walk = func(r *Request, set map[string]bool, seen map[*Request]bool) {
		if seen[r] || r.Metadata == nil {
			return
		}
		seen[r] = true
		for _, d := range r.Metadata.Depends {
			set[d] = true
			if dr, ok := byName[d]; ok {
				walk(dr, set, seen)
			}
		}
	}
	for _, r := range f.Requests {
		set := map[string]bool{}
		walk(r, set, map[*Request]bool{})
		out[r] = set
	}
	return out
}

// findDependencyCycle returns "a -> b -> a" for the first @depends cycle, or "".
func findDependencyCycle(f *File, byName map[string]*Request) string {
	const (
		visiting = 1
		done     = 2
	)
	state := map[string]int{}
	var path []string
	var visit func(name string) string
	visit = func(name string) string {
		switch state[name] {
		case done:
			return ""
		case visiting:
			for i, n := range path {
				if n == name {
					return strings.Join(append(append([]string(nil), path[i:]...), name), " -> ")
				}
			}
			return name + " -> " + name
		}
		r, ok := byName[name]
		if !ok || r.Metadata == nil {
			return ""
		}
		state[name] = visiting
		path = append(path, name)
		for _, d := range r.Metadata.Depends {
			if c := visit(d); c != "" {
				return c
			}
		}
		path = path[:len(path)-1]
		state[name] = done
		return ""
	}
	for _, r := range f.Requests {
		if r.Name == "" {
			continue
		}
		if c := visit(r.Name); c != "" {
			return c
		}
	}
	return ""
}

// forEachTemplateText calls fn with every piece of request text that can hold
// a {{variable}}, with the line it came from.
func forEachTemplateText(r *Request, fn func(line int, text string)) {
	fn(r.Line, r.URL)
	for _, h := range r.Headers {
		fn(h.Line, h.Key)
		fn(h.Line, h.Value)
	}
	for _, q := range r.QueryParams {
		fn(q.Line, q.Key)
		fn(q.Line, q.Value)
	}
	if b := r.Body; b != nil {
		fn(b.Line, b.Raw)
		fn(b.Line, b.FilePath)
		for _, m := range b.Multipart {
			fn(b.Line, m.Value)
			fn(b.Line, m.Path)
		}
		if b.GraphQL != nil {
			fn(b.Line, b.GraphQL.Query)
			fn(b.Line, b.GraphQL.Variables)
		}
	}
	if m := r.Metadata; m != nil {
		if m.Auth != nil {
			for _, p := range m.Auth.Params {
				fn(r.Line, p)
			}
		}
		if m.Condition != nil {
			fn(r.Line, m.Condition.Expression)
		}
		fn(r.Line, m.DBConnection)
		if m.WaitFor != nil {
			fn(r.Line, m.WaitFor.URL)
		}
		for _, h := range m.PreHooks {
			fn(r.Line, h.Command)
		}
		for _, h := range m.PostHooks {
			fn(r.Line, h.Command)
		}
	}
	for _, a := range r.Assertions {
		forEachString(a.Expected, func(s string) { fn(a.Line, s) })
	}
	for _, d := range r.DBAssertions {
		fn(d.Line, d.Query)
		forEachString(d.Expected, func(s string) { fn(d.Line, s) })
	}
	for _, c := range r.ShellCommands {
		fn(c.Line, c.Command)
	}
}

func forEachString(v any, fn func(string)) {
	switch t := v.(type) {
	case string:
		fn(t)
	case []any:
		for _, e := range t {
			forEachString(e, fn)
		}
	case map[string]any:
		for _, e := range t {
			forEachString(e, fn)
		}
	case *EachSpec:
		forEachString(t.Expected, fn)
	}
}
