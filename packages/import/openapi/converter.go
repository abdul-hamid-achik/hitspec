// Package openapi provides functionality to convert OpenAPI/Swagger specifications to hitspec format.
package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Converter converts OpenAPI specs to hitspec format
type Converter struct {
	baseURL       string
	includeTags   []string
	excludeTags   []string
	includeOnly   []string // specific operation IDs
	generateTests bool
}

// Option is a functional option for Converter
type Option func(*Converter)

// WithBaseURL sets a custom base URL, overriding the one from spec
func WithBaseURL(url string) Option {
	return func(c *Converter) {
		c.baseURL = url
	}
}

// WithTags filters operations by tags
func WithTags(tags []string) Option {
	return func(c *Converter) {
		c.includeTags = tags
	}
}

// WithExcludeTags excludes operations with these tags
func WithExcludeTags(tags []string) Option {
	return func(c *Converter) {
		c.excludeTags = tags
	}
}

// WithOperations filters to specific operation IDs
func WithOperations(ops []string) Option {
	return func(c *Converter) {
		c.includeOnly = ops
	}
}

// WithTests generates test assertions from response schemas
func WithTests(generate bool) Option {
	return func(c *Converter) {
		c.generateTests = generate
	}
}

// NewConverter creates a new OpenAPI converter
func NewConverter(opts ...Option) *Converter {
	c := &Converter{
		generateTests: true,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ConvertFile converts an OpenAPI file to hitspec format
func (c *Converter) ConvertFile(path string) (string, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	var doc *openapi3.T
	var err error

	// Check if it's a URL or file path
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		doc, err = loader.LoadFromURI(&url.URL{Scheme: "https", Host: strings.TrimPrefix(strings.TrimPrefix(path, "https://"), "http://"), Path: ""})
		if err != nil {
			// Try loading from URL directly
			doc, err = c.loadFromURL(path)
		}
	} else {
		doc, err = loader.LoadFromFile(path)
	}

	if err != nil {
		return "", fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	return c.Convert(doc)
}

// loadFromURL loads an OpenAPI spec from a URL
func (c *Converter) loadFromURL(urlStr string) (*openapi3.T, error) {
	resp, err := http.Get(urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch spec: %s", resp.Status)
	}

	data, err := readAll(resp.Body)
	if err != nil {
		return nil, err
	}

	loader := openapi3.NewLoader()
	return loader.LoadFromData(data)
}

func readAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var result []byte
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
	}
	return result, nil
}

