package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/curl"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/insomnia"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/openapi"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/postman"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/result"
	"github.com/spf13/cobra"
)

var (
	importOutputFlag  string
	importBaseURLFlag string
	importTagsFlag    string
	importNoTestsFlag bool
)

var importCmd = &cobra.Command{
	Use:   "import <format> <source>",
	Short: "Import API definitions into hitspec format",
	Long: `Convert API definitions from other formats into .http files.

Supported formats:
  openapi   OpenAPI 3.0/3.1 specification (YAML or JSON, local file or URL)
  postman   Postman Collection v2.1
  curl      curl commands (inline or from a file with @filename)
  insomnia  Insomnia export (v4 format)

Whatever cannot be carried over is reported on stderr as "warning: ..." and,
where it belongs to one request, left in the generated file as a "# TODO:"
comment. Imports never drop input silently.

-o accepts a file or a directory (a trailing "/" or an existing directory).
For a directory, openapi writes one file per tag and the other formats write
<name>.http.

Examples:
  hitspec import openapi spec.yaml -o tests/api.http
  hitspec import openapi spec.yaml -o tests/
  hitspec import postman collection.json -o tests/
  hitspec import curl "curl https://api.example.com/users"
  hitspec import insomnia export.json -o tests/api.http`,
	// Without a RunE, cobra prints help and exits 0 for an unknown
	// subcommand (e.g. a typo'd format), silently passing in scripts.
	// Reject unknown formats with a non-zero exit instead.
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return fmt.Errorf("unknown import format %q; supported formats: openapi, postman, curl, insomnia", args[0])
	},
}

var importOpenAPICmd = &cobra.Command{
	Use:   "openapi <spec-file-or-url>",
	Short: "Import from OpenAPI/Swagger specification",
	Long: `Import API tests from an OpenAPI 3.0/3.1 specification file or URL.

The command reads the OpenAPI spec and generates hitspec-compatible .http files
with requests and basic assertions for each operation.

Path parameters such as /users/{id} become {{id}} with an @id variable set to the
spec's example (or a type-based default). With -o <dir>/ one file is written
per tag (operations without tags are grouped by their first path segment).

Examples:
  hitspec import openapi spec.yaml
  hitspec import openapi spec.yaml -o tests/api.http
  hitspec import openapi spec.yaml -o tests/
  hitspec import openapi https://api.example.com/openapi.json
  hitspec import openapi spec.yaml --tags users,auth
  hitspec import openapi spec.yaml --base-url http://localhost:3000
  hitspec import openapi spec.yaml --no-tests`,
	Args: cobra.ExactArgs(1),
	RunE: importOpenAPICommand,
}

var importPostmanCmd = &cobra.Command{
	Use:   "postman <collection-file>",
	Short: "Import from Postman collection",
	Long: `Import API tests from a Postman Collection v2.1 file.

The command reads the Postman collection and generates hitspec-compatible .http files
with requests, converting Postman variables and tests to hitspec format.

  - Collection variables become @name = value lines.
  - A Postman environment export becomes an environments entry for hitspec.yaml.
  - Collection, folder and request auth (bearer, basic, API key, digest) becomes @auth.
  - pm.test scripts are translated for the common patterns (status, response
    time, headers, JSON body fields). Anything else is kept as a
    "# TODO: untranslated test: ..." comment instead of an invented assertion.

Examples:
  hitspec import postman collection.json
  hitspec import postman collection.json -o tests/`,
	Args: cobra.ExactArgs(1),
	RunE: importPostmanCommand,
}

var importCurlNoTestsFlag bool
var importInsomniaNoTestsFlag bool

var importCurlCmd = &cobra.Command{
	Use:   "curl <command-or-file>",
	Short: "Import from curl command",
	Long: `Import API tests from a curl command or a file containing curl commands.

The command parses curl flags and converts them to hitspec format.
Use @filename to read commands from a file (one command per line, supports line continuations with \).

Supported curl flags:
  -X, --request       HTTP method
  -H, --header        Request header
  -d, --data          Request body (repeat to join with &; @file becomes < ./file)
  --data-raw          Request body ('@' is literal)
  --data-urlencode    Percent-encoded body part
  --json              JSON body plus Content-Type/Accept: application/json
  -F, --form          Multipart field or file (name=value, name=@file)
  -G, --get           Send the data as a query string
  -I, --head          HEAD request
  -T, --upload-file   PUT a file
  -u, --user          Basic auth (user:password) -> @auth basic
  --digest            Digest auth with -u
  --oauth2-bearer     Bearer token -> @auth bearer
  -m, --max-time      Timeout -> @timeout
  -A, --user-agent    User-Agent header
  -e, --referer       Referer header
  -b, --cookie        Cookie header
  -k, --insecure      Reported as a warning (set validateSSL: false in hitspec.yaml)

Flags that only change how curl prints or transfers (-s, -S, -v, -i, -f,
--compressed, ...) are ignored. Any other flag produces a warning.

Examples:
  hitspec import curl "curl https://api.example.com/users"
  hitspec import curl "curl -X POST -H 'Content-Type: application/json' -d '{\"name\":\"John\"}' https://api.example.com/users"
  hitspec import curl @commands.txt -o tests/api.http
  hitspec import curl @commands.txt --no-tests`,
	Args: cobra.ExactArgs(1),
	RunE: importCurlCommand,
}

