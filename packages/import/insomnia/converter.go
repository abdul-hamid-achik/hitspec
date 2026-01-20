// Package insomnia provides Insomnia export to hitspec format conversion.
package insomnia

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Converter converts Insomnia exports to hitspec format.
type Converter struct {
	generateAssertions bool
}

// Option is a functional option for Converter.
type Option func(*Converter)

// WithAssertions configures whether to generate test assertions.
func WithAssertions(generate bool) Option {
	return func(c *Converter) {
		c.generateAssertions = generate
	}
}

// NewConverter creates a new Insomnia converter.
func NewConverter(opts ...Option) *Converter {
	c := &Converter{
		generateAssertions: true,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Export represents an Insomnia export file.
type Export struct {
	Type      string     `json:"_type"`
	ExportFormat int      `json:"__export_format"`
	Resources []Resource `json:"resources"`
}

// Resource represents an Insomnia resource (request, folder, environment, etc).
type Resource struct {
	ID           string        `json:"_id"`
	Type         string        `json:"_type"`
	ParentID     string        `json:"parentId"`
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	Method       string        `json:"method,omitempty"`
	URL          string        `json:"url,omitempty"`
	Headers      []Header      `json:"headers,omitempty"`
	Body         *Body         `json:"body,omitempty"`
	Parameters   []Parameter   `json:"parameters,omitempty"`
	Authentication *Auth       `json:"authentication,omitempty"`
}

// Header represents an Insomnia header.
type Header struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled,omitempty"`
}

// Body represents an Insomnia request body.
type Body struct {
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

// Parameter represents an Insomnia query parameter.
type Parameter struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled,omitempty"`
}

// Auth represents Insomnia authentication.
type Auth struct {
	Type     string `json:"type"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
}

// ConvertFile converts an Insomnia export file to hitspec format.
func (c *Converter) ConvertFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return c.Convert(data)
}

// Convert converts Insomnia export JSON to hitspec format.
func (c *Converter) Convert(data []byte) (string, error) {
	var export Export
	if err := json.Unmarshal(data, &export); err != nil {
		return "", fmt.Errorf("failed to parse Insomnia export: %w", err)
	}

	// Build parent-child relationships
	children := make(map[string][]Resource)
	requests := make([]Resource, 0)
	folders := make(map[string]Resource)

	for _, res := range export.Resources {
		switch res.Type {
		case "request":
			requests = append(requests, res)
			children[res.ParentID] = append(children[res.ParentID], res)
		case "request_group":
			folders[res.ID] = res
			children[res.ParentID] = append(children[res.ParentID], res)
		}
	}

	var sb strings.Builder
	sb.WriteString("# Generated from Insomnia export\n\n")

	// Process requests, grouping by folder
	c.writeRequests(&sb, requests, folders)

	return sb.String(), nil
}

func (c *Converter) writeRequests(sb *strings.Builder, requests []Resource, folders map[string]Resource) {
	for _, req := range requests {
		// Get folder path
		folderPath := c.getFolderPath(req.ParentID, folders)

		// Request separator
		sb.WriteString("### ")
		if folderPath != "" {
			sb.WriteString(folderPath)
			sb.WriteString(" - ")
		}
		sb.WriteString(req.Name)
		sb.WriteString("\n")

		// Name annotation
		sb.WriteString("# @name ")
		sb.WriteString(sanitizeName(req.Name))
		sb.WriteString("\n")

		// Description
		if req.Description != "" {
			sb.WriteString("# @description ")
			sb.WriteString(strings.ReplaceAll(req.Description, "\n", " "))
			sb.WriteString("\n")
		}

		// Auth annotation
		if req.Authentication != nil {
			c.writeAuth(sb, req.Authentication)
		}

		// Method and URL
		method := req.Method
		if method == "" {
			method = "GET"
		}
		sb.WriteString(method)
		sb.WriteString(" ")
		sb.WriteString(c.convertURL(req.URL))
		sb.WriteString("\n")

		// Query parameters (as separate lines)
		for _, param := range req.Parameters {
			if param.Disabled {
				continue
			}
			sb.WriteString("? ")
			sb.WriteString(param.Name)
			sb.WriteString(" = ")
			sb.WriteString(c.convertVariable(param.Value))
			sb.WriteString("\n")
		}

		// Headers
		for _, h := range req.Headers {
			if h.Disabled {
				continue
			}
			sb.WriteString(h.Name)
			sb.WriteString(": ")
			sb.WriteString(c.convertVariable(h.Value))
			sb.WriteString("\n")
		}

		// Body
		if req.Body != nil && req.Body.Text != "" {
			sb.WriteString("\n")
			sb.WriteString(c.convertVariable(req.Body.Text))
			sb.WriteString("\n")
		}

		// Assertions
		if c.generateAssertions {
			sb.WriteString("\n>>>\n")
			sb.WriteString("expect status >= 200\n")
			sb.WriteString("expect status < 400\n")
			sb.WriteString("<<<\n")
		}

		sb.WriteString("\n")
	}
}

func (c *Converter) writeAuth(sb *strings.Builder, auth *Auth) {
	switch auth.Type {
	case "basic":
		if auth.Username != "" {
			sb.WriteString("# @auth basic ")
			sb.WriteString(c.convertVariable(auth.Username))
			sb.WriteString(" ")
			sb.WriteString(c.convertVariable(auth.Password))
			sb.WriteString("\n")
		}
	case "bearer":
		if auth.Token != "" {
			sb.WriteString("# @auth bearer ")
			sb.WriteString(c.convertVariable(auth.Token))
			sb.WriteString("\n")
		}
	}
}

func (c *Converter) getFolderPath(parentID string, folders map[string]Resource) string {
	var path []string
	currentID := parentID

	for {
		folder, exists := folders[currentID]
		if !exists {
			break
		}
		path = append([]string{folder.Name}, path...)
		currentID = folder.ParentID
	}

	return strings.Join(path, "/")
}

func (c *Converter) convertURL(url string) string {
	return c.convertVariable(url)
}

// convertVariable converts Insomnia variable syntax to hitspec syntax.
// Insomnia uses {{ _.variableName }} or {{ variableName }}
func (c *Converter) convertVariable(s string) string {
	// Convert {{ _.variableName }} to {{variableName}}
	s = regexp.MustCompile(`\{\{\s*_\.(\w+)\s*\}\}`).ReplaceAllString(s, "{{$1}}")
	// Convert {{ variableName }} to {{variableName}} (normalize spaces)
	s = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`).ReplaceAllString(s, "{{$1}}")
	return s
}

// sanitizeName sanitizes a name for use as an identifier.
func sanitizeName(name string) string {
	// Replace non-alphanumeric characters with underscores
	result := regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(name, "_")

	// Remove leading/trailing underscores
	result = strings.Trim(result, "_")

	// Collapse multiple underscores
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}

	// Convert to camelCase
	return strings.ToLower(result)
}
