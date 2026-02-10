package output

import (
	"bytes"
	"encoding/xml"
	"errors"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/assertions"
	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
)

func TestJUnitFormatter_PassingTest(t *testing.T) {
	var buf bytes.Buffer
	f := NewJUnitFormatter(JUnitWithWriter(&buf))

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

	// Parse XML
	var suites JUnitTestSuites
	if err := xml.Unmarshal(stripXMLHeader(buf.Bytes()), &suites); err != nil {
		t.Fatalf("invalid XML: %v\n%s", err, buf.String())
	}

	if suites.Tests != 1 {
		t.Errorf("total tests = %d, want 1", suites.Tests)
	}
	if suites.Failures != 0 {
		t.Errorf("failures = %d, want 0", suites.Failures)
	}
	if len(suites.TestSuites) != 1 {
		t.Fatalf("suites count = %d, want 1", len(suites.TestSuites))
	}

	tc := suites.TestSuites[0].TestCases[0]
	if tc.Name != "GET /users" {
		t.Errorf("name = %q, want %q", tc.Name, "GET /users")
	}
	if tc.Failure != nil {
		t.Error("passing test should not have failure")
	}
	if tc.Error != nil {
		t.Error("passing test should not have error")
	}
}

func TestJUnitFormatter_FailedAssertion(t *testing.T) {
	var buf bytes.Buffer
	f := NewJUnitFormatter(JUnitWithWriter(&buf))

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
					Message:  "unauthorized",
				},
			},
		},
	}))

	if err := f.Flush(50 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	var suites JUnitTestSuites
	if err := xml.Unmarshal(stripXMLHeader(buf.Bytes()), &suites); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}

	if suites.Failures != 1 {
		t.Errorf("failures = %d, want 1", suites.Failures)
	}

	tc := suites.TestSuites[0].TestCases[0]
	if tc.Failure == nil {
		t.Fatal("failed test should have failure element")
	}
	if tc.Failure.Message != "Assertion failed" {
		t.Errorf("failure message = %q, want %q", tc.Failure.Message, "Assertion failed")
	}
}

func TestJUnitFormatter_ErrorTest(t *testing.T) {
	var buf bytes.Buffer
	f := NewJUnitFormatter(JUnitWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{
			Name:   "Broken request",
			Passed: false,
			Error:  errors.New("dns resolution failed"),
		},
	}))

	if err := f.Flush(10 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	var suites JUnitTestSuites
	if err := xml.Unmarshal(stripXMLHeader(buf.Bytes()), &suites); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}

	if suites.Errors != 1 {
		t.Errorf("errors = %d, want 1", suites.Errors)
	}

	tc := suites.TestSuites[0].TestCases[0]
	if tc.Error == nil {
		t.Fatal("error test should have error element")
	}
	if tc.Error.Message != "dns resolution failed" {
		t.Errorf("error message = %q, want %q", tc.Error.Message, "dns resolution failed")
	}
}

func TestJUnitFormatter_SkippedTest(t *testing.T) {
	var buf bytes.Buffer
	f := NewJUnitFormatter(JUnitWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{
			Name:       "Optional test",
			Skipped:    true,
			SkipReason: "env not set",
		},
	}))

	if err := f.Flush(10 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	var suites JUnitTestSuites
	if err := xml.Unmarshal(stripXMLHeader(buf.Bytes()), &suites); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}

	if suites.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", suites.Skipped)
	}

	tc := suites.TestSuites[0].TestCases[0]
	if tc.Skipped == nil {
		t.Fatal("skipped test should have skipped element")
	}
	if tc.Skipped.Message != "env not set" {
		t.Errorf("skip message = %q, want %q", tc.Skipped.Message, "env not set")
	}
}

func TestJUnitFormatter_MultipleSuites(t *testing.T) {
	var buf bytes.Buffer
	f := NewJUnitFormatter(JUnitWithWriter(&buf))

	f.FormatResult(&runner.RunResult{
		File:     "auth.http",
		Results:  []*runner.RequestResult{{Name: "Login", Passed: true, Duration: time.Millisecond}},
		Duration: 10 * time.Millisecond,
		Passed:   1,
	})
	f.FormatResult(&runner.RunResult{
		File:     "users.http",
		Results:  []*runner.RequestResult{{Name: "List Users", Passed: true, Duration: time.Millisecond}},
		Duration: 10 * time.Millisecond,
		Passed:   1,
	})

	if err := f.Flush(100 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	var suites JUnitTestSuites
	if err := xml.Unmarshal(stripXMLHeader(buf.Bytes()), &suites); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}

	if len(suites.TestSuites) != 2 {
		t.Errorf("suites count = %d, want 2", len(suites.TestSuites))
	}
	if suites.Tests != 2 {
		t.Errorf("total tests = %d, want 2", suites.Tests)
	}
}

// stripXMLHeader removes the <?xml ...?> declaration to allow xml.Unmarshal
func stripXMLHeader(data []byte) []byte {
	s := string(data)
	if idx := bytes.Index(data, []byte("<testsuites")); idx >= 0 {
		return []byte(s[idx:])
	}
	return data
}
