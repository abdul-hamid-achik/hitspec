package clientmgr

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
	"time"
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
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "escape.http", "### out\nPOST "+provider.URL+"/x\n\n< "+secret+"\n"); err != nil {
		t.Fatal(err)
	}

	if err := m.StartStress(ctx, StressStartReq{Files: []string{"escape.http"}, Duration: "400ms", Rate: 20}); err != nil {
		t.Fatal(err)
	}
	var result *StressResultDTO
	waitFor(t, 10*time.Second, "stress result", func() bool {
		r, err := m.StressResult(ctx)
		result = r
		return err == nil
	})
	if result.Errors == 0 || result.Success != 0 {
		t.Fatalf("stress result = %+v; want every attempt refused", result)
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "outside the workspace") {
		t.Fatalf("warnings = %v; want the refusal explained", result.Warnings)
	}

	res, err := m.VerifyContracts(ctx, ContractVerifyReq{Files: []string{"escape.http"}, ProviderURL: provider.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Failed != 1 || !strings.Contains(res[0].Results[0].Error, "outside the workspace") {
		t.Fatalf("contract result = %+v; want the interaction refused", res)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 0 {
		t.Fatalf("the provider received %q; nothing may be sent", received)
	}
}
