package cmd

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runconfig"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/coverage"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/export/metrics"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/history"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/notify"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/output"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/stress"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
	verboseFlag     int // 0=off, 1=-v, 2=-vv, 3=-vvv
	quietFlag       bool
	bailFlag        bool
	timeoutFlag     string
	noColorFlag     bool
	dryRunFlag      bool
	outputFlag      string
	outputFileFlag  string
	parallelFlag    bool
	concurrencyFlag int
	watchFlag       bool
	proxyFlag       string
	insecureFlag    bool
	configFlag      string

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

	// Metrics flags
	metricsFlag       string
	metricsPortFlag   int
	metricsFileFlag   string
	datadogAPIKeyFlag string
	datadogSiteFlag   string
	datadogTagsFlag   string

	// Notification flags
	notifyFlag       string
	notifyOnFlag     string
	slackWebhookFlag string
	slackChannelFlag string
	teamsWebhookFlag string

	// Snapshot testing flags
	updateSnapshotsFlag bool

	// Security flags
	allowShellFlag bool
	allowDBFlag    bool

	// Coverage flags
	coverageFlag bool
	openapiFlag  string

	// History flags
	historyDBFlag string
	noHistoryFlag bool
)

// registerRunFlags binds the run flags to their package variables.
//
// Flags that mirror a hitspec.yaml key (timeout, concurrency, proxy, insecure,
// parallel, bail) default to "not set" instead of the effective default, so
// the config file can supply the value; runOverrides layers the HITSPEC_*
// variable under an explicit flag and runconfig.Build resolves the rest. The
// help text states the effective default.
func registerRunFlags(f *pflag.FlagSet) {
	// Core flags
	f.StringVarP(&envFlag, "env", "e", getEnvString("HITSPEC_ENV", ""), "Environment to use (env: HITSPEC_ENV; default: hitspec.yaml defaultEnvironment, else dev)")
	f.StringVar(&envFileFlag, "env-file", getEnvString("HITSPEC_ENV_FILE", ""), "Path to .env file for variable interpolation (env: HITSPEC_ENV_FILE)")
	f.StringVar(&configFlag, "config", getEnvString("HITSPEC_CONFIG", ""), "Path to config file (env: HITSPEC_CONFIG)")
	f.StringVarP(&nameFlag, "name", "n", "", "Run only tests matching name pattern")
	f.StringVarP(&tagsFlag, "tags", "t", getEnvString("HITSPEC_TAGS", ""), "Run only tests with specified tags (comma-separated) (env: HITSPEC_TAGS)")

	// Output flags
	f.CountVarP(&verboseFlag, "verbose", "v", "Verbose output (-v, -vv, -vvv for more detail)")
	f.BoolVarP(&quietFlag, "quiet", "q", getEnvBool("HITSPEC_QUIET", false), "Suppress the report on stdout; failures are still listed on stderr and --output-file is still written (env: HITSPEC_QUIET)")
	f.BoolVar(&noColorFlag, "no-color", getEnvBool("HITSPEC_NO_COLOR", false), "Disable colored output (env: HITSPEC_NO_COLOR)")
	f.StringVarP(&outputFlag, "output", "o", getEnvString("HITSPEC_OUTPUT", "console"), "Output format: console, json, junit, tap, html (env: HITSPEC_OUTPUT)")
	f.StringVar(&outputFileFlag, "output-file", getEnvString("HITSPEC_OUTPUT_FILE", ""), "Write output to file (default: stdout) (env: HITSPEC_OUTPUT_FILE)")

	// Execution flags
	f.BoolVar(&bailFlag, "bail", false, "Stop on first failure (env: HITSPEC_BAIL; default: hitspec.yaml bail, else false)")
	f.StringVar(&timeoutFlag, "timeout", "", "Request timeout, e.g. 30s, 1m (env: HITSPEC_TIMEOUT; default: hitspec.yaml timeout, else 30s)")
	f.BoolVar(&dryRunFlag, "dry-run", false, "Parse and show what would run without executing")
	f.BoolVarP(&parallelFlag, "parallel", "p", false, "Run requests in parallel (when no dependencies) (env: HITSPEC_PARALLEL; default: hitspec.yaml parallel, else false)")
	f.IntVar(&concurrencyFlag, "concurrency", 0, "Number of concurrent requests when running in parallel (env: HITSPEC_CONCURRENCY; default: hitspec.yaml concurrency, else 5)")
	f.BoolVarP(&watchFlag, "watch", "w", false, "Watch files for changes and re-run tests")

	// Network flags
	f.StringVar(&proxyFlag, "proxy", "", "Proxy URL for HTTP requests (env: HITSPEC_PROXY; default: hitspec.yaml proxy)")
	f.BoolVarP(&insecureFlag, "insecure", "k", false, "Disable SSL certificate validation (env: HITSPEC_INSECURE; default: hitspec.yaml validateSSL, else validate)")

	// Stress testing flags
	f.BoolVar(&stressFlag, "stress", false, "Enable stress testing mode")
	f.StringVarP(&stressDurationFlag, "duration", "d", "30s", "Stress test duration (e.g., 30s, 5m, 1h)")
	f.Float64VarP(&stressRateFlag, "rate", "r", 10, "Target requests per second")
	f.IntVar(&stressVUsFlag, "vus", 0, "Number of virtual users (alternative to rate)")
	f.IntVar(&stressMaxVUsFlag, "max-vus", 100, "Maximum concurrent requests")
	f.StringVar(&stressThinkTimeFlag, "think-time", "0s", "Think time between requests per VU")
	f.StringVar(&stressRampUpFlag, "ramp-up", "0s", "Ramp-up time to reach target rate/VUs")
	f.StringVar(&stressThresholdFlag, "threshold", "", "Pass/fail thresholds (e.g., \"p95<200ms,errors<0.1%\")")
	f.StringVar(&stressProfileFlag, "profile", "", "Load stress profile from config")
	f.BoolVar(&stressNoProgressFlag, "no-progress", false, "Disable real-time progress display")
	f.BoolVar(&stressJSONFlag, "stress-json", false, "Output stress results as JSON")

	// Metrics flags
	f.StringVar(&metricsFlag, "metrics", getEnvString("HITSPEC_METRICS", ""), "Metrics export format: prometheus, datadog, json (env: HITSPEC_METRICS)")
	f.IntVar(&metricsPortFlag, "metrics-port", getEnvInt("HITSPEC_METRICS_PORT", 9090), "Port for Prometheus metrics HTTP endpoint (env: HITSPEC_METRICS_PORT)")
	f.StringVar(&metricsFileFlag, "metrics-file", getEnvString("HITSPEC_METRICS_FILE", ""), "Output file for metrics (JSON format) (env: HITSPEC_METRICS_FILE)")
	f.StringVar(&datadogAPIKeyFlag, "datadog-api-key", getEnvString("DD_API_KEY", ""), "DataDog API key (env: DD_API_KEY)")
	f.StringVar(&datadogSiteFlag, "datadog-site", getEnvString("DD_SITE", "datadoghq.com"), "DataDog site (env: DD_SITE)")
	f.StringVar(&datadogTagsFlag, "datadog-tags", getEnvString("DD_TAGS", ""), "Comma-separated DataDog tags (env: DD_TAGS)")

	// Notification flags
	f.StringVar(&notifyFlag, "notify", getEnvString("HITSPEC_NOTIFY", ""), "Notification service: slack, teams (env: HITSPEC_NOTIFY)")
	f.StringVar(&notifyOnFlag, "notify-on", getEnvString("HITSPEC_NOTIFY_ON", "failure"), "When to notify: always, failure, success, recovery (env: HITSPEC_NOTIFY_ON)")
	f.StringVar(&slackWebhookFlag, "slack-webhook", getEnvString("SLACK_WEBHOOK", ""), "Slack webhook URL (env: SLACK_WEBHOOK)")
	f.StringVar(&slackChannelFlag, "slack-channel", getEnvString("SLACK_CHANNEL", ""), "Slack channel override (env: SLACK_CHANNEL)")
	f.StringVar(&teamsWebhookFlag, "teams-webhook", getEnvString("TEAMS_WEBHOOK", ""), "Microsoft Teams webhook URL (env: TEAMS_WEBHOOK)")

	// Snapshot testing flags
	f.BoolVar(&updateSnapshotsFlag, "update-snapshots", false, "Update snapshot files instead of comparing")

	// Security flags
	f.BoolVar(&allowShellFlag, "allow-shell", false, "Allow shell command execution (>>>shell blocks and hooks)")
	f.BoolVar(&allowDBFlag, "allow-db", false, "Allow database assertions (>>>db blocks)")

	// Coverage flags
	f.BoolVar(&coverageFlag, "coverage", false, "Generate API coverage report against OpenAPI spec")
	f.StringVar(&openapiFlag, "openapi", "", "Path to OpenAPI spec file for coverage analysis")

	// History flags
	f.StringVar(&historyDBFlag, "history-db", getEnvString("HITSPEC_HISTORY_DB", ""), "Path to history database (default: ~/.hitspec/history.db) (env: HITSPEC_HISTORY_DB)")
	f.BoolVar(&noHistoryFlag, "no-history", getEnvBool("HITSPEC_NO_HISTORY", false), "Disable run history recording (env: HITSPEC_NO_HISTORY)")
}

