package metrics

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/portprobe"
)

// PrometheusExporter exports metrics in Prometheus text format
type PrometheusExporter struct {
	mu        sync.RWMutex
	metrics   []*TestMetrics
	aggregate *AggregateMetrics
	writer    io.Writer
	serveHTTP bool
	host      string
	port      int
	server    *http.Server // guarded by mu
}

// PrometheusOption is a functional option for PrometheusExporter
type PrometheusOption func(*PrometheusExporter)

// WithPrometheusWriter sets the output writer for Prometheus metrics
func WithPrometheusWriter(w io.Writer) PrometheusOption {
	return func(p *PrometheusExporter) {
		p.writer = w
	}
}

// DefaultPrometheusHost is the interface the HTTP endpoint binds unless
// WithPrometheusHost says otherwise: metrics are not exposed to the network by
// default, like the mock server and the recording proxy.
const DefaultPrometheusHost = "127.0.0.1"

// WithPrometheusHost sets the interface the HTTP endpoint binds ("0.0.0.0" to
// let a Prometheus server on another machine scrape it). Empty keeps the
// loopback default.
func WithPrometheusHost(host string) PrometheusOption {
	return func(p *PrometheusExporter) {
		p.host = host
	}
}

// WithPrometheusHTTP enables HTTP endpoint serving
func WithPrometheusHTTP(port int) PrometheusOption {
	return func(p *PrometheusExporter) {
		p.serveHTTP = true
		p.port = port
	}
}

// NewPrometheusExporter creates a new Prometheus metrics exporter. When HTTP
// serving is enabled and the address cannot be bound, the exporter is returned
// without an endpoint and the failure is dropped; use NewPrometheusExporterE
// to learn about it.
func NewPrometheusExporter(opts ...PrometheusOption) *PrometheusExporter {
	p, err := NewPrometheusExporterE(opts...)
	if err != nil {
		return newPrometheusExporter(opts...)
	}
	return p
}

// NewPrometheusExporterE is NewPrometheusExporter that binds the HTTP endpoint
// before returning: a busy port or an unusable address is an error, not a
// metrics endpoint that quietly never came up.
func NewPrometheusExporterE(opts ...PrometheusOption) (*PrometheusExporter, error) {
	p := newPrometheusExporter(opts...)
	if p.serveHTTP {
		if err := p.startHTTPServer(); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func newPrometheusExporter(opts ...PrometheusOption) *PrometheusExporter {
	p := &PrometheusExporter{
		metrics:   make([]*TestMetrics, 0),
		aggregate: &AggregateMetrics{StatusCodes: make(map[int]int64), ByTest: make(map[string]*TestAggregate)},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// listenAddr is the host:port the HTTP endpoint binds.
func (p *PrometheusExporter) listenAddr() string {
	host := p.host
	if host == "" {
		host = DefaultPrometheusHost
	}
	return net.JoinHostPort(host, strconv.Itoa(p.port))
}

// startHTTPServer binds the endpoint synchronously and serves it in the
// background. The error names the address, like the mock server's.
func (p *PrometheusExporter) startHTTPServer() error {
	addr := p.listenAddr()
	// bind() can succeed next to a wildcard listener (macOS, SO_REUSEADDR):
	// probe first so that case is an error too.
	if portprobe.ShouldProbe(p.host) && portprobe.InUse(p.port) {
		return fmt.Errorf("metrics endpoint cannot listen on %s: port %d is already in use", addr, p.port)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("metrics endpoint cannot listen on %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", p.handleMetrics)
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	p.mu.Lock()
	p.server = srv
	p.mu.Unlock()

	go func() { _ = srv.Serve(ln) }()
	return nil
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
	p.mu.Lock()
	srv := p.server
	p.mu.Unlock()
	if srv != nil {
		return srv.Close()
	}
	return nil
}
