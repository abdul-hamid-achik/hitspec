package contract

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With a file root, a `< file` body outside it is refused before the request
// is sent; inside the root it works; without a root (the CLI) nothing changes.
func TestVerifierFileRootConfinesBodyFiles(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
	}))
	defer srv.Close()

	secret := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("TOP-SECRET"), 0o644))
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ok.json"), []byte(`{"fine":true}`), 0o644))
	inside := writeHTTPFile(t, dir, "inside.http", "### in\nPOST "+srv.URL+"/x\n\n< ./ok.json\n")
	escape := writeHTTPFile(t, dir, "escape.http", "### out\nPOST "+srv.URL+"/x\n\n< "+secret+"\n")

	v := NewVerifier(WithFileRoot(dir))
	res, err := v.VerifyFile(inside)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Passed)

	res, err = v.VerifyFile(escape)
	require.NoError(t, err)
	require.Len(t, res.Results, 1)
	assert.Equal(t, 1, res.Failed)
	require.Error(t, res.Results[0].Error)
	assert.Contains(t, res.Results[0].Error.Error(), "outside the workspace")

	mu.Lock()
	for _, b := range bodies {
		assert.NotContains(t, b, "TOP-SECRET")
	}
	assert.Len(t, bodies, 1, "only the inside request may reach the provider")
	mu.Unlock()

	// No root: unchanged behavior.
	res, err = NewVerifier().VerifyFile(escape)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Passed)
	mu.Lock()
	defer mu.Unlock()
	assert.True(t, strings.Contains(strings.Join(bodies, "|"), "TOP-SECRET"), "without a root the file is read as before")
}
