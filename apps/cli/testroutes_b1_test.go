package main

import (
	"fmt"
	"net/http"
	"strings"
)

func init() {
	testRoutes = append(testRoutes, func(mux *http.ServeMux) {
		// /b1/missing: a 404 with a small JSON error body.
		mux.HandleFunc("/b1/missing", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found", "id": 42})
		})

		// /b1/secure: 200 with the right bearer token, else 401. The body never
		// echoes the request, so a secret showing up in the report can only have
		// come from the formatter.
		mux.HandleFunc("/b1/secure", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Set-Cookie", "session=server-cookie-value; Path=/")
			w.Header().Set("X-Api-Key", "server-api-key-value")
			w.Header().Set("X-Trace", "trace-1")
			if r.Header.Get("Authorization") != "Bearer good-token" {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		})

		// /b1/big: a JSON document far larger than the failure excerpt.
		mux.HandleFunc("/b1/big", func(w http.ResponseWriter, r *http.Request) {
			items := make([]map[string]any, 0, 200)
			for i := 0; i < 200; i++ {
				items = append(items, map[string]any{"id": i, "name": fmt.Sprintf("item-%03d", i)})
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"items": items})
		})

		// /b1/binary: a non-text body.
		mux.HandleFunc("/b1/binary", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte{0x89, 'P', 'N', 'G', 0, 0, 1, 2, 0xff})
		})

		// /b1/ansi: a text body carrying a terminal escape sequence.
		mux.HandleFunc("/b1/ansi", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("bad\x1b[31mred\x1b[0m gateway"))
		})

		// /b1/login: a token to capture; /b1/ping answers 200 for every method.
		mux.HandleFunc("/b1/login", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"token": "good-token"})
		})
		mux.HandleFunc("/b1/ping", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"method": strings.ToLower(r.Method)})
		})
	})
}
