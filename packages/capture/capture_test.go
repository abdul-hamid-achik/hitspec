package capture

import (
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/http"
)

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(body),
		Duration:   42 * time.Millisecond,
	}
}

func textResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Headers:    map[string]string{"Content-Type": "text/plain", "X-Request-Id": "abc-123"},
		Body:       []byte(body),
		Duration:   10 * time.Millisecond,
	}
}

func TestExtract_BodyJSON(t *testing.T) {
	resp := jsonResponse(`{"user":{"name":"Alice","age":30},"tags":["admin","user"]}`)

	tests := []struct {
		name    string
		capture parser.Capture
		want    any
		wantOK  bool
	}{
		{
			name:    "nested field",
			capture: parser.Capture{Name: "name", Source: parser.CaptureBody, Path: "user.name"},
			want:    "Alice",
			wantOK:  true,
		},
		{
			name:    "numeric field",
			capture: parser.Capture{Name: "age", Source: parser.CaptureBody, Path: "user.age"},
			want:    float64(30),
			wantOK:  true,
		},
		{
			name:    "nonexistent path",
			capture: parser.Capture{Name: "x", Source: parser.CaptureBody, Path: "user.email"},
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "empty path returns whole body",
			capture: parser.Capture{Name: "all", Source: parser.CaptureBody, Path: ""},
			wantOK:  true,
		},
		{
			name:    "array path",
			capture: parser.Capture{Name: "tag", Source: parser.CaptureBody, Path: "tags.0"},
			want:    "admin",
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExtractor(resp)
			got, ok := e.Extract(&tt.capture)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.want != nil && got != tt.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestExtract_BodyText(t *testing.T) {
	resp := textResponse("Hello, World!")

	// Empty path on non-JSON body returns body string
	e := NewExtractor(resp)
	got, ok := e.Extract(&parser.Capture{Name: "body", Source: parser.CaptureBody, Path: ""})
	if !ok {
		t.Fatal("expected ok for empty path on text body")
	}
	if got != "Hello, World!" {
		t.Errorf("got %q, want %q", got, "Hello, World!")
	}

	// Non-empty path on non-JSON body returns false
	_, ok = e.Extract(&parser.Capture{Name: "x", Source: parser.CaptureBody, Path: "field"})
	if ok {
		t.Error("expected !ok for path on non-JSON body")
	}
}

func TestExtract_Header(t *testing.T) {
	resp := textResponse("body")

	e := NewExtractor(resp)
	got, ok := e.Extract(&parser.Capture{Name: "reqid", Source: parser.CaptureHeader, Path: "X-Request-Id"})
	if !ok {
		t.Fatal("expected ok for existing header")
	}
	if got != "abc-123" {
		t.Errorf("got %q, want %q", got, "abc-123")
	}

	// Non-existent header
	_, ok = e.Extract(&parser.Capture{Name: "x", Source: parser.CaptureHeader, Path: "X-Missing"})
	if ok {
		t.Error("expected !ok for missing header")
	}
}

func TestExtract_Status(t *testing.T) {
	resp := jsonResponse(`{}`)
	e := NewExtractor(resp)

	got, ok := e.Extract(&parser.Capture{Name: "code", Source: parser.CaptureStatus})
	if !ok {
		t.Fatal("expected ok for status")
	}
	if got != 200 {
		t.Errorf("got %v, want 200", got)
	}
}

func TestExtract_Duration(t *testing.T) {
	resp := jsonResponse(`{}`)
	e := NewExtractor(resp)

	got, ok := e.Extract(&parser.Capture{Name: "dur", Source: parser.CaptureDuration})
	if !ok {
		t.Fatal("expected ok for duration")
	}
	if got != int64(42) {
		t.Errorf("got %v, want 42", got)
	}
}

func TestExtract_UnknownSource(t *testing.T) {
	resp := jsonResponse(`{}`)
	e := NewExtractor(resp)

	_, ok := e.Extract(&parser.Capture{Name: "x", Source: parser.CaptureSource(99)})
	if ok {
		t.Error("expected !ok for unknown source")
	}
}

func TestExtractAll(t *testing.T) {
	resp := jsonResponse(`{"token":"jwt-abc","user":"alice"}`)

	captures := []*parser.Capture{
		{Name: "token", Source: parser.CaptureBody, Path: "token"},
		{Name: "user", Source: parser.CaptureBody, Path: "user"},
		{Name: "status", Source: parser.CaptureStatus},
		{Name: "missing", Source: parser.CaptureBody, Path: "nonexistent"},
	}

	results := ExtractAll(resp, captures)

	if results["token"] != "jwt-abc" {
		t.Errorf("token = %v, want %q", results["token"], "jwt-abc")
	}
	if results["user"] != "alice" {
		t.Errorf("user = %v, want %q", results["user"], "alice")
	}
	if results["status"] != 200 {
		t.Errorf("status = %v, want 200", results["status"])
	}
	if _, ok := results["missing"]; ok {
		t.Error("missing capture should not be in results")
	}
}

func TestExtractAll_Empty(t *testing.T) {
	resp := jsonResponse(`{}`)
	results := ExtractAll(resp, nil)
	if len(results) != 0 {
		t.Errorf("expected empty results, got %v", results)
	}
}
