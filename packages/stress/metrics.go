package stress

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

// Metrics collects and aggregates stress test metrics
type Metrics struct {
	mu sync.RWMutex

	// Counters
	totalRequests   atomic.Int64
	successRequests atomic.Int64
	errorRequests   atomic.Int64
	timeoutRequests atomic.Int64

	// Latency histogram (in microseconds for precision)
	histogram *hdrhistogram.Histogram

	// Per-request metrics
	requestMetrics map[string]*RequestMetrics

	// Time series for real-time display
	timeSeries    []TimePoint
	lastTimePoint time.Time

	// Test timing
	startTime time.Time
	endTime   time.Time

	// Active VUs
	activeVUs atomic.Int32
}

// RequestMetrics holds metrics for a specific request
type RequestMetrics struct {
	Name      string
	Total     atomic.Int64
	Success   atomic.Int64
	Errors    atomic.Int64
	Histogram *hdrhistogram.Histogram
	mu        sync.Mutex
}

// TimePoint represents a point in time for the time series
type TimePoint struct {
	Timestamp   time.Time
	Requests    int64
	Errors      int64
	P50         time.Duration
	P95         time.Duration
	P99         time.Duration
	ActiveVUs   int32
	RPS         float64
}

// NewMetrics creates a new Metrics collector
func NewMetrics() *Metrics {
	return &Metrics{
		// Histogram: 1us to 60s range, 3 significant digits
		histogram:      hdrhistogram.New(1, 60_000_000, 3),
		requestMetrics: make(map[string]*RequestMetrics),
		timeSeries:     make([]TimePoint, 0, 1000),
	}
}

// Start marks the beginning of the test
func (m *Metrics) Start() {
	m.startTime = time.Now()
	m.lastTimePoint = m.startTime
}

// Stop marks the end of the test
func (m *Metrics) Stop() {
	m.endTime = time.Now()
}

// Record records a request result
func (m *Metrics) Record(name string, duration time.Duration, err error) {
	m.totalRequests.Add(1)

	if err != nil {
		m.errorRequests.Add(1)
	} else {
		m.successRequests.Add(1)
	}

	// Record latency in microseconds
	latencyUs := duration.Microseconds()
	if latencyUs < 1 {
		latencyUs = 1
	}
	if latencyUs > 60_000_000 {
		latencyUs = 60_000_000
	}

	m.mu.Lock()
	_ = m.histogram.RecordValue(latencyUs)
	m.mu.Unlock()

	// Record per-request metrics
	if name != "" {
		m.recordRequestMetrics(name, duration, err)
	}
}

func (m *Metrics) recordRequestMetrics(name string, duration time.Duration, err error) {
	m.mu.Lock()
	rm, ok := m.requestMetrics[name]
	if !ok {
		rm = &RequestMetrics{
			Name:      name,
			Histogram: hdrhistogram.New(1, 60_000_000, 3),
		}
		m.requestMetrics[name] = rm
	}
	m.mu.Unlock()

	rm.Total.Add(1)
	if err != nil {
		rm.Errors.Add(1)
	} else {
		rm.Success.Add(1)
	}

	latencyUs := duration.Microseconds()
	if latencyUs < 1 {
		latencyUs = 1
	}
	if latencyUs > 60_000_000 {
		latencyUs = 60_000_000
	}

	rm.mu.Lock()
	_ = rm.Histogram.RecordValue(latencyUs)
	rm.mu.Unlock()
}

// RecordTimeout records a timeout
func (m *Metrics) RecordTimeout(name string) {
	m.totalRequests.Add(1)
	m.timeoutRequests.Add(1)
	m.errorRequests.Add(1)

	if name != "" {
		m.mu.Lock()
		rm, ok := m.requestMetrics[name]
		if !ok {
			rm = &RequestMetrics{
				Name:      name,
				Histogram: hdrhistogram.New(1, 60_000_000, 3),
			}
			m.requestMetrics[name] = rm
		}
		m.mu.Unlock()

		rm.Total.Add(1)
		rm.Errors.Add(1)
	}
}

// SetActiveVUs sets the current number of active virtual users
func (m *Metrics) SetActiveVUs(n int32) {
	m.activeVUs.Store(n)
}

// IncrementActiveVUs increments active VU count
func (m *Metrics) IncrementActiveVUs() {
	m.activeVUs.Add(1)
}

