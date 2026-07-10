package clientmgr

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManager_EnvExplicitNotOverriddenByConfig guards the regression where an
// explicit --env dev was silently overridden by hitspec.yaml's
// defaultEnvironment, because the default flag value "dev" was indistinguishable
// from an explicit `--env dev`. An explicit env must be honored; an unset env
// falls back to defaultEnvironment.
func TestManager_EnvExplicitNotOverriddenByConfig(t *testing.T) {
	dir := t.TempDir()
	// Config defaults to "staging" — the old code would override an explicit
	// "dev" with "staging".
	if err := os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte("defaultEnvironment: staging\nenvironments:\n  dev: {}\n  staging: {}\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Run("explicit env honored", func(t *testing.T) {
		m := New(WithWorkDir(dir), WithWatch(false), WithEnv("dev"), WithHistoryDBPath(filepath.Join(dir, "h1.db")))
		m.Start(context.Background())
		defer m.Close()
		if m.config.Env != "dev" {
			t.Fatalf("explicit --env dev overridden to %q; want dev", m.config.Env)
		}
	})

	t.Run("unset env falls back to defaultEnvironment", func(t *testing.T) {
		m := New(WithWorkDir(dir), WithWatch(false), WithHistoryDBPath(filepath.Join(dir, "h2.db")))
		m.Start(context.Background())
		defer m.Close()
		if m.config.Env != "staging" {
			t.Fatalf("unset env = %q, want fallback to staging", m.config.Env)
		}
	})
}

// TestManager_WithLogWriter verifies the manager's slog logs go to the
// configured writer (not os.Stderr), so the studio TUI can pass io.Discard to
// keep log lines from corrupting the alt screen.
func TestManager_WithLogWriter(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	// Force a history-db open failure so the manager emits a Warn: make the
	// parent path a regular file, not a directory.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	m := New(WithWorkDir(dir), WithWatch(false), WithHistoryDBPath(filepath.Join(blocker, "history.db")), WithLogWriter(&buf))
	m.Start(context.Background())
	defer m.Close()
	if !strings.Contains(buf.String(), "history database") {
		t.Fatalf("log output = %q, want a 'history database' warning routed to the configured writer", buf.String())
	}
}
