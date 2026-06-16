package clientmgr

import (
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

func sampleRequest() *parser.Request {
	return &parser.Request{
		Method: "POST",
		URL:    "https://api.example.com/users",
		Headers: []*parser.Header{
			{Key: "Content-Type", Value: "application/json"},
			{Key: "X-Api-Key", Value: "secret"},
		},
		Body: &parser.Body{
			ContentType: parser.BodyJSON,
			Raw:         `{"name":"Jane"}`,
		},
	}
}

// Each exporter must include the method, URL, every header, and the body so the
// snippet is a faithful, runnable copy of the request — not just method+URL.
func TestExportSnippet_IncludesFullRequest(t *testing.T) {
	r := sampleRequest()
	cases := []struct {
		format string
		want   []string
	}{
		{"fetch", []string{"fetch(", `"https://api.example.com/users"`, `method: "POST"`, "Content-Type", "X-Api-Key", `body:`, "Jane"}},
		{"python", []string{"import requests", `"POST"`, "api.example.com/users", "headers=headers", "Content-Type", "X-Api-Key", "data=data", "Jane"}},
		{"httpie", []string{"http POST", "api.example.com/users", "Content-Type:application/json", "X-Api-Key:secret", "echo", "Jane"}},
		{"go", []string{"http.NewRequest(", `"POST"`, "api.example.com/users", "strings.NewReader(", "req.Header.Set(", "Content-Type", "X-Api-Key", "Jane"}},
		{"ruby", []string{"Net::HTTP::Post", "uri = URI(", "api.example.com/users", "req[", "Content-Type", "X-Api-Key", "req.body =", "Jane"}},
		{"wget", []string{"wget --method=POST", "--header=", "Content-Type: application/json", "X-Api-Key: secret", "--body-data=", "Jane", "api.example.com/users"}},
	}
	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			got := exportSnippet(tc.format, r)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("exportSnippet(%q) missing %q\n---\n%s", tc.format, w, got)
				}
			}
		})
	}
}

// A GET with no headers/body must still produce a valid method+URL snippet and
// must not emit body/header scaffolding.
func TestExportSnippet_BareGET(t *testing.T) {
	r := &parser.Request{Method: "GET", URL: "https://api.example.com/ping"}
	for _, format := range []string{"fetch", "python", "httpie", "go", "ruby", "wget"} {
		got := exportSnippet(format, r)
		if !strings.Contains(got, "api.example.com/ping") {
			t.Errorf("exportSnippet(%q) missing URL:\n%s", format, got)
		}
		if strings.Contains(got, "body") || strings.Contains(got, "--body-data") {
			t.Errorf("exportSnippet(%q) leaked body scaffolding for bare GET:\n%s", format, got)
		}
	}
}

// Empty method must default to GET across languages.
func TestExportSnippet_DefaultsMethod(t *testing.T) {
	r := &parser.Request{URL: "https://api.example.com/x"}
	if got := exportSnippet("python", r); !strings.Contains(got, `"GET"`) {
		t.Errorf("expected GET default, got:\n%s", got)
	}
	if got := exportSnippet("wget", r); !strings.Contains(got, "--method=GET") {
		t.Errorf("expected GET default, got:\n%s", got)
	}
}

// An unknown format falls back to a plain "METHOD URL" line.
func TestExportSnippet_UnknownFormat(t *testing.T) {
	r := &parser.Request{Method: "GET", URL: "https://api.example.com/x"}
	if got := exportSnippet("cobol", r); got != "GET https://api.example.com/x" {
		t.Errorf("unexpected fallback: %q", got)
	}
}
