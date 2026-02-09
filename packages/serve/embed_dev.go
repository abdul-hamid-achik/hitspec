//go:build dev

package serve

import "net/http"

func spaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "SPA not available in dev mode. Run the Vite dev server on :5173 instead.", http.StatusServiceUnavailable)
	})
}
