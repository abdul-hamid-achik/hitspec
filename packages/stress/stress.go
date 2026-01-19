package stress

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/http"
)

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

	// Parsed requests
	file        *parser.File
	baseDir     string
	requests    []*parser.Request
	setupReqs   []*parser.Request
	teardownReqs []*parser.Request
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

// LoadFile parses and loads requests from a file
func (r *Runner) LoadFile(path string) error {
	file, err := parser.ParseFile(path)
	if err != nil {
		return fmt.Errorf("parsing file: %w", err)
	}

	r.file = file
	r.baseDir = filepath.Dir(path)

	// Load dotenv file if specified
	if r.envFile != "" {
		if err := r.resolver.LoadDotEnv(r.envFile); err != nil {
			r.reporter.Info("warning: failed to load env file: %v", err)
		}
	}

	// Load environment variables - pass config environments for proper resolution
	environment, err := env.LoadEnvironment(r.baseDir, r.envName, r.configEnvs)
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

		if cfg.Setup {
			r.setupReqs = append(r.setupReqs, req)
			continue
		}

		if cfg.Teardown {
			r.teardownReqs = append(r.teardownReqs, req)
			continue
		}

		r.requests = append(r.requests, req)
		r.scheduler.AddRequest(len(r.requests)-1, req.Name, cfg)
	}

	if len(r.requests) == 0 {
		return fmt.Errorf("no requests found in file (all may be skipped or marked as setup/teardown)")
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
	filename := ""
	if r.file != nil {
		filename = r.file.Path
	}
	r.reporter.Header("", filename, r.config)

	// Run setup requests
	if len(r.setupReqs) > 0 {
		r.reporter.Info("Running %d setup request(s)...", len(r.setupReqs))
		for _, req := range r.setupReqs {
			if err := r.executeRequest(ctx, req); err != nil {
				return nil, fmt.Errorf("setup request %q failed: %w", req.Name, err)
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
		for _, req := range r.teardownReqs {
			if err := r.executeRequest(teardownCtx, req); err != nil {
				r.reporter.Error("teardown request %q failed: %v", req.Name, err)
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

// executeScheduledRequest executes a scheduled request and records metrics
func (r *Runner) executeScheduledRequest(ctx context.Context, sched *ScheduledRequest) error {
	if sched.Index >= len(r.requests) {
		return nil
	}

	req := r.requests[sched.Index]
	start := time.Now()

	// Build HTTP request
	httpReq := http.BuildRequestFromASTWithBaseDir(req, r.resolver.Resolve, r.baseDir)

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
func (r *Runner) executeRequest(ctx context.Context, req *parser.Request) error {
	httpReq := http.BuildRequestFromASTWithBaseDir(req, r.resolver.Resolve, r.baseDir)

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return err
	}

	if !resp.IsSuccess() {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
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
