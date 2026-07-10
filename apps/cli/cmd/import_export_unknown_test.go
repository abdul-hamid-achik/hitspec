package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportUnknownTypeErrors guards the regression where `hitspec import <badtype>`
// (an unknown importer / typo'd format) printed help and exited 0, silently
// passing in scripts. An unknown import format must return an error so the CLI
// exits non-zero.
func TestImportUnknownTypeErrors(t *testing.T) {
	rootCmd.SetArgs([]string{"import", "curlX", "some-source"})
	err := rootCmd.Execute()
	require.Error(t, err, "unknown import format must error, not print help and exit 0")
	assert.Contains(t, err.Error(), "unknown import format")
	assert.Contains(t, err.Error(), "curlX")
}

// TestExportUnknownTypeErrors guards the same regression for
// `hitspec export <badtype>`: an unknown export format must error out.
func TestExportUnknownTypeErrors(t *testing.T) {
	rootCmd.SetArgs([]string{"export", "curlX", "some-file.http"})
	err := rootCmd.Execute()
	require.Error(t, err, "unknown export format must error, not print help and exit 0")
	assert.Contains(t, err.Error(), "unknown export format")
	assert.Contains(t, err.Error(), "curlX")
}
