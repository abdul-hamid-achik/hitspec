package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/spf13/cobra"
)

var (
	validateJSONFlag   bool
	validateStrictFlag bool
)

var validateCmd = &cobra.Command{
	Use:   "validate <file|directory>",
	Short: "Validate hitspec files for syntax errors",
	Long: `Validate hitspec files for syntax errors without executing them.

Errors (exit code 2) are constructs that would give a wrong result or that
"hitspec run" refuses: an unknown assertion operator, an operator without a
value, a misspelt block, duplicate request names, circular @depends.

Warnings are constructs that are probably mistakes but may be intentional: an
unknown # @directive (a typo such as "# @tag"), a @depends on a request that
does not exist, a capture path with spaces. With --strict, warnings are errors
and every {{variable}} must also be defined somewhere: in the file, a capture,
hitspec.yaml environments, a .env file next to the file, or the process
environment.

Examples:
  hitspec validate api.http
  hitspec validate ./tests/
  hitspec validate ./tests/ --strict
  hitspec validate ./tests/ --json`,
	Args:         cobra.MinimumNArgs(1),
	SilenceUsage: true,
	RunE:         validateCommand,
}

func init() {
	validateCmd.Flags().BoolVar(&validateJSONFlag, "json", false, "Output validation results as JSON")
	validateCmd.Flags().BoolVar(&validateStrictFlag, "strict", false, "Treat warnings as errors and check that every {{variable}} is defined")
}

// validateResult is the JSON shape emitted by `hitspec validate --json`.
type validateResult struct {
	File     string   `json:"file"`
	OK       bool     `json:"ok"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func validateCommand(cmd *cobra.Command, args []string) error {
	files, err := collectFiles(args)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no .http or .hitspec files found in %v", args)
	}

	errorCount, warningCount := 0, 0
	var jsonOut []validateResult
	for _, file := range files {
		errs, warns := validateFile(file, validateStrictFlag)
		if validateStrictFlag && len(warns) > 0 {
			// --strict: a warning is an error.
			errs = append(errs, warns...)
			warns = nil
		}
		warningCount += len(warns)
		if len(errs) > 0 {
			errorCount++
		}

		switch {
		case validateJSONFlag:
			jsonOut = append(jsonOut, validateResult{File: file, OK: len(errs) == 0, Errors: errs, Warnings: warns})
		case len(errs) > 0:
			fmt.Fprintf(cmd.OutOrStderr(), "FAIL  %s\n", file)
			for _, e := range errs {
				fmt.Fprintf(cmd.OutOrStderr(), "      %s\n", e)
			}
			for _, w := range warns {
				fmt.Fprintf(cmd.OutOrStderr(), "      warning: %s\n", w)
			}
		default:
			fmt.Fprintf(cmd.OutOrStdout(), "OK    %s\n", file)
			for _, w := range warns {
				fmt.Fprintf(cmd.OutOrStdout(), "      warning: %s\n", w)
			}
		}
	}

	if validateJSONFlag {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		_ = enc.Encode(jsonOut)
	} else {
		summary := fmt.Sprintf("\n%d file(s) checked, %d error(s)", len(files), errorCount)
		if warningCount > 0 {
			summary += fmt.Sprintf(", %d warning(s)", warningCount)
		}
		fmt.Fprintln(cmd.OutOrStdout(), summary)
	}

	if errorCount > 0 {
		return newExitError(ExitParseError, fmt.Sprintf("%d file(s) failed validation", errorCount))
	}
	return nil
}

// validateFile returns the errors and the warnings found in one file. Each is
// a printable message (with file:line:col where known). Parse errors
// (including the "unclosed >>> block" and unknown-operator errors) are
// propagated, never treated as OK.
func validateFile(file string, strict bool) (errs, warns []string) {
	parsed, err := parser.ParseFileQuiet(file)
	if err != nil {
		return []string{err.Error()}, nil
	}
	if vErr := validateParsedFile(file, parsed); vErr != nil {
		return []string{vErr.Error()}, nil
	}

	var defined func(string) bool
	if strict {
		defined = definedElsewhere(file)
	}
	lint := parser.Lint(parsed, defined)

	for _, w := range parsed.Warnings {
		warns = append(warns, w.String())
	}
	for _, w := range lint.Errors {
		errs = append(errs, w.String())
	}
	for _, w := range lint.Warnings {
		warns = append(warns, w.String())
	}
	return errs, warns
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

// definedElsewhere returns a predicate for variable names that something
// outside the file provides: any environment of the nearest hitspec.yaml, a
// .env file next to the file or in the working directory, or the process
// environment. It cannot know which environment a later "run --env" selects,
// so it accepts a name defined in any of them.
func definedElsewhere(file string) func(string) bool {
	names := map[string]bool{}

	dir, err := filepath.Abs(filepath.Dir(file))
	if err != nil {
		dir = filepath.Dir(file)
	}
	for d := dir; ; d = filepath.Dir(d) {
		if path := config.FindConfigPath(d); path != "" {
			if cfg, cerr := config.LoadConfig(path); cerr == nil && cfg != nil {
				for _, vars := range cfg.Environments {
					for k := range vars {
						names[k] = true
					}
				}
			}
			break
		}
		if filepath.Dir(d) == d {
			break
		}
	}

	cwd, _ := os.Getwd()
	for _, d := range []string{dir, cwd} {
		if d != "" {
			readDotEnvNames(filepath.Join(d, ".env"), names)
		}
	}

	return func(name string) bool {
		if names[name] {
			return true
		}
		_, ok := os.LookupEnv(name)
		return ok
	}
}

// readDotEnvNames adds the KEY of every KEY=value line of a .env file.
func readDotEnvNames(path string, into map[string]bool) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, _, ok := strings.Cut(line, "="); ok {
			into[strings.TrimSpace(k)] = true
		}
	}
}
