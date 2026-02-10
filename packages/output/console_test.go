package output

import (
	"testing"
)

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		maxLen int
		want   string
	}{
		{"string", "hello", 100, "hello"},
		{"int", 42, 100, "42"},
		{"truncated string", "abcdefghij", 5, "abcde..."},
		{"array", []any{1, 2, 3}, 100, "[array with 3 items]"},
		{"object", map[string]any{"a": 1, "b": 2}, 100, "{object with 2 keys}"},
		{"string map", map[string]string{"a": "b"}, 100, "{map with 1 entries}"},
		{"headers map", map[string][]string{"Accept": {"text/html"}}, 100, "{headers with 1 entries}"},
		{"nil", nil, 100, "<nil>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatValue(tt.value, tt.maxLen)
			if got != tt.want {
				t.Errorf("formatValue(%v, %d) = %q, want %q", tt.value, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestComputeJSONDiff(t *testing.T) {
	tests := []struct {
		name     string
		expected any
		actual   any
		wantLen  int
	}{
		{
			name:     "identical",
			expected: map[string]any{"a": float64(1)},
			actual:   map[string]any{"a": float64(1)},
			wantLen:  0,
		},
		{
			name:     "changed value",
			expected: map[string]any{"a": float64(1)},
			actual:   map[string]any{"a": float64(2)},
			wantLen:  1,
		},
		{
			name:     "added key",
			expected: map[string]any{"a": float64(1)},
			actual:   map[string]any{"a": float64(1), "b": float64(2)},
			wantLen:  1,
		},
		{
			name:     "removed key",
			expected: map[string]any{"a": float64(1), "b": float64(2)},
			actual:   map[string]any{"a": float64(1)},
			wantLen:  1,
		},
		{
			name:     "both nil",
			expected: nil,
			actual:   nil,
			wantLen:  0,
		},
		{
			name:     "expected nil",
			expected: nil,
			actual:   "value",
			wantLen:  1,
		},
		{
			name:     "actual nil",
			expected: "value",
			actual:   nil,
			wantLen:  1,
		},
		{
			name:     "type mismatch",
			expected: "string",
			actual:   float64(42),
			wantLen:  1,
		},
		{
			name:     "array diff length",
			expected: []any{float64(1), float64(2)},
			actual:   []any{float64(1), float64(2), float64(3)},
			wantLen:  1,
		},
		{
			name:     "array element changed",
			expected: []any{float64(1), float64(2)},
			actual:   []any{float64(1), float64(99)},
			wantLen:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs := computeJSONDiff(tt.expected, tt.actual, "")
			if len(diffs) != tt.wantLen {
				t.Errorf("got %d diffs, want %d: %+v", len(diffs), tt.wantLen, diffs)
			}
		})
	}
}

func TestComputeJSONDiff_Types(t *testing.T) {
	// Test diff types are set correctly
	diffs := computeJSONDiff(nil, "added", "")
	if len(diffs) != 1 || diffs[0].Type != DiffTypeAdded {
		t.Errorf("expected DiffTypeAdded, got %+v", diffs)
	}

	diffs = computeJSONDiff("removed", nil, "")
	if len(diffs) != 1 || diffs[0].Type != DiffTypeRemoved {
		t.Errorf("expected DiffTypeRemoved, got %+v", diffs)
	}

	diffs = computeJSONDiff("old", "new", "")
	if len(diffs) != 1 || diffs[0].Type != DiffTypeChanged {
		t.Errorf("expected DiffTypeChanged, got %+v", diffs)
	}
}

func TestParseToJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantNil bool
	}{
		{"nil input", nil, true},
		{"map input", map[string]any{"a": 1}, false},
		{"slice input", []any{1, 2}, false},
		{"valid JSON string", `{"key":"val"}`, false},
		{"invalid JSON string", "not json", true},
		{"int", 42, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseToJSON(tt.input)
			if tt.wantNil && got != nil {
				t.Errorf("expected nil, got %v", got)
			}
			if !tt.wantNil && got == nil {
				t.Error("expected non-nil, got nil")
			}
		})
	}
}
