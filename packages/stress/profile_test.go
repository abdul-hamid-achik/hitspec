package stress

import (
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
)

func TestApplyProfile(t *testing.T) {
	cfg := DefaultConfig()
	err := ApplyProfile(cfg, &config.StressProfile{
		Duration: "5s", Rate: 50, MaxVUs: 20, ThinkTime: "100ms", RampUp: "2s",
		Thresholds: map[string]string{"rps": "10", "p95": "200ms", "errors": "<=1%"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Duration != 5*time.Second || cfg.Rate != 50 || cfg.MaxVUs != 20 || cfg.ThinkTime != 100*time.Millisecond || cfg.RampUp != 2*time.Second {
		t.Errorf("cfg = %+v", cfg)
	}
	if cfg.Thresholds.MinRPS != 10 || cfg.Thresholds.P95 != 200*time.Millisecond || !cfg.Thresholds.ErrorRateSet || cfg.Thresholds.ErrorRate != 0.01 {
		t.Errorf("thresholds = %+v", cfg.Thresholds)
	}

	// VUs switch to VU mode; an empty profile changes nothing.
	vu := DefaultConfig()
	if err := ApplyProfile(vu, &config.StressProfile{VUs: 7}); err != nil || vu.Mode != VUMode || vu.VUs != 7 {
		t.Errorf("vus profile: %+v, %v", vu, err)
	}
	if err := ApplyProfile(DefaultConfig(), nil); err != nil {
		t.Error(err)
	}

	for name, p := range map[string]*config.StressProfile{
		"duration":   {Duration: "soon"},
		"thresholds": {Thresholds: map[string]string{"rps": "<5"}},
	} {
		if err := ApplyProfile(DefaultConfig(), p); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
