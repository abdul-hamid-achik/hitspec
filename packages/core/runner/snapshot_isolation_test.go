package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Concurrent runs (hitspec serve) must not share snapshot state: a run in
// update mode creates its baseline, a run in check mode against an existing
// baseline compares, whatever the other runs are doing at the same time.
func TestSnapshotManagerIsPerRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":1}`)
	}))
	defer srv.Close()

	const runs = 24
	content := "### snap\n# @name snap\nGET " + srv.URL + "/x\n\n>>>\nexpect status 200\nexpect body snapshot \"base\"\n<<<\n"
	paths := make([]string, runs)
	for i := range paths {
		dir := t.TempDir()
		paths[i] = filepath.Join(dir, "api.http")
		if err := os.WriteFile(paths[i], []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if i%2 == 1 {
			// Odd runs check against an existing baseline.
			snaps := filepath.Join(dir, "__snapshots__")
			if err := os.MkdirAll(snaps, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(snaps, "api.snap.json"), []byte(`{"snap::base":{"id":1}}`), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range paths {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cfg := baseConfig()
			cfg.UpdateSnapshots = i%2 == 0 // even runs create their baseline
			<-start
			res, err := NewRunner(cfg).RunFileContext(context.Background(), paths[i])
			if err != nil {
				t.Errorf("run %d: %v", i, err)
				return
			}
			if res.Failed != 0 {
				t.Errorf("run %d (update=%v): %d failed: %+v", i, cfg.UpdateSnapshots, res.Failed, res.Results[0].Assertions)
			}
		}(i)
	}
	close(start)
	wg.Wait()
}
