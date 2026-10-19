package runconfig

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	hitspechttp "github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

func ptr[T any](v T) *T { return &v }

// fileConfig is a hitspec.yaml that sets every key Build reads, to values that
// differ from the built-in defaults.
func fileConfig() *config.Config {
	return &config.Config{
		DefaultEnvironment: "staging",
		Timeout:            2500,
		Retries:            3,
		RetryDelay:         250,
		FollowRedirects:    ptr(false),
		MaxRedirects:       4,
		ValidateSSL:        ptr(false),
		Proxy:              "http://yaml-proxy:8080",
		Headers:            map[string]string{"X-From": "yaml", "Accept": "text/yaml"},
		Parallel:           ptr(true),
		Concurrency:        7,
		Bail:               ptr(true),
		Environments:       map[string]map[string]any{"staging": {"baseUrl": "http://staging"}},
	}
}

func TestBuildDefaults(t *testing.T) {
	for name, file := range map[string]*config.Config{"nil": nil, "default": config.DefaultConfig(), "zero": {}} {
		t.Run(name, func(t *testing.T) {
			cfg, opts := Build(file, Overrides{})
			want := struct {
				env            string
				timeout        time.Duration
				follow, ssl    bool
				maxRedirects   int
				retries        int
				retryDelay     time.Duration
				parallel       bool
				concurrency    int
				bail           bool
				proxy          string
				headers        int
				optionsAtLeast int
			}{"dev", 30 * time.Second, true, true, 10, 0, time.Second, false, 5, false, "", 0, 4}
			if cfg.Environment != want.env || cfg.Timeout != want.timeout || cfg.FollowRedirect != want.follow ||
				cfg.ValidateSSL != want.ssl || cfg.MaxRedirects != want.maxRedirects || cfg.Retries != want.retries ||
				cfg.RetryDelay != want.retryDelay || cfg.Parallel != want.parallel || cfg.Concurrency != want.concurrency ||
				cfg.Bail != want.bail || cfg.Proxy != want.proxy || len(cfg.DefaultHeaders) != want.headers {
				t.Fatalf("defaults = %+v", cfg)
			}
			if len(opts) < want.optionsAtLeast {
				t.Fatalf("got %d client options, want at least %d", len(opts), want.optionsAtLeast)
			}
		})
	}
}

func TestBuildYAMLBeatsDefault(t *testing.T) {
	cfg, _ := Build(fileConfig(), Overrides{})
	if cfg.Environment != "staging" || cfg.Timeout != 2500*time.Millisecond || cfg.Retries != 3 ||
		cfg.RetryDelay != 250*time.Millisecond || cfg.FollowRedirect || cfg.MaxRedirects != 4 || cfg.ValidateSSL ||
		cfg.Proxy != "http://yaml-proxy:8080" || !cfg.Parallel || cfg.Concurrency != 7 || !cfg.Bail {
		t.Fatalf("yaml values not honored: %+v", cfg)
	}
	if want := map[string]string{"X-From": "yaml", "Accept": "text/yaml"}; !reflect.DeepEqual(cfg.DefaultHeaders, want) {
		t.Fatalf("headers = %v, want %v", cfg.DefaultHeaders, want)
	}
}

