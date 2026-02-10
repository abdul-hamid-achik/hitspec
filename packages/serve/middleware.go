package serve

import (
	"log"
	"net/http"
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

// recoveryMiddleware catches panics and returns 500.
func recoveryMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("panic: %v", err)
					writeError(w, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// loggingMiddleware logs HTTP requests when verbose.
func loggingMiddleware(verbose bool) Middleware {
	return func(next http.Handler) http.Handler {
		if !verbose {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
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
