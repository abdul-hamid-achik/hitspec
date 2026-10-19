package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A --watch pass that reloads hitspec.yaml picks up `noColor` too, with the
// same precedence as the first pass (flag > HITSPEC_NO_COLOR > yaml).
func TestReloadRunSettingsReloadsNoColor(t *testing.T) {
	t.Setenv("HITSPEC_NO_COLOR", "")
	t.Cleanup(func() { registerRunFlags(pflag.NewFlagSet("reset", pflag.ContinueOnError)) })

	path := filepath.Join(t.TempDir(), "hitspec.yaml")

	reload := func(args ...string) bool {
		t.Helper()
		fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
		registerRunFlags(fs)
		require.NoError(t, fs.Parse(append([]string{"--config", path}, args...)))
		_, _, noColor, err := reloadRunSettings(fs, nil, nil)
		require.NoError(t, err)
		return noColor
	}

	require.NoError(t, os.WriteFile(path, []byte("noColor: false\n"), 0o644))
	assert.False(t, reload())

	require.NoError(t, os.WriteFile(path, []byte("noColor: true\n"), 0o644))
	assert.True(t, reload(), "yaml noColor turned on mid-watch")
	assert.False(t, reload("--no-color=false"), "an explicit flag still wins")
}
