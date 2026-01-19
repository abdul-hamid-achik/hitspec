package stress

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
)

// Reporter handles output for stress tests
type Reporter struct {
	writer     io.Writer
	noColor    bool
	noProgress bool
	verbose    bool

	// Colors
	green  *color.Color
	red    *color.Color
	yellow *color.Color
	cyan   *color.Color
	bold   *color.Color
	dim    *color.Color
}

// ReporterOption configures the reporter
type ReporterOption func(*Reporter)

// WithWriter sets the output writer
func WithWriter(w io.Writer) ReporterOption {
	return func(r *Reporter) {
		r.writer = w
	}
}

// WithNoColor disables colored output
func WithNoColor(noColor bool) ReporterOption {
	return func(r *Reporter) {
		r.noColor = noColor
	}
}

// WithNoProgress disables real-time progress display
func WithNoProgress(noProgress bool) ReporterOption {
	return func(r *Reporter) {
		r.noProgress = noProgress
	}
}

// WithVerbose enables verbose output
func WithVerbose(verbose bool) ReporterOption {
	return func(r *Reporter) {
		r.verbose = verbose
	}
}

// NewReporter creates a new reporter
func NewReporter(opts ...ReporterOption) *Reporter {
	r := &Reporter{
		writer: os.Stdout,
	}

	for _, opt := range opts {
		opt(r)
	}

	// Initialize colors
	color.NoColor = r.noColor
	r.green = color.New(color.FgGreen)
	r.red = color.New(color.FgRed)
	r.yellow = color.New(color.FgYellow)
	r.cyan = color.New(color.FgCyan)
	r.bold = color.New(color.Bold)
	r.dim = color.New(color.Faint)

	return r
}

// Header prints the test header
func (r *Reporter) Header(version, filename string, config *Config) {
	fmt.Fprintln(r.writer)
	r.bold.Fprintf(r.writer, "hitspec stress %s\n", version)
	fmt.Fprintln(r.writer)

	r.cyan.Fprintf(r.writer, "Stress Testing: %s\n", filename)

	var details []string
	if config.Mode == RateMode {
		details = append(details, fmt.Sprintf("Target: %.0f req/s", config.Rate))
	} else {
		details = append(details, fmt.Sprintf("VUs: %d", config.VUs))
	}
	details = append(details, fmt.Sprintf("Duration: %s", config.Duration))
	details = append(details, fmt.Sprintf("Max VUs: %d", config.MaxVUs))

	fmt.Fprintf(r.writer, "%s\n", strings.Join(details, " | "))
	fmt.Fprintln(r.writer)
}

// Progress prints real-time progress
func (r *Reporter) Progress(stats CurrentStats, duration time.Duration) {
	if r.noProgress {
		return
	}

	// Clear line and print progress
	fmt.Fprint(r.writer, "\r\033[K")

	// Progress bar
	progress := float64(stats.Elapsed) / float64(duration)
	if progress > 1 {
		progress = 1
	}
	barWidth := 30
	filled := int(progress * float64(barWidth))
	bar := strings.Repeat("━", filled) + strings.Repeat("─", barWidth-filled)

	fmt.Fprintf(r.writer, "Progress %s %s / %s\n", bar, formatDuration(stats.Elapsed), formatDuration(duration))

	// Stats line
	fmt.Fprintf(r.writer, "Requests: ")
	r.bold.Fprintf(r.writer, "%s", formatNumber(stats.Total))
	fmt.Fprintf(r.writer, " total | ")
	r.green.Fprintf(r.writer, "%s", formatNumber(stats.Success))
	fmt.Fprintf(r.writer, " success | ")
	if stats.Errors > 0 {
		r.red.Fprintf(r.writer, "%s", formatNumber(stats.Errors))
	} else {
		fmt.Fprintf(r.writer, "%s", formatNumber(stats.Errors))
	}
	fmt.Fprintf(r.writer, " errors (%.2f%%)\n", stats.ErrorRate*100)

	fmt.Fprintf(r.writer, "Rate: ")
	r.cyan.Fprintf(r.writer, "%.1f", stats.RPS)
	fmt.Fprintf(r.writer, " req/s | Active VUs: %d\n", stats.ActiveVUs)

	fmt.Fprintf(r.writer, "Latency: p50: %s | p95: %s | p99: %s | max: %s\n",
		formatLatency(stats.P50),
		formatLatency(stats.P95),
		formatLatency(stats.P99),
		formatLatency(stats.Max))

	// Move cursor up for next update
	fmt.Fprint(r.writer, "\033[4A")
}

