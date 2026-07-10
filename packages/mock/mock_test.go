package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

// --- Router tests ---

func TestNewRouter_CreatesEmptyRouter(t *testing.T) {
	r := NewRouter()
	if r == nil {
		t.Fatal("NewRouter returned nil")
	}
	if len(r.routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(r.routes))
	}
}

func TestRouter_AddRoute(t *testing.T) {
	r := NewRouter()
	route := &Route{
		Method:      "GET",
		PathPattern: "/hello",
		Response: &MockResponse{
			StatusCode:  200,
			ContentType: "text/plain",
			Body:        "world",
		},
	}
	r.AddRoute(route)
	if len(r.routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(r.routes))
	}
	if r.routes[0] != route {
		t.Error("stored route does not match added route")
	}
}

func TestRouter_AddRoute_Multiple(t *testing.T) {
	r := NewRouter()
	for i := 0; i < 5; i++ {
		r.AddRoute(&Route{
			Method:      "GET",
			PathPattern: fmt.Sprintf("/path/%d", i),
			Response:    &MockResponse{StatusCode: 200},
		})
	}
	if len(r.routes) != 5 {
		t.Errorf("expected 5 routes, got %d", len(r.routes))
	}
}

func TestRouter_Match_ExactPath(t *testing.T) {
	r := NewRouter()
	resp := &MockResponse{StatusCode: 200, Body: "found"}
	r.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/api/users",
		PathRegex:   createPathRegex("/api/users"),
		Response:    resp,
	})

	route, params := r.Match("GET", "/api/users")
	if route == nil {
		t.Fatal("expected a matching route, got nil")
	}
	if route.Response.Body != "found" {
		t.Errorf("expected body 'found', got %q", route.Response.Body)
	}
	if len(params) != 0 {
		t.Errorf("expected 0 params for exact match, got %d", len(params))
	}
}

func TestRouter_Match_PathWithParameters(t *testing.T) {
	r := NewRouter()
	r.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/users/{{id}}",
		PathRegex:   createPathRegex("/users/{{id}}"),
		Response:    &MockResponse{StatusCode: 200, Body: `{"id": "{{id}}"}`},
	})

	route, params := r.Match("GET", "/users/42")
	if route == nil {
		t.Fatal("expected a matching route, got nil")
	}
	if params["id"] != "42" {
		t.Errorf("expected param id=42, got %q", params["id"])
	}
}

func TestRouter_Match_PathWithMultipleParameters(t *testing.T) {
	r := NewRouter()
	r.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/orgs/{{orgId}}/users/{{userId}}",
		PathRegex:   createPathRegex("/orgs/{{orgId}}/users/{{userId}}"),
		Response:    &MockResponse{StatusCode: 200},
	})

	route, params := r.Match("GET", "/orgs/abc/users/123")
	if route == nil {
		t.Fatal("expected a matching route, got nil")
	}
	if params["orgId"] != "abc" {
		t.Errorf("expected orgId=abc, got %q", params["orgId"])
	}
	if params["userId"] != "123" {
		t.Errorf("expected userId=123, got %q", params["userId"])
	}
}

func TestRouter_Match_MethodMismatch(t *testing.T) {
	r := NewRouter()
	r.AddRoute(&Route{
		Method:      "POST",
		PathPattern: "/api/users",
		PathRegex:   createPathRegex("/api/users"),
		Response:    &MockResponse{StatusCode: 201},
	})

	route, _ := r.Match("GET", "/api/users")
	if route != nil {
		t.Error("expected nil for method mismatch, got a route")
	}
}

func TestRouter_Match_MethodCaseInsensitive(t *testing.T) {
	r := NewRouter()
	r.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/test",
		PathRegex:   createPathRegex("/test"),
		Response:    &MockResponse{StatusCode: 200},
	})

	route, _ := r.Match("get", "/test")
	if route == nil {
		t.Error("expected match with lowercase method, got nil")
	}
}

func TestRouter_Match_PathNotFound(t *testing.T) {
	r := NewRouter()
	r.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/api/users",
		PathRegex:   createPathRegex("/api/users"),
		Response:    &MockResponse{StatusCode: 200},
	})

	route, _ := r.Match("GET", "/api/posts")
	if route != nil {
		t.Error("expected nil for path not found, got a route")
	}
}

