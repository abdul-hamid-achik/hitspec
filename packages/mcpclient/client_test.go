package mcpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestTargetValidate(t *testing.T) {
	tests := []struct {
		name    string
		target  Target
		wantErr string
	}{
		{name: "stdio", target: Target{Command: []string{"server"}}},
		{name: "http", target: Target{URL: "https://example.com/mcp"}},
		{name: "missing", target: Target{}, wantErr: "exactly one"},
		{name: "ambiguous", target: Target{URL: "https://example.com/mcp", Command: []string{"server"}}, wantErr: "exactly one"},
		{name: "bad scheme", target: Target{URL: "file:///tmp/mcp"}, wantErr: "scheme"},
		{name: "URL credentials", target: Target{URL: "https://secret@example.com/mcp"}, wantErr: "must not contain credentials"},
		{name: "URL fragment", target: Target{URL: "https://example.com/mcp#secret"}, wantErr: "fragment"},
		{name: "managed header", target: Target{URL: "https://example.com/mcp", Headers: []string{"MCP-Session-Id: secret"}}, wantErr: "managed"},
		{name: "stdio header", target: Target{Command: []string{"server"}, Headers: []string{"Authorization: secret"}}, wantErr: "only valid with --url"},
		{name: "http env", target: Target{URL: "https://example.com/mcp", Env: []string{"TOKEN=secret"}}, wantErr: "only valid for stdio"},
		{name: "bad env", target: Target{Command: []string{"server"}, Env: []string{"TOKEN"}}, wantErr: "KEY=VALUE"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.target.Validate()
			if test.wantErr == "" && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("Validate() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestProbePaginatesSortsAndChecksRequiredTools(t *testing.T) {
	session := &fakeSession{
		init: initialized(),
		pages: map[string]*sdkmcp.ListToolsResult{
			"": {
				Tools:      []*sdkmcp.Tool{{Name: "zeta", InputSchema: map[string]any{"type": "object"}}},
				NextCursor: "page-2",
			},
			"page-2": {
				Tools: []*sdkmcp.Tool{{Name: "alpha", InputSchema: map[string]any{"type": "object"}}},
			},
		},
	}
	client := &Client{session: session, transport: "test"}
	report, err := client.Probe(context.Background(), []string{"missing", "alpha", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || len(report.MissingTools) != 1 || report.MissingTools[0] != "missing" {
		t.Fatalf("unexpected requirement report: %#v", report)
	}
	if len(report.Tools) != 2 || report.Tools[0].Name != "alpha" || report.Tools[1].Name != "zeta" {
		t.Fatalf("tools not sorted: %#v", report.Tools)
	}
	if strings.Join(session.cursors, ",") != ",page-2" {
		t.Fatalf("pagination cursors = %#v", session.cursors)
	}
}

func TestProbeRejectsCursorCycleAndInvalidSchema(t *testing.T) {
	t.Run("cursor cycle", func(t *testing.T) {
		session := &fakeSession{
			init: initialized(),
			pages: map[string]*sdkmcp.ListToolsResult{
				"":      {Tools: []*sdkmcp.Tool{}, NextCursor: "again"},
				"again": {Tools: []*sdkmcp.Tool{}, NextCursor: "again"},
			},
		}
		_, err := (&Client{session: session}).Probe(context.Background(), nil)
		if err == nil || !strings.Contains(err.Error(), "repeated pagination cursor") {
			t.Fatalf("Probe() error = %v", err)
		}
	})
	t.Run("schema must be object", func(t *testing.T) {
		session := &fakeSession{
			init: initialized(),
			pages: map[string]*sdkmcp.ListToolsResult{
				"": {Tools: []*sdkmcp.Tool{{Name: "bad", InputSchema: []any{"not", "an", "object"}}}},
			},
		}
		_, err := (&Client{session: session}).Probe(context.Background(), nil)
		if err == nil || !strings.Contains(err.Error(), "schema must be a JSON object") {
			t.Fatalf("Probe() error = %v", err)
		}
	})
}

func TestProbeRejectsMalformedToolDiscovery(t *testing.T) {
	tests := []struct {
		name    string
		session *fakeSession
		want    string
	}{
		{
			name: "missing capability",
			session: &fakeSession{
				init:  &sdkmcp.InitializeResult{ProtocolVersion: "2025-11-25", ServerInfo: &sdkmcp.Implementation{Name: "test", Version: "1"}, Capabilities: &sdkmcp.ServerCapabilities{}},
				pages: map[string]*sdkmcp.ListToolsResult{},
			},
			want: "tools capability",
		},
		{
			name:    "null tool",
			session: &fakeSession{init: initialized(), pages: map[string]*sdkmcp.ListToolsResult{"": {Tools: []*sdkmcp.Tool{nil}}}},
			want:    "null tool",
		},
		{
			name: "invalid output schema",
			session: &fakeSession{init: initialized(), pages: map[string]*sdkmcp.ListToolsResult{
				"": {Tools: []*sdkmcp.Tool{{Name: "bad", InputSchema: map[string]any{}, OutputSchema: "not-an-object"}}},
			}},
			want: "output schema",
		},
		{
			name:    "missing tools array",
			session: &fakeSession{init: initialized(), pages: map[string]*sdkmcp.ListToolsResult{"": {}}},
			want:    "omitted tools",
		},
		{
			name: "invalid JSON Schema keyword",
			session: &fakeSession{init: initialized(), pages: map[string]*sdkmcp.ListToolsResult{
				"": {Tools: []*sdkmcp.Tool{{Name: "bad", InputSchema: map[string]any{"type": 7}}}},
			}},
			want: "invalid input JSON Schema",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := (&Client{session: test.session}).Probe(context.Background(), nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Probe() error = %v, want %q", err, test.want)
			}
		})
	}

	session := &fakeSession{init: initialized(), pages: map[string]*sdkmcp.ListToolsResult{
		"": {Tools: []*sdkmcp.Tool{
			{Name: "bad name", InputSchema: map[string]any{}},
			{Name: "bad name", InputSchema: map[string]any{}},
		}},
	}}
	report, err := (&Client{session: session}).Probe(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Warnings) != 2 {
		t.Fatalf("warnings = %#v, want duplicate and invalid-name warnings", report.Warnings)
	}
}

func TestCallDistinguishesToolErrorAndUnknownTool(t *testing.T) {
	session := &fakeSession{
		init: initialized(),
		pages: map[string]*sdkmcp.ListToolsResult{
			"": {Tools: []*sdkmcp.Tool{{Name: "fail", InputSchema: map[string]any{"type": "object"}}}},
		},
		callResult: &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "expected failure"}},
			IsError: true,
		},
	}
	client := &Client{session: session, transport: "test"}
	report, err := client.Call(context.Background(), "fail", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !report.IsError || len(report.Content) != 1 {
		t.Fatalf("unexpected call report: %#v", report)
	}
	_, err = client.Call(context.Background(), "unknown", map[string]any{})
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("unknown tool error = %v", err)
	}
}

func TestCallRejectsMalformedResults(t *testing.T) {
	base := func(result *sdkmcp.CallToolResult) *Client {
		return &Client{session: &fakeSession{
			init: initialized(),
			pages: map[string]*sdkmcp.ListToolsResult{
				"": {Tools: []*sdkmcp.Tool{{Name: "tool", InputSchema: map[string]any{}}}},
			},
			callResult: result,
		}}
	}
	if _, err := base(nil).Call(context.Background(), "tool", map[string]any{}); err == nil || !strings.Contains(err.Error(), "no result") {
		t.Fatalf("nil result error = %v", err)
	}
	if _, err := base(&sdkmcp.CallToolResult{}).Call(context.Background(), "tool", map[string]any{}); err == nil || !strings.Contains(err.Error(), "omitted content") {
		t.Fatalf("missing content error = %v", err)
	}
	var nilContent sdkmcp.Content
	if _, err := base(&sdkmcp.CallToolResult{Content: []sdkmcp.Content{nilContent}}).Call(context.Background(), "tool", map[string]any{}); err == nil || !strings.Contains(err.Error(), "null content") {
		t.Fatalf("nil content error = %v", err)
	}
}

func TestCallValidatesAdvertisedInputAndOutputSchemas(t *testing.T) {
	inputSchema := map[string]any{
		"type":       "object",
		"required":   []any{"message"},
		"properties": map[string]any{"message": map[string]any{"type": "string"}},
	}
	outputSchema := map[string]any{
		"type":       "object",
		"required":   []any{"echo"},
		"properties": map[string]any{"echo": map[string]any{"type": "string"}},
	}
	session := &fakeSession{
		init: initialized(),
		pages: map[string]*sdkmcp.ListToolsResult{
			"": {Tools: []*sdkmcp.Tool{{Name: "echo", InputSchema: inputSchema, OutputSchema: outputSchema}}},
		},
		callResult: &sdkmcp.CallToolResult{
			Content:           []sdkmcp.Content{&sdkmcp.TextContent{Text: "ok"}},
			StructuredContent: map[string]any{"echo": "ok"},
		},
	}
	client := &Client{session: session}
	if _, err := client.Call(context.Background(), "echo", map[string]any{}); !errors.Is(err, ErrContractViolation) {
		t.Fatalf("invalid arguments error = %v", err)
	}
	session.callResult.StructuredContent = map[string]any{"echo": 7}
	if _, err := client.Call(context.Background(), "echo", map[string]any{"message": "ok"}); !errors.Is(err, ErrContractViolation) {
		t.Fatalf("invalid output error = %v", err)
	}
	session.callResult.StructuredContent = nil
	if _, err := client.Call(context.Background(), "echo", map[string]any{"message": "ok"}); !errors.Is(err, ErrContractViolation) {
		t.Fatalf("missing structured content error = %v", err)
	}
}

func TestCallValidatesJSONNumberWithoutChangingWireArguments(t *testing.T) {
	arguments := map[string]any{"id": json.Number("9007199254740993")}
	session := &fakeSession{
		init: initialized(),
		pages: map[string]*sdkmcp.ListToolsResult{
			"": {Tools: []*sdkmcp.Tool{{
				Name: "lookup",
				InputSchema: map[string]any{
					"type":       "object",
					"required":   []any{"id"},
					"properties": map[string]any{"id": map[string]any{"type": "integer"}},
				},
			}}},
		},
		callResult: &sdkmcp.CallToolResult{Content: []sdkmcp.Content{}},
	}
	if _, err := (&Client{session: session}).Call(context.Background(), "lookup", arguments); err != nil {
		t.Fatal(err)
	}
	if arguments["id"] != json.Number("9007199254740993") {
		t.Fatalf("wire arguments were changed: %#v", arguments)
	}
}

func TestConnectAndCallOverStdioWithoutShell(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	var stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := Connect(ctx, Target{
		Command: []string{os.Args[0], "-test.run=^$", "--", "$(touch " + marker + ")"},
		Env:     []string{stdioHelperEnv + "=1"},
		Stderr:  &stderr,
	}, "test")
	if err != nil {
		t.Fatal(err)
	}
	report, err := client.Probe(ctx, []string{"echo"})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Transport != "stdio" {
		t.Fatalf("unexpected probe: %#v", report)
	}
	call, err := client.Call(ctx, "echo", map[string]any{"message": "hola"})
	if err != nil {
		t.Fatal(err)
	}
	if !call.OK || call.IsError {
		t.Fatalf("unexpected call: %#v", call)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "mcp helper log") {
		t.Fatalf("stderr was not forwarded separately: %q", stderr.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("stdio argv was interpreted by a shell; marker error = %v", err)
	}
}

func TestConnectAndCallOverStreamableHTTPWithHeader(t *testing.T) {
	const headerName = "X-MCP-Test"
	server := testServer()
	handler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return server }, nil)
	httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get(headerName) != "present" {
			http.Error(writer, "missing test header", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(writer, request)
	}))
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := Connect(ctx, Target{URL: httpServer.URL, Headers: []string{headerName + ": present"}}, "test")
	if err != nil {
		t.Fatal(err)
	}
	report, err := client.Call(ctx, "echo", map[string]any{"message": "http"})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Transport != "streamable-http" {
		t.Fatalf("unexpected HTTP call report: %#v", report)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestConnectHonorsTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := Connect(ctx, Target{
		Command: []string{os.Args[0], "-test.run=^$"},
		Env:     []string{hangHelperEnv + "=1"},
	}, "test")
	if err == nil {
		t.Fatal("Connect() succeeded, want timeout")
	}
}

