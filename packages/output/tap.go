package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
)

// TAPFormatter formats test results in TAP (Test Anything Protocol) format
type TAPFormatter struct {
	writer    io.Writer
	testCount int
	results   []tapResult
}

type tapResult struct {
	number     int
	name       string
	passed     bool
	skipped    bool
	skipReason string
	error      string
	assertions []string
}

type TAPOption func(*TAPFormatter)

func NewTAPFormatter(opts ...TAPOption) *TAPFormatter {
	f := &TAPFormatter{
		writer:  os.Stdout,
		results: make([]tapResult, 0),
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func TAPWithWriter(w io.Writer) TAPOption {
	return func(f *TAPFormatter) {
		f.writer = w
	}
}

func (f *TAPFormatter) FormatResult(result *runner.RunResult) {
	for _, r := range result.Results {
		f.testCount++
		tr := tapResult{
			number:     f.testCount,
			name:       r.Name,
			passed:     r.Passed,
			skipped:    r.Skipped,
			skipReason: r.SkipReason,
		}

		if r.Error != nil {
			tr.error = r.Error.Error()
		}

		if !r.Passed && len(r.Assertions) > 0 {
			for _, a := range r.Assertions {
				if !a.Passed {
					tr.assertions = append(tr.assertions, fmt.Sprintf(
						"%s %s: expected %v, got %v",
						a.Subject, a.Operator, a.Expected, a.Actual))
				}
			}
		}

		f.results = append(f.results, tr)
	}
}

func (f *TAPFormatter) FormatError(err error) {
	// Errors are included in individual test results
}

func (f *TAPFormatter) FormatHeader(version string) {
	// Header is written in Flush
}

// Flush writes the accumulated TAP output
func (f *TAPFormatter) Flush(totalDuration time.Duration) error {
	// TAP version header
	fmt.Fprintf(f.writer, "TAP version 13\n")

	// Test plan
	fmt.Fprintf(f.writer, "1..%d\n", f.testCount)

	// Individual test results
	for _, r := range f.results {
		if r.skipped {
			reason := r.skipReason
			if reason == "" || reason == "filtered out" {
				reason = "SKIP"
			}
			fmt.Fprintf(f.writer, "ok %d - %s # SKIP %s\n", r.number, r.name, reason)
			continue
		}

		if r.error != "" {
			fmt.Fprintf(f.writer, "not ok %d - %s\n", r.number, r.name)
			fmt.Fprintf(f.writer, "  ---\n")
			fmt.Fprintf(f.writer, "  message: %s\n", escapeYAML(r.error))
			fmt.Fprintf(f.writer, "  severity: error\n")
			fmt.Fprintf(f.writer, "  ...\n")
			continue
		}

		if r.passed {
			fmt.Fprintf(f.writer, "ok %d - %s\n", r.number, r.name)
		} else {
			fmt.Fprintf(f.writer, "not ok %d - %s\n", r.number, r.name)
			if len(r.assertions) > 0 {
				fmt.Fprintf(f.writer, "  ---\n")
				fmt.Fprintf(f.writer, "  failures:\n")
				for _, a := range r.assertions {
					fmt.Fprintf(f.writer, "    - %s\n", escapeYAML(a))
				}
				fmt.Fprintf(f.writer, "  ...\n")
			}
		}
	}

	// Add final newline for proper TAP output
	fmt.Fprintln(f.writer)

	return nil
}

func escapeYAML(s string) string {
	// Simple YAML escaping - wrap in quotes if contains special chars
	if strings.ContainsAny(s, ":\n\"'[]{}#&*!|>%@`") {
		s = strings.ReplaceAll(s, "\"", "\\\"")
		return "\"" + s + "\""
	}
	return s
}
