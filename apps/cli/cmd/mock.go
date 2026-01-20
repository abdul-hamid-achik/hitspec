package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/mock"
	"github.com/spf13/cobra"
)

var (
	mockPortFlag    int
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

Examples:
  hitspec mock api.http
  hitspec mock api.http --port 3000
  hitspec mock api.http --port 3000 --delay 100ms
  hitspec mock ./tests/ --verbose`,
	Args: cobra.MinimumNArgs(1),
	RunE: mockCommand,
}

func init() {
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
		return fmt.Errorf("no .http or .hitspec files found")
	}

	// Create server
	server := mock.NewServer(
		mock.WithPort(mockPortFlag),
		mock.WithDelay(delay),
		mock.WithVerbose(mockVerboseFlag),
	)

	// Load files
	if err := server.LoadFiles(files); err != nil {
		return fmt.Errorf("failed to load files: %w", err)
	}

	routes := server.GetRoutes()
	if len(routes) == 0 {
		return fmt.Errorf("no routes found in the provided files")
	}

	fmt.Printf("Loaded %d routes from %d files\n", len(routes), len(files))

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
