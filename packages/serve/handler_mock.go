package serve

import (
	"context"
	"net/http"
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

	mockSrv := mock.NewServer(
		mock.WithPort(port),
		mock.WithDelay(delay),
		mock.WithVerbose(s.config.Verbose),
		mock.WithRequestCallback(func(method, path string, status int, duration time.Duration) {
			s.hub.Broadcast("mock:request", WSMockEvent{
				Event:     "request",
				Method:    method,
				Path:      path,
				Status:    status,
				Duration:  float64(duration.Milliseconds()),
				Timestamp: nowISO(),
			})
		}),
	)

	if err := mockSrv.LoadFiles(req.Files); err != nil {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithCancel(s.ctx)
	s.mockServer = mockSrv
	s.mockCancel = cancel
	s.mu.Unlock()

	go func() {
		s.hub.Broadcast("mock:started", WSMockEvent{
			Event:     "started",
			Timestamp: nowISO(),
		})

		_ = mockSrv.StartWithContext(ctx)

		s.hub.Broadcast("mock:stopped", WSMockEvent{
			Event:     "stopped",
			Timestamp: nowISO(),
		})

		s.mu.Lock()
		s.mockServer = nil
		s.mockCancel = nil
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

	s.mockCancel()
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopping"})
}

func (s *Server) handleMockRoutes(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	mockSrv := s.mockServer
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
		Routes:  routeDTOs,
	})
}