var importInsomniaCmd = &cobra.Command{
	Use:   "insomnia <export-file>",
	Short: "Import from Insomnia export",
	Long: `Import API tests from an Insomnia export file.

The command reads an Insomnia export (v4 format) and generates hitspec-compatible .http files
with requests, converting Insomnia variables, bodies (JSON, form, multipart,
GraphQL, file) and authentication to hitspec format.

Environments are imported too. A lone base environment becomes @name = value
lines. When sub environments exist, every variable goes to a hitspec.yaml
environments section instead (base plus one entry per sub environment), written
next to the output file, or printed to stderr when writing to stdout.

Examples:
  hitspec import insomnia export.json
  hitspec import insomnia export.json -o tests/api.http
  hitspec import insomnia export.json --no-tests`,
	Args: cobra.ExactArgs(1),
	RunE: importInsomniaCommand,
}

func init() {
	// OpenAPI flags
	importOpenAPICmd.Flags().StringVarP(&importOutputFlag, "output", "o", "", "Output file, or a directory for one file per tag (default: stdout)")
	importOpenAPICmd.Flags().StringVar(&importBaseURLFlag, "base-url", "", "Override base URL from spec")
	importOpenAPICmd.Flags().StringVar(&importTagsFlag, "tags", "", "Filter operations by tags (comma-separated)")
	importOpenAPICmd.Flags().BoolVar(&importNoTestsFlag, "no-tests", false, "Don't generate test assertions")

	// Postman flags
	importPostmanCmd.Flags().StringVarP(&importOutputFlag, "output", "o", "", "Output file or directory path (default: stdout)")

	// curl flags
	importCurlCmd.Flags().StringVarP(&importOutputFlag, "output", "o", "", "Output file or directory path (default: stdout)")
	importCurlCmd.Flags().BoolVar(&importCurlNoTestsFlag, "no-tests", false, "Don't generate test assertions")

	// Insomnia flags
	importInsomniaCmd.Flags().StringVarP(&importOutputFlag, "output", "o", "", "Output file or directory path (default: stdout)")
	importInsomniaCmd.Flags().BoolVar(&importInsomniaNoTestsFlag, "no-tests", false, "Don't generate test assertions")

	importCmd.AddCommand(importOpenAPICmd)
	importCmd.AddCommand(importPostmanCmd)
	importCmd.AddCommand(importCurlCmd)
	importCmd.AddCommand(importInsomniaCmd)
}

// isDirOutput reports whether -o names a directory: a trailing separator or
// an existing directory.
func isDirOutput(out string) bool {
	if strings.HasSuffix(out, "/") || strings.HasSuffix(out, string(os.PathSeparator)) {
		return true
	}
	info, err := os.Stat(out)
	return err == nil && info.IsDir()
}

// outputStem derives the default .http file name from the import source.
func outputStem(source, fallback string) string {
	base := filepath.Base(source)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if stem == "" || stem == "." || strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return fallback
	}
	return stem
}

// printWarnings writes importer warnings to stderr.
func printWarnings(w io.Writer, warnings []string) {
	for _, msg := range warnings {
		fmt.Fprintf(w, "warning: %s\n", msg)
	}
}

