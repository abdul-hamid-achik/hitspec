package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeServeWorkDirAcceptsFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "api.http")
	if err := os.WriteFile(file, []byte("GET https://example.com\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got := normalizeServeWorkDir(file)
	if got != dir {
		t.Fatalf("normalizeServeWorkDir(%q) = %q, want %q", file, got, dir)
	}
}
