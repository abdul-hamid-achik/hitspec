package serve

import (
	"context"
	"net/http"

	"github.com/abdul-hamid-achik/hitspec/packages/proxy"
)

// RecordStartReq is the request body for POST /record/start.
type RecordStartReq struct {
	TargetURL   string   `json:"targetUrl"`
	Port        int      `json:"port,omitempty"`
	Exclude     []string `json:"exclude,omitempty"`
	Sanitize    []string `json:"sanitize,omitempty"`
	Deduplicate bool     `json:"deduplicate,omitempty"`
}

// RecordStatusDTO is the response for GET /record/status.
type RecordStatusDTO struct {
	Running    bool           `json:"running"`
	TargetURL  string         `json:"targetUrl,omitempty"`
	Port       int            `json:"port,omitempty"`
	Count      int            `json:"count"`
	Recordings []RecordingDTO `json:"recordings,omitempty"`
}

// RecordingDTO is a single recorded request/response.
type RecordingDTO struct {
	Method      string  `json:"method"`
	Path        string  `json:"path"`
	URL         string  `json:"url"`
	ContentType string  `json:"contentType,omitempty"`
	StatusCode  int     `json:"statusCode,omitempty"`
	Duration    float64 `json:"duration,omitempty"`
}

func (s *Server) handleRecordStart(w http.ResponseWriter, r *http.Request) {
	var req RecordStartReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.TargetURL == "" {
		writeError(w, http.StatusBadRequest, "targetUrl is required")
		return
	}

	s.mu.Lock()
	if s.recorder != nil {
		s.mu.Unlock()
		writeError(w, http.StatusConflict, "recording proxy already running")
		return
	}

	port := req.Port
	if port == 0 {
		port = 8081
	}

	opts := []proxy.Option{
		proxy.WithTargetURL(req.TargetURL),
		proxy.WithPort(port),
		proxy.WithVerbose(s.config.Verbose),
		proxy.WithDeduplicate(req.Deduplicate),
	}
	if len(req.Exclude) > 0 {
		opts = append(opts, proxy.WithExclude(req.Exclude))
	}
	if len(req.Sanitize) > 0 {
		opts = append(opts, proxy.WithSanitize(req.Sanitize))
	}

	recorder := proxy.NewRecorder(opts...)
	ctx, cancel := context.WithCancel(s.ctx)
	s.recorder = recorder
	s.recorderCancel = cancel
	s.recorderPort = port
	s.recorderTarget = req.TargetURL
	s.mu.Unlock()

	s.logger.Info("recording proxy starting", "target", req.TargetURL, "port", port)

	go func() {
		_ = recorder.StartWithContext(ctx)
		s.mu.Lock()
		s.recorder = nil
		s.recorderCancel = nil
		s.mu.Unlock()
	}()

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "started",
		"port":   port,
	})
}

func (s *Server) handleRecordStop(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.recorderCancel == nil {
		writeError(w, http.StatusBadRequest, "no recording proxy running")
		return
	}

	s.logger.Info("recording proxy stopping")
	s.recorderCancel()
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopping"})
}

func (s *Server) handleRecordStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	recorder := s.recorder
	port := s.recorderPort
	target := s.recorderTarget
	s.mu.Unlock()

	if recorder == nil {
		writeJSON(w, http.StatusOK, RecordStatusDTO{Running: false})
		return
	}

	recordings := recorder.GetRecordings()
	dtos := make([]RecordingDTO, 0, len(recordings))
	for _, rec := range recordings {
		dto := RecordingDTO{
			Method:      rec.Method,
			Path:        rec.Path,
			URL:         rec.URL,
			ContentType: rec.ContentType,
		}
		if rec.Response != nil {
			dto.StatusCode = rec.Response.StatusCode
			dto.Duration = float64(rec.Response.Duration.Milliseconds())
		}
		dtos = append(dtos, dto)
	}

	writeJSON(w, http.StatusOK, RecordStatusDTO{
		Running:    true,
		TargetURL:  target,
		Port:       port,
		Count:      len(recordings),
		Recordings: dtos,
	})
}

func (s *Server) handleRecordExport(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	recorder := s.recorder
	s.mu.Unlock()

	if recorder == nil {
		writeError(w, http.StatusBadRequest, "no recording proxy running")
		return
	}

	content := recorder.Export()
	writeJSON(w, http.StatusOK, map[string]string{"content": content})
}

func (s *Server) handleRecordClear(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	recorder := s.recorder
	s.mu.Unlock()

	if recorder == nil {
		writeError(w, http.StatusBadRequest, "no recording proxy running")
		return
	}

	recorder.Clear()
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}
