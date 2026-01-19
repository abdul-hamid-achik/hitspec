package cmd

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
)

//go:embed llms.txt
var llmsTxt string

var llmsCmd = &cobra.Command{
	Use:   "llms",
	Short: "Output AI-readable documentation (llms.txt)",
	Long:  "Print the llms.txt content for AI agents to learn hitspec usage.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprint(cmd.OutOrStdout(), llmsTxt)
	},
}
