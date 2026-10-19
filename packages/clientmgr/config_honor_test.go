package clientmgr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// configServer echoes the X-From-Yaml request header on /echo and answers
// /slow after one second.
func configServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Header.Get("X-From-Yaml")))
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(time.Second):
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte("slept"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func configWorkspace(t *testing.T, yaml, httpFile string) *Manager {
	t.Helper()
	for _, k := range []string{"HITSPEC_TIMEOUT", "HITSPEC_PROXY", "HITSPEC_INSECURE", "HITSPEC_PARALLEL", "HITSPEC_CONCURRENCY", "HITSPEC_BAIL"} {
		t.Setenv(k, "")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "api.http"), []byte(httpFile), 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(WithWorkDir(dir), WithWatch(false), WithHistoryDBPath(filepath.Join(dir, "history.db")))
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// TestStudioHonorsYAMLTimeout: the studio used to hardcode a 30s timeout, so a
// hitspec.yaml timeout (and the Settings screen that edits it) had no effect.
func TestStudioHonorsYAMLTimeout(t *testing.T) {
	srv := configServer(t)
	m := configWorkspace(t, "timeout: 200\n", "### slow\n# @name slow\nGET "+srv.URL+"/slow\n")

	start := time.Now()
	dto, err := m.Execute(context.Background(), ExecuteReq{File: "api.http", RequestName: "slow"})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 900*time.Millisecond {
		t.Fatalf("request ran %v, the 200ms yaml timeout was not applied", elapsed)
	}
	if len(dto.Results) != 1 || dto.Results[0].Passed || dto.Results[0].Error == "" {
		t.Fatalf("expected a timed-out request: %+v", dto.Results)
	}

	// Editing the timeout from the Settings screen takes effect on the next run.
	if _, err := m.PutConfig(context.Background(), ConfigDTO{Timeout: 5000}); err != nil {
		t.Fatal(err)
	}
	dto, err = m.Execute(context.Background(), ExecuteReq{File: "api.http", RequestName: "slow"})
	if err != nil {
		t.Fatal(err)
	}
	if len(dto.Results) != 1 || !dto.Results[0].Passed {
		t.Fatalf("request should pass with a 5s timeout: %+v", dto.Results)
	}
}

// TestStudioHonorsYAMLHeaders covers the run, execute and ad-hoc paths.
func TestStudioHonorsYAMLHeaders(t *testing.T) {
	srv := configServer(t)
	m := configWorkspace(t, "headers:\n  X-From-Yaml: yaml-value\n", "### echo\n# @name echo\nGET "+srv.URL+"/echo\n")
	ctx := context.Background()

	run, err := m.RunFile(ctx, RunReq{File: "api.http"})
	if err != nil {
		t.Fatal(err)
	}
	exec, err := m.Execute(ctx, ExecuteReq{File: "api.http", RequestName: "echo"})
	if err != nil {
		t.Fatal(err)
	}
	adhoc, err := m.ExecuteAdHoc(ctx, AdHocReq{Method: "GET", URL: srv.URL + "/echo"})
	if err != nil {
		t.Fatal(err)
	}
	for name, dto := range map[string]*RunResultDTO{"RunFile": run, "Execute": exec, "ExecuteAdHoc": adhoc} {
		if len(dto.Results) != 1 || dto.Results[0].Response == nil || strings.TrimSpace(dto.Results[0].Response.Body) != "yaml-value" {
			t.Errorf("%s did not send the yaml default header: %+v", name, dto.Results)
		}
	}
}

// TestStudioHonorsHitspecEnv: HITSPEC_TIMEOUT sits above hitspec.yaml, as in
// `hitspec run`.
func TestStudioHonorsHitspecEnv(t *testing.T) {
	srv := configServer(t)
	m := configWorkspace(t, "timeout: 5000\n", "### slow\n# @name slow\nGET "+srv.URL+"/slow\n")
	t.Setenv("HITSPEC_TIMEOUT", "200ms")

	dto, err := m.Execute(context.Background(), ExecuteReq{File: "api.http", RequestName: "slow"})
	if err != nil {
		t.Fatal(err)
	}
	if len(dto.Results) != 1 || dto.Results[0].Passed || dto.Results[0].Error == "" {
		t.Fatalf("HITSPEC_TIMEOUT=200ms should beat yaml timeout 5000: %+v", dto.Results)
	}
}

// TestStudioContractUsesConfigClient: the contract verifier sends the yaml
// default headers like a normal run does.
func TestStudioContractUsesConfigClient(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("X-From-Yaml"))
		mu.Unlock()
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	m := configWorkspace(t, "headers:\n  X-From-Yaml: yaml-value\n", "### ping\n# @name ping\nGET {{baseUrl}}/ping\n")

	results, err := m.VerifyContracts(context.Background(), ContractVerifyReq{ProviderURL: srv.URL, Files: []string{"api.http"}})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(results) != 1 || len(seen) != 1 || seen[0] != "yaml-value" {
		t.Fatalf("contract verifier did not send the yaml header: results=%+v seen=%v", results, seen)
	}
}

// TestStudioStressUsesConfigClient: stress traffic carries the yaml default
// headers (it used to build a bare client).
func TestStudioStressUsesConfigClient(t *testing.T) {
	var mu sync.Mutex
	hits, withHeader := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		if r.Header.Get("X-From-Yaml") == "yaml-value" {
			withHeader++
		}
		mu.Unlock()
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	m := configWorkspace(t, "headers:\n  X-From-Yaml: yaml-value\n", "### ping\n# @name ping\nGET "+srv.URL+"/ping\n")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	events := m.Subscribe(ctx)
	if err := m.StartStress(ctx, StressStartReq{Files: []string{"api.http"}, Duration: "300ms", Rate: 20}); err != nil {
		t.Fatal(err)
	}
	for done := false; !done; {
		select {
		case ev := <-events:
			if sm, ok := ev.Payload.(StressMetrics); ok && sm.Completed {
				done = true
			}
		case <-ctx.Done():
			t.Fatal("stress run did not finish")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if hits == 0 || withHeader != hits {
		t.Fatalf("stress sent %d requests, %d with the yaml header", hits, withHeader)
	}
}
