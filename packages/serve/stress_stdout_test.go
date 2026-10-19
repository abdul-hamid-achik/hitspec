package serve

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// serve --api-only runs as a child process of the desktop app: a stress run
// must not print its header, progress or summary on the server's stdout.
func TestHandleStressStart_StdoutStaysQuiet(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()

	dir := writeStressWorkspace(t, "test", target.URL)
	s := NewServer(WithWorkDir(dir), WithEnv("test"), WithLogLevel("error"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.ctx, s.cancel = ctx, cancel

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	done := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- b
	}()

	startStressAndWait(t, s, `{"files":["probe.http"],"duration":"300ms","rate":20}`)

	os.Stdout = orig
	_ = w.Close()
	if out := <-done; len(out) != 0 {
		t.Errorf("stress run wrote to stdout:\n%s", out)
	}
}
