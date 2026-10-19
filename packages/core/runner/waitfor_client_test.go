package runner

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/testtls"
	hitspechttp "github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

func writeWaitForFile(t *testing.T, waitURL, requestURL string) string {
	t.Helper()
	content := "### guarded\n# @waitFor " + waitURL + " 200 600 50\n\nGET " + requestURL + "\n\n>>>\nexpect status 200\n<<<\n"
	file := filepath.Join(t.TempDir(), "w.http")
	require.NoError(t, os.WriteFile(file, []byte(content), 0o644))
	return file
}

// @waitFor must poll through the run's HTTP client: with a private CA only
// the configured client can reach the https health endpoint.
func TestWaitForUsesTheConfiguredCA(t *testing.T) {
	m := testtls.New(t)
	secure := m.Server(t, nil, false)
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer plain.Close()
	file := writeWaitForFile(t, secure.URL+"/health", plain.URL)

	res, err := NewRunner(&Config{ValidateSSL: true}).RunFile(file)
	require.NoError(t, err)
	require.False(t, res.Results[0].Passed, "an untrusted CA must fail the wait")
	assert.Contains(t, res.Results[0].Error.Error(), "not ready")

	res, err = NewRunner(&Config{ValidateSSL: true, CACert: m.CAPath}).RunFile(file)
	require.NoError(t, err)
	assert.True(t, res.Results[0].Passed, "%v", res.Results[0].Error)
}

func TestWaitForHonorsInsecure(t *testing.T) {
	m := testtls.New(t)
	secure := m.Server(t, nil, false)
	file := writeWaitForFile(t, secure.URL+"/health", secure.URL)

	res, err := NewRunner(&Config{ValidateSSL: false}).RunFile(file)
	require.NoError(t, err)
	assert.True(t, res.Results[0].Passed, "%v", res.Results[0].Error)
}

func TestWaitForGoesThroughTheConfiguredProxy(t *testing.T) {
	var proxied atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host == "waitfor.invalid" {
			proxied.Add(1)
		}
	}))
	defer proxy.Close()
	file := writeWaitForFile(t, "http://waitfor.invalid/health", "http://waitfor.invalid/api")

	res, err := NewRunner(&Config{ValidateSSL: true, Proxy: proxy.URL}).RunFile(file)
	require.NoError(t, err)
	assert.True(t, res.Results[0].Passed, "%v", res.Results[0].Error)
	assert.GreaterOrEqual(t, int(proxied.Load()), 2, "the wait poll and the request both use the proxy")
}

// A @waitFor target that refuses connections is a network error (exit 4), not
// an ordinary failure: the wrapped dial error must stay reachable.
func TestWaitForRefusedConnectionIsANetworkError(t *testing.T) {
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer plain.Close()
	file := writeWaitForFile(t, "http://127.0.0.1:1/health", plain.URL)

	res, err := NewRunner(&Config{}).RunFile(file)
	require.NoError(t, err)
	rr := res.Results[0]
	require.False(t, rr.Passed)
	require.Error(t, rr.Error)
	assert.Contains(t, rr.Error.Error(), "not ready")
	assert.True(t, hitspechttp.IsNetworkError(rr.Error), "refused dial inside @waitFor: %v", rr.Error)

	// A service that answers with the wrong status is up, so not a network error.
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer up.Close()
	res, err = NewRunner(&Config{}).RunFile(writeWaitForFile(t, up.URL+"/health", plain.URL))
	require.NoError(t, err)
	require.Error(t, res.Results[0].Error)
	assert.False(t, hitspechttp.IsNetworkError(res.Results[0].Error))
}
