package contract

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	hitspechttp "github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

func writeContract(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "c.http")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestWithHTTPClient: the verifier calls the provider with the client it is
// given, so hitspec.yaml settings (here a default header) apply.
func TestWithHTTPClient(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-From-Yaml")
	}))
	defer srv.Close()
	path := writeContract(t, "### ping\nGET "+srv.URL+"/ping\n")

	client := hitspechttp.NewClient(hitspechttp.WithDefaultHeaders(map[string]string{"X-From-Yaml": "yaml-value"}))
	res, err := NewVerifier(WithHTTPClient(client)).VerifyFile(path)
	if err != nil || res.Passed != 1 || got != "yaml-value" {
		t.Fatalf("err=%v passed=%d header=%q", err, res.Passed, got)
	}

	// A nil client keeps the default one.
	got = ""
	res, err = NewVerifier(WithHTTPClient(nil)).VerifyFile(path)
	if err != nil || res.Passed != 1 || got != "" {
		t.Fatalf("nil client: err=%v passed=%d header=%q", err, res.Passed, got)
	}
}

// TestVerifyFileContextCancel: cancelling aborts the in-flight provider call
// and does not start the next interaction.
func TestVerifyFileContextCancel(t *testing.T) {
	started := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- r.URL.Path
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	defer srv.Close()
	path := writeContract(t, "### one\nGET "+srv.URL+"/one\n\n### two\nGET "+srv.URL+"/two\n")

	ctx, cancel := context.WithCancel(context.Background())
	type outcome struct {
		res *VerificationResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := NewVerifier().VerifyFileContext(ctx, path)
		done <- outcome{res, err}
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("provider call never started")
	}
	cancel()

	select {
	case out := <-done:
		if !errors.Is(out.err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", out.err)
		}
		if out.res == nil || len(out.res.Results) != 1 || out.res.Results[0].Passed {
			t.Errorf("want the single aborted interaction in the partial result, got %+v", out.res)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("verification did not stop after cancel")
	}
	select {
	case p := <-started:
		t.Errorf("second interaction started: %s", p)
	default:
	}
}