func init() { registerRunFlags(runCmd.Flags()) }

// runOverrides collects what the user pinned on the command line, under the
// HITSPEC_* variables, as runconfig overrides: flag > env var; hitspec.yaml and
// the built-in default fill in whatever is left (runconfig.Build).
func runOverrides(flags *pflag.FlagSet, environment string) (runconfig.Overrides, error) {
	o := runconfig.Overrides{
		Environment: environment,
		EnvFile:     envFileFlag,
		Proxy:       proxyFlag,
		Concurrency: concurrencyFlag,
	}
	if timeoutFlag != "" {
		timeout, err := time.ParseDuration(timeoutFlag)
		if err != nil {
			return o, fmt.Errorf("invalid timeout value %q: %w (use format like 30s, 1m, 500ms)", timeoutFlag, err)
		}
		o.Timeout = timeout
	}
	// A bool flag overrides hitspec.yaml when it is true or was passed
	// explicitly (so --parallel=false can beat parallel: true).
	if insecureFlag || flags.Changed("insecure") {
		validate := !insecureFlag
		o.ValidateSSL = &validate
	}
	if parallelFlag || flags.Changed("parallel") {
		o.Parallel = &parallelFlag
	}
	if bailFlag || flags.Changed("bail") {
		o.Bail = &bailFlag
	}
	fromEnv, err := runconfig.FromEnv()
	if err != nil {
		return o, err
	}
	return o.Over(fromEnv), nil
}