// Convert converts an OpenAPI document to hitspec format
func (c *Converter) Convert(doc *openapi3.T) (string, error) {
	if err := doc.Validate(context.Background()); err != nil {
		// Log warning but continue - some specs have minor validation issues
		fmt.Fprintf(os.Stderr, "warning: OpenAPI spec validation: %v\n", err)
	}

	var sb strings.Builder

	// Write header comment
	sb.WriteString("# Generated from OpenAPI spec")
	if doc.Info != nil && doc.Info.Title != "" {
		sb.WriteString(": ")
		sb.WriteString(doc.Info.Title)
	}
	sb.WriteString("\n")
	if doc.Info != nil && doc.Info.Version != "" {
		sb.WriteString("# Version: ")
		sb.WriteString(doc.Info.Version)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Write base URL variable
	baseURL := c.baseURL
	if baseURL == "" {
		baseURL = c.getBaseURL(doc)
	}
	if baseURL != "" {
		sb.WriteString("@baseUrl = ")
		sb.WriteString(baseURL)
		sb.WriteString("\n\n")
	}

	// Get sorted paths for consistent output
	paths := make([]string, 0, len(doc.Paths.Map()))
	for path := range doc.Paths.Map() {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	// Convert each path/operation
	for _, path := range paths {
		pathItem := doc.Paths.Map()[path]
		if pathItem == nil {
			continue
		}

		operations := []struct {
			method string
			op     *openapi3.Operation
		}{
			{"GET", pathItem.Get},
			{"POST", pathItem.Post},
			{"PUT", pathItem.Put},
			{"PATCH", pathItem.Patch},
			{"DELETE", pathItem.Delete},
			{"HEAD", pathItem.Head},
			{"OPTIONS", pathItem.Options},
		}

		for _, op := range operations {
			if op.op == nil {
				continue
			}

			if !c.shouldInclude(op.op) {
				continue
			}

			request := c.convertOperation(path, op.method, op.op, pathItem.Parameters)
			sb.WriteString(request)
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

func (c *Converter) getBaseURL(doc *openapi3.T) string {
	if len(doc.Servers) > 0 && doc.Servers[0].URL != "" {
		return doc.Servers[0].URL
	}
	return "http://localhost:3000"
}

func (c *Converter) shouldInclude(op *openapi3.Operation) bool {
	// Check operation ID filter
	if len(c.includeOnly) > 0 {
		found := false
		for _, id := range c.includeOnly {
			if op.OperationID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check tag filters
	if len(c.includeTags) > 0 {
		found := false
		for _, tag := range op.Tags {
			for _, includeTag := range c.includeTags {
				if tag == includeTag {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}

	// Check exclude tags
	if len(c.excludeTags) > 0 {
		for _, tag := range op.Tags {
			for _, excludeTag := range c.excludeTags {
				if tag == excludeTag {
					return false
				}
			}
		}
	}

	return true
}

func (c *Converter) convertOperation(path, method string, op *openapi3.Operation, pathParams openapi3.Parameters) string {
	var sb strings.Builder

	// Request separator with name
	name := op.OperationID
	if name == "" {
		name = strings.ToLower(method) + strings.ReplaceAll(toTitle(path), "/", "")
	}
	sb.WriteString("### ")
	if op.Summary != "" {
		sb.WriteString(op.Summary)
	} else {
		sb.WriteString(name)
	}
	sb.WriteString("\n")

	// Annotations
	sb.WriteString("# @name ")
	sb.WriteString(sanitizeName(name))
	sb.WriteString("\n")

	if len(op.Tags) > 0 {
		sb.WriteString("# @tags ")
		sb.WriteString(strings.Join(op.Tags, ","))
		sb.WriteString("\n")
	}

	if op.Description != "" {
		sb.WriteString("# @description ")
		// Single line description
		desc := strings.ReplaceAll(op.Description, "\n", " ")
		if len(desc) > 100 {
			desc = desc[:100] + "..."
		}
		sb.WriteString(desc)
		sb.WriteString("\n")
	}

	// Method and URL
	sb.WriteString(method)
	sb.WriteString(" {{baseUrl}}")

	// Convert path parameters
	convertedPath := c.convertPathParams(path, op.Parameters, pathParams)
	sb.WriteString(convertedPath)
	sb.WriteString("\n")

	// Query parameters
	allParams := append(pathParams, op.Parameters...)
	for _, paramRef := range allParams {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		param := paramRef.Value
		if param.In == "query" {
			sb.WriteString("  ? ")
			sb.WriteString(param.Name)
			sb.WriteString(" = ")
			sb.WriteString(c.getParamExample(param))
			sb.WriteString("\n")
		}
	}

	// Headers
	for _, paramRef := range allParams {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		param := paramRef.Value
		if param.In == "header" {
			sb.WriteString(param.Name)
			sb.WriteString(": ")
			sb.WriteString(c.getParamExample(param))
			sb.WriteString("\n")
		}
	}

	// Content-Type header for body
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		for contentType := range op.RequestBody.Value.Content {
			if strings.Contains(contentType, "json") {
				sb.WriteString("Content-Type: application/json\n")
				break
			} else if strings.Contains(contentType, "form") {
				sb.WriteString("Content-Type: application/x-www-form-urlencoded\n")
				break
			}
		}
	}

	// Request body
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		body := c.generateRequestBody(op.RequestBody.Value)
		if body != "" {
			sb.WriteString("\n")
			sb.WriteString(body)
			sb.WriteString("\n")
		}
	}

	// Assertions
	if c.generateTests {
		assertions := c.generateAssertions(op)
		if assertions != "" {
			sb.WriteString("\n")
			sb.WriteString(assertions)
		}
	}

	return sb.String()
}

func (c *Converter) convertPathParams(path string, opParams, pathParams openapi3.Parameters) string {
	result := path

	allParams := append(pathParams, opParams...)
	for _, paramRef := range allParams {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		param := paramRef.Value
		if param.In == "path" {
			// Replace {param} with {{param}}
			oldPattern := "{" + param.Name + "}"
			newPattern := "{{" + param.Name + "}}"
			result = strings.ReplaceAll(result, oldPattern, newPattern)
		}
	}

	return result
}

func (c *Converter) getParamExample(param *openapi3.Parameter) string {
	// Try to get example
	if param.Example != nil {
		return fmt.Sprintf("%v", param.Example)
	}

	// Try schema example
	if param.Schema != nil && param.Schema.Value != nil {
		schema := param.Schema.Value
		if schema.Example != nil {
			return fmt.Sprintf("%v", schema.Example)
		}

		// Generate based on type
		switch schema.Type.Slice()[0] {
		case "integer":
			return "1"
		case "number":
			return "1.0"
		case "boolean":
			return "true"
		case "string":
			if schema.Format == "date" {
				return "2024-01-01"
			} else if schema.Format == "date-time" {
				return "2024-01-01T00:00:00Z"
			} else if schema.Format == "email" {
				return "user@example.com"
			} else if schema.Format == "uuid" {
				return "{{$uuid()}}"
			}
			return "example"
		}
	}

	return "{{" + param.Name + "}}"
}

func (c *Converter) generateRequestBody(reqBody *openapi3.RequestBody) string {
	// Prefer JSON
	for contentType, mediaType := range reqBody.Content {
		if strings.Contains(contentType, "json") && mediaType.Schema != nil {
			return c.generateJSONFromSchema(mediaType.Schema.Value, 0)
		}
	}

	// Try form data
	for contentType, mediaType := range reqBody.Content {
		if strings.Contains(contentType, "form") && mediaType.Schema != nil {
			return c.generateFormFromSchema(mediaType.Schema.Value)
		}
	}

	return ""
}

func (c *Converter) generateJSONFromSchema(schema *openapi3.Schema, depth int) string {
	if schema == nil || depth > 5 {
		return "{}"
	}

	if len(schema.Type.Slice()) == 0 {
		return "{}"
	}

	switch schema.Type.Slice()[0] {
	case "object":
		var sb strings.Builder
		sb.WriteString("{\n")

		props := make([]string, 0, len(schema.Properties))
		for name := range schema.Properties {
			props = append(props, name)
		}
		sort.Strings(props)

		for i, name := range props {
			propSchema := schema.Properties[name]
			indent := strings.Repeat("  ", depth+1)
			sb.WriteString(indent)
			sb.WriteString("\"")
			sb.WriteString(name)
			sb.WriteString("\": ")

			if propSchema != nil && propSchema.Value != nil {
				sb.WriteString(c.generateJSONValue(propSchema.Value, depth+1))
			} else {
				sb.WriteString("null")
			}

			if i < len(props)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}

		sb.WriteString(strings.Repeat("  ", depth))
		sb.WriteString("}")
		return sb.String()

	case "array":
		if schema.Items != nil && schema.Items.Value != nil {
			item := c.generateJSONValue(schema.Items.Value, depth+1)
			return "[" + item + "]"
		}
		return "[]"

	default:
		return c.generateJSONValue(schema, depth)
	}
}

func (c *Converter) generateJSONValue(schema *openapi3.Schema, depth int) string {
	if schema == nil {
		return "null"
	}

	// Use example if available
	if schema.Example != nil {
		data, err := json.Marshal(schema.Example)
		if err == nil {
			return string(data)
		}
	}

	if len(schema.Type.Slice()) == 0 {
		return "null"
	}

	switch schema.Type.Slice()[0] {
	case "string":
		if schema.Format == "date" {
			return "\"2024-01-01\""
		} else if schema.Format == "date-time" {
			return "\"2024-01-01T00:00:00Z\""
		} else if schema.Format == "email" {
			return "\"user@example.com\""
		} else if schema.Format == "uuid" {
			return "\"{{$uuid()}}\""
		}
		if len(schema.Enum) > 0 {
			return fmt.Sprintf("\"%v\"", schema.Enum[0])
		}
		return "\"example\""
	case "integer":
		if schema.Min != nil {
			return fmt.Sprintf("%.0f", *schema.Min)
		}
		return "1"
	case "number":
		if schema.Min != nil {
			return fmt.Sprintf("%v", *schema.Min)
		}
		return "1.0"
	case "boolean":
		return "true"
	case "array":
		if schema.Items != nil && schema.Items.Value != nil {
			item := c.generateJSONValue(schema.Items.Value, depth+1)
			return "[" + item + "]"
		}
		return "[]"
	case "object":
		return c.generateJSONFromSchema(schema, depth)
	default:
		return "null"
	}
}

func (c *Converter) generateFormFromSchema(schema *openapi3.Schema) string {
	if schema == nil || len(schema.Properties) == 0 {
		return ""
	}

	var parts []string
	for name, propSchema := range schema.Properties {
		value := "example"
		if propSchema != nil && propSchema.Value != nil {
			if propSchema.Value.Example != nil {
				value = fmt.Sprintf("%v", propSchema.Value.Example)
			}
		}
		parts = append(parts, name+"="+value)
	}
	sort.Strings(parts)
	return strings.Join(parts, "&")
}

func (c *Converter) generateAssertions(op *openapi3.Operation) string {
	if op.Responses == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(">>>\n")

	// Find success response (2xx)
	var successCode string
	var successResp *openapi3.Response

	for code, respRef := range op.Responses.Map() {
		if strings.HasPrefix(code, "2") && respRef != nil && respRef.Value != nil {
			successCode = code
			successResp = respRef.Value
			break
		}
	}

	if successCode != "" {
		sb.WriteString("expect status == ")
		sb.WriteString(successCode)
		sb.WriteString("\n")

		// Add content-type assertion if JSON
		if successResp != nil {
			for contentType := range successResp.Content {
				if strings.Contains(contentType, "json") {
					sb.WriteString("expect header Content-Type contains application/json\n")
					break
				}
			}
		}
	} else {
		// Default to 200
		sb.WriteString("expect status == 200\n")
	}

	sb.WriteString("<<<\n")
	return sb.String()
}

func sanitizeName(name string) string {
	// Remove special characters and convert to camelCase
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, name)

	// Remove consecutive underscores
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	result = strings.Trim(result, "_")

	return result
}

// toTitle converts a string to title case (simple implementation)
func toTitle(s string) string {
	var result strings.Builder
	capitalizeNext := true
	for _, r := range s {
		if r == '/' || r == '-' || r == '_' || r == ' ' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext && r >= 'a' && r <= 'z' {
			result.WriteRune(r - 32) // Convert to uppercase
			capitalizeNext = false
		} else {
			result.WriteRune(r)
			capitalizeNext = false
		}
	}
	return result.String()
}

// ConvertToFile converts and writes to a file
func (c *Converter) ConvertToFile(specPath, outputPath string) error {
	content, err := c.ConvertFile(specPath)
	if err != nil {
		return err
	}

	// Create directory if needed
	if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	return os.WriteFile(outputPath, []byte(content), 0644)
}
