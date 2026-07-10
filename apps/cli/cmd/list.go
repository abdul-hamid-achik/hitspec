package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/spf13/cobra"
)

var listJSONFlag bool

var listCmd = &cobra.Command{
	Use:   "list <file|directory>",
	Short: "List all tests in hitspec files",
	Long: `List all tests defined in .http or .hitspec files.

Examples:
  hitspec list api.http
  hitspec list ./tests/
  hitspec list ./tests/ --json`,
	Args: cobra.MinimumNArgs(1),
	RunE: listCommand,
}

func init() {
	listCmd.Flags().BoolVar(&listJSONFlag, "json", false, "Output the test list as JSON")
}

// listFileRequest is the JSON shape emitted by `hitspec list --json`.
type listFileRequest struct {
	Name string   `json:"name"`
	URL  string   `json:"url"`
	Tags []string `json:"tags,omitempty"`
}

// listFile is the JSON shape for one file's tests.
type listFile struct {
	File     string            `json:"file"`
	Requests []listFileRequest `json:"requests"`
}

func listCommand(cmd *cobra.Command, args []string) error {
	files, err := collectFiles(args)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no .http or .hitspec files found in %v", args)
	}

	var jsonOut []listFile
	totalRequests := 0
	for _, file := range files {
		f, err := parser.ParseFile(file)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Error parsing %s: %v\n", file, err)
			continue
		}

		if listJSONFlag {
			reqs := make([]listFileRequest, 0, len(f.Requests))
			for _, req := range f.Requests {
				reqs = append(reqs, listFileRequest{Name: req.Name, URL: req.URL, Tags: req.Tags})
			}
			jsonOut = append(jsonOut, listFile{File: file, Requests: reqs})
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
		totalRequests += len(f.Requests)
	}

	if listJSONFlag {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(jsonOut)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n%d request(s) in %d file(s)\n", totalRequests, len(files))

	return nil
}