// buildRunnerConfig turns hitspec.yaml plus the run flags into the runner
// configuration for `hitspec run`.
func buildRunnerConfig(flags *pflag.FlagSet, fileConfig *config.Config, environment string, tagsFilter []string) (*runner.Config, []http.ClientOption, error) {
	overrides, err := runOverrides(flags, environment)
	if err != nil {
		return nil, nil, err
	}
	cfg, clientOpts := runconfig.Build(fileConfig, overrides)
	cfg.Verbose = verboseFlag > 0
	cfg.NameFilter = nameFlag
	cfg.TagsFilter = tagsFilter
	cfg.UpdateSnapshots = updateSnapshotsFlag
	cfg.AllowShell = allowShellFlag
	cfg.AllowDB = allowDBFlag
	return cfg, clientOpts, nil
}

// resolveEnvironment picks the environment: the --env flag (whose default is
// HITSPEC_ENV), then hitspec.yaml defaultEnvironment, then "dev". explicit is
// true when the user chose it (flag or HITSPEC_ENV) rather than a default.
func resolveEnvironment(flagValue string, cfg *config.Config) (name string, explicit bool) {
	return runconfig.ResolveEnvironment(flagValue, cfg)
}

// Environment variable helpers
func getEnvString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		return val == "true" || val == "1" || val == "yes"
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

