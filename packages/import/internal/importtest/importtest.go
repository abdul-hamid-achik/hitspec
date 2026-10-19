// Package importtest holds golden-file helpers shared by the importer tests.
package importtest

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
)

var update = flag.Bool("update", false, "rewrite golden files")

// Golden compares got with testdata/<name> in the calling package, rewriting it
// when the test runs with -update.
func Golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create it)", path, err)
	}
	if string(want) != got {
		t.Errorf("output differs from %s (run with -update to accept)\n--- want\n%s\n--- got\n%s", path, want, got)
	}
}

// Validate fails the test unless content parses as a valid .http file, the same
// check `hitspec validate` performs. It returns the parsed file.
func Validate(t *testing.T, content string) *parser.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.http")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := parser.ParseFile(path)
	if err != nil {
		t.Fatalf("importer output does not validate: %v\n%s", err, content)
	}
	return f
}
