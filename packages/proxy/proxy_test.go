package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// freePort returns an available TCP port.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// startBackend creates a simple httptest server that echoes back request info.
func startBackend(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]`))
	})

	mux.HandleFunc("/api/users/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1,"name":"Alice"}`))
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/users/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":3,"name":"Charlie"}`))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/plain", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello world"))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<h1>Root</h1>"))
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// startRecorderProxy creates a recorder with the given options, starts it with
// context, and waits for it to be listening. Returns the recorder, proxy base
// URL, and a cancel function.
func startRecorderProxy(t *testing.T, backend *httptest.Server, opts ...Option) (*Recorder, string, context.CancelFunc) {
	t.Helper()
	port := freePort(t)

	allOpts := []Option{
		WithPort(port),
		WithTargetURL(backend.URL),
	}
	allOpts = append(allOpts, opts...)
	rec := NewRecorder(allOpts...)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- rec.StartWithContext(ctx)
	}()

	// Wait until the proxy is accepting connections.
	proxyURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(3 * time.Second)
	for {
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("proxy did not start within timeout")
		}
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Cleanup(func() {
		cancel()
		// Drain error (expected http.ErrServerClosed).
		<-errCh
	})

	return rec, proxyURL, cancel
}

// -----------------------------------------------------------------
// Tests
// -----------------------------------------------------------------

func TestNewRecorder_Defaults(t *testing.T) {
	r := NewRecorder()

	if r.port != 8080 {
		t.Errorf("expected default port 8080, got %d", r.port)
	}
	if r.targetURL != "" {
		t.Errorf("expected empty targetURL, got %q", r.targetURL)
	}
	if r.verbose {
		t.Error("expected verbose to be false by default")
	}
	if r.deduplicate {
		t.Error("expected deduplicate to be false by default")
	}
	if len(r.recordings) != 0 {
		t.Errorf("expected 0 recordings, got %d", len(r.recordings))
	}
	if len(r.seen) != 0 {
		t.Errorf("expected empty seen map, got %d", len(r.seen))
	}
	// Default sanitized headers
	expected := []string{"Authorization", "Cookie", "X-Api-Key", "Api-Key"}
	if len(r.sanitize) != len(expected) {
		t.Fatalf("expected %d sanitize headers, got %d", len(expected), len(r.sanitize))
	}
	for i, h := range expected {
		if r.sanitize[i] != h {
			t.Errorf("sanitize[%d]: expected %q, got %q", i, h, r.sanitize[i])
		}
	}
}

func TestNewRecorder_WithOptions(t *testing.T) {
	r := NewRecorder(
		WithPort(9999),
		WithTargetURL("http://example.com"),
		WithVerbose(true),
		WithExclude([]string{"/health", "/metrics"}),
		WithSanitize([]string{"X-Secret"}),
		WithDeduplicate(true),
	)

	if r.port != 9999 {
		t.Errorf("expected port 9999, got %d", r.port)
	}
	if r.targetURL != "http://example.com" {
		t.Errorf("expected target http://example.com, got %q", r.targetURL)
	}
	if !r.verbose {
		t.Error("expected verbose to be true")
	}
	if len(r.exclude) != 2 || r.exclude[0] != "/health" || r.exclude[1] != "/metrics" {
		t.Errorf("unexpected exclude: %v", r.exclude)
	}
	if len(r.sanitize) != 1 || r.sanitize[0] != "X-Secret" {
		t.Errorf("unexpected sanitize: %v", r.sanitize)
	}
	if !r.deduplicate {
		t.Error("expected deduplicate to be true")
	}
}

func TestStartWithContext_NoTarget(t *testing.T) {
	r := NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := r.StartWithContext(ctx)
	if err == nil || !strings.Contains(err.Error(), "target URL is required") {
		t.Errorf("expected target URL required error, got: %v", err)
	}
}

func TestStart_NoTarget(t *testing.T) {
	r := NewRecorder()
	err := r.Start()
	if err == nil || !strings.Contains(err.Error(), "target URL is required") {
		t.Errorf("expected target URL required error, got: %v", err)
	}
}

