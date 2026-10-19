//go:build windows

package serve

import (
	"errors"
	"syscall"
)

// processAlive reports whether pid is a running process: a handle to a process
// that has exited is signalled, one to a live process is not.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := syscall.OpenProcess(syscall.SYNCHRONIZE|syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		// Access denied: it exists but we may not open it. Anything else
		// (invalid parameter) means the pid is gone.
		return errors.Is(err, syscall.ERROR_ACCESS_DENIED)
	}
	defer syscall.CloseHandle(h) //nolint:errcheck // nothing to do on failure
	event, err := syscall.WaitForSingleObject(h, 0)
	return err == nil && event == syscall.WAIT_TIMEOUT
}