// ClearProgress clears the progress display
func (r *Reporter) ClearProgress() {
	if r.noProgress {
		return
	}
	// Move down and clear the progress lines
	fmt.Fprint(r.writer, "\033[4B\r\033[K\033[A\r\033[K\033[A\r\033[K\033[A\r\033[K")
}

// Summary prints the final summary
func (r *Reporter) Summary(summary *Summary, thresholdResults []ThresholdResult) {
	fmt.Fprintln(r.writer)
	r.bold.Fprintln(r.writer, "STRESS TEST SUMMARY")
	fmt.Fprintln(r.writer, strings.Repeat("─", 40))

	// Duration and totals
	fmt.Fprintf(r.writer, "Duration:   %s\n", formatDuration(summary.Duration))
	fmt.Fprintf(r.writer, "Total:      ")
	r.bold.Fprintf(r.writer, "%s", formatNumber(summary.TotalRequests))
	fmt.Fprintf(r.writer, " requests (%.1f req/s)\n", summary.RPS)

	fmt.Fprintf(r.writer, "Success:    ")
	r.green.Fprintf(r.writer, "%s", formatNumber(summary.SuccessCount))
	fmt.Fprintf(r.writer, " (%.1f%%)\n", summary.SuccessRate*100)

	fmt.Fprintf(r.writer, "Failed:     ")
	if summary.ErrorCount > 0 {
		r.red.Fprintf(r.writer, "%s", formatNumber(summary.ErrorCount))
	} else {
		fmt.Fprintf(r.writer, "%s", formatNumber(summary.ErrorCount))
	}
	fmt.Fprintf(r.writer, " (%.1f%%)\n", summary.ErrorRate*100)

	if summary.TimeoutCount > 0 {
		fmt.Fprintf(r.writer, "Timeouts:   ")
		r.yellow.Fprintf(r.writer, "%s\n", formatNumber(summary.TimeoutCount))
	}

	// Latency
	fmt.Fprintln(r.writer)
	r.bold.Fprintln(r.writer, "LATENCY (ms)")
	fmt.Fprintf(r.writer, "  p50: %-6s | p95: %-6s | p99: %-6s | max: %s\n",
		formatLatencyMs(summary.P50),
		formatLatencyMs(summary.P95),
		formatLatencyMs(summary.P99),
		formatLatencyMs(summary.Max))
	fmt.Fprintf(r.writer, "  min: %-6s | mean: %-5s | stddev: %s\n",
		formatLatencyMs(summary.Min),
		formatLatencyMs(summary.Mean),
		formatLatencyMs(summary.StdDev))

	// Per-request breakdown (if verbose)
	if r.verbose && len(summary.RequestBreakdown) > 0 {
		fmt.Fprintln(r.writer)
		r.bold.Fprintln(r.writer, "PER-REQUEST BREAKDOWN")
		for name, rs := range summary.RequestBreakdown {
			fmt.Fprintf(r.writer, "  %s:\n", name)
			fmt.Fprintf(r.writer, "    Total: %s | Success: %s | Errors: %s\n",
				formatNumber(rs.Total), formatNumber(rs.Success), formatNumber(rs.Errors))
			fmt.Fprintf(r.writer, "    p50: %s | p95: %s | p99: %s\n",
				formatLatency(rs.P50), formatLatency(rs.P95), formatLatency(rs.P99))
		}
	}

	// Thresholds
	if len(thresholdResults) > 0 {
		fmt.Fprintln(r.writer)
		r.bold.Fprintln(r.writer, "THRESHOLDS")
		allPassed := true
		for _, tr := range thresholdResults {
			if tr.Passed {
				r.green.Fprintf(r.writer, "  ✓ ")
			} else {
				r.red.Fprintf(r.writer, "  ✗ ")
				allPassed = false
			}
			fmt.Fprintf(r.writer, "%s %s    (actual: %s)\n", tr.Name, tr.Expected, tr.Actual)
		}

		fmt.Fprintln(r.writer)
		if allPassed {
			r.green.Fprintln(r.writer, "All thresholds passed!")
		} else {
			r.red.Fprintln(r.writer, "Some thresholds failed!")
		}
	}

	fmt.Fprintln(r.writer)
}

