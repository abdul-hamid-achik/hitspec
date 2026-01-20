package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/packages/import/openapi"
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
	Short: "Import API specs from various formats",
	Long: `Import API specifications from various formats and convert them to hitspec format.

Supported formats:
  openapi - OpenAPI 3.0/3.1 (YAML or JSON)
  postman - Postman Collection v2.1

Examples:
  hitspec import openapi spec.yaml
  hitspec import openapi spec.yaml -o tests/api.http
  hitspec import openapi https://petstore.swagger.io/v2/swagger.json
  hitspec import openapi spec.yaml --tags users,auth --base-url http://localhost:3000

  hitspec import postman collection.json
  hitspec import postman collection.json -o tests/`,
}

var importOpenAPICmd = &cobra.Command{
	Use:   "openapi <spec-file-or-url>",
	Short: "Import from OpenAPI/Swagger specification",
	Long: `Import API tests from an OpenAPI 3.0/3.1 specification file or URL.

The command reads the OpenAPI spec and generates hitspec-compatible .http files
with requests and basic assertions for each operation.

Examples:
  hitspec import openapi spec.yaml
  hitspec import openapi spec.yaml -o tests/api.http
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

Examples:
  hitspec import postman collection.json
  hitspec import postman collection.json -o tests/`,
	Args: cobra.ExactArgs(1),
	RunE: importPostmanCommand,
}

func init() {
	// OpenAPI flags
	importOpenAPICmd.Flags().StringVarP(&importOutputFlag, "output", "o", "", "Output file path (default: stdout)")
	importOpenAPICmd.Flags().StringVar(&importBaseURLFlag, "base-url", "", "Override base URL from spec")
	importOpenAPICmd.Flags().StringVar(&importTagsFlag, "tags", "", "Filter operations by tags (comma-separated)")
	importOpenAPICmd.Flags().BoolVar(&importNoTestsFlag, "no-tests", false, "Don't generate test assertions")

	// Postman flags
	importPostmanCmd.Flags().StringVarP(&importOutputFlag, "output", "o", "", "Output file or directory path (default: stdout)")

	importCmd.AddCommand(importOpenAPICmd)
	importCmd.AddCommand(importPostmanCmd)
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

	// Convert
	content, err := converter.ConvertFile(specPath)
	if err != nil {
		return fmt.Errorf("failed to convert OpenAPI spec: %w", err)
	}

	// Output
	if importOutputFlag != "" {
		// Create directory if needed
		if dir := filepath.Dir(importOutputFlag); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		if err := os.WriteFile(importOutputFlag, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		fmt.Printf("Successfully imported to %s\n", importOutputFlag)
	} else {
		fmt.Print(content)
	}

	return nil
}

func importPostmanCommand(cmd *cobra.Command, args []string) error {
	collectionPath := args[0]

	// Convert Postman collection
	content, err := convertPostmanCollection(collectionPath)
	if err != nil {
		return fmt.Errorf("failed to convert Postman collection: %w", err)
	}

	// Output
	if importOutputFlag != "" {
		// Create directory if needed
		if dir := filepath.Dir(importOutputFlag); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		if err := os.WriteFile(importOutputFlag, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		fmt.Printf("Successfully imported to %s\n", importOutputFlag)
	} else {
		fmt.Print(content)
	}

	return nil
}

// PostmanCollection represents a Postman Collection v2.1 structure
type PostmanCollection struct {
	Info PostmanInfo   `json:"info"`
	Item []PostmanItem `json:"item"`
}

type PostmanInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Schema      string `json:"schema"`
}

type PostmanItem struct {
	Name        string            `json:"name"`
	Request     *PostmanRequest   `json:"request,omitempty"`
	Response    []PostmanResponse `json:"response,omitempty"`
	Item        []PostmanItem     `json:"item,omitempty"` // For folders
	Description string            `json:"description,omitempty"`
}

type PostmanRequest struct {
	Method string            `json:"method"`
	Header []PostmanHeader   `json:"header,omitempty"`
	Body   *PostmanBody      `json:"body,omitempty"`
	URL    PostmanURL        `json:"url"`
	Auth   *PostmanAuth      `json:"auth,omitempty"`
}

type PostmanHeader struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled,omitempty"`
}

type PostmanBody struct {
	Mode       string            `json:"mode"`
	Raw        string            `json:"raw,omitempty"`
	URLEncoded []PostmanKV       `json:"urlencoded,omitempty"`
	FormData   []PostmanFormData `json:"formdata,omitempty"`
}

type PostmanKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type PostmanFormData struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"` // "text" or "file"
	Src   string `json:"src,omitempty"`
}

type PostmanURL struct {
	Raw   string   `json:"raw"`
	Host  []string `json:"host,omitempty"`
	Path  []string `json:"path,omitempty"`
	Query []PostmanKV `json:"query,omitempty"`
}

type PostmanAuth struct {
	Type   string        `json:"type"`
	Bearer []PostmanKV   `json:"bearer,omitempty"`
	Basic  []PostmanKV   `json:"basic,omitempty"`
	APIKey []PostmanKV   `json:"apikey,omitempty"`
}

type PostmanResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Code   int    `json:"code"`
	Body   string `json:"body,omitempty"`
}

func convertPostmanCollection(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var collection PostmanCollection
	if err := json.Unmarshal(data, &collection); err != nil {
		return "", fmt.Errorf("failed to parse Postman collection: %w", err)
	}

	var sb strings.Builder

	// Header
	sb.WriteString("# Generated from Postman collection")
	if collection.Info.Name != "" {
		sb.WriteString(": ")
		sb.WriteString(collection.Info.Name)
	}
	sb.WriteString("\n\n")

	// Convert items recursively
	convertPostmanItems(&sb, collection.Item, "")

	return sb.String(), nil
}

func convertPostmanItems(sb *strings.Builder, items []PostmanItem, prefix string) {
	for _, item := range items {
		// If it's a folder (has nested items), recurse
		if len(item.Item) > 0 {
			if prefix != "" {
				convertPostmanItems(sb, item.Item, prefix+"/"+item.Name)
			} else {
				convertPostmanItems(sb, item.Item, item.Name)
			}
			continue
		}

		// Skip if no request
		if item.Request == nil {
			continue
		}

		// Request separator
		sb.WriteString("### ")
		if prefix != "" {
			sb.WriteString(prefix)
			sb.WriteString(" - ")
		}
		sb.WriteString(item.Name)
		sb.WriteString("\n")

		// Name annotation
		sb.WriteString("# @name ")
		sb.WriteString(sanitizePostmanName(item.Name))
		sb.WriteString("\n")

		// Method and URL
		req := item.Request
		sb.WriteString(req.Method)
		sb.WriteString(" ")
		sb.WriteString(convertPostmanURL(req.URL.Raw))
		sb.WriteString("\n")

		// Headers
		for _, h := range req.Header {
			if h.Disabled {
				continue
			}
			sb.WriteString(h.Key)
			sb.WriteString(": ")
			sb.WriteString(convertPostmanVariable(h.Value))
			sb.WriteString("\n")
		}

		// Body
		if req.Body != nil {
			switch req.Body.Mode {
			case "raw":
				if req.Body.Raw != "" {
					sb.WriteString("\n")
					sb.WriteString(convertPostmanVariable(req.Body.Raw))
					sb.WriteString("\n")
				}
			case "urlencoded":
				if len(req.Body.URLEncoded) > 0 {
					sb.WriteString("\n")
					var parts []string
					for _, kv := range req.Body.URLEncoded {
						parts = append(parts, kv.Key+"="+convertPostmanVariable(kv.Value))
					}
					sb.WriteString(strings.Join(parts, "&"))
					sb.WriteString("\n")
				}
			}
		}

		// Basic assertion
		sb.WriteString("\n>>>\n")
		sb.WriteString("expect status == 200\n")
		sb.WriteString("<<<\n\n")
	}
}

func convertPostmanURL(url string) string {
	// Convert Postman variable syntax {{var}} to hitspec syntax (same)
	return convertPostmanVariable(url)
}

func convertPostmanVariable(s string) string {
	// Postman uses {{variable}} which is the same as hitspec
	// But some dynamic variables need conversion
	s = strings.ReplaceAll(s, "{{$guid}}", "{{$uuid()}}")
	s = strings.ReplaceAll(s, "{{$timestamp}}", "{{$timestamp()}}")
	s = strings.ReplaceAll(s, "{{$randomInt}}", "{{$random(0, 1000)}}")
	s = strings.ReplaceAll(s, "{{$randomEmail}}", "{{$randomEmail()}}")
	s = strings.ReplaceAll(s, "{{$randomUUID}}", "{{$uuid()}}")
	return s
}

// sanitizePostmanName sanitizes a name for use as an identifier
func sanitizePostmanName(name string) string {
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, name)

	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	result = strings.Trim(result, "_")

	return result
}
