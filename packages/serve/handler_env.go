package serve

import (
	"net/http"
)

func (s *Server) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, http.StatusOK, envs)
}

func (s *Server) handleGetEnvironment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if s.fileConfig != nil && s.fileConfig.Environments != nil {
		if vars, ok := s.fileConfig.Environments[name]; ok {
			writeJSON(w, http.StatusOK, EnvironmentDTO{
				Name:      name,
				Variables: vars,
			})
			return
		}
	}

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

	s.mu.Lock()
	s.config.Env = req.Name
	s.mu.Unlock()

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

	if s.fileConfig == nil {
		writeError(w, http.StatusBadRequest, "no config file found")
		return
	}
	if s.fileConfig.Environments == nil {
		s.fileConfig.Environments = make(map[string]map[string]any)
	}
	s.fileConfig.Environments[name] = dto.Variables

	writeJSON(w, http.StatusOK, EnvironmentDTO{
		Name:      name,
		Variables: dto.Variables,
	})
}