// emitImport writes an importer result to stdout or to the -o target and
// reports its warnings and environments. source and fallback pick the file
// name when -o is a directory.
func emitImport(cmd *cobra.Command, res *result.Result, source, fallback string) error {
	stdout, stderr := cmd.OutOrStdout(), cmd.ErrOrStderr()
	printWarnings(stderr, res.Warnings)

	// An environment-only import (a Postman environment export) has no
	// requests: the hitspec.yaml fragment is the whole result.
	envOnly := strings.TrimSpace(res.Content) == "" && res.HasEnvironments()

	if importOutputFlag == "" {
		fmt.Fprint(stdout, res.Content)
		if res.HasEnvironments() {
			y, err := res.EnvironmentsYAML()
			if err != nil {
				return err
			}
			if envOnly {
				fmt.Fprint(stdout, y)
			} else {
				fmt.Fprintf(stderr, "\n# Environments: add this to hitspec.yaml\n%s", y)
			}
		}
		return nil
	}

	outPath := importOutputFlag
	if isDirOutput(outPath) {
		if err := os.MkdirAll(outPath, 0o755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		outPath = filepath.Join(outPath, outputStem(source, fallback)+".http")
	} else if dir := filepath.Dir(outPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}
	if !envOnly {
		if err := os.WriteFile(outPath, []byte(res.Content), 0o644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(stdout, "Successfully imported to %s\n", outPath)
	}

	if res.HasEnvironments() {
		y, err := res.EnvironmentsYAML()
		if err != nil {
			return err
		}
		// Never overwrite an existing hitspec.yaml: the user may have settings in it.
		cfg := filepath.Join(filepath.Dir(outPath), "hitspec.yaml")
		note := ""
		if _, err := os.Stat(cfg); err == nil {
			cfg = filepath.Join(filepath.Dir(outPath), "hitspec.imported.yaml")
			note = " (hitspec.yaml already exists: merge the environments section into it)"
		}
		if err := os.WriteFile(cfg, []byte(y), 0o644); err != nil {
			return fmt.Errorf("failed to write environments: %w", err)
		}
		fmt.Fprintf(stdout, "Wrote %d environment(s) to %s%s\n", len(res.Environments), cfg, note)
	}
	return nil
}

func importOpenAPICommand(cmd *cobra.Command, args []string) error {
	specPath := args[0]

	// Build converter options
	var opts []openapi.Option

	if importBaseURLFlag != "" {
		opts = append(opts, openapi.WithBaseURL(importBaseURLFlag))
	}

	if importTagsFlag != "" {
		tags := strings.Split(importTagsFlag, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
		opts = append(opts, openapi.WithTags(tags))
	}

	if importNoTestsFlag {
		opts = append(opts, openapi.WithTests(false))
	}

	converter := openapi.NewConverter(opts...)

	// -o <dir>/: one file per tag.
	if importOutputFlag != "" && isDirOutput(importOutputFlag) {
		split, err := converter.ConvertFilesResult(specPath)
		if err != nil {
			return fmt.Errorf("failed to convert OpenAPI spec: %w", err)
		}
		if err := os.MkdirAll(importOutputFlag, 0o755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		printWarnings(cmd.ErrOrStderr(), split.Warnings)
		if len(split.Files) == 0 {
			return fmt.Errorf("no operations to import from %s", specPath)
		}
		for _, f := range split.Files {
			target := filepath.Join(importOutputFlag, f.Name)
			if err := os.WriteFile(target, []byte(f.Content), 0o644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully imported to %s\n", target)
		}
		return nil
	}

	res, err := converter.ConvertFileResult(specPath)
	if err != nil {
		return fmt.Errorf("failed to convert OpenAPI spec: %w", err)
	}
	return emitImport(cmd, res, specPath, "openapi")
}

func importInsomniaCommand(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	// Build converter options
	var opts []insomnia.Option
	if importInsomniaNoTestsFlag {
		opts = append(opts, insomnia.WithAssertions(false))
	}

	res, err := insomnia.NewConverter(opts...).ConvertFileResult(filePath)
	if err != nil {
		return fmt.Errorf("failed to convert Insomnia export: %w", err)
	}
	return emitImport(cmd, res, filePath, "insomnia")
}

func importCurlCommand(cmd *cobra.Command, args []string) error {
	input := args[0]

	// Build converter options
	var opts []curl.Option
	if importCurlNoTestsFlag {
		opts = append(opts, curl.WithAssertions(false))
	}

	converter := curl.NewConverter(opts...)

	var res *result.Result
	var err error

	// Check if input is a file reference (@filename)
	source := "curl"
	if strings.HasPrefix(input, "@") {
		source = strings.TrimPrefix(input, "@")
		res, err = converter.ConvertFileResult(source)
		if err != nil {
			return fmt.Errorf("failed to convert curl file: %w", err)
		}
	} else {
		// Direct curl command
		res, err = converter.ConvertCommandResult(input)
		if err != nil {
			return fmt.Errorf("failed to convert curl command: %w", err)
		}
	}
	return emitImport(cmd, res, source, "curl")
}

func importPostmanCommand(cmd *cobra.Command, args []string) error {
	collectionPath := args[0]

	res, err := postman.NewConverter().ConvertFile(collectionPath)
	if err != nil {
		return fmt.Errorf("failed to convert Postman collection: %w", err)
	}
	return emitImport(cmd, res, collectionPath, "postman")
}
