package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	"github.com/spf13/cobra"
)

// loadConfigForCommand loads hitspec.yaml for a command: the explicit path
// (--config / HITSPEC_CONFIG) when given, otherwise the nearest hitspec.yaml
// from the working directory upward (config.Discover). Problems with the file
// itself (unreadable, bad syntax) are printed to stderr and returned as an
// exit-3 error; unknown keys are only warned about. With verbose, the config
// file in use is reported on stderr.
func loadConfigForCommand(cmd *cobra.Command, explicit string, verbose bool) (*config.Config, error) {
	var (
		loaded *config.Loaded
		err    error
	)
	if explicit != "" {
		loaded, err = config.Load(explicit)
	} else {
		loaded, err = config.Discover(".")
	}
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return nil, newExitError(ExitConfigError, err.Error())
	}
	printConfigWarnings(cmd.ErrOrStderr(), loaded.Warnings)
	if verbose {
		if loaded.Path == "" {
			fmt.Fprintln(cmd.ErrOrStderr(), "config: none found (using defaults)")
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "config: %s\n", config.DisplayPath(loaded.Path))
		}
	}
	return loaded.Config, nil
}

func printConfigWarnings(w io.Writer, warnings []string) {
	for _, warning := range warnings {
		fmt.Fprintf(w, "warning: %s\n", warning)
	}
}

// checkEnvFile fails with an exit-3 error when --env-file (or
// HITSPEC_ENV_FILE) names a file that cannot be opened: running on without it
// would leave every variable it defines unresolved.
func checkEnvFile(cmd *cobra.Command, path string) error {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		var info os.FileInfo
		if info, err = f.Stat(); err == nil && info.IsDir() {
			err = fmt.Errorf("%s is a directory", path)
		}
	}
	if err != nil {
		msg := fmt.Sprintf("env file: %v", err)
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", msg)
		return newExitError(ExitConfigError, msg)
	}
	return nil
}

// preflightWorkspaceConfig checks the hitspec.yaml that studio / serve will
// load (the explicit --config, else the nearest one from workDir upward)
// before the app starts, while stderr is still visible: the app itself
// silences it, and a broken config would otherwise be dropped without a word.
func preflightWorkspaceConfig(cmd *cobra.Command, workDir, explicit string) error {
	var (
		loaded *config.Loaded
		err    error
	)
	if explicit != "" {
		loaded, err = config.Load(explicit)
	} else {
		loaded, err = config.Discover(workDir)
	}
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return newExitError(ExitConfigError, err.Error())
	}
	printConfigWarnings(cmd.ErrOrStderr(), loaded.Warnings)
	return nil
}
