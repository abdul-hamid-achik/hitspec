package serve

import (
	"net/http"
	"path/filepath"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
)

// configToDTO is the wire form of the editable settings.
func configToDTO(c *config.Config) ConfigDTO {
	return ConfigDTO{
		DefaultEnvironment: c.DefaultEnvironment,
		Timeout:            c.Timeout,
		Retries:            c.Retries,
		RetryDelay:         c.RetryDelay,
		FollowRedirects:    c.FollowRedirects,
		MaxRedirects:       c.MaxRedirects,
		ValidateSSL:        c.ValidateSSL,
		Proxy:              c.Proxy,
		Headers:            c.Headers,
		Parallel:           c.Parallel,
		Concurrency:        c.Concurrency,
	}
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	s.configMu.RLock()
	if s.fileConfig == nil {
		s.configMu.RUnlock()
		writeJSON(w, http.StatusOK, ConfigDTO{})
		return
	}
	dto := configToDTO(s.fileConfig)
	s.configMu.RUnlock()

	writeJSON(w, http.StatusOK, dto)
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	// Pointer fields: a key that is present in the body is applied even when it
	// is 0 or "", so retries can be set to 0 and the proxy cleared.
	var patch config.Patch
	if err := readJSON(r, &patch); err != nil {
		writeReadError(w, err)
		return
	}

	s.configMu.Lock()
	// If no config loaded yet, initialize with defaults so we can save
	current := s.fileConfig
	if current == nil {
		current = config.DefaultConfig()
	}

	// Work on a copy so a rejected patch or a failed save leaves the live
	// config (and what the next run uses) unchanged.
	next := *current
	if err := next.Apply(patch); err != nil {
		s.configMu.Unlock()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// If no config path was discovered, create hitspec.yaml in the workspace
	configPath := s.configPath
	if configPath == "" {
		configPath = filepath.Join(s.config.WorkDir, "hitspec.yaml")
	}

	// Save while holding the lock to prevent concurrent mutation of fileConfig
	if err := next.SaveConfig(configPath); err != nil {
		s.configMu.Unlock()
		s.logger.Error("failed to save config", "error", err, "path", configPath)
		writeError(w, http.StatusInternalServerError, "failed to save config to disk")
		return
	}
	s.fileConfig = &next
	s.configPath = configPath
	dto := configToDTO(&next)
	s.configMu.Unlock()

	writeJSON(w, http.StatusOK, dto)
}
