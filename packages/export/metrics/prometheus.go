package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// PrometheusExporter exports metrics in Prometheus text format
type PrometheusExporter struct {
	mu           sync.RWMutex
	metrics      []*TestMetrics
	aggregate    *AggregateMetrics
	writer       io.Writer
	serveHTTP    bool
	port         int
	server       *http.Server
	shutdownChan chan struct{}
}

// PrometheusOption is a functional option for PrometheusExporter
type PrometheusOption func(*PrometheusExporter)

// WithPrometheusWriter sets the output writer for Prometheus metrics
func WithPrometheusWriter(w io.Writer) PrometheusOption {
	return func(p *PrometheusExporter) {
		p.writer = w
	}
}

// WithPrometheusHTTP enables HTTP endpoint serving
func WithPrometheusHTTP(port int) PrometheusOption {
	return func(p *PrometheusExporter) {
		p.serveHTTP = true
		p.port = port
	}
}

// NewPrometheusExporter creates a new Prometheus metrics exporter
func NewPrometheusExporter(opts ...PrometheusOption) *PrometheusExporter {
	p := &PrometheusExporter{
		metrics:      make([]*TestMetrics, 0),
		aggregate:    &AggregateMetrics{StatusCodes: make(map[int]int64), ByTest: make(map[string]*TestAggregate)},
		shutdownChan: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(p)
	}

	if p.serveHTTP {
		go p.startHTTPServer()
	}

	return p
}

func (p *PrometheusExporter) startHTTPServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", p.handleMetrics)

	p.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", p.port),
		Handler: mux,
	}

	go func() {
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Prometheus HTTP server error: %v\n", err)
		}
	}()

	<-p.shutdownChan
}

func (p *PrometheusExporter) handleMetrics(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	p.writeMetrics(w)
}

// Export exports aggregated metrics
func (p *PrometheusExporter) Export(metrics *AggregateMetrics) error {
	p.mu.Lock()
	p.aggregate = metrics
	p.mu.Unlock()

	if p.writer != nil {
		p.mu.RLock()
		defer p.mu.RUnlock()
		p.writeMetrics(p.writer)
	}

	return nil
}

// ExportSingle records a single test metric
func (p *PrometheusExporter) ExportSingle(metric *TestMetrics) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.metrics = append(p.metrics, metric)
	p.updateAggregate(metric)

	return nil
}

func (p *PrometheusExporter) updateAggregate(m *TestMetrics) {
	p.aggregate.TotalRequests++
	p.aggregate.TotalDurationMs += m.DurationMs

	if m.Passed {
		p.aggregate.SuccessCount++
	} else {
		p.aggregate.FailureCount++
	}

	if p.aggregate.TotalRequests == 1 {
		p.aggregate.MinDurationMs = m.DurationMs
		p.aggregate.MaxDurationMs = m.DurationMs
	} else {
		if m.DurationMs < p.aggregate.MinDurationMs {
			p.aggregate.MinDurationMs = m.DurationMs
		}
		if m.DurationMs > p.aggregate.MaxDurationMs {
			p.aggregate.MaxDurationMs = m.DurationMs
		}
	}

	p.aggregate.AvgDurationMs = p.aggregate.TotalDurationMs / float64(p.aggregate.TotalRequests)
	p.aggregate.StatusCodes[m.StatusCode]++
}