func TestBuildOverrideBeatsYAML(t *testing.T) {
	o := Overrides{
		Environment:     "prod",
		EnvFile:         ".env.ci",
		Timeout:         9 * time.Second,
		Retries:         ptr(0),
		RetryDelay:      time.Second,
		FollowRedirects: ptr(true),
		MaxRedirects:    2,
		ValidateSSL:     ptr(true),
		Proxy:           "http://flag-proxy:1",
		Headers:         map[string]string{"X-From": "flag", "X-New": "1"},
		Parallel:        ptr(false),
		Concurrency:     3,
		Bail:            ptr(false),
	}
	cfg, _ := Build(fileConfig(), o)
	if cfg.Environment != "prod" || cfg.EnvFile != ".env.ci" || cfg.Timeout != 9*time.Second || cfg.Retries != 0 ||
		cfg.RetryDelay != time.Second || !cfg.FollowRedirect || cfg.MaxRedirects != 2 || !cfg.ValidateSSL ||
		cfg.Proxy != "http://flag-proxy:1" || cfg.Parallel || cfg.Concurrency != 3 || cfg.Bail {
		t.Fatalf("overrides not honored: %+v", cfg)
	}
	// Headers merge: the override wins per key, other yaml headers stay.
	if want := map[string]string{"X-From": "flag", "Accept": "text/yaml", "X-New": "1"}; !reflect.DeepEqual(cfg.DefaultHeaders, want) {
		t.Fatalf("headers = %v, want %v", cfg.DefaultHeaders, want)
	}
}

func TestBuildTreatsNonPositiveAsUnset(t *testing.T) {
	file := fileConfig()
	file.Timeout, file.RetryDelay, file.MaxRedirects, file.Concurrency, file.Retries = 0, -5, 0, 0, -1
	cfg, _ := Build(file, Overrides{Timeout: -time.Second})
	if cfg.Timeout != 30*time.Second || cfg.RetryDelay != time.Second || cfg.MaxRedirects != 10 || cfg.Concurrency != 5 || cfg.Retries != 0 {
		t.Fatalf("non-positive values should fall back to defaults: %+v", cfg)
	}
}

func TestBuildDoesNotAliasFileConfig(t *testing.T) {
	file := fileConfig()
	cfg, _ := Build(file, Overrides{})
	cfg.DefaultHeaders["X-From"] = "mutated"
	cfg.ConfigEnvironments["staging"]["baseUrl"] = "mutated"
	if file.Headers["X-From"] != "yaml" || file.Environments["staging"]["baseUrl"] != "http://staging" {
		t.Fatal("Build result aliases the config it was built from")
	}
}

// TestClientOptionsApply checks the returned options by behavior: the client
// built from them sends the default headers, enforces the timeout and stops at
// the redirect limit.
func TestClientOptionsApply(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	defer srv.Close()
	mux := srv.Config.Handler.(*http.ServeMux)
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Header.Get("X-From")))
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	})
	mux.HandleFunc("/hop", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/hop", http.StatusFound)
	})

	file := &config.Config{Timeout: 150, MaxRedirects: 2, Headers: map[string]string{"X-From": "yaml"}}
	_, opts := Build(file, Overrides{})
	client := hitspechttp.NewClient(opts...)

	resp, err := client.Do(&hitspechttp.Request{Method: "GET", URL: srv.URL + "/echo"})
	if err != nil || string(resp.Body) != "yaml" {
		t.Fatalf("default header: %v %v", resp, err)
	}
	start := time.Now()
	if _, err := client.Do(&hitspechttp.Request{Method: "GET", URL: srv.URL + "/slow"}); err == nil || time.Since(start) > time.Second {
		t.Fatalf("timeout not enforced (err=%v after %v)", err, time.Since(start))
	}
	resp, err = client.Do(&hitspechttp.Request{Method: "GET", URL: srv.URL + "/hop"})
	if err != nil || resp.StatusCode != http.StatusFound {
		t.Fatalf("redirect limit: %v %v", resp, err)
	}
}

