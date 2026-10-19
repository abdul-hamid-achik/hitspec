package mcp

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hitspec_fetch on a workspace file never picks up a hitspec.yaml that lives
// above the workspace root.
func TestFetchIgnoresConfigAboveWorkspace(t *testing.T) {
	recorder := &recordedRequest{}
	web := httptest.NewServer(recorder.handler())
	defer web.Close()

	parent := t.TempDir()
	t.Setenv("HOME", filepath.Join(parent, "home"))
	if err := os.WriteFile(filepath.Join(parent, "hitspec.yaml"), []byte("headers:\n  X-Parent-Cfg: leaked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(parent, "ws")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceFile(t, workspace, "a.http", "GET "+web.URL+"/x\n")

	server, err := NewServer("test", workspace, Options{AllowPrivateNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := server.handleFetch(context.Background(), nil, fetchInput{File: "a.http"}); err != nil {
		t.Fatal(err)
	}
	got := recorder.all()
	if got == "" {
		t.Fatal("fake server received no request")
	}
	if strings.Contains(strings.ToLower(got), "x-parent-cfg") {
		t.Fatalf("a config above the workspace was applied:\n%s", got)
	}
}
