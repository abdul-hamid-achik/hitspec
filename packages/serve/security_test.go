package serve

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// startSecured serves s through its real middleware chain (buildHandler) on a
// loopback httptest listener, so Host/Origin/token behavior is exercised the way
// Start wires it. It returns the base URL and the listening port.
func startSecured(t *testing.T, opts ...Option) (*Server, string, int) {
	t.Helper()
	defaults := []Option{
		WithHistoryDBPath(filepath.Join(t.TempDir(), "history.db")),
		WithWatch(false),
	}
	s := newTestServer(t, append(defaults, opts...)...)
	ts := httptest.NewUnstartedServer(nil)
	p := ts.Listener.Addr().(*net.TCPAddr).Port
	ts.Config.Handler = s.buildHandler(p)
	ts.Start()
	t.Cleanup(ts.Close)
	return s, ts.URL, p
}

type reqOpt func(*http.Request)

func withHeader(k, v string) reqOpt { return func(r *http.Request) { r.Header.Set(k, v) } }
func withHost(h string) reqOpt      { return func(r *http.Request) { r.Host = h } }

func do(t *testing.T, method, url, body string, opts ...reqOpt) *http.Response {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range opts {
		o(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestJSONContentTypeEnforced(t *testing.T) {
	_, base, _ := startSecured(t)
	body := `{"name":"production"}`

	// A CORS "simple request": text/plain needs no preflight, so a web page can
	// send it cross-origin. It must not reach a JSON endpoint.
	for _, ct := range []string{"text/plain", "text/plain;charset=UTF-8", "application/x-www-form-urlencoded", "multipart/form-data; boundary=x", ""} {
		opts := []reqOpt{}
		if ct != "" {
			opts = append(opts, withHeader("Content-Type", ct))
		} else {
			opts = append(opts, func(r *http.Request) { r.Header.Del("Content-Type") })
		}
		resp := do(t, http.MethodPut, base+"/api/v1/environments/active", body, opts...)
		if resp.StatusCode != http.StatusUnsupportedMediaType {
			t.Errorf("Content-Type %q: status = %d, want 415", ct, resp.StatusCode)
		}
	}

	for _, ct := range []string{"application/json", "application/json; charset=utf-8", "Application/JSON"} {
		resp := do(t, http.MethodPut, base+"/api/v1/environments/active", body, withHeader("Content-Type", ct))
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Content-Type %q: status = %d, want 204", ct, resp.StatusCode)
		}
	}
}

func TestBodylessPostStillWorks(t *testing.T) {
	_, base, _ := startSecured(t)
	// /stop endpoints take no body; neither "no Content-Type" nor a stray one may
	// turn them into a 415.
	for _, ct := range []string{"", "text/plain"} {
		opts := []reqOpt{}
		if ct != "" {
			opts = append(opts, withHeader("Content-Type", ct))
		}
		resp := do(t, http.MethodPost, base+"/api/v1/stress/stop", "", opts...)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("bodyless POST /stress/stop (Content-Type %q) = %d, want 200", ct, resp.StatusCode)
		}
	}
}

func TestRawFileSaveKeepsTextPlain(t *testing.T) {
	s, base, _ := startSecured(t)
	path := filepath.Join(s.config.WorkDir, "a.http")
	if err := os.WriteFile(path, []byte("### a\nGET https://example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := do(t, http.MethodPut, base+"/api/v1/files/a.http", "### b\nGET https://example.com/b\n", withHeader("Content-Type", "text/plain; charset=utf-8"))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("raw PUT status = %d, want 200", resp.StatusCode)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "/b") {
		t.Fatalf("file not saved: %q", got)
	}
}

func TestHostAllowlist(t *testing.T) {
	_, base, port := startSecured(t)
	p := strconv.Itoa(port)

	for host, want := range map[string]int{
		"localhost:" + p:       http.StatusOK,
		"127.0.0.1:" + p:       http.StatusOK,
		"[::1]:" + p:           http.StatusOK,
		"localhost":            http.StatusOK,
		"127.0.0.1":            http.StatusOK,
		"[::1]":                http.StatusOK,
		"LOCALHOST:" + p:       http.StatusOK,
		"rebind.evil.com":      http.StatusForbidden,
		"rebind.evil.com:" + p: http.StatusForbidden,
		"localhost:1":          http.StatusForbidden, // wrong port
		"localhost.evil.com":   http.StatusForbidden,
		"127.0.0.2:" + p:       http.StatusForbidden,
		"0.0.0.0:" + p:         http.StatusForbidden,
		"::1":                  http.StatusForbidden, // malformed (unbracketed IPv6)
	} {
		resp := do(t, http.MethodGet, base+"/api/v1/system/info", "", withHost(host))
		if resp.StatusCode != want {
			t.Errorf("Host %q: status = %d, want %d", host, resp.StatusCode, want)
		}
	}
}

func TestHostNotCheckedOnNonLoopbackBind(t *testing.T) {
	s := newTestServer(t, WithHost("0.0.0.0"), WithHistoryDBPath(filepath.Join(t.TempDir(), "h.db")))
	h := s.buildHandler(4000)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
	req.Host = "myserver.lan:4000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("non-loopback bind: status = %d, want 200 (Host cannot be checked; token covers it)", rec.Code)
	}
}

func TestOriginPolicy(t *testing.T) {
	t.Run("without --cors", func(t *testing.T) {
		_, base, port := startSecured(t)
		p := strconv.Itoa(port)
		for origin, want := range map[string]int{
			"":                             http.StatusOK, // CLI, curl, Node fetch
			"http://evil.example":          http.StatusForbidden,
			"https://localhost.evil.com":   http.StatusForbidden,
			"http://localhost:" + p:        http.StatusOK,
			"http://127.0.0.1:" + p:        http.StatusOK,
			"http://[::1]:" + p:            http.StatusOK,
			"http://localhost:5173":        http.StatusForbidden, // other localhost port needs --cors
			"null":                         http.StatusForbidden,
			"http://localhost:" + p + "/x": http.StatusForbidden,
		} {
			opts := []reqOpt{}
			if origin != "" {
				opts = append(opts, withHeader("Origin", origin))
			}
			resp := do(t, http.MethodGet, base+"/api/v1/system/info", "", opts...)
			if resp.StatusCode != want {
				t.Errorf("Origin %q: status = %d, want %d", origin, resp.StatusCode, want)
			}
		}
	})

	t.Run("with --cors", func(t *testing.T) {
		_, base, _ := startSecured(t, WithCORS(true))
		resp := do(t, http.MethodGet, base+"/api/v1/system/info", "", withHeader("Origin", "http://localhost:5173"))
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("allowed origin: status = %d, want 200", resp.StatusCode)
		}
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
			t.Errorf("Access-Control-Allow-Origin = %q", got)
		}
		resp = do(t, http.MethodGet, base+"/api/v1/system/info", "", withHeader("Origin", "http://evil.example"))
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("evil origin with --cors: status = %d, want 403", resp.StatusCode)
		}
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("evil origin must get no ACAO header, got %q", got)
		}
	})

	t.Run("evil origin cannot trigger a mutating simple request", func(t *testing.T) {
		_, base, _ := startSecured(t, WithCORS(true))
		resp := do(t, http.MethodPost, base+"/api/v1/execute", `{"file":"x"}`,
			withHeader("Content-Type", "text/plain"), withHeader("Origin", "http://evil.example"))
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", resp.StatusCode)
		}
	})
}

