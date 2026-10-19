package output

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/assertions"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
	"gopkg.in/yaml.v3"
)

func failedResult(body string) *runner.RequestResult {
	return &runner.RequestResult{
		Name:     "get-user",
		Duration: 12 * time.Millisecond,
		Request: &http.Request{
			Method:  "GET",
			URL:     "https://api.example.com/users/42?api_key=hunter2&page=2",
			Headers: map[string]string{"Authorization": "Bearer hunter2", "Accept": "application/json"},
			Body:    `{"q":"x"}`,
		},
		Response: &http.Response{
			StatusCode: 404,
			Status:     "404 Not Found",
			Headers:    map[string]string{"Set-Cookie": "sid=hunter2", "X-Trace": "t1"},
			Body:       []byte(body),
			Duration:   9 * time.Millisecond,
		},
		Assertions: []*assertions.Result{{Subject: "status", Operator: "==", Expected: 200, Actual: 404}},
		Warnings:   []string{`capture "id" captured nothing`},
	}
}

func consoleOut(level int, r *runner.RequestResult) string {
	var buf bytes.Buffer
	f := NewConsoleFormatter(WithWriter(&buf), WithNoColor(true), WithVerbosity(level))
	f.FormatResult(makeRunResult([]*runner.RequestResult{r}))
	return buf.String()
}