// joinEnvNames returns a sorted, comma-separated list of defined
// environment names for use in warning messages.
func joinEnvNames(envs map[string]map[string]any) string {
	names := make([]string, 0, len(envs))
	for name := range envs {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
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
	// Validate output format
	validFormats := map[string]bool{"console": true, "json": true, "junit": true, "tap": true, "html": true}
	if !validFormats[strings.ToLower(outputFlag)] {
		return fmt.Errorf("unknown output format %q (valid formats: console, json, junit, tap, html)", outputFlag)
	}

	// Set up notification manager
	var notifyManager *notify.Manager
	if notifyFlag != "" {
		var notifiers []notify.Notifier
		notifyOn := notify.NotifyOn(notifyOnFlag)

		for _, service := range strings.Split(notifyFlag, ",") {
			service = strings.TrimSpace(service)
			switch strings.ToLower(service) {
			case "slack":
				if slackWebhookFlag == "" {
					return fmt.Errorf("--slack-webhook is required when using --notify slack")
				}
				slackOpts := []notify.SlackOption{}
				if slackChannelFlag != "" {
					slackOpts = append(slackOpts, notify.WithSlackChannel(slackChannelFlag))
				}
				notifiers = append(notifiers, notify.NewSlackNotifier(slackWebhookFlag, slackOpts...))

			case "teams":
				if teamsWebhookFlag == "" {
					return fmt.Errorf("--teams-webhook is required when using --notify teams")
				}
				notifiers = append(notifiers, notify.NewTeamsNotifier(teamsWebhookFlag))
			}
		}

		if len(notifiers) > 0 {
			notifyManager = notify.NewManager(notifyOn, notifiers...)
		}
	}

	files, err := collectFiles(args)
	if err != nil {
		// Root's Execute wrapper prints this to stderr once; avoid the
		// double "Error:" line that formatter.FormatError + return caused.
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no .http or .hitspec files found in %v", args)
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
	fileConfig, err := config.LoadConfig(configFlag)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return newExitError(ExitConfigError, err.Error())
	}
	// Resolve the environment: --env flag > HITSPEC_ENV (the flag's default)
	// > hitspec.yaml defaultEnvironment > "dev". envFlag is left untouched so
	// repeated in-process invocations don't mistake a resolved default for an
	// explicit choice.
	environment, envExplicit := resolveEnvironment(envFlag, fileConfig)

	// Warn if the explicitly chosen environment isn't defined in the config.
	// Only check when the user picked it (flag or HITSPEC_ENV), so an implicit
	// default doesn't trigger a warning on every run. Backward compatible:
	// this is only a stderr warning, never a hard failure.
	if envExplicit && len(fileConfig.Environments) > 0 {
		if _, ok := fileConfig.Environments[environment]; !ok {
			fmt.Fprintf(os.Stderr, "Warning: environment %q is not defined in hitspec.yaml (defined: %s)\n",
				environment, joinEnvNames(fileConfig.Environments))
		}
	}

	cfg, clientOpts, err := buildRunnerConfig(cmd.Flags(), fileConfig, environment, tagsFilter)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// If stress mode is enabled, delegate to stress runner
	if stressFlag {
		return runStressMode(cmd, files, fileConfig, environment, clientOpts)
	}

	// Open history store unless disabled
	if !noHistoryFlag {
		dbPath := historyDBFlag
		if dbPath == "" {
			home, err := os.UserHomeDir()
			if err == nil {
				dir := filepath.Join(home, ".hitspec")
				_ = os.MkdirAll(dir, 0o755)
				dbPath = filepath.Join(dir, "history.db")
			}
		}
		if dbPath != "" {
			// Ensure parent directory exists
			_ = os.MkdirAll(filepath.Dir(dbPath), 0o755)
			store, err := history.NewStore(dbPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to open history db: %v\n", err)
			} else {
				cfg.HistoryStore = store
				defer store.Close()
			}
		}
	}

	r := runner.NewRunner(cfg)
	// Flush any in-flight history writes before the deferred store.Close() runs.
	// (defers are LIFO, so this runs before the store.Close registered above.)
	defer r.WaitHistory()

	filtersSet := nameFlag != "" || len(tagsFilter) > 0
	stderr := cmd.ErrOrStderr()

	// runTests runs every file once and returns the accounting.
	var allRunResults []*runner.RunResult
	runTests := func(formatter Formatter, reporting bool) runTotals {
		var t runTotals
		allRunResults = nil
		startTime := time.Now()

		for _, file := range files {
			if dryRunFlag {
				fmt.Fprintf(cmd.OutOrStdout(), "Would run: %s\n", file)
				continue
			}

			result, err := r.RunFileContext(ctx, file)
			if err != nil {
				// A file that cannot be loaded (syntax error, duplicate
				// @name, @depends cycle) fails the run; the error goes to
				// stderr so stdout stays a clean report stream.
				t.BrokenFiles++
				fmt.Fprintf(stderr, "Error: %s\n", brokenFileMessage(file, err))
				if reporting {
					if strings.EqualFold(outputFlag, "console") {
						// Console report only carries the error when it goes
						// to a file; on stdout it would duplicate stderr.
						if outputFileFlag != "" {
							formatter.FormatError(err)
						}
					} else {
						formatter.FormatResult(brokenFileResult(file, err))
					}
				}
				if cfg.Bail {
					break
				}
				continue
			}

			allRunResults = append(allRunResults, result)
			if reporting {
				formatter.FormatResult(result)
			}
			if quietFlag {
				writeFailureList(stderr, result)
			}
			t.Passed += result.Passed
			t.Failed += result.Failed
			t.Skipped += result.Skipped
			t.Matched += result.Matched

			if cfg.Bail && result.Failed > 0 {
				break
			}
		}

		t.Duration = time.Since(startTime)
		return t
	}

	// executeRun performs one full pass: open the report destination (fresh
	// per pass), run, flush, close, and report filters that matched nothing.
	executeRun := func(header bool) (runTotals, error) {
		formatter, closeReport, err := openReport(outputFlag, outputFileFlag, verboseFlag, noColorFlag, quietFlag)
		if err != nil {
			return runTotals{}, err
		}
		// --quiet drops the stdout report, but --output-file still gets it.
		reporting := !quietFlag || outputFileFlag != ""
		if header && reporting {
			formatter.FormatHeader(version)
		}

		t := runTests(formatter, reporting)

		// Flush output for formatters that accumulate results
		var flushErr error
		if flushable, ok := formatter.(Flushable); ok && reporting {
			if err := flushable.Flush(t.Duration); err != nil {
				flushErr = fmt.Errorf("error writing output: %w", err)
			}
		}
		if err := closeReport(); err != nil && flushErr == nil {
			flushErr = fmt.Errorf("error writing output: %w", err)
		}

		if t.BrokenFiles > 0 {
			fmt.Fprintln(stderr, brokenFilesSummary(t.BrokenFiles))
		} else if !dryRunFlag && filtersSet && t.Matched == 0 {
			fmt.Fprintln(stderr, noMatchMessage(nameFlag, tagsFilter))
		}
		return t, flushErr
	}

	// Run tests once
	totals, err := executeRun(true)
	if err != nil {
		return err
	}
	totalPassed, totalFailed, totalSkipped, totalDuration := totals.Passed, totals.Failed, totals.Skipped, totals.Duration

	// Generate coverage report if --coverage flag is set
	if coverageFlag {
		if openapiFlag == "" {
			return fmt.Errorf("--openapi flag is required when using --coverage")
		}

		analyzer := coverage.NewAnalyzer()
		if err := analyzer.LoadOpenAPI(openapiFlag); err != nil {
			return fmt.Errorf("failed to load OpenAPI spec: %w", err)
		}

		// Collect executed requests from all run results
		var executedRequests []coverage.ExecutedRequest
		for _, runResult := range allRunResults {
			for _, reqResult := range runResult.Results {
				if reqResult.Request != nil && !reqResult.Skipped {
					// Extract path from URL for matching against OpenAPI spec
					reqPath := reqResult.Request.URL
					if u, parseErr := url.Parse(reqResult.Request.URL); parseErr == nil {
						reqPath = u.Path
					}
					executedRequests = append(executedRequests, coverage.ExecutedRequest{
						Method: reqResult.Request.Method,
						Path:   reqPath,
					})
				}
			}
		}

		report := analyzer.Analyze(executedRequests)
		fmt.Fprint(cmd.OutOrStdout(), report.FormatConsole())
	}

	// Send notifications if configured
	if notifyManager != nil {
		summary := &notify.RunSummary{
			TotalFiles:   len(files),
			TotalTests:   totalPassed + totalFailed + totalSkipped + totals.BrokenFiles,
			PassedTests:  totalPassed,
			FailedTests:  totalFailed + totals.BrokenFiles,
			SkippedTests: totalSkipped,
			Duration:     totalDuration,
			Environment:  environment,
		}
		if err := notifyManager.Notify(summary); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to send notification: %v\n", err)
		}
	}

	// If watch mode is not enabled, exit normally
	if !watchFlag {
		return exitErrorFor(totals, filtersSet)
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
				fmt.Fprintf(stderr, "Error: failed to watch %s: %v\n", dir, err)
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

	// Banners go to stderr so stdout stays a clean stream of reports.
	fmt.Fprintf(stderr, "\nWatching for changes... (press Ctrl+C to stop)\n\n")

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
					fmt.Fprintf(stderr, "\n\nFile changed: %s\nRe-running tests...\n\n", event.Name)

					// Each pass opens a fresh report (truncating --output-file),
					// so JSON/JUnit output stays one valid document per run.
					if _, err := executeRun(false); err != nil {
						fmt.Fprintf(stderr, "Error: %v\n", err)
					}

					fmt.Fprintf(stderr, "\nWatching for changes... (press Ctrl+C to stop)\n")
				})
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(stderr, "Error: watcher error: %v\n", err)
		}
	}
}

