package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

// waitForRequestTimeout bounds each poll.
const waitForRequestTimeout = 5 * time.Second

// waitForService polls a URL until it returns the expected status code, times
// out, or ctx ends. Each poll goes through the runner's own HTTP client, so it
// uses the run's proxy, CA/client certificates, TLS validation, redirect policy
// and default headers instead of a bare net/http client.
func (r *Runner) waitForService(ctx context.Context, cfg *parser.WaitForConfig, resolver func(string) string) error {
	if cfg == nil {
		return nil
	}

	url := resolver(cfg.URL)
	timeout := time.Duration(cfg.Timeout) * time.Millisecond
	interval := time.Duration(cfg.Interval) * time.Millisecond
	expectedStatus := cfg.Status

	if r.config.Verbose {
		fmt.Printf("Waiting for %s to return %d (timeout: %v, interval: %v)\n",
			url, expectedStatus, timeout, interval)
	}

	deadline := time.Now().Add(timeout)
	var lastErr error
	var lastStatus int

	for time.Now().Before(deadline) {
		if err := http.ValidateURL(url); err != nil {
			return fmt.Errorf("service %s: %w", url, err)
		}
		resp, err := r.client.DoWithContext(ctx, &http.Request{Method: "GET", URL: url, Timeout: waitForRequestTimeout})
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = err
			if !sleepContext(ctx, interval) {
				return ctx.Err()
			}
			continue
		}
		lastStatus = resp.StatusCode

		if resp.StatusCode == expectedStatus {
			if r.config.Verbose {
				fmt.Printf("Service %s is ready (status: %d)\n", url, resp.StatusCode)
			}
			return nil
		}

		if !sleepContext(ctx, interval) {
			return ctx.Err()
		}
	}

	if lastErr != nil {
		return fmt.Errorf("service %s not ready after %v: %w", url, timeout, lastErr)
	}
	return fmt.Errorf("service %s not ready after %v: got status %d, expected %d",
		url, timeout, lastStatus, expectedStatus)
}

// sleepContext waits for d or until ctx ends; it reports whether the full wait
// elapsed.
func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
