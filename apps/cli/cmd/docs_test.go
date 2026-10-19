package cmd

import (
	"os"
	"testing"
)

// The embedded copy must match the repository's llms.txt; run
// `go generate ./apps/cli/cmd` after editing llms.txt.
func TestEmbeddedLLMsMatchesRepoRoot(t *testing.T) {
	root, err := os.ReadFile("../../../llms.txt")
	if err != nil {
		t.Fatalf("read repo llms.txt: %v", err)
	}
	if string(root) != llmsTxt {
		t.Fatal("apps/cli/cmd/llms.txt is out of date: run `go generate ./apps/cli/cmd`")
	}
}
