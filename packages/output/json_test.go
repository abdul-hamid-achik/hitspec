package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/assertions"
	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
	"github.com/abdul-hamid-achik/hitspec/packages/http"
)

func makeRunResult(results []*runner.RequestResult) *runner.RunResult {
	passed, failed, skipped := 0, 0, 0
	for _, r := range results {
		switch {
		case r.Skipped:
			skipped++
		case r.Passed:
			passed++
		default:
			failed++
		}
	}
	return &runner.RunResult{
		File:     "test.http",
		Results:  results,
		Duration: 100 * time.Millisecond,
		Passed:   passed,
		Failed:   failed,
		Skipped:  skipped,
	}
}

func TestJSONFormatter_PassingTest(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(JSONWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{
			Name:        "GET /users",
			Description: "List all users",
			Passed:      true,
			Duration:    42 * time.Millisecond,
			Request: &http.Request{
				Method: "GET",
				URL:    "http://localhost/users",
			},
			Response: &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Duration:   42 * time.Millisecond,
			},
		},
	}))

	if err := f.Flush(100 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	var out JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if out.Summary.Total != 1 {
		t.Errorf("total = %d, want 1", out.Summary.Total)
	}
	if out.Summary.Passed != 1 {
		t.Errorf("passed = %d, want 1", out.Summary.Passed)
	}
	if out.Summary.Failed != 0 {
		t.Errorf("failed = %d, want 0", out.Summary.Failed)
	}
	if out.Tests[0].Name != "GET /users" {
		t.Errorf("name = %q, want %q", out.Tests[0].Name, "GET /users")
	}
	if out.Tests[0].Description != "List all users" {
		t.Errorf("description = %q, want %q", out.Tests[0].Description, "List all users")
	}
	if !out.Tests[0].Passed {
		t.Error("test should be passed")
	}
	if out.Tests[0].Request.Method != "GET" {
		t.Errorf("method = %q, want %q", out.Tests[0].Request.Method, "GET")
	}
}

func TestJSONFormatter_FailedAssertions(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(JSONWithWriter(&buf))

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
					Message:  "authentication failed",
				},
			},
		},
	}))

	if err := f.Flush(50 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	var out JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if out.Summary.Failed != 1 {
		t.Errorf("failed = %d, want 1", out.Summary.Failed)
	}
	if len(out.Tests[0].Assertions) != 1 {
		t.Fatalf("assertions count = %d, want 1", len(out.Tests[0].Assertions))
	}
	a := out.Tests[0].Assertions[0]
	if a.Passed {
		t.Error("assertion should not be passed")
	}
	if a.Subject != "status" {
		t.Errorf("subject = %q, want %q", a.Subject, "status")
	}
	if a.Message != "authentication failed" {
		t.Errorf("message = %q, want %q", a.Message, "authentication failed")
	}
}

func TestJSONFormatter_SkippedTest(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(JSONWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{
			Name:       "Conditional request",
			Skipped:    true,
			SkipReason: "@if condition false",
		},
	}))

	if err := f.Flush(10 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	var out JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if out.Summary.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", out.Summary.Skipped)
	}
	if !out.Tests[0].Skipped {
		t.Error("test should be skipped")
	}
	if out.Tests[0].SkipReason != "@if condition false" {
		t.Errorf("skip reason = %q, want %q", out.Tests[0].SkipReason, "@if condition false")
	}
}

func TestJSONFormatter_FilteredOutSkipReason(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(JSONWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{
			Name:       "Filtered request",
			Skipped:    true,
			SkipReason: "filtered out",
		},
	}))

	if err := f.Flush(10 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	var out JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// "filtered out" should be suppressed
	if out.Tests[0].SkipReason != "" {
		t.Errorf("skip reason should be empty for 'filtered out', got %q", out.Tests[0].SkipReason)
	}
}

func TestJSONFormatter_ErrorTest(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(JSONWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{
			Name:   "Timeout request",
			Passed: false,
			Error:  errors.New("connection timeout"),
		},
	}))

	if err := f.Flush(10 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	var out JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if out.Tests[0].Error != "connection timeout" {
		t.Errorf("error = %q, want %q", out.Tests[0].Error, "connection timeout")
	}
}

func TestJSONFormatter_Captures(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(JSONWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{
			Name:     "Login",
			Passed:   true,
			Duration: 5 * time.Millisecond,
			Captures: map[string]any{"token": "abc123"},
		},
	}))

	if err := f.Flush(10 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	var out JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if out.Tests[0].Captures["token"] != "abc123" {
		t.Errorf("capture token = %v, want %q", out.Tests[0].Captures["token"], "abc123")
	}
}

func TestJSONFormatter_MixedResults(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(JSONWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{Name: "Test1", Passed: true, Duration: time.Millisecond},
		{Name: "Test2", Passed: false, Duration: time.Millisecond},
		{Name: "Test3", Skipped: true},
	}))

	if err := f.Flush(100 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	var out JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if out.Summary.Total != 3 {
		t.Errorf("total = %d, want 3", out.Summary.Total)
	}
	if out.Summary.Passed != 1 {
		t.Errorf("passed = %d, want 1", out.Summary.Passed)
	}
	if out.Summary.Failed != 1 {
		t.Errorf("failed = %d, want 1", out.Summary.Failed)
	}
	if out.Summary.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", out.Summary.Skipped)
	}
}

func TestRedactHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    map[string]string
	}{
		{
			name:    "nil headers",
			headers: nil,
			want:    nil,
		},
		{
			name:    "no sensitive headers",
			headers: map[string]string{"Content-Type": "application/json"},
			want:    map[string]string{"Content-Type": "application/json"},
		},
		{
			name:    "authorization redacted",
			headers: map[string]string{"Authorization": "Bearer secret123"},
			want:    map[string]string{"Authorization": "[REDACTED]"},
		},
		{
			name:    "case insensitive match",
			headers: map[string]string{"X-Api-Key": "key123", "x-auth-token": "tok"},
			want:    map[string]string{"X-Api-Key": "[REDACTED]", "x-auth-token": "[REDACTED]"},
		},
		{
			name: "mixed sensitive and normal",
			headers: map[string]string{
				"Content-Type":  "text/html",
				"Cookie":        "session=abc",
				"X-Request-ID":  "123",
			},
			want: map[string]string{
				"Content-Type":  "text/html",
				"Cookie":        "[REDACTED]",
				"X-Request-ID":  "123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactHeaders(tt.headers)
			if tt.want == nil {
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("header %q = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestJSONFormatter_HeaderRedaction(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(JSONWithWriter(&buf))

	f.FormatResult(makeRunResult([]*runner.RequestResult{
		{
			Name:     "Auth request",
			Passed:   true,
			Duration: time.Millisecond,
			Request: &http.Request{
				Method:  "GET",
				URL:     "http://localhost/me",
				Headers: map[string]string{"Authorization": "Bearer secret"},
			},
			Response: &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Headers:    map[string]string{"Set-Cookie": "sess=abc"},
				Duration:   time.Millisecond,
			},
		},
	}))

	if err := f.Flush(10 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	var out JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if out.Tests[0].Request.Headers["Authorization"] != "[REDACTED]" {
		t.Errorf("request Authorization should be redacted, got %q", out.Tests[0].Request.Headers["Authorization"])
	}
	if out.Tests[0].Response.Headers["Set-Cookie"] != "[REDACTED]" {
		t.Errorf("response Set-Cookie should be redacted, got %q", out.Tests[0].Response.Headers["Set-Cookie"])
	}
}
