package serve

import (
	"net/http"
	"path/filepath"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runconfig"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
	hitspechttp "github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

// runSettings resolves hitspec.yaml (as currently held, including edits made
// through PUT /config), the HITSPEC_* variables and the active environment into
// the runner configuration and the matching HTTP client options. Callers add
// their filters and progress callback. The result does not alias the live
// config, so it stays valid after the lock is released.
func (s *Server) runSettings(environment string) (*runner.Config, []hitspechttp.ClientOption) {
	overrides, err := runconfig.FromEnv()
	if err != nil {
		s.logger.Warn("ignoring invalid environment setting", "error", err)
	}
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	overrides.Environment = s.config.Env
	if environment != "" {
		overrides.Environment = environment
	}
	cfg, clientOpts := runconfig.Build(s.fileConfig, overrides)
	cfg.AllowShell = s.config.AllowShell
	cfg.AllowDB = s.config.AllowDB
	// A `< file` body must not read outside the workspace the API serves.
	cfg.FileRoot = s.config.WorkDir
	return cfg, clientOpts
}

func (s *Server) handleExecuteRequest(w http.ResponseWriter, r *http.Request) {
	clearWriteDeadline(w) // a run lasts as long as the API it calls
	var req ExecuteReq
	if err := readJSON(r, &req); err != nil {
		writeReadError(w, err)
		return
	}

	if req.File == "" {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}

	absPath := filepath.Join(s.config.WorkDir, req.File)
	if !isPathWithin(s.config.WorkDir, absPath) {
		writeError(w, http.StatusForbidden, "path outside workspace")
		return
	}

	cfg, _ := s.runSettings(req.Environment)
	env := cfg.Environment

	execID := generateID()
	s.logger.Info("execution started", "exec_id", execID, "file", req.File, "request_name", req.RequestName, "environment", env)

	s.hub.Broadcast("execution_start", WSExecEvent{
		ID:        execID,
		File:      req.File,
		Status:    "started",
		Timestamp: nowISO(),
	})

	cfg.NameFilter = req.RequestName

	cfg.OnProgress = func(event runner.ProgressEvent) {
		progress := WSRequestProgress{
			ExecID:      execID,
			File:        req.File,
			RequestName: event.RequestName,
			Status:      event.Status,
			Index:       event.Index,
			Total:       event.Total,
			Timestamp:   nowISO(),
		}
		if event.Status == "completed" && event.Result != nil {
			progress.Passed = event.Result.Passed
			progress.Duration = float64(event.Result.Duration.Milliseconds())
		}
		s.hub.Broadcast("request_progress", progress)
	}

	rn := runner.NewRunner(cfg)
	result, err := rn.RunFileContext(r.Context(), absPath)
	if err != nil {
		s.logger.Error("execution failed", "exec_id", execID, "file", req.File, "error", err)
		s.hub.Broadcast("error", WSExecEvent{
			ID:        execID,
			File:      req.File,
			Status:    "error",
			Error:     err.Error(),
			Timestamp: nowISO(),
		})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Strip "filtered out" stubs — API returns only executed requests
	if cfg.NameFilter != "" {
		executed := make([]*runner.RequestResult, 0, len(result.Results))
		for _, rr := range result.Results {
			if rr.Skipped && rr.SkipReason == "filtered out" {
				continue
			}
			executed = append(executed, rr)
		}
		result.Results = executed
		result.Skipped = 0
		for _, rr := range executed {
			if rr.Skipped {
				result.Skipped++
			}
		}
	}

	dto := convertRunResult(result)
	s.hub.Broadcast("execution_complete", WSExecEvent{
		ID:        execID,
		File:      req.File,
		Status:    "completed",
		Result:    dto,
		Timestamp: nowISO(),
	})

	// Add to in-memory history
	for _, rr := range result.Results {
		entry := HistoryEntryDTO{
			ID:          generateID(),
			File:        req.File,
			RequestName: rr.Name,
			Duration:    float64(rr.Duration.Milliseconds()),
			Passed:      rr.Passed,
			Timestamp:   nowISO(),
		}
		if rr.Request != nil {
			entry.Method = rr.Request.Method
			entry.URL = rr.Request.URL
		}
		if rr.Response != nil {
			entry.StatusCode = rr.Response.StatusCode
		}
		s.history.Add(entry)
	}

	// Record to persistent history (committed before we respond)
	s.recordRunToHistory(r.Context(), req.File, env, dto)

	s.logger.Info("execution completed", "exec_id", execID, "file", req.File, "duration_ms", result.Duration.Milliseconds(), "passed", result.Passed, "failed", result.Failed, "skipped", result.Skipped)

	writeJSON(w, http.StatusOK, dto)
}

func (s *Server) handleRunFile(w http.ResponseWriter, r *http.Request) {
	clearWriteDeadline(w) // a run lasts as long as the API it calls
	var req RunReq
	if err := readJSON(r, &req); err != nil {
		writeReadError(w, err)
		return
	}

	if req.File == "" {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}

	absPath := filepath.Join(s.config.WorkDir, req.File)
	if !isPathWithin(s.config.WorkDir, absPath) {
		writeError(w, http.StatusForbidden, "path outside workspace")
		return
	}

	cfg, _ := s.runSettings(req.Environment)
	env := cfg.Environment

	execID := generateID()

	cfg.OnProgress = func(event runner.ProgressEvent) {
		progress := WSRequestProgress{
			ExecID:      execID,
			File:        req.File,
			RequestName: event.RequestName,
			Status:      event.Status,
			Index:       event.Index,
			Total:       event.Total,
			Timestamp:   nowISO(),
		}
		if event.Status == "completed" && event.Result != nil {
			progress.Passed = event.Result.Passed
			progress.Duration = float64(event.Result.Duration.Milliseconds())
		}
		s.hub.Broadcast("request_progress", progress)
	}

	s.logger.Info("run file started", "exec_id", execID, "file", req.File, "environment", env)

	s.hub.Broadcast("execution_start", WSExecEvent{
		ID:        execID,
		File:      req.File,
		Status:    "started",
		Timestamp: nowISO(),
	})

	rn := runner.NewRunner(cfg)
	result, err := rn.RunFileContext(r.Context(), absPath)
	if err != nil {
		s.logger.Error("run file failed", "exec_id", execID, "file", req.File, "error", err)
		s.hub.Broadcast("error", WSExecEvent{
			ID:        execID,
			File:      req.File,
			Status:    "error",
			Error:     err.Error(),
			Timestamp: nowISO(),
		})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	dto := convertRunResult(result)
	s.hub.Broadcast("execution_complete", WSExecEvent{
		ID:        execID,
		File:      req.File,
		Status:    "completed",
		Result:    dto,
		Timestamp: nowISO(),
	})

	// Add to in-memory history
	for _, rr := range result.Results {
		entry := HistoryEntryDTO{
			ID:          generateID(),
			File:        req.File,
			RequestName: rr.Name,
			Duration:    float64(rr.Duration.Milliseconds()),
			Passed:      rr.Passed,
			Timestamp:   nowISO(),
		}
		if rr.Request != nil {
			entry.Method = rr.Request.Method
			entry.URL = rr.Request.URL
		}
		if rr.Response != nil {
			entry.StatusCode = rr.Response.StatusCode
		}
		s.history.Add(entry)
	}

	// Record to persistent history (committed before we respond)
	s.recordRunToHistory(r.Context(), req.File, env, dto)

	s.logger.Info("run file completed", "exec_id", execID, "file", req.File, "duration_ms", result.Duration.Milliseconds(), "passed", result.Passed, "failed", result.Failed, "skipped", result.Skipped)

	writeJSON(w, http.StatusOK, dto)
}