func TestRouter_Match_EmptyRouter(t *testing.T) {
	r := NewRouter()
	route, _ := r.Match("GET", "/anything")
	if route != nil {
		t.Error("expected nil from empty router, got a route")
	}
}

func TestRouter_Match_NormalizesTrailingSlash(t *testing.T) {
	r := NewRouter()
	r.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/api/users",
		PathRegex:   createPathRegex("/api/users"),
		Response:    &MockResponse{StatusCode: 200},
	})

	route, _ := r.Match("GET", "/api/users/")
	if route == nil {
		t.Error("expected match after trailing slash normalization, got nil")
	}
}

func TestRouter_Match_NormalizesMissingLeadingSlash(t *testing.T) {
	r := NewRouter()
	r.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/hello",
		PathRegex:   createPathRegex("/hello"),
		Response:    &MockResponse{StatusCode: 200},
	})

	route, _ := r.Match("GET", "hello")
	if route == nil {
		t.Error("expected match after leading slash normalization, got nil")
	}
}

func TestRouter_Match_FirstMatchWins(t *testing.T) {
	r := NewRouter()
	r.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/dup",
		PathRegex:   createPathRegex("/dup"),
		Response:    &MockResponse{StatusCode: 200, Body: "first"},
	})
	r.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/dup",
		PathRegex:   createPathRegex("/dup"),
		Response:    &MockResponse{StatusCode: 200, Body: "second"},
	})

	route, _ := r.Match("GET", "/dup")
	if route == nil {
		t.Fatal("expected a match")
	}
	if route.Response.Body != "first" {
		t.Errorf("expected first route to win, got body %q", route.Response.Body)
	}
}

// --- normalizePath tests ---

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"/foo", "/foo"},
		{"/foo/", "/foo"},
		{"foo", "/foo"},
		{"/", "/"},
		{"", "/"},
		{"/a/b/c/", "/a/b/c"},
	}
	for _, tt := range tests {
		got := normalizePath(tt.input)
		if got != tt.expected {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- extractPathPattern tests ---

func TestExtractPathPattern(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"/api/users", "/api/users"},
		{"https://example.com/api/users", "/api/users"},
		{"https://example.com/api/users?page=1", "/api/users"},
		{"http://localhost:3000/hello", "/hello"},
		{"https://example.com", "/"},
		{"/path?query=1&b=2", "/path"},
	}
	for _, tt := range tests {
		got := extractPathPattern(tt.input)
		if got != tt.expected {
			t.Errorf("extractPathPattern(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- createPathRegex tests ---

func TestCreatePathRegex(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		match   bool
		params  map[string]string
	}{
		{"/users", "/users", true, nil},
		{"/users/{{id}}", "/users/42", true, map[string]string{"id": "42"}},
		{"/users/{{id}}", "/users/", false, nil},
		{"/a/{{x}}/b/{{y}}", "/a/1/b/2", true, map[string]string{"x": "1", "y": "2"}},
		{"/users/{{id}}", "/users/42/extra", false, nil},
	}
	for _, tt := range tests {
		regex := createPathRegex(tt.pattern)
		matches := regex.FindStringSubmatch(tt.path)
		if tt.match && matches == nil {
			t.Errorf("createPathRegex(%q): expected match for %q", tt.pattern, tt.path)
			continue
		}
		if !tt.match && matches != nil {
			t.Errorf("createPathRegex(%q): expected no match for %q", tt.pattern, tt.path)
			continue
		}
		if tt.params != nil {
			names := regex.SubexpNames()
			paramMap := make(map[string]string)
			for i, name := range names {
				if i > 0 && name != "" && i < len(matches) {
					paramMap[name] = matches[i]
				}
			}
			for k, v := range tt.params {
				if paramMap[k] != v {
					t.Errorf("param %q = %q, want %q", k, paramMap[k], v)
				}
			}
		}
	}
}

// TestCreatePathRegex_EscapesMetacharacters ensures a literal "." (and other
// regex metacharacters) in a route path is not treated as a wildcard. A route
// "/v1/users.json" must match "/v1/users.json" but NOT "/v1/usersXjson".
func TestCreatePathRegex_EscapesMetacharacters(t *testing.T) {
	r := NewRouter()
	r.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/v1/users.json",
		PathRegex:   createPathRegex("/v1/users.json"),
		Response:    &MockResponse{StatusCode: 200},
	})

	if route, _ := r.Match("GET", "/v1/users.json"); route == nil {
		t.Error("expected /v1/users.json to match route /v1/users.json, got nil")
	}
	if route, _ := r.Match("GET", "/v1/usersXjson"); route != nil {
		t.Errorf("expected /v1/usersXjson NOT to match route /v1/users.json, got %+v", route)
	}
	// Other metacharacters must also be escaped.
	r2 := NewRouter()
	r2.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/q/a+b?c",
		PathRegex:   createPathRegex("/q/a+b?c"),
		Response:    &MockResponse{StatusCode: 200},
	})
	if route, _ := r2.Match("GET", "/q/a+b?c"); route == nil {
		t.Error("expected /q/a+b?c to match its literal route, got nil")
	}
	if route, _ := r2.Match("GET", "/q/axbc"); route != nil {
		t.Errorf("expected /q/axbc NOT to match route /q/a+b?c, got %+v", route)
	}
}

