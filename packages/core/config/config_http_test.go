package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePathIsRelativeToTheConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hitspec.yaml")
	if err := os.WriteFile(path, []byte("cacert: certs/ca.pem\ncookies: true\nmaxResponseBytes: -1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.ResolvePath(cfg.CACert), filepath.Join(dir, "certs", "ca.pem"); got != want {
		t.Errorf("ResolvePath = %q, want %q", got, want)
	}
	if abs := filepath.Join(dir, "abs.pem"); cfg.ResolvePath(abs) != abs {
		t.Error("an absolute path must be left alone")
	}
	if cfg.ResolvePath("") != "" {
		t.Error("an empty path must stay empty")
	}
	if !cfg.GetCookies() || cfg.GetHTTP1() || cfg.GetRedirectForwardHeaders() || cfg.MaxResponseBytes != -1 {
		t.Errorf("parsed HTTP settings: %+v", cfg)
	}
	if (&Config{}).ResolvePath("x.pem") != "x.pem" {
		t.Error("a config that did not come from a file keeps paths as written")
	}
}
