package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// JSONExporter exports metrics to JSON format
type JSONExporter struct {
	writer    io.Writer
	filePath  string
	pretty    bool
	metrics   []*TestMetrics
	startTime time.Time
}

// JSONOption is a functional option for JSONExporter
type JSONOption func(*JSONExporter)

// WithJSONWriter sets the output writer for JSON metrics
func WithJSONWriter(w io.Writer) JSONOption {
	return func(j *JSONExporter) {
		j.writer = w
	}
}

// WithJSONFile sets the output file for JSON metrics
func WithJSONFile(path string) JSONOption {
	return func(j *JSONExporter) {
		j.filePath = path
	}
}

// WithJSONPretty enables pretty-printed JSON output
func WithJSONPretty(pretty bool) JSONOption {
	return func(j *JSONExporter) {
		j.pretty = pretty
	}
}

// NewJSONExporter creates a new JSON metrics exporter
func NewJSONExporter(opts ...JSONOption) *JSONExporter {
	j := &JSONExporter{
		metrics:   make([]*TestMetrics, 0),
		startTime: time.Now(),
		pretty:    true,
	}

	for _, opt := range opts {
		opt(j)
	}

	return j
}

// JSONMetricsOutput is the complete JSON output structure
type JSONMetricsOutput struct {
	Metadata   JSONMetadata       `json:"metadata"`
	Summary    *AggregateMetrics  `json:"summary"`
	TestResults []*TestMetrics    `json:"test_results"`
}

// JSONMetadata contains metadata about the metrics collection
type JSONMetadata struct {
	GeneratedAt string `json:"generated_at"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Duration    string `json:"duration"`
	Version     string `json:"version"`
}

// Export exports aggregated metrics to JSON
func (j *JSONExporter) Export(metrics *AggregateMetrics) error {
	endTime := time.Now()

	output := JSONMetricsOutput{
		Metadata: JSONMetadata{
			GeneratedAt: endTime.Format(time.RFC3339),
			StartTime:   j.startTime.Format(time.RFC3339),
			EndTime:     endTime.Format(time.RFC3339),
			Duration:    endTime.Sub(j.startTime).String(),
			Version:     "1.0",
		},
		Summary:     metrics,
		TestResults: j.metrics,
	}

	var data []byte
	var err error

	if j.pretty {
		data, err = json.MarshalIndent(output, "", "  ")
	} else {
		data, err = json.Marshal(output)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	// Write to file if path is specified
	if j.filePath != "" {
		if err := os.WriteFile(j.filePath, data, 0644); err != nil {
			return fmt.Errorf("failed to write metrics file: %w", err)
		}
	}

	// Write to writer if specified
	if j.writer != nil {
		if _, err := j.writer.Write(data); err != nil {
			return fmt.Errorf("failed to write metrics: %w", err)
		}
		if _, err := j.writer.Write([]byte("\n")); err != nil {
			return fmt.Errorf("failed to write newline: %w", err)
		}
	}

	return nil
}

// ExportSingle records a single test metric
func (j *JSONExporter) ExportSingle(metric *TestMetrics) error {
	j.metrics = append(j.metrics, metric)
	return nil
}

// Close closes the JSON exporter
func (j *JSONExporter) Close() error {
	return nil
}