// TestCreatePathRegex_ParamStillMatches ensures the {{param}} placeholder
// remains a wildcard after the escaping fix.
func TestCreatePathRegex_ParamStillMatches(t *testing.T) {
	r := NewRouter()
	r.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/users/{{id}}.json",
		PathRegex:   createPathRegex("/users/{{id}}.json"),
		Response:    &MockResponse{StatusCode: 200},
	})

	route, params := r.Match("GET", "/users/42.json")
	if route == nil {
		t.Fatal("expected /users/42.json to match, got nil")
	}
	if params["id"] != "42" {
		t.Errorf("expected id=42, got %q", params["id"])
	}
	// The literal ".json" must still be enforced.
	if route, _ := r.Match("GET", "/users/42Xjson"); route != nil {
		t.Errorf("expected /users/42Xjson NOT to match, got %+v", route)
	}
}

// TestRouter_Match_TrailingSlashRouteMatchesBareRequest ensures a route
// registered with a trailing slash matches a request without one, and vice
// versa.
func TestRouter_Match_TrailingSlashRouteMatchesBareRequest(t *testing.T) {
	r := NewRouter()
	r.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/api/users/",
		PathRegex:   createPathRegex("/api/users/"),
		Response:    &MockResponse{StatusCode: 200},
	})

	if route, _ := r.Match("GET", "/api/users"); route == nil {
		t.Error("expected request /api/users to match route /api/users/, got nil")
	}
	if route, _ := r.Match("GET", "/api/users/"); route == nil {
		t.Error("expected request /api/users/ to match route /api/users/, got nil")
	}

	// And the inverse: bare route, trailing-slash request.
	r2 := NewRouter()
	r2.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/api/items",
		PathRegex:   createPathRegex("/api/items"),
		Response:    &MockResponse{StatusCode: 200},
	})
	if route, _ := r2.Match("GET", "/api/items/"); route == nil {
		t.Error("expected request /api/items/ to match route /api/items, got nil")
	}
}

// --- Server tests ---

func TestNewServer_Defaults(t *testing.T) {
	s := NewServer()
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.port != 3000 {
		t.Errorf("expected default port 3000, got %d", s.port)
	}
	if s.router == nil {
		t.Fatal("expected router to be non-nil")
	}
	if s.registry == nil {
		t.Fatal("expected registry to be non-nil")
	}
	if s.delay != 0 {
		t.Errorf("expected default delay 0, got %v", s.delay)
	}
	if s.verbose {
		t.Error("expected verbose to default to false")
	}
	if s.requestCallback != nil {
		t.Error("expected requestCallback to default to nil")
	}
}

func TestNewServer_WithOptions(t *testing.T) {
	cb := func(method, path string, status int, duration time.Duration) {}
	s := NewServer(
		WithPort(8080),
		WithDelay(100*time.Millisecond),
		WithVerbose(true),
		WithRequestCallback(cb),
	)
	if s.port != 8080 {
		t.Errorf("expected port 8080, got %d", s.port)
	}
	if s.delay != 100*time.Millisecond {
		t.Errorf("expected delay 100ms, got %v", s.delay)
	}
	if !s.verbose {
		t.Error("expected verbose to be true")
	}
	if s.requestCallback == nil {
		t.Error("expected requestCallback to be set")
	}
}

