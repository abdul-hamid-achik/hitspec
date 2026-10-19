package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/mock"
	"github.com/spf13/cobra"
)

var (
	mockHostFlag    string
	mockPortFlag    int
	mockCORSFlag    bool
	mockDelayFlag   string
	mockVerboseFlag bool
)

var mockCmd = &cobra.Command{
	Use:   "mock <file|directory>",
	Short: "Start a mock server based on hitspec files",
	Long: `Start an HTTP mock server that responds based on the requests and assertions
defined in your hitspec files.

The mock server:
- Parses your .http files to extract routes
- Generates mock responses from assertions or explicit mock blocks
- Supports path parameters (e.g., /users/{{id}})
- Can add artificial delays to simulate network latency
- Answers 405 with an Allow header when the path exists for another method,
  and serves HEAD from GET routes

It listens on 127.0.0.1 only. Use --host 0.0.0.0 to make it reachable from other
machines or from outside a container. A port that is already in use is an error.

Examples:
  hitspec mock api.http
  hitspec mock api.http --port 3000
  hitspec mock api.http --port 3000 --delay 100ms
  hitspec mock api.http --host 0.0.0.0 --cors
  hitspec mock ./tests/ --verbose`,
	Args: cobra.MinimumNArgs(1),
	RunE: mockCommand,
}

func init() {
	mockCmd.Flags().StringVar(&mockHostFlag, "host", mock.DefaultHost, "Address to bind (use 0.0.0.0 to expose the server beyond this machine)")
	mockCmd.Flags().BoolVar(&mockCORSFlag, "cors", false, "Answer CORS preflights and add Access-Control-Allow-* headers")
	mockCmd.Flags().IntVarP(&mockPortFlag, "port", "p", 3000, "Port to run the mock server on")
	mockCmd.Flags().StringVarP(&mockDelayFlag, "delay", "d", "0", "Delay to add to all responses (e.g., 100ms, 1s)")
	mockCmd.Flags().BoolVarP(&mockVerboseFlag, "verbose", "v", false, "Enable verbose logging")
}

func mockCommand(cmd *cobra.Command, args []string) error {
	// Parse delay
	var delay time.Duration
	if mockDelayFlag != "0" {
		var err error
		delay, err = time.ParseDuration(mockDelayFlag)
		if err != nil {
			return fmt.Errorf("invalid delay value %q: %w", mockDelayFlag, err)
		}
	}

	// Collect files
	files, err := collectFiles(args)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no .http or .hitspec files found in %v", args)
	}

	// Create server
	server := mock.NewServer(
		mock.WithHost(mockHostFlag),
		mock.WithPort(mockPortFlag),
		mock.WithCORS(mockCORSFlag),
		mock.WithDelay(delay),
		mock.WithVerbose(mockVerboseFlag),
	)

	// Load files
	if err := server.LoadFiles(files); err != nil {
		return fmt.Errorf("failed to load files: %w", err)
	}

	routes := server.GetRoutes()
	if len(routes) == 0 {
		return fmt.Errorf("no routes found in the provided files; make sure your .http files contain HTTP requests")
	}

	// Bind before announcing anything, so a busy port is an error and not a
	// server that "started".
	if _, err := server.Listen(); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Mock server: loaded %d route(s) from %d file(s)\n", len(routes), len(files))
	if ip := net.ParseIP(mockHostFlag); mockHostFlag == "" || (ip != nil && ip.IsUnspecified()) {
		fmt.Fprintln(cmd.ErrOrStderr(), "warning: the mock server is listening on all network interfaces")
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down mock server...")
		cancel()
	}()

	// Start server
	return server.StartWithContext(ctx)
}
