package cmd

import (
	"fmt"
	"os"

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
		return fmt.Errorf("no .http or .hitspec files found in %v", args)
	}

	errorCount := 0
	for _, file := range files {
		_, err := parser.ParseFile(file)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "FAIL  %s\n      %v\n", file, err)
			errorCount++
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "OK    %s\n", file)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n%d file(s) checked, %d error(s)\n", len(files), errorCount)

	if errorCount > 0 {
		os.Exit(ExitParseError)
	}

	return nil
}
