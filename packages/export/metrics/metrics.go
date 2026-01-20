// Package metrics provides metrics export functionality for hitspec test results.
package metrics

import (
	"time"
)

// Metric represents a single metric data point
type Metric struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Type      MetricType        `json:"type"`
}

// MetricType represents the type of metric
type MetricType string

const (
	Counter   MetricType = "counter"
	Gauge     MetricType = "gauge"
	Histogram MetricType = "histogram"
)

// TestMetrics represents metrics collected from test runs
type TestMetrics struct {
	TestName       string    `json:"test_name"`
	RequestMethod  string    `json:"request_method"`
	RequestURL     string    `json:"request_url"`
	StatusCode     int       `json:"status_code"`
	DurationMs     float64   `json:"duration_ms"`
	Passed         bool      `json:"passed"`
	AssertionCount int       `json:"assertion_count"`
	FailedCount    int       `json:"failed_count"`
	Timestamp      time.Time `json:"timestamp"`
}

// AggregateMetrics represents aggregated metrics from multiple test runs
type AggregateMetrics struct {
	TotalRequests   int64           `json:"total_requests"`
	SuccessCount    int64           `json:"success_count"`
	FailureCount    int64           `json:"failure_count"`
	TotalDurationMs float64         `json:"total_duration_ms"`
	MinDurationMs   float64         `json:"min_duration_ms"`
	MaxDurationMs   float64         `json:"max_duration_ms"`
	AvgDurationMs   float64         `json:"avg_duration_ms"`
	P50DurationMs   float64         `json:"p50_duration_ms"`
	P95DurationMs   float64         `json:"p95_duration_ms"`
	P99DurationMs   float64         `json:"p99_duration_ms"`
	StatusCodes     map[int]int64   `json:"status_codes"`
	ByTest          map[string]*TestAggregate `json:"by_test"`
}

// TestAggregate represents aggregated metrics for a single test
type TestAggregate struct {
	Name          string  `json:"name"`
	TotalRequests int64   `json:"total_requests"`
	SuccessCount  int64   `json:"success_count"`
	FailureCount  int64   `json:"failure_count"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
	MinDurationMs float64 `json:"min_duration_ms"`
	MaxDurationMs float64 `json:"max_duration_ms"`
}

// Exporter is the interface for metrics exporters
type Exporter interface {
	// Export exports metrics to the target destination
	Export(metrics *AggregateMetrics) error

	// ExportSingle exports a single test metric
	ExportSingle(metric *TestMetrics) error

	// Close closes the exporter and flushes any buffered data
	Close() error
}

// Collector collects metrics from test runs
type Collector struct {
	metrics    []*TestMetrics
	aggregate  *AggregateMetrics
	exporters  []Exporter
}

// NewCollector creates a new metrics collector
func NewCollector(exporters ...Exporter) *Collector {
	return &Collector{
		metrics:   make([]*TestMetrics, 0),
		exporters: exporters,
		aggregate: &AggregateMetrics{
			StatusCodes: make(map[int]int64),
			ByTest:      make(map[string]*TestAggregate),
		},
	}
}

// Record records a test metric
func (c *Collector) Record(m *TestMetrics) {
	c.metrics = append(c.metrics, m)
	c.updateAggregate(m)

	// Export to all exporters
	for _, exp := range c.exporters {
		_ = exp.ExportSingle(m)
	}
}

func (c *Collector) updateAggregate(m *TestMetrics) {
	c.aggregate.TotalRequests++
	c.aggregate.TotalDurationMs += m.DurationMs

	if m.Passed {
		c.aggregate.SuccessCount++
	} else {
		c.aggregate.FailureCount++
	}

	// Update min/max
	if c.aggregate.TotalRequests == 1 {
		c.aggregate.MinDurationMs = m.DurationMs
		c.aggregate.MaxDurationMs = m.DurationMs
	} else {
		if m.DurationMs < c.aggregate.MinDurationMs {
			c.aggregate.MinDurationMs = m.DurationMs
		}
		if m.DurationMs > c.aggregate.MaxDurationMs {
			c.aggregate.MaxDurationMs = m.DurationMs
		}
	}

	c.aggregate.AvgDurationMs = c.aggregate.TotalDurationMs / float64(c.aggregate.TotalRequests)

	// Update status codes
	c.aggregate.StatusCodes[m.StatusCode]++

	// Update per-test aggregates
	if _, ok := c.aggregate.ByTest[m.TestName]; !ok {
		c.aggregate.ByTest[m.TestName] = &TestAggregate{
			Name:          m.TestName,
			MinDurationMs: m.DurationMs,
			MaxDurationMs: m.DurationMs,
		}
	}

	ta := c.aggregate.ByTest[m.TestName]
	ta.TotalRequests++
	if m.Passed {
		ta.SuccessCount++
	} else {
		ta.FailureCount++
	}
	if m.DurationMs < ta.MinDurationMs {
		ta.MinDurationMs = m.DurationMs
	}
	if m.DurationMs > ta.MaxDurationMs {
		ta.MaxDurationMs = m.DurationMs
	}
	ta.AvgDurationMs = (ta.AvgDurationMs*float64(ta.TotalRequests-1) + m.DurationMs) / float64(ta.TotalRequests)
}

// GetAggregate returns the aggregated metrics
func (c *Collector) GetAggregate() *AggregateMetrics {
	return c.aggregate
}

// Export exports the provided aggregate metrics directly
func (c *Collector) Export(aggregate *AggregateMetrics) error {
	for _, exp := range c.exporters {
		if err := exp.Export(aggregate); err != nil {
			return err
		}
	}
	return nil
}

// Flush exports all aggregated metrics
func (c *Collector) Flush() error {
	for _, exp := range c.exporters {
		if err := exp.Export(c.aggregate); err != nil {
			return err
		}
	}
	return nil
}

// Close closes all exporters
func (c *Collector) Close() error {
	for _, exp := range c.exporters {
		if err := exp.Close(); err != nil {
			return err
		}
	}
	return nil
}
