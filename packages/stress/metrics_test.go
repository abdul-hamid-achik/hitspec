package stress

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsRecord(t *testing.T) {
	m := NewMetrics()
	m.Start()

	// Record some successful requests
	m.Record("test1", 100*time.Millisecond, nil)
	m.Record("test1", 150*time.Millisecond, nil)
	m.Record("test2", 200*time.Millisecond, nil)

	// Record an error
	m.Record("test1", 50*time.Millisecond, errors.New("test error"))

	m.Stop()

	stats := m.GetCurrentStats()
	assert.Equal(t, int64(4), stats.Total)
	assert.Equal(t, int64(3), stats.Success)
	assert.Equal(t, int64(1), stats.Errors)
}

func TestMetricsRecordTimeout(t *testing.T) {
	m := NewMetrics()
	m.Start()

	m.Record("test1", 100*time.Millisecond, nil)
	m.RecordTimeout("test1")

	m.Stop()

	summary := m.GetSummary()
	assert.Equal(t, int64(2), summary.TotalRequests)
	assert.Equal(t, int64(1), summary.TimeoutCount)
	assert.Equal(t, int64(1), summary.ErrorCount) // Timeouts count as errors
}

func TestMetricsActiveVUs(t *testing.T) {
	m := NewMetrics()

	m.IncrementActiveVUs()
	m.IncrementActiveVUs()
	assert.Equal(t, int32(2), m.GetCurrentStats().ActiveVUs)

	m.DecrementActiveVUs()
	assert.Equal(t, int32(1), m.GetCurrentStats().ActiveVUs)

	m.SetActiveVUs(10)
	assert.Equal(t, int32(10), m.GetCurrentStats().ActiveVUs)
}

func TestMetricsSummary(t *testing.T) {
	m := NewMetrics()
	m.Start()

	// Record requests with known latencies
	for i := 0; i < 100; i++ {
		m.Record("test", time.Duration(i+1)*time.Millisecond, nil)
	}

	m.Stop()

	summary := m.GetSummary()
	assert.Equal(t, int64(100), summary.TotalRequests)
	assert.Equal(t, int64(100), summary.SuccessCount)
	assert.Equal(t, int64(0), summary.ErrorCount)
	assert.InDelta(t, 1.0, summary.SuccessRate, 0.001)
	assert.InDelta(t, 0.0, summary.ErrorRate, 0.001)

	// Check percentiles are reasonable
	assert.True(t, summary.P50 > 0)
	assert.True(t, summary.P95 > summary.P50)
	assert.True(t, summary.P99 >= summary.P95)
	assert.True(t, summary.Max >= summary.P99)
	assert.True(t, summary.Min > 0)
}

func TestMetricsPerRequestBreakdown(t *testing.T) {
	m := NewMetrics()
	m.Start()

	m.Record("create", 100*time.Millisecond, nil)
	m.Record("create", 110*time.Millisecond, nil)
	m.Record("read", 50*time.Millisecond, nil)
	m.Record("read", 60*time.Millisecond, nil)
	m.Record("read", 55*time.Millisecond, nil)

	m.Stop()

	summary := m.GetSummary()
	require.Len(t, summary.RequestBreakdown, 2)

	createStats := summary.RequestBreakdown["create"]
	require.NotNil(t, createStats)
	assert.Equal(t, int64(2), createStats.Total)
	assert.Equal(t, int64(2), createStats.Success)

	readStats := summary.RequestBreakdown["read"]
	require.NotNil(t, readStats)
	assert.Equal(t, int64(3), readStats.Total)
	assert.Equal(t, int64(3), readStats.Success)
}

func TestMetricsEvaluateThresholds(t *testing.T) {
	m := NewMetrics()
	m.Start()

	// Record requests with predictable latencies
	for i := 0; i < 100; i++ {
		m.Record("test", 10*time.Millisecond, nil)
	}
	// Record one error
	m.Record("test", 10*time.Millisecond, errors.New("error"))

	m.Stop()

	// Test passing thresholds
	thresholds := Thresholds{
		P95:       100 * time.Millisecond, // Should pass
		ErrorRate: 0.05,                   // Should pass (actual ~1%)
	}

	results := m.EvaluateThresholds(thresholds)
	require.Len(t, results, 2)

	for _, r := range results {
		assert.True(t, r.Passed, "threshold %s should pass", r.Name)
	}

	// Test failing thresholds
	thresholds = Thresholds{
		P95:       1 * time.Millisecond, // Should fail
		ErrorRate: 0.001,                // Should fail
	}

	results = m.EvaluateThresholds(thresholds)
	require.Len(t, results, 2)

	failCount := 0
	for _, r := range results {
		if !r.Passed {
			failCount++
		}
	}
	assert.Equal(t, 2, failCount)
}

func TestMetricsSnapshot(t *testing.T) {
	m := NewMetrics()
	m.Start()

	m.Record("test", 100*time.Millisecond, nil)
	m.IncrementActiveVUs()

	snapshot := m.Snapshot()

	assert.Equal(t, int64(1), snapshot.Requests)
	assert.Equal(t, int64(0), snapshot.Errors)
	assert.Equal(t, int32(1), snapshot.ActiveVUs)
}

func TestMetricsTimeSeries(t *testing.T) {
	m := NewMetrics()
	m.Start()

	m.Record("test", 100*time.Millisecond, nil)
	point1 := m.Snapshot()
	m.AddTimePoint(point1)

	m.Record("test", 100*time.Millisecond, nil)
	point2 := m.Snapshot()
	m.AddTimePoint(point2)

	summary := m.GetSummary()
	assert.Len(t, summary.TimeSeries, 2)
}
