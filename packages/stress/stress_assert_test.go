package stress

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeHTTP(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "t.http")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

func runStress(t *testing.T, cfg *Config, file string) (*Result, string) {
	t.Helper()
	var out bytes.Buffer
	runner := NewRunner(cfg, WithReporter(NewReporter(WithWriter(&out), WithNoProgress(true), WithNoColor(true))))
	require.NoError(t, runner.LoadFile(file))
	res, err := runner.Run(context.Background())
	require.NoError(t, err)
	return res, out.String()
}

func TestAssertionFailuresAreCountedApartFromErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false}`))
	}))
	defer srv.Close()
	file := writeHTTP(t, "### A\nGET "+srv.URL+"/a\n\n>>>\nexpect status 200\nexpect body.ok == true\n<<<\n")

	thr, err := ParseThresholds("errors<1%,assertions<0%")
	require.NoError(t, err)
	res, out := runStress(t, &Config{Mode: RateMode, Duration: 500 * time.Millisecond, Rate: 20, MaxVUs: 5, Thresholds: thr}, file)

	s := res.Summary
	require.Greater(t, s.TotalRequests, int64(0))
	assert.Zero(t, s.ErrorCount, "a 200 with a failed body assertion is not a transport error")
	assert.Equal(t, s.TotalRequests, s.AssertionChecks)
	assert.Equal(t, s.TotalRequests, s.AssertionFailures)
	assert.InDelta(t, 1.0, s.AssertionFailRate, 0.0001)
	assert.False(t, res.Passed, "assertions<0%% must fail the run")
	byName := map[string]bool{}
	for _, tr := range res.Thresholds {
		byName[tr.Name] = tr.Passed
	}
	assert.True(t, byName["error rate"])
	assert.False(t, byName["assertion failure rate"])
	assert.Contains(t, out, "Assertions:")
}

func TestPassingAssertionsAndExpectedNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	file := writeHTTP(t, "### Missing\nGET "+srv.URL+"/nope\n\n>>>\nexpect status 404\n<<<\n")

	res, _ := runStress(t, &Config{Mode: RateMode, Duration: 400 * time.Millisecond, Rate: 20, MaxVUs: 5}, file)
	assert.Zero(t, res.Summary.ErrorCount, "a 404 the request asserts on is expected")
	assert.Zero(t, res.Summary.AssertionFailures)
	assert.Equal(t, res.Summary.TotalRequests, res.Summary.AssertionChecks)
}

func TestInFlightAtEndIsIncompleteNotError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer srv.Close()
	file := writeHTTP(t, "### Slow\nGET "+srv.URL+"/slow\n")

	res, out := runStress(t, &Config{Mode: RateMode, Duration: 300 * time.Millisecond, Rate: 20, MaxVUs: 10}, file)
	assert.Greater(t, res.Summary.IncompleteCount, int64(0))
	assert.Zero(t, res.Summary.ErrorCount)
	assert.Zero(t, res.Summary.TotalRequests)
	assert.Contains(t, out, "Incomplete:")
}

func TestWarnsWhenTargetRateIsNotReached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
	}))
	defer srv.Close()
	file := writeHTTP(t, "### S\nGET "+srv.URL+"/s\n")

	// One slot and 150ms per request cannot reach 50 req/s.
	res, out := runStress(t, &Config{Mode: RateMode, Duration: time.Second, Rate: 50, MaxVUs: 1}, file)
	require.NotEmpty(t, res.Summary.Warnings)
	assert.Contains(t, res.Summary.Warnings[0], "below the 50 req/s target")
	assert.True(t, strings.Contains(out, "Warning:"))
}

func TestNoWarningWhenTargetIsMet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	file := writeHTTP(t, "### Fast\nGET "+srv.URL+"/f\n")

	res, _ := runStress(t, &Config{Mode: RateMode, Duration: time.Second, Rate: 20, MaxVUs: 10}, file)
	assert.Empty(t, res.Summary.Warnings, "started %d of ~20", res.Summary.TotalRequests)
}

func TestParseAssertionThreshold(t *testing.T) {
	thr, err := ParseThresholds("assertions<2%")
	require.NoError(t, err)
	assert.True(t, thr.AssertionFailRateSet)
	assert.InDelta(t, 0.02, thr.AssertionFailRate, 1e-9)
	assert.True(t, thr.HasThresholds())

	zero, err := ParseThresholds("assertions<0")
	require.NoError(t, err)
	assert.True(t, zero.AssertionFailRateSet)
	assert.True(t, zero.HasThresholds())

	_, err = ParseThresholds("assertions>1%")
	assert.Error(t, err)
}

// errors<0 (and errors<=0, errors<0%) mean "no request may error": a zero
// threshold is a threshold, not an unset one.
func TestZeroErrorThresholdIsExpressible(t *testing.T) {
	for _, expr := range []string{"errors<0", "errors<=0", "errors<0%", "errors<=0%"} {
		thr, err := ParseThresholds(expr)
		require.NoError(t, err, expr)
		assert.True(t, thr.HasThresholds(), expr)
		assert.True(t, thr.ErrorRateSet, expr)
		assert.Zero(t, thr.ErrorRate, expr)
	}
	_, err := ParseThresholds("errors<-1")
	assert.Error(t, err)

	// A server that fails one request in ten: errors<0 must fail the run,
	// errors<50% must not.
	var n atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1)%10 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	file := writeHTTP(t, "### A\nGET "+srv.URL+"/a\n")

	for expr, wantPass := range map[string]bool{"errors<0": false, "errors<50%": true} {
		thr, err := ParseThresholds(expr)
		require.NoError(t, err)
		n.Store(0)
		res, _ := runStress(t, &Config{Mode: VUMode, VUs: 1, MaxVUs: 1, Duration: 600 * time.Millisecond, Thresholds: thr}, file)
		require.Greater(t, res.Summary.ErrorCount, int64(0), "the server should have failed some requests")
		require.Len(t, res.Thresholds, 1, expr)
		assert.Equal(t, wantPass, res.Passed, expr)
	}
}

// The live counters (CurrentStats) expose assertion checks and failures too.
func TestCurrentStatsCarriesAssertionCounts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false}`))
	}))
	defer srv.Close()
	file := writeHTTP(t, "### A\nGET "+srv.URL+"/a\n\n>>>\nexpect body.ok == true\n<<<\n")
	runner := NewRunner(&Config{Mode: RateMode, Duration: 300 * time.Millisecond, Rate: 20, MaxVUs: 5},
		WithReporter(NewReporter(WithWriter(&bytes.Buffer{}), WithNoProgress(true))))
	require.NoError(t, runner.LoadFile(file))
	_, err := runner.Run(context.Background())
	require.NoError(t, err)
	stats := runner.GetCurrentStats()
	assert.Greater(t, stats.AssertionChecks, int64(0))
	assert.Equal(t, stats.AssertionChecks, stats.AssertionFailures)
}
