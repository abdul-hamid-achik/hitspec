package proxy

import (
	"bufio"
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

func TestDefaultsLoopbackAndRedact(t *testing.T) {
	r := NewRecorder(WithPort(0), WithTargetURL("http://example.invalid"))
	addr, err := r.Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer r.listener.Close()
	if host, _, _ := net.SplitHostPort(addr.String()); host != "127.0.0.1" {
		t.Errorf("default bind = %s, want 127.0.0.1", host)
	}
	if !r.redact {
		t.Error("redaction should default to on")
	}
}

func TestStartWithContext_BusyPortIsAnError(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port

	r := NewRecorder(WithPort(port), WithTargetURL("http://example.invalid"))
	err = r.StartWithContext(context.Background())
	if err == nil {
		t.Fatal("expected an error for a busy port")
	}
	if !strings.Contains(err.Error(), strconv.Itoa(port)) {
		t.Errorf("error should name the port: %v", err)
	}
}

// An SSE response must reach the client event by event, while the upstream
// connection is still open, and the recording must still be stored afterwards.
func TestProxyStreamsSSEIncrementally(t *testing.T) {
	release := make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		_, _ = io.WriteString(w, "data: one\n\n")
		w.(http.Flusher).Flush()
		<-release
		_, _ = io.WriteString(w, "data: two\n\n")
	}))
	defer backend.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()

	rec, proxyURL, _ := startRecorderProxy(t, backend)
	resp, err := http.Get(proxyURL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	br := bufio.NewReader(resp.Body)
	lineCh := make(chan string, 1)
	go func() {
		l, _ := br.ReadString('\n')
		lineCh <- l
	}()
	select {
	case l := <-lineCh:
		if l != "data: one\n" {
			t.Fatalf("first line = %q", l)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first SSE event was buffered until the stream ended")
	}
	if n := len(rec.GetRecordings()); n != 0 {
		t.Errorf("recording stored before the stream ended (%d)", n)
	}

	close(release)
	rest, _ := io.ReadAll(br)
	if !strings.Contains(string(rest), "data: two") {
		t.Errorf("second event missing: %q", rest)
	}
	deadline := time.Now().Add(3 * time.Second)
	for len(rec.GetRecordings()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	got := rec.GetRecordings()
	if len(got) != 1 {
		t.Fatalf("recordings = %d, want 1", len(got))
	}
	if !strings.Contains(got[0].Response.Body, "data: one") || !strings.Contains(got[0].Response.Body, "data: two") {
		t.Errorf("recorded body = %q", got[0].Response.Body)
	}
}

func TestRecordedBodyIsCapped(t *testing.T) {
	big := strings.Repeat("x", 5000)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, big)
	}))
	defer backend.Close()

	rec, proxyURL, _ := startRecorderProxy(t, backend, WithMaxBodyBytes(100))
	resp, err := http.Get(proxyURL + "/big")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if len(body) != len(big) {
		t.Fatalf("client received %d bytes, want the full %d", len(body), len(big))
	}
	got := rec.GetRecordings()
	if len(got) != 1 {
		t.Fatalf("recordings = %d", len(got))
	}
	if len(got[0].Response.Body) != 100 || !got[0].Response.Truncated {
		t.Errorf("recorded %d bytes truncated=%v, want 100/true", len(got[0].Response.Body), got[0].Response.Truncated)
	}
}

func redactBackend(t *testing.T) *httptest.Server {
	t.Helper()
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s3cr3t-session"})
		_, _ = io.WriteString(w, `{"user":"ann","access_token":"tok-123","profile":{"apiKey":"k-9","password":"p"},"tokenType":"Bearer"}`)
	}))
	t.Cleanup(b.Close)
	return b
}

