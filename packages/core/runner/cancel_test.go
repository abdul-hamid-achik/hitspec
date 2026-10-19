package runner

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func writeHTTPFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "run.http")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// slowServer records the paths it saw and blocks each request until the client
// goes away (or 10s pass).
func slowServer(t *testing.T) (srv *httptest.Server, started chan string, seen func() []string) {
	t.Helper()
	var mu sync.Mutex
	var paths []string
	started = make(chan string, 8)
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		started <- r.URL.Path
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	t.Cleanup(srv.Close)
	return srv, started, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), paths...)
	}
}

func baseConfig() *Config {
	return &Config{Environment: "dev", FollowRedirect: true, ValidateSSL: true, Timeout: time.Minute}
}

// TestCancelStopsRunAfterInFlightRequest: cancelling mid-run aborts the request
// in flight promptly, never starts the next request, and still returns what
// ran, marked cancelled.
func TestCancelStopsRunAfterInFlightRequest(t *testing.T) {
	srv, started, seen := slowServer(t)
	path := writeHTTPFile(t, "### first\n# @name first\nGET "+srv.URL+"/first\n\n### second\n# @name second\nGET "+srv.URL+"/second\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type outcome struct {
		result *RunResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := NewRunner(baseConfig()).RunFileContext(ctx, path)
		done <- outcome{result, err}
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first request never started")
	}
	cancelledAt := time.Now()
	cancel()

	var got outcome
	select {
	case got = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunFileContext did not return after cancel")
	}
	if elapsed := time.Since(cancelledAt); elapsed > time.Second {
		t.Errorf("cancel took %v to take effect", elapsed)
	}
	if got.err != nil {
		t.Fatalf("a cancelled run returns its partial result, not an error: %v", got.err)
	}
	r := got.result
	if !r.Cancelled {
		t.Error("RunResult.Cancelled not set")
	}
	if len(r.Results) != 2 {
		t.Fatalf("want 2 results (one aborted, one never started), got %d", len(r.Results))
	}
	first, second := r.Results[0], r.Results[1]
	if first.Passed || !errors.Is(first.Error, context.Canceled) {
		t.Errorf("in-flight request: passed=%v err=%v, want a context.Canceled failure", first.Passed, first.Error)
	}
	if !second.Skipped || second.SkipReason != SkipReasonCancelled {
		t.Errorf("second request: skipped=%v reason=%q, want skipped %q", second.Skipped, second.SkipReason, SkipReasonCancelled)
	}
	if r.Failed != 1 || r.Skipped != 1 || r.Passed != 0 {
		t.Errorf("counts passed/failed/skipped = %d/%d/%d, want 0/1/1", r.Passed, r.Failed, r.Skipped)
	}
	if paths := seen(); len(paths) != 1 || paths[0] != "/first" {
		t.Errorf("server saw %v, want only /first", paths)
	}
}

// TestCancelledBeforeStart: an already-cancelled context starts nothing.
func TestCancelledBeforeStart(t *testing.T) {
	srv, _, seen := slowServer(t)
	path := writeHTTPFile(t, "### a\nGET "+srv.URL+"/a\n\n### b\nGET "+srv.URL+"/b\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r, err := NewRunner(baseConfig()).RunFileContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Cancelled || r.Skipped != 2 || len(r.Results) != 2 {
		t.Fatalf("cancelled=%v skipped=%d results=%d, want cancelled with 2 skipped", r.Cancelled, r.Skipped, len(r.Results))
	}
	if paths := seen(); len(paths) != 0 {
		t.Errorf("server saw %v, want nothing", paths)
	}
}

// TestCancelParallelRun: requests waiting for a concurrency slot are not
// started once the context ends.
func TestCancelParallelRun(t *testing.T) {
	srv, started, seen := slowServer(t)
	path := writeHTTPFile(t, "### a\nGET "+srv.URL+"/a\n\n### b\nGET "+srv.URL+"/b\n\n### c\nGET "+srv.URL+"/c\n")
	cfg := baseConfig()
	cfg.Parallel, cfg.Concurrency = true, 1

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan *RunResult, 1)
	go func() {
		r, _ := NewRunner(cfg).RunFileContext(ctx, path)
		done <- r
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("no request started")
	}
	cancel()

	select {
	case r := <-done:
		if !r.Cancelled || len(r.Results) != 3 {
			t.Fatalf("cancelled=%v results=%d", r.Cancelled, len(r.Results))
		}
		if r.Skipped != 2 || r.Failed != 1 {
			t.Errorf("failed/skipped = %d/%d, want 1/2", r.Failed, r.Skipped)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parallel run did not return after cancel")
	}
	if paths := seen(); len(paths) != 1 {
		t.Errorf("server saw %v, want exactly one request", paths)
	}
}

// TestCancelNotMarkedWhenFinished: cancelling after everything ran does not
// relabel a complete run.
func TestCancelNotMarkedWhenFinished(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	path := writeHTTPFile(t, "### ok\nGET "+srv.URL+"\n")
	ctx, cancel := context.WithCancel(context.Background())
	r, err := NewRunner(baseConfig()).RunFileContext(ctx, path)
	cancel()
	if err != nil || r.Cancelled || r.Passed != 1 {
		t.Fatalf("err=%v cancelled=%v passed=%d", err, r != nil && r.Cancelled, r.Passed)
	}
}

// TestRunFileStillWorks: RunFile stays a wrapper around RunFileContext.
func TestRunFileStillWorks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	path := writeHTTPFile(t, "### ok\nGET "+srv.URL+"\n")
	r, err := NewRunner(baseConfig()).RunFile(path)
	if err != nil || r.Passed != 1 || r.Cancelled {
		t.Fatalf("RunFile: err=%v result=%+v", err, r)
	}
}
