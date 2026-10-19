package mock

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func addTestRoute(s *Server, method, path string, status int, body string) {
	s.router.AddRoute(&Route{
		Method:      method,
		PathPattern: path,
		PathRegex:   createPathRegex(path),
		Response:    &MockResponse{StatusCode: status, ContentType: "text/plain", Body: body},
	})
}

func do(t *testing.T, method, url string, hdr map[string]string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func TestServer_DefaultsToLoopback(t *testing.T) {
	s := NewServer(WithPort(0))
	addr, err := s.Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer s.listener.Close()
	host, _, _ := net.SplitHostPort(addr.String())
	if host != "127.0.0.1" {
		t.Errorf("default bind = %s, want 127.0.0.1", host)
	}
}

func TestServer_WithHostBindsAllInterfaces(t *testing.T) {
	s := NewServer(WithPort(0), WithHost("0.0.0.0"))
	addr, err := s.Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer s.listener.Close()
	if ip := addr.(*net.TCPAddr).IP; !ip.IsUnspecified() {
		t.Errorf("bind = %s, want unspecified (0.0.0.0)", ip)
	}
}

func TestServer_BusyPortIsAnError(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port

	s := NewServer(WithPort(port))
	err = s.StartWithContext(context.Background())
	if err == nil {
		t.Fatal("StartWithContext on a busy port returned nil")
	}
	if !strings.Contains(err.Error(), strconv.Itoa(port)) {
		t.Errorf("error should name the port: %v", err)
	}
}

func TestServer_StartReportsAddrAndServes(t *testing.T) {
	s := NewServer(WithPort(0))
	addTestRoute(s, "GET", "/x", 200, "hi")
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.StartWithContext(ctx) }()
	var addr net.Addr
	for i := 0; i < 100 && addr == nil; i++ {
		addr = s.Addr()
		time.Sleep(10 * time.Millisecond)
	}
	if addr == nil {
		t.Fatal("never bound")
	}
	resp, body := do(t, "GET", "http://"+addr.String()+"/x", nil)
	if resp.StatusCode != 200 || body != "hi" {
		t.Errorf("got %d %q", resp.StatusCode, body)
	}
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("shutdown error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no shutdown")
	}
	if s.Addr() != nil {
		t.Error("Addr should be nil after shutdown")
	}
}

func TestHandler_405SetsAllowHeader(t *testing.T) {
	s := NewServer()
	addTestRoute(s, "GET", "/items", 200, "list")
	addTestRoute(s, "POST", "/items", 201, "made")
	ts := httptest.NewServer(http.HandlerFunc(s.handleRequest))
	defer ts.Close()

	resp, _ := do(t, "DELETE", ts.URL+"/items", nil)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
	if got := resp.Header.Get("Allow"); got != "GET, HEAD, POST" {
		t.Errorf("Allow = %q, want %q", got, "GET, HEAD, POST")
	}
	resp, _ = do(t, "GET", ts.URL+"/nope", nil)
	if resp.StatusCode != 404 || resp.Header.Get("Allow") != "" {
		t.Errorf("unknown path: %d, Allow=%q", resp.StatusCode, resp.Header.Get("Allow"))
	}
}

func TestHandler_HEADServedFromGET(t *testing.T) {
	s := NewServer()
	addTestRoute(s, "GET", "/items", 200, "list-body")
	ts := httptest.NewServer(http.HandlerFunc(s.handleRequest))
	defer ts.Close()

	resp, body := do(t, "HEAD", ts.URL+"/items", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("HEAD status = %d, want 200", resp.StatusCode)
	}
	if body != "" {
		t.Errorf("HEAD body = %q, want empty", body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("HEAD Content-Type = %q", ct)
	}
}

func TestHandler_CORSOptIn(t *testing.T) {
	// Off by default.
	s := NewServer()
	addTestRoute(s, "GET", "/items", 200, "x")
	ts := httptest.NewServer(http.HandlerFunc(s.handleRequest))
	defer ts.Close()
	resp, _ := do(t, "GET", ts.URL+"/items", map[string]string{"Origin": "http://app.test"})
	if resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("CORS headers present without WithCORS")
	}

	c := NewServer(WithCORS(true))
	addTestRoute(c, "GET", "/items", 200, "x")
	cts := httptest.NewServer(http.HandlerFunc(c.handleRequest))
	defer cts.Close()

	resp, _ = do(t, "GET", cts.URL+"/items", map[string]string{"Origin": "http://app.test"})
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("simple request missing ACAO: %v", resp.Header)
	}
	resp, _ = do(t, "OPTIONS", cts.URL+"/items", map[string]string{
		"Origin":                         "http://app.test",
		"Access-Control-Request-Method":  "PUT",
		"Access-Control-Request-Headers": "X-Custom, Content-Type",
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Headers"); got != "X-Custom, Content-Type" {
		t.Errorf("Allow-Headers = %q", got)
	}
	if !strings.Contains(resp.Header.Get("Access-Control-Allow-Methods"), "PUT") {
		t.Errorf("Allow-Methods = %q", resp.Header.Get("Access-Control-Allow-Methods"))
	}
}

// A holder on the wildcard address (a dev server, a Docker port forward) must
// count as "port in use" even where bind() on 127.0.0.1 would still succeed
// (macOS with SO_REUSEADDR).
func TestServer_WildcardHolderIsBusy(t *testing.T) {
	holder, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	port := holder.Addr().(*net.TCPAddr).Port

	s := NewServer(WithPort(port))
	if _, err := s.Listen(); err == nil {
		_ = s.listener.Close()
		t.Fatalf("Listen succeeded on port %d held by a wildcard listener", port)
	}
}
