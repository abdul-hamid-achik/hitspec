package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/export/curl"
	"github.com/spf13/cobra"
)

var (
	exportNameFlag    string
	exportTagsFlag    string
	exportOutputFlag  string
	exportEnvFlag     string
	exportExecFlag    bool
	exportVerboseFlag bool
)

var exportCmd = &cobra.Command{
	Use:   "export <format> <file>",
	Short: "Export hitspec files to other formats",
	Long: `Export hitspec requests to executable formats like curl.

Supported formats:
  curl    Export as curl commands (can pipe to shell or save to .sh file)

Examples:
  hitspec export curl tests/api.http
  hitspec export curl tests/api.http --name "Login*"
  hitspec export curl tests/api.http --exec`,
	// Without a RunE, cobra prints help and exits 0 for an unknown
	// subcommand (e.g. a typo'd format), silently passing in scripts.
	// Reject unknown formats with a non-zero exit instead.
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return fmt.Errorf("unknown export format %q; supported formats: curl", args[0])
	},
}

var exportCurlCmd = &cobra.Command{
	Use:   "curl <file>",
	Short: "Export requests as curl commands",
	Long: `Export hitspec requests as curl commands that can be run directly in the terminal.

Examples:
  # Export all requests from a file
  hitspec export curl tests/api.http

  # Filter by request name (glob pattern)
  hitspec export curl tests/api.http --name "Login*"
  hitspec export curl tests/api.http -n "createUser"

  # Filter by tags
  hitspec export curl tests/api.http --tags smoke,auth

  # Output to file instead of stdout
  hitspec export curl tests/api.http -o commands.sh

  # Execute the curl command directly (single request only)
  hitspec export curl tests/api.http --name "Login" --exec

  # Include verbose flag in curl output
  hitspec export curl tests/api.http --verbose

  # Resolve variables from environment before export
  hitspec export curl tests/api.http --env staging`,
	Args: cobra.ExactArgs(1),
	RunE: runExportCurl,
}

func init() {
	// Curl export flags
	exportCurlCmd.Flags().StringVarP(&exportNameFlag, "name", "n", "", "Filter by request name (glob pattern)")
	exportCurlCmd.Flags().StringVarP(&exportTagsFlag, "tags", "t", "", "Filter by tags (comma-separated)")
	exportCurlCmd.Flags().StringVarP(&exportOutputFlag, "output", "o", "", "Output file path (default: stdout)")
	exportCurlCmd.Flags().StringVarP(&exportEnvFlag, "env", "e", "", "Environment for variable resolution")
	exportCurlCmd.Flags().BoolVar(&exportExecFlag, "exec", false, "Execute the curl command directly (single request only)")
	exportCurlCmd.Flags().BoolVarP(&exportVerboseFlag, "verbose", "v", false, "Include -v in curl output")

	exportCmd.AddCommand(exportCurlCmd)
}

func runExportCurl(cmd *cobra.Command, args []string) error {
	filePath := args[0]

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

	// Set up resolver if env flag is provided
	var opts []curl.Option
	if exportEnvFlag != "" {
		resolver := env.NewResolver()

		// Load file variables
		for _, v := range f.Variables {
			resolver.SetVariable(v.Name, v.Value)
		}

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

		opts = append(opts, curl.WithResolver(resolver.Resolve))
	} else {
		// Still resolve file-level variables without environment
		if len(f.Variables) > 0 {
			resolver := env.NewResolver()
			for _, v := range f.Variables {
				resolver.SetVariable(v.Name, v.Value)
			}
			opts = append(opts, curl.WithResolver(resolver.Resolve))
		}
	}

	if exportVerboseFlag {
		opts = append(opts, curl.WithVerbose(true))
	}

	exporter := curl.New(opts...)

	// Execute mode
	if exportExecFlag {
		curlCmd := exporter.Export(requests[0])

		// Parse the curl command and execute it
		// We'll use bash -c to handle the command properly
		execCmd := exec.Command("bash", "-c", curlCmd)
		execCmd.Stdout = cmd.OutOrStdout()
		execCmd.Stderr = cmd.ErrOrStderr()
		execCmd.Stdin = os.Stdin

		return execCmd.Run()
	}

	// Generate output
	output := exporter.ExportFormatted(requests)

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

		fmt.Fprintf(cmd.OutOrStdout(), "Exported %d curl command(s) to %s\n", len(requests), exportOutputFlag)
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
