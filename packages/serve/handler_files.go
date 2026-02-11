package serve

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	files, err := collectHitspecFiles(s.config.WorkDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	dtos := make([]FileInfoDTO, 0, len(files))
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}

		rel, _ := filepath.Rel(s.config.WorkDir, f)
		if rel == "" {
			rel = f
		}

		// Quick-parse to get request count
		reqCount := 0
		parsed, err := parser.ParseFile(f)
		if err == nil {
			reqCount = len(parsed.Requests)
		}

		dtos = append(dtos, FileInfoDTO{
			Path:         f,
			RelativePath: rel,
			Name:         filepath.Base(f),
			Size:         info.Size(),
			ModTime:      info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
			RequestCount: reqCount,
		})
	}

	writeJSON(w, http.StatusOK, dtos)
}

func (s *Server) handleGetFile(w http.ResponseWriter, r *http.Request) {
	relPath := r.PathValue("path")
	if relPath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	absPath := filepath.Join(s.config.WorkDir, relPath)
	if !isPathWithin(s.config.WorkDir, absPath) {
		writeError(w, http.StatusForbidden, "path outside workspace")
		return
	}

	parsed, err := parser.ParseFile(absPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, convertFile(parsed))
}

func (s *Server) handleSaveFile(w http.ResponseWriter, r *http.Request) {
	relPath := r.PathValue("path")
	if relPath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	absPath := filepath.Join(s.config.WorkDir, relPath)
	if !isPathWithin(s.config.WorkDir, absPath) {
		writeError(w, http.StatusForbidden, "path outside workspace")
		return
	}

	// Only allow writing to .http/.hitspec files
	if !isHitspecFile(absPath) {
		writeError(w, http.StatusBadRequest, "only .http and .hitspec files can be saved")
		return
	}

	// File must exist for PUT
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	// Read raw body content (plain text, not JSON)
	body := http.MaxBytesReader(w, r.Body, maxRequestBody)
	defer body.Close()
	content, err := io.ReadAll(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	// Suppress the watcher for this self-write
	s.suppressWatch(absPath)

	if err := os.WriteFile(absPath, content, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Re-parse and return the updated file
	parsed, err := parser.ParseFile(absPath)
	if err != nil {
		// File was saved but failed to parse — still a success
		writeJSON(w, http.StatusOK, map[string]string{"status": "saved", "warning": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, convertFile(parsed))
}

func (s *Server) handleCreateFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	// Ensure the path has a valid extension
	if !isHitspecFile(req.Path) {
		// Default to .http if no valid extension
		if !strings.HasSuffix(req.Path, ".http") && !strings.HasSuffix(req.Path, ".hitspec") {
			req.Path = req.Path + ".http"
		}
	}

	absPath := filepath.Join(s.config.WorkDir, req.Path)
	if !isPathWithin(s.config.WorkDir, absPath) {
		writeError(w, http.StatusForbidden, "path outside workspace")
		return
	}

	// File must NOT exist
	if _, err := os.Stat(absPath); err == nil {
		writeError(w, http.StatusConflict, "file already exists")
		return
	}

	// Ensure parent directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	content := req.Content
	if content == "" {
		content = "### New Request\nGET https://example.com\n"
	}

	s.suppressWatch(absPath)

	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Parse and return
	parsed, err := parser.ParseFile(absPath)
	if err != nil {
		relPath, _ := filepath.Rel(s.config.WorkDir, absPath)
		writeJSON(w, http.StatusCreated, map[string]string{"path": relPath, "status": "created"})
		return
	}

	writeJSON(w, http.StatusCreated, convertFile(parsed))
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	relPath := r.PathValue("path")
	if relPath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	absPath := filepath.Join(s.config.WorkDir, relPath)
	if !isPathWithin(s.config.WorkDir, absPath) {
		writeError(w, http.StatusForbidden, "path outside workspace")
		return
	}

	if !isHitspecFile(absPath) {
		writeError(w, http.StatusBadRequest, "only .http and .hitspec files can be deleted")
		return
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	s.suppressWatch(absPath)

	if err := os.Remove(absPath); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetFileRaw returns the raw text content of a file (for the editor).
func (s *Server) handleGetFileRaw(w http.ResponseWriter, r *http.Request) {
	relPath := r.PathValue("path")
	if relPath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	absPath := filepath.Join(s.config.WorkDir, relPath)
	if !isPathWithin(s.config.WorkDir, absPath) {
		writeError(w, http.StatusForbidden, "path outside workspace")
		return
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}
