package metrics

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestJSONExporter covers the JSON metrics exporter (a test-gap area): Export
// writes valid JSON with the aggregate summary and recorded test results.
func TestJSONExporter(t *testing.T) {
	var buf bytes.Buffer
	j := NewJSONExporter(WithJSONWriter(&buf))
	_ = j.ExportSingle(&TestMetrics{TestName: "login", DurationMs: 12, Passed: true, StatusCode: 200, Timestamp: time.Now()})
	agg := &AggregateMetrics{
		TotalRequests: 1, SuccessCount: 1, TotalDurationMs: 12,
		StatusCodes: map[int]int64{200: 1},
		ByTest:      make(map[string]*TestAggregate),
	}
	if err := j.Export(agg); err != nil {
		t.Fatalf("Export: %v", err)
	}

	var out JSONMetricsOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if out.Summary == nil || out.Summary.TotalRequests != 1 {
		t.Errorf("summary.TotalRequests = %v, want 1", out.Summary)
	}
	if len(out.TestResults) != 1 || out.TestResults[0].TestName != "login" {
		t.Errorf("test_results = %+v, want one 'login' entry", out.TestResults)
	}
}

// TestPrometheusExporter covers the Prometheus metrics exporter (a test-gap
// area): Export writes text/plain Prometheus-format metrics to the configured
// writer with the expected metric names and labels.
func TestPrometheusExporter(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrometheusExporter(WithPrometheusWriter(&buf))
	defer p.Close()

	_ = p.ExportSingle(&TestMetrics{TestName: "login", DurationMs: 12, Passed: true, StatusCode: 200, Timestamp: time.Now()})
	agg := &AggregateMetrics{
		TotalRequests: 1, SuccessCount: 1, TotalDurationMs: 12,
		StatusCodes: map[int]int64{200: 1},
		ByTest:      make(map[string]*TestAggregate),
	}
	if err := p.Export(agg); err != nil {
		t.Fatalf("Export: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"hitspec_requests_total", "hitspec_requests_success_total", "hitspec_requests_by_status_total"} {
		if !strings.Contains(out, want) {
			t.Errorf("prometheus output missing %q\n%s", want, out)
		}
	}
}
