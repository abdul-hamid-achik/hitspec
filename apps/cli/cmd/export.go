package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/export/snippet"
	"github.com/spf13/cobra"
)

var (
	exportNameFlag    string
	exportTagsFlag    string
	exportOutputFlag  string
	exportEnvFlag     string
	exportFormatFlag  string
	exportExecFlag    bool
	exportVerboseFlag bool
)

var exportCmd = &cobra.Command{
	Use:   "export [format] <file>",
	Short: "Export hitspec files to other formats",
	Long: `Export hitspec requests as runnable commands or client code.

Supported formats (the same names Studio's export action uses):
  curl     curl commands (can pipe to shell or save to .sh file)
  fetch    JavaScript fetch (alias: javascript)
  wget     wget commands
  python   Python requests
  httpie   HTTPie commands
  go       Go net/http
  ruby     Ruby Net::HTTP

Pick the format with --format or as the first argument:

  hitspec export --format python tests/api.http
  hitspec export python tests/api.http

Examples:
  hitspec export curl tests/api.http
  hitspec export curl tests/api.http --name "Login*"
  hitspec export --format fetch tests/api.http -o snippets.js
  hitspec export curl tests/api.http --exec`,
	Args: cobra.ArbitraryArgs,
	// Without a RunE, cobra prints help and exits 0 for an unknown
	// subcommand (e.g. a typo'd format), silently passing in scripts.
	// Reject unknown formats with a non-zero exit instead.
	RunE: func(cmd *cobra.Command, args []string) error {
		switch len(args) {
		case 0:
			return cmd.Help()
		case 1:
			// export <file> with --format
			return runExport(cmd, exportFormatFlag, args[0])
		}
		// export <format> <file> for aliases that have no subcommand (js, py).
		if f, ok := snippet.Normalize(args[0]); ok && len(args) == 2 {
			return runExport(cmd, f, args[1])
		}
		return fmt.Errorf("unknown export format %q; supported formats: %s", args[0], strings.Join(snippet.Formats, ", "))
	},
}

func init() {
	// Shared by `export <file>` and every `export <format> <file>` subcommand.
	pf := exportCmd.PersistentFlags()
	pf.StringVarP(&exportNameFlag, "name", "n", "", "Filter by request name (glob pattern)")
	pf.StringVarP(&exportTagsFlag, "tags", "t", "", "Filter by tags (comma-separated)")
	pf.StringVarP(&exportOutputFlag, "output", "o", "", "Output file path (default: stdout)")
	pf.StringVarP(&exportEnvFlag, "env", "e", "", "Environment for variable resolution")
	pf.BoolVar(&exportExecFlag, "exec", false, "Execute the curl command directly (curl only, single request)")
	pf.BoolVarP(&exportVerboseFlag, "verbose", "v", false, "Include -v in curl output (curl only)")
	exportCmd.Flags().StringVarP(&exportFormatFlag, "format", "f", "curl", "Export format: "+strings.Join(snippet.Formats, ", "))

	for _, format := range snippet.Formats {
		format := format
		exportCmd.AddCommand(&cobra.Command{
			Use:   format + " <file>",
			Short: "Export requests as " + format,
			Long: fmt.Sprintf(`Export hitspec requests as %s.

Examples:
  hitspec export %s tests/api.http
  hitspec export %s tests/api.http --name "Login*"
  hitspec export %s tests/api.http -o out.txt
  hitspec export %s tests/api.http --env staging`, format, format, format, format, format),
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return runExport(cmd, format, args[0])
			},
		})
	}
}

// exportResolver builds the {{variable}} resolver for an export: file
// variables, plus .env files when --env is set. It returns nil when there is
// nothing to resolve.
func exportResolver(cmd *cobra.Command, f *parser.File) func(string) string {
	if exportEnvFlag == "" && len(f.Variables) == 0 {
		return nil
	}
	resolver := env.NewResolver()

	// Load file variables
	for _, v := range f.Variables {
		resolver.SetVariable(v.Name, v.Value)
	}

	if exportEnvFlag != "" {
		// Try to load environment-specific .env file
		envFile := fmt.Sprintf(".env.%s", exportEnvFlag)
		if _, err := os.Stat(envFile); err == nil {
			if err := resolver.LoadDotEnv(envFile); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to load %s: %v\n", envFile, err)
			}
		}

		// Try to load .env file
		if _, err := os.Stat(".env"); err == nil {
			if err := resolver.LoadDotEnv(".env"); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to load .env: %v\n", err)
			}
		}
	}
	return resolver.Resolve
}

