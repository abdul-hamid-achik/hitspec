package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/testtls"
)

// runHTTPFile writes content as req.http and runs it with extra flags,
// returning the command's error (nil, or an ExitCoder).
func runHTTPFile(t *testing.T, dir, content string, extra ...string) error {
	t.Helper()
	resetRootCommandForTest(t)
	file := filepath.Join(dir, "req.http")
	require.NoError(t, os.WriteFile(file, []byte(content), 0o644))
	rootCmd.SetArgs(append([]string{"run", file, "--quiet", "--no-history"}, extra...))
	return rootCmd.Execute()
}

func exitCodeOf(err error) int {
	var ec ExitCoder
	if errors.As(err, &ec) {
		return ec.ExitCode()
	}
	if err != nil {
		return -1
	}
	return 0
}

func oneRequest(url string) string {
	return "### r\nGET " + url + "\n\n>>>\nexpect status 200\n<<<\n"
}

func TestRunHTTP_CACert(t *testing.T) {
	m := testtls.New(t)
	srv := m.Server(t, nil, false)
	dir := t.TempDir()

	// Untrusted CA: the request fails with a TLS (network) error, exit 4.
	err := runHTTPFile(t, dir, oneRequest(srv.URL))
	assert.Equal(t, ExitNetworkError, exitCodeOf(err), "an unknown CA must fail the request: %v", err)

	require.NoError(t, runHTTPFile(t, dir, oneRequest(srv.URL), "--cacert", m.CAPath))

	t.Setenv("HITSPEC_CACERT", m.CAPath)
	require.NoError(t, runHTTPFile(t, dir, oneRequest(srv.URL)), "HITSPEC_CACERT")
}

func TestRunHTTP_CACertFromYAMLIsRelativeToTheFile(t *testing.T) {
	m := testtls.New(t)
	srv := m.Server(t, nil, false)

	cfgDir := t.TempDir()
	ca, err := os.ReadFile(m.CAPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "ca.pem"), ca, 0o600))
	cfgFile := filepath.Join(cfgDir, "hitspec.yaml")
	require.NoError(t, os.WriteFile(cfgFile, []byte("cacert: ca.pem\n"), 0o600))

	// Run from a different directory: the path must not resolve against the cwd.
	require.NoError(t, runHTTPFile(t, t.TempDir(), oneRequest(srv.URL), "--config", cfgFile))
}

func TestRunHTTP_MutualTLS(t *testing.T) {
	m := testtls.New(t)
	srv := m.Server(t, nil, true)
	dir := t.TempDir()

	err := runHTTPFile(t, dir, oneRequest(srv.URL), "--cacert", m.CAPath)
	assert.Equal(t, ExitNetworkError, exitCodeOf(err), "the server demands a client certificate: %v", err)

	require.NoError(t, runHTTPFile(t, dir, oneRequest(srv.URL), "--cacert", m.CAPath, "--cert", m.ClientCertPath, "--key", m.ClientKeyPath))

	// A certificate without its key is a configuration error.
	err = runHTTPFile(t, dir, oneRequest(srv.URL), "--cacert", m.CAPath, "--cert", m.ClientCertPath)
	assert.Equal(t, ExitConfigError, exitCodeOf(err), "%v", err)
}

func TestRunHTTP_BadTLSFilesExitWithConfigError(t *testing.T) {
	m := testtls.New(t)
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.pem")

	err := runHTTPFile(t, dir, oneRequest("http://127.0.0.1:1"), "--cacert", missing)
	require.Error(t, err)
	assert.Equal(t, ExitConfigError, exitCodeOf(err))
	assert.Contains(t, err.Error(), missing)

	notPEM := filepath.Join(dir, "junk.pem")
	require.NoError(t, os.WriteFile(notPEM, []byte("not a certificate"), 0o600))
	err = runHTTPFile(t, dir, oneRequest("http://127.0.0.1:1"), "--cacert", notPEM)
	assert.Equal(t, ExitConfigError, exitCodeOf(err))
	assert.Contains(t, fmt.Sprint(err), notPEM)

	err = runHTTPFile(t, dir, oneRequest("http://127.0.0.1:1"), "--cert", m.ClientCertPath, "--key", missing)
	assert.Equal(t, ExitConfigError, exitCodeOf(err))
	assert.Contains(t, fmt.Sprint(err), missing)
}

