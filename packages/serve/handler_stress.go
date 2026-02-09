package serve

import (
	"context"
	"net/http"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/stress"
)

func (s *Server) handleStressStart(w http.ResponseWriter, r *http.Request) {
	var req StressStartReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	if s.stressRunner != nil {
		s.mu.Unlock()
		writeError(w, http.StatusConflict, "stress test already running")
		return
	}

	duration, err := time.ParseDuration(req.Duration)
	if err != nil {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, "invalid duration: "+err.Error())
		return
	}

	cfg := stress.DefaultConfig()
	cfg.Duration = duration
	if req.Rate > 0 {
		cfg.Rate = req.Rate
	}
	if req.VUs > 0 {
		cfg.VUs = req.VUs
		cfg.Mode = stress.VUMode
	}
	if req.MaxVUs > 0 {
		cfg.MaxVUs = req.MaxVUs
	}

	stressRunner := stress.NewRunner(cfg)
	if err := stressRunner.LoadFiles(req.Files); err != nil {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithCancel(s.ctx)
	s.stressRunner = stressRunner
	s.stressCancel = cancel
	s.mu.Unlock()

	// Run stress test in background, broadcast metrics via WebSocket
	go func() {
		// Broadcast metrics periodically
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = stressRunner.Run(ctx)
		}()

		for {
			select {
			case <-done:
				stats := stressRunner.GetCurrentStats()
				s.hub.Broadcast("stress:completed", WSStressMetrics{
					Stats:     convertStressStats(stats),
					Elapsed:   stats.Elapsed.Seconds(),
					Timestamp: nowISO(),
				})

				s.mu.Lock()
				s.stressRunner = nil
				s.stressCancel = nil
				s.mu.Unlock()
				return

			case <-ticker.C:
				stats := stressRunner.GetCurrentStats()
				s.hub.Broadcast("stress:metrics", WSStressMetrics{
					Stats:     convertStressStats(stats),
					Elapsed:   stats.Elapsed.Seconds(),
					Timestamp: nowISO(),
				})
			}
		}
	}()

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (s *Server) handleStressStop(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stressCancel == nil {
		writeError(w, http.StatusBadRequest, "no stress test running")
		return
	}

	s.stressCancel()
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopping"})
}

func (s *Server) handleStressStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	runner := s.stressRunner
	s.mu.Unlock()

	if runner == nil {
		writeJSON(w, http.StatusOK, StressStatusDTO{Running: false})
		return
	}

	stats := runner.GetCurrentStats()
	dto := StressStatusDTO{
		Running: true,
		Elapsed: stats.Elapsed.Seconds(),
		Stats:   ptrStressStats(convertStressStats(stats)),
	}

	writeJSON(w, http.StatusOK, dto)
}

func convertStressStats(s stress.CurrentStats) StressStatsDTO {
	return StressStatsDTO{
		Total:     s.Total,
		Success:   s.Success,
		Errors:    s.Errors,
		RPS:       s.RPS,
		P50Ms:     float64(s.P50.Milliseconds()),
		P95Ms:     float64(s.P95.Milliseconds()),
		P99Ms:     float64(s.P99.Milliseconds()),
		MaxMs:     float64(s.Max.Milliseconds()),
		ErrorRate: s.ErrorRate,
		ActiveVUs: s.ActiveVUs,
	}
}

func ptrStressStats(s StressStatsDTO) *StressStatsDTO {
	return &s
}
