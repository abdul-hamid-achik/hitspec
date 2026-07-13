package runner

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunFileContextCancelsInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "cancel.http")
	if err := os.WriteFile(path, []byte("### cancellable\n# @name cancellable\nGET "+server.URL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan *RunResult, 1)
	go func() {
		result, _ := NewRunner(&Config{Environment: "dev", FollowRedirect: true, ValidateSSL: true, Timeout: time.Minute}).RunFileContext(ctx, path)
		done <- result
	}()
	select {
	case <-started:
		cancel()
	case <-time.After(2 * time.Second):
		t.Fatal("request did not start")
	}
	select {
	case result := <-done:
		if result == nil || len(result.Results) != 1 || !errors.Is(result.Results[0].Error, context.Canceled) {
			t.Fatalf("unexpected result: %#v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunFileContext did not return after cancellation")
	}
}
