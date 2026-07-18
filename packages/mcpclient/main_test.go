package mcpclient

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	stdioHelperEnv = "HITSPEC_TEST_MCPCLIENT_HELPER"
	hangHelperEnv  = "HITSPEC_TEST_MCPCLIENT_HANG"
)

type echoInput struct {
	Message string `json:"message" jsonschema:"message to echo"`
}

type echoOutput struct {
	Echo string `json:"echo"`
}

func TestMain(m *testing.M) {
	if os.Getenv(hangHelperEnv) == "1" {
		for {
			time.Sleep(time.Hour)
		}
	}
	if os.Getenv(stdioHelperEnv) == "1" {
		_, _ = fmt.Fprintln(os.Stderr, "mcp helper log")
		server := testServer()
		if err := server.Run(context.Background(), &sdkmcp.StdioTransport{}); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func testServer() *sdkmcp.Server {
	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "mcpclient-test", Title: "MCP Client Test", Version: "1.0.0"},
		nil,
	)
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "echo", Description: "Echo a message"}, func(
		_ context.Context,
		_ *sdkmcp.CallToolRequest,
		input echoInput,
	) (*sdkmcp.CallToolResult, echoOutput, error) {
		return nil, echoOutput{Echo: input.Message}, nil
	})
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "fail", Description: "Return a tool execution error"}, func(
		_ context.Context,
		_ *sdkmcp.CallToolRequest,
		_ struct{},
	) (*sdkmcp.CallToolResult, struct{}, error) {
		return nil, struct{}{}, errors.New("intentional tool failure")
	})
	return server
}
