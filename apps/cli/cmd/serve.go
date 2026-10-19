package cmd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/serve"
	"github.com/spf13/cobra"
)

var (
	servePortFlag       int
	serveHostFlag       string
	serveWatchFlag      bool
	serveCORSFlag       bool
	serveAPIOnlyFlag    bool
	serveReadOnlyFlag   bool
	serveEnvFlag        string
	serveConfigFlag     string
	serveVerboseFlag    bool
	serveAllowShellFlag bool
	serveAllowDBFlag    bool
	serveLogFormatFlag  string
	serveLogLevelFlag   string
	serveAPITokenFlag   string
)

var serveCmd = &cobra.Command{
	Use:   "serve [file|directory]",
	Short: "Start the REST/WebSocket API server",
	Long: `Start the hitspec REST/WebSocket API server for integrations and editors.

It exposes JSON endpoints (files, execute/run, environments, stress, mock,
contract, record, import/export, history) plus a WebSocket for realtime events.

  hitspec serve --api-only        Start the API server

For the interactive terminal app, use:

  hitspec studio                  Open the interactive app

(Running 'serve' without --api-only opens the interactive app for backward
compatibility, but 'hitspec studio' is the dedicated command.)

Examples:
  hitspec serve --api-only --port 8080 --cors
  HITSPEC_API_TOKEN=secret hitspec serve --api-only
  hitspec serve --api-only ./tests/ --read-only`,
	Args: cobra.MaximumNArgs(1),
	RunE: serveCommand,
}

func init() {
	serveCmd.Flags().IntVarP(&servePortFlag, "port", "p", 4000, "API server port")
	serveCmd.Flags().StringVar(&serveHostFlag, "host", "localhost", "API server bind address")
	serveCmd.Flags().BoolVarP(&serveWatchFlag, "watch", "w", true, "Watch for file changes")
	serveCmd.Flags().BoolVar(&serveCORSFlag, "cors", false, "Enable CORS headers (API server)")
	serveCmd.Flags().BoolVar(&serveAPIOnlyFlag, "api-only", false, "Start the REST/WebSocket API server")
	serveCmd.Flags().BoolVar(&serveReadOnlyFlag, "read-only", false, "Disallow file mutations")
	serveCmd.Flags().StringVarP(&serveEnvFlag, "env", "e", "", "Default environment (default: hitspec.yaml defaultEnvironment, else dev)")
	serveCmd.Flags().StringVar(&serveConfigFlag, "config", "", "Path to hitspec.yaml")
	serveCmd.Flags().BoolVarP(&serveVerboseFlag, "verbose", "v", false, "Verbose logging")
	serveCmd.Flags().BoolVar(&serveAllowShellFlag, "allow-shell", false, "Allow shell command execution")
	serveCmd.Flags().BoolVar(&serveAllowDBFlag, "allow-db", false, "Allow database assertions")
	serveCmd.Flags().StringVar(&serveLogFormatFlag, "log-format", "text", "Log format: text or json")
	serveCmd.Flags().StringVar(&serveLogLevelFlag, "log-level", "info", "Log level: debug, info, warn, error")
	serveCmd.Flags().StringVar(&serveAPITokenFlag, "api-token", "", "Require a bearer token for all REST/WebSocket requests (env: HITSPEC_API_TOKEN, preferred: argv is visible in ps). Requests must send Authorization: Bearer <token> or ?token=<token>. A random token is generated and printed when the bind address is not loopback or --allow-shell/--allow-db is set")
}

func serveCommand(cmd *cobra.Command, args []string) error {
	workDir := "."
	if len(args) > 0 {
		workDir = args[0]
	}
	workDir = normalizeServeWorkDir(workDir)

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Backward compatibility: `serve` without --api-only opened the interactive
	// app. Keep that working but steer users to the dedicated `studio` command.
	if !serveAPIOnlyFlag {
		cmd.PrintErrln(`tip: use "hitspec studio" to open the interactive app ("hitspec serve --api-only" starts the API server).`)
		return launchStudio(ctx, workDir, studioLaunchFlags{
			watch:      serveWatchFlag,
			readOnly:   serveReadOnlyFlag,
			env:        serveEnvFlag,
			config:     serveConfigFlag,
			verbose:    serveVerboseFlag,
			allowShell: serveAllowShellFlag,
			allowDB:    serveAllowDBFlag,
			logFormat:  serveLogFormatFlag,
			logLevel:   serveLogLevelFlag,
		})
	}

	s := serve.NewServer(
		serve.WithPort(servePortFlag),
		serve.WithHost(serveHostFlag),
		serve.WithWorkDir(workDir),
		serve.WithOpen(false),
		serve.WithWatch(serveWatchFlag),
		serve.WithCORS(serveCORSFlag),
		serve.WithAPIOnly(true),
		serve.WithReadOnly(serveReadOnlyFlag),
		serve.WithEnv(serveEnvFlag),
		serve.WithConfigPath(serveConfigFlag),
		serve.WithVerbose(serveVerboseFlag),
		serve.WithAllowShell(serveAllowShellFlag),
		serve.WithAllowDB(serveAllowDBFlag),
		serve.WithAPIToken(resolveAPIToken(serveAPITokenFlag)),
		serve.WithLogFormat(serveLogFormatFlag),
		serve.WithLogLevel(serveLogLevelFlag),
	)
	s.Version = version
	s.BuildTime = buildTime
	return s.Start(ctx)
}

// resolveAPIToken returns the --api-token flag, falling back to the
// HITSPEC_API_TOKEN environment variable. The env var is read here rather than
// used as the flag default so that `--help` never prints the secret.
func resolveAPIToken(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("HITSPEC_API_TOKEN")
}

func normalizeServeWorkDir(path string) string {
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		return filepath.Dir(path)
	}
	return path
}
