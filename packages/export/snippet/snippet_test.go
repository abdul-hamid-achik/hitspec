package snippet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
)

const sample = `### Create
# @name create
# @auth bearer tok123
POST https://api.example.com/users
? page = 2
Content-Type: application/json

{"name":"it's"}

### Upload
# @name upload
POST https://api.example.com/up

>>>multipart
field a = b
<<<
`

func parse(t *testing.T) map[string]*parser.Request {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s.http")
	if err := os.WriteFile(path, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := parser.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*parser.Request{}
	for _, r := range f.Requests {
		out[r.Name] = r
	}
	return out
}

func TestRenderEveryFormat(t *testing.T) {
	req := parse(t)["create"]
	want := map[string][]string{
		"curl":   {"curl -X POST", "Authorization: Bearer tok123", "page=2"},
		"fetch":  {`fetch("https://api.example.com/users?page=2"`, `"Authorization": "Bearer tok123"`},
		"wget":   {"wget --method=POST", "--header='Authorization: Bearer tok123'"},
		"python": {"import requests", `"Authorization": "Bearer tok123"`, "page=2"},
		"httpie": {"http POST 'https://api.example.com/users?page=2'", `'Authorization:Bearer tok123'`, `'\''`},
		"go":     {"http.NewRequest(\"POST\"", `req.Header.Set("Authorization", "Bearer tok123")`},
		"ruby":   {"Net::HTTP::Post.new(uri)", `req["Authorization"] = "Bearer tok123"`},
	}
	if len(want) != len(Formats) {
		t.Fatalf("test covers %d formats, package lists %d", len(want), len(Formats))
	}
	for _, f := range Formats {
		got, err := Render(f, req)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		for _, w := range want[f] {
			if !strings.Contains(got, w) {
				t.Errorf("%s output missing %q:\n%s", f, w, got)
			}
		}
	}
}

func TestRenderRejectsWhatItCannotExpress(t *testing.T) {
	if _, err := Render("python", parse(t)["upload"]); err == nil || !strings.Contains(err.Error(), "multipart") {
		t.Errorf("multipart to python must be an error, got %v", err)
	}
	if _, err := Render("curl", parse(t)["upload"]); err != nil {
		t.Errorf("curl handles multipart: %v", err)
	}
	if _, err := Render("cobol", parse(t)["create"]); err == nil || !strings.Contains(err.Error(), "supported formats") {
		t.Errorf("unknown format must list the supported ones, got %v", err)
	}
}

func TestNormalizeAliases(t *testing.T) {
	for in, want := range map[string]string{"": "curl", "JS": "fetch", "javascript": "fetch", "Python": "python", "golang": "go"} {
		if got, ok := Normalize(in); !ok || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
}

func TestResolver(t *testing.T) {
	req := parse(t)["create"]
	got, err := Render("python", req, WithResolver(func(s string) string { return strings.ReplaceAll(s, "tok123", "RESOLVED") }))
	if err != nil || !strings.Contains(got, "Bearer RESOLVED") {
		t.Errorf("resolver not applied: %v\n%s", err, got)
	}
}

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
// snippet is a faithful, runnable copy of the request, not just method+URL.
func TestIncludesFullRequest(t *testing.T) {
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
			got, err := Render(tc.format, r)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("Render(%q) missing %q\n---\n%s", tc.format, w, got)
				}
			}
		})
	}
}

// A GET with no headers/body must not emit body/header scaffolding, and an
// empty method defaults to GET.
func TestBareGET(t *testing.T) {
	r := &parser.Request{URL: "https://api.example.com/ping"}
	for _, format := range []string{"fetch", "python", "httpie", "go", "ruby", "wget"} {
		got, err := Render(format, r)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, "api.example.com/ping") {
			t.Errorf("Render(%q) missing URL:\n%s", format, got)
		}
		if strings.Contains(got, "body") || strings.Contains(got, "--body-data") {
			t.Errorf("Render(%q) leaked body scaffolding for bare GET:\n%s", format, got)
		}
		if !strings.Contains(strings.ToUpper(got), "GET") {
			t.Errorf("Render(%q) lost the GET default:\n%s", format, got)
		}
	}
}