func TestStartWithContext_InvalidTargetURL(t *testing.T) {
	r := NewRecorder(WithTargetURL("://bad"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := r.StartWithContext(ctx)
	if err == nil || !strings.Contains(err.Error(), "invalid target URL") {
		t.Errorf("expected invalid target URL error, got: %v", err)
	}
}

func TestProxyForwardsRequests(t *testing.T) {
	backend := startBackend(t)
	_, proxyURL, _ := startRecorderProxy(t, backend)

	resp, err := http.Get(proxyURL + "/api/users")
	if err != nil {
		t.Fatalf("GET /api/users through proxy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Alice") {
		t.Errorf("expected response to contain Alice, got %q", string(body))
	}
}

func TestRecordingCapture_GET(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend)

	resp, err := http.Get(proxyURL + "/api/users/1")
	if err != nil {
		t.Fatalf("GET through proxy: %v", err)
	}
	resp.Body.Close()

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(recordings))
	}

	r := recordings[0]
	if r.Method != "GET" {
		t.Errorf("expected method GET, got %q", r.Method)
	}
	if r.Path != "/api/users/1" {
		t.Errorf("expected path /api/users/1, got %q", r.Path)
	}
	if r.Response == nil {
		t.Fatal("expected response to be recorded")
	}
	if r.Response.StatusCode != http.StatusOK {
		t.Errorf("expected response status 200, got %d", r.Response.StatusCode)
	}
	if !strings.Contains(r.Response.Body, "Alice") {
		t.Errorf("expected response body to contain Alice, got %q", r.Response.Body)
	}
	if !strings.Contains(r.Response.ContentType, "application/json") {
		t.Errorf("expected response content-type json, got %q", r.Response.ContentType)
	}
	if r.Response.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestRecordingCapture_POST_WithBody(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend)

	reqBody := `{"name":"Charlie"}`
	resp, err := http.Post(proxyURL+"/api/users/create", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST through proxy: %v", err)
	}
	resp.Body.Close()

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(recordings))
	}

	r := recordings[0]
	if r.Method != "POST" {
		t.Errorf("expected method POST, got %q", r.Method)
	}
	if r.Body != reqBody {
		t.Errorf("expected body %q, got %q", reqBody, r.Body)
	}
	if r.ContentType != "application/json" {
		t.Errorf("expected content-type application/json, got %q", r.ContentType)
	}
	if r.Response == nil {
		t.Fatal("expected response to be recorded")
	}
	if r.Response.StatusCode != http.StatusCreated {
		t.Errorf("expected response status 201, got %d", r.Response.StatusCode)
	}
}

func TestRecordingCapture_PUT(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend)

	reqBody := `{"id":1,"name":"Alice Updated"}`
	req, _ := http.NewRequest(http.MethodPut, proxyURL+"/api/users/1", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT through proxy: %v", err)
	}
	resp.Body.Close()

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(recordings))
	}

	r := recordings[0]
	if r.Method != "PUT" {
		t.Errorf("expected method PUT, got %q", r.Method)
	}
	if r.Body != reqBody {
		t.Errorf("expected body %q, got %q", reqBody, r.Body)
	}
}

func TestRecordingCapture_DELETE(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend)

	req, _ := http.NewRequest(http.MethodDelete, proxyURL+"/api/users/1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE through proxy: %v", err)
	}
	resp.Body.Close()

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(recordings))
	}

	r := recordings[0]
	if r.Method != "DELETE" {
		t.Errorf("expected method DELETE, got %q", r.Method)
	}
	if r.Response.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", r.Response.StatusCode)
	}
}

func TestRecordingCapture_Headers(t *testing.T) {
	backend := startBackend(t)
	// Use no sanitize so we can verify custom headers are captured.
	rec, proxyURL, _ := startRecorderProxy(t, backend, WithSanitize(nil))

	req, _ := http.NewRequest(http.MethodGet, proxyURL+"/api/users", nil)
	req.Header.Set("X-Custom-Header", "custom-value")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET through proxy: %v", err)
	}
	resp.Body.Close()

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(recordings))
	}

	r := recordings[0]
	if val, ok := r.Headers["X-Custom-Header"]; !ok || val != "custom-value" {
		t.Errorf("expected X-Custom-Header=custom-value, got headers: %v", r.Headers)
	}
}

