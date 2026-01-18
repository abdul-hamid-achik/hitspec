package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "hitspec version %s\n", version)
		fmt.Fprintf(cmd.OutOrStdout(), "Built: %s\n", buildTime)
	},
}