func TestConsole_FailureContext(t *testing.T) {
	out := consoleOut(0, failedResult(`{"error":"not found"}`))
	for _, want := range []string{
		"Request:  GET https://api.example.com/users/42?api_key=[REDACTED]&page=2",
		"Response: 404 Not Found (9ms)",
		"Body:\n      {\n        \"error\": \"not found\"\n      }",
		"Expected: 200",
		"Actual:   404",
		"warning: capture \"id\" captured nothing",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	for _, bad := range []string{"hunter2", "Request headers", "Response headers", "Request body"} {
		if strings.Contains(out, bad) {
			t.Errorf("unexpected %q in:\n%s", bad, out)
		}
	}
}

func TestConsole_ErroredRequestShowsRequestLine(t *testing.T) {
	r := &runner.RequestResult{
		Name:    "down",
		Error:   errString("dial tcp: connection refused"),
		Request: &http.Request{Method: "POST", URL: "http://localhost:1/x"},
	}
	out := consoleOut(0, r)
	if !strings.Contains(out, "(dial tcp: connection refused)") || !strings.Contains(out, "Request:  POST http://localhost:1/x") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if strings.Contains(out, "Response:") {
		t.Errorf("no response expected:\n%s", out)
	}
	// An error before the request was built (waitFor, a hook) has no request.
	if out := consoleOut(0, &runner.RequestResult{Name: "hook", Error: errString("hook failed")}); strings.Contains(out, "Request:") {
		t.Errorf("unexpected request line:\n%s", out)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestConsole_VerbosityLevels(t *testing.T) {
	pass := failedResult(`{"ok":true}`)
	pass.Passed, pass.Assertions = true, nil
	pass.Response.StatusCode, pass.Response.Status = 200, "200 OK"

	v0 := consoleOut(0, pass)
	v1 := consoleOut(1, pass)
	v2 := consoleOut(2, pass)
	v3 := consoleOut(3, pass)

	has := func(out, s string) bool { return strings.Contains(out, s) }
	if has(v0, "Request:") || has(v0, "Response:") {
		t.Errorf("level 0 should be terse:\n%s", v0)
	}
	if !has(v1, "Request:  GET") || !has(v1, "Response: 200 OK") || has(v1, "headers:") || has(v1, "Body:") {
		t.Errorf("level 1 wrong:\n%s", v1)
	}
	if !has(v2, "Request headers:") || !has(v2, "Authorization: [REDACTED]") || !has(v2, "Set-Cookie: [REDACTED]") ||
		!has(v2, "X-Trace: t1") || has(v2, "Body:") || has(v2, "Request body:") {
		t.Errorf("level 2 wrong:\n%s", v2)
	}
	if !has(v3, "Request body:") || !has(v3, "\"q\": \"x\"") || !has(v3, "Body:") || !has(v3, "\"ok\": true") {
		t.Errorf("level 3 wrong:\n%s", v3)
	}
	if has(v2, "hunter2") || has(v3, "hunter2") {
		t.Errorf("credentials leaked:\n%s", v3)
	}
}

func TestConsole_WithVerboseBoolStillWorks(t *testing.T) {
	var buf bytes.Buffer
	f := NewConsoleFormatter(WithWriter(&buf), WithNoColor(true), WithVerbose(true))
	if f.verbose != 1 {
		t.Errorf("WithVerbose(true) = level %d, want 1", f.verbose)
	}
	if NewConsoleFormatter(WithVerbose(false)).verbose != 0 {
		t.Error("WithVerbose(false) should be level 0")
	}
}

func TestFormatBody(t *testing.T) {
	t.Run("pretty-prints json", func(t *testing.T) {
		b := formatBody([]byte(`{"a":[1,2]}`), true)
		if b.Truncated || !strings.Contains(b.Text, "\n  \"a\": [") {
			t.Errorf("got %+v", b)
		}
	})
	t.Run("caps lines", func(t *testing.T) {
		raw := strings.Repeat("line\n", 100)
		b := formatBody([]byte(raw), true)
		if !b.Truncated || strings.Count(b.Text, "\n") != excerptMaxLines-1 {
			t.Errorf("got %d lines truncated=%v", strings.Count(b.Text, "\n")+1, b.Truncated)
		}
		if !strings.Contains(b.Note, "more lines") {
			t.Errorf("note = %q", b.Note)
		}
	})
	t.Run("caps bytes on a rune boundary", func(t *testing.T) {
		b := formatBody([]byte(strings.Repeat("é", 3000)), true)
		if !b.Truncated || len(b.Text) > excerptMaxBytes || strings.ContainsRune(b.Text, '\uFFFD') {
			t.Errorf("bad cut: len=%d truncated=%v", len(b.Text), b.Truncated)
		}
	})
	t.Run("full keeps everything", func(t *testing.T) {
		raw := strings.Repeat("x", 5000)
		if b := formatBody([]byte(raw), false); b.Truncated || b.Text != raw {
			t.Error("full body was cut")
		}
	})
	t.Run("binary", func(t *testing.T) {
		b := formatBody([]byte{0x89, 'P', 0, 1}, true)
		if !b.Binary || b.Text != "[4 bytes, binary]" {
			t.Errorf("got %+v", b)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if b := formatBody(nil, true); b.Total != 0 || b.Text != "" {
			t.Errorf("got %+v", b)
		}
	})
}

func TestSanitize(t *testing.T) {
	if got := sanitize("a\x1b[31mb\tc\nd"); got != "a\uFFFD[31mb\tc\nd" {
		t.Errorf("got %q", got)
	}
}

func TestIsSensitiveName(t *testing.T) {
	for _, name := range []string{"Authorization", "cookie", "Set-Cookie", "X-Api-Key", "x_api_key", "X-Custom-Api-Key",
		"X-Amz-Security-Token", "X-Client-Secret", "api_key", "access_token", "password"} {
		if !isSensitiveName(name) {
			t.Errorf("%s should be sensitive", name)
		}
	}
	for _, name := range []string{"Accept", "Content-Type", "X-Trace", "page"} {
		if isSensitiveName(name) {
			t.Errorf("%s should not be sensitive", name)
		}
	}
}

func TestRedactURL(t *testing.T) {
	tests := map[string]string{
		"https://api.example.com/x":                     "https://api.example.com/x",
		"https://u:p4ss@api.example.com/x?a=1":          "https://u:[REDACTED]@api.example.com/x?a=1",
		"https://u@api.example.com/x":                   "https://u@api.example.com/x",
		"https://h/x?api_key=abc&page=2&access_token=t": "https://h/x?api_key=[REDACTED]&page=2&access_token=[REDACTED]",
		"https://h/x?token={{login.token}}":             "https://h/x?token={{login.token}}",
		"https://h/x?token=%7B%7Blogin.token%7D%7D":     "https://h/x?token=%7B%7Blogin.token%7D%7D",
		"https://h/x?a=b c%zz":                          "https://h/x?a=b c%zz",
		"not a url at all ? api_key=1":                  "not a url at all ? api_key=[REDACTED]",
		"https://u:pw@h:8080/p?x=1#frag":                "https://u:[REDACTED]@h:8080/p?x=1#frag",
		"https://h/x?flag&sig=1&token":                  "https://h/x?flag&sig=1&token",
	}
	for in, want := range tests {
		if got := RedactURL(in); got != want {
			t.Errorf("RedactURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJSON_FailureBodyExcerptAndWarnings(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(JSONWithWriter(&buf))
	big := `{"items":[` + strings.Repeat(`{"id":1},`, 500) + `{"id":2}]}`
	f.FormatResult(makeRunResult([]*runner.RequestResult{failedResult(big)}))
	if err := f.Flush(time.Second); err != nil {
		t.Fatal(err)
	}
	var out JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	tt := out.Tests[0]
	if tt.Request.Method != "GET" || tt.Response.StatusCode != 404 || tt.Response.Status != "404 Not Found" {
		t.Errorf("request/response: %+v %+v", tt.Request, tt.Response)
	}
	if !tt.Response.BodyTruncated || !strings.Contains(tt.Response.BodyExcerpt, `"items"`) {
		t.Errorf("excerpt = %q truncated=%v", tt.Response.BodyExcerpt, tt.Response.BodyTruncated)
	}
	if len(tt.Warnings) != 1 {
		t.Errorf("warnings = %v", tt.Warnings)
	}
	// Passing tests carry no excerpt.
	pass := failedResult(`{"ok":true}`)
	pass.Passed = true
	buf.Reset()
	f = NewJSONFormatter(JSONWithWriter(&buf))
	f.FormatResult(makeRunResult([]*runner.RequestResult{pass}))
	_ = f.Flush(time.Second)
	if strings.Contains(buf.String(), "bodyExcerpt") {
		t.Error("passing test should not carry bodyExcerpt")
	}
}

func TestJUnit_FailureContextAndSystemOut(t *testing.T) {
	var buf bytes.Buffer
	f := NewJUnitFormatter(JUnitWithWriter(&buf))
	f.FormatResult(makeRunResult([]*runner.RequestResult{failedResult(`{"error":"nope"}`)}))
	if err := f.Flush(time.Second); err != nil {
		t.Fatal(err)
	}
	var suites JUnitTestSuites
	if err := xml.Unmarshal(buf.Bytes(), &suites); err != nil {
		t.Fatalf("invalid XML: %v\n%s", err, buf.String())
	}
	tc := suites.TestSuites[0].TestCases[0]
	for _, want := range []string{"Request: GET https://api.example.com/users/42?api_key=[REDACTED]&page=2", "Response: 404 Not Found (9ms)", `"error": "nope"`} {
		if !strings.Contains(tc.Failure.Content, want) {
			t.Errorf("failure missing %q:\n%s", want, tc.Failure.Content)
		}
	}
	if !strings.Contains(tc.SystemOut, "warning: capture") {
		t.Errorf("system-out = %q", tc.SystemOut)
	}
}

func TestTAP_FailureYAMLish(t *testing.T) {
	var buf bytes.Buffer
	f := NewTAPFormatter(TAPWithWriter(&buf))
	f.FormatResult(makeRunResult([]*runner.RequestResult{failedResult(`{"error":"nope"}`)}))
	_ = f.Flush(time.Second)
	out := buf.String()
	for _, want := range []string{
		`  request: "GET https://api.example.com/users/42?api_key=[REDACTED]&page=2"`,
		"  response:\n    status: \"404 Not Found\"\n    durationMs: 9\n    body: |2-\n      {\n        \"error\": \"nope\"\n      }\n",
		"  warnings:\n    - \"capture",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestHTML_RedactsHeaders(t *testing.T) {
	var buf bytes.Buffer
	f := NewHTMLFormatter(HTMLWithWriter(&buf))
	f.FormatResult(makeRunResult([]*runner.RequestResult{failedResult(`{}`)}))
	_ = f.Flush(time.Second)
	if strings.Contains(buf.String(), "hunter2") && !strings.Contains(buf.String(), "[REDACTED]") {
		t.Error("HTML shows credentials")
	}
	if strings.Contains(buf.String(), "Bearer hunter2") || strings.Contains(buf.String(), "sid=hunter2") {
		t.Error("HTML leaked a header value")
	}
}

func TestRedactText(t *testing.T) {
	tests := map[string]string{
		`Get "http://h/x?api_key=abc&page=2": dial tcp 127.0.0.1:1: connect: connection refused`: `Get "http://h/x?api_key=[REDACTED]&page=2": dial tcp 127.0.0.1:1: connect: connection refused`,
		`Post "https://u:xxxxx@h/x?access_token=t": EOF`:                                         `Post "https://u:[REDACTED]@h/x?access_token=[REDACTED]": EOF`,
		`plain error`:                           `plain error`,
		`what? token={{login.token}} ok`:        `what? token={{login.token}} ok`,
		`Get "http://h/x?token={{login.t}}": x`: `Get "http://h/x?token={{login.t}}": x`,
	}
	for in, want := range tests {
		if got := RedactText(in); got != want {
			t.Errorf("RedactText(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}

func TestTAP_DiagnosticsAreValidYAML(t *testing.T) {
	r := failedResult("  leading spaces\n\nblank line above: {\"x\": [1]}\n\ttab")
	r.Request.URL = `https://h/x?q="quoted"&a=b#frag`
	r.Warnings = []string{"colon: and \"quotes\" and \\ backslash"}
	var buf bytes.Buffer
	f := NewTAPFormatter(TAPWithWriter(&buf))
	f.FormatResult(makeRunResult([]*runner.RequestResult{r}))
	_ = f.Flush(time.Second)

	out := buf.String()
	start := strings.Index(out, "  ---\n")
	end := strings.Index(out, "  ...\n")
	if start < 0 || end < start {
		t.Fatalf("no YAMLish block in:\n%s", out)
	}
	var doc struct {
		Request  string `yaml:"request"`
		Response struct {
			Status     string `yaml:"status"`
			DurationMs int    `yaml:"durationMs"`
			Body       string `yaml:"body"`
		} `yaml:"response"`
		Warnings []string `yaml:"warnings"`
	}
	if err := yaml.Unmarshal([]byte(out[start+len("  ---\n"):end]), &doc); err != nil {
		t.Fatalf("invalid YAML: %v\n%s", err, out)
	}
	if doc.Response.Status != "404 Not Found" || doc.Response.DurationMs != 9 {
		t.Errorf("response = %+v", doc.Response)
	}
	if !strings.HasPrefix(doc.Request, "GET https://h/x?q=") || !strings.Contains(doc.Request, `"quoted"`) {
		t.Errorf("request = %q", doc.Request)
	}
	if want := "  leading spaces\n\nblank line above: {\"x\": [1]}\n\ttab"; doc.Response.Body != want {
		t.Errorf("body = %q, want %q", doc.Response.Body, want)
	}
	if len(doc.Warnings) != 1 || doc.Warnings[0] != "colon: and \"quotes\" and \\ backslash" {
		t.Errorf("warnings = %q", doc.Warnings)
	}
}