func TestRenderersEscapeTerminalControls(t *testing.T) {
	var output bytes.Buffer
	report := &ProbeReport{
		Server:       ServerInfo{Name: "server\x1b[31m", Version: "1"},
		Tools:        []Tool{{Name: "tool", Description: "safe\x1b[2J"}},
		MissingTools: []string{"missing\x1b[2J"},
	}
	if err := WriteProbeHuman(&output, report); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(output.String(), '\x1b') {
		t.Fatalf("human output contains terminal escape: %q", output.String())
	}
	output.Reset()
	report.OK = false
	report.MissingTools = []string{"missing"}
	if err := WriteProbeHuman(&output, report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Probe: FAILED") || strings.Contains(output.String(), "Probe: OK") {
		t.Fatalf("failed probe status is misleading: %q", output.String())
	}
	output.Reset()
	if err := WriteJSON(&output, report); err != nil {
		t.Fatal(err)
	}
	var decoded ProbeReport
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	output.Reset()
	call := &CallReport{
		Tool:              "echo",
		Content:           []json.RawMessage{json.RawMessage(`{"type":"text","text":"hello\u001b[2J"}`), json.RawMessage(`{"type":"image","data":"","mimeType":"image/png"}`)},
		StructuredContent: map[string]any{"echo": "hello"},
	}
	if err := WriteCallHuman(&output, call); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(output.String(), '\x1b') || !strings.Contains(output.String(), "Structured content") {
		t.Fatalf("unsafe or incomplete call output: %q", output.String())
	}
}

