package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/assertions"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
)

// TestHTMLFormatter covers the HTML formatter's FormatResult + Flush path (a
// test-gap area): passing, failing, and skipped results render into a valid
// HTML report with the right summary counts and per-request detail.
func TestHTMLFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := NewHTMLFormatter(HTMLWithWriter(&buf))
	f.FormatHeader("1.2.3")

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{Name: "GET /users", Description: "List users", Passed: true, Duration: 12 * time.Millisecond},
		{Name: "POST /login", Passed: false, Duration: 5 * time.Millisecond,
			Assertions: []*assertions.Result{
				{Subject: "status", Operator: "==", Expected: 200, Actual: 500, Passed: false, Message: "expected 200, got 500"},
			}},
		{Name: "Optional", Skipped: true, SkipReason: "filtered out"},
	}))

	if err := f.Flush(100 * time.Millisecond); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	out := buf.String()
	if !strings.HasPrefix(strings.TrimSpace(out), "<!DOCTYPE html>") {
		t.Fatalf("HTML output should start with <!DOCTYPE html>, got: %q", out[:min(len(out), 40)])
	}
	for _, want := range []string{"hitspec", "GET /users", "POST /login", "Optional", "1.2.3"} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
	if !strings.Contains(out, "expected 200, got 500") {
		t.Error("HTML output should include the failing assertion message")
	}
	// The summary renders counts via the template; assert the report carries the
	// passed/failed/skipped summary cards (CSS classes) rather than a specific
	// "N passed" literal, which the template formats differently.
	for _, cls := range []string{"summary-card passed", "summary-card failed", "summary-card skipped"} {
		if !strings.Contains(out, cls) {
			t.Errorf("HTML output missing summary card %q", cls)
		}
	}
}

// TestConsoleFormatter_FormatResult covers the console FormatResult path for a
// mixed result set (pass/fail/skip) — a test-gap area.
func TestConsoleFormatter_FormatResult(t *testing.T) {
	var buf bytes.Buffer
	f := NewConsoleFormatter(WithWriter(&buf), WithNoColor(true))
	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{Name: "ok", Passed: true, Duration: time.Millisecond},
		{Name: "bad", Passed: false, Duration: time.Millisecond,
			Assertions: []*assertions.Result{
				{Subject: "status", Operator: parser.OpEquals.String(), Expected: 200, Actual: 404, Passed: false},
			}},
		{Name: "sk", Skipped: true, SkipReason: "dependency failed"},
	}))
	out := buf.String()
	for _, want := range []string{"ok", "bad", "sk", "Tests:", "1 passed", "1 failed", "1 skipped"} {
		if !strings.Contains(out, want) {
			t.Errorf("console output missing %q", out)
		}
	}
}