func TestWebSocketOrigin(t *testing.T) {
	_, base, port := startSecured(t)
	wsURL := "ws" + strings.TrimPrefix(base, "http") + "/api/v1/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dial := func(origin string) error {
		h := http.Header{}
		if origin != "" {
			h.Set("Origin", origin)
		}
		c, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: h})
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		if err == nil {
			_ = c.CloseNow()
		}
		return err
	}

	if err := dial("http://evil.example"); err == nil {
		t.Error("cross-site WebSocket upgrade with an evil Origin must be rejected")
	}
	if err := dial("http://localhost:1"); err == nil {
		t.Error("WebSocket upgrade from another localhost port must be rejected without --cors")
	}
	if err := dial(""); err != nil {
		t.Errorf("WebSocket upgrade without Origin (desktop's ws client): %v", err)
	}
	if err := dial("http://localhost:" + strconv.Itoa(port)); err != nil {
		t.Errorf("WebSocket upgrade from the server's own origin: %v", err)
	}
}

func TestWebSocketOriginWithCORSAndToken(t *testing.T) {
	_, base, _ := startSecured(t, WithCORS(true), WithAPIToken("tok"))
	wsURL := "ws" + strings.TrimPrefix(base, "http") + "/api/v1/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dial := func(url, origin string) error {
		h := http.Header{}
		h.Set("Origin", origin)
		c, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: h})
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		if err == nil {
			_ = c.CloseNow()
		}
		return err
	}
	if err := dial(wsURL+"?token=tok", "http://evil.example"); err == nil {
		t.Error("evil Origin must be rejected even with a valid token")
	}
	if err := dial(wsURL, "http://localhost:5173"); err == nil {
		t.Error("missing token must be rejected")
	}
	if err := dial(wsURL+"?token=tok", "http://localhost:5173"); err != nil {
		t.Errorf("allowed --cors origin with ?token=: %v", err)
	}
}

