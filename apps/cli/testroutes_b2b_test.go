package main

import (
	"net/http"
)

func init() {
	testRoutes = append(testRoutes, func(mux *http.ServeMux) {
		// /b2b/hangup accepts the request and closes the connection without
		// writing a response (a reset/closed connection before any response).
		mux.HandleFunc("/b2b/hangup", func(w http.ResponseWriter, r *http.Request) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
		})

		// /b2b/to-https redirects to the https scheme of the same plain-HTTP
		// listener, which cannot complete a TLS handshake.
		mux.HandleFunc("/b2b/to-https", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://"+r.Host+"/status/200", http.StatusFound)
		})

		// /b2b/midbody sends headers and part of the body, then drops the
		// connection: a response did arrive, so this is not a network error.
		mux.HandleFunc("/b2b/midbody", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "1000")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("0123456789"))
			w.(http.Flusher).Flush()
			if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
				_ = conn.Close()
			}
		})
	})
}
