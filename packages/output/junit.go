package output

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
)

// JUnit XML structures

// JUnitTestSuites is the root element
type JUnitTestSuites struct {
	XMLName    xml.Name         `xml:"testsuites"`
	Name       string           `xml:"name,attr,omitempty"`
	Tests      int              `xml:"tests,attr"`
	Failures   int              `xml:"failures,attr"`
	Errors     int              `xml:"errors,attr"`
	Skipped    int              `xml:"skipped,attr"`
	Time       float64          `xml:"time,attr"`
	Timestamp  string           `xml:"timestamp,attr,omitempty"`
	TestSuites []JUnitTestSuite `xml:"testsuite"`
}

// JUnitTestSuite represents a test suite (typically a file)
type JUnitTestSuite struct {
	XMLName   xml.Name        `xml:"testsuite"`
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Skipped   int             `xml:"skipped,attr"`
	Time      float64         `xml:"time,attr"`
	Timestamp string          `xml:"timestamp,attr,omitempty"`
	TestCases []JUnitTestCase `xml:"testcase"`
}

// JUnitTestCase represents a single test case
type JUnitTestCase struct {
	XMLName   xml.Name      `xml:"testcase"`
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
	Error     *JUnitError   `xml:"error,omitempty"`
	Skipped   *JUnitSkipped `xml:"skipped,omitempty"`
}

// JUnitFailure represents a test failure
type JUnitFailure struct {
	Message string `xml:"message,attr,omitempty"`
	Type    string `xml:"type,attr,omitempty"`
	Content string `xml:",chardata"`
}

// JUnitError represents a test error
type JUnitError struct {
	Message string `xml:"message,attr,omitempty"`
	Type    string `xml:"type,attr,omitempty"`
	Content string `xml:",chardata"`
}

// JUnitSkipped represents a skipped test
type JUnitSkipped struct {
	Message string `xml:"message,attr,omitempty"`
}

// JUnitFormatter formats test results as JUnit XML
type JUnitFormatter struct {
	writer     io.Writer
	testSuites []JUnitTestSuite
}

type JUnitOption func(*JUnitFormatter)

func NewJUnitFormatter(opts ...JUnitOption) *JUnitFormatter {
	f := &JUnitFormatter{
		writer:     os.Stdout,
		testSuites: make([]JUnitTestSuite, 0),
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func JUnitWithWriter(w io.Writer) JUnitOption {
	return func(f *JUnitFormatter) {
		f.writer = w
	}
}

func (f *JUnitFormatter) FormatResult(result *runner.RunResult) {
	suite := JUnitTestSuite{
		Name:      result.File,
		Tests:     len(result.Results),
		Failures:  result.Failed,
		Skipped:   result.Skipped,
		Time:      result.Duration.Seconds(),
		Timestamp: time.Now().Format(time.RFC3339),
		TestCases: make([]JUnitTestCase, 0, len(result.Results)),
	}

	for _, r := range result.Results {
		tc := JUnitTestCase{
			Name:      r.Name,
			ClassName: result.File,
			Time:      r.Duration.Seconds(),
		}

		if r.Skipped {
			tc.Skipped = &JUnitSkipped{
				Message: r.SkipReason,
			}
		} else if r.Error != nil {
			suite.Errors++
			tc.Error = &JUnitError{
				Message: r.Error.Error(),
				Type:    "Error",
			}
		} else if !r.Passed {
			// Collect failure messages from assertions
			var failureMsg strings.Builder
			for _, a := range r.Assertions {
				if !a.Passed {
					fmt.Fprintf(&failureMsg, "%s %s: expected %v, got %v. %s\n",
						a.Subject, a.Operator, a.Expected, a.Actual, a.Message)
				}
			}
			tc.Failure = &JUnitFailure{
				Message: "Assertion failed",
				Type:    "AssertionError",
				Content: failureMsg.String(),
			}
		}

		suite.TestCases = append(suite.TestCases, tc)
	}

	f.testSuites = append(f.testSuites, suite)
}

func (f *JUnitFormatter) FormatError(err error) {
	// Errors are included in individual test cases
}

func (f *JUnitFormatter) FormatHeader(version string) {
	// No header needed for JUnit XML
}

// Flush writes the accumulated JUnit XML output
func (f *JUnitFormatter) Flush(totalDuration time.Duration) error {
	var totalTests, totalFailures, totalErrors, totalSkipped int
	for _, suite := range f.testSuites {
		totalTests += suite.Tests
		totalFailures += suite.Failures
		totalErrors += suite.Errors
		totalSkipped += suite.Skipped
	}

	suites := JUnitTestSuites{
		Name:       "hitspec",
		Tests:      totalTests,
		Failures:   totalFailures,
		Errors:     totalErrors,
		Skipped:    totalSkipped,
		Time:       totalDuration.Seconds(),
		Timestamp:  time.Now().Format(time.RFC3339),
		TestSuites: f.testSuites,
	}

	fmt.Fprintf(f.writer, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	encoder := xml.NewEncoder(f.writer)
	encoder.Indent("", "  ")
	return encoder.Encode(suites)
}
