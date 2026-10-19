package cmd

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A busy --metrics-port is a startup error (exit 3, like a config problem):
// the stress run must not start and carry on without its metrics endpoint.
func TestRunStress_BusyMetricsPortStopsTheRun(t *testing.T) {
	resetRootCommandForTest(t)
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port

	file := filepath.Join(t.TempDir(), "api.http")
	require.NoError(t, os.WriteFile(file, []byte("### A\nGET http://127.0.0.1:1/a\n"), 0o644))

	var stderr, stdout bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"run", file, "--no-history", "--stress", "--duration", "30s", "--rate", "1",
		"--metrics", "prometheus", "--metrics-port", strconv.Itoa(port)})
	start := time.Now()
	err = rootCmd.Execute()
	require.Error(t, err)
	coder, ok := err.(ExitCoder)
	require.True(t, ok, "want an ExitCoder, got %T: %v", err, err)
	assert.Equal(t, ExitConfigError, coder.ExitCode())
	assert.Contains(t, stderr.String(), strconv.Itoa(port))
	assert.Contains(t, stderr.String(), "already in use")
	assert.NotContains(t, stdout.String(), "available at", "must not announce an endpoint that is not up")
	assert.Less(t, time.Since(start), 10*time.Second, "the stress run must not have started")
}
