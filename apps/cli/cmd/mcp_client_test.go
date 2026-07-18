package cmd

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

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

type cliEchoInput struct {
	Message string `json:"message" jsonschema:"message to echo"`
}

type cliEchoOutput struct {
	Echo string `json:"echo"`
}

func TestMCPProbeCommandJSONAndRequiredTools(t *testing.T) {
	server := newCLITestMCPServer(t)
	command := newMCPProbeCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"--url", server.URL, "--require-tool", "echo", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{`"ok": true`, `"transport": "streamable-http"`, `"name": "echo"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("probe output missing %q:\n%s", want, text)
		}
	}

	command = newMCPProbeCommand()
	output.Reset()
	command.SetOut(&output)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"--url", server.URL, "--require-tool", "missing", "--json"})
	err := command.Execute()
	assertExitCode(t, err, ExitTestFailure)
	if !strings.Contains(output.String(), `"missingTools":`) {
		t.Fatalf("failed probe did not emit its JSON report: %s", output.String())
	}
}

func TestMCPCallCommandSuccessAndToolError(t *testing.T) {
	server := newCLITestMCPServer(t)
	command := newMCPCallCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"echo", "--url", server.URL, "--args", `{"message":"hola"}`, "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"tool": "echo"`) || !strings.Contains(output.String(), `"ok": true`) {
		t.Fatalf("unexpected call output: %s", output.String())
	}

	command = newMCPCallCommand()
	output.Reset()
	command.SetOut(&output)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"fail", "--url", server.URL, "--json"})
	err := command.Execute()
	assertExitCode(t, err, ExitTestFailure)
	if !strings.Contains(output.String(), `"isError": true`) {
		t.Fatalf("tool error did not emit its result: %s", output.String())
	}
}

func TestMCPClientCommandsValidateUsageBeforeConnecting(t *testing.T) {
	tests := []struct {
		name    string
		command *cobra.Command
		args    []string
	}{
		{name: "probe missing target", command: newMCPProbeCommand()},
		{name: "probe invalid timeout", command: newMCPProbeCommand(), args: []string{"--timeout", "0s", "--url", "http://example.com"}},
		{name: "call invalid JSON", command: newMCPCallCommand(), args: []string{"echo", "--args", "[]", "--url", "http://example.com"}},
		{name: "call ambiguous target", command: newMCPCallCommand(), args: []string{"echo", "--url", "http://example.com", "--", "server"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.command.SetOut(&bytes.Buffer{})
			test.command.SetErr(&bytes.Buffer{})
			test.command.SetArgs(test.args)
			assertExitCode(t, test.command.Execute(), ExitUsageError)
		})
	}
}

func TestMCPClientCommandsAlwaysExplainEarlyErrors(t *testing.T) {
	t.Run("human diagnostic", func(t *testing.T) {
		command := newMCPProbeCommand()
		var stderr bytes.Buffer
		command.SetOut(&bytes.Buffer{})
		command.SetErr(&stderr)
		assertExitCode(t, command.Execute(), ExitUsageError)
		if !strings.Contains(stderr.String(), "Error:") || !strings.Contains(stderr.String(), "exactly one MCP target") {
			t.Fatalf("missing human diagnostic: %q", stderr.String())
		}
	})

	t.Run("JSON usage envelope", func(t *testing.T) {
		command := newMCPCallCommand()
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetErr(&bytes.Buffer{})
		command.SetArgs([]string{"echo", "--args", "[]", "--url", "http://example.com", "--json"})
		assertExitCode(t, command.Execute(), ExitUsageError)
		assertMCPErrorEnvelope(t, output.Bytes(), ExitUsageError, "usage")
	})

	t.Run("JSON unknown tool envelope", func(t *testing.T) {
		server := newCLITestMCPServer(t)
		command := newMCPCallCommand()
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetErr(&bytes.Buffer{})
		command.SetArgs([]string{"missing", "--url", server.URL, "--json"})
		assertExitCode(t, command.Execute(), ExitTestFailure)
		assertMCPErrorEnvelope(t, output.Bytes(), ExitTestFailure, "test")
	})
}

func TestMCPProbeCommandTimeout(t *testing.T) {
	stop := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodDelete {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		select {
		case <-request.Context().Done():
		case <-stop:
		}
	}))
	defer func() {
		close(stop)
		server.Close()
	}()
	command := newMCPProbeCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"--url", server.URL, "--timeout", "50ms", "--json"})
	assertExitCode(t, command.Execute(), ExitNetworkError)
	assertMCPErrorEnvelope(t, output.Bytes(), ExitNetworkError, "connection")
}

func TestReadMCPArgumentsInlineAndFile(t *testing.T) {
	inline, err := readMCPArguments(`{"count":2}`)
	if err != nil || inline["count"] != json.Number("2") {
		t.Fatalf("inline arguments = %#v, err=%v", inline, err)
	}
	large, err := readMCPArguments(`{"id":9007199254740993}`)
	if err != nil || large["id"] != json.Number("9007199254740993") {
		t.Fatalf("large integer lost precision: %#v, err=%v", large, err)
	}
	path := filepath.Join(t.TempDir(), "args.json")
	if err := os.WriteFile(path, []byte(`{"message":"from file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fromFile, err := readMCPArguments("@" + path)
	if err != nil || fromFile["message"] != "from file" {
		t.Fatalf("file arguments = %#v, err=%v", fromFile, err)
	}
	for _, invalid := range []string{"[]", "null", "{not-json}", "{} {}"} {
		if _, err := readMCPArguments(invalid); err == nil {
			t.Fatalf("readMCPArguments(%q) succeeded", invalid)
		}
	}
}

func assertMCPErrorEnvelope(t *testing.T, data []byte, wantCode int, wantKind string) {
	t.Helper()
	var report mcpErrorReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("invalid JSON error envelope %q: %v", data, err)
	}
	if report.OK || report.Error.Code != wantCode || report.Error.Kind != wantKind || report.Error.Message == "" {
		t.Fatalf("error envelope = %#v", report)
	}
}

func newCLITestMCPServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "cli-test", Version: "1.0.0"}, nil)
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "echo", Description: "Echo a message"}, func(
		_ context.Context,
		_ *sdkmcp.CallToolRequest,
		input cliEchoInput,
	) (*sdkmcp.CallToolResult, cliEchoOutput, error) {
		return nil, cliEchoOutput{Echo: input.Message}, nil
	})
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "fail", Description: "Fail intentionally"}, func(
		_ context.Context,
		_ *sdkmcp.CallToolRequest,
		_ struct{},
	) (*sdkmcp.CallToolResult, struct{}, error) {
		return nil, struct{}{}, errors.New("intentional failure")
	})
	handler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return server }, nil)
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)
	return httpServer
}

func assertExitCode(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatalf("command succeeded, want exit code %d", want)
	}
	coder, ok := err.(ExitCoder)
	if !ok {
		t.Fatalf("error type = %T (%v), want ExitCoder", err, err)
	}
	if coder.ExitCode() != want {
		t.Fatalf("exit code = %d, want %d (error: %v)", coder.ExitCode(), want, err)
	}
}
