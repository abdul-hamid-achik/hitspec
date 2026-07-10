package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportPostman_OutputDirectory guards the "import postman -o dir/ help
// lies" finding: the documented `-o tests/` (a directory) used to fail because
// the command wrote a single file at that path; it now writes
// <dir>/<collection-name>.http inside the directory.
func TestImportPostman_OutputDirectory(t *testing.T) {
	dir := t.TempDir()
	collectionPath := filepath.Join(dir, "col.json")
	// Minimal Postman Collection v2.1 with one GET item.
	require.NoError(t, os.WriteFile(collectionPath, []byte(`{
  "info": {"name": "DemoCollection", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
  "item": [{"name": "list users", "request": {"method": "GET", "url": {"raw": "https://api.example.com/users", "host": ["api","example","com"], "path": ["users"]}}}]
}`), 0o644))

	outDir := filepath.Join(dir, "out") + "/"
	importOutputFlag = ""
	rootCmd.SetArgs([]string{"import", "postman", collectionPath, "-o", outDir})
	require.NoError(t, rootCmd.Execute())

	// The collection-named .http file must exist inside the output directory.
	written := filepath.Join(outDir, "col.http")
	info, err := os.Stat(written)
	require.NoError(t, err, "expected %s to be created", written)
	assert.False(t, info.IsDir())
	b, err := os.ReadFile(written)
	require.NoError(t, err)
	assert.Contains(t, string(b), "GET")
}
