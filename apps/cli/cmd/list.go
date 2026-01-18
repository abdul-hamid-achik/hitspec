package cmd

import (
	"fmt"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list <file|directory>",
	Short: "List all tests in hitspec files",
	Long: `List all tests defined in .http or .hitspec files.

Examples:
  hitspec list api.http
  hitspec list ./tests/`,
	Args: cobra.MinimumNArgs(1),
	RunE: listCommand,
}

func listCommand(cmd *cobra.Command, args []string) error {
	files, err := collectFiles(args)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no .http or .hitspec files found")
	}

	for _, file := range files {
		f, err := parser.ParseFile(file)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Error parsing %s: %v\n", file, err)
			continue
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\n%s:\n", file)
		for _, req := range f.Requests {
			name := req.Name
			if name == "" {
				name = fmt.Sprintf("%s %s", req.Method, req.URL)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", name)
			if len(req.Tags) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "    tags: %v\n", req.Tags)
			}
		}
	}

	return nil
}
