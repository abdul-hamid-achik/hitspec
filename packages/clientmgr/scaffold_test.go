package clientmgr

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
