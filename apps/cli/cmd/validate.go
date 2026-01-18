package cmd

import (
	"fmt"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate <file|directory>",
	Short: "Validate hitspec files for syntax errors",
	Long: `Validate hitspec files for syntax errors without executing them.

Examples:
  hitspec validate api.http
  hitspec validate ./tests/`,
	Args: cobra.MinimumNArgs(1),
	RunE: validateCommand,
}

func validateCommand(cmd *cobra.Command, args []string) error {
	files, err := collectFiles(args)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no .http or .hitspec files found")
	}

	hasErrors := false
	for _, file := range files {
		_, err := parser.ParseFile(file)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Error in %s: %v\n", file, err)
			hasErrors = true
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Valid: %s\n", file)
		}
	}

	if hasErrors {
		return fmt.Errorf("validation failed")
	}

	return nil
}
