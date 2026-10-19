//go:build !windows

package serve

import (
	"errors"
	"syscall"
)

// processAlive reports whether pid is a running process (signal 0 probes
// without sending anything). EPERM means it exists but belongs to someone else.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
