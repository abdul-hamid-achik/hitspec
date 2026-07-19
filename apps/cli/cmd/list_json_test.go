package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestList_JSON guards the `hitspec list --json` flag (a polish/test-gap item):
// the test list is emitted as parseable JSON with per-file requests instead of
// the human-readable text format.
func TestList_JSON(t *testing.T) {
	resetRootCommandForTest(t)
	dir := t.TempDir()
	httpFile := filepath.Join(dir, "api.http")
	require.NoError(t, os.WriteFile(httpFile, []byte("### login\n# @name login\n# @tags smoke\nGET http://x/login\n\n### B\nGET http://x/b\n"), 0o644))

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"list", httpFile, "--json"})
	require.NoError(t, rootCmd.Execute())

	var got []listFile
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got), "output: %s", buf.String())
	require.Len(t, got, 1)
	assert.Len(t, got[0].Requests, 2)
	assert.Equal(t, "login", got[0].Requests[0].Name)
	assert.Equal(t, []string{"smoke"}, got[0].Requests[0].Tags)
	assert.Equal(t, "B", got[0].Requests[1].Name)
}

// TestList_Text ensures the default (no --json) output is the human-readable form.
func TestList_Text(t *testing.T) {
	resetRootCommandForTest(t)
	dir := t.TempDir()
	httpFile := filepath.Join(dir, "api.http")
	require.NoError(t, os.WriteFile(httpFile, []byte("### login\n# @name login\nGET http://x/login\n"), 0o644))

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"list", httpFile})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "login")
	assert.Contains(t, buf.String(), "1 request(s)")
}
