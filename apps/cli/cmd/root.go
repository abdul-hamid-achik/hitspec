package cmd

import (
	"fmt"
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
that look like actual HTTP requests.

Quick start:
  hitspec init                          Create a new project
  hitspec studio                        Open the interactive app
  hitspec run api.http                  Run tests in a file
  hitspec run ./tests/ --env staging    Run all tests with an environment
  hitspec import openapi spec.yaml      Import from OpenAPI spec`,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute(v, bt string) {
	version = v
	buildTime = bt
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("hitspec version {{.Version}}\n")
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(docsCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(mockCmd)
	rootCmd.AddCommand(recordCmd)
	rootCmd.AddCommand(contractCmd)
	rootCmd.AddCommand(studioCmd)
	rootCmd.AddCommand(serveCmd)
}