func TestRunHTTP_HTTP1Flag(t *testing.T) {
	m := testtls.New(t)
	var mu sync.Mutex
	var protos []int
	srv := m.Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		protos = append(protos, r.ProtoMajor)
		mu.Unlock()
	}), false)
	dir := t.TempDir()

	require.NoError(t, runHTTPFile(t, dir, oneRequest(srv.URL), "--cacert", m.CAPath))
	require.NoError(t, runHTTPFile(t, dir, oneRequest(srv.URL), "--cacert", m.CAPath, "--http1.1"))
	t.Setenv("HITSPEC_HTTP1", "true")
	require.NoError(t, runHTTPFile(t, dir, oneRequest(srv.URL), "--cacert", m.CAPath))

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int{2, 1, 1}, protos, "HTTP/2 by default, HTTP/1.1 with --http1.1 or HITSPEC_HTTP1")
}

func TestRunHTTP_CookiesAreOptIn(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc123", Path: "/"})
		case "/me":
			c, err := r.Cookie("session")
			mu.Lock()
			if err != nil {
				seen = append(seen, "none")
			} else {
				seen = append(seen, c.Value)
			}
			mu.Unlock()
		}
	}))
	defer srv.Close()

	file := "### login\nGET " + srv.URL + "/login\n\n>>>\nexpect status 200\n<<<\n\n" +
		"### me\nGET " + srv.URL + "/me\n\n>>>\nexpect status 200\n<<<\n"
	dir := t.TempDir()

	require.NoError(t, runHTTPFile(t, dir, file))
	require.NoError(t, runHTTPFile(t, dir, file, "--cookies"))
	require.NoError(t, runHTTPFile(t, dir, file, "--cookies=false"))

	cfg := filepath.Join(dir, "hitspec.yaml")
	require.NoError(t, os.WriteFile(cfg, []byte("cookies: true\n"), 0o600))
	require.NoError(t, runHTTPFile(t, dir, file, "--config", cfg))
	require.NoError(t, runHTTPFile(t, dir, file, "--config", cfg, "--cookies=false"))

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"none", "abc123", "none", "abc123", "none"}, seen,
		"default off; --cookies on; --cookies=false off; yaml on; flag beats yaml")
}

func TestRunHTTP_MaxResponseBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 5000)))
	}))
	defer srv.Close()
	dir := t.TempDir()

	require.NoError(t, runHTTPFile(t, dir, oneRequest(srv.URL)))
	err := runHTTPFile(t, dir, oneRequest(srv.URL), "--max-response-bytes", "1000")
	assert.Equal(t, 1, exitCodeOf(err), "a body over the bound fails the request: %v", err)
	require.NoError(t, runHTTPFile(t, dir, oneRequest(srv.URL), "--max-response-bytes", "-1"))

	cfg := filepath.Join(dir, "hitspec.yaml")
	require.NoError(t, os.WriteFile(cfg, []byte("maxResponseBytes: 1000\n"), 0o600))
	err = runHTTPFile(t, dir, oneRequest(srv.URL), "--config", cfg)
	assert.Equal(t, 1, exitCodeOf(err), "yaml maxResponseBytes: %v", err)
	require.NoError(t, runHTTPFile(t, dir, oneRequest(srv.URL), "--config", cfg, "--max-response-bytes", "-1"), "flag beats yaml")
}

func TestRunHTTP_RedirectForwardHeaders(t *testing.T) {
	var mu sync.Mutex
	var got []string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.Header.Get("X-Api-Key"))
		mu.Unlock()
	}))
	defer target.Close()
	// Different port = different origin.
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer origin.Close()

	content := "### r\nGET " + origin.URL + "\nX-Api-Key: secret\n\n>>>\nexpect status 200\n<<<\n"
	dir := t.TempDir()
	require.NoError(t, runHTTPFile(t, dir, content))
	require.NoError(t, runHTTPFile(t, dir, content, "--redirect-forward-headers"))

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"", "secret"}, got)
}

// `contract verify` builds its client from hitspec.yaml in the working
// directory, so the HTTP keys apply there too.
func TestContractRunConfigHonorsHTTPClientKeys(t *testing.T) {
	m := testtls.New(t)
	// Canonical: discovery resolves symlinks (/var -> /private/var on macOS).
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	m.WriteYAMLDir(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte(testtls.YAML), 0o600))

	cfg, err := contractRunConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "ca.pem"), cfg.CACert)
	assert.Equal(t, filepath.Join(dir, "client.pem"), cfg.ClientCert)
	assert.True(t, cfg.Cookies)
	assert.True(t, cfg.HTTP1)
}
