package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsolatedResolverIgnoresProcessEnvironment(t *testing.T) {
	t.Setenv("HITSPEC_ISO_SECRET", "s3cret")
	r := NewResolverWith(WithIsolatedEnvironment())
	for _, in := range []string{
		"{{HITSPEC_ISO_SECRET}}", "{{$HITSPEC_ISO_SECRET}}", "{{$env(HITSPEC_ISO_SECRET)}}",
		"{{ $env( HITSPEC_ISO_SECRET ) }}",
	} {
		if got := r.Resolve(in); got == "s3cret" {
			t.Errorf("Resolve(%q) = %q, want anything but the secret", in, got)
		}
	}
	if got := r.Resolve("{{$env(HITSPEC_ISO_SECRET, fallback)}}"); got != "fallback" {
		t.Errorf("$env default = %q, want fallback", got)
	}
	// Variables nested through a workspace-defined variable stay isolated.
	r.SetVariable("token", "{{$env(HITSPEC_ISO_SECRET)}}")
	if got := r.Resolve("{{token}}"); got == "s3cret" {
		t.Errorf("nested env reference leaked: %q", got)
	}
	if got := r.Clone().Resolve("{{$env(HITSPEC_ISO_SECRET)}}"); got == "s3cret" {
		t.Errorf("clone lost isolation: %q", got)
	}
}

func TestIsolatedResolverAllowlist(t *testing.T) {
	t.Setenv("HITSPEC_ISO_ALLOWED", "ok")
	t.Setenv("HITSPEC_ISO_DENIED", "no")
	r := NewResolverWith(WithIsolatedEnvironment("HITSPEC_ISO_ALLOWED"))
	for _, in := range []string{"{{HITSPEC_ISO_ALLOWED}}", "{{$HITSPEC_ISO_ALLOWED}}", "{{$env(HITSPEC_ISO_ALLOWED)}}"} {
		if got := r.Resolve(in); got != "ok" {
			t.Errorf("Resolve(%q) = %q, want ok", in, got)
		}
	}
	if got := r.Resolve("{{$env(HITSPEC_ISO_DENIED)}}"); got == "no" {
		t.Errorf("non-allowlisted variable leaked: %q", got)
	}
	if got := r.Clone().Resolve("{{$env(HITSPEC_ISO_ALLOWED)}}"); got != "ok" {
		t.Errorf("clone lost allowlist: %q", got)
	}
}

func TestIsolatedResolverDotEnvIsNotExported(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("HITSPEC_ISO_DOTENV=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Unsetenv("HITSPEC_ISO_DOTENV")
	r := NewResolverWith(WithIsolatedEnvironment())
	if err := r.LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if v, ok := os.LookupEnv("HITSPEC_ISO_DOTENV"); ok {
		t.Fatalf("isolated LoadDotEnv exported into the process environment: %q", v)
	}
	for _, in := range []string{"{{HITSPEC_ISO_DOTENV}}", "{{$HITSPEC_ISO_DOTENV}}", "{{$env(HITSPEC_ISO_DOTENV)}}"} {
		if got := r.Resolve(in); got != "from-file" {
			t.Errorf("Resolve(%q) = %q, want the .env value", in, got)
		}
	}
	if got := r.LookupEnv("HITSPEC_ISO_DOTENV"); got != "from-file" {
		t.Errorf("LookupEnv = %q", got)
	}
}

func TestDefaultResolverStillReadsProcessEnvironment(t *testing.T) {
	t.Setenv("HITSPEC_ISO_DEFAULT", "visible")
	r := NewResolver()
	for _, in := range []string{"{{HITSPEC_ISO_DEFAULT}}", "{{$HITSPEC_ISO_DEFAULT}}", "{{$env(HITSPEC_ISO_DEFAULT)}}"} {
		if got := r.Resolve(in); got != "visible" {
			t.Errorf("Resolve(%q) = %q, want visible", in, got)
		}
	}
	// Default behavior: LoadDotEnv still exports.
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("HITSPEC_ISO_EXPORTED=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Unsetenv("HITSPEC_ISO_EXPORTED")
	t.Cleanup(func() { _ = os.Unsetenv("HITSPEC_ISO_EXPORTED") })
	if err := r.LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("HITSPEC_ISO_EXPORTED") != "1" {
		t.Fatal("default LoadDotEnv no longer exports")
	}
}

func TestLoadEnvironmentWithLookup(t *testing.T) {
	t.Setenv("HITSPEC_ISO_CFG", "process")
	configEnvs := map[string]map[string]any{"dev": {"key": "${HITSPEC_ISO_CFG}"}}
	got, err := LoadEnvironmentWithLookup(".", "dev", configEnvs, func(string) string { return "" })
	if err != nil || got.Variables["key"] != "" {
		t.Fatalf("custom lookup = %#v err=%v", got, err)
	}
	got, err = LoadEnvironment(".", "dev", configEnvs)
	if err != nil || got.Variables["key"] != "process" {
		t.Fatalf("LoadEnvironment changed behavior: %#v err=%v", got, err)
	}
}