func collectFiles(args []string) ([]string, error) {
	var files []string

	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("file or directory not found: %s", arg)
			}
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
			if !isHitspecFile(arg) {
				return nil, fmt.Errorf("%s is not a .http or .hitspec file", arg)
			}
			files = append(files, arg)
		}
	}

	return files, nil
}

func isHitspecFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".http" || ext == ".hitspec"
}

// newFormatter builds the output formatter for the given format, wiring the
// --output-file writer (outWriter) when set. Shared by the initial run and the
// watch-mode re-run so --output-file isn't dropped on re-runs.
func newFormatter(format string, outWriter io.Writer, verbose int, noColor, quiet bool) Formatter {
	switch strings.ToLower(format) {
	case "json":
		opts := []output.JSONOption{}
		if outWriter != nil {
			opts = append(opts, output.JSONWithWriter(outWriter))
		}
		return output.NewJSONFormatter(opts...)
	case "junit":
		opts := []output.JUnitOption{}
		if outWriter != nil {
			opts = append(opts, output.JUnitWithWriter(outWriter))
		}
		return output.NewJUnitFormatter(opts...)
	case "tap":
		opts := []output.TAPOption{}
		if outWriter != nil {
			opts = append(opts, output.TAPWithWriter(outWriter))
		}
		return output.NewTAPFormatter(opts...)
	case "html":
		opts := []output.HTMLOption{}
		if outWriter != nil {
			opts = append(opts, output.HTMLWithWriter(outWriter))
		}
		return output.NewHTMLFormatter(opts...)
	default: // "console"
		consoleOpts := []output.ConsoleOption{
			output.WithVerbose(verbose > 0),
			output.WithNoColor(noColor || quiet),
		}
		if outWriter != nil {
			consoleOpts = append(consoleOpts, output.WithWriter(outWriter))
		}
		return output.NewConsoleFormatter(consoleOpts...)
	}
}

