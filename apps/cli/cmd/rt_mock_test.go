package cmd

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMock_BusyPortIsAnError(t *testing.T) {
	resetRootCommandForTest(t)
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port

	file := filepath.Join(t.TempDir(), "api.http")
	require.NoError(t, os.WriteFile(file, []byte("### A\nGET /a\n\n>>>\nexpect status 200\n<<<\n"), 0o644))

	rootCmd.SetArgs([]string{"mock", file, "--port", strconv.Itoa(port)})
	err = rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), strconv.Itoa(port))
	var ec ExitCoder
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, ExitConfigError, ec.ExitCode())
}

func TestMock_HostDefaultsToLoopback(t *testing.T) {
	f := mockCmd.Flags().Lookup("host")
	require.NotNil(t, f)
	assert.Equal(t, "127.0.0.1", f.DefValue)
	require.NotNil(t, mockCmd.Flags().Lookup("cors"))
}
