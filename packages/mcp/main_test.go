package mcp

import (
	"context"
	"fmt"
	"os"
	"testing"
)

const stdioHelperEnv = "HITSPEC_TEST_MCP_STDIO_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(stdioHelperEnv) == "1" {
		server, err := NewServer("test", ".", Options{})
		if err == nil {
			err = server.Run(context.Background())
		}
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