func doRedactRequest(t *testing.T, proxyURL string) {
	t.Helper()
	req, _ := http.NewRequest("POST", proxyURL+"/login", strings.NewReader(`{"username":"ann", "password": "hunter2", "remember": true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer abc")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
}

func TestRedactsSecretsByDefault(t *testing.T) {
	rec, proxyURL, _ := startRecorderProxy(t, redactBackend(t))
	doRedactRequest(t, proxyURL)
	got := rec.GetRecordings()
	if len(got) != 1 {
		t.Fatalf("recordings = %d", len(got))
	}
	r := got[0]
	if strings.Contains(r.Body, "hunter2") || !strings.Contains(r.Body, `"password": "{{PASSWORD}}"`) {
		t.Errorf("request body not redacted (or reformatted): %s", r.Body)
	}
	if !strings.Contains(r.Body, `"remember": true`) || !strings.Contains(r.Body, `"username":"ann"`) {
		t.Errorf("non-secret fields should be untouched: %s", r.Body)
	}
	if r.Headers["Authorization"] != "{{AUTHORIZATION}}" {
		t.Errorf("Authorization = %q", r.Headers["Authorization"])
	}
	rb := r.Response.Body
	for _, leak := range []string{"tok-123", "k-9", `"p"`} {
		if strings.Contains(rb, leak) {
			t.Errorf("response body leaks %s: %s", leak, rb)
		}
	}
	if !strings.Contains(rb, `"access_token":"{{ACCESS_TOKEN}}"`) || !strings.Contains(rb, `"apiKey":"{{API_KEY}}"`) || !strings.Contains(rb, `"tokenType":"Bearer"`) {
		t.Errorf("unexpected redacted response body: %s", rb)
	}
	if strings.Contains(r.Response.Headers["Set-Cookie"], "s3cr3t") {
		t.Errorf("Set-Cookie leaks: %q", r.Response.Headers["Set-Cookie"])
	}
}

func TestNoRedactKeepsEverything(t *testing.T) {
	rec, proxyURL, _ := startRecorderProxy(t, redactBackend(t), WithRedact(false))
	doRedactRequest(t, proxyURL)
	r := rec.GetRecordings()[0]
	if !strings.Contains(r.Body, "hunter2") || r.Headers["Authorization"] != "Bearer abc" {
		t.Errorf("--no-redact should keep request secrets: %q %v", r.Body, r.Headers)
	}
	if !strings.Contains(r.Response.Body, "tok-123") || !strings.Contains(r.Response.Headers["Set-Cookie"], "s3cr3t-session") {
		t.Errorf("--no-redact should keep response secrets: %v", r.Response)
	}
}

func TestRedactJSON(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"password":"a"}`, `{"password":"{{PASSWORD}}"}`},
		{`{ "Secret" : { "x": [1,"}"] } , "k":1}`, `{ "Secret" : "{{SECRET}}" , "k":1}`},
		{`[{"token": null},{"token": 12.5}]`, `[{"token": "{{TOKEN}}"},{"token": "{{TOKEN}}"}]`},
		{`{"client_secret":"s","note":"password: no"}`, `{"client_secret":"{{CLIENT_SECRET}}","note":"password: no"}`},
		{`{"a":"b \" password","tokens":["x"]}`, `{"a":"b \" password","tokens":["x"]}`},
		{`{"password":"unterm`, `{"password":"{{PASSWORD}}"`},
		{`{"password":"a"}`, `{"password":"{{PASSWORD}}"}`},
	}
	for _, c := range cases {
		if got := redactJSON(c.in); got != c.want {
			t.Errorf("redactJSON(%s)\n got %s\nwant %s", c.in, got, c.want)
		}
	}
}

func TestRecordingURLWithTrailingSlashTargetPrefix(t *testing.T) {
	var upstream string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream = r.URL.RequestURI()
	}))
	defer backend.Close()
	rec, proxyURL, _ := startRecorderProxy(t, backend, WithTargetURL(backend.URL+"/v2/api/"))
	resp, err := http.Get(proxyURL + "/things/7?a=b")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	got := rec.GetRecordings()
	if len(got) != 1 {
		t.Fatalf("recordings = %d", len(got))
	}
	if want := backend.URL + upstream; got[0].URL != want {
		t.Errorf("recorded URL %q does not match the proxied one %q", got[0].URL, want)
	}
}
