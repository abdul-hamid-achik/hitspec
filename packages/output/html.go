package output

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
)

// HTMLOutput represents the complete HTML output structure
type HTMLOutput struct {
	Version        string
	Summary        HTMLSummary
	Tests          []HTMLTest
	Duration       float64
	Time           string
	PassedPercent  float64
	FailedPercent  float64
	SkippedPercent float64
}

// HTMLSummary represents the test summary for HTML output
type HTMLSummary struct {
	Total   int
	Passed  int
	Failed  int
	Skipped int
}

// HTMLTest represents a single test result for HTML output
type HTMLTest struct {
	Name        string
	File        string
	Passed      bool
	Skipped     bool
	SkipReason  string
	Duration    float64
	Error       string
	StatusClass string
	Request     *HTMLRequest
	Response    *HTMLResponse
	Assertions  []HTMLAssertion
	Captures    map[string]any
}

// HTMLRequest represents request details for HTML output
type HTMLRequest struct {
	Method  string
	URL     string
	Headers map[string]string
}

// HTMLResponse represents response details for HTML output
type HTMLResponse struct {
	StatusCode int
	Status     string
	Headers    map[string]string
	Duration   float64
}

// HTMLAssertion represents an assertion result for HTML output
type HTMLAssertion struct {
	Subject     string
	Operator    string
	Expected    any
	Actual      any
	ExpectedStr string
	ActualStr   string
	Passed      bool
	Message     string
}

// HTMLFormatter formats test results as HTML
type HTMLFormatter struct {
	writer  io.Writer
	results []HTMLTest
	version string
}

// HTMLOption is a functional option for HTMLFormatter
type HTMLOption func(*HTMLFormatter)

// NewHTMLFormatter creates a new HTML formatter
func NewHTMLFormatter(opts ...HTMLOption) *HTMLFormatter {
	f := &HTMLFormatter{
		writer:  os.Stdout,
		results: make([]HTMLTest, 0),
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// HTMLWithWriter sets the output writer
func HTMLWithWriter(w io.Writer) HTMLOption {
	return func(f *HTMLFormatter) {
		f.writer = w
	}
}

// FormatResult accumulates a test result
func (f *HTMLFormatter) FormatResult(result *runner.RunResult) {
	for _, r := range result.Results {
		test := HTMLTest{
			Name:     r.Name,
			File:     result.File,
			Passed:   r.Passed,
			Skipped:  r.Skipped,
			Duration: float64(r.Duration.Milliseconds()),
			Captures: r.Captures,
		}

		// Set status class for CSS
		if r.Skipped {
			test.StatusClass = "skipped"
		} else if r.Passed {
			test.StatusClass = "passed"
		} else {
			test.StatusClass = "failed"
		}

		if r.SkipReason != "" && r.SkipReason != "filtered out" {
			test.SkipReason = r.SkipReason
		}

		if r.Error != nil {
			test.Error = r.Error.Error()
		}

		if r.Request != nil {
			test.Request = &HTMLRequest{
				Method:  r.Request.Method,
				URL:     r.Request.URL,
				Headers: r.Request.Headers,
			}
		}

		if r.Response != nil {
			test.Response = &HTMLResponse{
				StatusCode: r.Response.StatusCode,
				Status:     r.Response.Status,
				Headers:    r.Response.Headers,
				Duration:   float64(r.Response.Duration.Milliseconds()),
			}
		}

		if len(r.Assertions) > 0 {
			test.Assertions = make([]HTMLAssertion, len(r.Assertions))
			for i, a := range r.Assertions {
				test.Assertions[i] = HTMLAssertion{
					Subject:     a.Subject,
					Operator:    a.Operator,
					Expected:    a.Expected,
					Actual:      a.Actual,
					ExpectedStr: fmt.Sprintf("%v", a.Expected),
					ActualStr:   fmt.Sprintf("%v", a.Actual),
					Passed:      a.Passed,
					Message:     a.Message,
				}
			}
		}

		f.results = append(f.results, test)
	}
}

// FormatError handles errors (no-op for HTML, errors are in test results)
func (f *HTMLFormatter) FormatError(err error) {
	// Errors are included in individual test results
}

// FormatHeader captures the version for the HTML report
func (f *HTMLFormatter) FormatHeader(version string) {
	f.version = version
}

// Flush writes the accumulated HTML output
func (f *HTMLFormatter) Flush(totalDuration time.Duration) error {
	var passed, failed, skipped int
	for _, t := range f.results {
		if t.Skipped {
			skipped++
		} else if t.Passed {
			passed++
		} else {
			failed++
		}
	}

	total := len(f.results)
	var passedPct, failedPct, skippedPct float64
	if total > 0 {
		passedPct = float64(passed) / float64(total) * 100
		failedPct = float64(failed) / float64(total) * 100
		skippedPct = float64(skipped) / float64(total) * 100
	}

	output := HTMLOutput{
		Version: f.version,
		Summary: HTMLSummary{
			Total:   total,
			Passed:  passed,
			Failed:  failed,
			Skipped: skipped,
		},
		Tests:          f.results,
		Duration:       float64(totalDuration.Milliseconds()),
		Time:           time.Now().Format("2006-01-02 15:04:05"),
		PassedPercent:  passedPct,
		FailedPercent:  failedPct,
		SkippedPercent: skippedPct,
	}

	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}

	return tmpl.Execute(f.writer, output)
}