// runStressMode executes stress tests using the stress runner
func runStressMode(cmd *cobra.Command, files []string, fileConfig *config.Config, environment string, clientOpts []http.ClientOption) error {
	// Build stress config
	cfg, err := buildStressConfig(cmd, fileConfig)
	if err != nil {
		return err
	}

	// Set up metrics exporters
	var metricsExporters []metrics.Exporter
	var metricsCollector *metrics.Collector

	if metricsFlag != "" {
		for _, format := range strings.Split(metricsFlag, ",") {
			format = strings.TrimSpace(format)
			switch strings.ToLower(format) {
			case "prometheus":
				promExporter := metrics.NewPrometheusExporter(
					metrics.WithPrometheusHTTP(metricsPortFlag),
				)
				metricsExporters = append(metricsExporters, promExporter)
				fmt.Fprintf(cmd.OutOrStdout(), "Prometheus metrics available at http://localhost:%d/metrics\n", metricsPortFlag)

			case "datadog":
				ddOpts := []metrics.DataDogOption{}
				if datadogAPIKeyFlag != "" {
					ddOpts = append(ddOpts, metrics.WithDataDogAPIKey(datadogAPIKeyFlag))
				}
				if datadogSiteFlag != "" {
					ddOpts = append(ddOpts, metrics.WithDataDogSite(datadogSiteFlag))
				}
				if datadogTagsFlag != "" {
					tags := strings.Split(datadogTagsFlag, ",")
					ddOpts = append(ddOpts, metrics.WithDataDogTags(tags))
				}
				ddExporter := metrics.NewDataDogExporter(ddOpts...)
				metricsExporters = append(metricsExporters, ddExporter)

			case "json":
				jsonOpts := []metrics.JSONOption{}
				if metricsFileFlag != "" {
					jsonOpts = append(jsonOpts, metrics.WithJSONFile(metricsFileFlag))
				} else {
					jsonOpts = append(jsonOpts, metrics.WithJSONWriter(os.Stdout))
				}
				jsonExporter := metrics.NewJSONExporter(jsonOpts...)
				metricsExporters = append(metricsExporters, jsonExporter)
			}
		}

		if len(metricsExporters) > 0 {
			metricsCollector = metrics.NewCollector(metricsExporters...)
		}
	}

	// Cleanup metrics exporters on exit
	defer func() {
		if metricsCollector != nil {
			_ = metricsCollector.Flush()
			_ = metricsCollector.Close()
		}
	}()

	// Create HTTP client
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
		stress.WithVerbose(verboseFlag > 0),
	)

	// Create runner with config environments for proper variable resolution
	runnerOpts := []stress.RunnerOption{
		stress.WithHTTPClient(client),
		stress.WithResolver(resolver),
		stress.WithReporter(reporter),
	}
	if environment != "" {
		runnerOpts = append(runnerOpts, stress.WithEnvironment(environment))
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

	// Keep Prometheus scrapes current while the test runs, not just after it.
	stopLive := publishPrometheusLive(ctx, stressRunner, metricsExporters)

	// Run the stress test
	result, err := stressRunner.Run(ctx)
	stopLive()
	if err != nil {
		return err
	}

	// Export metrics if collector is configured
	if metricsCollector != nil && result.Summary != nil {
		aggregate := stressResultToMetrics(result.Summary)
		if err := metricsCollector.Export(aggregate); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to export metrics: %v\n", err)
		}
	}

	// Output JSON if requested
	if stressJSONFlag {
		return reporter.JSONSummary(result.Summary, result.Thresholds)
	}

	// Exit with error code if thresholds failed
	if result.HasThresholdFailures() {
		os.Exit(ExitTestFailure)
	}

	return nil
}

