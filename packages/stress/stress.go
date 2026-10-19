package stress

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/assertions"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/capture"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

// requestWithBaseDir pairs a request with its base directory for relative path resolution
type requestWithBaseDir struct {
	request *parser.Request
	baseDir string
}

// Runner executes stress tests
type Runner struct {
	config    *Config
	client    *http.Client
	resolver  *env.Resolver
	scheduler *Scheduler
	metrics   *Metrics
	reporter  *Reporter

	// Environment configuration
	envName    string
	envFile    string
	configEnvs map[string]map[string]any

	// Parsed requests with their base directories
	requests     []requestWithBaseDir
	setupReqs    []requestWithBaseDir
	teardownReqs []requestWithBaseDir

	// Track loaded files for header display
	loadedFiles []string

	// fileRoot, when set, confines `< file` bodies and multipart files to that
	// directory (the workspace of serve/studio), as runner.Config.FileRoot does.
	fileRoot string
	// refused counts requests the file check turned away; firstRefused keeps
	// the first reason so the summary can say why the run is all errors.
	refused      atomic.Int64
	firstRefused atomic.Pointer[string]
}

// RunnerOption configures the runner
type RunnerOption func(*Runner)

// WithHTTPClient sets the HTTP client
func WithHTTPClient(client *http.Client) RunnerOption {
	return func(r *Runner) {
		r.client = client
	}
}

// WithResolver sets the variable resolver
func WithResolver(resolver *env.Resolver) RunnerOption {
	return func(r *Runner) {
		r.resolver = resolver
	}
}

// WithFileRoot confines every local file a request reads (a `< file` body,
// multipart file parts) to root. A request that points outside is not sent: it
// counts as an error, and a setup request fails the run. Empty (the default)
// disables the check, which is what the CLI wants.
func WithFileRoot(root string) RunnerOption {
	return func(r *Runner) {
		r.fileRoot = root
	}
}

// checkFileRefs applies the FileRoot confinement to a request.
func (r *Runner) checkFileRefs(req requestWithBaseDir) error {
	if r.fileRoot == "" {
		return nil
	}
	err := http.CheckFileRefs(req.request, r.resolver.Resolve, req.baseDir, r.fileRoot)
	if err != nil {
		r.refused.Add(1)
		msg := fmt.Sprintf("%s: %v", req.request.Name, err)
		r.firstRefused.CompareAndSwap(nil, &msg)
	}
	return err
}

// fileRefWarnings reports the requests turned away by the FileRoot check.
func (r *Runner) fileRefWarnings() []string {
	n := r.refused.Load()
	if n == 0 {
		return nil
	}
	first := ""
	if p := r.firstRefused.Load(); p != nil {
		first = *p
	}
	return []string{fmt.Sprintf("%d request(s) were not sent because they read a file outside the workspace (first: %s)", n, first)}
}

// WithReporter sets the reporter
func WithReporter(reporter *Reporter) RunnerOption {
	return func(r *Runner) {
		r.reporter = reporter
	}
}

// WithEnvironment sets the environment name
func WithEnvironment(envName string) RunnerOption {
	return func(r *Runner) {
		r.envName = envName
	}
}

// WithEnvFile sets the .env file path
func WithEnvFile(envFile string) RunnerOption {
	return func(r *Runner) {
		r.envFile = envFile
	}
}

// WithConfigEnvironments sets the config environments for variable resolution
func WithConfigEnvironments(envs map[string]map[string]any) RunnerOption {
	return func(r *Runner) {
		r.configEnvs = envs
	}
}

// NewRunner creates a new stress test runner
func NewRunner(config *Config, opts ...RunnerOption) *Runner {
	r := &Runner{
		config:    config,
		metrics:   NewMetrics(),
		scheduler: NewScheduler(config),
	}

	for _, opt := range opts {
		opt(r)
	}

	// Create defaults if not provided
	if r.client == nil {
		r.client = http.NewClient()
	}

	if r.resolver == nil {
		r.resolver = env.NewResolver()
	}

	if r.reporter == nil {
		r.reporter = NewReporter()
	}

	return r
}

// LoadFiles loads requests from multiple files
func (r *Runner) LoadFiles(paths []string) error {
	for _, path := range paths {
		if err := r.loadFile(path); err != nil {
			return err
		}
	}
	if len(r.requests) == 0 {
		return fmt.Errorf("no requests found in files")
	}
	return nil
}

// LoadFile parses and loads requests from a single file
func (r *Runner) LoadFile(path string) error {
	if err := r.loadFile(path); err != nil {
		return err
	}
	if len(r.requests) == 0 {
		return fmt.Errorf("no requests found in file (all may be skipped or marked as setup/teardown)")
	}
	return nil
}

