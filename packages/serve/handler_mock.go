package serve

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/mock"
)

func (s *Server) handleMockStart(w http.ResponseWriter, r *http.Request) {
	var req MockStartReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	if s.mockServer != nil {
		s.mu.Unlock()
		writeError(w, http.StatusConflict, "mock server already running")
		return
	}

	port := 3000
	if req.Port > 0 {
		port = req.Port
	}

	var delay time.Duration
	if req.Delay != "" {
		var err error
		delay, err = time.ParseDuration(req.Delay)
		if err != nil {
			s.mu.Unlock()
			writeError(w, http.StatusBadRequest, "invalid delay: "+err.Error())
			return
		}
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

	mockSrv := mock.NewServer(
		mock.WithPort(port),
		mock.WithDelay(delay),
		mock.WithVerbose(s.config.Verbose),
		mock.WithRequestCallback(func(method, path string, status int, duration time.Duration) {
			s.hub.Broadcast("mock_request", WSMockEvent{
				Event:     "request",
				Method:    method,
				Path:      path,
				Status:    status,
				Duration:  float64(duration.Milliseconds()),
				Timestamp: nowISO(),
			})
		}),
	)

	if err := mockSrv.LoadFiles(absFiles); err != nil {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithCancel(s.ctx)
	s.mockServer = mockSrv
	s.mockCancel = cancel
	s.mockPort = port
	s.mu.Unlock()

	go func() {
		s.hub.Broadcast("mock_request", WSMockEvent{
			Event:     "started",
			Timestamp: nowISO(),
		})

		_ = mockSrv.StartWithContext(ctx)

		s.hub.Broadcast("mock_request", WSMockEvent{
			Event:     "stopped",
			Timestamp: nowISO(),
		})

		s.mu.Lock()
		s.mockServer = nil
		s.mockCancel = nil
		s.mockPort = 0
		s.mu.Unlock()
	}()

	routes := mockSrv.GetRoutes()
	routeDTOs := make([]MockRouteDTO, 0, len(routes))
	for _, route := range routes {
		routeDTOs = append(routeDTOs, MockRouteDTO{
			Method:      route.Method,
			Path:        route.PathPattern,
			Name:        route.Name,
			StatusCode:  route.Response.StatusCode,
			ContentType: route.Response.ContentType,
		})
	}

	s.logger.Info("mock server starting", "port", port, "routes", len(routes))

	writeJSON(w, http.StatusOK, MockStatusDTO{
		Running: true,
		Port:    port,
		Routes:  routeDTOs,
	})
}

func (s *Server) handleMockStop(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.mockCancel == nil {
		writeError(w, http.StatusBadRequest, "no mock server running")
		return
	}

	s.logger.Info("mock server stopping")
	s.mockCancel()
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopping"})
}

func (s *Server) handleMockRoutes(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	mockSrv := s.mockServer
	port := s.mockPort
	s.mu.Unlock()

	if mockSrv == nil {
		writeJSON(w, http.StatusOK, MockStatusDTO{Running: false})
		return
	}

	routes := mockSrv.GetRoutes()
	routeDTOs := make([]MockRouteDTO, 0, len(routes))
	for _, route := range routes {
		routeDTOs = append(routeDTOs, MockRouteDTO{
			Method:      route.Method,
			Path:        route.PathPattern,
			Name:        route.Name,
			StatusCode:  route.Response.StatusCode,
			ContentType: route.Response.ContentType,
		})
	}

	writeJSON(w, http.StatusOK, MockStatusDTO{
		Running: true,
		Port:    port,
		Routes:  routeDTOs,
	})
}
