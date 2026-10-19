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
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
)

// PatchConfig can do what PutConfig's zero-means-unchanged DTO cannot: set
// retries to 0 and clear the proxy. retryDelay and maxRedirects are exposed.
func TestPatchConfigExplicitZeroAndNewFields(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "hitspec.yaml", "retries: 3\nproxy: http://proxy.local:8080\ntimeout: 12000\n")
	m := newTestManagerAt(t, dir)
	ctx := context.Background()

	zero, empty, delay, redirects := 0, "", 250, 4
	got, err := m.PatchConfig(ctx, config.Patch{Retries: &zero, Proxy: &empty, RetryDelay: &delay, MaxRedirects: &redirects})
	if err != nil {
		t.Fatal(err)
	}
	if got.Retries != 0 || got.Proxy != "" || got.RetryDelay != 250 || got.MaxRedirects != 4 || got.Timeout != 12000 {
		t.Fatalf("PatchConfig = %+v", got)
	}
	disk, _ := os.ReadFile(filepath.Join(dir, "hitspec.yaml"))
	if strings.Contains(string(disk), "retries") || strings.Contains(string(disk), "proxy") || !strings.Contains(string(disk), "retryDelay: 250") {
		t.Fatalf("hitspec.yaml:\n%s", disk)
	}

	neg := -2
	if _, err := m.PatchConfig(ctx, config.Patch{Concurrency: &neg}); err == nil {
		t.Fatal("negative concurrency accepted")
	}
	if cfg, _ := m.GetConfig(ctx); cfg.RetryDelay != 250 {
		t.Fatalf("a rejected patch changed the config: %+v", cfg)
	}

	// PutConfig keeps its zero-means-unchanged contract (the TUI relies on it).
	if _, err := m.PutConfig(ctx, ConfigDTO{Timeout: 7000, MaxRedirects: 2}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := m.GetConfig(ctx)
	if cfg.Timeout != 7000 || cfg.MaxRedirects != 2 || cfg.RetryDelay != 250 {
		t.Fatalf("PutConfig = %+v", cfg)
	}
}

// A read-only manager still runs requests (and records them in history), but
// protects the workspace, settings and cookie store.
func TestReadOnlyManagerRunsButDoesNotWrite(t *testing.T) {
	srv := okServer(t)
	dir := t.TempDir()
	writeFile(t, dir, "api.http", fmt.Sprintf("### ping\n# @name ping\nGET %s/ping\n\n>>>\nexpect status 200\n<<<\n", srv.URL))
	m := newTestManagerAt(t, dir)
	_ = m.Close()
	m = New(WithWorkDir(dir), WithWatch(false), WithReadOnly(true), WithHistoryDBPath(filepath.Join(dir, "ro-history.db")))
	t.Cleanup(func() { _ = m.Close() })
	ctx := context.Background()

	if res, err := m.RunFile(ctx, RunReq{File: "api.http"}); err != nil || res.Passed != 1 {
		t.Fatalf("RunFile in read-only mode = %+v, %v", res, err)
	}
	if res, err := m.Execute(ctx, ExecuteReq{File: "api.http", RequestName: "ping"}); err != nil || res.Passed != 1 {
		t.Fatalf("Execute in read-only mode = %+v, %v", res, err)
	}
	if res, err := m.ExecuteAdHoc(ctx, AdHocReq{Method: "GET", URL: srv.URL + "/adhoc"}); err != nil || res.Passed != 1 {
		t.Fatalf("ExecuteAdHoc in read-only mode = %+v, %v", res, err)
	}
	if l, err := m.ListRuns(ctx, 10, 0); err != nil || l.Total != 2 {
		t.Fatalf("history after two persisted runs = %+v, %v", l, err)
	}

	if _, err := m.CreateFile(ctx, "new.http", "GET http://x\n"); err == nil {
		t.Error("CreateFile allowed in read-only mode")
	}
	if _, err := m.PutConfig(ctx, ConfigDTO{Timeout: 1}); err == nil {
		t.Error("PutConfig allowed in read-only mode")
	}
	if err := m.PutStressProfile(ctx, StressProfileReq{Name: "p"}); err == nil {
		t.Error("PutStressProfile allowed in read-only mode")
	}
	if _, err := m.PutCookie(ctx, CookieDTO{Domain: "x", Name: "a", Value: "b"}); err == nil {
		t.Error("PutCookie allowed in read-only mode")
	}
	if err := m.ClearRuns(ctx); err == nil {
		t.Error("ClearRuns allowed in read-only mode")
	}
	if _, err := os.Stat(filepath.Join(dir, "new.http")); err == nil {
		t.Error("a file was created")
	}
	if _, err := os.Stat(filepath.Join(dir, "hitspec.yaml")); err == nil {
		t.Error("hitspec.yaml was created")
	}
}