func TestHeaderAndEnvironmentHelpers(t *testing.T) {
	headers, err := parseHeaders([]string{"Authorization: Bearer secret", "X-Test: one", "X-Test: two"})
	if err != nil {
		t.Fatal(err)
	}
	if len(headers.Values("X-Test")) != 2 {
		t.Fatalf("headers = %#v", headers)
	}
	if _, err := parseHeaders([]string{"X-Test: bad\nvalue"}); err == nil {
		t.Fatal("control character header succeeded")
	}
	if _, err := parseHeaders([]string{"Bad Name: value"}); err == nil {
		t.Fatal("invalid header name succeeded")
	}
	if _, err := parseHeaders([]string{"X-Test: bad\x1bvalue"}); err == nil {
		t.Fatal("invalid header value succeeded")
	}
	merged := mergeEnvironment([]string{"A=old", "B=keep"}, []string{"A=new", "C=added"})
	if strings.Join(merged, ",") != "A=new,B=keep,C=added" {
		t.Fatalf("merged environment = %#v", merged)
	}
}

type fakeSession struct {
	init       *sdkmcp.InitializeResult
	pages      map[string]*sdkmcp.ListToolsResult
	cursors    []string
	callResult *sdkmcp.CallToolResult
	callErr    error
}

func (s *fakeSession) InitializeResult() *sdkmcp.InitializeResult { return s.init }

func (s *fakeSession) ListTools(_ context.Context, params *sdkmcp.ListToolsParams) (*sdkmcp.ListToolsResult, error) {
	cursor := ""
	if params != nil {
		cursor = params.Cursor
	}
	s.cursors = append(s.cursors, cursor)
	return s.pages[cursor], nil
}

func (s *fakeSession) CallTool(_ context.Context, _ *sdkmcp.CallToolParams) (*sdkmcp.CallToolResult, error) {
	return s.callResult, s.callErr
}

func (*fakeSession) Close() error { return nil }

func initialized() *sdkmcp.InitializeResult {
	return &sdkmcp.InitializeResult{
		ProtocolVersion: "2025-11-25",
		ServerInfo:      &sdkmcp.Implementation{Name: "test", Version: "1"},
		Capabilities:    &sdkmcp.ServerCapabilities{Tools: &sdkmcp.ToolCapabilities{}},
	}
}
