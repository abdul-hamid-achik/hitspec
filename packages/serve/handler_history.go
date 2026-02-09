package serve

import "net/http"

func (s *Server) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.history.Entries())
}