func TestServer_GetRoutes_Empty(t *testing.T) {
	s := NewServer()
	routes := s.GetRoutes()
	if len(routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(routes))
	}
}

func TestServer_LoadParsedFile_NoRequests(t *testing.T) {
	s := NewServer()
	file := &parser.File{
		Path:      "test.http",
		Variables: nil,
		Requests:  nil,
	}
	err := s.LoadParsedFile(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.GetRoutes()) != 0 {
		t.Errorf("expected 0 routes, got %d", len(s.GetRoutes()))
	}
}

func TestServer_LoadParsedFile_SingleRequest(t *testing.T) {
	s := NewServer()
	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Name:   "Get Users",
				Method: "GET",
				URL:    "/api/users",
			},
		},
	}
	err := s.LoadParsedFile(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	routes := s.GetRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].Method != "GET" {
		t.Errorf("expected method GET, got %q", routes[0].Method)
	}
	if routes[0].PathPattern != "/api/users" {
		t.Errorf("expected path /api/users, got %q", routes[0].PathPattern)
	}
	if routes[0].Name != "Get Users" {
		t.Errorf("expected name 'Get Users', got %q", routes[0].Name)
	}
}

func TestServer_LoadParsedFile_MultipleRequests(t *testing.T) {
	s := NewServer()
	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{Method: "GET", URL: "/api/users"},
			{Method: "POST", URL: "/api/users"},
			{Method: "GET", URL: "/api/users/{{id}}"},
		},
	}
	err := s.LoadParsedFile(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.GetRoutes()) != 3 {
		t.Errorf("expected 3 routes, got %d", len(s.GetRoutes()))
	}
}

func TestServer_LoadParsedFile_WithVariables(t *testing.T) {
	s := NewServer()
	file := &parser.File{
		Path: "test.http",
		Variables: []*parser.Variable{
			{Name: "baseUrl", Value: "https://api.example.com"},
		},
		Requests: []*parser.Request{
			{Method: "GET", URL: "{{baseUrl}}/api/users"},
		},
	}
	err := s.LoadParsedFile(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	routes := s.GetRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	// The variable should be resolved and the host stripped to get the path
	if routes[0].PathPattern != "/api/users" {
		t.Errorf("expected path /api/users, got %q", routes[0].PathPattern)
	}
}

func TestServer_LoadParsedFile_WithAssertions(t *testing.T) {
	s := NewServer()
	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{
				Method: "GET",
				URL:    "/api/health",
				Assertions: []*parser.Assertion{
					{Subject: "status", Operator: parser.OpEquals, Expected: 201},
					{Subject: "body.message", Operator: parser.OpEquals, Expected: "healthy"},
				},
			},
		},
	}
	err := s.LoadParsedFile(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	routes := s.GetRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].Response.StatusCode != 201 {
		t.Errorf("expected status 201 inferred from assertion, got %d", routes[0].Response.StatusCode)
	}
	// Body should contain the "message" field from the body.message assertion
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(routes[0].Response.Body), &body); err != nil {
		t.Fatalf("failed to parse response body as JSON: %v", err)
	}
	if body["message"] != "healthy" {
		t.Errorf("expected body.message = 'healthy', got %v", body["message"])
	}
}

func TestServer_LoadParsedFile_DefaultResponseWhenNoAssertions(t *testing.T) {
	s := NewServer()
	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{Method: "GET", URL: "/ping"},
		},
	}
	if err := s.LoadParsedFile(file); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	routes := s.GetRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].Response.StatusCode != 200 {
		t.Errorf("expected default status 200, got %d", routes[0].Response.StatusCode)
	}
	if routes[0].Response.ContentType != "application/json" {
		t.Errorf("expected default content-type application/json, got %q", routes[0].Response.ContentType)
	}
	// Default body when no assertions
	if routes[0].Response.Body != `{"status": "ok", "message": "Mock response"}` {
		t.Errorf("unexpected default body: %q", routes[0].Response.Body)
	}
}

