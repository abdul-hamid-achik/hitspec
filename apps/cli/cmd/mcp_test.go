package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpServeHelperEnv       = "HITSPEC_TEST_MCP_SERVE_HELPER"
	mcpServeWorkspaceEnv    = "HITSPEC_TEST_MCP_SERVE_WORKSPACE"
	mcpServeFcheapPathEnv   = "HITSPEC_TEST_MCP_SERVE_FCHEAP_PATH"
	mcpServeFakeTavilyToken = "test-only-tavily-key"
)

func TestMain(m *testing.M) {
	if os.Getenv(mcpServeHelperEnv) == "1" {
		command := newMCPServeCommand()
		command.SetArgs([]string{
			"--workspace", os.Getenv(mcpServeWorkspaceEnv),
			"--search-provider", "tavily",
			"--fcheap-path", os.Getenv(mcpServeFcheapPathEnv),
		})
		if err := command.Execute(); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	}
	rootCommandFlagDefaults = captureRootCommandFlagDefaults(rootCmd)
	os.Exit(m.Run())
}

func TestMCPServeCommandValidatesLimitsWithoutStartingTransport(t *testing.T) {
	command := newMCPServeCommand()
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"--max-body-bytes", "0"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "max-body-bytes") {
		t.Fatalf("error = %v, want max-body-bytes validation", err)
	}
}

func TestMCPServeCommandNegotiatesWithConfiguredCapabilities(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(workspace, "contract.http"),
		[]byte("### Contract\n# @name contract\nGET https://example.com\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	fcheapPath, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0])
	child.Env = []string{
		mcpServeHelperEnv + "=1",
		mcpServeWorkspaceEnv + "=" + workspace,
		mcpServeFcheapPathEnv + "=" + fcheapPath,
		"TAVILY_API_KEY=" + mcpServeFakeTavilyToken,
	}
	var stderr bytes.Buffer
	child.Stderr = &stderr

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "hitspec-cobra-test", Version: "0"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{
		Command: child, TerminateDuration: 3 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("Cobra stdio handshake: %v\nstderr: %s", err, stderr.String())
	}
	initialization := session.InitializeResult()
	if initialization == nil ||
		!strings.Contains(initialization.Instructions, "hitspec_search_web") ||
		!strings.Contains(initialization.Instructions, "hitspec_capture_webpage") {
		t.Fatalf("initialize result = %#v", initialization)
	}
	listed, err := session.ListTools(ctx, nil)
	if err != nil || len(listed.Tools) != 5 {
		t.Fatalf("tools/list = %#v, err=%v\nstderr: %s", listed, err, stderr.String())
	}
	byName := make(map[string]bool, len(listed.Tools))
	for _, tool := range listed.Tools {
		byName[tool.Name] = true
	}
	for _, name := range []string{
		"hitspec_fetch",
		"hitspec_search_web",
		"hitspec_capture_webpage",
		"hitspec_list_requests",
		"hitspec_validate",
	} {
		if !byName[name] {
			t.Errorf("tools/list omitted %q", name)
		}
	}
	discovered, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "hitspec_list_requests", Arguments: map[string]any{},
	})
	if err != nil || discovered.IsError {
		t.Fatalf("workspace discovery = %#v, err=%v", discovered, err)
	}
	if text := discovered.Content[0].(*sdkmcp.TextContent).Text; !strings.Contains(text, `"contract"`) {
		t.Fatalf("workspace flag was not applied: %s", text)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close stdio session: %v\nstderr: %s", err, stderr.String())
	}
}

func TestMCPServeCommandFailsClosedForInvalidServerOwnedProviders(t *testing.T) {
	t.Setenv("HITSPEC_SEARCH_PROVIDER", "")
	t.Setenv("HITSPEC_FCHEAP_PATH", "")
	t.Setenv("TAVILY_API_KEY", "")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown search provider", args: []string{"--search-provider", "model-choice"}, want: "unsupported search provider"},
		{name: "missing tavily key", args: []string{"--search-provider", "tavily"}, want: "TAVILY_API_KEY is required"},
		{name: "missing artifact executable", args: []string{"--fcheap-path", "hitspec-definitely-missing-fcheap"}, want: "resolve file.cheap executable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := newMCPServeCommand()
			command.SetOut(&bytes.Buffer{})
			command.SetErr(&bytes.Buffer{})
			command.SetArgs(test.args)
			err := command.Execute()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
