package serve

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAuthTokenMiddleware verifies the REST/WS API token gate: when a token is
// configured, requests without a valid token are rejected (401) and requests
// with a matching Bearer header or ?token= query are allowed through. With no
// token configured the middleware is a no-op (backward compatible).
func TestAuthTokenMiddleware(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	t.Run("no token configured is open", func(t *testing.T) {
		called = false
		h := authTokenMiddleware("")(next)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspace", nil)
		h.ServeHTTP(httptest.NewRecorder(), req)
		if !called {
			t.Fatal("expected handler to be called when no token is configured")
		}
	})

	t.Run("missing token rejected", func(t *testing.T) {
		called = false
		h := authTokenMiddleware("s3cret")(next)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspace", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if called {
			t.Fatal("handler must not run without a token")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if got := rec.Header().Get("WWW-Authenticate"); got == "" {
			t.Fatal("expected WWW-Authenticate header on 401")
		}
	})

	t.Run("wrong token rejected", func(t *testing.T) {
		called = false
		h := authTokenMiddleware("s3cret")(next)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspace", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		h.ServeHTTP(httptest.NewRecorder(), req)
		if called {
			t.Fatal("handler must not run with a wrong token")
		}
	})

	t.Run("bearer header accepted", func(t *testing.T) {
		called = false
		h := authTokenMiddleware("s3cret")(next)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspace", nil)
		req.Header.Set("Authorization", "Bearer s3cret")
		h.ServeHTTP(httptest.NewRecorder(), req)
		if !called {
			t.Fatal("handler must run with a valid Bearer token")
		}
	})

	t.Run("query param accepted for websocket", func(t *testing.T) {
		called = false
		h := authTokenMiddleware("s3cret")(next)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ws?token=s3cret", nil)
		h.ServeHTTP(httptest.NewRecorder(), req)
		if !called {
			t.Fatal("handler must run with a valid ?token= query (WebSocket clients)")
		}
	})

	t.Run("non-bearer scheme rejected", func(t *testing.T) {
		called = false
		h := authTokenMiddleware("s3cret")(next)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspace", nil)
		req.Header.Set("Authorization", "Basic s3cret")
		h.ServeHTTP(httptest.NewRecorder(), req)
		if called {
			t.Fatal("handler must not run for a non-Bearer scheme")
		}
	})
}
