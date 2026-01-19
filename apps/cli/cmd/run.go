package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
	"github.com/abdul-hamid-achik/hitspec/packages/http"
	"github.com/abdul-hamid-achik/hitspec/packages/output"
	"github.com/abdul-hamid-achik/hitspec/packages/stress"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <file|directory>",
	Short: "Run API tests from hitspec files",
	Long: `Run API tests defined in .http or .hitspec files.

Examples:
  hitspec run api.http
  hitspec run api.http --env staging
  hitspec run ./tests/ --tags smoke
  hitspec run api.http --name "createUser"

Stress Testing Mode:
  hitspec run api.http --stress --duration 1m --rate 100
  hitspec run ./tests/ --stress --duration 1m --rate 100
  hitspec run api.http users.http --stress -d 1m -r 100
  hitspec run api.http --stress --vus 50 --think-time 1s
  hitspec run api.http --stress -d 1m -r 100 --threshold "p95<200ms,errors<0.1%"
  hitspec run api.http --stress --profile load --env staging`,
	Args: cobra.MinimumNArgs(1),
	RunE: runCommand,
}

const (
	// WatchDebounceDelay is the debounce delay for file watch events
	WatchDebounceDelay = 300 * time.Millisecond
)

var (
	envFlag         string
	envFileFlag     string
	nameFlag        string
	tagsFlag        string
	verboseFlag     bool
	bailFlag        bool
	timeoutFlag     int
	noColorFlag     bool
	dryRunFlag      bool
	outputFlag      string
	outputFileFlag  string
	parallelFlag    bool
	concurrencyFlag int
	watchFlag       bool
	proxyFlag       string
	insecureFlag    bool

	// Stress testing flags
	stressFlag           bool
	stressDurationFlag   string
	stressRateFlag       float64
	stressVUsFlag        int
	stressMaxVUsFlag     int
	stressThinkTimeFlag  string
	stressRampUpFlag     string
	stressThresholdFlag  string
	stressProfileFlag    string
	stressNoProgressFlag bool
	stressJSONFlag       bool
)

func init() {
	runCmd.Flags().StringVarP(&envFlag, "env", "e", "dev", "Environment to use")
	runCmd.Flags().StringVar(&envFileFlag, "env-file", "", "Path to .env file for variable interpolation")
	runCmd.Flags().StringVarP(&nameFlag, "name", "n", "", "Run only tests matching name pattern")
	runCmd.Flags().StringVarP(&tagsFlag, "tags", "t", "", "Run only tests with specified tags (comma-separated)")
	runCmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "Verbose output")
	runCmd.Flags().BoolVar(&bailFlag, "bail", false, "Stop on first failure")
	runCmd.Flags().IntVar(&timeoutFlag, "timeout", 30000, "Global timeout in milliseconds")
	runCmd.Flags().BoolVar(&noColorFlag, "no-color", false, "Disable colored output")
	runCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Parse and show what would run without executing")
	runCmd.Flags().StringVarP(&outputFlag, "output", "o", "console", "Output format: console, json, junit, tap")
	runCmd.Flags().StringVar(&outputFileFlag, "output-file", "", "Write output to file (default: stdout)")
	runCmd.Flags().BoolVarP(&parallelFlag, "parallel", "p", false, "Run requests in parallel (when no dependencies)")
	runCmd.Flags().IntVar(&concurrencyFlag, "concurrency", 5, "Number of concurrent requests when running in parallel")
	runCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch files for changes and re-run tests")
	runCmd.Flags().StringVar(&proxyFlag, "proxy", "", "Proxy URL for HTTP requests")
	runCmd.Flags().BoolVarP(&insecureFlag, "insecure", "k", false, "Disable SSL certificate validation")

	// Stress testing flags
	runCmd.Flags().BoolVar(&stressFlag, "stress", false, "Enable stress testing mode")
	runCmd.Flags().StringVarP(&stressDurationFlag, "duration", "d", "30s", "Stress test duration (e.g., 30s, 5m, 1h)")
	runCmd.Flags().Float64VarP(&stressRateFlag, "rate", "r", 10, "Target requests per second")
	runCmd.Flags().IntVar(&stressVUsFlag, "vus", 0, "Number of virtual users (alternative to rate)")
	runCmd.Flags().IntVar(&stressMaxVUsFlag, "max-vus", 100, "Maximum concurrent requests")
	runCmd.Flags().StringVar(&stressThinkTimeFlag, "think-time", "0s", "Think time between requests per VU")
	runCmd.Flags().StringVar(&stressRampUpFlag, "ramp-up", "0s", "Ramp-up time to reach target rate/VUs")
	runCmd.Flags().StringVar(&stressThresholdFlag, "threshold", "", "Pass/fail thresholds (e.g., \"p95<200ms,errors<0.1%\")")
	runCmd.Flags().StringVar(&stressProfileFlag, "profile", "", "Load stress profile from config")
	runCmd.Flags().BoolVar(&stressNoProgressFlag, "no-progress", false, "Disable real-time progress display")
	runCmd.Flags().BoolVar(&stressJSONFlag, "stress-json", false, "Output stress results as JSON")
}

