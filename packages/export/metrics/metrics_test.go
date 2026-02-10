package metrics

import (
	"errors"
	"testing"
	"time"
)

type mockExporter struct {
	exported  []*AggregateMetrics
	singles   []*TestMetrics
	closed    bool
	exportErr error
	singleErr error
}

func (m *mockExporter) Export(metrics *AggregateMetrics) error {
	m.exported = append(m.exported, metrics)
	return m.exportErr
}

func (m *mockExporter) ExportSingle(metric *TestMetrics) error {
	m.singles = append(m.singles, metric)
	return m.singleErr
}

func (m *mockExporter) Close() error {
	m.closed = true
	return nil
}

func TestCollector_Record(t *testing.T) {
	exp := &mockExporter{}
	c := NewCollector(exp)

	c.Record(&TestMetrics{
		TestName:   "GET /users",
		StatusCode: 200,
		DurationMs: 42,
		Passed:     true,
		Timestamp:  time.Now(),
	})

	if len(exp.singles) != 1 {
		t.Fatalf("exported singles = %d, want 1", len(exp.singles))
	}
	if exp.singles[0].TestName != "GET /users" {
		t.Errorf("test name = %q, want %q", exp.singles[0].TestName, "GET /users")
	}

	agg := c.GetAggregate()
	if agg.TotalRequests != 1 {
		t.Errorf("total requests = %d, want 1", agg.TotalRequests)
	}
	if agg.SuccessCount != 1 {
		t.Errorf("success count = %d, want 1", agg.SuccessCount)
	}
}

func TestCollector_Aggregate(t *testing.T) {
	c := NewCollector()

	c.Record(&TestMetrics{TestName: "A", StatusCode: 200, DurationMs: 10, Passed: true, Timestamp: time.Now()})
	c.Record(&TestMetrics{TestName: "B", StatusCode: 404, DurationMs: 20, Passed: false, Timestamp: time.Now()})
	c.Record(&TestMetrics{TestName: "A", StatusCode: 200, DurationMs: 30, Passed: true, Timestamp: time.Now()})

	agg := c.GetAggregate()

	if agg.TotalRequests != 3 {
		t.Errorf("total = %d, want 3", agg.TotalRequests)
	}
	if agg.SuccessCount != 2 {
		t.Errorf("success = %d, want 2", agg.SuccessCount)
	}
	if agg.FailureCount != 1 {
		t.Errorf("failure = %d, want 1", agg.FailureCount)
	}
	if agg.MinDurationMs != 10 {
		t.Errorf("min = %f, want 10", agg.MinDurationMs)
	}
	if agg.MaxDurationMs != 30 {
		t.Errorf("max = %f, want 30", agg.MaxDurationMs)
	}
	if agg.AvgDurationMs != 20 {
		t.Errorf("avg = %f, want 20", agg.AvgDurationMs)
	}

	// Status code counts
	if agg.StatusCodes[200] != 2 {
		t.Errorf("status 200 count = %d, want 2", agg.StatusCodes[200])
	}
	if agg.StatusCodes[404] != 1 {
		t.Errorf("status 404 count = %d, want 1", agg.StatusCodes[404])
	}
}

func TestCollector_PerTestAggregate(t *testing.T) {
	c := NewCollector()

	c.Record(&TestMetrics{TestName: "Login", DurationMs: 100, Passed: true, StatusCode: 200, Timestamp: time.Now()})
	c.Record(&TestMetrics{TestName: "Login", DurationMs: 200, Passed: true, StatusCode: 200, Timestamp: time.Now()})
	c.Record(&TestMetrics{TestName: "Login", DurationMs: 300, Passed: false, StatusCode: 500, Timestamp: time.Now()})

	agg := c.GetAggregate()
	ta, ok := agg.ByTest["Login"]
	if !ok {
		t.Fatal("missing per-test aggregate for 'Login'")
	}

	if ta.TotalRequests != 3 {
		t.Errorf("total = %d, want 3", ta.TotalRequests)
	}
	if ta.SuccessCount != 2 {
		t.Errorf("success = %d, want 2", ta.SuccessCount)
	}
	if ta.FailureCount != 1 {
		t.Errorf("failure = %d, want 1", ta.FailureCount)
	}
	if ta.MinDurationMs != 100 {
		t.Errorf("min = %f, want 100", ta.MinDurationMs)
	}
	if ta.MaxDurationMs != 300 {
		t.Errorf("max = %f, want 300", ta.MaxDurationMs)
	}
}

func TestCollector_Flush(t *testing.T) {
	exp := &mockExporter{}
	c := NewCollector(exp)

	c.Record(&TestMetrics{TestName: "Test", DurationMs: 10, Passed: true, StatusCode: 200, Timestamp: time.Now()})

	if err := c.Flush(); err != nil {
		t.Fatal(err)
	}

	if len(exp.exported) != 1 {
		t.Fatalf("exported = %d, want 1", len(exp.exported))
	}
	if exp.exported[0].TotalRequests != 1 {
		t.Errorf("total requests in export = %d, want 1", exp.exported[0].TotalRequests)
	}
}

func TestCollector_Export(t *testing.T) {
	exp := &mockExporter{}
	c := NewCollector(exp)

	custom := &AggregateMetrics{
		TotalRequests: 42,
		StatusCodes:   make(map[int]int64),
		ByTest:        make(map[string]*TestAggregate),
	}

	if err := c.Export(custom); err != nil {
		t.Fatal(err)
	}

	if len(exp.exported) != 1 || exp.exported[0].TotalRequests != 42 {
		t.Error("Export should pass custom aggregate to exporter")
	}
}

func TestCollector_FlushError(t *testing.T) {
	exp := &mockExporter{exportErr: errors.New("export failed")}
	c := NewCollector(exp)

	if err := c.Flush(); err == nil {
		t.Error("expected error from flush")
	}
}

func TestCollector_Close(t *testing.T) {
	exp := &mockExporter{}
	c := NewCollector(exp)

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if !exp.closed {
		t.Error("exporter should be closed")
	}
}

func TestCollector_MultipleExporters(t *testing.T) {
	exp1 := &mockExporter{}
	exp2 := &mockExporter{}
	c := NewCollector(exp1, exp2)

	c.Record(&TestMetrics{TestName: "Test", DurationMs: 10, Passed: true, StatusCode: 200, Timestamp: time.Now()})

	if len(exp1.singles) != 1 {
		t.Error("exp1 should have received single metric")
	}
	if len(exp2.singles) != 1 {
		t.Error("exp2 should have received single metric")
	}

	if err := c.Flush(); err != nil {
		t.Fatal(err)
	}

	if len(exp1.exported) != 1 {
		t.Error("exp1 should have received aggregate")
	}
	if len(exp2.exported) != 1 {
		t.Error("exp2 should have received aggregate")
	}
}