// DecrementActiveVUs decrements active VU count
func (m *Metrics) DecrementActiveVUs() {
	m.activeVUs.Add(-1)
}

// Snapshot captures current metrics for time series
func (m *Metrics) Snapshot() TimePoint {
	now := time.Now()

	m.mu.RLock()
	defer m.mu.RUnlock()

	elapsed := now.Sub(m.lastTimePoint).Seconds()
	if elapsed == 0 {
		elapsed = 1
	}

	total := m.totalRequests.Load()
	prevTotal := int64(0)
	if len(m.timeSeries) > 0 {
		prevTotal = m.timeSeries[len(m.timeSeries)-1].Requests
	}

	rps := float64(total-prevTotal) / elapsed

	point := TimePoint{
		Timestamp: now,
		Requests:  total,
		Errors:    m.errorRequests.Load(),
		P50:       time.Duration(m.histogram.ValueAtQuantile(50)) * time.Microsecond,
		P95:       time.Duration(m.histogram.ValueAtQuantile(95)) * time.Microsecond,
		P99:       time.Duration(m.histogram.ValueAtQuantile(99)) * time.Microsecond,
		ActiveVUs: m.activeVUs.Load(),
		RPS:       rps,
	}

	return point
}

// AddTimePoint adds a time point to the series
func (m *Metrics) AddTimePoint(point TimePoint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.timeSeries = append(m.timeSeries, point)
	m.lastTimePoint = point.Timestamp
}

// Summary returns the final metrics summary
type Summary struct {
	Duration      time.Duration
	TotalRequests int64
	SuccessCount  int64
	ErrorCount    int64
	TimeoutCount  int64

	// Calculated rates
	RPS          float64
	SuccessRate  float64
	ErrorRate    float64

	// Latency percentiles
	P50     time.Duration
	P95     time.Duration
	P99     time.Duration
	Min     time.Duration
	Max     time.Duration
	Mean    time.Duration
	StdDev  time.Duration

	// Per-request breakdown
	RequestBreakdown map[string]*RequestSummary

	// Time series data
	TimeSeries []TimePoint
}

// RequestSummary holds summary for a specific request
type RequestSummary struct {
	Name        string
	Total       int64
	Success     int64
	Errors      int64
	P50         time.Duration
	P95         time.Duration
	P99         time.Duration
	Mean        time.Duration
}

// GetSummary returns the metrics summary
func (m *Metrics) GetSummary() *Summary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	duration := m.endTime.Sub(m.startTime)
	if m.endTime.IsZero() {
		duration = time.Since(m.startTime)
	}

	total := m.totalRequests.Load()
	success := m.successRequests.Load()
	errors := m.errorRequests.Load()
	timeouts := m.timeoutRequests.Load()

	rps := float64(0)
	if duration.Seconds() > 0 {
		rps = float64(total) / duration.Seconds()
	}

	successRate := float64(0)
	errorRate := float64(0)
	if total > 0 {
		successRate = float64(success) / float64(total)
		errorRate = float64(errors) / float64(total)
	}

	summary := &Summary{
		Duration:      duration,
		TotalRequests: total,
		SuccessCount:  success,
		ErrorCount:    errors,
		TimeoutCount:  timeouts,
		RPS:           rps,
		SuccessRate:   successRate,
		ErrorRate:     errorRate,
		P50:           time.Duration(m.histogram.ValueAtQuantile(50)) * time.Microsecond,
		P95:           time.Duration(m.histogram.ValueAtQuantile(95)) * time.Microsecond,
		P99:           time.Duration(m.histogram.ValueAtQuantile(99)) * time.Microsecond,
		Min:           time.Duration(m.histogram.Min()) * time.Microsecond,
		Max:           time.Duration(m.histogram.Max()) * time.Microsecond,
		Mean:          time.Duration(m.histogram.Mean()) * time.Microsecond,
		StdDev:        time.Duration(m.histogram.StdDev()) * time.Microsecond,
		TimeSeries:    m.timeSeries,
	}

	// Per-request breakdown
	summary.RequestBreakdown = make(map[string]*RequestSummary)
	for name, rm := range m.requestMetrics {
		rm.mu.Lock()
		summary.RequestBreakdown[name] = &RequestSummary{
			Name:    name,
			Total:   rm.Total.Load(),
			Success: rm.Success.Load(),
			Errors:  rm.Errors.Load(),
			P50:     time.Duration(rm.Histogram.ValueAtQuantile(50)) * time.Microsecond,
			P95:     time.Duration(rm.Histogram.ValueAtQuantile(95)) * time.Microsecond,
			P99:     time.Duration(rm.Histogram.ValueAtQuantile(99)) * time.Microsecond,
			Mean:    time.Duration(rm.Histogram.Mean()) * time.Microsecond,
		}
		rm.mu.Unlock()
	}

	return summary
}

