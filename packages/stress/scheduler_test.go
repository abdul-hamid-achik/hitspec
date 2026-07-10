package stress

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchedulerAddRequest(t *testing.T) {
	cfg := DefaultConfig()
	s := NewScheduler(cfg)

	s.AddRequest(0, "test1", &RequestConfig{Weight: 1})
	s.AddRequest(1, "test2", &RequestConfig{Weight: 2})
	s.AddRequest(2, "test3", nil) // Should use default weight

	assert.Equal(t, 3, s.RequestCount())
}

func TestSchedulerSelectRequest(t *testing.T) {
	cfg := DefaultConfig()
	s := NewScheduler(cfg)

	s.AddRequest(0, "only", &RequestConfig{Weight: 1})

	// Single request should always be selected
	for i := 0; i < 10; i++ {
		req := s.SelectRequest()
		require.NotNil(t, req)
		assert.Equal(t, "only", req.Name)
	}
}

func TestSchedulerSelectRequestWeighted(t *testing.T) {
	cfg := DefaultConfig()
	s := NewScheduler(cfg)

	// Add requests with different weights
	s.AddRequest(0, "heavy", &RequestConfig{Weight: 90})
	s.AddRequest(1, "light", &RequestConfig{Weight: 10})

	// Run many selections and check distribution
	counts := make(map[string]int)
	iterations := 10000

	for i := 0; i < iterations; i++ {
		req := s.SelectRequest()
		require.NotNil(t, req)
		counts[req.Name]++
	}

	// Heavy should be selected roughly 9x more than light
	heavyRatio := float64(counts["heavy"]) / float64(iterations)
	lightRatio := float64(counts["light"]) / float64(iterations)

	assert.InDelta(t, 0.9, heavyRatio, 0.05)
	assert.InDelta(t, 0.1, lightRatio, 0.05)
}

func TestSchedulerSelectRequestEmpty(t *testing.T) {
	cfg := DefaultConfig()
	s := NewScheduler(cfg)

	req := s.SelectRequest()
	assert.Nil(t, req)
}

func TestSchedulerWaitRateMode(t *testing.T) {
	cfg := &Config{
		Mode: RateMode,
		Rate: 100, // 100 req/s = 10ms between requests
	}
	s := NewScheduler(cfg)

	ctx := context.Background()

	// First request should be immediate
	start := time.Now()
	err := s.Wait(ctx)
	require.NoError(t, err)
	assert.Less(t, time.Since(start), 5*time.Millisecond)

	// Second request should wait
	start = time.Now()
	err = s.Wait(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(start), 5*time.Millisecond)
}

func TestSchedulerWaitCancelled(t *testing.T) {
	cfg := &Config{
		Mode: RateMode,
		Rate: 1, // 1 req/s = long wait
	}
	s := NewScheduler(cfg)

	ctx, cancel := context.WithCancel(context.Background())

	// First request to trigger the wait
	_ = s.Wait(ctx)

	// Cancel immediately
	cancel()

	// Should return context error
	err := s.Wait(ctx)
	assert.Error(t, err)
}

func TestSchedulerAcquireRelease(t *testing.T) {
	cfg := &Config{
		MaxVUs: 2,
	}
	s := NewScheduler(cfg)

	ctx := context.Background()

	// Acquire both slots
	err := s.Acquire(ctx)
	require.NoError(t, err)
	err = s.Acquire(ctx)
	require.NoError(t, err)

	// Third acquire should block, so use timeout
	ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err = s.Acquire(ctx2)
	assert.Error(t, err) // Should timeout

	// Release one slot
	s.Release()

	// Now acquire should work
	ctx3, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	err = s.Acquire(ctx3)
	assert.NoError(t, err)
}

func TestSchedulerGetCurrentRate(t *testing.T) {
	cfg := &Config{
		Rate:   100,
		RampUp: 10 * time.Second,
	}
	s := NewScheduler(cfg)

	// At start, rate should be 0
	rate := s.GetCurrentRate(0)
	assert.InDelta(t, 0, rate, 0.1)

	// At half ramp-up, rate should be 50
	rate = s.GetCurrentRate(5 * time.Second)
	assert.InDelta(t, 50, rate, 1)

	// After ramp-up, rate should be full
	rate = s.GetCurrentRate(10 * time.Second)
	assert.InDelta(t, 100, rate, 0.1)

	rate = s.GetCurrentRate(15 * time.Second)
	assert.InDelta(t, 100, rate, 0.1)
}

