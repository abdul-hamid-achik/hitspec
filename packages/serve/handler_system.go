package serve

import (
	"net/http"
	"runtime"
)

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, SystemInfoDTO{
		Version:   s.Version,
		BuildTime: s.BuildTime,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	})
}
