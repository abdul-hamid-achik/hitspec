package portprobe

import (
	"net"
	"testing"
)

func TestInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if !InUse(port) {
		t.Errorf("port %d is served but InUse = false", port)
	}
	_ = ln.Close()
	if InUse(port) {
		t.Errorf("port %d is free but InUse = true", port)
	}
	if InUse(0) {
		t.Error("port 0 is never in use")
	}
}

func TestShouldProbe(t *testing.T) {
	for host, want := range map[string]bool{
		"": true, "localhost": true, "127.0.0.1": true, "::1": true, "0.0.0.0": true, "::": true,
		"192.168.1.5": false, "10.0.0.1": false,
	} {
		if got := ShouldProbe(host); got != want {
			t.Errorf("ShouldProbe(%q) = %v, want %v", host, got, want)
		}
	}
}