func TestGetRecordings_ReturnsCopy(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend)

	resp, err := http.Get(proxyURL + "/api/users")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	recs1 := rec.GetRecordings()
	recs2 := rec.GetRecordings()

	if len(recs1) != 1 || len(recs2) != 1 {
		t.Fatalf("expected 1 recording in each copy")
	}

	// Mutating one should not affect the other.
	recs1[0].Method = "MUTATED"
	if recs2[0].Method == "MUTATED" {
		t.Error("GetRecordings did not return a copy")
	}
}

func TestClear(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend, WithDeduplicate(true))

	// Make a request to populate recordings and seen map.
	resp, err := http.Get(proxyURL + "/api/users")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if len(rec.GetRecordings()) != 1 {
		t.Fatal("expected 1 recording before clear")
	}

	rec.Clear()

	if len(rec.GetRecordings()) != 0 {
		t.Error("expected 0 recordings after clear")
	}

	// Make the same request again. With deduplicate, if seen was not cleared
	// this would be skipped.
	resp, err = http.Get(proxyURL + "/api/users")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if len(rec.GetRecordings()) != 1 {
		t.Error("expected 1 recording after clear + new request (seen map should be cleared)")
	}
}

func TestHeaderSanitization(t *testing.T) {
	backend := startBackend(t)
	// Use default sanitize list (Authorization, Cookie, X-Api-Key, Api-Key).
	rec, proxyURL, _ := startRecorderProxy(t, backend)

	req, _ := http.NewRequest(http.MethodGet, proxyURL+"/api/users", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Cookie", "session=abc123")
	req.Header.Set("X-Api-Key", "key-12345")
	req.Header.Set("Api-Key", "another-key")
	req.Header.Set("X-Safe-Header", "visible")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(recordings))
	}

	h := recordings[0].Headers
	if h["Authorization"] != "{{AUTHORIZATION}}" {
		t.Errorf("expected Authorization to be sanitized, got %q", h["Authorization"])
	}
	if h["Cookie"] != "{{COOKIE}}" {
		t.Errorf("expected Cookie to be sanitized, got %q", h["Cookie"])
	}
	if h["X-Api-Key"] != "{{X_API_KEY}}" {
		t.Errorf("expected X-Api-Key to be sanitized, got %q", h["X-Api-Key"])
	}
	if h["Api-Key"] != "{{API_KEY}}" {
		t.Errorf("expected Api-Key to be sanitized, got %q", h["Api-Key"])
	}
	if h["X-Safe-Header"] != "visible" {
		t.Errorf("expected X-Safe-Header to remain visible, got %q", h["X-Safe-Header"])
	}
}

func TestHeaderSanitization_CaseInsensitive(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend, WithSanitize([]string{"Authorization"}))

	req, _ := http.NewRequest(http.MethodGet, proxyURL+"/api/users", nil)
	// Go's http package canonicalizes header names, so "authorization" becomes
	// "Authorization" in req.Header. The sanitize check uses EqualFold so it
	// still matches regardless.
	req.Header.Set("authorization", "Bearer secret")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(recordings))
	}

	// Go canonicalizes to "Authorization"
	if v := recordings[0].Headers["Authorization"]; v != "{{AUTHORIZATION}}" {
		t.Errorf("expected sanitized header, got %q", v)
	}
}

func TestPathExclusion(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend, WithExclude([]string{"/health"}))

	// Request to excluded path -- should still be proxied, but not recorded.
	resp, err := http.Get(proxyURL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("excluded path should still be proxied, got status %d", resp.StatusCode)
	}
	if string(body) != "ok" {
		t.Errorf("expected body 'ok', got %q", string(body))
	}

	// Request to non-excluded path.
	resp, err = http.Get(proxyURL + "/api/users")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Fatalf("expected 1 recording (only /api/users), got %d", len(recordings))
	}
	if recordings[0].Path != "/api/users" {
		t.Errorf("expected path /api/users, got %q", recordings[0].Path)
	}
}

