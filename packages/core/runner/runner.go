package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/assertions"
	"github.com/abdul-hamid-achik/hitspec/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
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

	return r.runRequests(file)
}
