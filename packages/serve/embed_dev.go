//go:build dev

package serve

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

const viteDevServerURL = "http://localhost:5173"

func spaHandler() http.Handler {
	target, _ := url.Parse(viteDevServerURL)

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = target.Host
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Vite Dev Server Not Running</title></head>
<body style="font-family:system-ui,sans-serif;max-width:600px;margin:80px auto;text-align:center">
  <h1>Vite Dev Server Not Running</h1>
  <p>The hitspec UI dev server is not reachable at <code>%s</code>.</p>
  <p>Start it with:</p>
  <pre style="background:#f4f4f4;padding:12px;border-radius:6px">task client:dev</pre>
  <p style="color:#888;font-size:0.9em">Error: %s</p>
</body>
</html>`, viteDevServerURL, err.Error())
		},
	}

	return proxy
}
