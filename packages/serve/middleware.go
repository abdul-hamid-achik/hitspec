package serve

import (
	"bufio"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Middleware is an HTTP middleware function.
type Middleware func(http.Handler) http.Handler

// chain applies middlewares in order.
func chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// loopbackHostnames are the hostnames a loopback-bound server answers to.
var loopbackHostnames = map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}

// isLoopbackBind reports whether a --host bind address only accepts connections
// from the local machine.
func isLoopbackBind(host string) bool {
	h := strings.Trim(host, "[]")
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// parseHostPort splits an HTTP Host header or URL host into a lower-cased
// hostname and port (empty when absent).
func parseHostPort(h string) (hostname, port string, ok bool) {
	if hn, pt, err := net.SplitHostPort(h); err == nil {
		return strings.ToLower(hn), pt, true
	}
	if strings.HasPrefix(h, "[") && strings.HasSuffix(h, "]") {
		return strings.ToLower(h[1 : len(h)-1]), "", true
	}
	if h != "" && !strings.Contains(h, ":") {
		return strings.ToLower(h), "", true
	}
	return "", "", false
}

// hostAllowed reports whether a Host header names this loopback-bound server:
// localhost, 127.0.0.1 or [::1], on the listening port or without a port.
func hostAllowed(host string, port int) bool {
	hn, pt, ok := parseHostPort(host)
	if !ok || !loopbackHostnames[hn] {
		return false
	}
	return pt == "" || pt == strconv.Itoa(port)
}

// hostCheckMiddleware rejects requests whose Host header is not a loopback name
// of this server. A server bound to loopback is reachable from any web page via
// DNS rebinding (the attacker's domain re-resolves to 127.0.0.1), and the
// browser then treats the API as same-origin; the rebound request still carries
// the attacker's Host, so refusing unknown Hosts closes that path.
func hostCheckMiddleware(port int) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !hostAllowed(r.Host, port) {
				writeError(w, http.StatusForbidden, "host not allowed")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// originAllowed reports whether a browser Origin may talk to this server: a
// loopback origin on the server's own port, or (with --cors) on any port.
func originAllowed(origin string, port int, cors bool) bool {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	hn, pt, ok := parseHostPort(u.Host)
	if !ok || !loopbackHostnames[hn] {
		return false
	}
	if cors {
		return true
	}
	if pt == "" {
		pt = "80"
		if u.Scheme == "https" {
			pt = "443"
		}
	}
	return pt == strconv.Itoa(port)
}

// corsMiddleware enforces the Origin policy and, when CORS is enabled, answers
// preflights and adds CORS headers for allowed origins.
//
// A request without an Origin header (CLI, curl, Node fetch) passes. A request
// with one must come from an allowed origin, whether or not CORS is enabled:
// CORS only tells the browser which responses a page may read, it does not stop
// the request from being sent, so a cross-site page could still trigger side
// effects. The WebSocket upgrade goes through here as well.
func corsMiddleware(enabled bool, port int) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origins := r.Header.Values("Origin")
			for _, o := range origins {
				if !originAllowed(o, port, enabled) {
					writeError(w, http.StatusForbidden, "origin not allowed")
					return
				}
			}
			if enabled {
				if len(origins) == 1 {
					w.Header().Set("Access-Control-Allow-Origin", origins[0])
				}
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status  int
	written int64
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	return n, err
}

// Hijack delegates to the underlying ResponseWriter for WebSocket support.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}

// Flush delegates to the underlying ResponseWriter for SSE support.
func (w *statusWriter) Flush() {
	if fl, ok := w.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

// recoveryMiddleware catches panics and returns 500.
func recoveryMiddleware(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered", "error", err, "path", r.URL.Path)
					writeError(w, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// loggingMiddleware logs HTTP requests with structured fields.
func loggingMiddleware(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := generateID()
			ctx := contextWithRequestID(r.Context(), reqID)
			r = r.WithContext(ctx)

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			logger.Info("http request",
				"request_id", reqID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"bytes", sw.written,
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

// readOnlyMiddleware blocks mutating methods when read-only.
// In read-only mode, only GET and OPTIONS requests are allowed.
func readOnlyMiddleware(readOnly bool) Middleware {
	return func(next http.Handler) http.Handler {
		if !readOnly {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodOptions {
				writeError(w, http.StatusForbidden, "server is in read-only mode")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// authTokenMiddleware gates all REST and WebSocket requests behind a shared
// bearer token. The serve API exposes full file read/write/delete, arbitrary
// request execution (SSRF), and RCE when --allow-shell is set; without auth it is
// only "protected" by the default localhost bind. When token is empty the
// middleware is a no-op (backward compatible). The token is accepted via:
//   - Authorization: Bearer <token> header (REST clients)
//   - ?token=<token> query parameter (WebSocket/browser clients that cannot set
//     headers on the upgrade request)
//
// The comparison is constant-time to avoid timing side channels.
func authTokenMiddleware(token string) Middleware {
	return func(next http.Handler) http.Handler {
		if token == "" {
			return next
		}
		want := sha256.Sum256([]byte(token))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			present := ""
			// Only the Bearer scheme is accepted; the scheme name is
			// case-insensitive (RFC 9110).
			if scheme, cred, ok := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " "); ok && strings.EqualFold(scheme, "bearer") {
				present = strings.TrimSpace(cred)
			}
			if present == "" {
				present = r.URL.Query().Get("token")
			}
			got := sha256.Sum256([]byte(present))
			if present == "" || subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
				w.Header().Set("WWW-Authenticate", `Bearer realm="hitspec"`)
				writeError(w, http.StatusUnauthorized, "unauthorized: missing or invalid API token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