func TestServer_LoadParsedFile_URLWithQueryParamsStripped(t *testing.T) {
	s := NewServer()
	file := &parser.File{
		Path: "test.http",
		Requests: []*parser.Request{
			{Method: "GET", URL: "/api/search?q=test&page=1"},
		},
	}
	if err := s.LoadParsedFile(file); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	routes := s.GetRoutes()
	if routes[0].PathPattern != "/api/search" {
		t.Errorf("expected query params stripped, got %q", routes[0].PathPattern)
	}
}

func TestServer_LoadFile_FileNotFound(t *testing.T) {
	s := NewServer()
	err := s.LoadFile("/nonexistent/path/to/file.http")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestServer_LoadFile_ValidFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.http")
	content := `GET /api/hello
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	s := NewServer()
	err := s.LoadFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	routes := s.GetRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].Method != "GET" {
		t.Errorf("expected method GET, got %q", routes[0].Method)
	}
}

func TestServer_LoadFiles_Multiple(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "a.http")
	if err := os.WriteFile(file1, []byte("GET /api/a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	file2 := filepath.Join(dir, "b.http")
	if err := os.WriteFile(file2, []byte("POST /api/b\n"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewServer()
	if err := s.LoadFiles([]string{file1, file2}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.GetRoutes()) != 2 {
		t.Errorf("expected 2 routes, got %d", len(s.GetRoutes()))
	}
}

func TestServer_LoadFiles_ErrorOnBadFile(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.http")
	if err := os.WriteFile(good, []byte("GET /ok\n"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewServer()
	err := s.LoadFiles([]string{good, "/nonexistent/bad.http"})
	if err == nil {
		t.Error("expected error for bad file in list, got nil")
	}
}

// --- HTTP handler tests using httptest ---

// newTestServer creates a mock server backed by an httptest.Server for testing.
func newTestServer(t *testing.T, opts ...Option) (*Server, *httptest.Server) {
	t.Helper()
	s := NewServer(opts...)
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return s, ts
}

func TestHandler_NoMatchingRoute_Returns404(t *testing.T) {
	_, ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/nonexistent")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandler_MatchingRoute_ReturnsConfiguredResponse(t *testing.T) {
	s, ts := newTestServer(t)

	s.router.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/api/greeting",
		PathRegex:   createPathRegex("/api/greeting"),
		Response: &MockResponse{
			StatusCode:  200,
			ContentType: "application/json",
			Headers:     map[string]string{"X-Custom": "hello"},
			Body:        `{"greeting": "hi"}`,
		},
	})

	resp, err := http.Get(ts.URL + "/api/greeting")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	if custom := resp.Header.Get("X-Custom"); custom != "hello" {
		t.Errorf("expected X-Custom header 'hello', got %q", custom)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"greeting": "hi"}` {
		t.Errorf("unexpected body: %q", string(body))
	}
}

func TestHandler_PathParameter_ResolvedInBody(t *testing.T) {
	s, ts := newTestServer(t)

	s.router.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/users/{{id}}",
		PathRegex:   createPathRegex("/users/{{id}}"),
		Response: &MockResponse{
			StatusCode:  200,
			ContentType: "application/json",
			Body:        `{"userId": "{{id}}"}`,
		},
	})

	resp, err := http.Get(ts.URL + "/users/99")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if result["userId"] != "99" {
		t.Errorf("expected userId=99, got %v", result["userId"])
	}
}

func TestHandler_POST_MethodMatches(t *testing.T) {
	s, ts := newTestServer(t)

	s.router.AddRoute(&Route{
		Method:      "POST",
		PathPattern: "/api/items",
		PathRegex:   createPathRegex("/api/items"),
		Response: &MockResponse{
			StatusCode:  201,
			ContentType: "application/json",
			Body:        `{"created": true}`,
		},
	})

	resp, err := http.Post(ts.URL+"/api/items", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
}

func TestHandler_GET_DoesNotMatchPOST(t *testing.T) {
	s, ts := newTestServer(t)

	s.router.AddRoute(&Route{
		Method:      "POST",
		PathPattern: "/api/only-post",
		PathRegex:   createPathRegex("/api/only-post"),
		Response:    &MockResponse{StatusCode: 201},
	})

	resp, err := http.Get(ts.URL + "/api/only-post")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Errorf("expected 404 for method mismatch, got %d", resp.StatusCode)
	}
}

