package output

import (
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
)

// JSONOutput represents the complete JSON output structure
type JSONOutput struct {
	Summary  JSONSummary  `json:"summary"`
	Tests    []JSONTest   `json:"tests"`
	Duration float64      `json:"duration"`
	Time     string       `json:"time"`
}

// JSONSummary represents the test summary
type JSONSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// JSONTest represents a single test result
type JSONTest struct {
	Name       string          `json:"name"`
	File       string          `json:"file"`
	Passed     bool            `json:"passed"`
	Skipped    bool            `json:"skipped,omitempty"`
	SkipReason string          `json:"skipReason,omitempty"`
	Duration   float64         `json:"duration"`
	Error      string          `json:"error,omitempty"`
	Request    *JSONRequest    `json:"request,omitempty"`
	Response   *JSONResponse   `json:"response,omitempty"`
	Assertions []JSONAssertion `json:"assertions,omitempty"`
	Captures   map[string]any  `json:"captures,omitempty"`
}

// JSONRequest represents request details
type JSONRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// JSONResponse represents response details
type JSONResponse struct {
	StatusCode int               `json:"statusCode"`
	Status     string            `json:"status"`
	Headers    map[string]string `json:"headers,omitempty"`
	Duration   float64           `json:"duration"`
}

// JSONAssertion represents an assertion result
type JSONAssertion struct {
	Subject  string `json:"subject"`
	Operator string `json:"operator"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message,omitempty"`
}

// JSONFormatter formats test results as JSON
type JSONFormatter struct {
	writer  io.Writer
	results []JSONTest
}

type JSONOption func(*JSONFormatter)

func NewJSONFormatter(opts ...JSONOption) *JSONFormatter {
	f := &JSONFormatter{
		writer:  os.Stdout,
		results: make([]JSONTest, 0),
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func JSONWithWriter(w io.Writer) JSONOption {
	return func(f *JSONFormatter) {
		f.writer = w
	}
}

func (f *JSONFormatter) FormatResult(result *runner.RunResult) {
	for _, r := range result.Results {
		test := JSONTest{
			Name:     r.Name,
			File:     result.File,
			Passed:   r.Passed,
			Skipped:  r.Skipped,
			Duration: float64(r.Duration.Milliseconds()),
		}

		if r.SkipReason != "" && r.SkipReason != "filtered out" {
			test.SkipReason = r.SkipReason
		}

		if r.Error != nil {
			test.Error = r.Error.Error()
		}

		if r.Request != nil {
			test.Request = &JSONRequest{
				Method:  r.Request.Method,
				URL:     r.Request.URL,
				Headers: r.Request.Headers,
			}
		}

		if r.Response != nil {
			test.Response = &JSONResponse{
				StatusCode: r.Response.StatusCode,
				Status:     r.Response.Status,
				Headers:    r.Response.Headers,
				Duration:   float64(r.Response.Duration.Milliseconds()),
			}
		}

		if len(r.Assertions) > 0 {
			test.Assertions = make([]JSONAssertion, len(r.Assertions))
			for i, a := range r.Assertions {
				test.Assertions[i] = JSONAssertion{
					Subject:  a.Subject,
					Operator: a.Operator,
					Expected: a.Expected,
					Actual:   a.Actual,
					Passed:   a.Passed,
					Message:  a.Message,
				}
			}
		}

		if len(r.Captures) > 0 {
			test.Captures = r.Captures
		}

		f.results = append(f.results, test)
	}
}

func (f *JSONFormatter) FormatError(err error) {
	// Errors are included in individual test results
}

func (f *JSONFormatter) FormatHeader(version string) {
	// No header needed for JSON output
}

// Flush writes the accumulated JSON output
func (f *JSONFormatter) Flush(totalDuration time.Duration) error {
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

	output := JSONOutput{
		Summary: JSONSummary{
			Total:   len(f.results),
			Passed:  passed,
			Failed:  failed,
			Skipped: skipped,
		},
		Tests:    f.results,
		Duration: float64(totalDuration.Milliseconds()),
		Time:     time.Now().Format(time.RFC3339),
	}

	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
