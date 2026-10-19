package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/watchutil"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
	"github.com/spf13/pflag"
)

// watchSupportFiles lists the non-test files whose change must reload the
// settings before the next watch pass: the config file (--config, or the
// discovered hitspec.yaml) and the env file. envFile is the effective value
// (flag, HITSPEC_ENV_FILE or hitspec.yaml). Missing files are listed too, so a
// file created later is noticed.
func watchSupportFiles(configFlagValue, envFile string) []string {
	var out []string
	if configFlagValue != "" {
		out = append(out, configFlagValue)
	} else if p := config.FindConfigPathUp("."); p != "" {
		out = append(out, p)
	} else {
		out = append(out, config.ConfigFilenames...) // not there yet; watch for it to appear
	}
	if envFile != "" {
		out = append(out, envFile)
	}
	return out
}

// isSupportFile reports whether path is one of the support files.
func isSupportFile(path string, support []string) bool {
	for _, s := range support {
		if watchutil.SamePath(path, s) {
			return true
		}
	}
	return false
}

// addWatchDirs watches everything a pass depends on: the directory of every
// test file, every directory below a directory argument (never .git,
// node_modules or vendor), and the directories of the support files. The
// directories, not the files, are watched, so a file replaced by an atomic
// save stays watched.
func addWatchDirs(w watchutil.Adder, files, args, support []string, stderr io.Writer) {
	added := make(map[string]bool)
	add := func(dir string) {
		abs, err := filepath.Abs(dir)
		if err != nil {
			abs = dir
		}
		if added[abs] {
			return
		}
		added[abs] = true
		if err := w.Add(dir); err != nil {
			fmt.Fprintf(stderr, "Error: failed to watch %s: %v\n", dir, err)
		}
	}
	for _, file := range files {
		add(filepath.Dir(file))
	}
	for _, arg := range args {
		if info, err := os.Stat(arg); err == nil && info.IsDir() {
			_ = watchutil.AddTree(adderFunc(func(name string) error {
				add(name)
				return nil
			}), arg, nil)
		}
	}
	for _, s := range support {
		add(filepath.Dir(s))
	}
}

type adderFunc func(name string) error

func (f adderFunc) Add(name string) error { return f(name) }

// reloadRunSettings rebuilds the runner configuration for a watch pass from
// hitspec.yaml, the env file and the flags as they are now, so an edit to
// either takes effect on the next pass (a runner reads its env file once, at
// construction). The history store is carried over. noColor is the reloaded
// color setting, which lives outside the runner configuration. On error the
// caller keeps the previous runner.
func reloadRunSettings(flags *pflag.FlagSet, previous *runner.Config, tagsFilter []string) (*runner.Runner, *runner.Config, bool, error) {
	r, cfg, noColor, _, err := reloadSettings(flags, previous, tagsFilter)
	return r, cfg, noColor, err
}

// reloadSettings is reloadRunSettings that also resolves the report
// destinations (hitspec.yaml `reporters` and `outputDir`) as they are now, so
// an edit to them takes effect on the next pass. The plan is nil for a dry
// run, which ignores them. An invalid value is an error like any other config
// error, so the previous settings stay.
func reloadSettings(flags *pflag.FlagSet, previous *runner.Config, tagsFilter []string) (*runner.Runner, *runner.Config, bool, *reportPlan, error) {
	fileConfig, err := config.LoadConfig(configFlag)
	if err != nil {
		return nil, nil, false, nil, err
	}
	var plan *reportPlan
	if !dryRunFlag {
		if plan, err = resolveReportPlan(flags, os.LookupEnv, fileConfig, reportConfigLabel()); err != nil {
			return nil, nil, false, nil, err
		}
		if err = plan.ensureDir(); err != nil {
			return nil, nil, false, nil, fmt.Errorf("cannot create outputDir: %w", err)
		}
	}
	environment, _ := resolveEnvironment(envFlag, fileConfig)
	cfg, _, err := buildRunnerConfig(flags, fileConfig, environment, tagsFilter)
	if err != nil {
		return nil, nil, false, nil, err
	}
	// Same rule as the first pass: a missing env file is a config error, so
	// the previous settings stay rather than running without its variables.
	if cfg.EnvFile != "" {
		if _, err := os.Stat(cfg.EnvFile); err != nil {
			return nil, nil, false, nil, fmt.Errorf("env file: %w", err)
		}
	}
	if previous != nil {
		cfg.HistoryStore = previous.HistoryStore
	}
	return runner.NewRunner(cfg), cfg, noColorEnabled(flags, fileConfig.GetNoColor()), plan, nil
}