// Formatter interface for all output formatters
type Formatter interface {
	FormatResult(result *runner.RunResult)
	FormatError(err error)
	FormatHeader(version string)
}

// Flushable interface for formatters that need to flush output
type Flushable interface {
	Flush(totalDuration time.Duration) error
}

func runCommand(cmd *cobra.Command, args []string) error {
	// Setup output writer
	var outWriter *os.File
	var err error
	if outputFileFlag != "" {
		outWriter, err = os.Create(outputFileFlag)
		if err != nil {
			return fmt.Errorf("cannot create output file: %w", err)
		}
		defer outWriter.Close()
	}

	// Create formatter based on output flag
	var formatter Formatter
	switch strings.ToLower(outputFlag) {
	case "json":
		opts := []output.JSONOption{}
		if outWriter != nil {
			opts = append(opts, output.JSONWithWriter(outWriter))
		}
		formatter = output.NewJSONFormatter(opts...)
	case "junit":
		opts := []output.JUnitOption{}
		if outWriter != nil {
			opts = append(opts, output.JUnitWithWriter(outWriter))
		}
		formatter = output.NewJUnitFormatter(opts...)
	case "tap":
		opts := []output.TAPOption{}
		if outWriter != nil {
			opts = append(opts, output.TAPWithWriter(outWriter))
		}
		formatter = output.NewTAPFormatter(opts...)
	default: // "console"
		consoleOpts := []output.ConsoleOption{
			output.WithVerbose(verboseFlag),
			output.WithNoColor(noColorFlag),
		}
		if outWriter != nil {
			consoleOpts = append(consoleOpts, output.WithWriter(outWriter))
		}
		formatter = output.NewConsoleFormatter(consoleOpts...)
	}

	formatter.FormatHeader(version)

	files, err := collectFiles(args)
	if err != nil {
		formatter.FormatError(err)
		return err
	}

	if len(files) == 0 {
		formatter.FormatError(fmt.Errorf("no .http or .hitspec files found"))
		return fmt.Errorf("no files found")
	}

	var tagsFilter []string
	if tagsFlag != "" {
		for _, t := range strings.Split(tagsFlag, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tagsFilter = append(tagsFilter, t)
			}
		}
	}

	// Load config from file (if present) and apply CLI overrides
	fileConfig, _ := config.LoadConfig("")

	// If stress mode is enabled, delegate to stress runner
	if stressFlag {
		return runStressMode(cmd, files, fileConfig)
	}

	// Determine proxy and validateSSL from config file, allowing CLI flags to override
	proxy := fileConfig.Proxy
	if proxyFlag != "" {
		proxy = proxyFlag
	}

	validateSSL := fileConfig.GetValidateSSL()
	if insecureFlag {
		validateSSL = false
	}

	cfg := &runner.Config{
		Environment:        envFlag,
		EnvFile:            envFileFlag,
		Verbose:            verboseFlag,
		Timeout:            time.Duration(timeoutFlag) * time.Millisecond,
		FollowRedirect:     fileConfig.GetFollowRedirects(),
		Bail:               bailFlag,
		NameFilter:         nameFlag,
		TagsFilter:         tagsFilter,
		Parallel:           parallelFlag,
		Concurrency:        concurrencyFlag,
		ValidateSSL:        validateSSL,
		Proxy:              proxy,
		DefaultHeaders:     fileConfig.Headers,
		ConfigEnvironments: fileConfig.Environments,
	}

	r := runner.NewRunner(cfg)

	// Create a function to run all tests
	runTests := func() (int, int, int, time.Duration) {
		totalPassed := 0
		totalFailed := 0
		totalSkipped := 0
		startTime := time.Now()

		for _, file := range files {
			if dryRunFlag {
				fmt.Fprintf(cmd.OutOrStdout(), "Would run: %s\n", file)
				continue
			}

			result, err := r.RunFile(file)
			if err != nil {
				formatter.FormatError(err)
				if bailFlag {
					break
				}
				continue
			}

			formatter.FormatResult(result)
			totalPassed += result.Passed
			totalFailed += result.Failed
			totalSkipped += result.Skipped

			if bailFlag && result.Failed > 0 {
				break
			}
		}

		return totalPassed, totalFailed, totalSkipped, time.Since(startTime)
	}

	// Run tests once
	_, totalFailed, _, totalDuration := runTests()

	// Flush output for formatters that accumulate results
	if flushable, ok := formatter.(Flushable); ok {
		if err := flushable.Flush(totalDuration); err != nil {
			return fmt.Errorf("error writing output: %w", err)
		}
	}

	// If watch mode is not enabled, exit normally
	if !watchFlag {
		if totalFailed > 0 {
			os.Exit(1)
		}
		return nil
	}

	// Watch mode: set up file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	// Add files and directories to watch
	watchedDirs := make(map[string]bool)
	for _, file := range files {
		dir := filepath.Dir(file)
		if !watchedDirs[dir] {
			if err := watcher.Add(dir); err != nil {
				formatter.FormatError(fmt.Errorf("failed to watch %s: %w", dir, err))
			}
			watchedDirs[dir] = true
		}
	}

	// Also watch the original args if they're directories
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err == nil && info.IsDir() {
			_ = filepath.Walk(arg, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() && !watchedDirs[path] {
					_ = watcher.Add(path)
					watchedDirs[path] = true
				}
				return nil
			})
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nWatching for changes... (press Ctrl+C to stop)\n\n")

	// Debounce timer for rapid file changes
	var debounceTimer *time.Timer

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Only react to write events on hitspec files
			if event.Has(fsnotify.Write) && isHitspecFile(event.Name) {
				// Debounce: reset timer on each event
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(WatchDebounceDelay, func() {
					fmt.Fprintf(cmd.OutOrStdout(), "\n\nFile changed: %s\nRe-running tests...\n\n", event.Name)

					// Re-create formatter for new output (for JSON/JUnit, need fresh state)
					switch strings.ToLower(outputFlag) {
					case "json":
						formatter = output.NewJSONFormatter()
					case "junit":
						formatter = output.NewJUnitFormatter()
					case "tap":
						formatter = output.NewTAPFormatter()
					default:
						formatter = output.NewConsoleFormatter(
							output.WithVerbose(verboseFlag),
							output.WithNoColor(noColorFlag),
						)
					}

					// Re-run tests
					_, _, _, duration := runTests()

					// Flush output
					if flushable, ok := formatter.(Flushable); ok {
						_ = flushable.Flush(duration)
					}

					fmt.Fprintf(cmd.OutOrStdout(), "\nWatching for changes... (press Ctrl+C to stop)\n")
				})
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			formatter.FormatError(fmt.Errorf("watcher error: %w", err))
		}
	}
}

