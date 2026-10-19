package cmd

import (
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stressProfileConfig() *config.Config {
	return &config.Config{Stress: &config.StressConfig{Profiles: map[string]*config.StressProfile{
		"load": {Duration: "5s", Rate: 50, MaxVUs: 20, RampUp: "2s"},
	}}}
}

// A profile in hitspec.yaml must not be masked by flag defaults, and a flag
// passed explicitly must win even when it equals the flag's default.
func TestBuildStressConfig_ProfileVersusFlags(t *testing.T) {
	resetRootCommandForTest(t)

	require.NoError(t, runCmd.Flags().Parse([]string{"--stress", "--profile", "load"}))
	cfg, err := buildStressConfig(runCmd, stressProfileConfig())
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, cfg.Duration)
	assert.Equal(t, 50.0, cfg.Rate)
	assert.Equal(t, 20, cfg.MaxVUs)
	assert.Equal(t, 2*time.Second, cfg.RampUp)

	// Values equal to the flag defaults, but passed explicitly.
	resetRootCommandForTest(t)
	require.NoError(t, runCmd.Flags().Parse([]string{"--stress", "--profile", "load", "--rate", "10", "--duration", "30s", "--max-vus", "100", "--ramp-up", "0s"}))
	cfg, err = buildStressConfig(runCmd, stressProfileConfig())
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, cfg.Duration)
	assert.Equal(t, 10.0, cfg.Rate)
	assert.Equal(t, 100, cfg.MaxVUs)
	assert.Equal(t, time.Duration(0), cfg.RampUp)
}

func TestBuildStressConfig_DefaultsWithoutProfile(t *testing.T) {
	resetRootCommandForTest(t)
	require.NoError(t, runCmd.Flags().Parse([]string{"--stress"}))
	cfg, err := buildStressConfig(runCmd, nil)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, cfg.Duration)
	assert.Equal(t, 10.0, cfg.Rate)
	assert.Equal(t, 100, cfg.MaxVUs)
}
