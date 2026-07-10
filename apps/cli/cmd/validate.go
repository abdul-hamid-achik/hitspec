package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/spf13/cobra"
)

var validateJSONFlag bool

var validateCmd = &cobra.Command{
	Use:   "validate <file|directory>",
	Short: "Validate hitspec files for syntax errors",
	Long: `Validate hitspec files for syntax errors without executing them.

Examples:
  hitspec validate api.http
  hitspec validate ./tests/
  hitspec validate ./tests/ --json`,
	Args:         cobra.MinimumNArgs(1),
	SilenceUsage: true,
	RunE:         validateCommand,
}

func init() {
	validateCmd.Flags().BoolVar(&validateJSONFlag, "json", false, "Output validation results as JSON")
}

// validateResult is the JSON shape emitted by `hitspec validate --json`.
type validateResult struct {
	File   string   `json:"file"`
	OK     bool     `json:"ok"`
	Errors []string `json:"errors,omitempty"`
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
	var jsonOut []validateResult
	for _, file := range files {
		parsed, err := parser.ParseFile(file)
		if err != nil {
			// Propagate parse errors (including the hardened "unclosed >>>"
			// assertion block error) instead of treating them as OK.
			msg := err.Error()
			if validateJSONFlag {
				jsonOut = append(jsonOut, validateResult{File: file, OK: false, Errors: []string{msg}})
			} else {
				fmt.Fprintf(cmd.OutOrStderr(), "FAIL  %s\n      %v\n", file, err)
			}
			errorCount++
			continue
		}
		if vErr := validateParsedFile(file, parsed); vErr != nil {
			msg := vErr.Error()
			if validateJSONFlag {
				jsonOut = append(jsonOut, validateResult{File: file, OK: false, Errors: []string{msg}})
			} else {
				fmt.Fprintf(cmd.OutOrStderr(), "FAIL  %s\n      %v\n", file, vErr)
			}
			errorCount++
			continue
		}
		if validateJSONFlag {
			jsonOut = append(jsonOut, validateResult{File: file, OK: true})
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "OK    %s\n", file)
		}
	}

	if validateJSONFlag {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		_ = enc.Encode(jsonOut)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "\n%d file(s) checked, %d error(s)\n", len(files), errorCount)
	}

	if errorCount > 0 {
		return newExitError(ExitParseError, fmt.Sprintf("%d file(s) failed validation", errorCount))
	}
	return nil
}

// validateParsedFile reports structural problems the parser accepts but that
// are still invalid hitspec: an empty file (no requests at all) and a request
// with an empty/missing URL. Both used to print "OK" and pass silently.
func validateParsedFile(file string, parsed *parser.File) error {
	if len(parsed.Requests) == 0 {
		return fmt.Errorf("%s: no requests found (empty file)", file)
	}
	for _, req := range parsed.Requests {
		if req.URL == "" {
			label := req.Name
			if label == "" {
				label = "request"
			}
			return fmt.Errorf("%s: %s has an empty URL (line %d)", file, label, req.Line)
		}
	}
	return nil
}
