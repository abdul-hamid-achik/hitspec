package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/packages/http"
	"github.com/abdul-hamid-achik/hitspec/packages/stress"
	"github.com/spf13/cobra"
)

var stressCmd = &cobra.Command{
	Use:   "stress <file>",
	Short: "Run stress tests against an API",
	Long: `Run Artillery-style stress tests using requests defined in .http files.

Examples:
  # Simple constant rate test
  hitspec stress api.http --duration 1m --rate 100

  # Virtual users mode with think time
  hitspec stress api.http --duration 2m --vus 50 --think-time 1s

  # With ramp-up
  hitspec stress api.http --duration 5m --rate 200 --ramp-up 30s

  # Using config profile
  hitspec stress api.http --profile load --env staging

  # With thresholds for CI/CD
  hitspec stress api.http -d 1m -r 100 --threshold "p95<200ms,errors<0.1%"`,
	Args: cobra.ExactArgs(1),
	RunE: stressCommand,
}

var (
	stressDurationFlag   string
	stressRateFlag       float64
	stressVUsFlag        int
	stressMaxVUsFlag     int
	stressThinkTimeFlag  string
	stressRampUpFlag     string
	stressThresholdFlag  string
	stressProfileFlag    string
	stressEnvFlag        string
	stressNoProgressFlag bool
	stressNoColorFlag    bool
	stressVerboseFlag    bool
	stressJSONFlag       bool
	stressProxyFlag      string
	stressInsecureFlag   bool
)

func init() {
	stressCmd.Flags().StringVarP(&stressDurationFlag, "duration", "d", "30s", "Test duration (e.g., 30s, 5m, 1h)")
	stressCmd.Flags().Float64VarP(&stressRateFlag, "rate", "r", 10, "Target requests per second")
	stressCmd.Flags().IntVarP(&stressVUsFlag, "vus", "u", 0, "Number of virtual users (alternative to rate)")
	stressCmd.Flags().IntVar(&stressMaxVUsFlag, "max-vus", 100, "Maximum concurrent requests")
	stressCmd.Flags().StringVarP(&stressThinkTimeFlag, "think-time", "t", "0s", "Think time between requests per VU")
	stressCmd.Flags().StringVar(&stressRampUpFlag, "ramp-up", "0s", "Ramp-up time to reach target rate/VUs")
	stressCmd.Flags().StringVar(&stressThresholdFlag, "threshold", "", "Pass/fail thresholds (e.g., \"p95<200ms,errors<0.1%\")")
	stressCmd.Flags().StringVarP(&stressProfileFlag, "profile", "p", "", "Load stress profile from config")
	stressCmd.Flags().StringVarP(&stressEnvFlag, "env", "e", "", "Environment to use")
	stressCmd.Flags().BoolVar(&stressNoProgressFlag, "no-progress", false, "Disable real-time progress display")
	stressCmd.Flags().BoolVar(&stressNoColorFlag, "no-color", false, "Disable colored output")
	stressCmd.Flags().BoolVarP(&stressVerboseFlag, "verbose", "v", false, "Verbose output with per-request breakdown")
	stressCmd.Flags().BoolVar(&stressJSONFlag, "json", false, "Output results as JSON")
	stressCmd.Flags().StringVar(&stressProxyFlag, "proxy", "", "Proxy URL for HTTP requests")
	stressCmd.Flags().BoolVarP(&stressInsecureFlag, "insecure", "k", false, "Disable SSL certificate validation")
}

