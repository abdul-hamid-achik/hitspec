package cmd

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
)

//go:generate go run gen_llms.go

// llmsTxt is a copy of the repository's llms.txt (kept in sync by go generate
// and TestEmbeddedLLMsMatchesRepoRoot).
//
//go:embed llms.txt
var llmsTxt string

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Output machine-readable documentation (llms.txt)",
	Long: `Print structured documentation suitable for AI coding assistants.

Pipe the output to a file or directly into an LLM context:
  hitspec docs > llms.txt
  hitspec docs | pbcopy`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprint(cmd.OutOrStdout(), llmsTxt)
	},
}
