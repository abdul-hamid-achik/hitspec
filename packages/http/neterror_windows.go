//go:build windows

package http

import (
	"errors"
	"syscall"
)

// isWinsockReset reports a connection the peer reset or aborted. On Windows
// syscall.ECONNRESET and syscall.ECONNABORTED are values Go invents for
// portability: Winsock reports WSAECONNRESET and WSAECONNABORTED, which
// errors.Is never matches with the portable names.
func isWinsockReset(err error) bool {
	return errors.Is(err, syscall.WSAECONNRESET) || errors.Is(err, syscall.WSAECONNABORTED)
}
