package stress

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, RateMode, cfg.Mode)
	assert.Equal(t, 30*time.Second, cfg.Duration)
	assert.Equal(t, float64(10), cfg.Rate)
	assert.Equal(t, 100, cfg.MaxVUs)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid rate mode config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "valid VU mode config",
			config: &Config{
				Mode:     VUMode,
				Duration: 30 * time.Second,
				VUs:      10,
				MaxVUs:   100,
			},
			wantErr: false,
		},
		{
			name: "invalid duration",
			config: &Config{
				Mode:     RateMode,
				Duration: 0,
				Rate:     10,
				MaxVUs:   100,
			},
			wantErr: true,
		},
		{
			name: "invalid rate in rate mode",
			config: &Config{
				Mode:     RateMode,
				Duration: 30 * time.Second,
				Rate:     0,
				MaxVUs:   100,
			},
			wantErr: true,
		},
		{
			name: "invalid VUs in VU mode",
			config: &Config{
				Mode:     VUMode,
				Duration: 30 * time.Second,
				VUs:      0,
				MaxVUs:   100,
			},
			wantErr: true,
		},
		{
			name: "invalid maxVUs",
			config: &Config{
				Mode:     RateMode,
				Duration: 30 * time.Second,
				Rate:     10,
				MaxVUs:   0,
			},
			wantErr: true,
		},
		{
			name: "rampUp exceeds duration",
			config: &Config{
				Mode:     RateMode,
				Duration: 30 * time.Second,
				Rate:     10,
				MaxVUs:   100,
				RampUp:   60 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseThresholds(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Thresholds
		wantErr  bool
	}{
		{
			name:  "p95 threshold",
			input: "p95<200ms",
			expected: Thresholds{
				P95: 200 * time.Millisecond,
			},
		},
		{
			name:  "p99 threshold",
			input: "p99<500ms",
			expected: Thresholds{
				P99: 500 * time.Millisecond,
			},
		},
		{
			name:  "error rate percentage",
			input: "errors<1%",
			expected: Thresholds{
				ErrorRate: 0.01,
			},
		},
		{
			name:  "error rate decimal",
			input: "errors<0.001",
			expected: Thresholds{
				ErrorRate: 0.001,
			},
		},
		{
			name:  "multiple thresholds",
			input: "p95<200ms,errors<0.1%",
			expected: Thresholds{
				P95:       200 * time.Millisecond,
				ErrorRate: 0.001,
			},
		},
		{
			name:  "with spaces",
			input: "p95 < 200ms, errors < 1%",
			expected: Thresholds{
				P95:       200 * time.Millisecond,
				ErrorRate: 0.01,
			},
		},
		{
			name:  "rps threshold",
			input: "rps>50",
			expected: Thresholds{
				MinRPS: 50,
			},
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "invalid metric",
			input:   "unknown<100",
			wantErr: true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: Thresholds{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseThresholds(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected.P50, result.P50)
				assert.Equal(t, tt.expected.P95, result.P95)
				assert.Equal(t, tt.expected.P99, result.P99)
				assert.InDelta(t, tt.expected.ErrorRate, result.ErrorRate, 0.0001)
				assert.Equal(t, tt.expected.MinRPS, result.MinRPS)
			}
		})
	}
}

func TestThresholdsHasThresholds(t *testing.T) {
	tests := []struct {
		name       string
		thresholds Thresholds
		expected   bool
	}{
		{
			name:       "empty thresholds",
			thresholds: Thresholds{},
			expected:   false,
		},
		{
			name: "with p95",
			thresholds: Thresholds{
				P95: 200 * time.Millisecond,
			},
			expected: true,
		},
		{
			name: "with error rate",
			thresholds: Thresholds{
				ErrorRate: 0.01,
			},
			expected: true,
		},
		{
			name: "with min RPS",
			thresholds: Thresholds{
				MinRPS: 50,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.thresholds.HasThresholds())
		})
	}
}

func TestDefaultRequestConfig(t *testing.T) {
	cfg := DefaultRequestConfig()

	assert.Equal(t, 1, cfg.Weight)
	assert.Equal(t, 0, cfg.Think)
	assert.False(t, cfg.Skip)
	assert.False(t, cfg.Setup)
	assert.False(t, cfg.Teardown)
}
