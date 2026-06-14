package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
	"github.com/spf13/cobra"
)

var forceInit bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new hitspec project",
	Long: `Initialize a new hitspec project in the current directory.

This creates:
  - hitspec.yaml   - Configuration file with environments
  - example.http   - Example test file

Examples:
  hitspec init
  hitspec init --force`,
	RunE: initCommand,
}

func init() {
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Overwrite existing files")
}

func initCommand(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	configFile := filepath.Join(cwd, clientmgr.SampleConfigFile)
	exampleFile := filepath.Join(cwd, clientmgr.SampleRequestFile)

	if !forceInit {
		for _, f := range []string{configFile, exampleFile} {
			if _, err := os.Stat(f); err == nil {
				return fmt.Errorf("file already exists: %s (use --force to overwrite)", f)
			}
		}
	}

	// The sample config and example file are shared with the in-app "generate
	// sample project" action (clientmgr.ScaffoldSample) so both stay in sync.
	if err := os.WriteFile(configFile, []byte(clientmgr.SampleConfigYAML), 0644); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", configFile)

	if err := os.WriteFile(exampleFile, []byte(clientmgr.SampleRequestHTTP), 0644); err != nil {
		return fmt.Errorf("failed to create example file: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", exampleFile)

	fmt.Fprintf(cmd.OutOrStdout(), "\nProject initialized. Next steps:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  hitspec studio                    Open the interactive app\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  hitspec run example.http          Run the example tests\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  hitspec run example.http -v       Run with verbose output\n")

	return nil
}
