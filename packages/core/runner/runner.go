package runner

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/assertions"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/history"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/snapshot"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/sse"
)

const (
	// DefaultConcurrency is the default number of concurrent requests in parallel mode
	DefaultConcurrency = 5
	// DefaultRetryDelayMs is the default delay between retries in milliseconds
	DefaultRetryDelayMs = 1000
	// SkipReasonCancelled marks requests that never started because the run's
	// context ended first.
	SkipReasonCancelled = "cancelled"
)

type Runner struct {
	client    *http.Client
	resolver  *env.Resolver
	config    *Config
	historyWG sync.WaitGroup    // tracks in-flight history-record goroutines
	snapshots *snapshot.Manager // snapshot state of the current run (see RunFileContext)
}

type Config struct {
	Environment        string
	EnvFile            string
	Verbose            bool
	Timeout            time.Duration
	FollowRedirect     bool
	MaxRedirects       int           // redirect limit; <= 0 keeps the HTTP client default
	Retries            int           // default retries for a failed request; a request's @retry overrides it
	RetryDelay         time.Duration // default delay between retries; <= 0 means DefaultRetryDelayMs. @retryDelay overrides it
	Bail               bool
	NameFilter         string
	IndexFilter        *int // run only the request at this source index; nil = unset (studio TUI runs untitled requests)
	TagsFilter         []string
	Parallel           bool
	Concurrency        int
	ValidateSSL        bool
	Proxy              string
	DefaultHeaders     map[string]string
	ConfigEnvironments map[string]map[string]any
	UpdateSnapshots    bool                      // Update snapshots instead of comparing
	FileRoot           string                    // when set, `< file` bodies and multipart files must live under this directory (the workspace of serve/studio)
	AllowShell         bool                      // Allow shell command execution (>>>shell blocks and hooks)
	AllowDB            bool                      // Allow database assertions (>>>db blocks)
	HistoryStore       *history.Store            // Optional persistent history store
	OnProgress         func(event ProgressEvent) // Optional callback for per-request progress
	MaxResponseBytes   int64                     // Response-body bound: 0 = http default (100 MiB), < 0 = unlimited

	// HTTP client settings that have no per-request equivalent.
	CACert                 string // PEM CA bundle trusted in addition to the system roots
	ClientCert             string // PEM client certificate for mutual TLS (needs ClientKey)
	ClientKey              string // PEM private key for ClientCert
	HTTP1                  bool   // disable HTTP/2
	Cookies                bool   // keep a cookie jar for the runner's lifetime
	RedirectForwardHeaders bool   // keep user headers on cross-origin redirects
}

// ClientOptions returns the HTTP client options this configuration implies. The
// runner builds its own client from them, and callers that drive the HTTP layer
// directly (stress, contract) use the same list so they honor the same settings.
func (c *Config) ClientOptions() []http.ClientOption {
	opts := []http.ClientOption{}
	if c.Timeout > 0 {
		opts = append(opts, http.WithTimeout(c.Timeout))
	}
	opts = append(opts, http.WithFollowRedirects(c.FollowRedirect))
	if c.MaxRedirects > 0 {
		opts = append(opts, http.WithMaxRedirects(c.MaxRedirects))
	}
	opts = append(opts, http.WithValidateSSL(c.ValidateSSL))
	if c.Proxy != "" {
		opts = append(opts, http.WithProxy(c.Proxy))
	}
	if len(c.DefaultHeaders) > 0 {
		opts = append(opts, http.WithDefaultHeaders(c.DefaultHeaders))
	}
	if c.MaxResponseBytes != 0 {
		opts = append(opts, http.WithMaxBodyBytes(c.MaxResponseBytes))
	}
	if c.CACert != "" {
		opts = append(opts, http.WithCACert(c.CACert))
	}
	if c.ClientCert != "" || c.ClientKey != "" {
		opts = append(opts, http.WithClientCert(c.ClientCert, c.ClientKey))
	}
	if c.HTTP1 {
		opts = append(opts, http.WithHTTP2(false))
	}
	if c.Cookies {
		opts = append(opts, http.WithCookies(true))
	}
	if c.RedirectForwardHeaders {
		opts = append(opts, http.WithForwardRedirectHeaders(true))
	}
	return opts
}

func NewRunner(cfg *Config) *Runner {
	if cfg == nil {
		cfg = &Config{
			ValidateSSL: true, // Default to validating SSL
		}
	}

	clientOpts := cfg.ClientOptions()

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
	// Cancelled is set when the run's context ended before every request ran.
	// Results holds what finished; the requests that never started are listed
	// as skipped with SkipReason "cancelled", and a request that was in flight
	// carries the context error in Error.
	Cancelled bool
	// Matched counts the requests in the file that satisfied the NameFilter and
	// TagsFilter (all requests when neither is set), including ones later
	// skipped by @skip or @only. Zero across a whole run means the filters
	// selected nothing.
	Matched int
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
	// Warnings are non-fatal findings about the request: a capture that matched
	// nothing, a {{reference}} that was sent unresolved. They never change
	// Passed or the exit code.
	Warnings []string
	Error    error
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
	return r.RunFileContext(context.Background(), path)
}

// RunFileContext executes a file while propagating cancellation to HTTP calls.
func (r *Runner) RunFileContext(ctx context.Context, path string) (*RunResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
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

	// Each run owns its snapshot manager: concurrent runs (hitspec serve) must
	// not share update mode or the file cache through a process-wide global.
	r.snapshots = snapshot.NewManager(filepath.Dir(path), r.config.UpdateSnapshots)

	result, err := r.runRequests(ctx, file)
	if err != nil {
		return nil, err
	}

	// Record to persistent history in a background goroutine (non-blocking). The
	// goroutine is tracked so WaitHistory can flush it before the store is closed
	// (previously store.Close() raced the writer and lost rows).
	if r.config.HistoryStore != nil {
		r.historyWG.Add(1)
		go func() {
			defer r.historyWG.Done()
			r.recordHistory(result)
		}()
	}

	return result, nil
}

// WaitHistory blocks until all in-flight history-record goroutines finish. Call
// before closing the HistoryStore to avoid a close/write race that loses rows.
func (r *Runner) WaitHistory() {
	r.historyWG.Wait()
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
			if len(preview) > 65536 {
				preview = preview[:65536]
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
