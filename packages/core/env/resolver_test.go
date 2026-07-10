package env

import (
	"testing"
)

func TestResolverHasUnresolvedVariables(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		variables map[string]any
		expected  bool
	}{
		{
			name:      "no variables",
			input:     "hello world",
			variables: nil,
			expected:  false,
		},
		{
			name:      "resolved variable",
			input:     "{{foo}}",
			variables: map[string]any{"foo": "bar"},
			expected:  false,
		},
		{
			name:      "unresolved variable",
			input:     "{{foo}}",
			variables: nil,
			expected:  true,
		},
		{
			name:      "mixed resolved and unresolved",
			input:     "{{foo}} and {{bar}}",
			variables: map[string]any{"foo": "hello"},
			expected:  true,
		},
		{
			name:      "all resolved",
			input:     "{{foo}} and {{bar}}",
			variables: map[string]any{"foo": "hello", "bar": "world"},
			expected:  false,
		},
		{
			name:      "nested path unresolved",
			input:     "{{setupProject.projectId}}",
			variables: nil,
			expected:  true,
		},
		{
			name:      "nested path resolved via capture",
			input:     "{{setupProject.projectId}}",
			variables: nil,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResolver()
			if tt.variables != nil {
				r.SetVariables(tt.variables)
			}
			// Special case for nested path capture test
			if tt.name == "nested path resolved via capture" {
				r.SetCapture("setupProject", "projectId", "123")
			}

			got := r.HasUnresolvedVariables(tt.input)
			if got != tt.expected {
				t.Errorf("HasUnresolvedVariables(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestResolver_NestedVariables guards the regression where the resolver was
// single-pass: a variable whose value contained {{...}} left nested references
// literal ({{apiUrl}} resolved to "{{base}}/v1" instead of the full URL).
func TestResolver_NestedVariables(t *testing.T) {
	r := NewResolver()
	r.SetVariable("scheme", "https")
	r.SetVariable("base", "{{scheme}}://api.example.com")
	r.SetVariable("apiUrl", "{{base}}/v1")

	if got := r.Resolve("{{apiUrl}}"); got != "https://api.example.com/v1" {
		t.Errorf("Resolve({{apiUrl}}) = %q, want %q", got, "https://api.example.com/v1")
	}
	if got := r.Resolve("{{base}}"); got != "https://api.example.com" {
		t.Errorf("Resolve({{base}}) = %q, want %q", got, "https://api.example.com")
	}
}

// TestResolver_SelfReferentialNoInfiniteLoop guards against a self-referential
// variable (e.g. @baseUrl = {{baseUrl}}) recursing forever; it must resolve to
// the stable literal, not hang.
func TestResolver_SelfReferentialNoInfiniteLoop(t *testing.T) {
	r := NewResolver()
	r.SetVariable("baseUrl", "{{baseUrl}}")
	got := r.Resolve("{{baseUrl}}")
	if got != "{{baseUrl}}" {
		t.Errorf("self-referential {{baseUrl}} = %q, want the literal {{baseUrl}}", got)
	}
}

func TestResolverGetUnresolvedVariables(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		variables map[string]any
		expected  []string
	}{
		{
			name:      "no variables",
			input:     "hello world",
			variables: nil,
			expected:  nil,
		},
		{
			name:      "resolved variable",
			input:     "{{foo}}",
			variables: map[string]any{"foo": "bar"},
			expected:  nil,
		},
		{
			name:      "single unresolved variable",
			input:     "{{foo}}",
			variables: nil,
			expected:  []string{"foo"},
		},
		{
			name:      "multiple unresolved variables",
			input:     "{{foo}} and {{bar}}",
			variables: nil,
			expected:  []string{"foo", "bar"},
		},
		{
			name:      "mixed resolved and unresolved",
			input:     "{{foo}} and {{bar}} and {{baz}}",
			variables: map[string]any{"bar": "middle"},
			expected:  []string{"foo", "baz"},
		},
		{
			name:      "nested path unresolved",
			input:     "{{setupProject.projectId}}/tasks",
			variables: nil,
			expected:  []string{"setupProject.projectId"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResolver()
			if tt.variables != nil {
				r.SetVariables(tt.variables)
			}

			got := r.GetUnresolvedVariables(tt.input)

			if tt.expected == nil {
				if got != nil {
					t.Errorf("GetUnresolvedVariables(%q) = %v, want nil", tt.input, got)
				}
				return
			}

			if len(got) != len(tt.expected) {
				t.Errorf("GetUnresolvedVariables(%q) returned %d vars, want %d", tt.input, len(got), len(tt.expected))
				return
			}

			for i, v := range tt.expected {
				if got[i] != v {
					t.Errorf("GetUnresolvedVariables(%q)[%d] = %q, want %q", tt.input, i, got[i], v)
				}
			}
		})
	}
}

func TestResolverResolve(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		variables map[string]any
		captures  map[string]string
		expected  string
	}{
		{
			name:     "no variables",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:      "simple variable",
			input:     "hello {{name}}",
			variables: map[string]any{"name": "world"},
			expected:  "hello world",
		},
		{
			name:      "multiple variables",
			input:     "{{greeting}} {{name}}!",
			variables: map[string]any{"greeting": "Hello", "name": "World"},
			expected:  "Hello World!",
		},
		{
			name:     "capture variable",
			input:    "project {{projectId}}",
			captures: map[string]string{"projectId": "123"},
			expected: "project 123",
		},
		{
			name:     "namespaced capture",
			input:    "project {{setup.projectId}}",
			captures: map[string]string{"setup.projectId": "456"},
			expected: "project 456",
		},
		{
			name:     "unresolved stays as-is",
			input:    "hello {{unknown}}",
			expected: "hello {{unknown}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResolver()
			if tt.variables != nil {
				r.SetVariables(tt.variables)
			}
			for k, v := range tt.captures {
				// Use internal captures map directly since SetCapture expects requestName and captureName
				r.mu.Lock()
				r.captures[k] = v
				r.mu.Unlock()
			}

			got := r.Resolve(tt.input)
			if got != tt.expected {
				t.Errorf("Resolve(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestResolver_CloneIsIndependent verifies Clone produces a detached copy:
// mutating the clone (or the original) does not affect the other.
func TestResolver_CloneIsIndependent(t *testing.T) {
	r := NewResolver()
	r.SetVariable("shared", "original")
	r.SetCapture("req", "cap", "capval")

	clone := r.Clone()
	clone.SetVariable("onlyClone", "c")
	r.SetVariable("onlyOriginal", "o")

	if got := r.Resolve("{{shared}}"); got != "original" {
		t.Errorf("original shared = %q, want original", got)
	}
	if got := clone.Resolve("{{shared}}"); got != "original" {
		t.Errorf("clone shared = %q, want original", got)
	}
	// Variables added to one must not appear in the other.
	if r.HasVariable("onlyClone") {
		t.Error("original must not see clone-only variable")
	}
	if !clone.HasVariable("onlyClone") {
		t.Error("clone must see its own variable")
	}
	if !r.HasVariable("onlyOriginal") {
		t.Error("original must see its own variable")
	}
	if clone.HasVariable("onlyOriginal") {
		t.Error("clone must not see original-only variable")
	}
	// Capture must be cloned.
	if got, ok := clone.GetCapture("cap"); !ok || got != "capval" {
		t.Errorf("clone capture = %v ok=%v, want capval true", got, ok)
	}
}

// TestResolver_Precedence verifies capture > variable > dotenv ordering: when
// the same name is set at multiple levels, the highest-precedence value wins.
func TestResolver_Precedence(t *testing.T) {
	r := NewResolver()
	r.SetVariable("name", "fromVar")
	// A capture with the same name overrides the variable.
	r.SetCapture("req", "name", "fromCapture")
	if got := r.Resolve("{{name}}"); got != "fromCapture" {
		t.Errorf("capture should win over variable, got %q", got)
	}
}