// JSONSummary outputs the summary as JSON
func (r *Reporter) JSONSummary(summary *Summary, thresholdResults []ThresholdResult) error {
	output := map[string]interface{}{
		"duration": summary.Duration.String(),
		"requests": map[string]interface{}{
			"total":    summary.TotalRequests,
			"success":  summary.SuccessCount,
			"failed":   summary.ErrorCount,
			"timeouts": summary.TimeoutCount,
		},
		"rates": map[string]interface{}{
			"rps":         summary.RPS,
			"successRate": summary.SuccessRate,
			"errorRate":   summary.ErrorRate,
		},
		"latency": map[string]interface{}{
			"p50":    summary.P50.Milliseconds(),
			"p95":    summary.P95.Milliseconds(),
			"p99":    summary.P99.Milliseconds(),
			"min":    summary.Min.Milliseconds(),
			"max":    summary.Max.Milliseconds(),
			"mean":   summary.Mean.Milliseconds(),
			"stddev": summary.StdDev.Milliseconds(),
		},
	}

	if len(thresholdResults) > 0 {
		thresholds := make([]map[string]interface{}, len(thresholdResults))
		for i, tr := range thresholdResults {
			thresholds[i] = map[string]interface{}{
				"name":     tr.Name,
				"passed":   tr.Passed,
				"expected": tr.Expected,
				"actual":   tr.Actual,
			}
		}
		output["thresholds"] = thresholds
	}

	if len(summary.RequestBreakdown) > 0 {
		breakdown := make(map[string]interface{})
		for name, rs := range summary.RequestBreakdown {
			breakdown[name] = map[string]interface{}{
				"total":   rs.Total,
				"success": rs.Success,
				"errors":  rs.Errors,
				"p50":     rs.P50.Milliseconds(),
				"p95":     rs.P95.Milliseconds(),
				"p99":     rs.P99.Milliseconds(),
				"mean":    rs.Mean.Milliseconds(),
			}
		}
		output["requestBreakdown"] = breakdown
	}

	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// Error prints an error message
func (r *Reporter) Error(format string, args ...interface{}) {
	r.red.Fprintf(r.writer, "Error: "+format+"\n", args...)
}

// Info prints an info message
func (r *Reporter) Info(format string, args ...interface{}) {
	fmt.Fprintf(r.writer, format+"\n", args...)
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if seconds == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm %02ds", minutes, seconds)
}

// formatLatency formats latency for display
func formatLatency(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dμs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// formatLatencyMs formats latency in milliseconds
func formatLatencyMs(d time.Duration) string {
	ms := float64(d.Microseconds()) / 1000
	if ms < 1 {
		return fmt.Sprintf("%.2f", ms)
	}
	if ms < 10 {
		return fmt.Sprintf("%.1f", ms)
	}
	return fmt.Sprintf("%.0f", ms)
}

// formatNumber formats a number with commas
func formatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	s := fmt.Sprintf("%d", n)
	result := make([]byte, 0, len(s)+(len(s)-1)/3)

	start := len(s) % 3
	if start == 0 {
		start = 3
	}

	result = append(result, s[:start]...)
	for i := start; i < len(s); i += 3 {
		result = append(result, ',')
		result = append(result, s[i:i+3]...)
	}

	return string(result)
}
