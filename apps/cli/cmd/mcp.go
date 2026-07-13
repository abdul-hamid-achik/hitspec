package cmd

import (
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	hitspecmcp "github.com/abdul-hamid-achik/hitspec/packages/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = newMCPCommand()

func newMCPCommand() *cobra.Command {
	command := &cobra.Command{Use: "mcp", Short: "Model Context Protocol commands"}
	command.AddCommand(newMCPServeCommand())
	return command
}

func newMCPServeCommand() *cobra.Command {
	var workspace string
	var maxBodyBytes int64
	var timeout time.Duration
	var allowPrivate bool
	command := &cobra.Command{
		Use:   "serve",
		Short: "Start the Hitspec MCP server over stdio",
		Long: `Start a bounded, workspace-scoped MCP server over stdin/stdout.

stdout is reserved for JSON-RPC. The server exposes fetch, request discovery,
and validation tools. Network fetches allow public destinations by default;
private and loopback targets require an explicit server-start option.

MCP client configuration:
  command: hitspec
  args: ["mcp", "serve", "--workspace", "/absolute/workspace"]`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(command *cobra.Command, _ []string) error {
			if maxBodyBytes <= 0 {
				return errors.New("--max-body-bytes must be greater than zero")
			}
			if timeout <= 0 {
				return errors.New("--timeout must be greater than zero")
			}
			server, err := hitspecmcp.NewServer(version, workspace, hitspecmcp.Options{
				MaxBodyBytes: maxBodyBytes, Timeout: timeout, AllowPrivateNetwork: allowPrivate,
			})
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return server.Run(ctx)
		},
	}
	command.Flags().StringVar(&workspace, "workspace", ".", "Fixed workspace exposed to MCP tools")
	command.Flags().Int64Var(&maxBodyBytes, "max-body-bytes", 1<<20, "Maximum fetched response body size in bytes")
	command.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Maximum duration of one HTTP request")
	command.Flags().BoolVar(&allowPrivate, "allow-private-network", false, "Allow loopback and private network targets")
	return command
}