func TestPreflightBeforeAuth(t *testing.T) {
	_, base, _ := startSecured(t, WithCORS(true), WithAPIToken("s3cret"))

	resp := do(t, http.MethodOptions, base+"/api/v1/execute", "",
		withHeader("Origin", "http://localhost:5173"),
		withHeader("Access-Control-Request-Method", "POST"),
		withHeader("Access-Control-Request-Headers", "authorization, content-type"))
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight without token = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("preflight ACAO = %q", got)
	}
	if !strings.Contains(resp.Header.Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Errorf("preflight must allow the Authorization header, got %q", resp.Header.Get("Access-Control-Allow-Headers"))
	}

	// The real request still needs the token.
	resp = do(t, http.MethodGet, base+"/api/v1/system/info", "", withHeader("Origin", "http://localhost:5173"))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("actual request without token = %d, want 401", resp.StatusCode)
	}
	resp = do(t, http.MethodGet, base+"/api/v1/system/info", "",
		withHeader("Origin", "http://localhost:5173"), withHeader("Authorization", "Bearer s3cret"))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("actual request with token = %d, want 200", resp.StatusCode)
	}

	// A preflight from an evil origin is refused outright.
	resp = do(t, http.MethodOptions, base+"/api/v1/execute", "",
		withHeader("Origin", "http://evil.example"), withHeader("Access-Control-Request-Method", "POST"))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("evil preflight = %d, want 403", resp.StatusCode)
	}
}

func TestBearerSchemeCaseInsensitive(t *testing.T) {
	_, base, _ := startSecured(t, WithAPIToken("s3cret"))
	for auth, want := range map[string]int{
		"Bearer s3cret":  http.StatusOK,
		"bearer s3cret":  http.StatusOK,
		"BEARER s3cret":  http.StatusOK,
		"bearer  s3cret": http.StatusOK,
		"bearer s3cre":   http.StatusUnauthorized,
		"bearer s3crets": http.StatusUnauthorized,
		"bearer":         http.StatusUnauthorized,
		"Basic s3cret":   http.StatusUnauthorized,
		"s3cret":         http.StatusUnauthorized,
	} {
		resp := do(t, http.MethodGet, base+"/api/v1/system/info", "", withHeader("Authorization", auth))
		if resp.StatusCode != want {
			t.Errorf("Authorization %q: status = %d, want %d", auth, resp.StatusCode, want)
		}
	}
	resp := do(t, http.MethodGet, base+"/api/v1/system/info?token=s3cret", "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("?token= = %d, want 200", resp.StatusCode)
	}
}

