package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
)

// runTotals is the accounting for one pass over all files of a `run`.
type runTotals struct {
	Passed  int
	Failed  int
	Skipped int
	// BrokenFiles counts files that could not be run at all (syntax error,
	// duplicate @name, @depends cycle). They are not requests, so they are not
	// part of Passed/Failed/Skipped, but they fail the run with ExitParseError.
	BrokenFiles int
	// Matched counts the requests that satisfied --name/--tags across all files.
	Matched  int
	Duration time.Duration
}

// openReport returns the formatter for one run and a function that closes its
// destination. The destination is:
//
//   - the --output-file, created (truncated) afresh on every call, so watch
//     re-runs never append a second document to the first;
//   - io.Discard under --quiet without --output-file, so the report is dropped
//     while the formatter code path stays identical;
//   - the formatter's default (stdout) otherwise.
//
// The writer must stay a nil interface (not a nil *os.File) for the stdout
// case, or every formatter would write to a nil file instead of stdout.
func openReport(format, outputFile string, verbose int, noColor, quiet bool) (Formatter, func() error, error) {
	var w io.Writer
	closeFn := func() error { return nil }
	switch {
	case outputFile != "":
		f, err := os.Create(outputFile)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot create output file: %w", err)
		}
		w = f
		closeFn = f.Close
	case quiet:
		w = io.Discard
	}
	return newFormatter(format, w, verbose, noColor, quiet), closeFn, nil
}

// brokenFileMessage names the file in an error from RunFile. Parse errors
// already carry the path; duplicate-name and dependency-cycle errors do not.
func brokenFileMessage(file string, err error) string {
	if strings.Contains(err.Error(), file) {
		return err.Error()
	}
	return file + ": " + err.Error()
}

// brokenFileResult represents a file that could not be run as a failed result
// so structured formats (json, junit, tap, html) show it instead of silently
// omitting it.
func brokenFileResult(file string, err error) *runner.RunResult {
	return &runner.RunResult{
		File: file,
		Results: []*runner.RequestResult{{
			Name:  "file error",
			Error: errors.New(brokenFileMessage(file, err)),
		}},
	}
}

// brokenFilesSummary is the final line counting files that failed to load.
func brokenFilesSummary(n int) string {
	if n == 1 {
		return "1 file failed to parse"
	}
	return fmt.Sprintf("%d files failed to parse", n)
}

// noMatchMessage describes filters that selected no request, or "" when no
// filter is set.
func noMatchMessage(name string, tags []string) string {
	var parts []string
	if name != "" {
		parts = append(parts, fmt.Sprintf("--name %q", name))
	}
	if len(tags) > 0 {
		parts = append(parts, fmt.Sprintf("--tags %q", strings.Join(tags, ",")))
	}
	if len(parts) == 0 {
		return ""
	}
	return "no requests matched " + strings.Join(parts, " ")
}

// exitErrorFor maps the totals of a finished run to its exit status.
// Precedence: parse errors (2) > request failures (1) > filters that matched
// nothing (64) > success. A broken file wins over "no match" because the
// filter may well have matched inside the file that could not be loaded.
// Messages were already written to stderr, so the error text is empty.
func exitErrorFor(t runTotals, filtersSet bool) error {
	switch {
	case t.BrokenFiles > 0:
		return newExitError(ExitParseError, "")
	case t.Failed > 0:
		return newExitError(ExitTestFailure, "")
	case filtersSet && t.Matched == 0:
		return newExitError(ExitUsageError, "")
	}
	return nil
}

// failureReason is the one-line cause of a failed request: the error, the
// first failing assertion, or (for requests without assertions) the status.
func failureReason(rr *runner.RequestResult) string {
	if rr.Error != nil {
		return rr.Error.Error()
	}
	for _, a := range rr.Assertions {
		if a.Passed {
			continue
		}
		msg := fmt.Sprintf("%s %s: expected %v, got %v", a.Subject, a.Operator, a.Expected, a.Actual)
		if a.Message != "" {
			msg += " (" + a.Message + ")"
		}
		return msg
	}
	for _, d := range rr.DBAssertions {
		if !d.Passed {
			msg := "database assertion failed: " + d.Query
			if d.Message != "" {
				msg += " (" + d.Message + ")"
			}
			return msg
		}
	}
	for _, s := range rr.ShellResults {
		if !s.Passed {
			return "shell command failed: " + s.Command
		}
	}
	if rr.Response != nil {
		return fmt.Sprintf("unexpected status %d", rr.Response.StatusCode)
	}
	return "failed"
}

// writeFailureList prints one line per failed request of a file; --quiet uses
// it so a silent run still says what broke.
func writeFailureList(w io.Writer, res *runner.RunResult) {
	for _, rr := range res.Results {
		if rr.Passed || rr.Skipped {
			continue
		}
		name := rr.Name
		if name == "" {
			name = "(unnamed request)"
		}
		fmt.Fprintf(w, "FAIL %s: %s: %s\n", res.File, name, failureReason(rr))
	}
}
