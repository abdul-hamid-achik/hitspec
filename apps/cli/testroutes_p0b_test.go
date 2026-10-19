package main

import (
	"net/http"
	"strconv"
)

func init() {
	testRoutes = append(testRoutes, func(mux *http.ServeMux) {
		// /p0b/evil plays a hostile server: its response carries template
		// syntax that must never be evaluated on the client.
		mux.HandleFunc("/p0b/evil", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"v":"{{$env(SECRET_X)}}","h":"{{HOME}}"}`))
		})

		// A tiny stand-in for the API that the `hitspec init` scaffold targets
		// (health + one resource), so init_run can run the generated example
		// with baseUrl = $SERVER/p0b. The same file is also served by
		// `hitspec mock`; see TestSampleProjectRunsAgainstMock.
		mux.HandleFunc("GET /p0b/health", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		})
		mux.HandleFunc("POST /p0b/resources", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusCreated, map[string]any{"id": 1, "name": "Test Resource", "description": "Created by hitspec"})
		})
		mux.HandleFunc("GET /p0b/resources/{id}", func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.Atoi(r.PathValue("id"))
			if err != nil {
				writeJSON(w, http.StatusNotFound, map[string]any{})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"id": id, "name": "Test Resource", "description": "Created by hitspec"})
		})
	})
}