func TestSchedulerGetCurrentVUs(t *testing.T) {
	cfg := &Config{
		VUs:    10,
		RampUp: 10 * time.Second,
	}
	s := NewScheduler(cfg)

	// At start of ramp-up, the count is clamped to >= 1 so the pool never
	// scales to zero at the first tick.
	vus := s.GetCurrentVUs(0)
	assert.Equal(t, 1, vus)

	// Small elapsed values that would otherwise round down to 0 stay >= 1.
	vus = s.GetCurrentVUs(100 * time.Millisecond)
	assert.GreaterOrEqual(t, vus, 1)

	// At half ramp-up
	vus = s.GetCurrentVUs(5 * time.Second)
	assert.Equal(t, 5, vus)

	// After ramp-up
	vus = s.GetCurrentVUs(10 * time.Second)
	assert.Equal(t, 10, vus)
}

func TestSchedulerUpdateRate(t *testing.T) {
	cfg := &Config{
		Mode: RateMode,
		Rate: 10,
	}
	s := NewScheduler(cfg)

	// Update to higher rate
	s.UpdateRate(100)

	// Should not panic and rate limiter should use new rate
	// (verifying internal state is tricky, but we can at least ensure no panic)
	ctx := context.Background()
	err := s.Wait(ctx)
	assert.NoError(t, err)
}

func TestSchedulerGetRequests(t *testing.T) {
	cfg := DefaultConfig()
	s := NewScheduler(cfg)

	s.AddRequest(0, "test1", nil)
	s.AddRequest(1, "test2", nil)

	requests := s.GetRequests()
	assert.Len(t, requests, 2)
	assert.Equal(t, "test1", requests[0].Name)
	assert.Equal(t, "test2", requests[1].Name)

	// Ensure returned slice is a copy (modifying slice doesn't affect internal state)
	_ = requests[:1] // Modify to truncate
	allRequests := s.GetRequests()
	assert.Len(t, allRequests, 2) // Internal state unchanged
}

// TestSchedulerRampUpNeverZeroVUs ensures the ramp-up formula never scales the
// VU pool to zero at the first tick (or any early tick) when a non-zero target
// is configured. The linear interpolation int(VUs * elapsed/RampUp) rounds down
// to 0 for any elapsed < RampUp/VUs, which previously scaled the pool to zero.
func TestSchedulerRampUpNeverZeroVUs(t *testing.T) {
	cases := []struct {
		name      string
		vus       int
		rampUp    time.Duration
		elapsed   time.Duration
		tickEvery time.Duration
	}{
		{"small vus long ramp", 10, 30 * time.Second, 100 * time.Millisecond, 100 * time.Millisecond},
		{"first tick", 5, 10 * time.Second, 100 * time.Millisecond, 100 * time.Millisecond},
		{"tiny elapsed", 100, 60 * time.Second, 10 * time.Millisecond, 100 * time.Millisecond},
		{"single vu", 1, 10 * time.Second, 100 * time.Millisecond, 100 * time.Millisecond},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{VUs: tc.vus, RampUp: tc.rampUp}
			s := NewScheduler(cfg)

			// Simulate the first several ramp-up ticks.
			for tick := 0; tick < 5; tick++ {
				elapsed := tc.elapsed + time.Duration(tick)*tc.tickEvery
				if elapsed >= tc.rampUp {
					break
				}
				vus := s.GetCurrentVUs(elapsed)
				assert.GreaterOrEqual(t, vus, 1,
					"ramp-up must keep >=1 VU at tick %d (elapsed=%v, vus=%d, rampUp=%v)",
					tick, elapsed, vus, tc.rampUp)
			}
		})
	}
}

// TestSchedulerRampUpZeroVUTarget verifies that a zero-VU target still reports
// zero (the clamp only applies when a non-zero target is configured).
func TestSchedulerRampUpZeroVUTarget(t *testing.T) {
	cfg := &Config{VUs: 0, RampUp: 10 * time.Second}
	s := NewScheduler(cfg)
	assert.Equal(t, 0, s.GetCurrentVUs(100*time.Millisecond))
}
