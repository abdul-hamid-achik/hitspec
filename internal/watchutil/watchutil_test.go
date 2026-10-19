package watchutil

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingAdder struct{ dirs []string }

func (r *recordingAdder) Add(name string) error {
	r.dirs = append(r.dirs, name)
	return nil
}

func TestAddTreeWalksNestedDirsAndReportsFiles(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"a/b/c", ".git/objects", "node_modules/pkg", "vendor/x"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, d), 0o755))
	}
	files := []string{"top.http", "a/b/mid.hitspec", "a/b/c/deep.http", "a/notes.txt", "node_modules/pkg/skip.http"}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(root, f), []byte("GET http://x\n"), 0o644))
	}

	var w recordingAdder
	var found []string
	require.NoError(t, AddTree(&w, root, func(p string) { found = append(found, p) }))

	rel := func(paths []string) []string {
		out := make([]string, 0, len(paths))
		for _, p := range paths {
			r, err := filepath.Rel(root, p)
			require.NoError(t, err)
			out = append(out, filepath.ToSlash(r))
		}
		sort.Strings(out)
		return out
	}
	assert.Equal(t, []string{".", "a", "a/b", "a/b/c"}, rel(w.dirs))
	assert.Equal(t, []string{"a/b/c/deep.http", "a/b/mid.hitspec", "top.http"}, rel(found))
}

func TestAddTreeRootIsAlwaysAdded(t *testing.T) {
	// A root that is itself named like a skipped dir is still watched: the
	// skip list applies to children only.
	root := filepath.Join(t.TempDir(), "vendor")
	require.NoError(t, os.MkdirAll(root, 0o755))
	var w recordingAdder
	require.NoError(t, AddTree(&w, root, nil))
	assert.Equal(t, []string{root}, w.dirs)
}

func TestOperation(t *testing.T) {
	tests := []struct {
		name    string
		last    fsnotify.Op
		created bool
		want    string
	}{
		{"create only", fsnotify.Create, true, "created"},
		{"create then write (inotify WriteFile)", fsnotify.Write, true, "created"},
		{"write only", fsnotify.Write, false, "changed"},
		{"chmod only", fsnotify.Chmod, false, "changed"},
		{"remove", fsnotify.Remove, false, "deleted"},
		{"create then remove", fsnotify.Remove, true, "deleted"},
		{"rename away", fsnotify.Rename, false, "deleted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Operation(tt.last, tt.created))
		})
	}
}

// waitForEvent returns the first event for target that ContentChanged accepts.
func waitForEvent(t *testing.T, w *fsnotify.Watcher, target string) fsnotify.Event {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev := <-w.Events:
			if SamePath(ev.Name, target) && ContentChanged(ev) {
				return ev
			}
		case err := <-w.Errors:
			t.Fatalf("watcher error: %v", err)
		case <-deadline:
			t.Fatalf("no content-change event for %s", target)
		}
	}
}

func newDirWatcher(t *testing.T, dir string) *fsnotify.Watcher {
	t.Helper()
	w, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	require.NoError(t, w.Add(dir))
	return w
}

func TestContentChanged_PlainWrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "api.http")
	require.NoError(t, os.WriteFile(target, []byte("one"), 0o644))
	w := newDirWatcher(t, dir)

	require.NoError(t, os.WriteFile(target, []byte("two"), 0o644))
	waitForEvent(t, w, target)
}

// An atomic save never writes the target: it renames a finished temp file over
// it. The watcher must still report the target as changed.
func TestContentChanged_AtomicSave(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "api.http")
	require.NoError(t, os.WriteFile(target, []byte("one"), 0o644))
	w := newDirWatcher(t, dir)

	tmp := filepath.Join(dir, ".api.http.tmp123")
	require.NoError(t, os.WriteFile(tmp, []byte("two"), 0o644))
	require.NoError(t, os.Rename(tmp, target))
	waitForEvent(t, w, target)
}

// vim without backupcopy: the original is renamed to a backup, then a fresh
// file is created under the original name.
func TestContentChanged_BackupThenCreate(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "api.http")
	require.NoError(t, os.WriteFile(target, []byte("one"), 0o644))
	w := newDirWatcher(t, dir)

	require.NoError(t, os.Rename(target, target+"~"))
	require.NoError(t, os.WriteFile(target, []byte("two"), 0o644))
	waitForEvent(t, w, target)
}

func TestContentChanged_Ops(t *testing.T) {
	for _, op := range []fsnotify.Op{fsnotify.Write, fsnotify.Create, fsnotify.Rename, fsnotify.Remove, fsnotify.Create | fsnotify.Chmod} {
		assert.True(t, ContentChanged(fsnotify.Event{Name: "x.http", Op: op}), op.String())
	}
	assert.False(t, ContentChanged(fsnotify.Event{Name: "x.http", Op: fsnotify.Chmod}), "a bare chmod changes no content")
}

func TestSamePath(t *testing.T) {
	dir := t.TempDir()
	assert.True(t, SamePath(filepath.Join(dir, "a", "..", "b.http"), filepath.Join(dir, "b.http")))
	assert.False(t, SamePath(filepath.Join(dir, "a.http"), filepath.Join(dir, "b.http")))
	wd, err := os.Getwd()
	require.NoError(t, err)
	assert.True(t, SamePath("hitspec.yaml", filepath.Join(wd, "hitspec.yaml")))
}
