package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRun_QuietSuppressesOutput guards the regression where `hitspec run --quiet`
// was a documented no-op: it only forced no-color and printed the full header +
// per-request results. --quiet must suppress all normal formatter output
// (header + results + summary). Output is captured via --output-file so the
// formatter writes to a file we control instead of os.Stdout.
func TestRun_QuietSuppressesOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "ok.http")
	content := "### Passing\nGET " + server.URL + "\n\n>>>\nexpect status 200\n<<<\n"
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0o644))

	runOnce := func(label string, extra ...string) string {
		outFile := filepath.Join(dir, "out-"+label+".txt")
		args := []string{"run", testFile, "--output-file", outFile}
		args = append(args, extra...)
		rootCmd.SetArgs(args)
		require.NoError(t, rootCmd.Execute())
		b, err := os.ReadFile(outFile)
		if os.IsNotExist(err) {
			return ""
		}
		require.NoError(t, err)
		return string(b)
	}

	normal := runOnce("normal", "--no-color")
	quiet := runOnce("quiet", "--quiet")

	assert.NotEmpty(t, normal, "non-quiet run must produce output")
	assert.Contains(t, normal, "Tests:")
	assert.Empty(t, quiet, "--quiet must suppress all normal output")
}

// TestNewFormatter_WiresOutputFile guards the regression where watch-mode
// re-runs dropped --output-file: newFormatter must wire the provided writer so
// JSON/JUnit/TAP/HTML/console output lands in the file on every run, not just
// the first.
func TestNewFormatter_WiresOutputFile(t *testing.T) {
	var buf bytes.Buffer
	f := newFormatter("json", &buf, 0, false, false)
	if f == nil {
		t.Fatal("newFormatter returned nil")
	}
	f.FormatHeader("9.9.9")
	rr := &runner.RunResult{File: "x.http", Results: []*runner.RequestResult{{Name: "r", Passed: true}}, Passed: 1}
	f.FormatResult(rr)
	if flushable, ok := f.(Flushable); ok {
		_ = flushable.Flush(time.Millisecond)
	} else {
		t.Fatal("JSON formatter should be Flushable")
	}
	out := buf.String()
	if !strings.Contains(out, "x.http") || !strings.Contains(out, "r") {
		t.Errorf("JSON output not written to the provided writer: %q", out)
	}
}

// TestRun_NoDoublePrintedError guards the double-"Error:" regression: when a
// run failed early (missing file), the command called formatter.FormatError
// (writing "Error:" to stdout) AND returned the error, so root's Execute
// wrapper then printed "Error:" again on stderr. The command must not print
// the error itself — root's wrapper is the single printer — so stdout must
// contain no "Error:" line for a missing-file failure.
func TestRun_NoDoublePrintedError(t *testing.T) {
	// Capture os.Stdout (where the console formatter writes) to prove the
	// command itself emits no "Error:" line.
	rOut, wOut, err := os.Pipe()
	require.NoError(t, err)
	origStdout := os.Stdout
	os.Stdout = wOut
	defer func() { os.Stderr = origStdout; os.Stdout = origStdout }()

	rootCmd.SetArgs([]string{"run", "this-file-does-not-exist.http"})
	_ = rootCmd.Execute()
	_ = wOut.Close()
	out, _ := io.ReadAll(rOut)

	assert.NotContains(t, string(out), "Error:", "command must not print the error (root prints it once); got: %s", out)
}
