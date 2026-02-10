package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/assertions"
	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
)

func TestTAPFormatter_PassingTest(t *testing.T) {
	var buf bytes.Buffer
	f := NewTAPFormatter(TAPWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{
			Name:     "GET /users",
			Passed:   true,
			Duration: 42 * time.Millisecond,
		},
	}))

	if err := f.Flush(100 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "TAP version 13") {
		t.Error("missing TAP version header")
	}
	if !strings.Contains(out, "1..1") {
		t.Error("missing test plan")
	}
	if !strings.Contains(out, "ok 1 - GET /users") {
		t.Errorf("missing passing test line, got:\n%s", out)
	}
}

func TestTAPFormatter_FailingTest(t *testing.T) {
	var buf bytes.Buffer
	f := NewTAPFormatter(TAPWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{
			Name:     "POST /login",
			Passed:   false,
			Duration: 10 * time.Millisecond,
			Assertions: []*assertions.Result{
				{
					Subject:  "status",
					Operator: "equals",
					Expected: 200,
					Actual:   401,
					Passed:   false,
				},
			},
		},
	}))

	if err := f.Flush(50 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "not ok 1 - POST /login") {
		t.Errorf("missing failure line, got:\n%s", out)
	}
	if !strings.Contains(out, "failures:") {
		t.Error("missing failures YAML block")
	}
	if !strings.Contains(out, "status equals") {
		t.Errorf("missing assertion details, got:\n%s", out)
	}
}

func TestTAPFormatter_SkippedTest(t *testing.T) {
	var buf bytes.Buffer
	f := NewTAPFormatter(TAPWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{
			Name:       "Conditional",
			Skipped:    true,
			SkipReason: "@unless env == prod",
		},
	}))

	if err := f.Flush(10 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "ok 1 - Conditional # SKIP @unless env == prod") {
		t.Errorf("missing skip directive, got:\n%s", out)
	}
}

func TestTAPFormatter_SkippedFilteredOut(t *testing.T) {
	var buf bytes.Buffer
	f := NewTAPFormatter(TAPWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{
			Name:       "Filtered",
			Skipped:    true,
			SkipReason: "filtered out",
		},
	}))

	if err := f.Flush(10 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	// "filtered out" maps to generic SKIP
	if !strings.Contains(out, "# SKIP SKIP") {
		t.Errorf("expected generic SKIP for 'filtered out', got:\n%s", out)
	}
}

func TestTAPFormatter_ErrorTest(t *testing.T) {
	var buf bytes.Buffer
	f := NewTAPFormatter(TAPWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{
			Name:   "Timeout",
			Passed: false,
			Error:  errors.New("connection refused"),
		},
	}))

	if err := f.Flush(10 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "not ok 1 - Timeout") {
		t.Errorf("missing error line, got:\n%s", out)
	}
	if !strings.Contains(out, "message:") {
		t.Error("missing error message YAML")
	}
	if !strings.Contains(out, "connection refused") {
		t.Errorf("missing error detail, got:\n%s", out)
	}
}

func TestTAPFormatter_Description(t *testing.T) {
	var buf bytes.Buffer
	f := NewTAPFormatter(TAPWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{
			Name:        "GET /health",
			Description: "Health check endpoint",
			Passed:      true,
			Duration:    time.Millisecond,
		},
	}))

	if err := f.Flush(10 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "description:") {
		t.Errorf("missing description YAML block, got:\n%s", out)
	}
	if !strings.Contains(out, "Health check endpoint") {
		t.Errorf("missing description text, got:\n%s", out)
	}
}

func TestTAPFormatter_MultipleTests(t *testing.T) {
	var buf bytes.Buffer
	f := NewTAPFormatter(TAPWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{Name: "Test A", Passed: true, Duration: time.Millisecond},
		{Name: "Test B", Passed: false, Duration: time.Millisecond},
		{Name: "Test C", Skipped: true},
	}))

	if err := f.Flush(100 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "1..3") {
		t.Errorf("test plan should be 1..3, got:\n%s", out)
	}
	if !strings.Contains(out, "ok 1 - Test A") {
		t.Error("missing Test A")
	}
	if !strings.Contains(out, "not ok 2 - Test B") {
		t.Error("missing Test B")
	}
	if !strings.Contains(out, "ok 3 - Test C # SKIP") {
		t.Error("missing Test C skip")
	}
}

func TestEscapeYAML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple text", "simple text"},
		{"has: colon", `"has: colon"`},
		{`has "quotes"`, `"has \"quotes\""`},
		{"has\nnewline", "\"has\nnewline\""},
	}
	for _, tt := range tests {
		got := escapeYAML(tt.input)
		if got != tt.want {
			t.Errorf("escapeYAML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