func TestIsLoopbackBind(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost": true, "LOCALHOST": true, "127.0.0.1": true, "127.0.0.5": true, "::1": true, "[::1]": true,
		"": false, "0.0.0.0": false, "::": false, "192.168.1.5": false, "example.com": false,
	} {
		if got := isLoopbackBind(host); got != want {
			t.Errorf("isLoopbackBind(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestEnsureAPIToken(t *testing.T) {
	hex64 := regexp.MustCompile(`[0-9a-f]{64}`)

	cases := []struct {
		name     string
		opts     []Option
		wantTok  bool
		keepsTok string
	}{
		{"loopback default stays open", nil, false, ""},
		{"loopback ipv6 stays open", []Option{WithHost("::1")}, false, ""},
		{"non-loopback bind", []Option{WithHost("0.0.0.0")}, true, ""},
		{"all interfaces (empty host)", []Option{WithHost("")}, true, ""},
		{"allow-shell", []Option{WithAllowShell(true)}, true, ""},
		{"allow-db", []Option{WithAllowDB(true)}, true, ""},
		{"configured token is kept", []Option{WithHost("0.0.0.0"), WithAllowShell(true), WithAPIToken("mine")}, false, "mine"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(t, append([]Option{WithHistoryDBPath(filepath.Join(t.TempDir(), "h.db"))}, tc.opts...)...)
			var out bytes.Buffer
			s.ensureAPIToken(&out)
			tok := s.config.APIToken
			switch {
			case tc.wantTok:
				if !hex64.MatchString(tok) || len(tok) != 64 {
					t.Fatalf("expected a 32-byte hex token, got %q", tok)
				}
				if strings.Count(out.String(), tok) != 1 {
					t.Fatalf("token must be printed exactly once, output: %q", out.String())
				}
			case tc.keepsTok != "":
				if tok != tc.keepsTok || out.Len() != 0 {
					t.Fatalf("configured token must be kept silently: tok=%q out=%q", tok, out.String())
				}
			default:
				if tok != "" || out.Len() != 0 {
					t.Fatalf("no token expected: tok=%q out=%q", tok, out.String())
				}
			}
		})
	}

	// Two servers must not share a token.
	a := newTestServer(t, WithHost("0.0.0.0"), WithHistoryDBPath(filepath.Join(t.TempDir(), "a.db")))
	b := newTestServer(t, WithHost("0.0.0.0"), WithHistoryDBPath(filepath.Join(t.TempDir(), "b.db")))
	a.ensureAPIToken(io.Discard)
	b.ensureAPIToken(io.Discard)
	if a.config.APIToken == b.config.APIToken {
		t.Fatal("generated tokens must differ")
	}
}

// contractFixture writes a contract file whose interaction declares a provider
// state, and returns a provider that answers 200.
func contractFixture(t *testing.T, workDir string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(workDir, "c.http"), []byte("### x\n# @contract.state ready\nGET {{baseUrl}}/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	t.Cleanup(provider.Close)
	return provider.URL
}

// markerScript writes an executable that creates marker when run.
func markerScript(t *testing.T, dir, marker string) string {
	t.Helper()
	p := filepath.Join(dir, "state.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func verifyBody(t *testing.T, provider, handler string) string {
	t.Helper()
	b := `{"providerUrl":` + strconv.Quote(provider) + `,"stateHandler":` + strconv.Quote(handler) + `}`
	return b
}

func TestContractStateHandler(t *testing.T) {
	jsonCT := withHeader("Content-Type", "application/json")

	t.Run("refused without --allow-shell and never executed", func(t *testing.T) {
		s, base, _ := startSecured(t)
		provider := contractFixture(t, s.config.WorkDir)
		marker := filepath.Join(t.TempDir(), "PWNED")
		script := markerScript(t, s.config.WorkDir, marker)

		resp := do(t, http.MethodPost, base+"/api/v1/contract/verify", verifyBody(t, provider, script), jsonCT)
		if resp.StatusCode != http.StatusForbidden {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want 403; body %s", resp.StatusCode, b)
		}
		b, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(b), "allow-shell") {
			t.Errorf("error should name --allow-shell: %s", b)
		}
		if _, err := os.Stat(marker); err == nil {
			t.Fatal("state handler was executed although --allow-shell is off")
		}
	})

	t.Run("the original text/plain cross-site exploit no longer works", func(t *testing.T) {
		s, base, _ := startSecured(t)
		provider := contractFixture(t, s.config.WorkDir)
		marker := filepath.Join(t.TempDir(), "PWNED")
		script := markerScript(t, s.config.WorkDir, marker)

		resp := do(t, http.MethodPost, base+"/api/v1/contract/verify", verifyBody(t, provider, script),
			withHeader("Content-Type", "text/plain"), withHeader("Origin", "http://evil.example"), withHost("rebind.evil.example"))
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", resp.StatusCode)
		}
		if _, err := os.Stat(marker); err == nil {
			t.Fatal("state handler was executed")
		}
	})

	t.Run("--allow-shell: handler outside the workspace is refused", func(t *testing.T) {
		s, base, _ := startSecured(t, WithAllowShell(true))
		provider := contractFixture(t, s.config.WorkDir)
		outside := t.TempDir()
		marker := filepath.Join(t.TempDir(), "PWNED")
		script := markerScript(t, outside, marker)

		for _, h := range []string{script, "../" + filepath.Base(outside) + "/state.sh", "/bin/sh"} {
			resp := do(t, http.MethodPost, base+"/api/v1/contract/verify", verifyBody(t, provider, h), jsonCT)
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("handler %q: status = %d, want 403", h, resp.StatusCode)
			}
		}
		if _, err := os.Stat(marker); err == nil {
			t.Fatal("state handler outside the workspace was executed")
		}
	})

	t.Run("--allow-shell: symlink escaping the workspace is refused", func(t *testing.T) {
		s, base, _ := startSecured(t, WithAllowShell(true))
		provider := contractFixture(t, s.config.WorkDir)
		marker := filepath.Join(t.TempDir(), "PWNED")
		script := markerScript(t, t.TempDir(), marker)
		link := filepath.Join(s.config.WorkDir, "link.sh")
		if err := os.Symlink(script, link); err != nil {
			t.Skip("symlinks unavailable: " + err.Error())
		}
		resp := do(t, http.MethodPost, base+"/api/v1/contract/verify", verifyBody(t, provider, "link.sh"), jsonCT)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", resp.StatusCode)
		}
		if _, err := os.Stat(marker); err == nil {
			t.Fatal("symlinked state handler was executed")
		}
	})

	t.Run("--allow-shell: handler inside the workspace runs (relative and absolute)", func(t *testing.T) {
		for _, relative := range []bool{true, false} {
			s, base, _ := startSecured(t, WithAllowShell(true))
			provider := contractFixture(t, s.config.WorkDir)
			marker := filepath.Join(t.TempDir(), "ran")
			script := markerScript(t, s.config.WorkDir, marker)
			h := script
			if relative {
				h = "state.sh"
			}
			resp := do(t, http.MethodPost, base+"/api/v1/contract/verify", verifyBody(t, provider, h), jsonCT)
			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				t.Fatalf("relative=%v: status = %d, want 200; %s", relative, resp.StatusCode, b)
			}
			if _, err := os.Stat(marker); err != nil {
				t.Fatalf("relative=%v: state handler did not run: %v", relative, err)
			}
		}
	})

	t.Run("no stateHandler needs no --allow-shell", func(t *testing.T) {
		s, base, _ := startSecured(t)
		provider := contractFixture(t, s.config.WorkDir)
		resp := do(t, http.MethodPost, base+"/api/v1/contract/verify", `{"providerUrl":`+strconv.Quote(provider)+`}`, jsonCT)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
	})
}

// The upgrade handler repeats the Origin policy on its own (defense in depth),
// so it stays safe even if it is ever mounted without the middleware chain.
func TestWebSocketHandlerChecksOriginItself(t *testing.T) {
	s := newTestServer(t, WithHistoryDBPath(filepath.Join(t.TempDir(), "h.db")))
	ts := httptest.NewUnstartedServer(nil)
	s.boundPort = ts.Listener.Addr().(*net.TCPAddr).Port
	ts.Config.Handler = http.HandlerFunc(s.handleWebSocket)
	ts.Start()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	for origin, wantOK := range map[string]bool{
		"http://evil.example":                           false,
		"http://localhost:1":                            false,
		"http://localhost:" + strconv.Itoa(s.boundPort): true,
	} {
		h := http.Header{}
		h.Set("Origin", origin)
		c, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: h})
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		if err == nil {
			_ = c.CloseNow()
		}
		if (err == nil) != wantOK {
			t.Errorf("Origin %q: accepted=%v, want %v (err %v)", origin, err == nil, wantOK, err)
		}
	}
}
