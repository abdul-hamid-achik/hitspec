package clientmgr

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/mock"
)

func TestScaffoldSampleCreatesProject(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	created, err := m.ScaffoldSample(ctx)
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("created = %v, want 2 files", created)
	}

	for _, name := range []string{SampleConfigFile, SampleRequestFile} {
		if _, err := os.Stat(filepath.Join(m.config.WorkDir, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}

	// The example file must parse and expose its requests through the workspace.
	ws, err := m.Workspace(ctx)
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	if ws.TotalRequests == 0 {
		t.Fatal("scaffolded example file produced no requests")
	}
}

func TestScaffoldSampleIsIdempotent(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	if _, err := m.ScaffoldSample(ctx); err != nil {
		t.Fatalf("first scaffold: %v", err)
	}
	// A second run must not clobber anything and should report that the files
	// already exist rather than silently overwriting.
	if _, err := m.ScaffoldSample(ctx); err == nil {
		t.Fatal("expected second scaffold to report files already exist")
	}
}

func TestScaffoldSampleRespectsReadOnly(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t, WithReadOnly(true))

	if _, err := m.ScaffoldSample(ctx); err == nil {
		t.Fatal("expected scaffold in read-only mode to fail")
	}
	if _, err := os.Stat(filepath.Join(m.config.WorkDir, SampleConfigFile)); !os.IsNotExist(err) {
		t.Fatalf("read-only scaffold must not create files, stat: %v", err)
	}
}

// TestSampleConfigYAMLParses guards against the regression where the scaffolded
// hitspec.yaml used a duration string ("30s") for the int-millisecond Timeout
// field, which made config.LoadConfig return (nil, err) and panic `hitspec run`
// on a freshly initialised project.
func TestSampleConfigYAMLParses(t *testing.T) {
	// Write the scaffolded YAML to a temp file and load it the same way the
	// CLI does. It must parse without error and expose the expected defaults.
	dir := t.TempDir()
	path := filepath.Join(dir, SampleConfigFile)
	if err := os.WriteFile(path, []byte(SampleConfigYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("scaffolded config must parse cleanly, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("scaffolded config must return a non-nil Config, got nil")
	}
	if cfg.Timeout != config.DefaultTimeoutMs {
		t.Errorf("Timeout = %d, want %d (DefaultTimeoutMs)", cfg.Timeout, config.DefaultTimeoutMs)
	}
	// The scaffolded User-Agent must not be a hardcoded stale version
	// (e.g. "hitspec/1.0") that rots between releases.
	if ua := cfg.Headers["User-Agent"]; ua != "hitspec" {
		t.Errorf("scaffolded User-Agent = %q, want non-stale \"hitspec\"", ua)
	}
}

// TestSampleProjectRunsAgainstMock is the `hitspec init` acceptance test: the
// scaffolded example.http, served by `hitspec mock`, must pass `hitspec run`
// with the scaffolded hitspec.yaml (only the port differs, to avoid clashes).
func TestSampleProjectRunsAgainstMock(t *testing.T) {
	dir := t.TempDir()
	examplePath := filepath.Join(dir, SampleRequestFile)
	if err := os.WriteFile(examplePath, []byte(SampleRequestHTTP), 0o644); err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	srv := mock.NewServer(mock.WithPort(port))
	if err := srv.LoadFiles([]string{examplePath}); err != nil {
		t.Fatal(err)
	}
	if got := len(srv.GetRoutes()); got != 3 {
		t.Fatalf("mock routes = %d, want 3", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.StartWithContext(ctx) }()
	for i := 0; ; i++ {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			_ = c.Close()
			break
		}
		if i > 50 {
			t.Fatalf("mock server did not start: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	r := runner.NewRunner(&runner.Config{
		Environment:    "dev",
		Timeout:        5 * time.Second,
		ValidateSSL:    true,
		FollowRedirect: true,
		ConfigEnvironments: map[string]map[string]any{
			"dev": {"baseUrl": fmt.Sprintf("http://127.0.0.1:%d", port)},
		},
	})
	res, err := r.RunFile(examplePath)
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed != 0 || res.Passed != 3 {
		for _, rr := range res.Results {
			t.Logf("%s passed=%v err=%v", rr.Name, rr.Passed, rr.Error)
		}
		t.Fatalf("passed=%d failed=%d skipped=%d, want 3 passed", res.Passed, res.Failed, res.Skipped)
	}
}
