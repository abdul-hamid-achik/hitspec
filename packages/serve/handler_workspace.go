package serve

import (
	"net/http"
	"path/filepath"
)

func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	files, err := collectHitspecFiles(s.config.WorkDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	relFiles := make([]string, 0, len(files))
	for _, f := range files {
		rel, _ := filepath.Rel(s.config.WorkDir, f)
		if rel == "" {
			rel = f
		}
		relFiles = append(relFiles, rel)
	}

	writeJSON(w, http.StatusOK, WorkspaceDTO{
		Directory:   s.config.WorkDir,
		FileCount:   len(files),
		Files:       relFiles,
		Environment: s.config.Env,
		HasConfig:   s.fileConfig != nil,
	})
}
