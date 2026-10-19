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

func TestResolveAPIToken(t *testing.T) {
	t.Setenv("HITSPEC_API_TOKEN", "from-env")
	if got := resolveAPIToken(""); got != "from-env" {
		t.Fatalf("env only: got %q, want from-env", got)
	}
	if got := resolveAPIToken("from-flag"); got != "from-flag" {
		t.Fatalf("flag wins over env: got %q, want from-flag", got)
	}
	t.Setenv("HITSPEC_API_TOKEN", "")
	if got := resolveAPIToken(""); got != "" {
		t.Fatalf("neither set: got %q, want empty", got)
	}
	if got := resolveAPIToken("from-flag"); got != "from-flag" {
		t.Fatalf("flag only: got %q, want from-flag", got)
	}
	// --help must not print a secret taken from the environment.
	t.Setenv("HITSPEC_API_TOKEN", "super-secret")
	if def := serveCmd.Flags().Lookup("api-token").DefValue; def != "" {
		t.Fatalf("api-token flag default must be empty, got %q", def)
	}
}
