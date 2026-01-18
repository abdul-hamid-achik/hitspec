package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/assertions"
	"github.com/abdul-hamid-achik/hitspec/packages/capture"
	"github.com/abdul-hamid-achik/hitspec/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/http"
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
	Environment    string
	Verbose        bool
	Timeout        time.Duration
	FollowRedirect bool
	Bail           bool
	NameFilter     string
	TagsFilter     []string
	Parallel       bool
	Concurrency    int
}

func NewRunner(cfg *Config) *Runner {
	if cfg == nil {
		cfg = &Config{}
	}

	clientOpts := []http.ClientOption{}
	if cfg.Timeout > 0 {
		clientOpts = append(clientOpts, http.WithTimeout(cfg.Timeout))
	}
	clientOpts = append(clientOpts, http.WithFollowRedirects(cfg.FollowRedirect))

	return &Runner{
		client:   http.NewClient(clientOpts...),
		resolver: env.NewResolver(),
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
	Name       string
	Passed     bool
	Skipped    bool
	SkipReason string
	Duration   time.Duration
	Request    *http.Request
	Response   *http.Response
	Assertions []*assertions.Result
	Captures   map[string]any
	Error      error
}

func (r *Runner) RunFile(path string) (*RunResult, error) {
	file, err := parser.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("parsing file: %w", err)
	}

	environment, err := env.LoadEnvironment(filepath.Dir(path), r.config.Environment)
	if err != nil {
		return nil, fmt.Errorf("loading environment: %w", err)
	}

	r.resolver.SetVariables(environment.Variables)

	for _, v := range file.Variables {
		r.resolver.SetVariable(v.Name, v.Value)
	}

	return r.runRequests(file)
}

