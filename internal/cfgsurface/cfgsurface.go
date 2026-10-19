// Package cfgsurface is test support for the "which surface honors which
// hitspec.yaml key" characterization tables.
//
// hitspec turns a hitspec.yaml (plus flags and HITSPEC_* variables) into HTTP
// settings on several surfaces: `hitspec run`, the studio Manager, the
// `serve --api-only` handlers, and the stress and contract engines. Each
// surface's test resolves the same scenarios into a Settings value and compares
// the whole table against a golden JSON file, so a change in what any surface
// actually uses shows up as a reviewable diff.
package cfgsurface

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

var update = flag.Bool("update-surfaces", false, "rewrite the config-surface golden files")

// Settings is the effective configuration a surface ends up using.
type Settings struct {
	Environment     string            `json:"environment,omitempty"`
	Timeout         string            `json:"timeout"`
	FollowRedirects bool              `json:"followRedirects"`
	MaxRedirects    int               `json:"maxRedirects"`
	ValidateSSL     bool              `json:"validateSSL"`
	Proxy           string            `json:"proxy,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	Retries         int               `json:"retries"`
	RetryDelay      string            `json:"retryDelay"`
	Parallel        *bool             `json:"parallel,omitempty"`
	Concurrency     *int              `json:"concurrency,omitempty"`
	Bail            *bool             `json:"bail,omitempty"`
}

// Scenario is one input combination every surface is resolved against.
type Scenario struct {
	Name string
	// YAML is the hitspec.yaml content; empty means no config file.
	YAML string
	// Env is set as HITSPEC_* environment variables.
	Env map[string]string
	// Flags are `hitspec run` arguments. Only surfaces with flags use them.
	Flags []string
}

const fullYAML = `defaultEnvironment: staging
timeout: 2500
retries: 3
retryDelay: 250
followRedirects: false
maxRedirects: 4
validateSSL: %s
proxy: http://yaml-proxy.invalid:8080
headers:
  X-From: yaml
  Accept: application/yaml
parallel: true
concurrency: 7
bail: true
environments:
  staging:
    baseUrl: http://staging.invalid
`

func yaml(validateSSL string) string { return fmt.Sprintf(fullYAML, validateSSL) }

var envVars = map[string]string{
	"HITSPEC_TIMEOUT":     "7s",
	"HITSPEC_PROXY":       "http://env-proxy.invalid:8080",
	"HITSPEC_CONCURRENCY": "2",
	"HITSPEC_PARALLEL":    "false",
	"HITSPEC_BAIL":        "false",
}

// Scenarios returns the shared input matrix.
func Scenarios() []Scenario {
	return []Scenario{
		{Name: "defaults"},
		{Name: "yaml", YAML: yaml("false")},
		{Name: "yaml+env", YAML: yaml("false"), Env: envVars},
		{
			Name: "yaml+env+flags", YAML: yaml("true"), Env: envVars,
			Flags: []string{"--timeout", "9s", "--proxy", "http://flag-proxy.invalid:8080", "--insecure", "--concurrency", "3", "--parallel", "--bail"},
		},
	}
}

// Setup writes the scenario's hitspec.yaml into a fresh directory, sets its
// environment variables for the test, and returns the directory.
func Setup(t *testing.T, sc Scenario) string {
	t.Helper()
	for _, k := range []string{"HITSPEC_TIMEOUT", "HITSPEC_PROXY", "HITSPEC_INSECURE", "HITSPEC_CONCURRENCY", "HITSPEC_PARALLEL", "HITSPEC_BAIL", "HITSPEC_ENV", "HITSPEC_CONFIG"} {
		t.Setenv(k, "")
	}
	for k, v := range sc.Env {
		t.Setenv(k, v)
	}
	dir := t.TempDir()
	if sc.YAML != "" {
		if err := os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte(sc.YAML), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Bool and Int return pointers for the optional Settings fields.
func Bool(b bool) *bool { return &b }
func Int(i int) *int    { return &i }

// Bare is what a surface uses when it builds its HTTP client with no options:
// the http package defaults.
func Bare() Settings {
	return Settings{
		Timeout:         http.DefaultTimeout.String(),
		FollowRedirects: true,
		MaxRedirects:    http.DefaultMaxRedirects,
		ValidateSSL:     true,
		RetryDelay:      (time.Duration(runner.DefaultRetryDelayMs) * time.Millisecond).String(),
	}
}

// Check compares table against the golden file at path (rewriting it under
// -update-surfaces).
func Check(t *testing.T, path string, table map[string]map[string]Settings) {
	t.Helper()
	got, err := json.MarshalIndent(table, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update-surfaces to create it)", err)
	}
	if string(want) != string(got) {
		t.Errorf("config surface table differs from %s (run with -update-surfaces to accept):\n--- got ---\n%s", path, got)
	}
}

// FromRunner reports what a runner.Config makes the runner do. The runner
// cannot be told a redirect limit or default retries, so those are the
// implicit http/runner defaults.
func FromRunner(cfg *runner.Config) Settings {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = http.DefaultTimeout
	}
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = runner.DefaultConcurrency
	}
	s := Bare()
	s.Environment = cfg.Environment
	s.Timeout = timeout.String()
	s.FollowRedirects = cfg.FollowRedirect
	s.ValidateSSL = cfg.ValidateSSL
	s.Proxy = cfg.Proxy
	if len(cfg.DefaultHeaders) > 0 {
		s.Headers = cfg.DefaultHeaders
	}
	s.Parallel = Bool(cfg.Parallel)
	s.Concurrency = Int(concurrency)
	s.Bail = Bool(cfg.Bail)
	return s
}
