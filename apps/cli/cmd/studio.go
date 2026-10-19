package cmd

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/clientmgr"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/tui"
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
	studioMouseFlag      bool
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
		workDir = normalizeServeWorkDir(workDir)
		if err := preflightWorkspaceConfig(cmd, workDir, studioConfigFlag); err != nil {
			return err
		}
		return launchStudio(ctx, workDir, studioLaunchFlags{
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
			mouse:      studioMouseFlag,
		})
	},
}

func init() {
	f := studioCmd.Flags()
	f.BoolVarP(&studioWatchFlag, "watch", "w", true, "Watch for file changes")
	f.BoolVar(&studioReadOnlyFlag, "read-only", false, "Refuse workspace and hitspec.yaml writes (running requests, stress, mock and contract stay available)")
	f.StringVarP(&studioEnvFlag, "env", "e", "", "Default environment (default: hitspec.yaml defaultEnvironment, else dev)")
	f.StringVar(&studioConfigFlag, "config", "", "Path to hitspec.yaml")
	f.BoolVarP(&studioVerboseFlag, "verbose", "v", false, "Verbose logging")
	f.BoolVar(&studioAllowShellFlag, "allow-shell", false, "Allow shell command execution")
	f.BoolVar(&studioAllowDBFlag, "allow-db", false, "Allow database assertions")
	f.StringVar(&studioLogFormatFlag, "log-format", "text", "Log format: text or json")
	f.StringVar(&studioLogLevelFlag, "log-level", "info", "Log level: debug, info, warn, error")
	f.StringVar(&studioThemeFlag, "theme", "", "Color theme: Nord, Catppuccin Mocha, Dracula, Tokyo Night, Gruvbox Dark")
	f.BoolVar(&studioMouseFlag, "mouse", false, "Capture the mouse (wheel scrolling); off by default so the terminal can select and copy text")
}

type studioLaunchFlags struct {
	watch, readOnly, verbose, allowShell, allowDB, mouse bool
	env, config, logFormat, logLevel, theme              string
}

// newStudioManager builds the in-process Manager from the studio flags. It is a
// separate function so tests can check that every flag reaches the Manager.
func newStudioManager(workDir string, f studioLaunchFlags) *clientmgr.Manager {
	return clientmgr.New(
		clientmgr.WithWorkDir(workDir),
		clientmgr.WithWatch(f.watch),
		clientmgr.WithReadOnly(f.readOnly),
		clientmgr.WithEnv(f.env),
		clientmgr.WithConfigPath(f.config),
		clientmgr.WithLogWriter(io.Discard),
		clientmgr.WithVerbose(f.verbose),
		clientmgr.WithAllowShell(f.allowShell),
		clientmgr.WithAllowDB(f.allowDB),
		clientmgr.WithLogFormat(f.logFormat),
		clientmgr.WithLogLevel(f.logLevel),
	)
}

// launchStudio boots the in-process Manager and runs the interactive app. It is
// shared by `hitspec studio` and the deprecated `hitspec serve` UI path.
func launchStudio(ctx context.Context, workDir string, f studioLaunchFlags) error {
	// The engines the studio drives (mock, record proxy, contract verifier,
	// runner hooks/shell/wait-for, parser and builtin warnings) write to the
	// std logger, os.Stdout or os.Stderr, which would be painted over the
	// alt-screen UI. Point them at /dev/null for the lifetime of the app and
	// hand the real terminal to the UI. Registered first so it is undone last.
	tuiOut, restoreStdio := silenceStdio()
	defer restoreStdio()

	mgr := newStudioManager(workDir, f)
	mgr.Version = version
	mgr.BuildTime = buildTime
	mgr.Start(ctx)
	defer mgr.Close()
	return tui.Run(ctx, mgr, tui.Options{Mouse: f.mouse, Theme: f.theme, Output: tuiOut})
}

// silenceStdio redirects os.Stdout, os.Stderr and the std logger to the null
// device and returns the real stdout (for the UI to draw on) plus a function
// that undoes the redirection. A pipe would block once full, so the null device
// is used. If it cannot be opened nothing is redirected.
func silenceStdio() (*os.File, func()) {
	realOut, realErr := os.Stdout, os.Stderr
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return realOut, func() {}
	}
	os.Stdout, os.Stderr = devNull, devNull
	log.SetOutput(io.Discard) // the std logger captured os.Stderr at init
	return realOut, func() {
		os.Stdout, os.Stderr = realOut, realErr
		log.SetOutput(realErr)
		_ = devNull.Close()
	}
}
