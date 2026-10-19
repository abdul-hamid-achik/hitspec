// Package portprobe detects a TCP port that is already served on this machine
// even when bind() would still succeed.
//
// On macOS (and with SO_REUSEADDR elsewhere) binding 127.0.0.1:P succeeds while
// another process holds the wildcard address *:P, so a dev server or Docker
// forward would silently shadow, or be shadowed by, the server we are about to
// start. Dialing the loopback addresses first catches that case.
package portprobe

import (
	"net"
	"strconv"
	"time"
)

// dialTimeout bounds each probe; loopback answers (or refuses) immediately.
const dialTimeout = 300 * time.Millisecond

// InUse reports whether something already accepts TCP connections on port at
// 127.0.0.1 or ::1. Port 0 (pick any) is never in use.
func InUse(port int) bool {
	if port <= 0 {
		return false
	}
	for _, host := range []string{"127.0.0.1", "::1"} {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), dialTimeout)
		if err == nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}

// ShouldProbe reports whether a server about to bind host could collide with a
// loopback listener: the loopback names and the unspecified address. A specific
// non-loopback address does not conflict with a loopback-only holder.
func ShouldProbe(host string) bool {
	switch host {
	case "", "localhost":
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsUnspecified())
}
