package clientmgr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// An `expect body schema <file>` that reaches outside the workspace through a
// symlink is refused in stress and contract runs, like `< file` bodies are.
func TestStressAndContractRefuseSchemaOutsideWorkspace(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"a":1}`))
	}))
	defer provider.Close()

	// A schema that accepts the response: if it is read, the assertion passes.
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "schema.json"), []byte(`{"type":"object"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(outside, "schema.json"), filepath.Join(dir, "link.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inside.json"), []byte(`{"type":"object"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newTestManagerAt(t, dir)
	ctx := context.Background()
	for name, schema := range map[string]string{"escape.http": "./link.json", "inside.http": "./inside.json"} {
		if _, err := m.CreateFile(ctx, name, "### chk\n# @name chk\nGET "+provider.URL+"/x\n\n>>>\nexpect status 200\nexpect body schema "+schema+"\n<<<\n"); err != nil {
			t.Fatal(err)
		}
	}

	stressFailures := func(file string) *StressResultDTO {
		t.Helper()
		// A fresh manager per run: StressResult keeps the previous run's result.
		m := newTestManagerAt(t, dir)
		if err := m.StartStress(ctx, StressStartReq{Files: []string{file}, Duration: "400ms", Rate: 20}); err != nil {
			t.Fatal(err)
		}
		var result *StressResultDTO
		waitFor(t, 10*time.Second, "stress result", func() bool {
			r, err := m.StressResult(ctx)
			result = r
			return err == nil && r.Total > 0
		})
		return result
	}
	if r := stressFailures("inside.http"); r.AssertionChecks == 0 || r.AssertionFailures != 0 {
		t.Fatalf("a schema inside the workspace: checks=%d failures=%d; want all passing", r.AssertionChecks, r.AssertionFailures)
	}
	if r := stressFailures("escape.http"); r.AssertionChecks == 0 || r.AssertionFailures != r.AssertionChecks {
		t.Fatalf("a schema behind an outside symlink: checks=%d failures=%d; want every check refused", r.AssertionChecks, r.AssertionFailures)
	}

	res, err := m.VerifyContracts(ctx, ContractVerifyReq{Files: []string{"escape.http"}, ProviderURL: provider.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Failed != 1 {
		t.Fatalf("contract result = %+v; want the interaction refused", res)
	}
	res, err = m.VerifyContracts(ctx, ContractVerifyReq{Files: []string{"inside.http"}, ProviderURL: provider.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Passed != 1 {
		t.Fatalf("contract result = %+v; want the in-workspace schema to pass", res)
	}
}
