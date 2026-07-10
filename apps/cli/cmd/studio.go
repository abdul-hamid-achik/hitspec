package cmd

import (
	"context"
	"io"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
	"github.com/abdul-hamid-achik/hitspec/packages/tui"
	"github.com/spf13/cobra"
)

var (
	studioWatchFlag      bool
	studioReadOnlyFlag   bool
	studioEnvFlag        string
	studioConfigFlag     string
	studioVerboseFlag    bool
	studioAllowShellFlag bool
	studioAllowDBFlag    bool
	studioLogFormatFlag  string
	studioLogLevelFlag   string
	studioThemeFlag      string
)

var studioCmd = &cobra.Command{
	Use:     "studio [file|directory]",
	Aliases: []string{"ui"},
	Short:   "Open the interactive app for working with hitspec files",
	Long: `Open hitspec studio — a keyboard-first interactive workspace for your .http
files, right in the terminal.

It provides:
  - Inline .http editing with syntax-highlighted source and save
  - Request execution with a tabbed response viewer (body, headers, assertions, timing, captures)
  - Run history with drill-down, stress, mock, contract, record, import, cookies, and settings
  - Copy/export as curl, HTTPie, Python, fetch, or Go; workspace search; environment switching
  - File watching and live execution progress

Examples:
  hitspec studio
  hitspec studio ./tests/
  hitspec studio ./tests/users.http
  hitspec studio --read-only`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir := "."
		if len(args) > 0 {
			workDir = args[0]
		}
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		return launchStudio(ctx, normalizeServeWorkDir(workDir), studioLaunchFlags{
			watch:      studioWatchFlag,
			readOnly:   studioReadOnlyFlag,
			env:        studioEnvFlag,
			config:     studioConfigFlag,
			verbose:    studioVerboseFlag,
			allowShell: studioAllowShellFlag,
			allowDB:    studioAllowDBFlag,
			logFormat:  studioLogFormatFlag,
			logLevel:   studioLogLevelFlag,
			theme:      studioThemeFlag,
		})
	},
}

func init() {
	f := studioCmd.Flags()
	f.BoolVarP(&studioWatchFlag, "watch", "w", true, "Watch for file changes")
	f.BoolVar(&studioReadOnlyFlag, "read-only", false, "Disallow file mutations")
	f.StringVarP(&studioEnvFlag, "env", "e", "", "Default environment (default: hitspec.yaml defaultEnvironment, else dev)")
	f.StringVar(&studioConfigFlag, "config", "", "Path to hitspec.yaml")
	f.BoolVarP(&studioVerboseFlag, "verbose", "v", false, "Verbose logging")
	f.BoolVar(&studioAllowShellFlag, "allow-shell", false, "Allow shell command execution")
	f.BoolVar(&studioAllowDBFlag, "allow-db", false, "Allow database assertions")
	f.StringVar(&studioLogFormatFlag, "log-format", "text", "Log format: text or json")
	f.StringVar(&studioLogLevelFlag, "log-level", "info", "Log level: debug, info, warn, error")
	f.StringVar(&studioThemeFlag, "theme", "", "Color theme: Nord, Catppuccin Mocha, Dracula, Tokyo Night, Gruvbox Dark")
}

type studioLaunchFlags struct {
	watch, readOnly, verbose, allowShell, allowDB bool
	env, config, logFormat, logLevel, theme       string
}

// launchStudio boots the in-process Manager and runs the interactive app. It is
// shared by `hitspec studio` and the deprecated `hitspec serve` UI path.
func launchStudio(ctx context.Context, workDir string, f studioLaunchFlags) error {
	mgr := clientmgr.New(
		clientmgr.WithWorkDir(workDir),
		clientmgr.WithWatch(f.watch),
		clientmgr.WithReadOnly(f.readOnly),
		clientmgr.WithLogLevel(f.logLevel),
		clientmgr.WithLogWriter(io.Discard),
		clientmgr.WithVerbose(f.verbose),
		clientmgr.WithAllowShell(f.allowShell),
		clientmgr.WithAllowDB(f.allowDB),
		clientmgr.WithLogFormat(f.logFormat),
		clientmgr.WithLogLevel(f.logLevel),
	)
	mgr.Version = version
	mgr.BuildTime = buildTime
	mgr.Start(ctx)
	defer mgr.Close()
	return tui.Run(ctx, mgr, tui.Options{Mouse: true, Theme: f.theme})
}
