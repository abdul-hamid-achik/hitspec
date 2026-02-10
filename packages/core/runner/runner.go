package runner

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/assertions"
	"github.com/abdul-hamid-achik/hitspec/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/history"
	"github.com/abdul-hamid-achik/hitspec/packages/http"
	"github.com/abdul-hamid-achik/hitspec/packages/snapshot"
	"github.com/abdul-hamid-achik/hitspec/packages/sse"
)

const (
	// DefaultConcurrency is the default number of concurrent requests in parallel mode
	DefaultConcurrency = 5
	// DefaultRetryDelayMs is the default delay between retries in milliseconds
	DefaultRetryDelayMs = 1000
)

type Runner struct {
	client   *http.Client
	resolver *env.Resolver
	config   *Config
}

type Config struct {
	Environment        string
	EnvFile            string
	Verbose            bool
	Timeout            time.Duration
	FollowRedirect     bool
	Bail               bool
	NameFilter         string
	TagsFilter         []string
	Parallel           bool
	Concurrency        int
	ValidateSSL        bool
	Proxy              string
	DefaultHeaders     map[string]string
	ConfigEnvironments map[string]map[string]any
	UpdateSnapshots    bool // Update snapshots instead of comparing
	AllowShell         bool // Allow shell command execution (>>>shell blocks and hooks)
	AllowDB            bool // Allow database assertions (>>>db blocks)
	HistoryStore       *history.Store // Optional persistent history store
	OnProgress         func(event ProgressEvent) // Optional callback for per-request progress
}

func NewRunner(cfg *Config) *Runner {
	if cfg == nil {
		cfg = &Config{
			ValidateSSL: true, // Default to validating SSL
		}
	}

	clientOpts := []http.ClientOption{}
	if cfg.Timeout > 0 {
		clientOpts = append(clientOpts, http.WithTimeout(cfg.Timeout))
	}
	clientOpts = append(clientOpts, http.WithFollowRedirects(cfg.FollowRedirect))
	clientOpts = append(clientOpts, http.WithValidateSSL(cfg.ValidateSSL))

	if cfg.Proxy != "" {
		clientOpts = append(clientOpts, http.WithProxy(cfg.Proxy))
	}

	if len(cfg.DefaultHeaders) > 0 {
		clientOpts = append(clientOpts, http.WithDefaultHeaders(cfg.DefaultHeaders))
	}

	resolver := env.NewResolver()
	// Set up warning function to print to stderr
	resolver.SetWarnFunc(func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
	})

	// Load dotenv file if specified
	if cfg.EnvFile != "" {
		if err := resolver.LoadDotEnv(cfg.EnvFile); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load env file: %v\n", err)
		}
	}

	return &Runner{
		client:   http.NewClient(clientOpts...),
		resolver: resolver,
		config:   cfg,
	}
}

type RunResult struct {
	File     string
	Results  []*RequestResult
	Duration time.Duration
	Passed   int
	Failed   int
	Skipped  int
}

type RequestResult struct {
	Name         string
	Description  string
	Passed       bool
	Skipped      bool
	SkipReason   string
	Duration     time.Duration
	Request      *http.Request
	Response     *http.Response
	Assertions   []*assertions.Result
	DBAssertions []*DBAssertionResult
	ShellResults []*ShellResult
	SSEEvents    []sse.Event
	Captures     map[string]any
	Error        error
}

// ProgressEvent is fired before and after each request execution.
type ProgressEvent struct {
	RequestName string         `json:"requestName"`
	Status      string         `json:"status"` // "started" or "completed"
	Index       int            `json:"index"`
	Total       int            `json:"total"`
	Result      *RequestResult `json:"-"` // Only set for "completed" events
}

func (r *Runner) RunFile(path string) (*RunResult, error) {
	file, err := parser.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	environment, err := env.LoadEnvironment(filepath.Dir(path), r.config.Environment, r.config.ConfigEnvironments)
	if err != nil {
		return nil, fmt.Errorf("failed to load environment %q: %w", r.config.Environment, err)
	}

	r.resolver.SetVariables(environment.Variables)

	for _, v := range file.Variables {
		r.resolver.SetVariable(v.Name, v.Value)
	}

	// Initialize snapshot manager for this file
	snapshotManager := snapshot.NewManager(filepath.Dir(path), r.config.UpdateSnapshots)
	snapshot.SetGlobalManager(snapshotManager)

	result, err := r.runRequests(file)
	if err != nil {
		return nil, err
	}

	// Record to persistent history in a background goroutine (non-blocking)
	if r.config.HistoryStore != nil {
		go r.recordHistory(result)
	}

	return result, nil
}

// recordHistory persists the run result to the history store.
// Errors are logged but do not propagate.
func (r *Runner) recordHistory(result *RunResult) {
	ctx := context.Background()
	store := r.config.HistoryStore

	runID, err := store.RecordRun(ctx, result.File, r.config.Environment)
	if err != nil {
		log.Printf("history: failed to record run: %v", err)
		return
	}

	for _, rr := range result.Results {
		method, url := "", ""
		statusCode := 0
		if rr.Request != nil {
			method = rr.Request.Method
			url = rr.Request.URL
		}
		if rr.Response != nil {
			statusCode = rr.Response.StatusCode
		}
		errMsg := ""
		if rr.Error != nil {
			errMsg = rr.Error.Error()
		}

		bodyPreview := ""
		if rr.Response != nil && len(rr.Response.Body) > 0 {
			preview := string(rr.Response.Body)
			if len(preview) > 512 {
				preview = preview[:512]
			}
			bodyPreview = preview
		}

		resultID, err := store.RecordResult(ctx, runID,
			rr.Name, method, url, statusCode,
			rr.Duration.Milliseconds(),
			rr.Passed, rr.Skipped,
			errMsg, rr.Description, bodyPreview,
		)
		if err != nil {
			log.Printf("history: failed to record result %q: %v", rr.Name, err)
			continue
		}

		if len(rr.Assertions) > 0 {
			records := make([]history.AssertionRecord, 0, len(rr.Assertions))
			for _, a := range rr.Assertions {
				records = append(records, history.AssertionRecord{
					Operator: a.Operator,
					Subject:  a.Subject,
					Expected: fmt.Sprintf("%v", a.Expected),
					Actual:   fmt.Sprintf("%v", a.Actual),
					Passed:   a.Passed,
					Message:  a.Message,
				})
			}
			if err := store.RecordAssertions(ctx, resultID, records); err != nil {
				log.Printf("history: failed to record assertions for %q: %v", rr.Name, err)
			}
		}
	}

	if err := store.FinishRun(ctx, runID, result.Duration,
		int64(result.Passed), int64(result.Failed), int64(result.Skipped),
		int64(result.Passed+result.Failed+result.Skipped)); err != nil {
		log.Printf("history: failed to finish run: %v", err)
	}
}