// publishPrometheusLive pushes the runner's running totals to every Prometheus
// exporter once a second, so a scrape during the test sees the current numbers
// instead of zeros until the end. Other exporters (JSON, Datadog) are written
// once, after the run. The returned func stops the loop and waits for it.
func publishPrometheusLive(ctx context.Context, runner *stress.Runner, exporters []metrics.Exporter) (stop func()) {
	var live []*metrics.PrometheusExporter
	for _, e := range exporters {
		if p, ok := e.(*metrics.PrometheusExporter); ok {
			live = append(live, p)
		}
	}
	if len(live) == 0 {
		return func() {}
	}
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				aggregate := stressResultToMetrics(runner.GetSummary())
				for _, p := range live {
					_ = p.Export(aggregate)
				}
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}

// buildStressConfig builds stress configuration from flags and profile
func buildStressConfig(cmd *cobra.Command, fileConfig *config.Config) (*stress.Config, error) {
	cfg := stress.DefaultConfig()
	// A flag overrides the profile only when it was actually passed. Comparing
	// against the flag's default instead would let the default mask the
	// profile, and make an explicit "--rate 10" impossible to honor.
	flagSet := func(name string) bool { return cmd.Flags().Changed(name) }

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
	if flagSet("duration") {
		d, err := time.ParseDuration(stressDurationFlag)
		if err != nil {
			return nil, fmt.Errorf("invalid duration: %w", err)
		}
		cfg.Duration = d
	}

	if flagSet("rate") {
		cfg.Rate = stressRateFlag
	}

	if flagSet("vus") && stressVUsFlag > 0 {
		cfg.VUs = stressVUsFlag
		cfg.Mode = stress.VUMode
	}

	if flagSet("max-vus") {
		cfg.MaxVUs = stressMaxVUsFlag
	}

	if flagSet("think-time") {
		d, err := time.ParseDuration(stressThinkTimeFlag)
		if err != nil {
			return nil, fmt.Errorf("invalid think time: %w", err)
		}
		cfg.ThinkTime = d
	}

	if flagSet("ramp-up") {
		d, err := time.ParseDuration(stressRampUpFlag)
		if err != nil {
			return nil, fmt.Errorf("invalid ramp-up: %w", err)
		}
		cfg.RampUp = d
	}

	if flagSet("threshold") && stressThresholdFlag != "" {
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

// stressResultToMetrics converts stress test summary to metrics aggregate format
func stressResultToMetrics(summary *stress.Summary) *metrics.AggregateMetrics {
	aggregate := &metrics.AggregateMetrics{
		TotalRequests:   summary.TotalRequests,
		SuccessCount:    summary.SuccessCount,
		FailureCount:    summary.ErrorCount,
		TotalDurationMs: float64(summary.Duration.Milliseconds()),
		MinDurationMs:   float64(summary.Min.Milliseconds()),
		MaxDurationMs:   float64(summary.Max.Milliseconds()),
		AvgDurationMs:   float64(summary.Mean.Milliseconds()),
		P50DurationMs:   float64(summary.P50.Milliseconds()),
		P95DurationMs:   float64(summary.P95.Milliseconds()),
		P99DurationMs:   float64(summary.P99.Milliseconds()),
		StatusCodes:     make(map[int]int64),
		ByTest:          make(map[string]*metrics.TestAggregate),
	}

	// Copy per-request breakdown as test aggregates
	for name, reqSummary := range summary.RequestBreakdown {
		aggregate.ByTest[name] = &metrics.TestAggregate{
			Name:          name,
			TotalRequests: reqSummary.Total,
			SuccessCount:  reqSummary.Success,
			FailureCount:  reqSummary.Errors,
			AvgDurationMs: float64(reqSummary.Mean.Milliseconds()),
			MinDurationMs: float64(reqSummary.P50.Milliseconds()), // Use P50 as approx min
			MaxDurationMs: float64(reqSummary.P99.Milliseconds()), // Use P99 as approx max
		}
	}

	return aggregate
}
