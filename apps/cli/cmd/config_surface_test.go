package cmd

import (
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/cfgsurface"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	"github.com/spf13/pflag"
)

// TestConfigSurfaces records the settings each CLI path (run, run --stress,
// contract verify) ends up using for the shared hitspec.yaml / env / flag
// scenarios. See internal/cfgsurface.
func TestConfigSurfaces(t *testing.T) {
	table := map[string]map[string]cfgsurface.Settings{
		"cli.run":      {},
		"cli.stress":   {},
		"cli.contract": {},
	}
	for _, sc := range cfgsurface.Scenarios() {
		t.Run(sc.Name, func(t *testing.T) {
			// Registered before Setup so it runs after the env is restored:
			// re-registering reads the flag defaults from the clean env.
			t.Cleanup(func() { registerRunFlags(pflag.NewFlagSet("reset", pflag.ContinueOnError)) })
			dir := cfgsurface.Setup(t, sc)

			fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
			registerRunFlags(fs)
			if err := fs.Parse(sc.Flags); err != nil {
				t.Fatal(err)
			}
			fileConfig, err := config.FindAndLoadConfig(dir)
			if err != nil {
				t.Fatal(err)
			}
			environment, _ := resolveEnvironment(envFlag, fileConfig)

			cfg, err := buildRunnerConfig(fileConfig, environment, nil)
			if err != nil {
				t.Fatal(err)
			}
			table["cli.run"][sc.Name] = cfgsurface.FromRunner(cfg)

			st := resolveStressClientSettings(fileConfig)
			bare := cfgsurface.Bare()
			bare.FollowRedirects = st.followRedirects
			bare.Proxy = st.proxy
			bare.ValidateSSL = st.validateSSL
			table["cli.stress"][sc.Name] = bare

			// `contract verify` loads no config and builds a bare client.
			table["cli.contract"][sc.Name] = cfgsurface.Bare()
		})
	}
	cfgsurface.Check(t, filepath.Join("testdata", "config_surfaces.golden.json"), table)
}
