package stress

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/capture"
	"github.com/abdul-hamid-achik/hitspec/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/http"
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
		config:   config,
		metrics:  NewMetrics(),
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

	start := time.Now()

	// Build HTTP request using the request's own base directory
	httpReq := http.BuildRequestFromASTWithBaseDir(reqWithDir.request, r.resolver.Resolve, reqWithDir.baseDir)

	// Execute request
	resp, err := r.client.Do(httpReq)
	duration := time.Since(start)

	// Record metrics
	if err != nil {
		if ctx.Err() != nil {
			// Context cancelled/timeout
			r.metrics.RecordTimeout(sched.Name)
		} else {
			r.metrics.Record(sched.Name, duration, err)
		}
		return err
	}

	// Check if response indicates an error
	var recordErr error
	if !resp.IsSuccess() {
		recordErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	r.metrics.Record(sched.Name, duration, recordErr)
	return recordErr
}

// executeRequest executes a single request (for setup/teardown)
func (r *Runner) executeRequest(ctx context.Context, reqWithDir requestWithBaseDir) error {
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

// HasThresholdFailures returns true if any thresholds failed
func (r *Result) HasThresholdFailures() bool {
	for _, tr := range r.Thresholds {
		if !tr.Passed {
			return true
		}
	}
	return false
}
