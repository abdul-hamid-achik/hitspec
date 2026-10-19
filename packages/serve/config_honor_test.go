package serve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func configHonorServer(t *testing.T, yaml, httpFile string) (base string) {
	_, base = configHonorServerWith(t, yaml, httpFile)
	return base
}

func configHonorServerWith(t *testing.T, yaml, httpFile string) (*Server, string) {
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
	s, base, _ := startSecured(t, WithWorkDir(dir))
	return s, base
}

func postJSON(t *testing.T, url, body string) (int, RunResultDTO) {
	t.Helper()
	resp := do(t, http.MethodPost, url, body, withHeader("Content-Type", "application/json"))
	defer func() { _ = resp.Body.Close() }()
	var dto RunResultDTO
	_ = json.NewDecoder(resp.Body).Decode(&dto)
	return resp.StatusCode, dto
}

// TestServeHonorsYAMLTimeout: the execute and run handlers used to hardcode a
// 30s timeout, ignoring hitspec.yaml.
func TestServeHonorsYAMLTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(time.Second):
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte("slept"))
	}))
	defer upstream.Close()
	base := configHonorServer(t, "timeout: 200\n", "### slow\n# @name slow\nGET "+upstream.URL+"\n")

	for name, tc := range map[string]struct{ path, body string }{
		"execute": {"/api/v1/execute", `{"file":"api.http","requestName":"slow"}`},
		"run":     {"/api/v1/run", `{"file":"api.http"}`},
	} {
		start := time.Now()
		code, dto := postJSON(t, base+tc.path, tc.body)
		if code != http.StatusOK || len(dto.Results) != 1 || dto.Results[0].Passed || dto.Results[0].Error == "" {
			t.Errorf("%s: want a timed-out request, got %d %+v", name, code, dto.Results)
		}
		if elapsed := time.Since(start); elapsed > 900*time.Millisecond {
			t.Errorf("%s: ran %v, the 200ms yaml timeout was not applied", name, elapsed)
		}
	}

	// PUT /config takes effect on the next run.
	resp := do(t, http.MethodPut, base+"/api/v1/config", `{"timeout":5000}`, withHeader("Content-Type", "application/json"))
	_ = resp.Body.Close()
	code, dto := postJSON(t, base+"/api/v1/run", `{"file":"api.http"}`)
	if code != http.StatusOK || len(dto.Results) != 1 || !dto.Results[0].Passed {
		t.Fatalf("request should pass with a 5s timeout: %d %+v", code, dto.Results)
	}
}

// TestServeHonorsYAMLHeaders: default headers from hitspec.yaml are sent.
func TestServeHonorsYAMLHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Header.Get("X-From-Yaml")))
	}))
	defer upstream.Close()
	base := configHonorServer(t, "headers:\n  X-From-Yaml: yaml-value\n", "### echo\n# @name echo\nGET "+upstream.URL+"\n")

	for name, tc := range map[string]struct{ path, body string }{
		"execute": {"/api/v1/execute", `{"file":"api.http","requestName":"echo"}`},
		"run":     {"/api/v1/run", `{"file":"api.http"}`},
	} {
		_, dto := postJSON(t, base+tc.path, tc.body)
		if len(dto.Results) != 1 || dto.Results[0].Response == nil || strings.TrimSpace(dto.Results[0].Response.Body) != "yaml-value" {
			t.Errorf("%s did not send the yaml default header: %+v", name, dto.Results)
		}
	}
}

// TestServeStressUsesConfigClient: stress traffic carries the yaml headers.
func TestServeStressUsesConfigClient(t *testing.T) {
	var mu sync.Mutex
	hits, tagged := 0, 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		if r.Header.Get("X-From-Yaml") == "yaml-value" {
			tagged++
		}
		mu.Unlock()
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()
	s, base := configHonorServerWith(t, "headers:\n  X-From-Yaml: yaml-value\n", "### ping\n# @name ping\nGET "+upstream.URL+"\n")
	// Start normally provides the server context the stress run derives from.
	s.ctx, s.cancel = context.WithCancel(context.Background())
	t.Cleanup(s.cancel)

	resp := do(t, http.MethodPost, base+"/api/v1/stress/start", `{"files":["api.http"],"duration":"300ms","rate":20}`,
		withHeader("Content-Type", "application/json"))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stress start: %d", resp.StatusCode)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r := do(t, http.MethodGet, base+"/api/v1/stress/result", "")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = r.Body.Close()
		if r.StatusCode == http.StatusOK && len(body) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if hits == 0 || tagged != hits {
		t.Fatalf("stress sent %d requests, %d with the yaml header", hits, tagged)
	}
}

// TestServeRunStopsWhenClientDisconnects: the handlers pass the request
// context to the runner, so an abandoned request aborts the in-flight call and
// does not start the next one.
func TestServeRunStopsWhenClientDisconnects(t *testing.T) {
	var mu sync.Mutex
	paths := []string{}
	aborted := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		select {
		case <-time.After(5 * time.Second):
		case <-r.Context().Done():
			aborted <- struct{}{}
		}
	}))
	defer upstream.Close()
	base := configHonorServer(t, "timeout: 30000\n", "### first\n# @name first\nGET "+upstream.URL+"/first\n\n### second\n# @name second\nGET "+upstream.URL+"/second\n")

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/run", strings.NewReader(`{"file":"api.http"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	done := make(chan struct{})
	go func() {
		defer close(done)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}()
	time.Sleep(200 * time.Millisecond) // let the first request reach the upstream
	cancel()

	select {
	case <-aborted:
	case <-time.After(2 * time.Second):
		t.Fatal("the in-flight upstream request was not aborted after the client went away")
	}
	<-done
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != 1 || paths[0] != "/first" {
		t.Fatalf("upstream saw %v, want only /first", paths)
	}
}