// CurrentStats returns current statistics for real-time display
type CurrentStats struct {
	Elapsed      time.Duration
	Total        int64
	Success      int64
	Errors       int64
	RPS          float64
	P50          time.Duration
	P95          time.Duration
	P99          time.Duration
	Max          time.Duration
	ActiveVUs    int32
	ErrorRate    float64
}

// GetCurrentStats returns current statistics
func (m *Metrics) GetCurrentStats() CurrentStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	elapsed := time.Since(m.startTime)
	total := m.totalRequests.Load()
	success := m.successRequests.Load()
	errors := m.errorRequests.Load()

	rps := float64(0)
	if elapsed.Seconds() > 0 {
		rps = float64(total) / elapsed.Seconds()
	}

	errorRate := float64(0)
	if total > 0 {
		errorRate = float64(errors) / float64(total)
	}

	return CurrentStats{
		Elapsed:   elapsed,
		Total:     total,
		Success:   success,
		Errors:    errors,
		RPS:       rps,
		P50:       time.Duration(m.histogram.ValueAtQuantile(50)) * time.Microsecond,
		P95:       time.Duration(m.histogram.ValueAtQuantile(95)) * time.Microsecond,
		P99:       time.Duration(m.histogram.ValueAtQuantile(99)) * time.Microsecond,
		Max:       time.Duration(m.histogram.Max()) * time.Microsecond,
		ActiveVUs: m.activeVUs.Load(),
		ErrorRate: errorRate,
	}
}

// EvaluateThresholds evaluates the thresholds against the summary
func (m *Metrics) EvaluateThresholds(t Thresholds) []ThresholdResult {
	summary := m.GetSummary()
	var results []ThresholdResult

	if t.P50 > 0 {
		passed := summary.P50 <= t.P50
		results = append(results, ThresholdResult{
			Name:     "p50",
			Passed:   passed,
			Expected: "< " + t.P50.String(),
			Actual:   summary.P50.String(),
		})
	}

	if t.P95 > 0 {
		passed := summary.P95 <= t.P95
		results = append(results, ThresholdResult{
			Name:     "p95",
			Passed:   passed,
			Expected: "< " + t.P95.String(),
			Actual:   summary.P95.String(),
		})
	}

	if t.P99 > 0 {
		passed := summary.P99 <= t.P99
		results = append(results, ThresholdResult{
			Name:     "p99",
			Passed:   passed,
			Expected: "< " + t.P99.String(),
			Actual:   summary.P99.String(),
		})
	}

	if t.MaxLatency > 0 {
		passed := summary.Max <= t.MaxLatency
		results = append(results, ThresholdResult{
			Name:     "max latency",
			Passed:   passed,
			Expected: "< " + t.MaxLatency.String(),
			Actual:   summary.Max.String(),
		})
	}

	if t.ErrorRate > 0 {
		passed := summary.ErrorRate <= t.ErrorRate
		results = append(results, ThresholdResult{
			Name:     "error rate",
			Passed:   passed,
			Expected: formatPercent(t.ErrorRate),
			Actual:   formatPercent(summary.ErrorRate),
		})
	}

	if t.MinRPS > 0 {
		passed := summary.RPS >= t.MinRPS
		results = append(results, ThresholdResult{
			Name:     "min RPS",
			Passed:   passed,
			Expected: "> " + formatFloat(t.MinRPS),
			Actual:   formatFloat(summary.RPS),
		})
	}

	return results
}

func formatPercent(f float64) string {
	return formatFloat(f*100) + "%"
}

func formatFloat(f float64) string {
	if f == float64(int(f)) {
		return strconv.Itoa(int(f))
	}
	return strconv.FormatFloat(f, 'f', 2, 64)
}