func stressCommand(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	// Validate file exists
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("cannot access file: %w", err)
	}

	// Load config from file
	fileConfig, _ := config.LoadConfig("")

	// Build stress config
	cfg, err := buildStressConfig(fileConfig)
	if err != nil {
		return err
	}

	// Create HTTP client
	clientOpts := []http.ClientOption{}
	if fileConfig != nil {
		clientOpts = append(clientOpts, http.WithFollowRedirects(fileConfig.GetFollowRedirects()))
		if fileConfig.Proxy != "" && stressProxyFlag == "" {
			stressProxyFlag = fileConfig.Proxy
		}
	}
	if stressProxyFlag != "" {
		clientOpts = append(clientOpts, http.WithProxy(stressProxyFlag))
	}
	validateSSL := true
	if fileConfig != nil {
		validateSSL = fileConfig.GetValidateSSL()
	}
	if stressInsecureFlag {
		validateSSL = false
	}
	clientOpts = append(clientOpts, http.WithValidateSSL(validateSSL))
	client := http.NewClient(clientOpts...)

	// Create resolver
	resolver := env.NewResolver()
	resolver.SetWarnFunc(func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
	})

	// Create reporter
	reporter := stress.NewReporter(
		stress.WithNoColor(stressNoColorFlag),
		stress.WithNoProgress(stressNoProgressFlag),
		stress.WithVerbose(stressVerboseFlag),
	)

	// Create runner
	runner := stress.NewRunner(cfg,
		stress.WithHTTPClient(client),
		stress.WithResolver(resolver),
		stress.WithReporter(reporter),
	)

	// Load file
	if err := runner.LoadFile(filePath); err != nil {
		return err
	}

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nReceived interrupt, stopping gracefully...")
		cancel()
	}()

	// Run the stress test
	result, err := runner.Run(ctx)
	if err != nil {
		return err
	}

	// Output JSON if requested
	if stressJSONFlag {
		return reporter.JSONSummary(result.Summary, result.Thresholds)
	}

	// Exit with error code if thresholds failed
	if result.HasThresholdFailures() {
		os.Exit(1)
	}

	return nil
}

func buildStressConfig(fileConfig *config.Config) (*stress.Config, error) {
	cfg := stress.DefaultConfig()

	// Load from profile if specified
	if stressProfileFlag != "" && fileConfig != nil && fileConfig.Stress != nil {
		if profile, ok := fileConfig.Stress.Profiles[stressProfileFlag]; ok {
			if profile.Duration != "" {
				d, err := time.ParseDuration(profile.Duration)
				if err != nil {
					return nil, fmt.Errorf("invalid duration in profile: %w", err)
				}
				cfg.Duration = d
			}
			if profile.Rate > 0 {
				cfg.Rate = profile.Rate
			}
			if profile.VUs > 0 {
				cfg.VUs = profile.VUs
				cfg.Mode = stress.VUMode
			}
			if profile.MaxVUs > 0 {
				cfg.MaxVUs = profile.MaxVUs
			}
			if profile.ThinkTime != "" {
				d, err := time.ParseDuration(profile.ThinkTime)
				if err != nil {
					return nil, fmt.Errorf("invalid think time in profile: %w", err)
				}
				cfg.ThinkTime = d
			}
			if profile.RampUp != "" {
				d, err := time.ParseDuration(profile.RampUp)
				if err != nil {
					return nil, fmt.Errorf("invalid ramp-up in profile: %w", err)
				}
				cfg.RampUp = d
			}
			if len(profile.Thresholds) > 0 {
				thresholdStr := buildThresholdString(profile.Thresholds)
				t, err := stress.ParseThresholds(thresholdStr)
				if err != nil {
					return nil, fmt.Errorf("invalid thresholds in profile: %w", err)
				}
				cfg.Thresholds = t
			}
		} else {
			return nil, fmt.Errorf("stress profile %q not found in config", stressProfileFlag)
		}
	}

	// Override with CLI flags
	if stressDurationFlag != "30s" { // Only override if explicitly set
		d, err := time.ParseDuration(stressDurationFlag)
		if err != nil {
			return nil, fmt.Errorf("invalid duration: %w", err)
		}
		cfg.Duration = d
	}

	if stressRateFlag != 10 { // Only override if explicitly set
		cfg.Rate = stressRateFlag
	}

	if stressVUsFlag > 0 {
		cfg.VUs = stressVUsFlag
		cfg.Mode = stress.VUMode
	}

	if stressMaxVUsFlag != 100 { // Only override if explicitly set
		cfg.MaxVUs = stressMaxVUsFlag
	}

	if stressThinkTimeFlag != "0s" {
		d, err := time.ParseDuration(stressThinkTimeFlag)
		if err != nil {
			return nil, fmt.Errorf("invalid think time: %w", err)
		}
		cfg.ThinkTime = d
	}

	if stressRampUpFlag != "0s" {
		d, err := time.ParseDuration(stressRampUpFlag)
		if err != nil {
			return nil, fmt.Errorf("invalid ramp-up: %w", err)
		}
		cfg.RampUp = d
	}

	if stressThresholdFlag != "" {
		t, err := stress.ParseThresholds(stressThresholdFlag)
		if err != nil {
			return nil, fmt.Errorf("invalid thresholds: %w", err)
		}
		cfg.Thresholds = t
	}

	return cfg, nil
}

func buildThresholdString(thresholds map[string]string) string {
	var parts []string
	for k, v := range thresholds {
		parts = append(parts, k+"<"+v)
	}
	return strings.Join(parts, ",")
}
