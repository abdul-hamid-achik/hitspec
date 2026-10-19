package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/artifact"
	hitspecmcp "github.com/abdul-hamid-achik/hitspec/v2/packages/mcp"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/search"
	"github.com/spf13/cobra"
)

var mcpCmd = newMCPCommand()

func newMCPCommand() *cobra.Command {
	command := &cobra.Command{Use: "mcp", Short: "Model Context Protocol commands"}
	command.AddCommand(newMCPServeCommand())
	command.AddCommand(newMCPProbeCommand())
	command.AddCommand(newMCPCallCommand())
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
	var allowBroadWorkspace bool
	var allowEnv []string
	command := &cobra.Command{
		Use:   "serve",
		Short: "Start the Hitspec MCP server over stdio",
		Long: `Start a bounded, workspace-scoped MCP server over stdin/stdout.

stdout is reserved for JSON-RPC. The server exposes fetch, request discovery,
and validation tools. A configured search provider adds public-web discovery;
a configured file.cheap executable adds explicit durable page capture. Network
fetches allow public destinations by default; private and loopback targets require
an explicit server-start option.

Saved requests (hitspec_fetch with file) are resolved in isolation: {{NAME}},
{{$NAME}}, {{$env(NAME)}} and ${NAME} read only the workspace's hitspec.yaml
environment, the optional env file and the file's own variables, never the
server's process environment, unless the operator lists a variable with
--allow-env. The search provider key is removed from the process environment
at startup. The workspace may not be the home directory, one of its ancestors
or the filesystem root unless --allow-broad-workspace is given.

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
			searchProvider, err := newSearchProvider(searchProviderName)
			if err != nil {
				return err
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
				AllowedEnv: allowEnv, AllowBroadWorkspace: allowBroadWorkspace,
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
	command.Flags().BoolVar(&allowPrivate, "allow-private-network", false, "Allow non-public network targets and standard saved-request modes")
	command.Flags().BoolVar(&allowBroadWorkspace, "allow-broad-workspace", false, "Allow a workspace that is the home directory, contains it, or is the filesystem root")
	command.Flags().StringSliceVar(&allowEnv, "allow-env", nil, "Process environment variable saved requests may read (repeatable; default none)")
	command.Flags().StringVar(&searchProviderName, "search-provider", os.Getenv("HITSPEC_SEARCH_PROVIDER"), "Live web search provider: none or tavily")
	command.Flags().StringVar(&fcheapPath, "fcheap-path", os.Getenv("HITSPEC_FCHEAP_PATH"), "Fixed file.cheap executable used by durable webpage capture")
	command.Flags().StringVar(&fcheapStashDir, "fcheap-stash-dir", os.Getenv("HITSPEC_FCHEAP_STASH_DIR"), "Optional fixed file.cheap stash directory")
	return command
}

// newSearchProvider builds the server-owned search provider. The Tavily key is
// read once and then removed from the process environment, so nothing running
// later in this process (saved-request resolution, child processes) can read it.
func newSearchProvider(name string) (search.Provider, error) {
	defer func() { _ = os.Unsetenv("TAVILY_API_KEY") }()
	switch name = strings.ToLower(strings.TrimSpace(name)); name {
	case "", "none":
		return nil, nil
	case "tavily":
		provider, err := search.NewTavily(os.Getenv("TAVILY_API_KEY"))
		if err != nil {
			return nil, err
		}
		return provider, nil
	default:
		return nil, fmt.Errorf("unsupported search provider %q (valid values: none, tavily)", name)
	}
}