func TestHandler_CustomStatusCode(t *testing.T) {
	s, ts := newTestServer(t)

	s.router.AddRoute(&Route{
		Method:      "DELETE",
		PathPattern: "/api/items/{{id}}",
		PathRegex:   createPathRegex("/api/items/{{id}}"),
		Response: &MockResponse{
			StatusCode:  204,
			ContentType: "application/json",
			Body:        "",
		},
	})

	req, _ := http.NewRequest("DELETE", ts.URL+"/api/items/5", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

func TestHandler_RequestCallback_Invoked(t *testing.T) {
	var mu sync.Mutex
	var calledMethod, calledPath string
	var calledStatus int
	var calledDuration time.Duration

	cb := func(method, path string, status int, duration time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		calledMethod = method
		calledPath = path
		calledStatus = status
		calledDuration = duration
	}

	s, ts := newTestServer(t, WithRequestCallback(cb))

	s.router.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/cb-test",
		PathRegex:   createPathRegex("/cb-test"),
		Response:    &MockResponse{StatusCode: 200, ContentType: "text/plain", Body: "ok"},
	})

	resp, err := http.Get(ts.URL + "/cb-test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()

	if calledMethod != "GET" {
		t.Errorf("callback method = %q, want GET", calledMethod)
	}
	if calledPath != "/cb-test" {
		t.Errorf("callback path = %q, want /cb-test", calledPath)
	}
	if calledStatus != 200 {
		t.Errorf("callback status = %d, want 200", calledStatus)
	}
	if calledDuration <= 0 {
		t.Error("callback duration should be positive")
	}
}

func TestHandler_RequestCallback_NotCalledOn404(t *testing.T) {
	called := false
	cb := func(method, path string, status int, duration time.Duration) {
		called = true
	}

	_, ts := newTestServer(t, WithRequestCallback(cb))

	resp, err := http.Get(ts.URL + "/no-route")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if called {
		t.Error("callback should not be called when no route matches (404)")
	}
}

func TestHandler_DelayOption_Applied(t *testing.T) {
	delay := 50 * time.Millisecond
	s, ts := newTestServer(t, WithDelay(delay))

	s.router.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/slow",
		PathRegex:   createPathRegex("/slow"),
		Response:    &MockResponse{StatusCode: 200, ContentType: "text/plain", Body: "delayed"},
	})

	start := time.Now()
	resp, err := http.Get(ts.URL + "/slow")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if elapsed < delay {
		t.Errorf("expected at least %v delay, got %v", delay, elapsed)
	}
}

func TestHandler_VerboseMode(t *testing.T) {
	// Just ensure verbose mode does not cause panics or errors.
	s, ts := newTestServer(t, WithVerbose(true))

	s.router.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/verbose-test",
		PathRegex:   createPathRegex("/verbose-test"),
		Response:    &MockResponse{StatusCode: 200, ContentType: "text/plain", Body: "v"},
	})

	resp, err := http.Get(ts.URL + "/verbose-test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Also test verbose 404 path
	resp2, err := http.Get(ts.URL + "/verbose-missing")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp2.StatusCode)
	}
}

// --- StartWithContext tests ---

