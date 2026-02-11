package serve

import (
	"net/http"
)

func (s *Server) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
	s.configMu.RLock()
	envs := make([]EnvironmentDTO, 0)

	if s.fileConfig != nil && s.fileConfig.Environments != nil {
		for name, vars := range s.fileConfig.Environments {
			envs = append(envs, EnvironmentDTO{
				Name:      name,
				Variables: vars,
			})
		}
	}

	// Always include the active environment even if empty
	found := false
	for _, e := range envs {
		if e.Name == s.config.Env {
			found = true
			break
		}
	}
	if !found {
		envs = append(envs, EnvironmentDTO{
			Name:      s.config.Env,
			Variables: make(map[string]any),
		})
	}
	s.configMu.RUnlock()

	writeJSON(w, http.StatusOK, envs)
}

func (s *Server) handleGetEnvironment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	s.configMu.RLock()
	if s.fileConfig != nil && s.fileConfig.Environments != nil {
		if vars, ok := s.fileConfig.Environments[name]; ok {
			s.configMu.RUnlock()
			writeJSON(w, http.StatusOK, EnvironmentDTO{
				Name:      name,
				Variables: vars,
			})
			return
		}
	}
	s.configMu.RUnlock()

	writeJSON(w, http.StatusOK, EnvironmentDTO{
		Name:      name,
		Variables: make(map[string]any),
	})
}

func (s *Server) handleSelectEnvironment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	s.configMu.Lock()
	s.config.Env = req.Name
	s.configMu.Unlock()

	s.hub.Broadcast("environment_changed", map[string]string{
		"name":      req.Name,
		"timestamp": nowISO(),
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePutEnvironment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	var dto EnvironmentDTO
	if err := readJSON(r, &dto); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.configMu.Lock()
	if s.fileConfig == nil {
		s.configMu.Unlock()
		writeError(w, http.StatusBadRequest, "no config file found")
		return
	}
	if s.fileConfig.Environments == nil {
		s.fileConfig.Environments = make(map[string]map[string]any)
	}
	s.fileConfig.Environments[name] = dto.Variables

	// Persist to disk while holding the lock
	if !s.saveConfig(w) {
		s.configMu.Unlock()
		return
	}
	s.configMu.Unlock()

	writeJSON(w, http.StatusOK, EnvironmentDTO{
		Name:      name,
		Variables: dto.Variables,
	})
}
