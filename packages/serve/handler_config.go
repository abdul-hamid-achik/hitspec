package serve

import (
	"net/http"
)

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if s.fileConfig == nil {
		writeJSON(w, http.StatusOK, ConfigDTO{})
		return
	}

	dto := ConfigDTO{
		DefaultEnvironment: s.fileConfig.DefaultEnvironment,
		Timeout:            s.fileConfig.Timeout,
		Retries:            s.fileConfig.Retries,
		FollowRedirects:    s.fileConfig.FollowRedirects,
		ValidateSSL:        s.fileConfig.ValidateSSL,
		Proxy:              s.fileConfig.Proxy,
		Headers:            s.fileConfig.Headers,
		Parallel:           s.fileConfig.Parallel,
		Concurrency:        s.fileConfig.Concurrency,
	}

	writeJSON(w, http.StatusOK, dto)
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var dto ConfigDTO
	if err := readJSON(r, &dto); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if s.fileConfig == nil {
		writeError(w, http.StatusBadRequest, "no config file found")
		return
	}

	if dto.DefaultEnvironment != "" {
		s.fileConfig.DefaultEnvironment = dto.DefaultEnvironment
	}
	if dto.Timeout > 0 {
		s.fileConfig.Timeout = dto.Timeout
	}
	if dto.Retries > 0 {
		s.fileConfig.Retries = dto.Retries
	}
	if dto.FollowRedirects != nil {
		s.fileConfig.FollowRedirects = dto.FollowRedirects
	}
	if dto.ValidateSSL != nil {
		s.fileConfig.ValidateSSL = dto.ValidateSSL
	}
	if dto.Proxy != "" {
		s.fileConfig.Proxy = dto.Proxy
	}
	if dto.Headers != nil {
		s.fileConfig.Headers = dto.Headers
	}
	if dto.Parallel != nil {
		s.fileConfig.Parallel = dto.Parallel
	}
	if dto.Concurrency > 0 {
		s.fileConfig.Concurrency = dto.Concurrency
	}

	writeJSON(w, http.StatusOK, dto)
}
