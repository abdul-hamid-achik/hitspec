//go:build !windows

package http

// isWinsockReset is the Windows-only reset check; elsewhere the portable
// syscall.ECONNRESET and syscall.ECONNABORTED already match.
func isWinsockReset(error) bool { return false }
