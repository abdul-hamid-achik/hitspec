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

// TestValidate_JSON guards the `hitspec validate --json` flag: results are
// emitted as parseable JSON with ok/errors per file instead of the text format.
func TestValidate_JSON(t *testing.T) {
	validateJSONFlag = false // reset (cobra flag values leak across Execute)
	dir := t.TempDir()
	okFile := filepath.Join(dir, "ok.http")
	require.NoError(t, os.WriteFile(okFile, []byte("### ok\nGET http://x\n"), 0o644))
	badFile := filepath.Join(dir, "bad.http")
	require.NoError(t, os.WriteFile(badFile, []byte("### bad\nGET\n"), 0o644))

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"validate", okFile, badFile, "--json"})
	// validate returns an ExitCoder on failure; Execute surfaces it but the JSON
	// is already written to the buffer.
	_ = rootCmd.Execute()

	var got []validateResult
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got), "output: %s", buf.String())
	require.Len(t, got, 2)
	assert.True(t, got[0].OK)
	assert.False(t, got[1].OK)
	assert.NotEmpty(t, got[1].Errors)
}
