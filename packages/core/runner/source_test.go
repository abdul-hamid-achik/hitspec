package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestRunFileContextRunsUnsavedSource: with WithSource the runner executes the
// given text under the file's real path, not what is on disk.
func TestRunFileContextRunsUnsavedSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	onDisk := "### ping\n# @name ping\nGET " + srv.URL + "\n\n>>>\nexpect status 200\n<<<\n"
	edited := "### ping\n# @name ping\nGET " + srv.URL + "\n\n>>>\nexpect status 201\n<<<\n"
	path := filepath.Join(t.TempDir(), "api.http")
	if err := os.WriteFile(path, []byte(onDisk), 0o600); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(&Config{})
	res, err := r.RunFileContext(context.Background(), path)
	if err != nil || res.Failed != 0 || res.Passed != 1 {
		t.Fatalf("file on disk: err=%v res=%+v", err, res)
	}

	res, err = NewRunner(&Config{}).RunFileContext(WithSource(context.Background(), edited), path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed != 1 || res.Passed != 0 {
		t.Fatalf("the edited source must run (expect 201 fails against a 200): %+v", res)
	}
	if res.File != path {
		t.Fatalf("results should report the real path, got %q", res.File)
	}
	if got, _ := os.ReadFile(path); string(got) != onDisk {
		t.Fatal("running a source must not write the file")
	}
}

func TestRunFileContextSourceParseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api.http")
	_, err := NewRunner(&Config{}).RunFileContext(WithSource(context.Background(), "### x\nNOPE\n"), path)
	if err == nil {
		t.Fatal("an unparseable source should fail the run")
	}
}
