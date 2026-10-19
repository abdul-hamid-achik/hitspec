package cmd

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/export/metrics"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/stress"
	"github.com/stretchr/testify/assert"
)

type fakeSummary struct{}

func (fakeSummary) GetSummary() *stress.Summary {
	return &stress.Summary{TotalRequests: 42, SuccessCount: 40, ErrorCount: 2, Duration: time.Second}
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// Prometheus numbers must be published while the run is still going, not only
// from the final Export.
func TestPublishPrometheusLive_UpdatesBeforeStop(t *testing.T) {
	var buf lockedBuffer
	exp := metrics.NewPrometheusExporter(metrics.WithPrometheusWriter(&buf))
	stop := publishPrometheusLive(context.Background(), fakeSummary{}, []metrics.Exporter{exp})

	deadline := time.Now().Add(livePublishInterval*2 + time.Second)
	for !strings.Contains(buf.String(), "hitspec_requests_total 42") && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	assert.Contains(t, buf.String(), "hitspec_requests_total 42", "exporter was not refreshed before stop()")
	stop()
}

func TestPublishPrometheusLive_NoPrometheusExporterIsNoop(t *testing.T) {
	stop := publishPrometheusLive(context.Background(), fakeSummary{}, nil)
	stop() // must not block or panic
}
