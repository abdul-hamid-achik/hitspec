package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
	"github.com/abdul-hamid-achik/hitspec/packages/output"
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
  hitspec run api.http --name "createUser"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runCommand,
}

const (
	// WatchDebounceDelay is the debounce delay for file watch events
	WatchDebounceDelay = 300 * time.Millisecond
)

var (
	envFlag        string
	nameFlag       string
	tagsFlag       string
	verboseFlag    bool
	bailFlag       bool
	timeoutFlag    int
	noColorFlag    bool
	dryRunFlag     bool
	outputFlag     string
	outputFileFlag string
	parallelFlag   bool
	concurrencyFlag int
	watchFlag      bool
)

func init() {
	runCmd.Flags().StringVarP(&envFlag, "env", "e", "dev", "Environment to use")
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

	cfg := &runner.Config{
		Environment:    envFlag,
		Verbose:        verboseFlag,
		Timeout:        time.Duration(timeoutFlag) * time.Millisecond,
		FollowRedirect: true,
		Bail:           bailFlag,
		NameFilter:     nameFlag,
		TagsFilter:     tagsFilter,
		Parallel:       parallelFlag,
		Concurrency:    concurrencyFlag,
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