// Set-Cookie headers from a run are not written to the cookie store by a
// read-only manager.
func TestReadOnlyManagerDoesNotPersistCookies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "1", Path: "/"})
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()
	dir := t.TempDir()
	writeFile(t, dir, "api.http", fmt.Sprintf("### c\n# @name c\nGET %s/c\n", srv.URL))

	ro := New(WithWorkDir(dir), WithWatch(false), WithReadOnly(true), WithHistoryDBPath(filepath.Join(dir, "h.db")))
	defer ro.Close()
	if _, err := ro.RunFile(context.Background(), RunReq{File: "api.http"}); err != nil {
		t.Fatal(err)
	}
	if cookies, _ := ro.ListCookies(context.Background()); len(cookies) != 0 {
		t.Fatalf("read-only run stored cookies: %+v", cookies)
	}

	rw := New(WithWorkDir(dir), WithWatch(false), WithHistoryDBPath(filepath.Join(dir, "h2.db")))
	defer rw.Close()
	if _, err := rw.RunFile(context.Background(), RunReq{File: "api.http"}); err != nil {
		t.Fatal(err)
	}
	if cookies, _ := rw.ListCookies(context.Background()); len(cookies) != 1 {
		t.Fatalf("a normal run should store the cookie, got %+v", cookies)
	}
}

// `< file` bodies, in saved files and in ad-hoc requests, cannot read outside
// the workspace.
func TestBodyFileConfinedToWorkspace(t *testing.T) {
	var mu sync.Mutex
	var received []string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, string(b))
		mu.Unlock()
		fmt.Fprint(w, "ok")
	}))
	defer backend.Close()

	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("TOP-SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeFile(t, dir, "ok.json", `{"fine":true}`)
	writeFile(t, dir, "inside.http", "### inside\n# @name inside\nPOST "+backend.URL+"/x\n\n< ./ok.json\n\n>>>\nexpect status 200\n<<<\n")
	writeFile(t, dir, "escape.http", "### escape\n# @name escape\nPOST "+backend.URL+"/x\n\n< "+secret+"\n\n>>>\nexpect status 200\n<<<\n")
	m := newTestManagerAt(t, dir)
	ctx := context.Background()

	if res, err := m.RunFile(ctx, RunReq{File: "inside.http"}); err != nil || res.Passed != 1 {
		t.Fatalf("inside.http = %+v, %v", res, err)
	}
	for name, run := range map[string]func() (*RunResultDTO, error){
		"RunFile": func() (*RunResultDTO, error) { return m.RunFile(ctx, RunReq{File: "escape.http"}) },
		"Execute": func() (*RunResultDTO, error) {
			return m.Execute(ctx, ExecuteReq{File: "escape.http", RequestName: "escape"})
		},
		"AdHoc": func() (*RunResultDTO, error) {
			return m.ExecuteAdHoc(ctx, AdHocReq{Method: "POST", URL: backend.URL + "/x", Body: "< " + secret})
		},
	} {
		res, err := run()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if res.Failed != 1 || len(res.Results) != 1 || !strings.Contains(res.Results[0].Error, "outside the workspace") {
			t.Fatalf("%s = %+v; want a failed request with an outside-the-workspace error", name, res)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for _, b := range received {
		if strings.Contains(b, "TOP-SECRET") {
			t.Fatalf("a file outside the workspace reached the backend: %q", b)
		}
	}
	if len(received) != 1 {
		t.Fatalf("backend saw %d requests, want only inside.http's: %q", len(received), received)
	}
}
