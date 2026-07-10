package serve

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
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

// corsMiddleware adds CORS headers when enabled, restricted to localhost origins.
func corsMiddleware(enabled bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if enabled {
				origin := r.Header.Get("Origin")
				if isLocalhostOrigin(origin) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
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

// isLocalhostOrigin checks if an origin is a localhost address.
func isLocalhostOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	// Allow localhost, 127.0.0.1, [::1] on any port
	for _, prefix := range []string{
		"http://localhost",
		"https://localhost",
		"http://127.0.0.1",
		"https://127.0.0.1",
		"http://[::1]",
		"https://[::1]",
	} {
		if origin == prefix || len(origin) > len(prefix) && origin[:len(prefix)+1] == prefix+":" {
			return true
		}
	}
	return false
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
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			present := r.Header.Get("Authorization")
			if strings.HasPrefix(present, "Bearer ") {
				present = strings.TrimPrefix(present, "Bearer ")
			} else {
				present = "" // only the Bearer scheme is accepted
			}
			if present == "" {
				present = r.URL.Query().Get("token")
			}
			if subtle.ConstantTimeCompare([]byte(present), []byte(token)) != 1 {
				w.Header().Set("WWW-Authenticate", `Bearer realm="hitspec"`)
				writeError(w, http.StatusUnauthorized, "unauthorized: missing or invalid API token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
