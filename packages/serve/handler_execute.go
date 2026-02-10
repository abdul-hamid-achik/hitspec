package serve

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
)

func (s *Server) handleExecuteRequest(w http.ResponseWriter, r *http.Request) {
	var req ExecuteReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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

	env := s.config.Env
	if req.Environment != "" {
		env = req.Environment
	}

	execID := generateID()
	s.hub.Broadcast("execution_start", WSExecEvent{
		ID:        execID,
		File:      req.File,
		Status:    "started",
		Timestamp: nowISO(),
	})

	cfg := &runner.Config{
		Environment:        env,
		Timeout:            30 * time.Second,
		FollowRedirect:     true,
		ValidateSSL:        true,
		NameFilter:         req.RequestName,
		ConfigEnvironments: s.getConfigEnvs(),
		AllowShell:         s.config.AllowShell,
		AllowDB:            s.config.AllowDB,
	}

	rn := runner.NewRunner(cfg)
	result, err := rn.RunFile(absPath)
	if err != nil {
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

	// Add to history
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

	writeJSON(w, http.StatusOK, dto)
}

func (s *Server) handleRunFile(w http.ResponseWriter, r *http.Request) {
	var req RunReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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

	env := s.config.Env
	if req.Environment != "" {
		env = req.Environment
	}

	cfg := &runner.Config{
		Environment:        env,
		Timeout:            30 * time.Second,
		FollowRedirect:     true,
		ValidateSSL:        true,
		ConfigEnvironments: s.getConfigEnvs(),
		AllowShell:         s.config.AllowShell,
		AllowDB:            s.config.AllowDB,
	}

	execID := generateID()
	s.hub.Broadcast("execution_start", WSExecEvent{
		ID:        execID,
		File:      req.File,
		Status:    "started",
		Timestamp: nowISO(),
	})

	rn := runner.NewRunner(cfg)
	result, err := rn.RunFile(absPath)
	if err != nil {
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

	// Add to history
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

	writeJSON(w, http.StatusOK, dto)
}

func (s *Server) getConfigEnvs() map[string]map[string]any {
	if s.fileConfig != nil {
		return s.fileConfig.Environments
	}
	return nil
}
