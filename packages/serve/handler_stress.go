package serve

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/config"
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

	// Validate all file paths are within the workspace
	absFiles := make([]string, 0, len(req.Files))
	for _, f := range req.Files {
		absPath := filepath.Join(s.config.WorkDir, f)
		if !isPathWithin(s.config.WorkDir, absPath) {
			s.mu.Unlock()
			writeError(w, http.StatusForbidden, "path outside workspace: "+f)
			return
		}
		absFiles = append(absFiles, absPath)
	}

	stressRunner := stress.NewRunner(cfg)
	if err := stressRunner.LoadFiles(absFiles); err != nil {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithCancel(s.ctx)
	s.stressRunner = stressRunner
	s.stressCancel = cancel
	s.mu.Unlock()

	s.logger.Info("stress test starting", "duration", req.Duration, "rate", req.Rate, "vus", req.VUs)

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
				resultDTO := convertStressResult(stressRunner)

				s.hub.Broadcast("stress_update", WSStressMetrics{
					Running:   false,
					Completed: true,
					Stats:     convertStressStats(stats),
					Elapsed:   stats.Elapsed.Seconds(),
					Timestamp: nowISO(),
				})

				s.mu.Lock()
				s.lastStressResult = resultDTO
				s.stressRunner = nil
				s.stressCancel = nil
				s.mu.Unlock()
				return

			case <-ticker.C:
				stats := stressRunner.GetCurrentStats()
				s.hub.Broadcast("stress_update", WSStressMetrics{
					Running:   true,
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
		writeJSON(w, http.StatusOK, map[string]string{"status": "already_stopped"})
		return
	}

	s.logger.Info("stress test stopping")
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

func (s *Server) handleStressProfiles(w http.ResponseWriter, r *http.Request) {
	type ProfileDTO struct {
		Name       string            `json:"name"`
		Duration   string            `json:"duration,omitempty"`
		Rate       float64           `json:"rate,omitempty"`
		VUs        int               `json:"vus,omitempty"`
		MaxVUs     int               `json:"maxVUs,omitempty"`
		ThinkTime  string            `json:"thinkTime,omitempty"`
		RampUp     string            `json:"rampUp,omitempty"`
		Thresholds map[string]string `json:"thresholds,omitempty"`
	}

	var profiles []ProfileDTO

	if s.fileConfig != nil && s.fileConfig.Stress != nil && s.fileConfig.Stress.Profiles != nil {
		for name, p := range s.fileConfig.Stress.Profiles {
			profiles = append(profiles, ProfileDTO{
				Name:       name,
				Duration:   p.Duration,
				Rate:       p.Rate,
				VUs:        p.VUs,
				MaxVUs:     p.MaxVUs,
				ThinkTime:  p.ThinkTime,
				RampUp:     p.RampUp,
				Thresholds: p.Thresholds,
			})
		}
	}

	if profiles == nil {
		profiles = []ProfileDTO{}
	}

	writeJSON(w, http.StatusOK, profiles)
}

// saveConfig persists fileConfig to the resolved config file path.
func (s *Server) saveConfig(w http.ResponseWriter) bool {
	if s.configPath == "" {
		s.configPath = filepath.Join(s.config.WorkDir, "hitspec.yaml")
	}
	if err := s.fileConfig.SaveConfig(s.configPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return false
	}
	return true
}

func (s *Server) handleCreateStressProfile(w http.ResponseWriter, r *http.Request) {
	var req StressProfileReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "profile name is required")
		return
	}

	if s.fileConfig == nil {
		writeError(w, http.StatusBadRequest, "no config file found")
		return
	}

	if s.fileConfig.Stress == nil {
		s.fileConfig.Stress = &config.StressConfig{}
	}
	if s.fileConfig.Stress.Profiles == nil {
		s.fileConfig.Stress.Profiles = make(map[string]*config.StressProfile)
	}

	if _, exists := s.fileConfig.Stress.Profiles[req.Name]; exists {
		writeError(w, http.StatusConflict, "profile already exists: "+req.Name)
		return
	}

	s.fileConfig.Stress.Profiles[req.Name] = &config.StressProfile{
		Duration:   req.Duration,
		Rate:       req.Rate,
		VUs:        req.VUs,
		MaxVUs:     req.MaxVUs,
		ThinkTime:  req.ThinkTime,
		RampUp:     req.RampUp,
		Thresholds: req.Thresholds,
	}

	if !s.saveConfig(w) {
		return
	}

	writeJSON(w, http.StatusCreated, req)
}

