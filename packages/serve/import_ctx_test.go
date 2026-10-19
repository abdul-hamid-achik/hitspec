package serve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A cancelled request stops fetching a remote OpenAPI spec.
func TestImportOpenAPIHonorsRequestContext(t *testing.T) {
	s := NewServer(WithWorkDir(t.TempDir()), WithEnv("test"), WithLogLevel("error"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/openapi", strings.NewReader(`{"specPath":"https://spec.example.invalid/openapi.yaml"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleImportOpenAPI(w, req)
	if !strings.Contains(w.Body.String(), "context canceled") {
		t.Fatalf("got %d %s; want a context cancellation", w.Code, w.Body.String())
	}
}