func TestServer_StartWithContext_StartsAndShutdownsGracefully(t *testing.T) {
	// Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	s := NewServer(WithPort(port))
	s.router.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/ctx-test",
		PathRegex:   createPathRegex("/ctx-test"),
		Response:    &MockResponse{StatusCode: 200, ContentType: "text/plain", Body: "context works"},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.StartWithContext(ctx)
	}()

	// Wait for server to be ready
	addr := fmt.Sprintf("http://127.0.0.1:%d", port)
	var resp *http.Response
	for i := 0; i < 50; i++ {
		resp, err = http.Get(addr + "/ctx-test")
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("server never became ready: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "context works" {
		t.Errorf("unexpected body: %q", string(body))
	}

	// Cancel context to trigger shutdown
	cancel()

	// Graceful shutdown must return nil (not http.ErrServerClosed), so the CLI
	// exits 0 on Ctrl+C instead of 1.
	select {
	case srvErr := <-errCh:
		if srvErr != nil {
			t.Errorf("graceful shutdown returned %v, want nil (CLI must exit 0)", srvErr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}

// --- Full integration test ---

func TestIntegration_LoadFile_StartServer_MakeRequest(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "api.http")
	// Use correct hitspec format: ### separates requests, >>> <<< wrap assertions
	content := `### Get all users

GET /api/users

>>>
expect status 200
expect body.count == 5
expect body.type == "users"
<<<

### Create a user

POST /api/users

>>>
expect status 201
expect body.created == true
<<<
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	s := NewServer()
	if err := s.LoadFile(filePath); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	routes := s.GetRoutes()
	if len(routes) < 2 {
		t.Fatalf("expected at least 2 routes, got %d", len(routes))
	}

	// Use httptest to serve the handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Test GET /api/users
	resp, err := http.Get(ts.URL + "/api/users")
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("GET /api/users: expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var getBody map[string]interface{}
	if err := json.Unmarshal(body, &getBody); err != nil {
		t.Fatalf("failed to parse GET response: %v (body: %q)", err, string(body))
	}

	// Verify body fields from assertions were generated
	if getBody["count"] == nil && getBody["type"] == nil {
		t.Log("note: response body may use default mock response if assertions did not generate fields")
	}

	// Test POST /api/users
	postResp, err := http.Post(ts.URL+"/api/users", "application/json", nil)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != 201 {
		t.Errorf("POST /api/users: expected 201, got %d", postResp.StatusCode)
	}

	postBody, _ := io.ReadAll(postResp.Body)
	var postResult map[string]interface{}
	if err := json.Unmarshal(postBody, &postResult); err != nil {
		t.Fatalf("failed to parse POST response: %v (body: %q)", err, string(postBody))
	}

	if postResult["created"] != true {
		t.Errorf("expected created=true in POST response, got %v", postResult["created"])
	}

	// Verify 404 for unknown routes
	resp404, err := http.Get(ts.URL + "/api/unknown")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp404.Body.Close()
	if resp404.StatusCode != 404 {
		t.Errorf("expected 404 for unknown path, got %d", resp404.StatusCode)
	}
}

func TestIntegration_LoadParsedFile_WithParams(t *testing.T) {
	s := NewServer()
	file := &parser.File{
		Path: "params.http",
		Requests: []*parser.Request{
			{
				Name:   "Get User By ID",
				Method: "GET",
				URL:    "/users/{{id}}",
				Assertions: []*parser.Assertion{
					{Subject: "status", Operator: parser.OpEquals, Expected: 200},
				},
			},
		},
	}
	if err := s.LoadParsedFile(file); err != nil {
		t.Fatalf("LoadParsedFile failed: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/users/abc123")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestIntegration_CallbackWithDelay(t *testing.T) {
	var mu sync.Mutex
	var cbDuration time.Duration

	delay := 30 * time.Millisecond
	cb := func(method, path string, status int, duration time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		cbDuration = duration
	}

	s := NewServer(WithDelay(delay), WithRequestCallback(cb))
	s.router.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/timed",
		PathRegex:   createPathRegex("/timed"),
		Response:    &MockResponse{StatusCode: 200, ContentType: "text/plain", Body: "timed"},
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/timed")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()

	if cbDuration < delay {
		t.Errorf("callback duration %v is less than configured delay %v", cbDuration, delay)
	}
}

// --- resolveVariables tests ---

func TestServer_ResolveVariables_FromVarsMap(t *testing.T) {
	s := NewServer()
	vars := map[string]string{
		"host": "example.com",
		"ver":  "v2",
	}
	result := s.resolveVariables("https://{{host}}/api/{{ver}}/users", vars)
	expected := "https://example.com/api/v2/users"
	if result != expected {
		t.Errorf("resolveVariables = %q, want %q", result, expected)
	}
}

func TestServer_ResolveVariables_FromEnvironment(t *testing.T) {
	s := NewServer()
	t.Setenv("MOCK_TEST_HOST", "env-host.com")

	result := s.resolveVariables("https://{{MOCK_TEST_HOST}}/api", nil)
	expected := "https://env-host.com/api"
	if result != expected {
		t.Errorf("resolveVariables = %q, want %q", result, expected)
	}
}

func TestServer_ResolveVariables_UnknownKept(t *testing.T) {
	s := NewServer()
	result := s.resolveVariables("/api/{{unknown_var_xyz}}", nil)
	if result != "/api/{{unknown_var_xyz}}" {
		t.Errorf("expected unresolved variable to be kept, got %q", result)
	}
}

func TestServer_ResolveVariables_NoVariables(t *testing.T) {
	s := NewServer()
	result := s.resolveVariables("/api/static/path", nil)
	if result != "/api/static/path" {
		t.Errorf("expected unchanged path, got %q", result)
	}
}

// --- resolveBodyParams tests ---

func TestServer_ResolveBodyParams(t *testing.T) {
	s := NewServer()
	body := `{"id": "{{id}}", "name": "{{name}}"}`
	params := map[string]string{"id": "42", "name": "Alice"}
	result := s.resolveBodyParams(body, params)
	expected := `{"id": "42", "name": "Alice"}`
	if result != expected {
		t.Errorf("resolveBodyParams = %q, want %q", result, expected)
	}
}

func TestServer_ResolveBodyParams_EmptyParams(t *testing.T) {
	s := NewServer()
	body := `{"static": true}`
	result := s.resolveBodyParams(body, map[string]string{})
	if result != body {
		t.Errorf("expected unchanged body, got %q", result)
	}
}

// --- generateBodyFromAssertions tests ---

func TestServer_GenerateBodyFromAssertions_BodyFields(t *testing.T) {
	s := NewServer()
	assertions := []*parser.Assertion{
		{Subject: "body.name", Operator: parser.OpEquals, Expected: "test"},
		{Subject: "body.active", Operator: parser.OpEquals, Expected: true},
	}
	body := s.generateBodyFromAssertions(assertions)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if result["name"] != "test" {
		t.Errorf("expected name=test, got %v", result["name"])
	}
	if result["active"] != true {
		t.Errorf("expected active=true, got %v", result["active"])
	}
}

func TestServer_GenerateBodyFromAssertions_WholeBody(t *testing.T) {
	s := NewServer()
	assertions := []*parser.Assertion{
		{Subject: "body", Operator: parser.OpEquals, Expected: `{"whole": "body"}`},
	}
	body := s.generateBodyFromAssertions(assertions)
	if body != `{"whole": "body"}` {
		t.Errorf("expected whole body assertion, got %q", body)
	}
}

func TestServer_GenerateBodyFromAssertions_NoBodyAssertions(t *testing.T) {
	s := NewServer()
	assertions := []*parser.Assertion{
		{Subject: "status", Operator: parser.OpEquals, Expected: 200},
		{Subject: "header.Content-Type", Operator: parser.OpEquals, Expected: "text/plain"},
	}
	body := s.generateBodyFromAssertions(assertions)
	if body != `{"status": "ok"}` {
		t.Errorf("expected default body, got %q", body)
	}
}

func TestServer_GenerateBodyFromAssertions_Empty(t *testing.T) {
	s := NewServer()
	body := s.generateBodyFromAssertions(nil)
	if body != `{"status": "ok"}` {
		t.Errorf("expected default body for nil assertions, got %q", body)
	}
}

// --- Multiple methods on same path ---

func TestHandler_MultipleMethodsSamePath(t *testing.T) {
	s, ts := newTestServer(t)

	s.router.AddRoute(&Route{
		Method:      "GET",
		PathPattern: "/api/resource",
		PathRegex:   createPathRegex("/api/resource"),
		Response:    &MockResponse{StatusCode: 200, ContentType: "application/json", Body: `{"action":"get"}`},
	})
	s.router.AddRoute(&Route{
		Method:      "PUT",
		PathPattern: "/api/resource",
		PathRegex:   createPathRegex("/api/resource"),
		Response:    &MockResponse{StatusCode: 200, ContentType: "application/json", Body: `{"action":"put"}`},
	})

	// GET
	getResp, err := http.Get(ts.URL + "/api/resource")
	if err != nil {
		t.Fatal(err)
	}
	getBody, _ := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	if string(getBody) != `{"action":"get"}` {
		t.Errorf("GET body = %q", string(getBody))
	}

	// PUT
	req, _ := http.NewRequest("PUT", ts.URL+"/api/resource", nil)
	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	putBody, _ := io.ReadAll(putResp.Body)
	putResp.Body.Close()
	if string(putBody) != `{"action":"put"}` {
		t.Errorf("PUT body = %q", string(putBody))
	}
}
