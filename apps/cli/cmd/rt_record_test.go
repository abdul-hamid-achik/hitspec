package cmd

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecord_BusyPortIsAnError(t *testing.T) {
	resetRootCommandForTest(t)
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port

	rootCmd.SetArgs([]string{"record", "--target", "http://127.0.0.1:1", "--port", strconv.Itoa(port)})
	err = rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), strconv.Itoa(port))
}

func TestRecord_FlagDefaults(t *testing.T) {
	host := recordCmd.Flags().Lookup("host")
	require.NotNil(t, host)
	assert.Equal(t, "127.0.0.1", host.DefValue)
	nr := recordCmd.Flags().Lookup("no-redact")
	require.NotNil(t, nr)
	assert.Equal(t, "false", nr.DefValue)
}
