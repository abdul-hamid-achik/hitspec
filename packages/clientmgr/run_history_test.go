package clientmgr

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// waitFor polls cond until it returns true or the timeout elapses.
func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", timeout, what)
}

// okServer returns a server that answers 200 {"ok":true} to any request.
func okServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// writeRunnableFile creates a one-request .http file pointing at url.
func writeRunnableFile(t *testing.T, m *Manager, rel, url string) {
	t.Helper()
	content := fmt.Sprintf("### ping\n# @name ping\nGET %s/ping\n\n>>>\nexpect status 200\n<<<\n", url)
	if _, err := m.CreateFile(context.Background(), rel, content); err != nil {
		t.Fatalf("create %s: %v", rel, err)
	}
}

func TestExecuteAndRunFile(t *testing.T) {
	ctx := context.Background()
	srv := okServer(t)
	m := newTestManager(t)
	writeRunnableFile(t, m, "api.http", srv.URL)

	// Execute a single named request.
	res, err := m.Execute(ctx, ExecuteReq{File: "api.http", RequestName: "ping"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Passed != 1 || res.Failed != 0 || len(res.Results) != 1 {
		t.Fatalf("execute result = %+v, want 1 passed", res)
	}

	// Run the whole file.
	full, err := m.RunFile(ctx, RunReq{File: "api.http"})
	if err != nil {
		t.Fatalf("RunFile: %v", err)
	}
	if full.Passed != 1 {
		t.Fatalf("runfile passed = %d, want 1", full.Passed)
	}
}

// TestExecuteByIndex guards the regression where running a single untitled
// request from the studio TUI was a silent no-op (the "line N" display name
// can't match the runner's name filter). Execute must run the request at the
// given source index and only that one.
func TestExecuteByIndex(t *testing.T) {
	ctx := context.Background()
	var hit []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hit = append(hit, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	m := newTestManager(t)
	content := fmt.Sprintf("### First\nGET %s/first\n\n### Second\nGET %s/second\n", srv.URL, srv.URL)
	if _, err := m.CreateFile(ctx, "api.http", content); err != nil {
		t.Fatalf("create: %v", err)
	}

	idx := 1
	res, err := m.Execute(ctx, ExecuteReq{File: "api.http", RequestIndex: &idx})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Passed != 1 || len(res.Results) != 1 {
		t.Fatalf("execute result = %+v, want exactly 1 passed", res)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(hit) != 1 || hit[0] != "/second" {
		t.Fatalf("hits = %v, want only [/second]", hit)
	}
}

func TestExportRequestAsCurl(t *testing.T) {
	ctx := context.Background()
	srv := okServer(t)
	m := newTestManager(t)
	writeRunnableFile(t, m, "api.http", srv.URL)

	out, err := m.Export(ctx, ExportReq{File: "api.http", RequestName: "ping", Format: "curl"})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(out.Commands) == 0 {
		t.Fatal("expected at least one exported command")
	}
}

func TestRunFilePersistsHistoryCRUD(t *testing.T) {
	ctx := context.Background()
	srv := okServer(t)
	m := newTestManager(t)
	writeRunnableFile(t, m, "api.http", srv.URL)

	if _, err := m.RunFile(ctx, RunReq{File: "api.http"}); err != nil {
		t.Fatalf("RunFile: %v", err)
	}

	// The run is committed, with every result, before RunFile returns: no
	// polling.
	list, err := m.ListRuns(ctx, 30, 0)
	if err != nil || list.Total != 1 || len(list.Runs) != 1 {
		t.Fatalf("ListRuns right after RunFile = %+v, err %v; want exactly the finished run", list, err)
	}
	run, err := m.GetRun(ctx, list.Runs[0].ID)
	if err != nil || len(run.Results) == 0 || run.FinishedAt == "" || run.Total != int64(len(run.Results)) {
		t.Fatalf("GetRun right after RunFile = %+v, err %v; want all results and a finished run", run, err)
	}
	id := run.ID

	if err := m.DeleteRun(ctx, id); err != nil {
		t.Fatalf("DeleteRun: %v", err)
	}
	if _, err := m.GetRun(ctx, id); err == nil {
		t.Fatal("GetRun after delete should error")
	}

	// Re-run, then clear all.
	if _, err := m.RunFile(ctx, RunReq{File: "api.http"}); err != nil {
		t.Fatalf("RunFile (2): %v", err)
	}
	if l, _ := m.ListRuns(ctx, 30, 0); l.Total != 1 {
		t.Fatalf("second run not persisted before RunFile returned: total %d", l.Total)
	}
	if err := m.ClearRuns(ctx); err != nil {
		t.Fatalf("ClearRuns: %v", err)
	}
	after, err := m.ListRuns(ctx, 30, 0)
	if err != nil {
		t.Fatalf("ListRuns after clear: %v", err)
	}
	if after.Total != 0 {
		t.Fatalf("after ClearRuns total = %d, want 0", after.Total)
	}
}
