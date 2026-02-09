package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/abdul-hamid-achik/hitspec/packages/serve"
	"github.com/spf13/cobra"
)

var (
	servePortFlag       int
	serveHostFlag       string
	serveOpenFlag       bool
	serveWatchFlag      bool
	serveCORSFlag       bool
	serveAPIOnlyFlag    bool
	serveReadOnlyFlag   bool
	serveEnvFlag        string
	serveConfigFlag     string
	serveVerboseFlag    bool
	serveAllowShellFlag bool
	serveAllowDBFlag    bool
)

var serveCmd = &cobra.Command{
	Use:   "serve [file|directory]",
	Short: "Start the API Client Manager web interface",
	Long: `Start a browser-based API Client Manager for working with hitspec files.

The serve command launches an HTTP server that provides:
  - A web-based UI for editing and running API tests
  - A REST API for programmatic access
  - Real-time file watching with WebSocket updates
  - Stress testing dashboard
  - Mock server management

Examples:
  hitspec serve
  hitspec serve ./tests/
  hitspec serve --port 8080
  hitspec serve --api-only
  hitspec serve --read-only --cors`,
	Args: cobra.MaximumNArgs(1),
	RunE: serveCommand,
}

func init() {
	serveCmd.Flags().IntVarP(&servePortFlag, "port", "p", 4000, "Port to run the server on")
	serveCmd.Flags().StringVar(&serveHostFlag, "host", "localhost", "Bind address")
	serveCmd.Flags().BoolVar(&serveOpenFlag, "open", true, "Auto-open browser")
	serveCmd.Flags().BoolVarP(&serveWatchFlag, "watch", "w", true, "Watch for file changes")
	serveCmd.Flags().BoolVar(&serveCORSFlag, "cors", false, "Enable CORS headers")
	serveCmd.Flags().BoolVar(&serveAPIOnlyFlag, "api-only", false, "REST API only, no SPA")
	serveCmd.Flags().BoolVar(&serveReadOnlyFlag, "read-only", false, "Disallow file mutations")
	serveCmd.Flags().StringVarP(&serveEnvFlag, "env", "e", "dev", "Default environment")
	serveCmd.Flags().StringVar(&serveConfigFlag, "config", "", "Path to hitspec.yaml")
	serveCmd.Flags().BoolVarP(&serveVerboseFlag, "verbose", "v", false, "Verbose logging")
	serveCmd.Flags().BoolVar(&serveAllowShellFlag, "allow-shell", false, "Allow shell command execution")
	serveCmd.Flags().BoolVar(&serveAllowDBFlag, "allow-db", false, "Allow database assertions")
}

func serveCommand(cmd *cobra.Command, args []string) error {
	workDir := "."
	if len(args) > 0 {
		workDir = args[0]
	}

	s := serve.NewServer(
		serve.WithPort(servePortFlag),
		serve.WithHost(serveHostFlag),
		serve.WithWorkDir(workDir),
		serve.WithOpen(serveOpenFlag),
		serve.WithWatch(serveWatchFlag),
		serve.WithCORS(serveCORSFlag),
		serve.WithAPIOnly(serveAPIOnlyFlag),
		serve.WithReadOnly(serveReadOnlyFlag),
		serve.WithEnv(serveEnvFlag),
		serve.WithConfigPath(serveConfigFlag),
		serve.WithVerbose(serveVerboseFlag),
		serve.WithAllowShell(serveAllowShellFlag),
		serve.WithAllowDB(serveAllowDBFlag),
	)

	s.Version = version
	s.BuildTime = buildTime

	// Auto-open browser
	if serveOpenFlag && !serveAPIOnlyFlag {
		go func() {
			url := fmt.Sprintf("http://%s:%d", serveHostFlag, servePortFlag)
			openBrowser(url)
		}()
	}

	return s.Start(context.Background())
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return
	}
	_ = cmd.Start()
}