// loadFile is the internal method that loads requests from a file and appends to existing requests
func (r *Runner) loadFile(path string) error {
	file, err := parser.ParseFile(path)
	if err != nil {
		return fmt.Errorf("parsing file %s: %w", path, err)
	}

	baseDir := filepath.Dir(path)
	r.loadedFiles = append(r.loadedFiles, path)

	// Load dotenv file if specified (only once, on first file)
	if len(r.loadedFiles) == 1 && r.envFile != "" {
		if err := r.resolver.LoadDotEnv(r.envFile); err != nil {
			r.reporter.Info("warning: failed to load env file: %v", err)
		}
	}

	// Load environment variables - pass config environments for proper resolution
	environment, err := env.LoadEnvironment(baseDir, r.envName, r.configEnvs)
	if err != nil {
		// Non-fatal, just log it
		r.reporter.Info("warning: failed to load environment: %v", err)
	} else {
		r.resolver.SetVariables(environment.Variables)
	}

	// Set file variables
	for _, v := range file.Variables {
		r.resolver.SetVariable(v.Name, v.Value)
	}

	// Categorize requests
	for _, req := range file.Requests {
		cfg := r.getRequestConfig(req)

		if cfg.Skip {
			continue
		}

		reqWithDir := requestWithBaseDir{
			request: req,
			baseDir: baseDir,
		}

		if cfg.Setup {
			r.setupReqs = append(r.setupReqs, reqWithDir)
			continue
		}

		if cfg.Teardown {
			r.teardownReqs = append(r.teardownReqs, reqWithDir)
			continue
		}

		r.requests = append(r.requests, reqWithDir)
		r.scheduler.AddRequest(len(r.requests)-1, req.Name, cfg)
	}

	return nil
}

// getRequestConfig extracts stress test configuration from request metadata
func (r *Runner) getRequestConfig(req *parser.Request) *RequestConfig {
	cfg := DefaultRequestConfig()

	if req.Metadata == nil || req.Metadata.Stress == nil {
		return cfg
	}

	stress := req.Metadata.Stress
	if stress.Weight > 0 {
		cfg.Weight = stress.Weight
	}
	if stress.Think > 0 {
		cfg.Think = stress.Think
	}
	cfg.Skip = stress.Skip
	cfg.Setup = stress.Setup
	cfg.Teardown = stress.Teardown

	return cfg
}

// Run executes the stress test
func (r *Runner) Run(ctx context.Context) (*Result, error) {
	if err := r.config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Print header
	r.reporter.Header("", r.loadedFiles, r.config)

	// Run setup requests
	if len(r.setupReqs) > 0 {
		r.reporter.Info("Running %d setup request(s)...", len(r.setupReqs))
		for _, reqWithDir := range r.setupReqs {
			if err := r.executeRequest(ctx, reqWithDir); err != nil {
				return nil, fmt.Errorf("setup request %q failed: %w", reqWithDir.request.Name, err)
			}
		}
		r.reporter.Info("Setup complete.\n")
	}

	// Start metrics collection
	r.metrics.Start()

	// Create cancellable context
	ctx, cancel := context.WithTimeout(ctx, r.config.Duration)
	defer cancel()

	// Start progress reporter
	progressDone := make(chan struct{})
	go r.progressLoop(ctx, progressDone)

	// Run the stress test
	if r.config.Mode == VUMode {
		r.runVUMode(ctx)
	} else {
		r.runRateMode(ctx)
	}

	// Stop metrics and progress
	r.metrics.Stop()
	close(progressDone)

	// Clear progress display
	r.reporter.ClearProgress()

	// Run teardown requests
	if len(r.teardownReqs) > 0 {
		r.reporter.Info("\nRunning %d teardown request(s)...", len(r.teardownReqs))
		teardownCtx := context.Background() // Use fresh context for teardown
		for _, reqWithDir := range r.teardownReqs {
			if err := r.executeRequest(teardownCtx, reqWithDir); err != nil {
				r.reporter.Error("teardown request %q failed: %v", reqWithDir.request.Name, err)
			}
		}
		r.reporter.Info("Teardown complete.")
	}

	// Get summary and evaluate thresholds
	summary := r.metrics.GetSummary()
	summary.Warnings = append(r.rateWarnings(summary), r.fileRefWarnings()...)
	var thresholdResults []ThresholdResult
	if r.config.Thresholds.HasThresholds() {
		thresholdResults = r.metrics.EvaluateThresholds(r.config.Thresholds)
	}

	// Print summary
	r.reporter.Summary(summary, thresholdResults)

	// Check if all thresholds passed
	passed := true
	for _, tr := range thresholdResults {
		if !tr.Passed {
			passed = false
			break
		}
	}

	return &Result{
		Summary:    summary,
		Thresholds: thresholdResults,
		Passed:     passed,
	}, nil
}

