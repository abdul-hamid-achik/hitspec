package clientmgr

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// countingServer counts the requests it receives on any path.
func countingServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// envManager returns a Manager whose hitspec.yaml has a "dev" and a "qa"
// environment, each pointing baseUrl at its own server, with "dev" as the
// default. extra is appended to the config verbatim.
func envManager(t *testing.T, devURL, qaURL string) *Manager {
	t.Helper()
	dir := t.TempDir()
	cfg := fmt.Sprintf("defaultEnvironment: dev\nenvironments:\n  dev:\n    baseUrl: %s\n    apiKey: dev-key\n  qa:\n    baseUrl: %s\n    apiKey: qa-key\n", devURL, qaURL)
	if err := os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return newTestManagerAt(t, dir)
}

func newTestManagerAt(t *testing.T, dir string) *Manager {
	t.Helper()
	m := New(WithWorkDir(dir), WithWatch(false), WithHistoryDBPath(filepath.Join(dir, "history.db")))
	m.Start(context.Background())
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// swapStdio redirects os.Stdout and os.Stderr into pipes, drained in the
// background, until the returned restore func is called. restore returns what
// was written. Nothing may log (t.Log under -v writes to stdout) between swap
// and restore.
func swapStdio(t *testing.T) (restore func() string) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = wOut, wErr
	var wg sync.WaitGroup
	var outBuf, errBuf []byte
	wg.Add(2)
	go func() { defer wg.Done(); outBuf, _ = io.ReadAll(rOut) }()
	go func() { defer wg.Done(); errBuf, _ = io.ReadAll(rErr) }()
	var once sync.Once
	var captured string
	return func() string {
		once.Do(func() {
			os.Stdout, os.Stderr = origOut, origErr
			_ = wOut.Close()
			_ = wErr.Close()
			wg.Wait()
			_ = rOut.Close()
			_ = rErr.Close()
			captured = string(outBuf) + string(errBuf)
		})
		return captured
	}
}

func waitStressDone(t *testing.T, m *Manager) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := m.StressResult(context.Background()); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Error("timed out waiting for the stress run to finish")
}

