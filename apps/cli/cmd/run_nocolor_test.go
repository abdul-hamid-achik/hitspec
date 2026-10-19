package cmd

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoColorEnabled pins the precedence for `run`'s color setting:
// --no-color flag > HITSPEC_NO_COLOR > hitspec.yaml noColor > default.
func TestNoColorEnabled(t *testing.T) {
	cases := []struct {
		name string
		env  string
		args []string
		yaml bool
		want bool
	}{
		{name: "default", want: false},
		{name: "yaml true", yaml: true, want: true},
		{name: "flag beats yaml false", args: []string{"--no-color"}, want: true},
		{name: "flag false beats yaml true", args: []string{"--no-color=false"}, yaml: true, want: false},
		{name: "env true", env: "true", want: true},
		{name: "env false beats yaml true", env: "false", yaml: true, want: false},
		{name: "flag false beats env true", env: "true", args: []string{"--no-color=false"}, want: false},
		{name: "env true with yaml false", env: "1", yaml: false, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HITSPEC_NO_COLOR", tc.env)
			// Re-register after the env is set (the flag default reads it) and
			// again on cleanup so later tests see a clean default.
			t.Cleanup(func() { registerRunFlags(pflag.NewFlagSet("reset", pflag.ContinueOnError)) })
			fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
			registerRunFlags(fs)
			require.NoError(t, fs.Parse(tc.args))
			assert.Equal(t, tc.want, noColorEnabled(fs, tc.yaml))
		})
	}
}
