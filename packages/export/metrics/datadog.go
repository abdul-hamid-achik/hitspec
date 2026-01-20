package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// DataDogExporter exports metrics to DataDog
type DataDogExporter struct {
	apiKey   string
	site     string // e.g., "datadoghq.com", "datadoghq.eu"
	tags     []string
	prefix   string
	client   *http.Client
}

// DataDogOption is a functional option for DataDogExporter
type DataDogOption func(*DataDogExporter)

// WithDataDogAPIKey sets the DataDog API key
func WithDataDogAPIKey(apiKey string) DataDogOption {
	return func(d *DataDogExporter) {
		d.apiKey = apiKey
	}
}

// WithDataDogSite sets the DataDog site (e.g., "datadoghq.com", "datadoghq.eu")
func WithDataDogSite(site string) DataDogOption {
	return func(d *DataDogExporter) {
		d.site = site
	}
}

// WithDataDogTags sets additional tags for all metrics
func WithDataDogTags(tags []string) DataDogOption {
	return func(d *DataDogExporter) {
		d.tags = tags
	}
}

// WithDataDogPrefix sets a prefix for metric names
func WithDataDogPrefix(prefix string) DataDogOption {
	return func(d *DataDogExporter) {
		d.prefix = prefix
	}
}

// NewDataDogExporter creates a new DataDog metrics exporter
func NewDataDogExporter(opts ...DataDogOption) *DataDogExporter {
	d := &DataDogExporter{
		site:   "datadoghq.com",
		prefix: "hitspec",
		client: &http.Client{Timeout: 10 * time.Second},
		tags:   make([]string, 0),
	}

	for _, opt := range opts {
		opt(d)
	}

	// Try to get API key from environment if not set
	if d.apiKey == "" {
		d.apiKey = os.Getenv("DD_API_KEY")
	}

	return d
}

// datadogMetric represents a metric in DataDog format
type datadogMetric struct {
	Metric string  `json:"metric"`
	Type   string  `json:"type"`
	Points [][]any `json:"points"`
	Tags   []string `json:"tags,omitempty"`
}

// datadogPayload is the payload sent to DataDog
type datadogPayload struct {
	Series []datadogMetric `json:"series"`
}

// Export exports aggregated metrics to DataDog
func (d *DataDogExporter) Export(metrics *AggregateMetrics) error {
	if d.apiKey == "" {
		return fmt.Errorf("DataDog API key not configured")
	}

	now := float64(time.Now().Unix())
	series := make([]datadogMetric, 0)

	// Total requests
	series = append(series, datadogMetric{
		Metric: d.metricName("requests.total"),
		Type:   "count",
		Points: [][]any{{now, float64(metrics.TotalRequests)}},
		Tags:   d.tags,
	})

	// Success/failure counts
	series = append(series, datadogMetric{
		Metric: d.metricName("requests.success"),
		Type:   "count",
		Points: [][]any{{now, float64(metrics.SuccessCount)}},
		Tags:   d.tags,
	})

	series = append(series, datadogMetric{
		Metric: d.metricName("requests.failed"),
		Type:   "count",
		Points: [][]any{{now, float64(metrics.FailureCount)}},
		Tags:   d.tags,
	})

	// Duration metrics
	series = append(series, datadogMetric{
		Metric: d.metricName("duration.avg"),
		Type:   "gauge",
		Points: [][]any{{now, metrics.AvgDurationMs}},
		Tags:   d.tags,
	})

	series = append(series, datadogMetric{
		Metric: d.metricName("duration.min"),
		Type:   "gauge",
		Points: [][]any{{now, metrics.MinDurationMs}},
		Tags:   d.tags,
	})

	series = append(series, datadogMetric{
		Metric: d.metricName("duration.max"),
		Type:   "gauge",
		Points: [][]any{{now, metrics.MaxDurationMs}},
		Tags:   d.tags,
	})

	if metrics.P50DurationMs > 0 {
		series = append(series, datadogMetric{
			Metric: d.metricName("duration.p50"),
			Type:   "gauge",
			Points: [][]any{{now, metrics.P50DurationMs}},
			Tags:   d.tags,
		})
	}

	if metrics.P95DurationMs > 0 {
		series = append(series, datadogMetric{
			Metric: d.metricName("duration.p95"),
			Type:   "gauge",
			Points: [][]any{{now, metrics.P95DurationMs}},
			Tags:   d.tags,
		})
	}

	if metrics.P99DurationMs > 0 {
		series = append(series, datadogMetric{
			Metric: d.metricName("duration.p99"),
			Type:   "gauge",
			Points: [][]any{{now, metrics.P99DurationMs}},
			Tags:   d.tags,
		})
	}

	// Status code distribution
	for code, count := range metrics.StatusCodes {
		tags := append([]string{fmt.Sprintf("status:%d", code)}, d.tags...)
		series = append(series, datadogMetric{
			Metric: d.metricName("requests.by_status"),
			Type:   "count",
			Points: [][]any{{now, float64(count)}},
			Tags:   tags,
		})
	}

	// Per-test metrics
	for name, ta := range metrics.ByTest {
		tags := append([]string{fmt.Sprintf("test:%s", name)}, d.tags...)

		series = append(series, datadogMetric{
			Metric: d.metricName("test.requests"),
			Type:   "count",
			Points: [][]any{{now, float64(ta.TotalRequests)}},
			Tags:   tags,
		})

		series = append(series, datadogMetric{
			Metric: d.metricName("test.duration.avg"),
			Type:   "gauge",
			Points: [][]any{{now, ta.AvgDurationMs}},
			Tags:   tags,
		})
	}

	return d.sendMetrics(series)
}

// ExportSingle exports a single test metric to DataDog
func (d *DataDogExporter) ExportSingle(metric *TestMetrics) error {
	if d.apiKey == "" {
		return fmt.Errorf("DataDog API key not configured")
	}

	now := float64(metric.Timestamp.Unix())
	tags := append([]string{
		fmt.Sprintf("test:%s", metric.TestName),
		fmt.Sprintf("method:%s", metric.RequestMethod),
		fmt.Sprintf("status:%d", metric.StatusCode),
	}, d.tags...)

	if metric.Passed {
		tags = append(tags, "result:passed")
	} else {
		tags = append(tags, "result:failed")
	}

	series := []datadogMetric{
		{
			Metric: d.metricName("request.duration"),
			Type:   "gauge",
			Points: [][]any{{now, metric.DurationMs}},
			Tags:   tags,
		},
		{
			Metric: d.metricName("request.count"),
			Type:   "count",
			Points: [][]any{{now, 1.0}},
			Tags:   tags,
		},
	}

	return d.sendMetrics(series)
}

func (d *DataDogExporter) metricName(name string) string {
	return d.prefix + "." + name
}

func (d *DataDogExporter) sendMetrics(series []datadogMetric) error {
	payload := datadogPayload{Series: series}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	url := fmt.Sprintf("https://api.%s/api/v1/series", d.site)
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", d.apiKey)

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DataDog API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Close closes the DataDog exporter
func (d *DataDogExporter) Close() error {
	return nil
}