func TestPathExclusion_ContainsMatch(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend, WithExclude([]string{"health"}))

	// The exclusion logic checks both HasPrefix and Contains, so
	// "/api/health" should also be excluded since it contains "health".
	// We need a backend that handles this path; our catchall "/" handler will
	// serve it.
	resp, err := http.Get(proxyURL + "/api/health-check")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if len(rec.GetRecordings()) != 0 {
		t.Error("expected path containing 'health' to be excluded")
	}
}

func TestDeduplication(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend, WithDeduplicate(true))

	// Make the same request three times.
	for i := 0; i < 3; i++ {
		resp, err := http.Get(proxyURL + "/api/users")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Errorf("expected 1 deduplicated recording, got %d", len(recordings))
	}
}

func TestDeduplication_DifferentMethodsSamePathBothRecorded(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend, WithDeduplicate(true))

	// GET /api/users/1
	resp, err := http.Get(proxyURL + "/api/users/1")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// DELETE /api/users/1 -- different method, should still be recorded.
	req, _ := http.NewRequest(http.MethodDelete, proxyURL+"/api/users/1", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	recordings := rec.GetRecordings()
	if len(recordings) != 2 {
		t.Errorf("expected 2 recordings (different methods), got %d", len(recordings))
	}
}

func TestDeduplication_Disabled(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend, WithDeduplicate(false))

	for i := 0; i < 3; i++ {
		resp, err := http.Get(proxyURL + "/api/users")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}

	recordings := rec.GetRecordings()
	if len(recordings) != 3 {
		t.Errorf("expected 3 recordings with dedup disabled, got %d", len(recordings))
	}
}

func TestExport_HitspecFormat(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend)

	resp, err := http.Get(proxyURL + "/api/users")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	output := rec.Export()

	if !strings.Contains(output, "# Recorded API requests") {
		t.Error("export should contain file header")
	}
	if !strings.Contains(output, "# Generated by hitspec record") {
		t.Error("export should contain generated-by comment")
	}
	if !strings.Contains(output, "GET ") {
		t.Error("export should contain GET method")
	}
	if !strings.Contains(output, "/api/users") {
		t.Error("export should contain the URL")
	}
	if !strings.Contains(output, "expect status == 200") {
		t.Error("export should contain status assertion")
	}
	if !strings.Contains(output, "expect header Content-Type contains application/json") {
		t.Error("export should contain content-type assertion for JSON response")
	}
	if !strings.Contains(output, ">>>") || !strings.Contains(output, "<<<") {
		t.Error("export should contain assertion block delimiters")
	}
}

func TestExport_WithBody(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend)

	reqBody := `{"name":"Charlie"}`
	resp, err := http.Post(proxyURL+"/api/users/create", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	output := rec.Export()

	if !strings.Contains(output, "POST ") {
		t.Error("export should contain POST method")
	}
	// The body should be pretty-printed since content-type is JSON.
	if !strings.Contains(output, `"name"`) {
		t.Error("export should contain the request body")
	}
	if !strings.Contains(output, "expect status == 201") {
		t.Error("export should contain status 201 assertion")
	}
}

func TestExportToJSON(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend)

	resp, err := http.Get(proxyURL + "/api/users/1")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	jsonBytes, err := rec.ExportToJSON()
	if err != nil {
		t.Fatalf("ExportToJSON error: %v", err)
	}

	var exported []Recording
	if err := json.Unmarshal(jsonBytes, &exported); err != nil {
		t.Fatalf("failed to unmarshal JSON export: %v", err)
	}

	if len(exported) != 1 {
		t.Fatalf("expected 1 recording in JSON, got %d", len(exported))
	}
	if exported[0].Method != "GET" {
		t.Errorf("expected method GET in JSON, got %q", exported[0].Method)
	}
	if exported[0].Path != "/api/users/1" {
		t.Errorf("expected path /api/users/1 in JSON, got %q", exported[0].Path)
	}
}

func TestExportToJSON_Empty(t *testing.T) {
	rec := NewRecorder()
	jsonBytes, err := rec.ExportToJSON()
	if err != nil {
		t.Fatalf("ExportToJSON on empty recorder: %v", err)
	}

	var exported []Recording
	if err := json.Unmarshal(jsonBytes, &exported); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(exported) != 0 {
		t.Errorf("expected 0 recordings, got %d", len(exported))
	}
}

