package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInterruptContext_FirstSignalCancelsSecondForcesExit(t *testing.T) {
	sigs := make(chan os.Signal, 2)
	forced := make(chan int, 1)
	var stderr bytes.Buffer

	ctx, interrupted, stop := interruptContext(context.Background(), sigs, &stderr, func(code int) { forced <- code })
	defer stop()

	assert.NoError(t, ctx.Err())
	assert.False(t, interrupted())

	sigs <- os.Interrupt
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("first signal did not cancel the context")
	}
	assert.True(t, interrupted())
	select {
	case code := <-forced:
		t.Fatalf("a single signal forced exit %d", code)
	case <-time.After(50 * time.Millisecond):
	}

	sigs <- os.Interrupt
	select {
	case code := <-forced:
		assert.Equal(t, ExitInterrupted, code)
	case <-time.After(2 * time.Second):
		t.Fatal("second signal did not force the exit")
	}
	assert.Contains(t, stderr.String(), "Interrupted")
}

func TestInterruptContext_StopWithoutSignal(t *testing.T) {
	sigs := make(chan os.Signal, 1)
	ctx, interrupted, stop := interruptContext(context.Background(), sigs, &bytes.Buffer{}, func(int) { t.Error("forced") })
	stop()
	stop() // idempotent
	assert.Error(t, ctx.Err(), "stop releases the context")
	assert.False(t, interrupted())
}

// A run interrupted while a request is in flight reports what it has, still
// writes --output-file, and exits 130.
func TestRun_InterruptWritesPartialResultsAndExits130(t *testing.T) {
	resetRootCommandForTest(t)

	var hits atomic.Int32
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fast" {
			hits.Add(1)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		hits.Add(1)
		select {
		case started <- struct{}{}:
		default:
		}
		<-r.Context().Done() // hangs until the client gives up
	}))
	defer server.Close()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "api.http")
	content := "### fast one\nGET " + server.URL + "/fast\n\n>>>\nexpect status 200\n<<<\n\n" +
		"### hanging\nGET " + server.URL + "/hang\n\n" +
		"### never started\nGET " + server.URL + "/fast\n"
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0o644))
	second := filepath.Join(dir, "second.http")
	require.NoError(t, os.WriteFile(second, []byte("### other file\nGET "+server.URL+"/fast\n"), 0o644))
	outFile := filepath.Join(dir, "report.json")

	sigs := make(chan os.Signal, 2)
	orig := notifyInterrupts
	notifyInterrupts = func() (<-chan os.Signal, func()) { return sigs, func() {} }
	defer func() { notifyInterrupts = orig }()

	go func() {
		select {
		case <-started:
			sigs <- os.Interrupt
		case <-time.After(10 * time.Second):
		}
	}()

	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"run", testFile, second, "-o", "json", "--output-file", outFile, "--no-history", "--no-color"})
	done := make(chan error, 1)
	go func() { done <- rootCmd.Execute() }()

	var err error
	select {
	case err = <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("run did not stop after the interrupt")
	}

	var coder ExitCoder
	require.True(t, errors.As(err, &coder), "err = %v", err)
	assert.Equal(t, ExitInterrupted, coder.ExitCode())
	assert.Contains(t, stderr.String(), "Interrupted")

	raw, readErr := os.ReadFile(outFile)
	require.NoError(t, readErr, "--output-file must still be written")
	var report struct {
		Files []struct {
			File    string `json:"file"`
			Results []struct {
				Name string `json:"name"`
			} `json:"results"`
		} `json:"files"`
	}
	require.NoError(t, json.Unmarshal(raw, &report), "report must be one valid JSON document:\n%s", raw)
	text := string(raw)
	assert.Contains(t, text, "fast one")
	assert.Contains(t, text, "hanging")
	assert.Contains(t, text, "cancel", "cancelled requests are marked")
	assert.NotContains(t, text, "other file", "files after the interrupt are not started")
	assert.EqualValues(t, 2, hits.Load(), "the never-started request must not run")
}
