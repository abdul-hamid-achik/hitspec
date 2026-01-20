package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/abdul-hamid-achik/hitspec/packages/proxy"
	"github.com/spf13/cobra"
)

var (
	recordPortFlag      int
	recordTargetFlag    string
	recordOutputFlag    string
	recordExcludeFlag   string
	recordVerboseFlag   bool
	recordDedupeFlag    bool
	recordJSONFlag      bool
)

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Start a recording proxy to capture HTTP requests",
	Long: `Start an HTTP proxy that records requests and responses, then exports
them to hitspec format.

The proxy:
- Forwards all requests to the target server
- Records requests and responses
- Sanitizes sensitive headers (Authorization, Cookie, etc.)
- Exports to .http format on exit

Examples:
  hitspec record --port 8080 --target https://api.example.com
  hitspec record --port 8080 --target https://api.example.com -o recorded.http
  hitspec record --port 8080 --target https://api.example.com --exclude "/health,/metrics"
  hitspec record --port 8080 --target https://api.example.com --dedupe`,
	RunE: recordCommand,
}

func init() {
	recordCmd.Flags().IntVarP(&recordPortFlag, "port", "p", 8080, "Port to run the proxy on")
	recordCmd.Flags().StringVarP(&recordTargetFlag, "target", "t", "", "Target URL to proxy to (required)")
	recordCmd.Flags().StringVarP(&recordOutputFlag, "output", "o", "", "Output file path (default: stdout)")
	recordCmd.Flags().StringVar(&recordExcludeFlag, "exclude", "", "Paths to exclude from recording (comma-separated)")
	recordCmd.Flags().BoolVarP(&recordVerboseFlag, "verbose", "v", false, "Enable verbose logging")
	recordCmd.Flags().BoolVar(&recordDedupeFlag, "dedupe", false, "Skip duplicate requests (same method+path)")
	recordCmd.Flags().BoolVar(&recordJSONFlag, "json", false, "Export as JSON instead of .http format")

	_ = recordCmd.MarkFlagRequired("target")
}

func recordCommand(cmd *cobra.Command, args []string) error {
	if recordTargetFlag == "" {
		return fmt.Errorf("target URL is required (--target or -t)")
	}

	// Parse exclude paths
	var excludePaths []string
	if recordExcludeFlag != "" {
		for _, p := range strings.Split(recordExcludeFlag, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				excludePaths = append(excludePaths, p)
			}
		}
	}

	// Create recorder
	recorder := proxy.NewRecorder(
		proxy.WithPort(recordPortFlag),
		proxy.WithTargetURL(recordTargetFlag),
		proxy.WithVerbose(recordVerboseFlag),
		proxy.WithExclude(excludePaths),
		proxy.WithDeduplicate(recordDedupeFlag),
	)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nStopping proxy and exporting recordings...")
		cancel()
	}()

	// Start proxy in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- recorder.StartWithContext(ctx)
	}()

	// Wait for shutdown or error
	select {
	case err := <-errCh:
		if err != nil && ctx.Err() == nil {
			return err
		}
	case <-ctx.Done():
	}

	// Export recordings
	recordings := recorder.GetRecordings()
	if len(recordings) == 0 {
		fmt.Println("No requests recorded")
		return nil
	}

	fmt.Printf("\nRecorded %d requests\n", len(recordings))

	var output string
	if recordJSONFlag {
		data, err := recorder.ExportToJSON()
		if err != nil {
			return fmt.Errorf("failed to export to JSON: %w", err)
		}
		output = string(data)
	} else {
		output = recorder.Export()
	}

	// Write output
	if recordOutputFlag != "" {
		if err := os.WriteFile(recordOutputFlag, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Exported to %s\n", recordOutputFlag)
	} else {
		fmt.Println("\n" + output)
	}

	return nil
}
