package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// diff writes its report to the command's writers, not to os.Stdout, so it can
// be captured and redirected like every other command.
func TestDiff_WritesToCommandOutput(t *testing.T) {
	resetRootCommandForTest(t)
	dir := t.TempDir()
	base := filepath.Join(dir, "base.json")
	cur := filepath.Join(dir, "cur.json")
	require.NoError(t, os.WriteFile(base, []byte(`{"summary":{"total":1,"passed":1},"tests":[{"name":"slower","file":"a.http","passed":true,"duration":100}]}`), 0o644))
	require.NoError(t, os.WriteFile(cur, []byte(`{"summary":{"total":1,"passed":1},"tests":[{"name":"slower","file":"a.http","passed":true,"duration":200}]}`), 0o644))

	// Anything that still wrote to os.Stdout would land in this pipe.
	realStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = realStdout }()

	for _, format := range []string{"console", "json", "html"} {
		var out, errOut bytes.Buffer
		rootCmd.SetOut(&out)
		rootCmd.SetErr(&errOut)
		rootCmd.SetArgs([]string{"diff", base, cur, "--output", format})
		require.NoError(t, rootCmd.Execute(), format)
		assert.Contains(t, out.String(), "slower", format)
		assert.Empty(t, errOut.String(), format)
		require.NoError(t, restoreRootCommandDefaults())
	}

	// The HTML report renders the threshold block too (it used to fail to
	// execute: the template compared a float with an int).
	var html bytes.Buffer
	rootCmd.SetOut(&html)
	rootCmd.SetErr(&bytes.Buffer{})
	rootCmd.SetArgs([]string{"diff", base, cur, "--output", "html", "--threshold", "10%"})
	_ = rootCmd.Execute()
	assert.Contains(t, html.String(), "</html>")
	require.NoError(t, restoreRootCommandDefaults())

	// A failing threshold still prints the verdict to the command's output.
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&bytes.Buffer{})
	rootCmd.SetArgs([]string{"diff", base, cur, "--threshold", "10%"})
	require.Error(t, rootCmd.Execute())
	assert.Contains(t, out.String(), "Threshold check failed")
	assert.NotContains(t, out.String(), "\x1b[", "no color escapes when not writing to the terminal")

	os.Stdout = realStdout
	require.NoError(t, w.Close())
	leaked := new(bytes.Buffer)
	_, _ = leaked.ReadFrom(r)
	assert.Empty(t, leaked.String(), "diff must not write to os.Stdout")
}
