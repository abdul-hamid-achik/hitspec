package serve

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestExecuteConcurrentPutEnvironment guards the regression where handleRunFile
// handed the live s.fileConfig.Environments map to the runner AFTER releasing
// configMu, while PUT /environments mutated the same map under the write lock — a
// concurrent map read/write that crashed the process. getConfigEnvsLocked now
// returns a snapshot copy, so concurrent execute-vs-PUT must stay race-free.
//
// A single execute goroutine races against many PUT /environments goroutines so
// the test isolates the environments race (concurrent execute-execute runs would
// separately race on the snapshot package's own global, which is out of scope).
func TestExecuteConcurrentPutEnvironment(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	dir := t.TempDir()
	cfgYaml := "defaultEnvironment: dev\nenvironments:\n  dev:\n    baseUrl: " + backend.URL + "\n  prod:\n    baseUrl: " + backend.URL + "\n"
	if err := os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte(cfgYaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	httpFile := filepath.Join(dir, "run.http")
	if err := os.WriteFile(httpFile, []byte("### R\nGET "+backend.URL+"\n\n>>>\nexpect status 200\n<<<\n"), 0o644); err != nil {
		t.Fatalf("write http: %v", err)
	}

	s := newTestServer(t, WithWorkDir(dir))

	runBody := func() []byte {
		b, _ := json.Marshal(RunReq{File: "run.http"})
		return b
	}
	putBody := func() []byte {
		b, _ := json.Marshal(EnvironmentDTO{Name: "prod", Variables: map[string]any{"baseUrl": backend.URL, "x": "y"}})
		return b
	}

	var wg sync.WaitGroup
	// One execute reader racing against many environment writers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 40; j++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader(runBody()))
			s.handleRunFile(httptest.NewRecorder(), req)
		}
	}()
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				req := httptest.NewRequest(http.MethodPut, "/api/v1/environments/prod", bytes.NewReader(putBody()))
				s.handlePutEnvironment(httptest.NewRecorder(), req)
			}
		}()
	}
	wg.Wait()
	// Passes if it completes without a "concurrent map read/write" fatal under -race.
}