// A stress run started by the studio must not write to the terminal: the TUI
// owns the screen and gets progress through stress_update events instead.
func TestStartStress_WritesNothingToStdio(t *testing.T) {
	srv, hits := countingServer(t)
	m := envManager(t, srv.URL, srv.URL)
	writeFile(t, m.config.WorkDir, "s.http", "### hit\nGET {{baseUrl}}/ping\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := m.Subscribe(ctx)

	restore := swapStdio(t)
	defer restore()
	err := m.StartStress(context.Background(), StressStartReq{Files: []string{"s.http"}, Duration: "1s", Rate: 20})
	if err == nil {
		waitStressDone(t, m)
	}
	out := restore()

	if err != nil {
		t.Fatalf("StartStress: %v", err)
	}
	if out != "" {
		t.Errorf("stress run wrote to stdout/stderr:\n%s", out)
	}
	if hits.Load() == 0 {
		t.Error("stress run sent no requests")
	}

	// Progress still reaches subscribers (the TUI) as stress_update events.
	gotUpdate := false
drain:
	for {
		select {
		case ev := <-events:
			if ev.Type == "stress_update" {
				gotUpdate = true
				break drain
			}
		default:
			break drain
		}
	}
	if !gotUpdate {
		t.Error("no stress_update event was published")
	}
}

// Stress must resolve {{vars}} from the active environment, and follow
// SelectEnvironment.
func TestStartStress_ResolvesActiveEnvironment(t *testing.T) {
	dev, devHits := countingServer(t)
	qa, qaHits := countingServer(t)
	m := envManager(t, dev.URL, qa.URL)
	writeFile(t, m.config.WorkDir, "s.http", "### hit\nGET {{baseUrl}}/ping\n")
	ctx := context.Background()

	if err := m.SelectEnvironment(ctx, "qa"); err != nil {
		t.Fatal(err)
	}
	if err := m.StartStress(ctx, StressStartReq{Files: []string{"s.http"}, Duration: "1s", Rate: 20}); err != nil {
		t.Fatalf("StartStress: %v", err)
	}
	waitStressDone(t, m)
	if qaHits.Load() == 0 {
		t.Error("active environment qa: baseUrl was not resolved, the qa server got no requests")
	}
	if devHits.Load() != 0 {
		t.Errorf("default environment dev server got %d requests, want 0", devHits.Load())
	}
	res, _ := m.StressResult(ctx)
	if res != nil && res.Total > 0 && res.Errors == res.Total {
		t.Errorf("every stress request failed: %+v", res)
	}
}

// An explicit per-request environment overrides the active one.
func TestStartStress_RequestEnvironmentOverride(t *testing.T) {
	dev, devHits := countingServer(t)
	qa, qaHits := countingServer(t)
	m := envManager(t, dev.URL, qa.URL)
	writeFile(t, m.config.WorkDir, "s.http", "### hit\nGET {{baseUrl}}/ping\n")
	if err := m.StartStress(context.Background(), StressStartReq{Files: []string{"s.http"}, Duration: "1s", Rate: 20, Environment: "qa"}); err != nil {
		t.Fatal(err)
	}
	waitStressDone(t, m)
	if qaHits.Load() == 0 || devHits.Load() != 0 {
		t.Errorf("qa hits=%d dev hits=%d, want qa>0 dev=0", qaHits.Load(), devHits.Load())
	}
}

// The mock server must resolve {{baseUrl}} from the active environment so the
// route is /users, not a wildcard "/{baseUrl}/users".
func TestStartMock_ResolvesActiveEnvironment(t *testing.T) {
	m := envManager(t, "http://dev.example.test/api", "http://qa.example.test/api")
	writeFile(t, m.config.WorkDir, "m.http", "### users\nGET {{baseUrl}}/users\n\n>>>mock\n{\"who\":\"mock\"}\n<<<\n")
	ctx := context.Background()
	if err := m.SelectEnvironment(ctx, "qa"); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	status, err := m.StartMock(ctx, MockStartReq{Files: []string{"m.http"}, Port: port})
	if err != nil {
		t.Fatalf("StartMock: %v", err)
	}
	if len(status.Routes) != 1 || status.Routes[0].Path != "/api/users" {
		t.Fatalf("routes = %+v, want one route at /api/users (baseUrl resolved from env qa)", status.Routes)
	}

	var body string
	waitFor(t, 5*time.Second, "mock to answer", func() bool {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/users", port))
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		body = string(b)
		return resp.StatusCode == 200
	})
	if !strings.Contains(body, "mock") {
		t.Errorf("mock body = %q", body)
	}
}

// A variable the file defines itself still wins over the environment.
func TestStartMock_FileVariableWinsOverEnvironment(t *testing.T) {
	m := envManager(t, "http://dev.example.test/dev", "http://qa.example.test/qa")
	writeFile(t, m.config.WorkDir, "m.http", "@baseUrl = http://x.test/own\n\n### users\nGET {{baseUrl}}/users\n")
	status, err := m.StartMock(context.Background(), MockStartReq{Files: []string{"m.http"}, Port: freePort(t)})
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Routes) != 1 || status.Routes[0].Path != "/own/users" {
		t.Fatalf("routes = %+v, want /own/users", status.Routes)
	}
}

// Contract verification resolves non-baseUrl variables from the environment
// (baseUrl itself is overridden by the provider URL).
func TestVerifyContracts_ResolvesActiveEnvironment(t *testing.T) {
	var gotKey atomic.Value
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey.Store(r.Header.Get("X-Api-Key"))
		w.WriteHeader(200)
	}))
	defer provider.Close()
	m := envManager(t, "http://unused.test", "http://unused.test")
	writeFile(t, m.config.WorkDir, "c.http", "### ping\nGET {{baseUrl}}/ping\nX-Api-Key: {{apiKey}}\n\n>>>\nexpect status 200\n<<<\n")
	ctx := context.Background()
	if err := m.SelectEnvironment(ctx, "qa"); err != nil {
		t.Fatal(err)
	}
	results, err := m.VerifyContracts(ctx, ContractVerifyReq{Files: []string{"c.http"}, ProviderURL: provider.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Passed != 1 || results[0].Failed != 0 {
		t.Fatalf("results = %+v", results)
	}
	if got, _ := gotKey.Load().(string); got != "qa-key" {
		t.Errorf("provider saw X-Api-Key %q, want qa-key from the active environment", got)
	}
}