// expectedRequests is how many requests a rate-mode run should have started:
// the target rate for the whole window, less what the linear ramp-up skips.
func (c *Config) expectedRequests() float64 {
	if c.Mode != RateMode || c.Rate <= 0 {
		return 0
	}
	steady := c.Duration - c.RampUp
	return c.Rate * (steady.Seconds() + c.RampUp.Seconds()/2)
}

// rateWarnings flags a rate-mode run that fell more than 10% short of its
// target. When that happens the load was lower than asked and latencies
// understate what a client arriving on schedule would see (coordinated
// omission), so the numbers must not be read as "the server handled N req/s".
func (r *Runner) rateWarnings(s *Summary) []string {
	expected := r.config.expectedRequests()
	if expected <= 0 {
		return nil
	}
	started := float64(s.TotalRequests + s.IncompleteCount)
	if started >= expected*0.9 {
		return nil
	}
	secs := r.config.Duration.Seconds()
	return []string{fmt.Sprintf(
		"achieved %.1f req/s, %.0f%% below the %.0f req/s target: the server or the client (max VUs %d) could not keep up, so the latency figures understate the load a real client would have seen",
		started/secs, (1-started/expected)*100, r.config.Rate, r.config.MaxVUs)}
}

// runRateMode executes the stress test in rate mode
func (r *Runner) runRateMode(ctx context.Context) {
	var wg sync.WaitGroup
	startTime := time.Now()

	// Ramp-up ticker
	var rampUpTicker *time.Ticker
	if r.config.RampUp > 0 {
		rampUpTicker = time.NewTicker(100 * time.Millisecond)
		defer rampUpTicker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		default:
		}

		// Update rate for ramp-up
		if rampUpTicker != nil {
			select {
			case <-rampUpTicker.C:
				elapsed := time.Since(startTime)
				newRate := r.scheduler.GetCurrentRate(elapsed)
				r.scheduler.UpdateRate(newRate)
			default:
			}
		}

		// Wait for rate limiter
		if err := r.scheduler.Wait(ctx); err != nil {
			wg.Wait()
			return
		}

		// Select request
		scheduled := r.scheduler.SelectRequest()
		if scheduled == nil {
			continue
		}

		// Acquire semaphore
		if err := r.scheduler.Acquire(ctx); err != nil {
			wg.Wait()
			return
		}

		// Execute request
		wg.Add(1)
		go func(sched *ScheduledRequest) {
			defer wg.Done()
			defer r.scheduler.Release()

			_ = r.executeScheduledRequest(ctx, sched)
		}(scheduled)
	}
}

// runVUMode executes the stress test in virtual user mode
func (r *Runner) runVUMode(ctx context.Context) {
	executor := func(execCtx context.Context, sched *ScheduledRequest) error {
		return r.executeScheduledRequest(execCtx, sched)
	}

	pool := NewVUPool(r.scheduler, r.config, r.metrics, executor)
	pool.Start(ctx)

	// Ramp-up ticker
	if r.config.RampUp > 0 {
		rampUpTicker := time.NewTicker(100 * time.Millisecond)
		startTime := time.Now()

		go func() {
			defer rampUpTicker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-rampUpTicker.C:
					elapsed := time.Since(startTime)
					targetVUs := r.scheduler.GetCurrentVUs(elapsed)
					pool.Scale(targetVUs)
				}
			}
		}()
	}

	// Wait for context to be done
	<-ctx.Done()

	// Stop and wait for all VUs
	pool.Stop()
	pool.Wait()
}

// hasUnresolvedVariables checks if a request has any unresolved template variables
// Returns true and the list of unresolved variable names if any are found
func (r *Runner) hasUnresolvedVariables(req *parser.Request) (bool, []string) {
	// Check URL
	if vars := r.resolver.GetUnresolvedVariables(req.URL); len(vars) > 0 {
		return true, vars
	}
	// Check headers
	for _, h := range req.Headers {
		if vars := r.resolver.GetUnresolvedVariables(h.Value); len(vars) > 0 {
			return true, vars
		}
	}
	// Check body
	if req.Body != nil && req.Body.Raw != "" {
		if vars := r.resolver.GetUnresolvedVariables(req.Body.Raw); len(vars) > 0 {
			return true, vars
		}
	}
	return false, nil
}

