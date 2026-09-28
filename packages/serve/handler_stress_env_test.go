package serve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// writeStressWorkspace creates a workspace whose only request URL comes from a
// hitspec.yaml environment variable, which is the common real-world shape.
func writeStressWorkspace(t *testing.T, envName, baseURL string) string {
	t.Helper()
	dir := t.TempDir()

	yaml := "environments:\n  " + envName + ":\n    baseUrl: " + baseURL + "\n"
	if err := os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write hitspec.yaml: %v", err)
	}
	file := "### probe\n# @name probe\nGET {{baseUrl}}/probe\n"
	if err := os.WriteFile(filepath.Join(dir, "probe.http"), []byte(file), 0o644); err != nil {
		t.Fatalf("write probe.http: %v", err)
	}
	return dir
}

// startStressAndWait runs POST /api/v1/stress/start against s and blocks until
// the run finishes, returning the stored result.
func startStressAndWait(t *testing.T, s *Server, body string) *StressResultDTO {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stress/start", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.handleStressStart(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("stress start: expected 200, got %d (%s)", w.Code, w.Body.String())
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		result := s.lastStressResult
		s.mu.Unlock()
		if result != nil {
			return result
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("stress test did not complete in time")
	return nil
}

// TestHandleStressStart_ResolvesConfigEnvironments guards the regression where
// the API server built the stress runner without WithEnvironment /
// WithConfigEnvironments, so {{baseUrl}} never resolved and every request
// failed with "unsupported URL scheme".
func TestHandleStressStart_ResolvesConfigEnvironments(t *testing.T) {
	var hits int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()

	dir := writeStressWorkspace(t, "test", target.URL)

	s := NewServer(WithWorkDir(dir), WithEnv("test"), WithLogLevel("error"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.ctx, s.cancel = ctx, cancel

	result := startStressAndWait(t, s, `{"files":["probe.http"],"duration":"400ms","rate":20}`)

	if result.Total == 0 {
		t.Errorf("total = 0, expected the stress runner to issue requests")
	}
	if result.Errors != 0 {
		t.Errorf("errors = %d, want 0 — {{baseUrl}} did not resolve", result.Errors)
	}
	if result.Success == 0 {
		t.Errorf("success = 0, want > 0")
	}
	if atomic.LoadInt32(&hits) == 0 {
		t.Error("target server never received a request")
	}
}

// TestHandleStressStart_RequestEnvironmentOverride covers the optional
// `environment` field: a client (the desktop app) can stress a different
// environment than the server's active one.
func TestHandleStressStart_RequestEnvironmentOverride(t *testing.T) {
	var defaultHits, overrideHits int32

	defaultTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&defaultHits, 1)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer defaultTarget.Close()

	overrideTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&overrideHits, 1)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer overrideTarget.Close()

	dir := t.TempDir()
	yaml := "environments:\n" +
		"  dev:\n    baseUrl: " + defaultTarget.URL + "\n" +
		"  staging:\n    baseUrl: " + overrideTarget.URL + "\n"
	if err := os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write hitspec.yaml: %v", err)
	}
	file := "### probe\n# @name probe\nGET {{baseUrl}}/probe\n"
	if err := os.WriteFile(filepath.Join(dir, "probe.http"), []byte(file), 0o644); err != nil {
		t.Fatalf("write probe.http: %v", err)
	}

	s := NewServer(WithWorkDir(dir), WithEnv("dev"), WithLogLevel("error"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.ctx, s.cancel = ctx, cancel

	result := startStressAndWait(t, s,
		`{"files":["probe.http"],"duration":"300ms","rate":20,"environment":"staging"}`)

	if result.Errors != 0 {
		t.Errorf("errors = %d, want 0", result.Errors)
	}
	if atomic.LoadInt32(&overrideHits) == 0 {
		t.Error("staging target received no traffic — environment override was ignored")
	}
	if got := atomic.LoadInt32(&defaultHits); got != 0 {
		t.Errorf("dev target received %d requests, want 0", got)
	}
}
