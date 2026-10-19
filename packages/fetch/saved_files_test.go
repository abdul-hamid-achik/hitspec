package fetch

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/testhome"
)

// A saved request with a workspace root can only send files that live inside
// it: a `< file` body or multipart file reached by ../, an absolute path, a
// symlink or a {{variable}} outside the workspace is refused with one message
// (so a missing file and an out-of-workspace file are indistinguishable), and
// nothing is sent.
func TestFetchSavedBodyFilesStayInWorkspace(t *testing.T) {
	var (
		mu       sync.Mutex
		received []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, string(b))
		mu.Unlock()
	}))
	defer server.Close()

	parent := t.TempDir()
	testhome.Set(t, filepath.Join(parent, "home"))
	workspace := filepath.Join(parent, "ws")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(parent, "secret.txt")
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(secret, "TOP-SECRET")
	write(filepath.Join(workspace, "ok.txt"), "WORKSPACE-FILE")
	linked := os.Symlink(secret, filepath.Join(workspace, "link.txt")) == nil

	cases := []struct {
		name, request string
		allowed       bool
		needsSymlink  bool
	}{
		{name: "inside", request: "POST " + server.URL + "\n\n< ./ok.txt\n", allowed: true},
		{name: "dotdot", request: "POST " + server.URL + "\n\n< ../secret.txt\n"},
		{name: "absolute", request: "POST " + server.URL + "\n\n< " + secret + "\n"},
		{name: "missing", request: "POST " + server.URL + "\n\n< ./nope.txt\n"},
		{name: "variable", request: "@p = " + secret + "\nPOST " + server.URL + "\n\n< {{p}}\n"},
		{name: "symlink", request: "POST " + server.URL + "\n\n< ./link.txt\n", needsSymlink: true},
		{name: "multipart", request: "POST " + server.URL + "\n\n>>>multipart\nfile avatar @" + secret + "\n<<<\n"},
	}
	var refused []string
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.needsSymlink && !linked {
				t.Skip("symlinks unavailable")
			}
			file := filepath.Join(workspace, tc.name+".http")
			write(file, tc.request)
			_, err := FetchSaved(context.Background(), SavedRequest{Path: file, WorkspaceRoot: workspace})
			if tc.allowed {
				if err != nil {
					t.Fatalf("a file inside the workspace was refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("a file outside the workspace was sent")
			}
			if err.Error() != errBodyFileNotAllowed.Error() {
				// A parse error for a case the grammar does not accept is
				// fine, but it must not carry the secret's location.
				if strings.Contains(err.Error(), parent) {
					t.Fatalf("error leaks an absolute path: %v", err)
				}
				return
			}
			refused = append(refused, tc.name)
		})
	}
	mu.Lock()
	defer mu.Unlock()
	for _, body := range received {
		if strings.Contains(body, "TOP-SECRET") {
			t.Fatalf("the outside file reached the server: %q", body)
		}
	}
	if len(refused) < 4 {
		t.Errorf("only %v were refused with the uniform message; want at least dotdot, absolute, missing, variable", refused)
	}
}
