package runner

import (
	"fmt"
	"net/http"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

// waitForService polls a URL until it returns the expected status code or times out
func (r *Runner) waitForService(cfg *parser.WaitForConfig, resolver func(string) string) error {
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
	client := &http.Client{
		Timeout: 5 * time.Second, // Per-request timeout
	}

	var lastErr error
	var lastStatus int

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(interval)
			continue
		}
		lastStatus = resp.StatusCode
		resp.Body.Close()

		if resp.StatusCode == expectedStatus {
			if r.config.Verbose {
				fmt.Printf("Service %s is ready (status: %d)\n", url, resp.StatusCode)
			}
			return nil
		}

		time.Sleep(interval)
	}

	if lastErr != nil {
		return fmt.Errorf("service %s not ready after %v: %v", url, timeout, lastErr)
	}
	return fmt.Errorf("service %s not ready after %v: got status %d, expected %d",
		url, timeout, lastStatus, expectedStatus)
}
