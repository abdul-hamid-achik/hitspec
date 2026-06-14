package clientmgr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportCurl(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	out, err := m.ImportCurl(ctx, ImportCurlReq{Command: "curl -X POST https://api.example.com/users -d '{\"a\":1}'"})
	if err != nil {
		t.Fatalf("ImportCurl: %v", err)
	}
	if out.RequestCount < 1 || !strings.Contains(out.Content, "POST") {
		t.Fatalf("ImportCurl content = %q (count %d)", out.Content, out.RequestCount)
	}
	if _, err := m.ImportCurl(ctx, ImportCurlReq{}); err == nil {
		t.Fatal("ImportCurl with no input should error")
	}
}

func TestImportInsomnia(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	data := `{
      "_type": "export",
      "__export_format": 4,
      "resources": [
        {"_type": "request", "_id": "req_1", "name": "List users", "method": "GET", "url": "https://api.example.com/users"}
      ]
    }`
	out, err := m.ImportInsomnia(ctx, ImportInsomniaReq{Data: data})
	if err != nil {
		t.Fatalf("ImportInsomnia: %v", err)
	}
	if out.RequestCount < 1 || !strings.Contains(out.Content, "api.example.com/users") {
		t.Fatalf("ImportInsomnia content = %q (count %d)", out.Content, out.RequestCount)
	}
	if _, err := m.ImportInsomnia(ctx, ImportInsomniaReq{}); err == nil {
		t.Fatal("ImportInsomnia with no input should error")
	}
}

func TestImportPostman(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	data := `{
      "info": {"name": "T", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
      "item": [
        {"name": "List users", "request": {"method": "GET", "url": {"raw": "https://api.example.com/users"}}}
      ]
    }`
	out, err := m.ImportPostman(ctx, ImportPostmanReq{Data: data})
	if err != nil {
		t.Fatalf("ImportPostman: %v", err)
	}
	if out.RequestCount < 1 || !strings.Contains(out.Content, "api.example.com/users") {
		t.Fatalf("ImportPostman content = %q (count %d)", out.Content, out.RequestCount)
	}
	if _, err := m.ImportPostman(ctx, ImportPostmanReq{}); err == nil {
		t.Fatal("ImportPostman with no input should error")
	}
}

func TestImportOpenAPI(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	spec := `{
      "openapi": "3.0.0",
      "info": {"title": "T", "version": "1.0.0"},
      "servers": [{"url": "https://api.example.com"}],
      "paths": {"/users": {"get": {"summary": "List users", "responses": {"200": {"description": "ok"}}}}}
    }`
	// OpenAPI specs aren't .http files, so write directly into the workspace.
	if err := os.WriteFile(filepath.Join(m.config.WorkDir, "openapi.json"), []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	out, err := m.ImportOpenAPI(ctx, ImportOpenAPIReq{SpecPath: "openapi.json"})
	if err != nil {
		t.Fatalf("ImportOpenAPI: %v", err)
	}
	if out.RequestCount < 1 {
		t.Fatalf("ImportOpenAPI count = %d, want >=1", out.RequestCount)
	}
	if _, err := m.ImportOpenAPI(ctx, ImportOpenAPIReq{}); err == nil {
		t.Fatal("ImportOpenAPI with no specPath should error")
	}
	// SSRF guard: internal addresses are rejected.
	if _, err := m.ImportOpenAPI(ctx, ImportOpenAPIReq{SpecPath: "http://127.0.0.1/spec.json"}); err == nil {
		t.Fatal("ImportOpenAPI to an internal address should be rejected")
	}
}

func TestContractFilesAndVerifyGuards(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "contract.http", "### one\nGET https://example.com\n"); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	status, err := m.ContractFiles(ctx)
	if err != nil {
		t.Fatalf("ContractFiles: %v", err)
	}
	if len(status.Files) != 1 || status.Files[0] != "contract.http" {
		t.Fatalf("ContractFiles = %+v, want [contract.http]", status.Files)
	}

	// VerifyContracts requires a provider URL.
	if _, err := m.VerifyContracts(ctx, ContractVerifyReq{}); err == nil {
		t.Fatal("VerifyContracts without providerUrl should error")
	}

	// With a provider URL it returns per-file results (the facade conversion loop)
	// even when the file has no contract interactions.
	results, err := m.VerifyContracts(ctx, ContractVerifyReq{ProviderURL: "https://example.com", Files: []string{"contract.http"}})
	if err != nil {
		t.Fatalf("VerifyContracts: %v", err)
	}
	if len(results) != 1 || results[0].File != "contract.http" {
		t.Fatalf("VerifyContracts results = %+v", results)
	}
}
