package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidate_RejectsBadFiles guards the regression where `hitspec validate`
// reported "OK" for files that are clearly invalid: a request with an empty
// URL, an unclosed >>> assertion block, and an empty file (no requests). Each
// must be reported as a failure with a non-zero exit instead of silently
// passing. The command is driven through rootCmd.Execute() so the returned
// error (an ExitCoder carrying ExitParseError) is observable in-process.
func TestValidate_RejectsBadFiles(t *testing.T) {
	cases := []struct {
		name    string // test case label
		content string // file body
		wantSub string // substring expected in the FAIL diagnostic on stderr
	}{
		{
			name: "missing URL",
			// GET with no URL line: the parser accepts it with an empty URL.
			content: "### no url\nGET\n\n>>>\nexpect status 200\n<<<\n",
			wantSub: "empty URL",
		},
		{
			name: "unclosed assertion block",
			// >>> block with no closing <<<: the hardened parser returns an
			// "unclosed" error that validate must propagate, not swallow.
			content: "### unclosed\nGET http://x/a\n\n>>>\nexpect status 200\n",
			wantSub: "unclosed",
		},
		{
			name: "empty file",
			// No requests at all: previously printed OK and exited 0.
			content: "",
			wantSub: "empty file",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "bad.http")
			require.NoError(t, os.WriteFile(path, []byte(tc.content), 0o644))

			// Capture cobra output so we can assert the FAIL diagnostic is
			// actually reported, not just a silent non-zero return. Route both
			// the stdout and stderr writers to the same buffer: in this cobra
			// version OutOrStderr() resolves through the same writer as
			// SetOut(), so a shared buffer captures everything regardless.
			var buf bytes.Buffer
			prevOut, prevErr := rootCmd.OutOrStdout(), rootCmd.OutOrStderr()
			rootCmd.SetOut(&buf)
			rootCmd.SetErr(&buf)
			t.Cleanup(func() {
				rootCmd.SetOut(prevOut)
				rootCmd.SetErr(prevErr)
			})

			rootCmd.SetArgs([]string{"validate", path})
			err := rootCmd.Execute()

			require.Error(t, err, "validate must report an error for %s", tc.name)
			coder, ok := err.(ExitCoder)
			require.True(t, ok, "validate error must be an ExitCoder, got %T", err)
			assert.Equal(t, ExitParseError, coder.ExitCode(), "parse/structure errors exit with ExitParseError")

			output := buf.String()
			assert.Contains(t, output, "FAIL", "validate must print a FAIL line")
			assert.Contains(t, output, tc.wantSub, "validate must surface the %s diagnostic", tc.name)
			assert.NotContains(t, output, "OK    ", "validate must not report a bad file as OK")
		})
	}
}

// TestValidate_AcceptsValidFile ensures the rejection logic does not produce
// false positives: a well-formed file still validates cleanly (OK, exit 0).
func TestValidate_AcceptsValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.http")
	content := "### ok\nGET http://x/a\n\n>>>\nexpect status 200\n<<<\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	var buf bytes.Buffer
	prevOut, prevErr := rootCmd.OutOrStdout(), rootCmd.OutOrStderr()
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	t.Cleanup(func() {
		rootCmd.SetOut(prevOut)
		rootCmd.SetErr(prevErr)
	})

	rootCmd.SetArgs([]string{"validate", path})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "OK    ", "a valid file must be reported as OK")
	assert.NotContains(t, buf.String(), "FAIL", "no FAIL diagnostics for a valid file")
}
