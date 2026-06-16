package mock

import (
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

// An explicit >>>mock block is returned verbatim as the response body, and the
// status still comes from an `expect status` assertion.
func TestCreateMockResponse_UsesMockBlock(t *testing.T) {
	s := NewServer()
	req := &parser.Request{
		Method:   "GET",
		URL:      "http://localhost:3000/users/1",
		MockBody: `{"id": 1, "name": "Alice"}`,
		Assertions: []*parser.Assertion{
			{Subject: "status", Expected: 200},
		},
	}
	resp := s.createMockResponse(req)
	if resp.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(resp.Body, `"name": "Alice"`) {
		t.Errorf("body should be the mock block, got: %s", resp.Body)
	}
}

// Without a >>>mock block, the body is inferred from body assertions and the
// status from an `expect status` assertion (backward-compatible behavior).
func TestCreateMockResponse_FallsBackToAssertions(t *testing.T) {
	s := NewServer()
	req := &parser.Request{
		Method: "POST",
		URL:    "http://localhost:3000/users",
		Assertions: []*parser.Assertion{
			{Subject: "status", Expected: 201},
			{Subject: "body.name", Operator: parser.OpEquals, Expected: "Bob"},
		},
	}
	resp := s.createMockResponse(req)
	if resp.StatusCode != 201 {
		t.Errorf("status: got %d, want 201", resp.StatusCode)
	}
	if !strings.Contains(resp.Body, "Bob") {
		t.Errorf("body should be inferred from assertions, got: %s", resp.Body)
	}
}

// A verbatim >>>mock body gets a Content-Type matching its content, not a
// hardcoded application/json.
func TestCreateMockResponse_InfersContentType(t *testing.T) {
	s := NewServer()
	cases := []struct{ body, want string }{
		{`{"a":1}`, "application/json"},
		{`[1,2,3]`, "application/json"},
		{`<?xml version="1.0"?><root/>`, "application/xml"},
		{`<html><body>hi</body></html>`, "text/html; charset=utf-8"},
		{`plain text response`, "text/plain; charset=utf-8"},
	}
	for _, c := range cases {
		req := &parser.Request{Method: "GET", URL: "http://x/a", MockBody: c.body}
		resp := s.createMockResponse(req)
		if resp.ContentType != c.want {
			t.Errorf("body %q: got Content-Type %q, want %q", c.body, resp.ContentType, c.want)
		}
	}
}