func (p *PrometheusExporter) writeMetrics(w io.Writer) {
	now := time.Now().UnixMilli()

	// Total requests counter
	fmt.Fprintf(w, "# HELP hitspec_requests_total Total number of HTTP requests made\n")
	fmt.Fprintf(w, "# TYPE hitspec_requests_total counter\n")
	fmt.Fprintf(w, "hitspec_requests_total %d %d\n", p.aggregate.TotalRequests, now)
	fmt.Fprintln(w)

	// Success/failure counters
	fmt.Fprintf(w, "# HELP hitspec_requests_success_total Total number of successful requests\n")
	fmt.Fprintf(w, "# TYPE hitspec_requests_success_total counter\n")
	fmt.Fprintf(w, "hitspec_requests_success_total %d %d\n", p.aggregate.SuccessCount, now)
	fmt.Fprintln(w)

	fmt.Fprintf(w, "# HELP hitspec_requests_failed_total Total number of failed requests\n")
	fmt.Fprintf(w, "# TYPE hitspec_requests_failed_total counter\n")
	fmt.Fprintf(w, "hitspec_requests_failed_total %d %d\n", p.aggregate.FailureCount, now)
	fmt.Fprintln(w)

	// Duration metrics
	fmt.Fprintf(w, "# HELP hitspec_request_duration_ms Request duration in milliseconds\n")
	fmt.Fprintf(w, "# TYPE hitspec_request_duration_ms gauge\n")
	fmt.Fprintf(w, "hitspec_request_duration_ms{quantile=\"min\"} %.2f %d\n", p.aggregate.MinDurationMs, now)
	fmt.Fprintf(w, "hitspec_request_duration_ms{quantile=\"max\"} %.2f %d\n", p.aggregate.MaxDurationMs, now)
	fmt.Fprintf(w, "hitspec_request_duration_ms{quantile=\"avg\"} %.2f %d\n", p.aggregate.AvgDurationMs, now)
	if p.aggregate.P50DurationMs > 0 {
		fmt.Fprintf(w, "hitspec_request_duration_ms{quantile=\"0.50\"} %.2f %d\n", p.aggregate.P50DurationMs, now)
	}
	if p.aggregate.P95DurationMs > 0 {
		fmt.Fprintf(w, "hitspec_request_duration_ms{quantile=\"0.95\"} %.2f %d\n", p.aggregate.P95DurationMs, now)
	}
	if p.aggregate.P99DurationMs > 0 {
		fmt.Fprintf(w, "hitspec_request_duration_ms{quantile=\"0.99\"} %.2f %d\n", p.aggregate.P99DurationMs, now)
	}
	fmt.Fprintln(w)

	// Status code distribution
	fmt.Fprintf(w, "# HELP hitspec_requests_by_status_total Requests by HTTP status code\n")
	fmt.Fprintf(w, "# TYPE hitspec_requests_by_status_total counter\n")

	// Sort status codes for consistent output
	codes := make([]int, 0, len(p.aggregate.StatusCodes))
	for code := range p.aggregate.StatusCodes {
		codes = append(codes, code)
	}
	sort.Ints(codes)

	for _, code := range codes {
		count := p.aggregate.StatusCodes[code]
		fmt.Fprintf(w, "hitspec_requests_by_status_total{status=\"%d\"} %d %d\n", code, count, now)
	}
	fmt.Fprintln(w)

	// Per-test metrics
	if len(p.aggregate.ByTest) > 0 {
		fmt.Fprintf(w, "# HELP hitspec_test_requests_total Requests per test\n")
		fmt.Fprintf(w, "# TYPE hitspec_test_requests_total counter\n")

		// Sort test names for consistent output
		names := make([]string, 0, len(p.aggregate.ByTest))
		for name := range p.aggregate.ByTest {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			ta := p.aggregate.ByTest[name]
			safeName := sanitizeLabel(name)
			fmt.Fprintf(w, "hitspec_test_requests_total{test=\"%s\"} %d %d\n", safeName, ta.TotalRequests, now)
		}
		fmt.Fprintln(w)

		fmt.Fprintf(w, "# HELP hitspec_test_duration_avg_ms Average request duration per test\n")
		fmt.Fprintf(w, "# TYPE hitspec_test_duration_avg_ms gauge\n")
		for _, name := range names {
			ta := p.aggregate.ByTest[name]
			safeName := sanitizeLabel(name)
			fmt.Fprintf(w, "hitspec_test_duration_avg_ms{test=\"%s\"} %.2f %d\n", safeName, ta.AvgDurationMs, now)
		}
	}
}

// sanitizeLabel makes a string safe for use as a Prometheus label value
func sanitizeLabel(s string) string {
	// Replace characters that need escaping
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// Close shuts down the exporter
func (p *PrometheusExporter) Close() error {
	if p.server != nil {
		close(p.shutdownChan)
		return p.server.Close()
	}
	return nil
}
