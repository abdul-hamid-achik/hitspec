package serve

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestHandleExecuteRequest covers the POST /api/v1/execute handler (a serve
// test-gap area): a single named request executes and returns a run-result DTO
// with the right pass/fail counts.
func TestHandleExecuteRequest(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	dir := t.TempDir()
	s := newTestServer(t, WithWorkDir(dir))
	content := "### ping\n# @name ping\nGET " + backend.URL + "\n\n>>>\nexpect status 200\n<<<\n"
	if err := os.WriteFile(filepath.Join(dir, "api.http"), []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	body, _ := json.Marshal(ExecuteReq{File: "api.http", RequestName: "ping"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/execute", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.handleExecuteRequest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("execute status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var dto RunResultDTO
	if err := json.NewDecoder(rec.Body).Decode(&dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.Passed != 1 || len(dto.Results) != 1 {
		t.Fatalf("execute dto = %+v, want 1 passed / 1 result", dto)
	}
}

// TestHandleCreateAndSaveFile covers the mutating file handlers (a serve
// test-gap area): POST /files creates a file and PUT /files/{path} saves new
// content that a subsequent raw GET retrieves.
func TestHandleCreateAndSaveFile(t *testing.T) {
	dir := t.TempDir()
	s := newTestServer(t, WithWorkDir(dir))

	// Create
	createBody, _ := json.Marshal(struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}{Path: "new.http", Content: "### r\nGET https://example.com\n"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/files", bytes.NewReader(createBody))
	rec := httptest.NewRecorder()
	s.handleCreateFile(rec, createReq)
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 200/201; body: %s", rec.Code, rec.Body.String())
	}
	// Save new content
	saveReq := httptest.NewRequest(http.MethodPut, "/api/v1/files/new.http", bytes.NewReader([]byte("### r2\nGET https://example.com/x\n")))
	saveReq.SetPathValue("path", "new.http")
	rec2 := httptest.NewRecorder()
	s.handleSaveFile(rec2, saveReq)
	if rec2.Code != http.StatusOK {
		t.Fatalf("save status = %d, want 200; body: %s", rec2.Code, rec2.Body.String())
	}

	// Get raw content back
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/files/raw/new.http", nil)
	getReq.SetPathValue("path", "new.http")
	rec3 := httptest.NewRecorder()
	s.handleGetFileRaw(rec3, getReq)
	if rec3.Code != http.StatusOK {
		t.Fatalf("get-raw status = %d, want 200", rec3.Code)
	}
	if !bytes.Contains(rec3.Body.Bytes(), []byte("example.com/x")) {
		t.Errorf("raw content = %q, want the saved URL", rec3.Body.String())
	}
}
