package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/stress"
)

// A profile threshold keeps its operator: "rps": ">=100" must reach the
// parser as rps>=100, not be rewritten to rps<100.
func TestBuildThresholdStringKeepsOperators(t *testing.T) {
	cases := []struct {
		key, value, want string
	}{
		{"p95", "<200ms", "p95<200ms"},
		{"p95", "<=200ms", "p95<=200ms"},
		{"p99", "200ms", "p99<200ms"}, // bare value: the metric's natural operator
		{"errors", "<1%", "errors<1%"},
		{"errors", "<=0", "errors<=0"},
		{"errors", "0%", "errors<0%"},
		{"rps", ">100", "rps>100"},
		{"rps", ">=100", "rps>=100"},
		{"rps", "45", "rps>45"}, // the documented bare form
		{"assertions", "<=0%", "assertions<=0%"},
	}
	for _, c := range cases {
		got := buildThresholdString(map[string]string{c.key: c.value})
		assert.Equal(t, c.want, got, "%s: %s", c.key, c.value)
		_, err := stress.ParseThresholds(got)
		assert.NoError(t, err, got)
	}

	// Every operator the parser accepts survives the round trip.
	thr, err := stress.ParseThresholds(buildThresholdString(map[string]string{
		"p95": "<=200ms", "rps": ">=100", "errors": "<=0",
	}))
	require.NoError(t, err)
	assert.Equal(t, 200*time.Millisecond, thr.P95)
	assert.Equal(t, 100.0, thr.MinRPS)
	assert.True(t, thr.ErrorRateSet)

	// The wrong direction is a parse error, never a silent flip.
	_, err = stress.ParseThresholds(buildThresholdString(map[string]string{"rps": "<100"}))
	assert.Error(t, err)
	_, err = stress.ParseThresholds(buildThresholdString(map[string]string{"p95": ">200ms"}))
	assert.Error(t, err)

	// Multiple keys come out in a stable order.
	assert.Equal(t, "errors<1%,p95<200ms,rps>10", buildThresholdString(map[string]string{"rps": "10", "p95": "200ms", "errors": "1%"}))
}

// End to end through buildStressConfig: operators and a zero error budget in a
// profile reach the runner's thresholds.
func TestBuildStressConfigProfileThresholds(t *testing.T) {
	resetRootCommandForTest(t)
	fc := stressProfileConfig()
	fc.Stress.Profiles["load"].Thresholds = map[string]string{"rps": ">=100", "errors": "<=0", "p95": "<=250ms"}

	require.NoError(t, runCmd.Flags().Parse([]string{"--stress", "--profile", "load"}))
	cfg, err := buildStressConfig(runCmd, fc)
	require.NoError(t, err)
	assert.Equal(t, 100.0, cfg.Thresholds.MinRPS)
	assert.True(t, cfg.Thresholds.ErrorRateSet)
	assert.Equal(t, 250*time.Millisecond, cfg.Thresholds.P95)
}

func TestPromDisplayAddr(t *testing.T) {
	assert.Equal(t, "localhost:9090", promDisplayAddr("127.0.0.1", 9090))
	assert.Equal(t, "localhost:9090", promDisplayAddr("0.0.0.0", 9090))
	assert.Equal(t, "10.1.2.3:9090", promDisplayAddr("10.1.2.3", 9090))
}