func TestExportRecordings_Standalone(t *testing.T) {
	recordings := []Recording{
		{
			Method: "GET",
			URL:    "http://localhost:3000/api/users",
			Path:   "/api/users",
			Headers: map[string]string{
				"Accept": "application/json",
			},
			Response: &RecordedResponse{
				StatusCode:  200,
				Status:      "200 OK",
				ContentType: "application/json",
				Body:        `[{"id":1}]`,
			},
		},
		{
			Method: "POST",
			URL:    "http://localhost:3000/api/users",
			Path:   "/api/users",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body:        `{"name":"Test"}`,
			ContentType: "application/json",
			Response: &RecordedResponse{
				StatusCode:  201,
				Status:      "201 Created",
				ContentType: "application/json",
				Body:        `{"id":2,"name":"Test"}`,
			},
		},
	}

	output := ExportRecordings(recordings)

	if !strings.Contains(output, "# Recorded API requests") {
		t.Error("output should contain header")
	}
	if !strings.Contains(output, "GET http://localhost:3000/api/users") {
		t.Error("output should contain GET request line")
	}
	if !strings.Contains(output, "POST http://localhost:3000/api/users") {
		t.Error("output should contain POST request line")
	}
	if !strings.Contains(output, "expect status == 200") {
		t.Error("output should contain status 200 assertion")
	}
	if !strings.Contains(output, "expect status == 201") {
		t.Error("output should contain status 201 assertion")
	}
	// POST body should appear (pretty-printed).
	if !strings.Contains(output, `"name"`) {
		t.Error("output should contain POST body")
	}
	// Array body assertion.
	if !strings.Contains(output, "expect body type array") {
		t.Error("output should contain array body assertion")
	}
	// Object body assertion for the POST response.
	if !strings.Contains(output, "expect body.id exists") {
		t.Error("output should contain body.id exists assertion")
	}
}

func TestExportRecordings_NoResponse(t *testing.T) {
	recordings := []Recording{
		{
			Method: "GET",
			URL:    "http://localhost/test",
			Path:   "/test",
		},
	}

	output := ExportRecordings(recordings)
	if strings.Contains(output, ">>>") {
		t.Error("output should not contain assertion block when no response")
	}
}

func TestExportRecordings_SkipAutoHeaders(t *testing.T) {
	recordings := []Recording{
		{
			Method: "GET",
			URL:    "http://localhost/test",
			Path:   "/test",
			Headers: map[string]string{
				"Host":            "localhost",
				"Content-Length":  "0",
				"Accept-Encoding": "gzip",
				"Connection":      "keep-alive",
				"User-Agent":      "Go-http-client/1.1",
				"X-Custom":        "keep-me",
			},
			Response: &RecordedResponse{
				StatusCode: 200,
				Status:     "200 OK",
			},
		},
	}

	output := ExportRecordings(recordings)

	// These auto-generated headers should be skipped.
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, skip := range []string{"Host:", "Content-Length:", "Accept-Encoding:", "Connection:", "User-Agent:"} {
			if strings.HasPrefix(trimmed, skip) {
				t.Errorf("header %s should be skipped in export, but found line: %q", skip, trimmed)
			}
		}
	}

	// But X-Custom should be present.
	if !strings.Contains(output, "X-Custom: keep-me") {
		t.Error("custom header should be present in export")
	}
}

func TestExportRecordings_NonJSONBody(t *testing.T) {
	recordings := []Recording{
		{
			Method:      "POST",
			URL:         "http://localhost/test",
			Path:        "/test",
			Body:        "plain text body",
			ContentType: "text/plain",
			Response: &RecordedResponse{
				StatusCode:  200,
				ContentType: "text/plain",
				Body:        "response text",
			},
		},
	}

	output := ExportRecordings(recordings)
	if !strings.Contains(output, "plain text body") {
		t.Error("non-JSON body should appear as-is")
	}
	// No JSON content-type assertion for plain text response.
	if strings.Contains(output, "expect header Content-Type contains application/json") {
		t.Error("should not have JSON content-type assertion for text/plain response")
	}
}

// -----------------------------------------------------------------
// generateName tests
// -----------------------------------------------------------------

