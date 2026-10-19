package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tree creates files (relative slash paths) under a fresh temp dir and makes it
// the working directory for the test.
func tree(t *testing.T, files ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		p := filepath.Join(dir, filepath.FromSlash(f))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte("GET http://x\n"), 0o644))
	}
	t.Chdir(dir)
	return dir
}

func slashed(files []string) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = filepath.ToSlash(f)
	}
	return out
}

func TestCollectFiles_DirectoryWalkSkipsIgnoredDirs(t *testing.T) {
	tree(t,
		"tests/a.http", "tests/sub/b.hitspec", "tests/readme.md",
		"tests/node_modules/pkg/c.http", "tests/vendor/d.http", "tests/.git/e.http",
		"tests/deep/node_modules/f.http",
	)
	files, err := collectFiles([]string{"tests"})
	require.NoError(t, err)
	assert.Equal(t, []string{"tests/a.http", "tests/sub/b.hitspec"}, slashed(files))
}

func TestCollectFiles_ExplicitPathInsideIgnoredDirStillWorks(t *testing.T) {
	tree(t, "node_modules/pkg/c.http", "node_modules/pkg/nested/node_modules/x.http", "vendor/d.http")

	files, err := collectFiles([]string{"node_modules/pkg/c.http"})
	require.NoError(t, err)
	assert.Equal(t, []string{"node_modules/pkg/c.http"}, slashed(files))

	// Naming the ignored directory itself walks it, but not the ignored
	// directories nested in it.
	files, err = collectFiles([]string{"node_modules/pkg"})
	require.NoError(t, err)
	assert.Equal(t, []string{"node_modules/pkg/c.http"}, slashed(files))

	files, err = collectFiles([]string{"vendor"})
	require.NoError(t, err)
	assert.Equal(t, []string{"vendor/d.http"}, slashed(files))
}

func TestCollectFiles_DeduplicatesInFirstSeenOrder(t *testing.T) {
	tree(t, "t/a.http", "t/b.http", "c.http")
	files, err := collectFiles([]string{"t/b.http", "t", "./t/a.http", "c.http", "t/../c.http"})
	require.NoError(t, err)
	assert.Equal(t, []string{"t/b.http", "t/a.http", "c.http"}, slashed(files))
}

func TestCollectFiles_DoubleStarGlobs(t *testing.T) {
	tree(t,
		"tests/a.http", "tests/users/b.http", "tests/users/deep/c.http", "tests/users/deep/notes.txt",
		"tests/node_modules/x/d.http", "other/e.http", "top.http",
	)

	files, err := collectFiles([]string{"tests/**/*.http"})
	require.NoError(t, err)
	assert.Equal(t, []string{"tests/a.http", "tests/users/b.http", "tests/users/deep/c.http"}, slashed(files),
		"** matches zero or more directories and skips node_modules")

	files, err = collectFiles([]string{"**/*.http"})
	require.NoError(t, err)
	assert.Equal(t, []string{"other/e.http", "tests/a.http", "tests/users/b.http", "tests/users/deep/c.http", "top.http"}, slashed(files))

	files, err = collectFiles([]string{"tests/*/b.http"})
	require.NoError(t, err)
	assert.Equal(t, []string{"tests/users/b.http"}, slashed(files), "single * stays within one segment")

	files, err = collectFiles([]string{"tests/users/**"})
	require.NoError(t, err)
	assert.Equal(t, []string{"tests/users/b.http", "tests/users/deep/c.http"}, slashed(files), "only hitspec files are taken")
}

func TestCollectFiles_GlobMatchingNothingIsAnError(t *testing.T) {
	tree(t, "a.http")
	_, err := collectFiles([]string{"nope/**/*.http"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no .http or .hitspec files match")

	_, err = collectFiles([]string{"missing.http"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	_, err = collectFiles([]string{"[bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pattern")
}

func TestCollectFiles_ExistingFileWithBracketsIsNotAPattern(t *testing.T) {
	tree(t, "[id].http")
	files, err := collectFiles([]string{"[id].http"})
	require.NoError(t, err)
	assert.Equal(t, []string{"[id].http"}, files)
}

func TestIsHitspecFile_IgnoresEditorLockFiles(t *testing.T) {
	assert.True(t, isHitspecFile("a/b.http"))
	assert.True(t, isHitspecFile("b.hitspec"))
	assert.False(t, isHitspecFile("a/.#b.http"))
	assert.False(t, isHitspecFile("b.http~"))
	assert.False(t, isHitspecFile("b.http.swp"))
}

func TestMatchSegments(t *testing.T) {
	cases := []struct {
		pat, path string
		want      bool
	}{
		{"**/*.http", "a.http", true},
		{"**/*.http", "x/y/a.http", true},
		{"a/**/b.http", "a/b.http", true},
		{"a/**/b.http", "a/x/y/b.http", true},
		{"a/**/b.http", "b.http", false},
		{"a/**", "a", true},
		{"*.http", "x/a.http", false},
		{"a/**/**/b", "a/b", true},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, matchSegments(strings.Split(c.pat, "/"), strings.Split(c.path, "/")), "%s vs %s", c.pat, c.path)
	}
}

func TestReadStdinSource(t *testing.T) {
	src, err := readStdinSource(strings.NewReader("GET http://x\n"))
	require.NoError(t, err)
	assert.Equal(t, "GET http://x\n", string(src))

	_, err = readStdinSource(strings.NewReader("  \n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no input on stdin")
}
