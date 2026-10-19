package main

import (
	"io"
	"net/http"
)

// Routes for the http_* scripts (HTTP client options, multipart, encoding).
// Everything lives under /b3/.
func init() {
	testRoutes = append(testRoutes, func(mux *http.ServeMux) {
		// Describes every part of a multipart/form-data request.
		mux.HandleFunc("/b3/multipart", func(w http.ResponseWriter, r *http.Request) {
			mr, err := r.MultipartReader()
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
				return
			}
			parts := []map[string]string{}
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
					return
				}
				data, _ := io.ReadAll(p)
				parts = append(parts, map[string]string{
					"name":     p.FormName(),
					"filename": p.FileName(),
					"body":     string(data),
				})
			}
			writeJSON(w, http.StatusOK, map[string]any{"parts": parts})
		})

		// A tiny session: /b3/login sets a cookie, /b3/me reports what it got.
		mux.HandleFunc("/b3/login", func(w http.ResponseWriter, r *http.Request) {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc123", Path: "/"})
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		})
		mux.HandleFunc("/b3/me", func(w http.ResponseWriter, r *http.Request) {
			session := ""
			if c, err := r.Cookie("session"); err == nil {
				session = c.Value
			}
			writeJSON(w, http.StatusOK, map[string]any{"session": session})
		})

		// 4 KiB of text, for --max-response-bytes.
		mux.HandleFunc("/b3/big", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			for range 64 {
				_, _ = w.Write([]byte("0123456789012345678901234567890123456789012345678901234567890123"))
			}
		})
	})
}
