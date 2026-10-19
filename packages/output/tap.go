package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
)

// TAPFormatter formats test results in TAP (Test Anything Protocol) format
type TAPFormatter struct {
	writer    io.Writer
	testCount int
	results   []tapResult
}

type tapResult struct {
	number      int
	name        string
	description string
	passed      bool
	skipped     bool
	skipReason  string
	error       string
	assertions  []string
	request     string // "GET url" of a failed request
	status      string // response status line of a failed request
	millis      int64
	body        bodyText
	hasBody     bool
	warnings    []string
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
			number:      f.testCount,
			name:        r.Name,
			description: r.Description,
			passed:      r.Passed,
			skipped:     r.Skipped,
			skipReason:  r.SkipReason,
		}

		if r.Error != nil {
			tr.error = RedactText(r.Error.Error())
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

		tr.warnings = r.Warnings
		if !r.Skipped && !r.Passed {
			tr.request = requestLine(r)
			if r.Response != nil {
				tr.status = responseStatus(r)
				tr.millis = responseMillis(r)
				tr.body = formatBody(r.Response.Body, true)
				tr.hasBody = true
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
			f.writeContext(r)
			fmt.Fprintf(f.writer, "  ...\n")
			continue
		}

		if r.passed {
			fmt.Fprintf(f.writer, "ok %d - %s\n", r.number, r.name)
			if r.description != "" || len(r.warnings) > 0 {
				fmt.Fprintf(f.writer, "  ---\n")
				if r.description != "" {
					fmt.Fprintf(f.writer, "  description: %s\n", escapeYAML(r.description))
				}
				f.writeContext(r)
				fmt.Fprintf(f.writer, "  ...\n")
			}
		} else {
			fmt.Fprintf(f.writer, "not ok %d - %s\n", r.number, r.name)
			if len(r.assertions) > 0 || r.request != "" || len(r.warnings) > 0 {
				fmt.Fprintf(f.writer, "  ---\n")
				if len(r.assertions) > 0 {
					fmt.Fprintf(f.writer, "  failures:\n")
					for _, a := range r.assertions {
						fmt.Fprintf(f.writer, "    - %s\n", escapeYAML(a))
					}
				}
				f.writeContext(r)
				fmt.Fprintf(f.writer, "  ...\n")
			}
		}
	}

	// Add final newline for proper TAP output
	fmt.Fprintln(f.writer)

	return nil
}

// writeContext writes the YAMLish diagnostics of a failed request (request
// line, response status and body excerpt) and any warnings. Strings go through
// yamlString; the body is a literal block, so multi-line JSON stays readable.
func (f *TAPFormatter) writeContext(r tapResult) {
	if r.request != "" {
		fmt.Fprintf(f.writer, "  request: %s\n", yamlString(r.request))
	}
	if r.status != "" {
		fmt.Fprintf(f.writer, "  response:\n")
		fmt.Fprintf(f.writer, "    status: %s\n", yamlString(r.status))
		fmt.Fprintf(f.writer, "    durationMs: %d\n", r.millis)
		if r.hasBody && r.body.Total > 0 {
			// |2- : literal block, content indented two spaces past "body:",
			// no trailing newline; the indicator keeps a leading-space first
			// line from being misread.
			fmt.Fprintf(f.writer, "    body: |2-\n")
			for _, line := range strings.Split(sanitize(r.body.Text), "\n") {
				if line == "" {
					fmt.Fprintf(f.writer, "\n")
					continue
				}
				fmt.Fprintf(f.writer, "      %s\n", line)
			}
			if r.body.Truncated {
				fmt.Fprintf(f.writer, "    bodyTruncated: %s\n", yamlString(r.body.Note))
			}
		}
	}
	if len(r.warnings) > 0 {
		fmt.Fprintf(f.writer, "  warnings:\n")
		for _, w := range r.warnings {
			fmt.Fprintf(f.writer, "    - %s\n", yamlString(sanitize(w)))
		}
	}
}

// yamlString quotes s as a YAML double-quoted scalar (JSON string syntax is a
// subset of it).
func yamlString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return strconv.Quote(s)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// escapeYAML renders s as a YAML scalar: plain when that is safe, otherwise a
// double-quoted string with backslashes, quotes, newlines and control
// characters escaped, so a multi-line message cannot break out of the YAMLish
// diagnostic block.
func escapeYAML(s string) string {
	if s == "" || strings.ContainsAny(s, ":\n\r\t\\\"'[]{}#&*!|>%@`") || strings.TrimSpace(s) != s || hasControl(s) {
		return yamlString(s)
	}
	return s
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
