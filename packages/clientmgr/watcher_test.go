package clientmgr

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// waitForFileEvent drains events until one for wantPath arrives.
func waitForFileEvent(t *testing.T, events <-chan Event, wantPath, wantOp string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-events:
			fe, ok := ev.Payload.(FileEvent)
			if ev.Type != "file_changed" || !ok {
				continue
			}
			if filepath.ToSlash(fe.Path) == wantPath && fe.Operation == wantOp {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s event on %s", wantOp, wantPath)
		}
	}
}

func startWatchedManager(t *testing.T) (string, <-chan Event) {
	t.Helper()
	dir := t.TempDir()
	m := New(
		WithWorkDir(dir),
		WithWatch(true),
		WithHistoryDBPath(filepath.Join(t.TempDir(), "history.db")),
	)
	ctx := context.Background()
	events := m.Subscribe(ctx)
	m.Start(ctx)
	t.Cleanup(func() { _ = m.Close() })
	return dir, events
}

// A file created inside a nested directory made after startup must still be
// reported: the watcher has to add every new subdirectory, not only the top.
func TestWatcherReportsFilesInNewNestedDirs(t *testing.T) {
	dir, events := startWatchedManager(t)

	nested := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	// Give the watcher a moment to add the new tree before writing into it.
	time.Sleep(200 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(nested, "deep.http"), []byte("GET http://x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForFileEvent(t, events, "a/b/c/deep.http", "created")
}

// Files that already exist when a directory is moved into the workspace
// (cp -R, git checkout, unzip) produce no per-file fsnotify event, so the
// watcher must synthesize "created" for them.
func TestWatcherReportsFilesInDirMovedIntoWorkspace(t *testing.T) {
	dir, events := startWatchedManager(t)

	staging := filepath.Join(t.TempDir(), "suite")
	if err := os.MkdirAll(filepath.Join(staging, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "inner", "moved.http"), []byte("GET http://x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(staging, filepath.Join(dir, "suite")); err != nil {
		t.Fatal(err)
	}
	waitForFileEvent(t, events, "suite/inner/moved.http", "created")
}
