package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/history"
)

// runtimeWorkspace creates a workspace with one runnable file against a tiny
// backend and returns the workspace dir and the backend URL.
func runtimeWorkspace(t *testing.T, files map[string]string) (dir string, backend string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	}))
	t.Cleanup(srv.Close)
	dir = t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, srv.URL
}

func postJ(t *testing.T, url string, body string, opts ...reqOpt) *http.Response {
	t.Helper()
	return do(t, http.MethodPost, url, body, append([]reqOpt{withHeader("Content-Type", "application/json")}, opts...)...)
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A run is in the history, with every result, by the time /run responds.
func TestRunIsCommittedToHistoryBeforeResponse(t *testing.T) {
	dir, backend := runtimeWorkspace(t, map[string]string{
		"api.http": "### a\n# @name a\nGET {{base}}/a\n\n>>>\nexpect status 200\n<<<\n\n### b\n# @name b\nGET {{base}}/b\n\n>>>\nexpect status 200\n<<<\n",
	})
	if err := os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte("defaultEnvironment: dev\nenvironments:\n  test:\n    base: "+backend+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, base, _ := startSecured(t, WithWorkDir(dir))

	for _, route := range []struct{ path, body string }{
		{"/api/v1/run", `{"file":"api.http"}`},
		{"/api/v1/execute", `{"file":"api.http","requestName":"a"}`},
	} {
		resp := postJ(t, base+route.path, route.body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s = %d: %s", route.path, resp.StatusCode, readBody(t, resp))
		}
		readBody(t, resp)

		var list HistoryListDTO
		lr := do(t, http.MethodGet, base+"/api/v1/history/runs?limit=1", "")
		if err := json.Unmarshal([]byte(readBody(t, lr)), &list); err != nil || len(list.Runs) != 1 {
			t.Fatalf("%s: list = %+v err %v", route.path, list, err)
		}
		var run HistoryRunDTO
		gr := do(t, http.MethodGet, fmt.Sprintf("%s/api/v1/history/runs/%d", base, list.Runs[0].ID), "")
		if err := json.Unmarshal([]byte(readBody(t, gr)), &run); err != nil {
			t.Fatal(err)
		}
		want := 2
		if route.path == "/api/v1/execute" {
			want = 1
		}
		if len(run.Results) != want || run.FinishedAt == "" || run.Total != int64(want) {
			t.Fatalf("%s: GetRun right after the response = %+v; want %d results and a finished run", route.path, run, want)
		}
		if len(run.Results[0].Assertions) == 0 {
			t.Fatalf("%s: assertions missing from the committed run", route.path)
		}
		do(t, http.MethodDelete, base+"/api/v1/history/runs", "", withHeader("Content-Type", "application/json"))
	}
}

// Concurrent runs, one updating snapshots and one checking, do not share
// snapshot state (they used to, through a process-wide manager).
func TestConcurrentRunsKeepSnapshotsSeparate(t *testing.T) {
	snapFile := func(name string) string {
		return "### s\n# @name s\nGET {{base}}/x\n\n>>>\nexpect status 200\nexpect body snapshot \"base\"\n<<<\n"
	}
	files := map[string]string{}
	for i := 0; i < 8; i++ {
		files[fmt.Sprintf("d%d/api.http", i)] = snapFile("api")
		if i%2 == 1 {
			files[fmt.Sprintf("d%d/__snapshots__/api.snap.json", i)] = `{"s::base":{"ok":true}}`
		}
	}
	dir, backend := runtimeWorkspace(t, files)
	if err := os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte("defaultEnvironment: dev\nenvironments:\n  test:\n    base: "+backend+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, base, _ := startSecured(t, WithWorkDir(dir))

	// Even directories have no baseline and serve is not in update mode, so
	// they must fail; odd ones compare against their baseline and pass. Before
	// the fix one run could observe another run's manager.
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp := postJ(t, base+"/api/v1/run", fmt.Sprintf(`{"file":"d%d/api.http"}`, i))
			var out RunResultDTO
			if err := json.Unmarshal([]byte(readBody(t, resp)), &out); err != nil {
				t.Errorf("d%d: %v", i, err)
				return
			}
			if wantPass := i%2 == 1; (out.Failed == 0) != wantPass {
				t.Errorf("d%d: failed=%d, want pass=%v", i, out.Failed, wantPass)
			}
		}(i)
	}
	wg.Wait()
}

func putConfig(t *testing.T, base, body string) (int, ConfigDTO) {
	t.Helper()
	resp := do(t, http.MethodPut, base+"/api/v1/config", body, withHeader("Content-Type", "application/json"))
	var dto ConfigDTO
	raw := readBody(t, resp)
	_ = json.Unmarshal([]byte(raw), &dto)
	return resp.StatusCode, dto
}

func getConfig(t *testing.T, base string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(readBody(t, do(t, http.MethodGet, base+"/api/v1/config", ""))), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// PUT /config can set retries to 0 and clear the proxy (a plain "> 0" / "!= ”"
// guard could not), and the new retryDelay / maxRedirects fields round-trip.
func TestPutConfigFieldPresence(t *testing.T) {
	dir := t.TempDir()
	yaml := "defaultEnvironment: dev\ntimeout: 12000\nretries: 3\nproxy: http://proxy.local:8080\nconcurrency: 7\n"
	if err := os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	_, base, _ := startSecured(t, WithWorkDir(dir))

	if code, got := putConfig(t, base, `{"retries":0,"proxy":""}`); code != http.StatusOK || got.Retries != 0 || got.Proxy != "" || got.Timeout != 12000 || got.Concurrency != 7 {
		t.Fatalf("PUT {retries:0,proxy:\"\"} = %d %+v; want 200, retries/proxy cleared, the rest untouched", code, got)
	}
	cfg := getConfig(t, base)
	if _, ok := cfg["retries"]; ok {
		t.Errorf("GET still reports retries: %v", cfg)
	}
	if _, ok := cfg["proxy"]; ok {
		t.Errorf("GET still reports proxy: %v", cfg)
	}
	onDisk, _ := os.ReadFile(filepath.Join(dir, "hitspec.yaml"))
	if strings.Contains(string(onDisk), "retries") || strings.Contains(string(onDisk), "proxy") {
		t.Errorf("hitspec.yaml still holds the cleared fields:\n%s", onDisk)
	}

	if code, got := putConfig(t, base, `{"retryDelay":250,"maxRedirects":3}`); code != http.StatusOK || got.RetryDelay != 250 || got.MaxRedirects != 3 {
		t.Fatalf("PUT retryDelay/maxRedirects = %d %+v", code, got)
	}
	cfg = getConfig(t, base)
	if cfg["retryDelay"] != float64(250) || cfg["maxRedirects"] != float64(3) {
		t.Errorf("GET = %v, want retryDelay 250 and maxRedirects 3", cfg)
	}
	onDisk, _ = os.ReadFile(filepath.Join(dir, "hitspec.yaml"))
	if !strings.Contains(string(onDisk), "retryDelay: 250") || !strings.Contains(string(onDisk), "maxRedirects: 3") {
		t.Errorf("hitspec.yaml lost the new fields:\n%s", onDisk)
	}

	// Old clients only send what they change; the rest stays.
	if code, _ := putConfig(t, base, `{"timeout":5000}`); code != http.StatusOK {
		t.Fatalf("legacy partial PUT = %d", code)
	}
	cfg = getConfig(t, base)
	if cfg["timeout"] != float64(5000) || cfg["concurrency"] != float64(7) || cfg["retryDelay"] != float64(250) {
		t.Errorf("partial PUT clobbered other fields: %v", cfg)
	}

	if code, _ := putConfig(t, base, `{"retries":-1}`); code != http.StatusBadRequest {
		t.Errorf("negative retries = %d, want 400", code)
	}
	if code, _ := putConfig(t, base, `{"retries":"two"}`); code != http.StatusBadRequest {
		t.Errorf("non-numeric retries = %d, want 400", code)
	}
}

// `< file` request bodies are confined to the workspace for /execute and /run:
// a file outside is never read, let alone sent to the backend.
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

	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("TOP-SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir, _ := runtimeWorkspace(t, map[string]string{
		"fixtures/ok.json": `{"fine":true}`,
		"inside.http":      "### inside\n# @name inside\nPOST " + backend.URL + "/x\n\n< ./fixtures/ok.json\n\n>>>\nexpect status 200\n<<<\n",
		"escape.http":      "### escape\n# @name escape\nPOST " + backend.URL + "/x\n\n< " + secret + "\n\n>>>\nexpect status 200\n<<<\n",
		"traverse.http":    "### traverse\n# @name traverse\nPOST " + backend.URL + "/x\n\n< ../../../../../../../../etc/hosts\n\n>>>\nexpect status 200\n<<<\n",
	})
	_, base, _ := startSecured(t, WithWorkDir(dir))

	run := func(path, file string) RunResultDTO {
		resp := postJ(t, base+path, fmt.Sprintf(`{"file":%q}`, file))
		var out RunResultDTO
		if err := json.Unmarshal([]byte(readBody(t, resp)), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	for _, path := range []string{"/api/v1/run", "/api/v1/execute"} {
		if out := run(path, "inside.http"); out.Failed != 0 || out.Passed != 1 {
			t.Fatalf("%s inside.http = %+v; a body file inside the workspace must work", path, out)
		}
		for _, f := range []string{"escape.http", "traverse.http"} {
			out := run(path, f)
			if out.Failed != 1 || len(out.Results) != 1 || !strings.Contains(out.Results[0].Error, "outside the workspace") {
				t.Fatalf("%s %s = %+v; want the request to fail with an outside-the-workspace error", path, f, out)
			}
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for _, b := range received {
		if strings.Contains(b, "TOP-SECRET") || strings.Contains(b, "localhost") {
			t.Fatalf("a file outside the workspace reached the backend: %q", b)
		}
	}
	if len(received) != 2 {
		t.Fatalf("backend saw %d requests, want only the 2 inside.http ones: %q", len(received), received)
	}
}

// startReal starts s with the real Start (signal handling, graceful shutdown)
// on an ephemeral loopback port. It returns the base URL, a function that
// requests shutdown, and the channel Start's result arrives on.
func startReal(t *testing.T, s *Server, ctx context.Context) (base string, errCh chan error) {
	t.Helper()
	addr := make(chan string, 1)
	s.onListen = func(a net.Addr) { addr <- a.String() }
	s.config.Host = "127.0.0.1"
	s.config.Port = 0
	errCh = make(chan error, 1)
	go func() { errCh <- s.Start(ctx) }()
	select {
	case a := <-addr:
		return "http://" + a, errCh
	case err := <-errCh:
		t.Fatalf("Start returned early: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not start")
	}
	return "", nil
}

func slowBackend(t *testing.T, d time.Duration) (url string, started chan struct{}) {
	t.Helper()
	started = make(chan struct{}, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-time.After(d):
			fmt.Fprint(w, "slow-ok")
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL, started
}

// /run and /execute outlive the server-wide WriteTimeout: a run takes as long
// as the API it calls.
func TestLongRunOutlivesWriteTimeout(t *testing.T) {
	backend, _ := slowBackend(t, 900*time.Millisecond)
	dir, _ := runtimeWorkspace(t, map[string]string{
		"slow.http": "### slow\n# @name slow\nGET " + backend + "/slow\n\n>>>\nexpect status 200\nexpect body contains \"slow-ok\"\n<<<\n",
	})
	s := newTestServer(t, WithWorkDir(dir), WithWatch(false), WithHistoryDBPath(filepath.Join(dir, ".h.db")))
	s.writeTimeout = 300 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	base, errCh := startReal(t, s, ctx)
	defer func() { cancel(); <-errCh }()

	for _, path := range []string{"/api/v1/run", "/api/v1/execute"} {
		resp := postJ(t, base+path, `{"file":"slow.http"}`)
		var out RunResultDTO
		if err := json.Unmarshal([]byte(readBody(t, resp)), &out); err != nil || out.Passed != 1 {
			t.Fatalf("%s: %+v, %v (the response must survive WriteTimeout)", path, out, err)
		}
	}
}

// Shutdown drains an in-flight run, closes the history database only after it
// has been recorded, and Start returns after that.
func TestShutdownDrainsInFlightRun(t *testing.T) {
	backend, started := slowBackend(t, 600*time.Millisecond)
	dir, _ := runtimeWorkspace(t, map[string]string{
		"slow.http": "### slow\n# @name slow\nGET " + backend + "/slow\n\n>>>\nexpect status 200\n<<<\n",
	})
	dbPath := filepath.Join(dir, ".h.db")
	s := newTestServer(t, WithWorkDir(dir), WithWatch(false), WithHistoryDBPath(dbPath))
	ctx, cancel := context.WithCancel(context.Background())
	base, errCh := startReal(t, s, ctx)

	type result struct {
		out RunResultDTO
		err error
	}
	got := make(chan result, 1)
	go func() {
		resp, err := http.Post(base+"/api/v1/run", "application/json", strings.NewReader(`{"file":"slow.http"}`))
		if err != nil {
			got <- result{err: err}
			return
		}
		defer resp.Body.Close()
		var out RunResultDTO
		err = json.NewDecoder(resp.Body).Decode(&out)
		got <- result{out, err}
	}()
	<-started
	cancel() // shutdown requested while the run is in flight

	select {
	case r := <-got:
		if r.err != nil || r.out.Passed != 1 || r.out.Cancelled {
			t.Fatalf("in-flight run = %+v, %v; want it to finish, not be cancelled", r.out, r.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight run never answered")
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after shutdown")
	}

	// The run was recorded before the database was closed.
	store, err := history.NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if n, err := store.Queries().CountRuns(context.Background()); err != nil || n != 1 {
		t.Fatalf("runs in history after shutdown = %d, %v; want 1", n, err)
	}
}

// Past the grace period what is still running is cancelled and Start returns.
func TestShutdownCancelsAfterGrace(t *testing.T) {
	old := shutdownGrace
	shutdownGrace = 150 * time.Millisecond
	defer func() { shutdownGrace = old }()

	backend, started := slowBackend(t, 3*time.Second)
	dir, _ := runtimeWorkspace(t, map[string]string{
		"slow.http": "### slow\n# @name slow\nGET " + backend + "/slow\n",
	})
	s := newTestServer(t, WithWorkDir(dir), WithWatch(false), WithHistoryDBPath(filepath.Join(dir, ".h.db")))
	ctx, cancel := context.WithCancel(context.Background())
	base, errCh := startReal(t, s, ctx)
	go func() {
		resp, err := http.Post(base+"/api/v1/run", "application/json", strings.NewReader(`{"file":"slow.http"}`))
		if err == nil {
			resp.Body.Close()
		}
	}()
	<-started
	begin := time.Now()
	cancel()
	select {
	case <-errCh:
		if d := time.Since(begin); d > 2*time.Second {
			t.Fatalf("Start took %v to return after the grace period", d)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return")
	}
}

// --parent-pid: the server shuts itself down when the watched process exits.
func TestServerExitsWhenParentExits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a unix sleep process")
	}
	old := parentPollInterval
	parentPollInterval = 50 * time.Millisecond
	defer func() { parentPollInterval = old }()
	parent := exec.Command("sleep", "30")
	if err := parent.Start(); err != nil {
		t.Skipf("cannot start a helper process: %v", err)
	}
	defer func() { _ = parent.Process.Kill(); _ = parent.Wait() }()

	dir := t.TempDir()
	s := newTestServer(t, WithWorkDir(dir), WithWatch(false), WithHistoryDBPath(filepath.Join(dir, ".h.db")), WithParentPID(parent.Process.Pid))
	_, errCh := startReal(t, s, context.Background())

	select {
	case err := <-errCh:
		t.Fatalf("server stopped while its parent was alive: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	_ = parent.Process.Kill()
	_, _ = parent.Process.Wait()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server kept running after its parent exited")
	}
}

func TestWatchParent(t *testing.T) {
	var mu sync.Mutex
	alive := true
	check := func(int) bool { mu.Lock(); defer mu.Unlock(); return alive }
	gone := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watchParent(ctx, 999999, 10*time.Millisecond, check, func() { close(gone) })
	select {
	case <-gone:
		t.Fatal("reported gone while alive")
	case <-time.After(60 * time.Millisecond):
	}
	mu.Lock()
	alive = false
	mu.Unlock()
	select {
	case <-gone:
	case <-time.After(2 * time.Second):
		t.Fatal("never reported the parent gone")
	}

	// A cancelled context stops the watcher without calling onGone.
	ctx2, cancel2 := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		watchParent(ctx2, 999999, 10*time.Millisecond, func(int) bool { return true }, func() { t.Error("onGone called") })
		close(done)
	}()
	cancel2()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watcher ignored context cancellation")
	}
}

func TestProcessAlive(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Error("own pid reported dead")
	}
	if processAlive(0) || processAlive(-5) {
		t.Error("invalid pids reported alive")
	}
}