func TestGenerateName(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{"GET", "/api/users", "List Users"},
		{"GET", "/api/users/123", "Get Users"},     // 123 is numeric, skipped in name
		{"POST", "/api/users", "Create Users"},
		{"PUT", "/api/users/1", "Update Users"},
		{"PATCH", "/api/users/1", "Patch Users"},
		{"DELETE", "/api/users/1", "Delete Users"},
		{"GET", "/v1/products", "List Products"},
		{"GET", "/v2/items/42", "Get Items"},
		{"GET", "/", "List "},                       // root path with no parts after trim
		{"OPTIONS", "/api/test", "OPTIONS Test"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.method, tt.path), func(t *testing.T) {
			got := generateName(tt.method, tt.path)
			if got != tt.want {
				t.Errorf("generateName(%q, %q) = %q, want %q", tt.method, tt.path, got, tt.want)
			}
		})
	}
}

// -----------------------------------------------------------------
// sanitizeExportName tests
// -----------------------------------------------------------------

func TestSanitizeExportName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"List Users", "List_Users"},
		{"Get User 123", "Get_User_123"},
		{"Create  Item", "Create_Item"},  // double space -> single underscore
		{"hello-world", "hello_world"},
		{"a  b  c", "a_b_c"},
		{"__leading__", "leading"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeExportName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeExportName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// -----------------------------------------------------------------
// Helper function tests
// -----------------------------------------------------------------

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"123", true},
		{"0", true},
		{"abc", false},
		{"12a", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isNumeric(tt.input); got != tt.want {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCapitalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "Hello"},
		{"Hello", "Hello"},
		{"a", "A"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := capitalize(tt.input); got != tt.want {
				t.Errorf("capitalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestShouldSkipHeader(t *testing.T) {
	skipped := []string{"Host", "Content-Length", "Accept-Encoding", "Connection", "User-Agent"}
	for _, h := range skipped {
		if !shouldSkipHeader(h) {
			t.Errorf("shouldSkipHeader(%q) should be true", h)
		}
		// Case-insensitive check.
		if !shouldSkipHeader(strings.ToLower(h)) {
			t.Errorf("shouldSkipHeader(%q) should be true (lowercase)", strings.ToLower(h))
		}
	}

	notSkipped := []string{"Authorization", "Content-Type", "Accept", "X-Custom"}
	for _, h := range notSkipped {
		if shouldSkipHeader(h) {
			t.Errorf("shouldSkipHeader(%q) should be false", h)
		}
	}
}

func TestFormatBody_JSON(t *testing.T) {
	body := `{"name":"test","id":1}`
	result := formatBody(body, "application/json")

	// Should be pretty-printed.
	if !strings.Contains(result, "  ") {
		t.Error("JSON body should be pretty-printed with indentation")
	}
	if !strings.Contains(result, `"name"`) {
		t.Error("formatted body should contain the name field")
	}
}

func TestFormatBody_InvalidJSON(t *testing.T) {
	body := `not json`
	result := formatBody(body, "application/json")
	if result != body {
		t.Errorf("invalid JSON body should be returned as-is, got %q", result)
	}
}

func TestFormatBody_PlainText(t *testing.T) {
	body := "hello world"
	result := formatBody(body, "text/plain")
	if result != body {
		t.Errorf("plain text body should be returned as-is, got %q", result)
	}
}

func TestGenerateBodyAssertions_Object(t *testing.T) {
	body := `{"id":1,"name":"test"}`
	result := generateBodyAssertions(body)

	if !strings.Contains(result, "expect body.id exists") {
		t.Error("should assert body.id exists")
	}
	if !strings.Contains(result, "expect body.name exists") {
		t.Error("should assert body.name exists")
	}
}

func TestGenerateBodyAssertions_Array(t *testing.T) {
	body := `[{"id":1},{"id":2}]`
	result := generateBodyAssertions(body)

	if !strings.Contains(result, "expect body type array") {
		t.Error("should assert body type array")
	}
	if !strings.Contains(result, "expect body length >= 2") {
		t.Error("should assert body length >= 2")
	}
}

func TestGenerateBodyAssertions_EmptyString(t *testing.T) {
	result := generateBodyAssertions("")
	if result != "" {
		t.Errorf("expected empty string for empty body, got %q", result)
	}
}

func TestGenerateBodyAssertions_InvalidJSON(t *testing.T) {
	result := generateBodyAssertions("not json")
	if result != "" {
		t.Errorf("expected empty string for invalid JSON, got %q", result)
	}
}

func TestGenerateBodyAssertions_EmptyArray(t *testing.T) {
	result := generateBodyAssertions("[]")
	if !strings.Contains(result, "expect body type array") {
		t.Error("should assert body type array for empty array")
	}
	// Empty array should not have length assertion.
	if strings.Contains(result, "expect body length") {
		t.Error("should not have length assertion for empty array")
	}
}

// -----------------------------------------------------------------
// Concurrency
// -----------------------------------------------------------------

func TestConcurrentRecording(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend)

	const numRequests = 20
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			resp, err := http.Get(proxyURL + "/api/users")
			if err != nil {
				t.Errorf("concurrent GET: %v", err)
				return
			}
			resp.Body.Close()
		}()
	}

	wg.Wait()

	recordings := rec.GetRecordings()
	if len(recordings) != numRequests {
		t.Errorf("expected %d recordings, got %d", numRequests, len(recordings))
	}
}

