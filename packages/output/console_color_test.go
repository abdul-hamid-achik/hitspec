package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
	"github.com/fatih/color"
)

func consoleOutput(noColor bool) string {
	var buf bytes.Buffer
	f := NewConsoleFormatter(WithWriter(&buf), WithNoColor(noColor))
	f.FormatHeader("test")
	f.FormatResult(&runner.RunResult{File: "api.http", Results: []*runner.RequestResult{{Name: "ok", Passed: true}}})
	return buf.String()
}

// WithNoColor applies to its own formatter. It used to set the global
// color.NoColor, which raced with other formatters and kept colors off for
// every later formatter in the process (a watch pass that turned noColor
// back off).
func TestConsoleNoColorIsPerFormatter(t *testing.T) {
	prev := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = prev })

	if out := consoleOutput(true); strings.Contains(out, "\x1b[") {
		t.Fatalf("noColor still printed colors:\n%q", out)
	}
	if color.NoColor {
		t.Fatal("NewConsoleFormatter changed the global color.NoColor")
	}
	if out := consoleOutput(false); !strings.Contains(out, "\x1b[") {
		t.Fatalf("a formatter with colors got none after a noColor one:\n%q", out)
	}
}

func TestConsoleFollowsColorDetection(t *testing.T) {
	prev := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = prev })

	if out := consoleOutput(false); strings.Contains(out, "\x1b[") {
		t.Fatalf("colors on output that is not a terminal:\n%q", out)
	}
}
