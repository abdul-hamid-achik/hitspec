package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerToolsFetchListAndValidate(t *testing.T) {
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0, 0xff, 1})
	}))
	defer web.Close()
	workspace := t.TempDir()
	file := filepath.Join(workspace, "api.http")
	if err := os.WriteFile(file, []byte("### Sample\n# @name sample\n# @tags smoke\nGET "+web.URL+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	server, err := NewServer("test", workspace, Options{AllowPrivateNetwork: true, MaxBodyBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	session := connect(t, server)
	listed, err := session.ListTools(context.Background(), nil)
	if err != nil || len(listed.Tools) != 3 {
		t.Fatalf("tools=%#v err=%v", listed, err)
	}

	raw, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "hitspec_fetch", Arguments: map[string]any{"url": web.URL, "format": "raw"},
	})
	if err != nil || raw.IsError || len(raw.Content) != 1 {
		t.Fatalf("raw=%#v err=%v", raw, err)
	}
	text := raw.Content[0].(*sdkmcp.TextContent).Text
	if !strings.Contains(text, `"encoding": "base64"`) || !strings.Contains(text, `"data": "AP8B"`) {
		t.Fatalf("raw text content is not a base64 envelope: %s", text)
	}

	discovered, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "hitspec_list_requests", Arguments: map[string]any{}})
	if err != nil || discovered.IsError {
		t.Fatalf("list=%#v err=%v", discovered, err)
	}
	listText := discovered.Content[0].(*sdkmcp.TextContent).Text
	if !strings.Contains(listText, `"sample"`) || !strings.Contains(listText, `"api.http"`) || discovered.StructuredContent != nil {
		t.Fatalf("unexpected list output: %s", listText)
	}

	validated, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "hitspec_validate", Arguments: map[string]any{"file": "api.http"}})
	if err != nil || validated.IsError {
		t.Fatalf("validate=%#v err=%v", validated, err)
	}
	validationText := validated.Content[0].(*sdkmcp.TextContent).Text
	if !strings.Contains(validationText, `"valid": true`) || validated.StructuredContent != nil {
		t.Fatalf("unexpected validation output: %s", validationText)
	}

	saved, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "hitspec_fetch", Arguments: map[string]any{"file": "api.http", "name": "sample", "format": "raw"},
	})
	if err != nil || saved.IsError {
		t.Fatalf("saved fetch=%#v err=%v", saved, err)
	}
}

func TestServerRejectsPrivateTargetsAndWorkspaceEscape(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	server, err := NewServer("test", workspace, Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = server.handleFetch(context.Background(), nil, fetchInput{URL: "http://127.0.0.1/"})
	if err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("private target error = %v", err)
	}
	if _, err := server.resolvePath("/etc/passwd"); err == nil || !strings.Contains(err.Error(), "workspace-relative") {
		t.Fatalf("absolute path error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(parent, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "escape.http"), []byte("POST https://example.com\n\n< ../secret.txt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err = server.handleFetch(context.Background(), nil, fetchInput{File: "escape.http", Format: "raw"})
	if err == nil || !strings.Contains(err.Error(), "escapes the workspace") {
		t.Fatalf("body file escape error = %v", err)
	}
}

func TestListRequestsReportsInvalidFiles(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(workspace, "invalid.http"),
		[]byte("GET https://example.com\n\n>>>mock\n{}\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	server, err := NewServer("test", workspace, Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = server.handleList(context.Background(), nil, listInput{})
	if err == nil || !strings.Contains(err.Error(), "invalid.http") {
		t.Fatalf("list error = %v, want invalid file path", err)
	}
}

func connect(t *testing.T, server *Server) *sdkmcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
	go func() { _ = server.serve(ctx, serverTransport) }()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "hitspec-test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
		cancel()
	})
	return session
}
