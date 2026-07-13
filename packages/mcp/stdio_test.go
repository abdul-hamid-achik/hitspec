package mcp

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStdioServerNegotiatesAndListsTools(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), stdioHelperEnv+"=1")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "hitspec-stdio-test", Version: "0"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{
		Command: cmd, TerminateDuration: 3 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("stdio handshake: %v", err)
	}
	listed, err := session.ListTools(ctx, nil)
	if err != nil || len(listed.Tools) != 3 {
		t.Fatalf("tools/list = %#v, err=%v", listed, err)
	}
	foundFetch := false
	for _, tool := range listed.Tools {
		if tool.Name == "hitspec_fetch" {
			foundFetch = true
		}
	}
	if !foundFetch {
		t.Fatalf("tools/list omitted hitspec_fetch: %#v", listed.Tools)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close stdio session: %v", err)
	}
}
