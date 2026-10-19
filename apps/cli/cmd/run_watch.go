package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/watchutil"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/env"
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
	} else if p := config.FindConfigPath("."); p != "" {
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

// dotenvTracker remembers which process environment variables a runner's env
// file created. A runner exports the file's variables to the process
// environment unless they are already set, so an edited value would never
// replace the exported one on the next pass (`{{$env(NAME)}}` reads the process
// environment). release removes exactly the variables the file introduced, so
// a variable that was set before hitspec started is left alone.
type dotenvTracker struct{ keys []string }

// capture records the variables of path that are not set yet, which are the
// ones the runner about to be built will export. Call it before NewRunner.
func (d *dotenvTracker) capture(path string) {
	d.keys = nil
	if path == "" {
		return
	}
	vars, err := env.LoadDotEnv(path)
	if err != nil {
		return
	}
	for k := range vars {
		if os.Getenv(k) == "" {
			d.keys = append(d.keys, k)
		}
	}
}

// release unsets the variables recorded by capture.
func (d *dotenvTracker) release() {
	for _, k := range d.keys {
		_ = os.Unsetenv(k)
	}
	d.keys = nil
}

// reloadRunSettings rebuilds the runner configuration for a watch pass from
// hitspec.yaml, the env file and the flags as they are now, so an edit to
// either takes effect on the next pass (a runner reads its env file once, at
// construction). The history store is carried over. On error the caller keeps
// the previous runner.
func reloadRunSettings(flags *pflag.FlagSet, previous *runner.Config, tagsFilter []string, dotenv *dotenvTracker) (*runner.Runner, *runner.Config, error) {
	fileConfig, err := config.LoadConfig(configFlag)
	if err != nil {
		return nil, nil, err
	}
	environment, _ := resolveEnvironment(envFlag, fileConfig)
	cfg, _, err := buildRunnerConfig(flags, fileConfig, environment, tagsFilter)
	if err != nil {
		return nil, nil, err
	}
	if previous != nil {
		cfg.HistoryStore = previous.HistoryStore
	}
	dotenv.release()
	dotenv.capture(cfg.EnvFile)
	return runner.NewRunner(cfg), cfg, nil
}
