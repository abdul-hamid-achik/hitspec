package watchutil

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

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
