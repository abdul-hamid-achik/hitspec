//go:build ignore

// gen_llms copies the repository's llms.txt next to docs.go so it can be
// embedded (go:embed cannot reach parent directories). Run via go generate;
// TestEmbeddedLLMsMatchesRepoRoot fails if the two copies drift.
package main

import (
	"fmt"
	"os"
)

func main() {
	content, err := os.ReadFile("../../../llms.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading llms.txt: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("llms.txt", content, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing llms.txt: %v\n", err)
		os.Exit(1)
	}
}
