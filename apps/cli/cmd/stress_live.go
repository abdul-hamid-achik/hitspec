package cmd

import (
	"context"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/export/metrics"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/stress"
)

// summarySource is what publishPrometheusLive reads: the stress runner.
type summarySource interface {
	GetSummary() *stress.Summary
}

// livePublishInterval is how often Prometheus exporters are refreshed during a
// stress run.
const livePublishInterval = time.Second

// publishPrometheusLive pushes the runner's running totals to every Prometheus
// exporter once a second, so a scrape during the test sees the current numbers
// instead of zeros until the end. Other exporters (JSON, Datadog) are written
// once, after the run. The returned func stops the loop and waits for it.
func publishPrometheusLive(ctx context.Context, src summarySource, exporters []metrics.Exporter) (stop func()) {
	var live []*metrics.PrometheusExporter
	for _, e := range exporters {
		if p, ok := e.(*metrics.PrometheusExporter); ok {
			live = append(live, p)
		}
	}
	if len(live) == 0 {
		return func() {}
	}
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(livePublishInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				aggregate := stressResultToMetrics(src.GetSummary())
				for _, p := range live {
					_ = p.Export(aggregate)
				}
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}
