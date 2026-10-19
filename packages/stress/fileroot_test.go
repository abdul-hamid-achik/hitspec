package stress

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fileRootFixture(t *testing.T) (srvURL string, bodies func() []string, dir, secret string) {
	t.Helper()
	var mu sync.Mutex
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, string(b))
		mu.Unlock()
	}))
	t.Cleanup(srv.Close)
	secret = filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("TOP-SECRET"), 0o644))
	dir = t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ok.json"), []byte(`{"fine":true}`), 0o644))
	return srv.URL, func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), got...) }, dir, secret
}

// With a file root, a `< file` body outside it is never sent: every attempt is
// an error and the summary says why. Inside the root the file is sent.
func TestStressFileRootConfinesBodyFiles(t *testing.T) {
	url, bodies, dir, secret := fileRootFixture(t)
	cfg := func() *Config { return &Config{Mode: RateMode, Duration: 400 * time.Millisecond, Rate: 20, MaxVUs: 5} }

	escape := filepath.Join(dir, "escape.http")
	require.NoError(t, os.WriteFile(escape, []byte("### out\nPOST "+url+"/x\n\n< "+secret+"\n"), 0o644))
	inside := filepath.Join(dir, "inside.http")
	require.NoError(t, os.WriteFile(inside, []byte("### in\nPOST "+url+"/x\n\n< ./ok.json\n"), 0o644))

	runWith := func(file string, root string) *Result {
		runner := NewRunner(cfg(), WithFileRoot(root), WithReporter(NewReporter(WithWriter(io.Discard), WithNoProgress(true))))
		require.NoError(t, runner.LoadFile(file))
		res, err := runner.Run(t.Context())
		require.NoError(t, err)
		return res
	}

	res := runWith(escape, dir)
	assert.Greater(t, res.Summary.ErrorCount, int64(0))
	assert.Zero(t, res.Summary.SuccessCount)
	require.NotEmpty(t, res.Summary.Warnings)
	assert.Contains(t, strings.Join(res.Summary.Warnings, "\n"), "outside the workspace")
	assert.Empty(t, bodies(), "nothing may reach the server")

	res = runWith(inside, dir)
	assert.Zero(t, res.Summary.ErrorCount)
	assert.NotEmpty(t, bodies())

	// No root (the CLI): the file is read as before.
	runWith(escape, "")
	assert.Contains(t, strings.Join(bodies(), "|"), "TOP-SECRET")
}

// A setup request that reads outside the root fails the run instead of being sent.
func TestStressFileRootRefusesSetupRequest(t *testing.T) {
	url, bodies, dir, secret := fileRootFixture(t)
	file := filepath.Join(dir, "setup.http")
	require.NoError(t, os.WriteFile(file, []byte("### login\n# @stress.setup\nPOST "+url+"/login\n\n< "+secret+"\n\n### work\nGET "+url+"/w\n"), 0o644))
	runner := NewRunner(&Config{Mode: RateMode, Duration: 300 * time.Millisecond, Rate: 10, MaxVUs: 2}, WithFileRoot(dir),
		WithReporter(NewReporter(WithWriter(io.Discard), WithNoProgress(true))))
	require.NoError(t, runner.LoadFile(file))
	_, err := runner.Run(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the workspace")
	assert.Empty(t, bodies())
}
