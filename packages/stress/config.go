// Package stress provides Artillery-style stress testing capabilities for HTTP APIs.
// It supports rate-based and virtual user-based load generation with real-time
// metrics collection, threshold evaluation, and detailed reporting.
package stress

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ExecutionMode defines how the stress test schedules requests
type ExecutionMode int

const (
	// RateMode sends requests at a constant rate (requests per second)
	RateMode ExecutionMode = iota
	// VUMode uses virtual users that send requests with think time between them
	VUMode
)

// Config holds all configuration for a stress test
type Config struct {
	Mode       ExecutionMode
	Duration   time.Duration
	Rate       float64       // requests per second (RateMode)
	VUs        int           // number of virtual users (VUMode)
	MaxVUs     int           // max concurrent requests
	ThinkTime  time.Duration // time between requests per VU
	RampUp     time.Duration // ramp-up time
	Warmup     time.Duration // warmup time (not counted in metrics)
	Phases     []Phase       // multi-phase execution
	Thresholds Thresholds    // pass/fail thresholds
}

// Phase defines a test phase with specific settings
type Phase struct {
	Name     string
	Duration time.Duration
	Rate     float64 // for RateMode
	VUs      int     // for VUMode
}

// Thresholds defines pass/fail criteria for the stress test
type Thresholds struct {
	P50         time.Duration // 50th percentile latency
	P95         time.Duration // 95th percentile latency
	P99         time.Duration // 99th percentile latency
	MaxLatency  time.Duration // maximum allowed latency
	ErrorRate   float64       // maximum error rate (0.0 - 1.0)
	MinRPS      float64       // minimum requests per second
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Mode:      RateMode,
		Duration:  30 * time.Second,
		Rate:      10,
		MaxVUs:    100,
		ThinkTime: 0,
		RampUp:    0,
		Warmup:    0,
	}
}

// Validate checks if the config is valid
func (c *Config) Validate() error {
	if c.Duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}

	if c.Mode == RateMode && c.Rate <= 0 {
		return fmt.Errorf("rate must be positive in rate mode")
	}

	if c.Mode == VUMode && c.VUs <= 0 {
		return fmt.Errorf("VUs must be positive in VU mode")
	}

	if c.MaxVUs < 1 {
		return fmt.Errorf("maxVUs must be at least 1")
	}

	if c.RampUp < 0 {
		return fmt.Errorf("rampUp cannot be negative")
	}

	if c.RampUp > c.Duration {
		return fmt.Errorf("rampUp cannot exceed duration")
	}

	return nil
}

// ParseThresholds parses a threshold string like "p95<200ms,errors<0.1%"
func ParseThresholds(s string) (Thresholds, error) {
	var t Thresholds

	if s == "" {
		return t, nil
	}

	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if err := parseThresholdPart(part, &t); err != nil {
			return t, err
		}
	}

	return t, nil
}

func parseThresholdPart(part string, t *Thresholds) error {
	// Match patterns like "p95<200ms", "errors<0.1%", "rps>50"
	re := regexp.MustCompile(`^(\w+)\s*([<>]=?)\s*(.+)$`)
	matches := re.FindStringSubmatch(part)
	if len(matches) != 4 {
		return fmt.Errorf("invalid threshold format: %s", part)
	}

	metric := strings.ToLower(matches[1])
	op := matches[2]
	valueStr := matches[3]

	switch metric {
	case "p50":
		d, err := time.ParseDuration(valueStr)
		if err != nil {
			return fmt.Errorf("invalid duration for p50: %s", valueStr)
		}
		if op != "<" && op != "<=" {
			return fmt.Errorf("p50 threshold must use < or <=")
		}
		t.P50 = d

	case "p95":
		d, err := time.ParseDuration(valueStr)
		if err != nil {
			return fmt.Errorf("invalid duration for p95: %s", valueStr)
		}
		if op != "<" && op != "<=" {
			return fmt.Errorf("p95 threshold must use < or <=")
		}
		t.P95 = d

	case "p99":
		d, err := time.ParseDuration(valueStr)
		if err != nil {
			return fmt.Errorf("invalid duration for p99: %s", valueStr)
		}
		if op != "<" && op != "<=" {
			return fmt.Errorf("p99 threshold must use < or <=")
		}
		t.P99 = d

	case "max", "maxlatency":
		d, err := time.ParseDuration(valueStr)
		if err != nil {
			return fmt.Errorf("invalid duration for max latency: %s", valueStr)
		}
		if op != "<" && op != "<=" {
			return fmt.Errorf("max latency threshold must use < or <=")
		}
		t.MaxLatency = d

	case "errors", "error", "errorrate":
		// Handle percentage format like "0.1%" or decimal like "0.001"
		valueStr = strings.TrimSuffix(valueStr, "%")
		f, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return fmt.Errorf("invalid error rate: %s", valueStr)
		}
		if strings.Contains(part, "%") {
			f = f / 100 // Convert percentage to decimal
		}
		if op != "<" && op != "<=" {
			return fmt.Errorf("error rate threshold must use < or <=")
		}
		t.ErrorRate = f

	case "rps", "rate":
		f, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return fmt.Errorf("invalid RPS: %s", valueStr)
		}
		if op != ">" && op != ">=" {
			return fmt.Errorf("RPS threshold must use > or >=")
		}
		t.MinRPS = f

	default:
		return fmt.Errorf("unknown threshold metric: %s", metric)
	}

	return nil
}

// HasThresholds returns true if any thresholds are configured
func (t *Thresholds) HasThresholds() bool {
	return t.P50 > 0 || t.P95 > 0 || t.P99 > 0 || t.MaxLatency > 0 || t.ErrorRate > 0 || t.MinRPS > 0
}

// ThresholdResult holds the result of evaluating a threshold
type ThresholdResult struct {
	Name     string
	Passed   bool
	Expected string
	Actual   string
}

// RequestConfig holds per-request stress test configuration
type RequestConfig struct {
	Weight   int  // relative weight for request selection (default 1)
	Think    int  // think time in ms after this request
	Skip     bool // exclude from stress testing
	Setup    bool // run once before test starts
	Teardown bool // run once after test ends
}

// DefaultRequestConfig returns default request configuration
func DefaultRequestConfig() *RequestConfig {
	return &RequestConfig{
		Weight: 1,
		Think:  0,
		Skip:   false,
		Setup:  false,
	}
}
