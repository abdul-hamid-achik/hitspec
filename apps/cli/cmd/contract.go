package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/abdul-hamid-achik/hitspec/packages/contract"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	contractProviderFlag     string
	contractStateHandlerFlag string
	contractVerboseFlag      bool
)

var contractCmd = &cobra.Command{
	Use:   "contract",
	Short: "Contract testing commands",
	Long:  `Contract testing commands for verifying API contracts against providers.`,
}

var contractVerifyCmd = &cobra.Command{
	Use:   "verify <contracts-dir>",
	Short: "Verify contracts against a provider",
	Long: `Verify API contracts defined in hitspec files against a live provider.

The command reads contract files from the specified directory and verifies
each interaction against the provider URL.

Contract annotations:
  # @contract.state "user exists with id 1"  - Set up provider state
  # @contract.provider UserService           - Specify the provider name

Examples:
  hitspec contract verify contracts/ --provider http://localhost:3000
  hitspec contract verify contracts/ --provider http://localhost:3000 --state-handler ./setup-states.sh
  hitspec contract verify contracts/ --provider http://localhost:3000 -v`,
	Args: cobra.ExactArgs(1),
	RunE: contractVerifyCommand,
}

func init() {
	contractVerifyCmd.Flags().StringVarP(&contractProviderFlag, "provider", "p", "", "Provider URL (required)")
	contractVerifyCmd.Flags().StringVar(&contractStateHandlerFlag, "state-handler", "", "Path to state handler script")
	contractVerifyCmd.Flags().BoolVarP(&contractVerboseFlag, "verbose", "v", false, "Verbose output")

	contractVerifyCmd.MarkFlagRequired("provider")

	contractCmd.AddCommand(contractVerifyCmd)
}

func contractVerifyCommand(cmd *cobra.Command, args []string) error {
	contractsDir := args[0]

	// Verify directory exists
	info, err := os.Stat(contractsDir)
	if err != nil {
		return fmt.Errorf("cannot access contracts directory: %w", err)
	}

	// Create verifier
	verifier := contract.NewVerifier(
		contract.WithProviderURL(contractProviderFlag),
		contract.WithStateHandler(contractStateHandlerFlag),
		contract.WithVerbose(contractVerboseFlag),
	)

	// Verify contracts
	var results []*contract.VerificationResult
	if info.IsDir() {
		results, err = verifier.VerifyDirectory(contractsDir)
	} else {
		result, err := verifier.VerifyFile(contractsDir)
		if err != nil {
			return err
		}
		results = append(results, result)
	}

	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Print results
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()

	fmt.Printf("\n%s\n\n", bold("Contract Verification Results"))

	totalPassed := 0
	totalFailed := 0
	totalSkipped := 0

	for _, result := range results {
		relPath, _ := filepath.Rel(".", result.ContractFile)
		if relPath == "" {
			relPath = result.ContractFile
		}

		fmt.Printf("%s\n", bold(relPath))

		for _, interaction := range result.Results {
			var status string
			var statusSymbol string

			if interaction.Passed {
				status = green("passed")
				statusSymbol = green("✓")
			} else if interaction.Error != nil {
				status = red("error")
				statusSymbol = red("✗")
			} else {
				status = red("failed")
				statusSymbol = red("✗")
			}

			name := interaction.Name
			if name == "" {
				name = "(unnamed)"
			}

			fmt.Printf("  %s %s [%s] %s\n", statusSymbol, name, status, fmt.Sprintf("(%dms)", interaction.Duration.Milliseconds()))

			if interaction.Error != nil && contractVerboseFlag {
				fmt.Printf("    Error: %v\n", interaction.Error)
			}

			if !interaction.Passed && len(interaction.Assertions) > 0 {
				for _, a := range interaction.Assertions {
					if !a.Passed {
						fmt.Printf("    %s %s %s\n", red("→"), a.Subject, a.Operator)
						fmt.Printf("      Expected: %v\n", a.Expected)
						fmt.Printf("      Actual:   %v\n", a.Actual)
					}
				}
			}
		}

		totalPassed += result.Passed
		totalFailed += result.Failed
		totalSkipped += result.Skipped

		fmt.Println()
	}

	// Summary
	fmt.Printf("%s\n", bold("Summary"))
	fmt.Printf("  Contracts: %d file(s)\n", len(results))
	fmt.Printf("  Interactions: ")
	if totalPassed > 0 {
		fmt.Printf("%s, ", green(fmt.Sprintf("%d passed", totalPassed)))
	}
	if totalFailed > 0 {
		fmt.Printf("%s, ", red(fmt.Sprintf("%d failed", totalFailed)))
	}
	if totalSkipped > 0 {
		fmt.Printf("%s, ", yellow(fmt.Sprintf("%d skipped", totalSkipped)))
	}
	fmt.Printf("%d total\n\n", totalPassed+totalFailed+totalSkipped)

	if totalFailed > 0 {
		return fmt.Errorf("contract verification failed: %d interaction(s) failed", totalFailed)
	}

	fmt.Printf("%s All contracts verified successfully!\n", green("✓"))
	return nil
}