func collectFiles(args []string) ([]string, error) {
	var files []string

	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, fmt.Errorf("cannot access %s: %w", arg, err)
		}

		if info.IsDir() {
			err := filepath.Walk(arg, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && isHitspecFile(path) {
					files = append(files, path)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			if isHitspecFile(arg) {
				files = append(files, arg)
			}
		}
	}

	return files, nil
}

func isHitspecFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".http" || ext == ".hitspec"
}

// runStressMode executes stress tests using the stress runner
func runStressMode(cmd *cobra.Command, files []string, fileConfig *config.Config) error {
	// Build stress config
	cfg, err := buildStressConfig(fileConfig)
	if err != nil {
		return err
	}

	// Create HTTP client
	clientOpts := []http.ClientOption{}
	if fileConfig != nil {
		clientOpts = append(clientOpts, http.WithFollowRedirects(fileConfig.GetFollowRedirects()))
		if fileConfig.Proxy != "" && proxyFlag == "" {
			proxyFlag = fileConfig.Proxy
		}
	}
	if proxyFlag != "" {
		clientOpts = append(clientOpts, http.WithProxy(proxyFlag))
	}
	validateSSL := true
	if fileConfig != nil {
		validateSSL = fileConfig.GetValidateSSL()
	}
	if insecureFlag {
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
		stress.WithNoColor(noColorFlag),
		stress.WithNoProgress(stressNoProgressFlag),
		stress.WithVerbose(verboseFlag),
	)

	// Create runner with config environments for proper variable resolution
	runnerOpts := []stress.RunnerOption{
		stress.WithHTTPClient(client),
		stress.WithResolver(resolver),
		stress.WithReporter(reporter),
	}
	if envFlag != "" {
		runnerOpts = append(runnerOpts, stress.WithEnvironment(envFlag))
	}
	if envFileFlag != "" {
		runnerOpts = append(runnerOpts, stress.WithEnvFile(envFileFlag))
	}
	// Pass config environments for proper variable resolution
	if fileConfig != nil && fileConfig.Environments != nil {
		runnerOpts = append(runnerOpts, stress.WithConfigEnvironments(fileConfig.Environments))
	}
	stressRunner := stress.NewRunner(cfg, runnerOpts...)

	// Load files (supports single file, multiple files, or directories)
	if err := stressRunner.LoadFiles(files); err != nil {
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
	result, err := stressRunner.Run(ctx)
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

// buildStressConfig builds stress configuration from flags and profile
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

// buildThresholdString converts threshold map to string format
func buildThresholdString(thresholds map[string]string) string {
	var parts []string
	for k, v := range thresholds {
		parts = append(parts, k+"<"+v)
	}
	return strings.Join(parts, ",")
}