func (r *Runner) runRequests(file *parser.File) (*RunResult, error) {
	start := time.Now()
	result := &RunResult{
		File: file.Path,
	}

	// Get base directory for file path resolution (multipart files)
	baseDir := filepath.Dir(file.Path)

	hasOnly := false
	for _, req := range file.Requests {
		if req.Metadata != nil && req.Metadata.Only {
			hasOnly = true
			break
		}
	}

	// Determine execution order using topological sort
	sortedRequests, err := r.topologicalSort(file.Requests)
	if err != nil {
		return nil, err
	}

	// Filter requests first
	var filteredRequests []*parser.Request
	for _, req := range sortedRequests {
		if !r.shouldRun(req, hasOnly) {
			result.Results = append(result.Results, &RequestResult{
				Name:       req.Name,
				Skipped:    true,
				SkipReason: "filtered out",
			})
			result.Skipped++
			continue
		}

		if req.Metadata != nil && req.Metadata.Skip != "" {
			result.Results = append(result.Results, &RequestResult{
				Name:       req.Name,
				Skipped:    true,
				SkipReason: req.Metadata.Skip,
			})
			result.Skipped++
			continue
		}

		filteredRequests = append(filteredRequests, req)
	}

	// Check if we can run in parallel (no dependencies between remaining requests)
	hasDependencies := false
	for _, req := range filteredRequests {
		if req.Metadata != nil && len(req.Metadata.Depends) > 0 {
			hasDependencies = true
			break
		}
	}

	// Run in parallel if configured and no dependencies
	if r.config.Parallel && !hasDependencies {
		results := r.runParallel(filteredRequests, baseDir)
		for _, reqResult := range results {
			result.Results = append(result.Results, reqResult)
			if reqResult.Passed {
				result.Passed++
			} else if !reqResult.Skipped {
				result.Failed++
			}
		}
	} else {
		// Run sequentially with dependency checking
		executed := make(map[string]*RequestResult)

		for _, req := range filteredRequests {
			// Check dependencies - if any dependency failed, skip this request
			if req.Metadata != nil && len(req.Metadata.Depends) > 0 {
				dependencyFailed := false
				for _, depName := range req.Metadata.Depends {
					if depResult, exists := executed[depName]; exists {
						if !depResult.Passed {
							dependencyFailed = true
							break
						}
					}
				}
				if dependencyFailed {
					result.Results = append(result.Results, &RequestResult{
						Name:       req.Name,
						Skipped:    true,
						SkipReason: "dependency failed",
					})
					result.Skipped++
					continue
				}
			}

			reqResult := r.runRequest(req, baseDir)
			result.Results = append(result.Results, reqResult)

			// Track executed request
			if req.Name != "" {
				executed[req.Name] = reqResult
			}

			if reqResult.Passed {
				result.Passed++
			} else if !reqResult.Skipped {
				result.Failed++
				if r.config.Bail {
					break
				}
			}
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

func (r *Runner) runParallel(requests []*parser.Request, baseDir string) []*RequestResult {
	concurrency := r.config.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	results := make([]*RequestResult, len(requests))
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	for i, req := range requests {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore

		go func(idx int, request *parser.Request) {
			defer wg.Done()
			defer func() { <-sem }() // release semaphore

			results[idx] = r.runRequestParallel(request, baseDir)
		}(i, req)
	}

	wg.Wait()
	return results
}

// topologicalSort returns requests in dependency-respecting order
func (r *Runner) topologicalSort(requests []*parser.Request) ([]*parser.Request, error) {
	// Build adjacency list and in-degree count
	inDegree := make(map[string]int)
	adjacency := make(map[string][]string)
	requestMap := make(map[string]*parser.Request)

	// Initialize all requests
	for _, req := range requests {
		name := req.Name
		if name == "" {
			name = fmt.Sprintf("__anon_%p", req)
		}
		inDegree[name] = 0
		requestMap[name] = req
	}

	// Build graph from dependencies
	for _, req := range requests {
		name := req.Name
		if name == "" {
			name = fmt.Sprintf("__anon_%p", req)
		}

		if req.Metadata != nil && len(req.Metadata.Depends) > 0 {
			for _, dep := range req.Metadata.Depends {
				if _, exists := requestMap[dep]; exists {
					adjacency[dep] = append(adjacency[dep], name)
					inDegree[name]++
				} else {
					fmt.Fprintf(os.Stderr, "warning: request %q depends on %q which does not exist\n", name, dep)
				}
			}
		}
	}

	// Kahn's algorithm for topological sort
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	var sortedNames []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		sortedNames = append(sortedNames, current)

		for _, neighbor := range adjacency[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// Check for cycles
	if len(sortedNames) != len(requests) {
		return nil, fmt.Errorf("circular dependency detected in requests")
	}

	// Map sorted names back to requests, preserving original order for requests
	// with same dependency level
	var sorted []*parser.Request
	processedNames := make(map[string]bool)

	// First, add sorted requests in dependency order
	for _, name := range sortedNames {
		if req, exists := requestMap[name]; exists && !processedNames[name] {
			sorted = append(sorted, req)
			processedNames[name] = true
		}
	}

	return sorted, nil
}

func (r *Runner) shouldRun(req *parser.Request, hasOnly bool) bool {
	if hasOnly && (req.Metadata == nil || !req.Metadata.Only) {
		return false
	}

	if r.config.NameFilter != "" {
		if req.Name == "" || !matchesPattern(req.Name, r.config.NameFilter) {
			return false
		}
	}

	if len(r.config.TagsFilter) > 0 {
		if !hasAnyTag(req.Tags, r.config.TagsFilter) {
			return false
		}
	}

	return true
}

func (r *Runner) runRequest(req *parser.Request, baseDir string) *RequestResult {
	return r.runRequestWithRetry(req, baseDir, false)
}

// runRequestParallel runs a request in parallel mode (no captures set in resolver)
func (r *Runner) runRequestParallel(req *parser.Request, baseDir string) *RequestResult {
	return r.runRequestWithRetry(req, baseDir, true)
}

// runRequestWithRetry executes a request with retry logic
func (r *Runner) runRequestWithRetry(req *parser.Request, baseDir string, parallel bool) *RequestResult {
	// Determine retry settings
	maxRetries := 0
	retryDelay := DefaultRetryDelayMs
	var retryOnStatuses []int

	if req.Metadata != nil {
		if req.Metadata.Retry > 0 {
			maxRetries = req.Metadata.Retry
		}
		if req.Metadata.RetryDelay > 0 {
			retryDelay = req.Metadata.RetryDelay
		}
		if len(req.Metadata.RetryOn) > 0 {
			retryOnStatuses = req.Metadata.RetryOn
		}
	}

	var result *RequestResult
	for attempt := 0; attempt <= maxRetries; attempt++ {
		result = r.executeRequest(req, baseDir, parallel)

		// If passed, no need to retry
		if result.Passed {
			return result
		}

		// Check if we should retry based on status code
		if len(retryOnStatuses) > 0 && result.Response != nil {
			shouldRetry := false
			for _, status := range retryOnStatuses {
				if result.Response.StatusCode == status {
					shouldRetry = true
					break
				}
			}
			if !shouldRetry {
				return result
			}
		}

		// If we have more retries left, wait and try again
		if attempt < maxRetries {
			time.Sleep(time.Duration(retryDelay) * time.Millisecond)
		}
	}

	return result
}

func (r *Runner) executeRequest(req *parser.Request, baseDir string, parallel bool) *RequestResult {
	result := &RequestResult{
		Name:     req.Name,
		Captures: make(map[string]any),
	}

	start := time.Now()

	httpReq := http.BuildRequestFromASTWithBaseDir(req, r.resolver.Resolve, baseDir)
	result.Request = httpReq

	resp, err := r.client.Do(httpReq)
	result.Duration = time.Since(start)

	if err != nil {
		result.Error = err
		result.Passed = false
		return result
	}
	result.Response = resp

	if len(req.Assertions) > 0 {
		result.Assertions = assertions.EvaluateAllWithBaseDir(resp, req.Assertions, baseDir)
		result.Passed = true
		for _, a := range result.Assertions {
			if !a.Passed {
				result.Passed = false
				break
			}
		}
	} else {
		result.Passed = resp.IsSuccess()
	}

	if len(req.Captures) > 0 {
		captures := capture.ExtractAll(resp, req.Captures)
		for name, value := range captures {
			result.Captures[name] = value
			// Skip setting captures in resolver when running in parallel mode
			// to avoid race conditions (parallel mode doesn't support dependencies anyway)
			if !parallel {
				r.resolver.SetCapture(req.Name, name, value)
			}
		}
	}

	return result
}

func matchesPattern(name, pattern string) bool {
	if pattern == "" {
		return true
	}

	if pattern[0] == '*' && pattern[len(pattern)-1] == '*' {
		substr := pattern[1 : len(pattern)-1]
		for i := 0; i <= len(name)-len(substr); i++ {
			if name[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}

	if pattern[0] == '*' {
		suffix := pattern[1:]
		return len(name) >= len(suffix) && name[len(name)-len(suffix):] == suffix
	}

	if pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(name) >= len(prefix) && name[:len(prefix)] == prefix
	}

	return name == pattern
}

func hasAnyTag(tags []string, filters []string) bool {
	for _, filter := range filters {
		for _, tag := range tags {
			if tag == filter {
				return true
			}
		}
	}
	return false
}