// executeScheduledRequest executes a scheduled request and records metrics
func (r *Runner) executeScheduledRequest(ctx context.Context, sched *ScheduledRequest) error {
	if sched.Index >= len(r.requests) {
		return nil
	}

	reqWithDir := r.requests[sched.Index]

	// Skip requests with unresolved variables instead of sending literal {{var}} strings
	if hasUnresolved, vars := r.hasUnresolvedVariables(reqWithDir.request); hasUnresolved {
		err := fmt.Errorf("unresolved variables: %v", vars)
		r.metrics.Record(sched.Name, 0, err)
		return err
	}

	if err := r.checkFileRefs(reqWithDir); err != nil {
		r.metrics.Record(sched.Name, 0, err)
		return err
	}

	start := time.Now()

	// Build HTTP request using the request's own base directory
	httpReq := http.BuildRequestFromASTWithBaseDir(reqWithDir.request, r.resolver.Resolve, reqWithDir.baseDir)

	// Execute request
	resp, err := r.client.DoWithContext(ctx, httpReq)
	duration := time.Since(start)

	// Record metrics
	if err != nil {
		if ctx.Err() != nil {
			// The test window closed (or the run was interrupted) while this
			// request was in flight. That is the harness stopping, not the
			// server failing: count it apart from the errors.
			r.metrics.RecordIncomplete(sched.Name)
			return err
		}
		r.metrics.Record(sched.Name, duration, err)
		if isTimeout(err) {
			r.metrics.CountTimeout()
		}
		return err
	}

	r.metrics.RecordStatus(resp.StatusCode)

	// Assertions are evaluated on the response; failures are counted in their
	// own metric, apart from the errors. A non-2xx status the request itself
	// asserts on (expect status 404) is expected, so it is not an error.
	statusExpected := false
	if len(reqWithDir.request.Assertions) > 0 {
		toCheck := stressAssertions(reqWithDir.request.Assertions)
		results := assertions.EvaluateAllWithBaseDir(resp, toCheck, reqWithDir.baseDir,
			assertions.WithRequestName(reqWithDir.request.Name),
			assertions.WithFileRoot(r.fileRoot),
			assertions.WithResolver(r.resolver.Resolve))
		failed := false
		for i, res := range results {
			if !res.Passed {
				failed = true
			} else if toCheck[i].Subject == "status" {
				statusExpected = true
			}
		}
		if len(results) > 0 {
			r.metrics.RecordAssertions(sched.Name, failed)
		}
	}

	// Check if response indicates an error
	var recordErr error
	if !statusExpected && !resp.IsSuccess() {
		recordErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	r.metrics.Record(sched.Name, duration, recordErr)
	return recordErr
}

// stressAssertions drops the assertions that make no sense under load:
// snapshots would read and rewrite the same file from every worker.
func stressAssertions(in []*parser.Assertion) []*parser.Assertion {
	out := make([]*parser.Assertion, 0, len(in))
	for _, a := range in {
		if a.Operator == parser.OpSnapshot {
			continue
		}
		out = append(out, a)
	}
	return out
}

// isTimeout reports whether err is a client-side timeout (not the test window
// closing, which executeScheduledRequest handles first).
func isTimeout(err error) bool {
	var ne net.Error
	return errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout())
}

// executeRequest executes a single request (for setup/teardown)
func (r *Runner) executeRequest(ctx context.Context, reqWithDir requestWithBaseDir) error {
	if err := r.checkFileRefs(reqWithDir); err != nil {
		return err
	}
	httpReq := http.BuildRequestFromASTWithBaseDir(reqWithDir.request, r.resolver.Resolve, reqWithDir.baseDir)

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return err
	}

	if !resp.IsSuccess() {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Extract captures from setup responses so they can be used by subsequent requests
	if len(reqWithDir.request.Captures) > 0 {
		captures := capture.ExtractAll(resp, reqWithDir.request.Captures)
		for name, value := range captures {
			r.resolver.SetCapture(reqWithDir.request.Name, name, value)
		}
	}

	return nil
}

// progressLoop updates the progress display
func (r *Runner) progressLoop(ctx context.Context, done chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			stats := r.metrics.GetCurrentStats()
			r.reporter.Progress(stats, r.config.Duration)

			// Also record time series point
			point := r.metrics.Snapshot()
			r.metrics.AddTimePoint(point)
		}
	}
}

// Result holds the final result of a stress test
type Result struct {
	Summary    *Summary
	Thresholds []ThresholdResult
	Passed     bool
}

// GetCurrentStats returns current stress test statistics for external consumers
func (r *Runner) GetCurrentStats() CurrentStats {
	return r.metrics.GetCurrentStats()
}

// Duration returns how long the run is configured to last.
func (r *Runner) Duration() time.Duration {
	return r.config.Duration
}

// GetSummary returns the final metrics summary
func (r *Runner) GetSummary() *Summary {
	return r.metrics.GetSummary()
}

// HasThresholdFailures returns true if any thresholds failed
func (r *Result) HasThresholdFailures() bool {
	for _, tr := range r.Thresholds {
		if !tr.Passed {
			return true
		}
	}
	return false
}
