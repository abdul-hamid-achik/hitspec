//go:build windows

package http

import (
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A server that rejects the client certificate after the TLS 1.3 handshake
// may reset the connection before its alert is read. Winsock reports that as
// WSAECONNRESET ("wsasend: An existing connection was forcibly closed by the
// remote host"), which syscall.ECONNRESET does not match on Windows.
func TestIsNetworkError_WinsockReset(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.WSAECONNRESET, syscall.WSAECONNABORTED} {
		err := &url.Error{Op: "Get", URL: "https://127.0.0.1:1", Err: &net.OpError{
			Op: "write", Net: "tcp", Err: os.NewSyscallError("wsasend", errno),
		}}
		assert.True(t, IsNetworkError(err), "%v", err)
	}
}