func TestConcurrentRecording_WithDeduplication(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend, WithDeduplicate(true))

	const numRequests = 20
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			resp, err := http.Get(proxyURL + "/api/users")
			if err != nil {
				t.Errorf("concurrent GET: %v", err)
				return
			}
			resp.Body.Close()
		}()
	}

	wg.Wait()

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Errorf("expected 1 deduplicated recording from %d concurrent requests, got %d", numRequests, len(recordings))
	}
}

// -----------------------------------------------------------------
// Verbose mode (just ensure it doesn't panic)
// -----------------------------------------------------------------

func TestVerboseMode(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend,
		WithVerbose(true),
		WithExclude([]string{"/health"}),
		WithDeduplicate(true),
	)

	// Normal request (recorded + verbose log).
	resp, err := http.Get(proxyURL + "/api/users")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Duplicate (dedup + verbose log).
	resp, err = http.Get(proxyURL + "/api/users")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Excluded path (exclusion + verbose log).
	resp, err = http.Get(proxyURL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if len(rec.GetRecordings()) != 1 {
		t.Errorf("expected 1 recording, got %d", len(rec.GetRecordings()))
	}
}

// -----------------------------------------------------------------
// Multiple recordings and export ordering
// -----------------------------------------------------------------

func TestMultipleRecordings_ExportOrder(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend)

	paths := []string{"/api/users", "/api/users/1", "/plain"}
	for _, p := range paths {
		resp, err := http.Get(proxyURL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		resp.Body.Close()
	}

	recordings := rec.GetRecordings()
	if len(recordings) != 3 {
		t.Fatalf("expected 3 recordings, got %d", len(recordings))
	}

	// Verify ordering matches request order.
	for i, p := range paths {
		if recordings[i].Path != p {
			t.Errorf("recording[%d] path: expected %q, got %q", i, p, recordings[i].Path)
		}
	}

	// Export should contain all three.
	output := rec.Export()
	for _, p := range paths {
		if !strings.Contains(output, p) {
			t.Errorf("export should contain path %q", p)
		}
	}
}

// -----------------------------------------------------------------
// Export with @name annotation
// -----------------------------------------------------------------

func TestExport_ContainsNameAnnotation(t *testing.T) {
	recordings := []Recording{
		{
			Method: "GET",
			URL:    "http://localhost/api/users",
			Path:   "/api/users",
			Response: &RecordedResponse{
				StatusCode: 200,
			},
		},
	}

	output := ExportRecordings(recordings)

	if !strings.Contains(output, "# @name ") {
		t.Error("export should contain @name annotation")
	}
	if !strings.Contains(output, "### ") {
		t.Error("export should contain ### separator")
	}
}

// -----------------------------------------------------------------
// sortedHeaders
// -----------------------------------------------------------------

