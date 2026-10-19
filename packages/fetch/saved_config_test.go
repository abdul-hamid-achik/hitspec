package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/testhome"
)

// savedConfigFixture lays out <tmp>/hitspec.yaml (parent) and
// <tmp>/ws/sub/a.http, and returns the parent, the workspace and the file.
func savedConfigFixture(t *testing.T, url string) (parent, workspace, file string) {
	t.Helper()
	parent = t.TempDir()
	testhome.Set(t, filepath.Join(parent, "home"))
	workspace = filepath.Join(parent, "ws")
	sub := filepath.Join(workspace, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "hitspec.yaml"), []byte("headers:\n  X-Cfg: from-parent\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file = filepath.Join(sub, "a.http")
	if err := os.WriteFile(file, []byte("GET "+url+"/x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return parent, workspace, file
}

func echoCfgHeader(t *testing.T) (*httptest.Server, *string) {
	t.Helper()
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Cfg")
	}))
	t.Cleanup(server.Close)
	return server, &got
}

// A hitspec.yaml above the workspace root never configures a saved request.
func TestFetchSavedIgnoresConfigAboveWorkspace(t *testing.T) {
	server, got := echoCfgHeader(t)
	_, workspace, file := savedConfigFixture(t, server.URL)
	if _, err := FetchSaved(context.Background(), SavedRequest{Path: file, WorkspaceRoot: workspace}); err != nil {
		t.Fatal(err)
	}
	if *got != "" {
		t.Fatalf("X-Cfg = %q: a config outside the workspace was applied", *got)
	}

	// The workspace's own config (above the file, below the root) still applies.
	if err := os.WriteFile(filepath.Join(workspace, "hitspec.yaml"), []byte("headers:\n  X-Cfg: from-workspace\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := FetchSaved(context.Background(), SavedRequest{Path: file, WorkspaceRoot: workspace}); err != nil {
		t.Fatal(err)
	}
	if *got != "from-workspace" {
		t.Fatalf("X-Cfg = %q, want from-workspace", *got)
	}
}

// Without a workspace boundary (hitspec fetch) discovery walks up as usual.
func TestFetchSavedWithoutWorkspaceWalksUp(t *testing.T) {
	server, got := echoCfgHeader(t)
	_, _, file := savedConfigFixture(t, server.URL)
	if _, err := FetchSaved(context.Background(), SavedRequest{Path: file}); err != nil {
		t.Fatal(err)
	}
	if *got != "from-parent" {
		t.Fatalf("X-Cfg = %q, want from-parent", *got)
	}
}