func runExport(cmd *cobra.Command, format, filePath string) error {
	format, ok := snippet.Normalize(format)
	if !ok {
		return fmt.Errorf("unknown export format %q; supported formats: %s", exportFormatFlag, strings.Join(snippet.Formats, ", "))
	}
	if exportExecFlag && format != "curl" {
		return fmt.Errorf("--exec only works with the curl format")
	}

	// Validate file exists and is a hitspec file
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", filePath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, please specify a file", filePath)
	}
	if !isHitspecFile(filePath) {
		return fmt.Errorf("%s is not a .http or .hitspec file", filePath)
	}

	// Parse the file
	f, err := parser.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", filePath, err)
	}

	// Filter requests
	requests := filterRequests(f.Requests, exportNameFlag, exportTagsFlag)

	if len(requests) == 0 {
		return fmt.Errorf("no requests found matching the filter criteria")
	}

	// Check exec mode constraints
	if exportExecFlag && len(requests) > 1 {
		return fmt.Errorf("--exec requires exactly one request, but %d matched. Use --name to filter to a single request", len(requests))
	}

	resolve := exportResolver(cmd, f)

	var opts []snippet.Option
	if resolve != nil {
		opts = append(opts, snippet.WithResolver(resolve))
	}
	if exportVerboseFlag {
		opts = append(opts, snippet.WithVerbose(true))
	}

	// Execute mode
	if exportExecFlag {
		curlCmd, err := snippet.Render("curl", requests[0], opts...)
		if err != nil {
			return err
		}

		// Parse the curl command and execute it
		// We'll use bash -c to handle the command properly
		execCmd := exec.Command("bash", "-c", curlCmd)
		execCmd.Stdout = cmd.OutOrStdout()
		execCmd.Stderr = cmd.ErrOrStderr()
		execCmd.Stdin = os.Stdin

		return execCmd.Run()
	}

	// Generate output
	comment := snippet.Comment(format)
	var sb strings.Builder
	rendered := 0
	var lastErr error
	for i, req := range requests {
		if i > 0 {
			sb.WriteString("\n")
		}
		name := req.Name
		if name == "" {
			name = fmt.Sprintf("%s %s", req.Method, req.URL)
		}
		sb.WriteString(comment + " Request: " + name + "\n")
		out, err := snippet.Render(format, req, opts...)
		if err != nil {
			// Say so in the output and on stderr rather than emit a wrong snippet.
			lastErr = err
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipped request %q: %v\n", name, err)
			sb.WriteString(comment + " SKIPPED: " + err.Error() + "\n")
			continue
		}
		rendered++
		sb.WriteString(out)
		sb.WriteString("\n")
	}
	if rendered == 0 {
		return lastErr
	}
	output := sb.String()

	// Write output
	if exportOutputFlag != "" {
		// Create directory if needed
		if dir := filepath.Dir(exportOutputFlag); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		if err := os.WriteFile(exportOutputFlag, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Exported %d %s snippet(s) to %s\n", rendered, format, exportOutputFlag)
	} else {
		fmt.Fprint(cmd.OutOrStdout(), output)
	}

	return nil
}

// filterRequests filters requests by name pattern and/or tags.
func filterRequests(requests []*parser.Request, namePattern, tagsStr string) []*parser.Request {
	if namePattern == "" && tagsStr == "" {
		return requests
	}

	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			tags = append(tags, strings.TrimSpace(t))
		}
	}

	var filtered []*parser.Request
	for _, req := range requests {
		// Check name pattern
		if namePattern != "" {
			if !matchGlob(req.Name, namePattern) && !matchGlob(req.Method+" "+req.URL, namePattern) {
				continue
			}
		}

		// Check tags
		if len(tags) > 0 {
			if !parser.HasAnyTag(req.Tags, tags) {
				continue
			}
		}

		filtered = append(filtered, req)
	}

	return filtered
}

// matchGlob performs simple glob matching supporting * and ? wildcards.
func matchGlob(s, pattern string) bool {
	// Empty pattern matches nothing (unless string is also empty)
	if pattern == "" {
		return s == ""
	}

	// Simple implementation: convert glob to regex-like matching
	i, j := 0, 0
	starIdx, matchIdx := -1, 0

	for i < len(s) {
		if j < len(pattern) && (pattern[j] == '?' || pattern[j] == s[i]) {
			i++
			j++
		} else if j < len(pattern) && pattern[j] == '*' {
			starIdx = j
			matchIdx = i
			j++
		} else if starIdx != -1 {
			j = starIdx + 1
			matchIdx++
			i = matchIdx
		} else {
			return false
		}
	}

	for j < len(pattern) && pattern[j] == '*' {
		j++
	}

	return j == len(pattern)
}
