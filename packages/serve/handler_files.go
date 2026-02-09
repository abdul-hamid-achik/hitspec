package serve

import (
	"net/http"
	"os"
	"path/filepath"

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
