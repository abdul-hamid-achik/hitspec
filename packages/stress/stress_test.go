package stress

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/env"
	hithttp "github.com/abdul-hamid-achik/hitspec/packages/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunnerIntegration(t *testing.T) {
	// Create a test server
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	// Create a test .http file
	tmpDir := t.TempDir()
	httpFile := filepath.Join(tmpDir, "test.http")
	content := `@baseUrl = ` + server.URL + `

### Health Check
# @name health

GET {{baseUrl}}/health

>>>
expect status 200
<<<
`
	err := os.WriteFile(httpFile, []byte(content), 0644)
	require.NoError(t, err)

	// Create config
	cfg := &Config{
		Mode:     RateMode,
		Duration: 2 * time.Second,
		Rate:     10,
		MaxVUs:   10,
	}

	// Create runner with silent reporter
	reporter := NewReporter(WithNoProgress(true), WithNoColor(true))
	client := hithttp.NewClient()
	resolver := env.NewResolver()

	runner := NewRunner(cfg,
		WithHTTPClient(client),
		WithResolver(resolver),
		WithReporter(reporter),
	)

	// Load file
	err = runner.LoadFile(httpFile)
	require.NoError(t, err)

	// Run test
	ctx := context.Background()
	result, err := runner.Run(ctx)
	require.NoError(t, err)

	// Verify results
	assert.True(t, result.Summary.TotalRequests > 0, "should have made requests")
	assert.True(t, result.Summary.SuccessCount > 0, "should have successful requests")
	assert.Equal(t, int64(0), result.Summary.ErrorCount, "should have no errors")
	assert.True(t, result.Passed, "test should pass")

	t.Logf("Made %d requests in %v (%.1f req/s)",
		result.Summary.TotalRequests,
		result.Summary.Duration,
		result.Summary.RPS)
}

func TestRunnerWithErrors(t *testing.T) {
	// Create a test server that fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "server error"}`))
	}))
	defer server.Close()

	// Create a test .http file
	tmpDir := t.TempDir()
	httpFile := filepath.Join(tmpDir, "test.http")
	content := `@baseUrl = ` + server.URL + `

### Failing Request
# @name fail

GET {{baseUrl}}/fail

>>>
expect status 200
<<<
`
	err := os.WriteFile(httpFile, []byte(content), 0644)
	require.NoError(t, err)

	cfg := &Config{
		Mode:     RateMode,
		Duration: 1 * time.Second,
		Rate:     5,
		MaxVUs:   5,
	}

	reporter := NewReporter(WithNoProgress(true), WithNoColor(true))
	runner := NewRunner(cfg, WithReporter(reporter))

	err = runner.LoadFile(httpFile)
	require.NoError(t, err)

	result, err := runner.Run(context.Background())
	require.NoError(t, err)

	// All requests should be errors (500 is not success)
	assert.True(t, result.Summary.ErrorCount > 0, "should have errors")
	assert.Equal(t, result.Summary.TotalRequests, result.Summary.ErrorCount, "all requests should be errors")
}

func TestRunnerWithThresholds(t *testing.T) {
	// Create a fast test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	httpFile := filepath.Join(tmpDir, "test.http")
	content := `@baseUrl = ` + server.URL + `

### Fast Request
GET {{baseUrl}}/fast
`
	err := os.WriteFile(httpFile, []byte(content), 0644)
	require.NoError(t, err)

	cfg := &Config{
		Mode:     RateMode,
		Duration: 1 * time.Second,
		Rate:     10,
		MaxVUs:   10,
		Thresholds: Thresholds{
			P95:       500 * time.Millisecond, // Should pass - local server is fast
			ErrorRate: 0.01,                   // Should pass - no errors expected
		},
	}

	reporter := NewReporter(WithNoProgress(true), WithNoColor(true))
	runner := NewRunner(cfg, WithReporter(reporter))

	err = runner.LoadFile(httpFile)
	require.NoError(t, err)

	result, err := runner.Run(context.Background())
	require.NoError(t, err)

	// Check thresholds were evaluated
	assert.Len(t, result.Thresholds, 2)
	assert.True(t, result.Passed, "thresholds should pass")

	for _, tr := range result.Thresholds {
		t.Logf("Threshold %s: passed=%v, expected=%s, actual=%s",
			tr.Name, tr.Passed, tr.Expected, tr.Actual)
	}
}

func TestRunnerVUMode(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		time.Sleep(10 * time.Millisecond) // Simulate some work
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	httpFile := filepath.Join(tmpDir, "test.http")
	content := `@baseUrl = ` + server.URL + `

### Request
GET {{baseUrl}}/
`
	err := os.WriteFile(httpFile, []byte(content), 0644)
	require.NoError(t, err)

	cfg := &Config{
		Mode:      VUMode,
		Duration:  1 * time.Second,
		VUs:       5,
		MaxVUs:    10,
		ThinkTime: 50 * time.Millisecond,
	}

	reporter := NewReporter(WithNoProgress(true), WithNoColor(true))
	runner := NewRunner(cfg, WithReporter(reporter))

	err = runner.LoadFile(httpFile)
	require.NoError(t, err)

	result, err := runner.Run(context.Background())
	require.NoError(t, err)

	assert.True(t, result.Summary.TotalRequests > 0, "should have made requests")
	t.Logf("VU mode: %d requests from %d VUs in %v",
		result.Summary.TotalRequests, cfg.VUs, result.Summary.Duration)
}

