package metrics

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPrometheusListenAddr(t *testing.T) {
	cases := []struct {
		name string
		opts []PrometheusOption
		want string
	}{
		{"default is loopback", []PrometheusOption{WithPrometheusHTTP(9090)}, "127.0.0.1:9090"},
		{"empty host keeps loopback", []PrometheusOption{WithPrometheusHTTP(9090), WithPrometheusHost("")}, "127.0.0.1:9090"},
		{"explicit wildcard", []PrometheusOption{WithPrometheusHTTP(9091), WithPrometheusHost("0.0.0.0")}, "0.0.0.0:9091"},
		{"ipv6", []PrometheusOption{WithPrometheusHTTP(9092), WithPrometheusHost("::1")}, "[::1]:9092"},
	}
	for _, c := range cases {
		p := &PrometheusExporter{}
		for _, o := range c.opts {
			o(p)
		}
		if got := p.listenAddr(); got != c.want {
			t.Errorf("%s: listenAddr = %q, want %q", c.name, got, c.want)
		}
	}
}

// The endpoint answers on loopback and is not bound to the wildcard address.
func TestPrometheusServesOnLoopbackOnly(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	p := NewPrometheusExporter(WithPrometheusHTTP(port))
	t.Cleanup(func() { _ = p.Close() })

	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/metrics"
	var resp *http.Response
	for i := 0; i < 50; i++ {
		if resp, err = http.Get(url); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("metrics endpoint not reachable on loopback: %v", err)
	}
	_ = resp.Body.Close()

	// Every non-loopback address of this machine must refuse the connection.
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok || ipn.IP.IsLoopback() || ipn.IP.IsLinkLocalUnicast() {
			continue
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ipn.IP.String(), strconv.Itoa(port)), time.Second)
		if err == nil {
			_ = conn.Close()
			t.Errorf("metrics endpoint is reachable on %s; it must bind loopback only", ipn.IP)
		}
	}
}

// A busy port is a startup error, reported before the constructor returns.
func TestPrometheusBusyPortIsAnError(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port

	p, err := NewPrometheusExporterE(WithPrometheusHTTP(port))
	if err == nil {
		_ = p.Close()
		t.Fatalf("NewPrometheusExporterE on busy port %d succeeded", port)
	}
	if !strings.Contains(err.Error(), strconv.Itoa(port)) || !strings.Contains(err.Error(), "already in use") {
		t.Errorf("error should name the port and say it is in use, got: %v", err)
	}

	// The legacy constructor keeps working (without an endpoint) and is safe to close.
	legacy := NewPrometheusExporter(WithPrometheusHTTP(port))
	if err := legacy.Close(); err != nil {
		t.Errorf("Close of an exporter without an endpoint: %v", err)
	}
}

// The endpoint is up as soon as the constructor returns, and Close right
// after it must not race with the server being set up (run with -race).
func TestPrometheusBindsBeforeReturningAndClosesCleanly(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	p, err := NewPrometheusExporterE(WithPrometheusHTTP(port))
	if err != nil {
		t.Fatal(err)
	}
	// No retry loop: the listener already exists.
	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/metrics")
	if err != nil {
		t.Fatalf("endpoint not up right after the constructor: %v", err)
	}
	_ = resp.Body.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = p.Export(&AggregateMetrics{StatusCodes: map[int]int64{}, ByTest: map[string]*TestAggregate{}})
	}()
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
	if err := p.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}