func TestSortedHeaders(t *testing.T) {
	headers := map[string]string{
		"Zebra":   "z",
		"Alpha":   "a",
		"Middle":  "m",
	}

	pairs := sortedHeaders(headers)
	if len(pairs) != 3 {
		t.Fatalf("expected 3 pairs, got %d", len(pairs))
	}
	if pairs[0].key != "Alpha" {
		t.Errorf("first key should be Alpha, got %q", pairs[0].key)
	}
	if pairs[1].key != "Middle" {
		t.Errorf("second key should be Middle, got %q", pairs[1].key)
	}
	if pairs[2].key != "Zebra" {
		t.Errorf("third key should be Zebra, got %q", pairs[2].key)
	}
}

// -----------------------------------------------------------------
// Timestamp is set
// -----------------------------------------------------------------

func TestRecordingTimestamp(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend)

	before := time.Now()
	resp, err := http.Get(proxyURL + "/api/users")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	after := time.Now()

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Fatal("expected 1 recording")
	}

	ts := recordings[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Errorf("timestamp %v should be between %v and %v", ts, before, after)
	}
}

// -----------------------------------------------------------------
// Response status string
// -----------------------------------------------------------------

func TestRecordingResponseStatus(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend)

	resp, err := http.Get(proxyURL + "/api/users")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Fatal("expected 1 recording")
	}
	if recordings[0].Response.Status != "200 OK" {
		t.Errorf("expected status '200 OK', got %q", recordings[0].Response.Status)
	}
}

// -----------------------------------------------------------------
// URL field includes full URL
// -----------------------------------------------------------------

func TestRecordingURL(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend)

	resp, err := http.Get(proxyURL + "/api/users?page=1&limit=10")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Fatal("expected 1 recording")
	}

	if !strings.Contains(recordings[0].URL, "/api/users") {
		t.Errorf("URL should contain path, got %q", recordings[0].URL)
	}
	if !strings.Contains(recordings[0].URL, "page=1") {
		t.Errorf("URL should contain query params, got %q", recordings[0].URL)
	}
}

// -----------------------------------------------------------------
// Custom sanitize headers
// -----------------------------------------------------------------

func TestCustomSanitizeHeaders(t *testing.T) {
	backend := startBackend(t)
	rec, proxyURL, _ := startRecorderProxy(t, backend,
		WithSanitize([]string{"X-Secret-Token"}),
	)

	req, _ := http.NewRequest(http.MethodGet, proxyURL+"/api/users", nil)
	req.Header.Set("X-Secret-Token", "super-secret")
	req.Header.Set("Authorization", "Bearer visible") // Not in custom sanitize list.

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Fatal("expected 1 recording")
	}

	h := recordings[0].Headers
	if h["X-Secret-Token"] != "{{X_SECRET_TOKEN}}" {
		t.Errorf("X-Secret-Token should be sanitized, got %q", h["X-Secret-Token"])
	}
	// Authorization should NOT be sanitized since custom sanitize only includes X-Secret-Token.
	if h["Authorization"] == "{{AUTHORIZATION}}" {
		t.Error("Authorization should not be sanitized with custom sanitize list")
	}
}

// -----------------------------------------------------------------
// Response headers are sanitized too
// -----------------------------------------------------------------

func TestResponseHeadersSanitized(t *testing.T) {
	// Create a backend that returns a sensitive header in the response.
	sensitiveBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Api-Key", "server-api-key")
		w.Header().Set("X-Safe", "visible")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer sensitiveBackend.Close()

	port := freePort(t)
	rec := NewRecorder(
		WithPort(port),
		WithTargetURL(sensitiveBackend.URL),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- rec.StartWithContext(ctx) }()

	proxyURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	resp, err := http.Get(proxyURL + "/test")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	cancel()
	<-errCh

	recordings := rec.GetRecordings()
	if len(recordings) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(recordings))
	}

	respHeaders := recordings[0].Response.Headers
	if respHeaders["X-Api-Key"] != "{{X_API_KEY}}" {
		t.Errorf("response X-Api-Key should be sanitized, got %q", respHeaders["X-Api-Key"])
	}
	if respHeaders["X-Safe"] != "visible" {
		t.Errorf("response X-Safe should be visible, got %q", respHeaders["X-Safe"])
	}
}
