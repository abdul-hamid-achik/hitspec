package env

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEnv(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// unsetAtEnd removes names after the test so the process env stays clean.
func unsetAtEnd(t *testing.T, names ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, n := range names {
			_ = os.Unsetenv(n)
		}
	})
}

// Reloading an edited file refreshes the values it exported earlier (a
// long-lived process used to keep the first value forever) and unsets the ones
// the file no longer defines.
func TestLoadAndExportDotEnvRefreshesItsOwnExports(t *testing.T) {
	unsetAtEnd(t, "HSPEC_DOTENV_A", "HSPEC_DOTENV_B")
	path := filepath.Join(t.TempDir(), ".env")

	writeEnv(t, path, "HSPEC_DOTENV_A=one\nHSPEC_DOTENV_B=keep\n")
	if _, err := LoadAndExportDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("HSPEC_DOTENV_A"); got != "one" {
		t.Fatalf("A = %q, want one", got)
	}

	writeEnv(t, path, "HSPEC_DOTENV_A=two\n")
	if _, err := LoadAndExportDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("HSPEC_DOTENV_A"); got != "two" {
		t.Errorf("A after the edit = %q, want two", got)
	}
	if got := os.Getenv("HSPEC_DOTENV_B"); got != "" {
		t.Errorf("B after being removed from the file = %q, want unset", got)
	}
}

// A variable the user set (before the first load, or changed since) is never
// overwritten: the shell wins over the file.
func TestLoadAndExportDotEnvNeverOverwritesUserValues(t *testing.T) {
	unsetAtEnd(t, "HSPEC_DOTENV_C", "HSPEC_DOTENV_D")
	path := filepath.Join(t.TempDir(), ".env")
	t.Setenv("HSPEC_DOTENV_C", "from-shell")

	writeEnv(t, path, "HSPEC_DOTENV_C=file\nHSPEC_DOTENV_D=file1\n")
	if _, err := LoadAndExportDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("HSPEC_DOTENV_C"); got != "from-shell" {
		t.Fatalf("C = %q, want the shell's value", got)
	}

	// The process changes D after we exported it: from then on it is theirs.
	t.Setenv("HSPEC_DOTENV_D", "changed-by-user")
	writeEnv(t, path, "HSPEC_DOTENV_C=file2\nHSPEC_DOTENV_D=file2\n")
	if _, err := LoadAndExportDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("HSPEC_DOTENV_C"); got != "from-shell" {
		t.Errorf("C = %q after reload, want the shell's value", got)
	}
	if got := os.Getenv("HSPEC_DOTENV_D"); got != "changed-by-user" {
		t.Errorf("D = %q after reload, want the user's change kept", got)
	}

	// A variable removed from the file but changed by the user is not unset.
	writeEnv(t, path, "")
	if _, err := LoadAndExportDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("HSPEC_DOTENV_D"); got != "changed-by-user" {
		t.Errorf("D = %q after removal, want the user's value kept", got)
	}
}

// The resolver sees the refreshed value through {{$env(NAME)}}.
func TestResolverEnvFollowsEditedDotEnv(t *testing.T) {
	unsetAtEnd(t, "HSPEC_DOTENV_E")
	path := filepath.Join(t.TempDir(), ".env")

	writeEnv(t, path, "HSPEC_DOTENV_E=v1\n")
	r := NewResolver()
	if err := r.LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := r.Resolve("{{$env(HSPEC_DOTENV_E)}}"); got != "v1" {
		t.Fatalf("first load = %q", got)
	}

	writeEnv(t, path, "HSPEC_DOTENV_E=v2\n")
	r2 := NewResolver()
	if err := r2.LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := r2.Resolve("{{$env(HSPEC_DOTENV_E)}}"); got != "v2" {
		t.Errorf("after the edit = %q, want v2", got)
	}
}
