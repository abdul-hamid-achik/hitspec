package serve

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T, opts ...Option) *Server {
	t.Helper()
	defaults := []Option{
		WithWorkDir(t.TempDir()),
		WithEnv("test"),
	}
	return NewServer(append(defaults, opts...)...)
}

func TestHandleSystemInfo(t *testing.T) {
	s := newTestServer(t)
	s.Version = "1.0.0"
	s.BuildTime = "2026-01-01"

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
	w := httptest.NewRecorder()
	s.handleSystemInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var info SystemInfoDTO
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if info.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", info.Version)
	}
	if info.GoVersion == "" {
		t.Error("goVersion should not be empty")
	}
}

func TestHandleGetWorkspace(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspace", nil)
	w := httptest.NewRecorder()
	s.handleGetWorkspace(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var ws WorkspaceDTO
	if err := json.NewDecoder(w.Body).Decode(&ws); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if ws.Environment != "test" {
		t.Errorf("env = %q, want test", ws.Environment)
	}
}

func TestRegisterRoutes_NoSPAFallback(t *testing.T) {
	s := newTestServer(t)
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	apiReq := httptest.NewRequest(http.MethodGet, "/api/v1/workspace", nil)
	apiW := httptest.NewRecorder()
	mux.ServeHTTP(apiW, apiReq)
	if apiW.Code != http.StatusOK {
		t.Fatalf("expected API route to remain available, got %d", apiW.Code)
	}

	spaReq := httptest.NewRequest(http.MethodGet, "/workspace", nil)
	spaW := httptest.NewRecorder()
	mux.ServeHTTP(spaW, spaReq)
	if spaW.Code != http.StatusNotFound {
		t.Fatalf("expected non-API route to 404 after SPA removal, got %d", spaW.Code)
	}
}

func TestHandleListFiles_Empty(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
	w := httptest.NewRecorder()
	s.handleListFiles(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var files []FileInfoDTO
	if err := json.NewDecoder(w.Body).Decode(&files); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestHandleGetHistory_Empty(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history", nil)
	w := httptest.NewRecorder()
	s.handleGetHistory(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var entries []HistoryEntryDTO
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestHandleListEnvironments(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments", nil)
	w := httptest.NewRecorder()
	s.handleListEnvironments(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var envs []EnvironmentDTO
	if err := json.NewDecoder(w.Body).Decode(&envs); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(envs) == 0 {
		t.Error("expected at least 1 environment (active)")
	}

	found := false
	for _, e := range envs {
		if e.Name == "test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find active env 'test'")
	}
}

func TestHandleSelectEnvironment(t *testing.T) {
	s := newTestServer(t)

	body := `{"name": "production"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/environments/active", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleSelectEnvironment(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	if s.config.Env != "production" {
		t.Errorf("env = %q, want production", s.config.Env)
	}
}

func TestHandleSelectEnvironment_EmptyName(t *testing.T) {
	s := newTestServer(t)

	body := `{"name": ""}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/environments/active", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleSelectEnvironment(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleGetConfig(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	w := httptest.NewRecorder()
	s.handleGetConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleStressStatus_NotRunning(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stress/status", nil)
	w := httptest.NewRecorder()
	s.handleStressStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var status StressStatusDTO
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if status.Running {
		t.Error("expected not running")
	}
}

func TestHandleMockRoutes_NotRunning(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mock/routes", nil)
	w := httptest.NewRecorder()
	s.handleMockRoutes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var status MockStatusDTO
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if status.Running {
		t.Error("expected not running")
	}
}

func TestHandleImportCurl(t *testing.T) {
	s := newTestServer(t)

	body := `{"command": "curl -X GET https://api.example.com/users"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/curl", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleImportCurl(w, req)

	if w.Code != http.StatusOK {
		b, _ := io.ReadAll(w.Body)
		t.Fatalf("expected 200, got %d: %s", w.Code, b)
	}

	var result ImportResultDTO
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if result.Content == "" {
		t.Error("expected non-empty content")
	}
}

func TestHandleExportCurl_MissingFile(t *testing.T) {
	s := newTestServer(t)

	body := `{"file": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/export/curl", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleExportCurl(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestReadOnlyMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := readOnlyMiddleware(true)
	wrapped := mw(handler)

	// PUT should be blocked
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("PUT expected 403, got %d", w.Code)
	}

	// GET should pass through
	req = httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	w = httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET expected 200, got %d", w.Code)
	}
}

func TestHistory(t *testing.T) {
	h := NewHistory()

	h.Add(HistoryEntryDTO{ID: "1", Method: "GET"})
	h.Add(HistoryEntryDTO{ID: "2", Method: "POST"})

	entries := h.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].ID != "1" {
		t.Errorf("first entry ID = %q, want 1", entries[0].ID)
	}
}

func TestIsPathWithin(t *testing.T) {
	if !isPathWithin("/home/user/project", "/home/user/project/file.http") {
		t.Error("expected path to be within base")
	}
	if isPathWithin("/home/user/project", "/home/user/other/file.http") {
		t.Error("expected path to be outside base")
	}
	if isPathWithin("/home/user/project", "/home/user/project/../other/file.http") {
		t.Error("expected traversal to be caught")
	}
}

func TestHelpers(t *testing.T) {
	id := generateID()
	if len(id) != 8 {
		t.Errorf("expected 8 char ID, got %d", len(id))
	}

	if !isHitspecFile("test.http") {
		t.Error("expected .http to be hitspec file")
	}
	if !isHitspecFile("test.hitspec") {
		t.Error("expected .hitspec to be hitspec file")
	}
	if isHitspecFile("test.go") {
		t.Error("expected .go to not be hitspec file")
	}
}
