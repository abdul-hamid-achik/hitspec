package serve

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	curlimport "github.com/abdul-hamid-achik/hitspec/packages/import/curl"
	"github.com/abdul-hamid-achik/hitspec/packages/import/insomnia"
	"github.com/abdul-hamid-achik/hitspec/packages/import/openapi"
)

func (s *Server) handleImportCurl(w http.ResponseWriter, r *http.Request) {
	var req ImportCurlReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	converter := curlimport.NewConverter()

	var content string
	var err error

	if req.Command != "" {
		content, err = converter.ConvertCommand(req.Command)
	} else if req.FilePath != "" {
		absPath := filepath.Join(s.config.WorkDir, req.FilePath)
		if !isPathWithin(s.config.WorkDir, absPath) {
			writeError(w, http.StatusForbidden, "path outside workspace")
			return
		}
		content, err = converter.ConvertFile(absPath)
	} else {
		writeError(w, http.StatusBadRequest, "command or filePath is required")
		return
	}

	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	reqCount := strings.Count(content, "###")
	writeJSON(w, http.StatusOK, ImportResultDTO{
		Content:      content,
		RequestCount: reqCount,
	})
}

func (s *Server) handleImportInsomnia(w http.ResponseWriter, r *http.Request) {
	var req ImportInsomniaReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	converter := insomnia.NewConverter()

	var content string
	var err error

	if req.Data != "" {
		content, err = converter.Convert([]byte(req.Data))
	} else if req.FilePath != "" {
		absPath := filepath.Join(s.config.WorkDir, req.FilePath)
		if !isPathWithin(s.config.WorkDir, absPath) {
			writeError(w, http.StatusForbidden, "path outside workspace")
			return
		}
		content, err = converter.ConvertFile(absPath)
	} else {
		writeError(w, http.StatusBadRequest, "data or filePath is required")
		return
	}

	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	reqCount := strings.Count(content, "###")
	writeJSON(w, http.StatusOK, ImportResultDTO{
		Content:      content,
		RequestCount: reqCount,
	})
}

func (s *Server) handleImportOpenAPI(w http.ResponseWriter, r *http.Request) {
	var req ImportOpenAPIReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.SpecPath == "" {
		writeError(w, http.StatusBadRequest, "specPath is required")
		return
	}

	opts := []openapi.Option{}
	if req.BaseURL != "" {
		opts = append(opts, openapi.WithBaseURL(req.BaseURL))
	}

	converter := openapi.NewConverter(opts...)

	// Resolve path: could be URL or local file
	specPath := req.SpecPath
	if strings.HasPrefix(specPath, "http://") || strings.HasPrefix(specPath, "https://") {
		// Validate URL to prevent SSRF against internal services
		u, err := url.Parse(specPath)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid URL: "+err.Error())
			return
		}
		host := strings.ToLower(u.Hostname())
		if host == "localhost" || host == "127.0.0.1" || host == "::1" ||
			host == "0.0.0.0" || strings.HasPrefix(host, "10.") ||
			strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "172.") ||
			host == "169.254.169.254" || host == "metadata.google.internal" {
			writeError(w, http.StatusForbidden, "URLs pointing to internal/private addresses are not allowed")
			return
		}
	} else {
		specPath = filepath.Join(s.config.WorkDir, specPath)
		if !isPathWithin(s.config.WorkDir, specPath) {
			writeError(w, http.StatusForbidden, "path outside workspace")
			return
		}
	}

	content, err := converter.ConvertFile(specPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	reqCount := strings.Count(content, "###")
	writeJSON(w, http.StatusOK, ImportResultDTO{
		Content:      content,
		RequestCount: reqCount,
	})
}
