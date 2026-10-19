package main

import (
	"net/http"
	"strings"
	"sync"
)

func init() {
	testRoutes = append(testRoutes, func(mux *http.ServeMux) {
		// /a2/flaky/<key> answers 503 twice per key, then 200: a request that
		// is retried (hitspec.yaml retries) eventually passes.
		var mu sync.Mutex
		calls := map[string]int{}
		mux.HandleFunc("/a2/flaky/", func(w http.ResponseWriter, r *http.Request) {
			key := strings.TrimPrefix(r.URL.Path, "/a2/flaky/")
			mu.Lock()
			calls[key]++
			n := calls[key]
			mu.Unlock()
			if n <= 2 {
				writeJSON(w, http.StatusServiceUnavailable, map[string]any{"attempt": n})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"attempt": n})
		})
	})
}
