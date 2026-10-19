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

// Request warnings are part of the run response.
func TestRunResponseCarriesWarnings(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	dir := t.TempDir()
	src := "### W\nGET " + backend.URL + "\nX-Token: {{nobody.token}}\n"
	if err := os.WriteFile(filepath.Join(dir, "w.http"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newTestServer(t, WithWorkDir(dir))

	body, _ := json.Marshal(RunReq{File: "w.http"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleRunFile(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var got RunResultDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 || len(got.Results[0].Warnings) == 0 {
		t.Fatalf("no warnings in %s", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"warnings":`)) {
		t.Errorf("response lacks a warnings key: %s", rec.Body.String())
	}
}
