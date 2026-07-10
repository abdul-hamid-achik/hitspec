package clientmgr

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/core/config"
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