func TestOverPrecedence(t *testing.T) {
	high := Overrides{Timeout: time.Second, Proxy: "high", Parallel: ptr(false), Headers: map[string]string{"A": "high"}}
	low := Overrides{Timeout: 2 * time.Second, Proxy: "low", Parallel: ptr(true), Concurrency: 4, Bail: ptr(true), Headers: map[string]string{"A": "low", "B": "low"}}
	got := high.Over(low)
	if got.Timeout != time.Second || got.Proxy != "high" || *got.Parallel || got.Concurrency != 4 || !*got.Bail {
		t.Fatalf("Over = %+v", got)
	}
	if want := map[string]string{"A": "high", "B": "low"}; !reflect.DeepEqual(got.Headers, want) {
		t.Fatalf("headers = %v, want %v", got.Headers, want)
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{EnvTimeout, EnvProxy, EnvInsecure, EnvParallel, EnvConcurrency, EnvBail} {
		t.Setenv(k, "")
	}
}

func TestFromEnv(t *testing.T) {
	clearEnv(t)
	o, err := FromEnv()
	if err != nil || !reflect.DeepEqual(o, Overrides{}) {
		t.Fatalf("empty env: %+v, %v", o, err)
	}

	t.Setenv(EnvTimeout, "7s")
	t.Setenv(EnvProxy, "http://env-proxy")
	t.Setenv(EnvInsecure, "true")
	t.Setenv(EnvParallel, "false")
	t.Setenv(EnvConcurrency, "2")
	t.Setenv(EnvBail, "1")
	o, err = FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if o.Timeout != 7*time.Second || o.Proxy != "http://env-proxy" || *o.ValidateSSL || *o.Parallel || o.Concurrency != 2 || !*o.Bail {
		t.Fatalf("FromEnv = %+v", o)
	}
}

func TestFromEnvInvalidKeepsValidValues(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvTimeout, "soon")
	t.Setenv(EnvProxy, "http://env-proxy")
	o, err := FromEnv()
	if err == nil {
		t.Fatal("expected an error for an unparsable timeout")
	}
	if o.Proxy != "http://env-proxy" || o.Timeout != 0 {
		t.Fatalf("FromEnv = %+v", o)
	}
}

// TestPrecedenceChain walks one key through all four layers.
func TestPrecedenceChain(t *testing.T) {
	clearEnv(t)
	file := &config.Config{Timeout: 2000}
	timeoutOf := func(flag Overrides) time.Duration {
		env, err := FromEnv()
		if err != nil {
			t.Fatal(err)
		}
		cfg, _ := Build(file, flag.Over(env))
		return cfg.Timeout
	}

	if got := timeoutOf(Overrides{}); got != 2*time.Second {
		t.Fatalf("yaml layer: %v", got)
	}
	t.Setenv(EnvTimeout, "7s")
	if got := timeoutOf(Overrides{}); got != 7*time.Second {
		t.Fatalf("env beats yaml: %v", got)
	}
	if got := timeoutOf(Overrides{Timeout: 9 * time.Second}); got != 9*time.Second {
		t.Fatalf("flag beats env: %v", got)
	}
	file.Timeout = 0
	t.Setenv(EnvTimeout, "")
	if got := timeoutOf(Overrides{}); got != hitspechttp.DefaultTimeout {
		t.Fatalf("default layer: %v", got)
	}
}

func TestResolveEnvironment(t *testing.T) {
	cfg := &config.Config{DefaultEnvironment: "staging"}
	for _, tc := range []struct {
		override string
		file     *config.Config
		want     string
		explicit bool
	}{
		{"prod", cfg, "prod", true},
		{"", cfg, "staging", false},
		{"", &config.Config{}, "dev", false},
		{"", nil, "dev", false},
	} {
		name, explicit := ResolveEnvironment(tc.override, tc.file)
		if name != tc.want || explicit != tc.explicit {
			t.Errorf("ResolveEnvironment(%q) = %q,%v want %q,%v", tc.override, name, explicit, tc.want, tc.explicit)
		}
	}
}

func TestLoadedYAMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hitspec.yaml")
	if err := os.WriteFile(path, []byte("timeout: 1500\nmaxRedirects: 3\nretries: 2\nretryDelay: 50\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := config.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := Build(file, Overrides{})
	if cfg.Timeout != 1500*time.Millisecond || cfg.MaxRedirects != 3 || cfg.Retries != 2 || cfg.RetryDelay != 50*time.Millisecond {
		t.Fatalf("config = %+v", cfg)
	}
}
