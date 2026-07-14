package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/artifact"
	hitspecmcp "github.com/abdul-hamid-achik/hitspec/packages/mcp"
	"github.com/abdul-hamid-achik/hitspec/packages/search"
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
	var searchProviderName string
	var fcheapPath string
	var fcheapStashDir string
	command := &cobra.Command{
		Use:   "serve",
		Short: "Start the Hitspec MCP server over stdio",
		Long: `Start a bounded, workspace-scoped MCP server over stdin/stdout.

stdout is reserved for JSON-RPC. The server exposes fetch, request discovery,
and validation tools. A configured search provider adds public-web discovery;
a configured file.cheap executable adds explicit durable page capture. Network
fetches allow public destinations by default; private and loopback targets require
an explicit server-start option.

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
			var searchProvider search.Provider
			switch name := strings.ToLower(strings.TrimSpace(searchProviderName)); name {
			case "", "none":
			case "tavily":
				provider, err := search.NewTavily(os.Getenv("TAVILY_API_KEY"))
				if err != nil {
					return err
				}
				searchProvider = provider
			default:
				return fmt.Errorf("unsupported search provider %q (valid values: none, tavily)", name)
			}
			var artifactSink artifact.Sink
			if strings.TrimSpace(fcheapPath) != "" {
				sink, err := artifact.NewFcheap(fcheapPath, fcheapStashDir)
				if err != nil {
					return err
				}
				artifactSink = sink
			}
			server, err := hitspecmcp.NewServer(version, workspace, hitspecmcp.Options{
				MaxBodyBytes: maxBodyBytes, Timeout: timeout, AllowPrivateNetwork: allowPrivate,
				SearchProvider: searchProvider, ArtifactSink: artifactSink,
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
	command.Flags().StringVar(&searchProviderName, "search-provider", os.Getenv("HITSPEC_SEARCH_PROVIDER"), "Live web search provider: none or tavily")
	command.Flags().StringVar(&fcheapPath, "fcheap-path", os.Getenv("HITSPEC_FCHEAP_PATH"), "Fixed file.cheap executable used by durable webpage capture")
	command.Flags().StringVar(&fcheapStashDir, "fcheap-stash-dir", os.Getenv("HITSPEC_FCHEAP_STASH_DIR"), "Optional fixed file.cheap stash directory")
	return command
}
