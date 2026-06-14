package tui

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// newTestManager builds a fully isolated Manager: a temp workdir, no file
// watcher (deterministic), and a per-test history DB so nothing leaks between
// tests. It registers Close via t.Cleanup.
func newTestManager(t *testing.T) *clientmgr.Manager {
	t.Helper()
	dir := t.TempDir()
	mgr := clientmgr.New(
		clientmgr.WithWorkDir(dir),
		clientmgr.WithWatch(false),
		clientmgr.WithHistoryDBPath(filepath.Join(dir, "history.db")),
	)
	mgr.Start(context.Background())
	t.Cleanup(func() { _ = mgr.Close() })
	return mgr
}