func (s *Server) handleUpdateStressProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "profile name is required")
		return
	}

	var req StressProfileReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if s.fileConfig == nil || s.fileConfig.Stress == nil || s.fileConfig.Stress.Profiles == nil {
		writeError(w, http.StatusNotFound, "profile not found: "+name)
		return
	}

	profile, exists := s.fileConfig.Stress.Profiles[name]
	if !exists {
		writeError(w, http.StatusNotFound, "profile not found: "+name)
		return
	}

	profile.Duration = req.Duration
	profile.Rate = req.Rate
	profile.VUs = req.VUs
	profile.MaxVUs = req.MaxVUs
	profile.ThinkTime = req.ThinkTime
	profile.RampUp = req.RampUp
	profile.Thresholds = req.Thresholds

	if !s.saveConfig(w) {
		return
	}

	req.Name = name
	writeJSON(w, http.StatusOK, req)
}

func (s *Server) handleDeleteStressProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "profile name is required")
		return
	}

	if s.fileConfig == nil || s.fileConfig.Stress == nil || s.fileConfig.Stress.Profiles == nil {
		writeError(w, http.StatusNotFound, "profile not found: "+name)
		return
	}

	if _, exists := s.fileConfig.Stress.Profiles[name]; !exists {
		writeError(w, http.StatusNotFound, "profile not found: "+name)
		return
	}

	delete(s.fileConfig.Stress.Profiles, name)

	if !s.saveConfig(w) {
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStressResult(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	result := s.lastStressResult
	s.mu.Unlock()

	if result == nil {
		writeError(w, http.StatusNotFound, "no stress test result available")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func convertStressResult(runner *stress.Runner) *StressResultDTO {
	summary := runner.GetSummary()
	if summary == nil {
		return nil
	}

	dto := &StressResultDTO{
		DurationMs:  float64(summary.Duration.Milliseconds()),
		Total:       summary.TotalRequests,
		Success:     summary.SuccessCount,
		Errors:      summary.ErrorCount,
		Timeouts:    summary.TimeoutCount,
		RPS:         summary.RPS,
		SuccessRate: summary.SuccessRate,
		ErrorRate:   summary.ErrorRate,
		P50Ms:       float64(summary.P50.Microseconds()) / 1000.0,
		P95Ms:       float64(summary.P95.Microseconds()) / 1000.0,
		P99Ms:       float64(summary.P99.Microseconds()) / 1000.0,
		MinMs:       float64(summary.Min.Microseconds()) / 1000.0,
		MaxMs:       float64(summary.Max.Microseconds()) / 1000.0,
		MeanMs:      float64(summary.Mean.Microseconds()) / 1000.0,
		StdDevMs:    float64(summary.StdDev.Microseconds()) / 1000.0,
		Timestamp:   nowISO(),
	}

	// Per-request breakdown
	for _, rs := range summary.RequestBreakdown {
		dto.Breakdown = append(dto.Breakdown, StressRequestBreakdownDTO{
			Name:    rs.Name,
			Total:   rs.Total,
			Success: rs.Success,
			Errors:  rs.Errors,
			P50Ms:   float64(rs.P50.Microseconds()) / 1000.0,
			P95Ms:   float64(rs.P95.Microseconds()) / 1000.0,
			P99Ms:   float64(rs.P99.Microseconds()) / 1000.0,
			MeanMs:  float64(rs.Mean.Microseconds()) / 1000.0,
		})
	}
	if dto.Breakdown == nil {
		dto.Breakdown = []StressRequestBreakdownDTO{}
	}

	// Time series
	for _, tp := range summary.TimeSeries {
		dto.TimeSeries = append(dto.TimeSeries, StressTimePointDTO{
			Timestamp: tp.Timestamp.UTC().Format("2006-01-02T15:04:05.000Z"),
			Requests:  tp.Requests,
			Errors:    tp.Errors,
			P50Ms:     float64(tp.P50.Microseconds()) / 1000.0,
			P95Ms:     float64(tp.P95.Microseconds()) / 1000.0,
			P99Ms:     float64(tp.P99.Microseconds()) / 1000.0,
			RPS:       tp.RPS,
			ActiveVUs: tp.ActiveVUs,
		})
	}
	if dto.TimeSeries == nil {
		dto.TimeSeries = []StressTimePointDTO{}
	}

	return dto
}

func convertStressStats(s stress.CurrentStats) StressStatsDTO {
	return StressStatsDTO{
		Total:     s.Total,
		Success:   s.Success,
		Errors:    s.Errors,
		RPS:       s.RPS,
		P50Ms:     float64(s.P50.Microseconds()) / 1000.0,
		P95Ms:     float64(s.P95.Microseconds()) / 1000.0,
		P99Ms:     float64(s.P99.Microseconds()) / 1000.0,
		MaxMs:     float64(s.Max.Microseconds()) / 1000.0,
		ErrorRate: s.ErrorRate,
		ActiveVUs: s.ActiveVUs,
	}
}

func ptrStressStats(s StressStatsDTO) *StressStatsDTO {
	return &s
}
