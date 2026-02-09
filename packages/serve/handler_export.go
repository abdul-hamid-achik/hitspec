package serve

import (
	"net/http"
	"path/filepath"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	curlexport "github.com/abdul-hamid-achik/hitspec/packages/export/curl"
)

func (s *Server) handleExportCurl(w http.ResponseWriter, r *http.Request) {
	var req ExportCurlReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.File == "" {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}

	absPath := filepath.Join(s.config.WorkDir, req.File)
	if !isPathWithin(s.config.WorkDir, absPath) {
		writeError(w, http.StatusForbidden, "path outside workspace")
		return
	}

	parsed, err := parser.ParseFile(absPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	exporter := curlexport.New()

	var reqs []*parser.Request
	if req.RequestName != "" {
		for _, r := range parsed.Requests {
			if r.Name == req.RequestName {
				reqs = append(reqs, r)
				break
			}
		}
		if len(reqs) == 0 {
			writeError(w, http.StatusNotFound, "request not found: "+req.RequestName)
			return
		}
	} else {
		reqs = parsed.Requests
	}

	commands := exporter.ExportAll(reqs)

	writeJSON(w, http.StatusOK, ExportResultDTO{Commands: commands})
}
