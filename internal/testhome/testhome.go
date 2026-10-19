// Package testhome keeps tests away from the developer's home directory.
package testhome

import (
	"fmt"
	"os"
	"testing"
)

// Run points HOME (and USERPROFILE, for Windows) at a fresh temporary
// directory, runs the package's tests, and removes the directory. Code under
// test falls back to ~/.hitspec for run history and the studio cookie jar, so
// packages that exercise it call Run from TestMain; otherwise `go test ./...`
// writes thousands of history rows and test cookies into the real ~/.hitspec.
func Run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "hitspec-test-home-")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "testhome:", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()
	for _, key := range homeVars {
		if err := os.Setenv(key, dir); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "testhome:", err)
			return 1
		}
	}
	return m.Run()
}

// homeVars are the variables os.UserHomeDir reads: HOME on Unix, USERPROFILE on
// Windows. A test that sets only HOME leaves the Windows runner's real profile
// in play.
var homeVars = []string{"HOME", "USERPROFILE"}

// Set points the home directory at dir for the duration of the test, on every
// platform. Use it instead of t.Setenv("HOME", dir).
func Set(t testing.TB, dir string) {
	t.Helper()
	for _, key := range homeVars {
		t.Setenv(key, dir)
	}
}
