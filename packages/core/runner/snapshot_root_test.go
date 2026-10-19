package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// A run with a FileRoot (serve, the studio) refuses a __snapshots__ directory
// that is a symlink out of the workspace; without one (hitspec run) the
// symlink is followed as before.
func TestSnapshotDirSymlinkConfinedByFileRoot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":1}`)
	}))
	defer srv.Close()

	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "__snapshots__")); err != nil {
		// Symlinks need privileges on Windows (developer mode or admin).
		t.Skipf("symlinks unavailable: %v", err)
	}
	path := filepath.Join(root, "api.http")
	content := "### snap\n# @name snap\nGET " + srv.URL + "/x\n\n>>>\nexpect body snapshot \"base\"\n<<<\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := baseConfig()
	cfg.UpdateSnapshots = true
	cfg.FileRoot = root
	res, err := NewRunner(cfg).RunFileContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed != 1 {
		t.Errorf("failed = %d, want 1 (snapshot outside the workspace)", res.Failed)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("snapshot written outside the workspace: %v", entries)
	}

	cfg.FileRoot = ""
	res, err = NewRunner(cfg).RunFileContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed != 0 {
		t.Errorf("without a root: %d failed, want 0", res.Failed)
	}
}
