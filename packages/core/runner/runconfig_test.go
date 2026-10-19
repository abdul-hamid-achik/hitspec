package runner

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// flakyServer answers 503 for the first `failures` requests, then 200.
func flakyServer(t *testing.T, failures int32) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) <= failures {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// TestConfigRetriesAreTheDefault: Config.Retries (hitspec.yaml retries) retries
// a request that has no @retry of its own, waiting Config.RetryDelay.
func TestConfigRetriesAreTheDefault(t *testing.T) {
	srv, calls := flakyServer(t, 2)
	path := writeHTTPFile(t, "### flaky\nGET "+srv.URL+"\n")
	cfg := baseConfig()
	cfg.Retries, cfg.RetryDelay = 2, 20*time.Millisecond

	start := time.Now()
	r, err := NewRunner(cfg).RunFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if r.Passed != 1 || calls.Load() != 3 {
		t.Fatalf("passed=%d after %d calls, want pass on the 3rd", r.Passed, calls.Load())
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond || elapsed > 900*time.Millisecond {
		t.Errorf("two retries with a 20ms delay took %v (the 1s built-in delay would be 2s)", elapsed)
	}
}

func TestNoRetriesByDefault(t *testing.T) {
	srv, calls := flakyServer(t, 1)
	path := writeHTTPFile(t, "### flaky\nGET "+srv.URL+"\n")
	r, err := NewRunner(baseConfig()).RunFile(path)
	if err != nil || r.Failed != 1 || calls.Load() != 1 {
		t.Fatalf("err=%v failed=%d calls=%d, want one failed attempt", err, r.Failed, calls.Load())
	}
}

// TestRequestRetryBeatsConfig: a request's @retry/@retryDelay override the
// config defaults.
func TestRequestRetryBeatsConfig(t *testing.T) {
	srv, calls := flakyServer(t, 3)
	path := writeHTTPFile(t, "### flaky\n# @retry 3\n# @retryDelay 10\nGET "+srv.URL+"\n")
	cfg := baseConfig()
	cfg.Retries, cfg.RetryDelay = 1, time.Hour // would never finish if used

	r, err := NewRunner(cfg).RunFile(path)
	if err != nil || r.Passed != 1 || calls.Load() != 4 {
		t.Fatalf("err=%v passed=%d calls=%d, want pass on the 4th call", err, r.Passed, calls.Load())
	}
}

// TestConfigMaxRedirects: Config.MaxRedirects limits redirect following; unset
// keeps the HTTP client's default of 10. (The client stops when it has already
// made MaxRedirects requests, so the exact count is the client's business; the
// test only pins that the config value is what changes it.)
func TestConfigMaxRedirects(t *testing.T) {
	var hops atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops.Add(1)
		http.Redirect(w, r, "/again", http.StatusFound)
	}))
	defer srv.Close()
	path := writeHTTPFile(t, "### loop\nGET "+srv.URL+"\n\n>>>\nexpect status 302\n<<<\n")

	run := func(maxRedirects int) int32 {
		hops.Store(0)
		cfg := baseConfig()
		cfg.MaxRedirects = maxRedirects
		r, err := NewRunner(cfg).RunFile(path)
		if err != nil || r.Passed != 1 {
			t.Fatalf("MaxRedirects=%d: err=%v passed=%d", maxRedirects, err, r.Passed)
		}
		return hops.Load()
	}
	if got := run(0); got < 10 {
		t.Fatalf("default limit made %d requests, want at least 10", got)
	}
	if got := run(3); got >= 10 || got < 3 {
		t.Fatalf("MaxRedirects=3 made %d requests, want it to stop well before the default 10", got)
	}
}

func TestClientOptionsOmitUnset(t *testing.T) {
	if got := len((&Config{}).ClientOptions()); got != 2 { // follow + validateSSL always
		t.Fatalf("zero Config yields %d options, want 2", got)
	}
	full := &Config{Timeout: time.Second, MaxRedirects: 1, Proxy: "http://p", DefaultHeaders: map[string]string{"A": "b"}, MaxResponseBytes: 5}
	if got := len(full.ClientOptions()); got != 7 {
		t.Fatalf("full Config yields %d options, want 7", got)
	}
}
