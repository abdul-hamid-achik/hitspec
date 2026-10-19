// Package watchutil holds the directory-walking helpers shared by the
// workspace file watchers in packages/clientmgr and packages/serve.
package watchutil

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// skippedDirs are never watched: they are large, rarely hold hitspec files
// and (with kqueue on macOS) cost one file descriptor per entry.
var skippedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
}

// SkipDir reports whether a directory with the given base name should be
// excluded from watching.
func SkipDir(name string) bool {
	return skippedDirs[name]
}

// IsHitspecFile reports whether path has a .http or .hitspec extension.
func IsHitspecFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".http" || ext == ".hitspec"
}

// Adder is the subset of *fsnotify.Watcher used by AddTree.
type Adder interface {
	Add(name string) error
}

// AddTree adds root and every directory below it to w, skipping SkipDir
// directories. For each hitspec file found it calls onFile (when non-nil),
// so callers can synthesize "created" events for files that already existed
// when a directory appeared (mkdir -p, cp -R, git checkout, unzip).
// It returns the first error from w.Add, if any; walking continues past it.
func AddTree(w Adder, root string, onFile func(path string)) error {
	var firstErr error
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && SkipDir(d.Name()) {
				return filepath.SkipDir
			}
			if addErr := w.Add(path); addErr != nil && firstErr == nil {
				firstErr = addErr
			}
			return nil
		}
		if onFile != nil && IsHitspecFile(path) && !strings.HasPrefix(d.Name(), ".#") {
			onFile(path)
		}
		return nil
	})
	return firstErr
}

// Operation maps the last fsnotify op seen for a path, and whether a Create
// was seen during the same debounce window, to the created/changed/deleted
// label carried by file_changed events. inotify reports os.WriteFile as
// Create followed by Write, which must still read as "created".
func Operation(last fsnotify.Op, created bool) string {
	switch {
	case last.Has(fsnotify.Remove), last.Has(fsnotify.Rename):
		return "deleted"
	case created:
		return "created"
	default:
		return "changed"
	}
}
