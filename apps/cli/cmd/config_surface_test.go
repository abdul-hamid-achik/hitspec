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

			cfg, _, err := buildRunnerConfig(fs, fileConfig, environment, nil)
			if err != nil {
				t.Fatal(err)
			}
			run := cfgsurface.FromRunner(cfg)
			table["cli.run"][sc.Name] = run
			// `run --stress` builds its client from the same options.
			table["cli.stress"][sc.Name] = cfgsurface.ClientOnly(run)

			// `contract verify` has no flags: yaml and HITSPEC_* only. It
			// reads hitspec.yaml from the working directory.
			contractCfg, err := contractRunConfig(dir)
			if err != nil {
				t.Fatal(err)
			}
			table["cli.contract"][sc.Name] = cfgsurface.ClientOnly(cfgsurface.FromRunner(contractCfg))
		})
	}
	cfgsurface.Check(t, filepath.Join("testdata", "config_surfaces.golden.json"), table)
}

// TestRunOverridesExplicitFalse: a bool flag passed as false still beats a
// hitspec.yaml value, and an unset flag leaves the yaml value alone.
func TestRunOverridesExplicitFalse(t *testing.T) {
	t.Cleanup(func() { registerRunFlags(pflag.NewFlagSet("reset", pflag.ContinueOnError)) })
	sc := cfgsurface.Scenarios()[1] // "yaml": parallel, bail true; validateSSL false
	dir := cfgsurface.Setup(t, sc)
	fileConfig, err := config.FindAndLoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	build := func(args ...string) (parallel, bail, validateSSL bool) {
		fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
		registerRunFlags(fs)
		if err := fs.Parse(args); err != nil {
			t.Fatal(err)
		}
		cfg, _, err := buildRunnerConfig(fs, fileConfig, "dev", nil)
		if err != nil {
			t.Fatal(err)
		}
		return cfg.Parallel, cfg.Bail, cfg.ValidateSSL
	}

	if p, b, v := build(); !p || !b || v {
		t.Errorf("no flags: parallel=%v bail=%v validateSSL=%v, want the yaml values true/true/false", p, b, v)
	}
	if p, b, v := build("--parallel=false", "--bail=false", "--insecure=false"); p || b || !v {
		t.Errorf("explicit false: parallel=%v bail=%v validateSSL=%v, want false/false/true", p, b, v)
	}
	if _, _, v := build("--insecure"); v {
		t.Error("--insecure should disable validation")
	}
}

// TestRunInvalidTimeout: both the flag and HITSPEC_TIMEOUT report a bad value.
func TestRunInvalidTimeout(t *testing.T) {
	t.Cleanup(func() { registerRunFlags(pflag.NewFlagSet("reset", pflag.ContinueOnError)) })
	dir := cfgsurface.Setup(t, cfgsurface.Scenarios()[0])
	fileConfig, err := config.FindAndLoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
	registerRunFlags(fs)
	if err := fs.Parse([]string{"--timeout", "soon"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := buildRunnerConfig(fs, fileConfig, "dev", nil); err == nil {
		t.Fatal("expected an error for --timeout soon")
	}

	t.Setenv("HITSPEC_TIMEOUT", "soon")
	fs = pflag.NewFlagSet("run", pflag.ContinueOnError)
	registerRunFlags(fs)
	if _, _, err := buildRunnerConfig(fs, fileConfig, "dev", nil); err == nil {
		t.Fatal("expected an error for HITSPEC_TIMEOUT=soon")
	}
}
