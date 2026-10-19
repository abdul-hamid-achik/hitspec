package main

import "net/http"

// Routes for the language-strictness scripts (lang_*, assert_*, capture_*).
// Everything lives under /a3/.
func init() {
	testRoutes = append(testRoutes, func(mux *http.ServeMux) {
		// RFC 7807 problem document: a +json media type with a charset.
		mux.HandleFunc("/a3/problem", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"type":"about:blank","title":"Not Found","status":404,"errors":[{"field":"id","code":"missing"}]}`))
		})

		// Not JSON, but the text looks like it: a body path must not match.
		mux.HandleFunc("/a3/html", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body>{"x": 1}</body></html>`))
		})

		// Values whose spelling the old expected-value grammar mangled.
		mux.HandleFunc("/a3/values", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{
				"name":    "Jane Doe",
				"path":    "./x",
				"caret":   "^x",
				"dollar":  "$x",
				"at":      "@x",
				"hash":    "#fff",
				"msg":     "line\nbreak",
				"cjk":     "日本語",
				"nothing": nil,
				"obj":     map[string]any{"a": 1, "b": []any{1, 2.5, "x"}},
				"list":    []any{1, 2, 3},
				"items": []map[string]any{
					{"id": 1, "name": "Jane Doe", "email": "a@x.io"},
					{"id": 2, "name": "Bob", "email": "b@x.io"},
				},
			})
		})
	})
}
