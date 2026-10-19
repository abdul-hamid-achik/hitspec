package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"
)

// testRoutes lets other _test.go files add routes to the test server without
// editing this file: func init() { testRoutes = append(testRoutes, ...) }.
var testRoutes []func(mux *http.ServeMux)

// newTestServer returns the local HTTP server that CLI scripts talk to via
// $SERVER. Everything is deterministic and offline. Routes:
//
//	/echo/...          JSON echo of method, path, raw query, headers, body
//	/status/{code}     empty JSON object with that status code
//	/json              fixed JSON document (large ints, nested arrays, null)
//	/headers           sets X-Request-Id and two Set-Cookie headers
//	/slow?ms=N         sleeps N ms, then 200
//	/stall             Content-Length 1000, writes 10 bytes, then stalls 3s
//	/text              text/plain body
func newTestServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/echo/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		headers := map[string]string{}
		for k, v := range r.Header {
			headers[k] = strings.Join(v, ", ")
		}
		if r.Host != "" {
			headers["Host"] = r.Host
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"method":  r.Method,
			"path":    r.URL.Path,
			"query":   r.URL.RawQuery,
			"headers": headers,
			"body":    string(body),
		})
	})

	mux.HandleFunc("/status/", func(w http.ResponseWriter, r *http.Request) {
		code, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/status/"))
		if err != nil || code < 100 || code > 599 {
			code = http.StatusBadRequest
		}
		writeJSON(w, code, map[string]any{})
	})

	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"id":    1000000,
			"big":   123456789012,
			"name":  "Jane Doe",
			"nullv": nil,
			"items": []map[string]any{
				{"id": 1, "name": "a", "tags": []string{"x", "y"}},
				{"id": 2, "name": "b", "tags": []string{}},
			},
		})
	})

	mux.HandleFunc("/headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req-123")
		w.Header().Add("Set-Cookie", "a=1; Path=/")
		w.Header().Add("Set-Cookie", "b=2; Path=/")
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		ms, _ := strconv.Atoi(r.URL.Query().Get("ms"))
		select {
		case <-time.After(time.Duration(ms) * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"slept": ms})
	})

	mux.HandleFunc("/stall", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("0123456789"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		select {
		case <-time.After(3 * time.Second):
		case <-r.Context().Done():
		}
	})

	mux.HandleFunc("/text", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain text body"))
	})

	for _, register := range testRoutes {
		register(mux)
	}

	return httptest.NewServer(mux)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
