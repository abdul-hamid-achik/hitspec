package stress

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
)

// setGlobalNoColor sets color.NoColor for one test and restores it after. The
// tests that use it must not run in parallel.
func setGlobalNoColor(t *testing.T, v bool) {
	t.Helper()
	prev := color.NoColor
	color.NoColor = v
	t.Cleanup(func() { color.NoColor = prev })
}

func reporterOutput(noColor bool) string {
	var buf bytes.Buffer
	r := NewReporter(WithWriter(&buf), WithNoProgress(true), WithNoColor(noColor))
	r.Header("test", []string{"api.http"}, DefaultConfig())
	return buf.String()
}

// Without noColor the reporter follows the package's detection: output that
// is not a terminal (color.NoColor set at startup) gets no escape codes.
func TestReporterFollowsColorDetection(t *testing.T) {
	setGlobalNoColor(t, true)
	if out := reporterOutput(false); strings.Contains(out, "\x1b[") {
		t.Fatalf("colors were forced on for output that is not a terminal:\n%q", out)
	}
}

func TestReporterColorsOnATerminal(t *testing.T) {
	setGlobalNoColor(t, false)
	if out := reporterOutput(false); !strings.Contains(out, "\x1b[") {
		t.Fatalf("no colors on a terminal:\n%q", out)
	}
	if out := reporterOutput(true); strings.Contains(out, "\x1b[") {
		t.Fatalf("noColor still printed colors:\n%q", out)
	}
	if color.NoColor {
		t.Fatal("NewReporter changed the global color.NoColor")
	}
}