func TestRunnerWithWeightedRequests(t *testing.T) {
	var heavyCount int64
	var lightCount int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/heavy" {
			atomic.AddInt64(&heavyCount, 1)
		} else {
			atomic.AddInt64(&lightCount, 1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	httpFile := filepath.Join(tmpDir, "test.http")
	content := `@baseUrl = ` + server.URL + `

### Heavy Request (should be called more often)
# @name heavy
# @stress.weight 9

GET {{baseUrl}}/heavy

### Light Request
# @name light
# @stress.weight 1

GET {{baseUrl}}/light
`
	err := os.WriteFile(httpFile, []byte(content), 0644)
	require.NoError(t, err)

	cfg := &Config{
		Mode:     RateMode,
		Duration: 2 * time.Second,
		Rate:     50,
		MaxVUs:   20,
	}

	reporter := NewReporter(WithNoProgress(true), WithNoColor(true))
	runner := NewRunner(cfg, WithReporter(reporter))

	err = runner.LoadFile(httpFile)
	require.NoError(t, err)

	result, err := runner.Run(context.Background())
	require.NoError(t, err)

	// Heavy should be called roughly 9x more than light
	heavy := atomic.LoadInt64(&heavyCount)
	light := atomic.LoadInt64(&lightCount)
	t.Logf("Heavy: %d, Light: %d, Total: %d", heavy, light, result.Summary.TotalRequests)

	if heavy+light > 20 { // Need enough samples
		ratio := float64(heavy) / float64(light)
		assert.True(t, ratio > 5, "heavy should be called much more than light (ratio: %.1f)", ratio)
	}
}

func TestRunnerSetupTeardown(t *testing.T) {
	var setupCalled int32
	var teardownCalled int32
	var mainCalled int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/setup":
			atomic.StoreInt32(&setupCalled, 1)
		case "/teardown":
			atomic.StoreInt32(&teardownCalled, 1)
		case "/main":
			atomic.AddInt64(&mainCalled, 1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	httpFile := filepath.Join(tmpDir, "test.http")
	content := `@baseUrl = ` + server.URL + `

### Setup Request (run once before)
# @name setup
# @stress.setup

GET {{baseUrl}}/setup

### Main Request (run during test)
# @name main

GET {{baseUrl}}/main

### Teardown Request (run once after)
# @name teardown
# @stress.teardown

GET {{baseUrl}}/teardown
`
	err := os.WriteFile(httpFile, []byte(content), 0644)
	require.NoError(t, err)

	cfg := &Config{
		Mode:     RateMode,
		Duration: 500 * time.Millisecond,
		Rate:     10,
		MaxVUs:   5,
	}

	reporter := NewReporter(WithNoProgress(true), WithNoColor(true))
	runner := NewRunner(cfg, WithReporter(reporter))

	err = runner.LoadFile(httpFile)
	require.NoError(t, err)

	_, err = runner.Run(context.Background())
	require.NoError(t, err)

	setup := atomic.LoadInt32(&setupCalled) == 1
	teardown := atomic.LoadInt32(&teardownCalled) == 1
	main := atomic.LoadInt64(&mainCalled)
	assert.True(t, setup, "setup should be called")
	assert.True(t, teardown, "teardown should be called")
	assert.True(t, main > 0, "main should be called multiple times")
	t.Logf("Setup: %v, Teardown: %v, Main calls: %d", setup, teardown, main)
}

func TestRunnerGracefulShutdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	httpFile := filepath.Join(tmpDir, "test.http")
	content := `@baseUrl = ` + server.URL + `

### Slow Request
GET {{baseUrl}}/slow
`
	err := os.WriteFile(httpFile, []byte(content), 0644)
	require.NoError(t, err)

	cfg := &Config{
		Mode:     RateMode,
		Duration: 10 * time.Second, // Long duration
		Rate:     5,
		MaxVUs:   5,
	}

	reporter := NewReporter(WithNoProgress(true), WithNoColor(true))
	runner := NewRunner(cfg, WithReporter(reporter))

	err = runner.LoadFile(httpFile)
	require.NoError(t, err)

	// Cancel after 500ms
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	result, err := runner.Run(ctx)
	require.NoError(t, err)

	// Should have stopped early
	assert.True(t, result.Summary.Duration < 2*time.Second, "should have stopped early")
	t.Logf("Stopped after %v with %d requests", result.Summary.Duration, result.Summary.TotalRequests)
}
