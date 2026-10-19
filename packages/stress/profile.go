package stress

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
)

// ApplyProfile overlays a saved hitspec.yaml stress profile (stress.profiles)
// onto cfg: duration, rate, VUs, max VUs, think time, ramp-up and thresholds.
// Fields the profile leaves out keep cfg's value, so callers apply explicit
// per-run values (flags, request fields) afterwards and those win.
func ApplyProfile(cfg *Config, p *config.StressProfile) error {
	if p == nil {
		return nil
	}
	if p.Duration != "" {
		d, err := time.ParseDuration(p.Duration)
		if err != nil {
			return fmt.Errorf("invalid duration in profile: %w", err)
		}
		cfg.Duration = d
	}
	if p.Rate > 0 {
		cfg.Rate = p.Rate
	}
	if p.VUs > 0 {
		cfg.VUs = p.VUs
		cfg.Mode = VUMode
	}
	if p.MaxVUs > 0 {
		cfg.MaxVUs = p.MaxVUs
	}
	if p.ThinkTime != "" {
		d, err := time.ParseDuration(p.ThinkTime)
		if err != nil {
			return fmt.Errorf("invalid think time in profile: %w", err)
		}
		cfg.ThinkTime = d
	}
	if p.RampUp != "" {
		d, err := time.ParseDuration(p.RampUp)
		if err != nil {
			return fmt.Errorf("invalid ramp-up in profile: %w", err)
		}
		cfg.RampUp = d
	}
	if len(p.Thresholds) > 0 {
		t, err := ParseThresholds(ThresholdString(p.Thresholds))
		if err != nil {
			return fmt.Errorf("invalid thresholds in profile: %w", err)
		}
		cfg.Thresholds = t
	}
	return nil
}

// ThresholdString converts a profile's threshold map ("p95": "200ms",
// "rps": ">=50") to the "p95<200ms,rps>=50" form ParseThresholds reads. A
// value that starts with an operator keeps it; a bare value gets the metric's
// natural one (> for rps, < for everything else). Keys are sorted so the
// result is stable.
func ThresholdString(thresholds map[string]string) string {
	keys := make([]string, 0, len(thresholds))
	for k := range thresholds {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := strings.TrimSpace(thresholds[k])
		if strings.HasPrefix(v, "<") || strings.HasPrefix(v, ">") {
			parts = append(parts, k+v)
			continue
		}
		op := "<"
		switch strings.ToLower(k) {
		case "rps", "rate":
			op = ">"
		}
		parts = append(parts, k+op+v)
	}
	return strings.Join(parts, ",")
}
