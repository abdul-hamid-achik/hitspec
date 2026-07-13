package runner

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/assertions"
	"github.com/abdul-hamid-achik/hitspec/packages/capture"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/http"
	"github.com/abdul-hamid-achik/hitspec/packages/sse"
)

// runRequests orchestrates execution of all requests in a parsed file,
// handling filtering, dependency ordering, parallel execution, and bail-on-failure.
func (r *Runner) runRequests(ctx context.Context, file *parser.File) (*RunResult, error) {
	start := time.Now()
	result := &RunResult{
		File: file.Path,
	}

	// Get base directory for file path resolution (multipart files)
	baseDir := filepath.Dir(file.Path)

	// IndexFilter: run a single request by its source index. The studio TUI uses
	// this to run an untitled request (which has no @name for the NameFilter to
	// match); without it, passing the synthesized "line N" display name as a name
	// filter matched nothing and the run was a silent no-op.
	if r.config.IndexFilter != nil {
		idx := *r.config.IndexFilter
		if idx < 0 || idx >= len(file.Requests) {
			return result, nil
		}
		single := *file
		single.Requests = []*parser.Request{file.Requests[idx]}
		file = &single
	}

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
				Name:        req.Name,
				Description: req.Description,
				Skipped:     true,
				SkipReason:  "filtered out",
			})
			result.Skipped++
			continue
		}

		if req.Metadata != nil && req.Metadata.Skip != "" {
			result.Results = append(result.Results, &RequestResult{
				Name:        req.Name,
				Description: req.Description,
				Skipped:     true,
				SkipReason:  req.Metadata.Skip,
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
		results := r.runParallel(ctx, filteredRequests, baseDir, file.Path)
		for _, reqResult := range results {
			result.Results = append(result.Results, reqResult)
			if reqResult.Passed {
				result.Passed++
			} else if reqResult.Skipped {
				result.Skipped++
			} else {
				result.Failed++
			}
		}
	} else {
		// Run sequentially with dependency checking
		executed := make(map[string]*RequestResult)

		for i, req := range filteredRequests {
			// Check dependencies - if any dependency failed OR didn't run (filtered
			// out, skipped, or nonexistent), skip this request. Previously a missing
			// dependency was ignored and the dependent ran anyway.
			if req.Metadata != nil && len(req.Metadata.Depends) > 0 {
				dependencyFailed := false
				for _, depName := range req.Metadata.Depends {
					depResult, exists := executed[depName]
					if !exists || !depResult.Passed {
						dependencyFailed = true
						break
					}
				}
				if dependencyFailed {
					result.Results = append(result.Results, &RequestResult{
						Name:        req.Name,
						Description: req.Description,
						Skipped:     true,
						SkipReason:  "dependency failed",
					})
					result.Skipped++
					continue
				}
			}

			// Notify progress: request starting
			if r.config.OnProgress != nil {
				r.config.OnProgress(ProgressEvent{
					RequestName: req.Name,
					Status:      "started",
					Index:       i,
					Total:       len(filteredRequests),
				})
			}

			reqResult := r.runRequest(ctx, req, baseDir, file.Path)
			result.Results = append(result.Results, reqResult)

			// Track executed request
			if req.Name != "" {
				executed[req.Name] = reqResult
			}

			// Notify progress: request completed
			if r.config.OnProgress != nil {
				r.config.OnProgress(ProgressEvent{
					RequestName: req.Name,
					Status:      "completed",
					Index:       i,
					Total:       len(filteredRequests),
					Result:      reqResult,
				})
			}

			if reqResult.Passed {
				result.Passed++
			} else if reqResult.Skipped {
				result.Skipped++
			} else {
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

// runParallel executes requests concurrently with a semaphore-based concurrency limit.
func (r *Runner) runParallel(ctx context.Context, requests []*parser.Request, baseDir string, filePath string) []*RequestResult {
	concurrency := r.config.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	total := len(requests)
	results := make([]*RequestResult, total)
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	for i, req := range requests {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore

		go func(idx int, request *parser.Request) {
			defer wg.Done()
			defer func() { <-sem }() // release semaphore

			if r.config.OnProgress != nil {
				r.config.OnProgress(ProgressEvent{
					RequestName: request.Name,
					Status:      "started",
					Index:       idx,
					Total:       total,
				})
			}

			results[idx] = r.runRequestWithRetry(ctx, request, baseDir, filePath, true)

			if r.config.OnProgress != nil {
				r.config.OnProgress(ProgressEvent{
					RequestName: request.Name,
					Status:      "completed",
					Index:       idx,
					Total:       total,
					Result:      results[idx],
				})
			}
		}(i, req)
	}

	wg.Wait()
	return results
}

func (r *Runner) runRequest(ctx context.Context, req *parser.Request, baseDir string, filePath string) *RequestResult {
	return r.runRequestWithRetry(ctx, req, baseDir, filePath, false)
}

// runRequestWithRetry executes a request with retry logic based on @retry,
// @retryDelay, and @retryOn annotations.
func (r *Runner) runRequestWithRetry(ctx context.Context, req *parser.Request, baseDir string, filePath string, parallel bool) *RequestResult {
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
		result = r.executeRequest(ctx, req, baseDir, filePath, parallel)

		if result.Passed || result.Skipped {
			return result
		}

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

		if attempt < maxRetries {
			timer := time.NewTimer(time.Duration(retryDelay) * time.Millisecond)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				result.Error = ctx.Err()
				return result
			case <-timer.C:
			}
		}
	}

	return result
}

// executeRequest performs a single HTTP request execution including condition checks,
// waitFor, hooks, assertions, DB assertions, shell commands, and captures.
func (r *Runner) executeRequest(ctx context.Context, req *parser.Request, baseDir string, filePath string, parallel bool) *RequestResult {
	result := &RequestResult{
		Name:        req.Name,
		Description: req.Description,
		Captures:    make(map[string]any),
	}

	// Check @if/@unless conditions
	if req.Metadata != nil && req.Metadata.Condition != nil {
		if !r.evaluateCondition(req.Metadata.Condition) {
			result.Skipped = true
			result.SkipReason = "condition not met"
			return result
		}
	}

	// Wait for service readiness if configured
	if req.Metadata != nil && req.Metadata.WaitFor != nil {
		if err := r.waitForService(req.Metadata.WaitFor, r.resolver.Resolve); err != nil {
			result.Error = err
			result.Passed = false
			return result
		}
	}

	// Execute pre-hooks
	if req.Metadata != nil && len(req.Metadata.PreHooks) > 0 {
		if err := r.executePreHooks(req.Metadata.PreHooks, baseDir, r.resolver.Resolve); err != nil {
			result.Error = err
			result.Passed = false
			return result
		}
	}

	// Defer post-hooks execution (always runs, even on failure)
	if req.Metadata != nil && len(req.Metadata.PostHooks) > 0 {
		defer func() {
			if err := r.executePostHooks(req.Metadata.PostHooks, baseDir, r.resolver.Resolve); err != nil {
				if result.Error == nil {
					result.Error = err
				}
			}
		}()
	}

	start := time.Now()

	httpReq := http.BuildRequestFromASTWithBaseDir(req, r.resolver.Resolve, baseDir)
	result.Request = httpReq

	resp, err := r.client.DoWithContext(ctx, httpReq)
	result.Duration = time.Since(start)

	if err != nil {
		result.Error = err
		result.Passed = false
		return result
	}
	result.Response = resp

	// Parse SSE events when response is text/event-stream
	if strings.HasPrefix(resp.ContentType(), "text/event-stream") && len(resp.Body) > 0 {
		sseClient := sse.NewClient("")
		events, _ := sseClient.ParseBody(resp.Body)
		result.SSEEvents = events
	}

	if len(req.Assertions) > 0 {
		result.Assertions = assertions.EvaluateAllWithBaseDir(resp, req.Assertions, baseDir,
			assertions.WithTestFile(filePath),
			assertions.WithRequestName(req.Name),
			assertions.WithResolver(r.resolver.Resolve))
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

	// Execute database assertions if configured
	if len(req.DBAssertions) > 0 && req.Metadata != nil && req.Metadata.DBConnection != "" {
		dbResults, err := r.executeDBAssertions(req.DBAssertions, req.Metadata.DBConnection, r.resolver.Resolve)
		if err != nil {
			result.Error = err
			result.Passed = false
		} else {
			result.DBAssertions = dbResults
			for _, dbResult := range dbResults {
				if !dbResult.Passed {
					result.Passed = false
					break
				}
			}
		}
	}

	// Execute shell commands if configured
	if len(req.ShellCommands) > 0 {
		shellResults, err := r.executeShellCommands(req.ShellCommands, baseDir, r.resolver.Resolve)
		if err != nil {
			result.Error = err
			result.Passed = false
		}
		result.ShellResults = shellResults
	}

	if len(req.Captures) > 0 {
		captures := capture.ExtractAll(resp, req.Captures)
		for name, value := range captures {
			result.Captures[name] = value
			if !parallel {
				r.resolver.SetCapture(req.Name, name, value)
			}
		}
	}

	return result
}
