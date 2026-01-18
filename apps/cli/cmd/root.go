package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "hitspec",
	Short: "Plain text API tests. No magic.",
	Long: `hitspec is a file-based HTTP API testing tool that emphasizes
simplicity and readability. Write your API tests in plain text files
that look like actual HTTP requests.`,
}

func Execute(v, bt string) {
	version = v
	buildTime = bt
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
}
