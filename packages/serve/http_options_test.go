package serve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/testtls"
	hitspechttp "github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

// The HTTP client keys of hitspec.yaml reach `serve --api-only`'s runner
// configuration and client options.
func TestRunSettingsHonorHTTPClientKeys(t *testing.T) {
	m := testtls.New(t)
	srv := m.Server(t, nil, true)
	dir := t.TempDir()
	m.WriteYAMLDir(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte(testtls.YAML), 0o600); err != nil {
		t.Fatal(err)
	}

	s := NewServer(WithWorkDir(dir), WithWatch(false), WithHistoryDBPath(filepath.Join(dir, "h.db")))
	t.Cleanup(func() {
		if s.historyStore != nil {
			_ = s.historyStore.Close()
		}
	})
	cfg, opts := s.runSettings("")

	if cfg.CACert != filepath.Join(dir, "ca.pem") || cfg.ClientCert != filepath.Join(dir, "client.pem") || cfg.ClientKey != filepath.Join(dir, "client-key.pem") {
		t.Errorf("paths must resolve against the yaml's directory: %q %q %q", cfg.CACert, cfg.ClientCert, cfg.ClientKey)
	}
	if !cfg.Cookies || !cfg.HTTP1 || !cfg.RedirectForwardHeaders || cfg.MaxResponseBytes != 2048 {
		t.Errorf("settings lost: %+v", cfg)
	}
	client, err := hitspechttp.NewClientE(opts...)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Get(srv.URL, nil)
	if err != nil {
		t.Fatalf("mutual TLS request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
}
