package runconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/testtls"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	hitspechttp "github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

func TestBuildHTTPClientSettingsPrecedence(t *testing.T) {
	file := &config.Config{
		CACert:                 "yaml-ca.pem",
		Cert:                   "yaml-cert.pem",
		Key:                    "yaml-key.pem",
		HTTP1:                  ptr(true),
		Cookies:                ptr(true),
		MaxResponseBytes:       111,
		RedirectForwardHeaders: ptr(true),
	}

	cfg, _ := Build(file, Overrides{})
	if cfg.CACert != "yaml-ca.pem" || cfg.ClientCert != "yaml-cert.pem" || cfg.ClientKey != "yaml-key.pem" ||
		!cfg.HTTP1 || !cfg.Cookies || cfg.MaxResponseBytes != 111 || !cfg.RedirectForwardHeaders {
		t.Fatalf("yaml values lost: %+v", cfg)
	}

	cfg, _ = Build(file, Overrides{
		CACert: "flag-ca.pem", Cert: "flag-cert.pem", Key: "flag-key.pem",
		HTTP1: ptr(false), Cookies: ptr(false), MaxResponseBytes: ptr(int64(-1)), RedirectForwardHeaders: ptr(false),
	})
	if cfg.CACert != "flag-ca.pem" || cfg.ClientCert != "flag-cert.pem" || cfg.ClientKey != "flag-key.pem" ||
		cfg.HTTP1 || cfg.Cookies || cfg.MaxResponseBytes != -1 || cfg.RedirectForwardHeaders {
		t.Fatalf("overrides did not beat yaml: %+v", cfg)
	}

	// Off by default: no cookie jar, HTTP/2, default body bound.
	cfg, _ = Build(nil, Overrides{})
	if cfg.Cookies || cfg.HTTP1 || cfg.RedirectForwardHeaders || cfg.MaxResponseBytes != 0 || cfg.CACert != "" {
		t.Fatalf("defaults changed: %+v", cfg)
	}
}

func TestFromEnvHTTPClientSettings(t *testing.T) {
	t.Setenv(EnvCACert, "env-ca.pem")
	t.Setenv(EnvCert, "env-cert.pem")
	t.Setenv(EnvKey, "env-key.pem")
	t.Setenv(EnvHTTP1, "1")
	t.Setenv(EnvCookies, "true")
	t.Setenv(EnvRedirectForwardHeaders, "false")
	t.Setenv(EnvMaxResponseBytes, "-1")

	o, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if o.CACert != "env-ca.pem" || o.Cert != "env-cert.pem" || o.Key != "env-key.pem" ||
		o.HTTP1 == nil || !*o.HTTP1 || o.Cookies == nil || !*o.Cookies ||
		o.RedirectForwardHeaders == nil || *o.RedirectForwardHeaders ||
		o.MaxResponseBytes == nil || *o.MaxResponseBytes != -1 {
		t.Fatalf("env not read: %+v", o)
	}

	// A flag-level override beats the env layer.
	merged := Overrides{Cookies: ptr(false), CACert: "flag.pem"}.Over(o)
	if *merged.Cookies || merged.CACert != "flag.pem" || merged.Cert != "env-cert.pem" {
		t.Fatalf("Over: %+v", merged)
	}

	t.Setenv(EnvMaxResponseBytes, "lots")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected an error for a non-numeric " + EnvMaxResponseBytes)
	}
}

// A hitspec.yaml with TLS keys reaches the HTTP client on every surface that
// goes through Build: the cert paths resolve against the yaml file's directory
// and the resulting client really does mutual TLS.
func TestBuildYAMLTLSReachesTheClient(t *testing.T) {
	m := testtls.New(t)
	srv := m.Server(t, nil, true)

	dir := t.TempDir()
	for name, src := range map[string]string{"ca.pem": m.CAPath, "c.pem": m.ClientCertPath, "c-key.pem": m.ClientKeyPath} {
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	yaml := "cacert: ca.pem\ncert: c.pem\nkey: c-key.pem\n"
	if err := os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := config.FindAndLoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, opts := Build(file, Overrides{})
	client, err := hitspechttp.NewClientE(opts...)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Get(srv.URL, nil)
	if err != nil {
		t.Fatalf("mTLS request with yaml-configured material: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}
