package serve

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
)

// Stress and contract verification run the same request files as `run`, so a
// `< /abs/secret` body outside the workspace is refused for them too.
func TestStressAndContractRefuseFilesOutsideWorkspace(t *testing.T) {
	var mu sync.Mutex
	var received []string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, string(b))
		mu.Unlock()
	}))
	defer provider.Close()

	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("TOP-SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "escape.http"), []byte("### out\nPOST "+provider.URL+"/x\n\n< "+secret+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewServer(WithWorkDir(dir), WithEnv("test"), WithLogLevel("error"), WithHistoryDBPath(filepath.Join(t.TempDir(), "h.db")), WithWatch(false))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.ctx, s.cancel = ctx, cancel

	result := startStressAndWait(t, s, `{"files":["escape.http"],"duration":"400ms","rate":20}`)
	if result.Errors == 0 || result.Success != 0 {
		t.Fatalf("stress result = %+v; want every attempt refused", result)
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "outside the workspace") {
		t.Fatalf("warnings = %v; want the refusal explained", result.Warnings)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/contract/verify", strings.NewReader(`{"files":["escape.http"],"providerUrl":"`+provider.URL+`"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleContractVerify(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "outside the workspace") || !strings.Contains(w.Body.String(), `"failed":1`) {
		t.Fatalf("contract verify = %d %s; want the interaction refused", w.Code, w.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 0 {
		t.Fatalf("the provider received %q; nothing may be sent", received)
	}
}
