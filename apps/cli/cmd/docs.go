package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

//go:generate go run gen_llms.go

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Output AI-readable documentation (llms.txt)",
	Long:  "Print the llms.txt content for AI agents to learn hitspec usage.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprint(cmd.OutOrStdout(), llmsTxt)
	},
}
