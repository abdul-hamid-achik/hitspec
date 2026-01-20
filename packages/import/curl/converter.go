// Package curl provides curl command to hitspec format conversion.
package curl

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Converter converts curl commands to hitspec format.
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

// NewConverter creates a new curl converter.
func NewConverter(opts ...Option) *Converter {
	c := &Converter{
		generateAssertions: true,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ParsedCurl represents a parsed curl command.
type ParsedCurl struct {
	Method      string
	URL         string
	Headers     map[string]string
	Body        string
	BasicAuth   string
	Insecure    bool
	FollowRedirects bool
	Name        string
}

// ConvertCommand converts a single curl command to hitspec format.
func (c *Converter) ConvertCommand(curlCmd string) (string, error) {
	parsed, err := c.Parse(curlCmd)
	if err != nil {
		return "", err
	}
	return c.ToHitspec(parsed), nil
}

// ConvertFile converts a file containing curl commands to hitspec format.
func (c *Converter) ConvertFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var commands []string
	var currentCmd strings.Builder
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle line continuations
		if strings.HasSuffix(line, "\\") {
			currentCmd.WriteString(strings.TrimSuffix(line, "\\"))
			currentCmd.WriteString(" ")
			continue
		}

		currentCmd.WriteString(line)
		commands = append(commands, currentCmd.String())
		currentCmd.Reset()
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Handle any remaining command
	if currentCmd.Len() > 0 {
		commands = append(commands, currentCmd.String())
	}

	var sb strings.Builder
	sb.WriteString("# Generated from curl commands\n\n")

	for i, cmd := range commands {
		converted, err := c.ConvertCommand(cmd)
		if err != nil {
			return "", fmt.Errorf("failed to convert command %d: %w", i+1, err)
		}
		sb.WriteString(converted)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// Parse parses a curl command string into a ParsedCurl struct.
func (c *Converter) Parse(curlCmd string) (*ParsedCurl, error) {
	parsed := &ParsedCurl{
		Method:  "GET",
		Headers: make(map[string]string),
	}

	// Normalize the command
	curlCmd = strings.TrimSpace(curlCmd)

	// Remove "curl" prefix if present
	if strings.HasPrefix(curlCmd, "curl ") {
		curlCmd = strings.TrimPrefix(curlCmd, "curl ")
	} else if curlCmd == "curl" {
		return nil, fmt.Errorf("no URL specified")
	}

	// Tokenize the command respecting quotes
	tokens := tokenize(curlCmd)

	i := 0
	for i < len(tokens) {
		token := tokens[i]

		switch {
		case token == "-X" || token == "--request":
			if i+1 < len(tokens) {
				parsed.Method = strings.ToUpper(tokens[i+1])
				i += 2
			} else {
				return nil, fmt.Errorf("missing value for %s", token)
			}

		case token == "-H" || token == "--header":
			if i+1 < len(tokens) {
				header := tokens[i+1]
				parts := strings.SplitN(header, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					parsed.Headers[key] = value
				}
				i += 2
			} else {
				return nil, fmt.Errorf("missing value for %s", token)
			}

		case token == "-d" || token == "--data" || token == "--data-raw" || token == "--data-binary":
			if i+1 < len(tokens) {
				parsed.Body = tokens[i+1]
				// If body is set, default method to POST
				if parsed.Method == "GET" {
					parsed.Method = "POST"
				}
				i += 2
			} else {
				return nil, fmt.Errorf("missing value for %s", token)
			}

		case token == "-u" || token == "--user":
			if i+1 < len(tokens) {
				parsed.BasicAuth = tokens[i+1]
				i += 2
			} else {
				return nil, fmt.Errorf("missing value for %s", token)
			}

		case token == "-k" || token == "--insecure":
			parsed.Insecure = true
			i++

		case token == "-L" || token == "--location":
			parsed.FollowRedirects = true
			i++

		case token == "-A" || token == "--user-agent":
			if i+1 < len(tokens) {
				parsed.Headers["User-Agent"] = tokens[i+1]
				i += 2
			} else {
				return nil, fmt.Errorf("missing value for %s", token)
			}

		case token == "-e" || token == "--referer":
			if i+1 < len(tokens) {
				parsed.Headers["Referer"] = tokens[i+1]
				i += 2
			} else {
				return nil, fmt.Errorf("missing value for %s", token)
			}

		case token == "-b" || token == "--cookie":
			if i+1 < len(tokens) {
				parsed.Headers["Cookie"] = tokens[i+1]
				i += 2
			} else {
				return nil, fmt.Errorf("missing value for %s", token)
			}

		case strings.HasPrefix(token, "-"):
			// Skip unknown flags with potential values
			if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") && !isURL(tokens[i+1]) {
				i += 2
			} else {
				i++
			}

		default:
			// This should be the URL
			if parsed.URL == "" && isURL(token) {
				parsed.URL = token
			}
			i++
		}
	}

	if parsed.URL == "" {
		return nil, fmt.Errorf("no URL found in curl command")
	}

	// Generate a name from the URL
	parsed.Name = generateName(parsed.URL, parsed.Method)

	return parsed, nil
}

// ToHitspec converts a ParsedCurl to hitspec format.
func (c *Converter) ToHitspec(parsed *ParsedCurl) string {
	var sb strings.Builder

	// Request separator and name
	sb.WriteString("### ")
	sb.WriteString(parsed.Name)
	sb.WriteString("\n")

	sb.WriteString("# @name ")
	sb.WriteString(sanitizeName(parsed.Name))
	sb.WriteString("\n")

	// Auth annotation if present
	if parsed.BasicAuth != "" {
		parts := strings.SplitN(parsed.BasicAuth, ":", 2)
		if len(parts) == 2 {
			sb.WriteString("# @auth basic ")
			sb.WriteString(parts[0])
			sb.WriteString(" ")
			sb.WriteString(parts[1])
			sb.WriteString("\n")
		}
	}

	// Method and URL
	sb.WriteString(parsed.Method)
	sb.WriteString(" ")
	sb.WriteString(parsed.URL)
	sb.WriteString("\n")

	// Headers
	for key, value := range parsed.Headers {
		sb.WriteString(key)
		sb.WriteString(": ")
		sb.WriteString(value)
		sb.WriteString("\n")
	}

	// Body
	if parsed.Body != "" {
		sb.WriteString("\n")
		sb.WriteString(parsed.Body)
		sb.WriteString("\n")
	}

	// Assertions
	if c.generateAssertions {
		sb.WriteString("\n>>>\n")
		sb.WriteString("expect status >= 200\n")
		sb.WriteString("expect status < 400\n")
		sb.WriteString("<<<\n")
	}

	return sb.String()
}

// tokenize splits a curl command into tokens, respecting quotes.
func tokenize(cmd string) []string {
	var tokens []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for _, r := range cmd {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		switch r {
		case '\\':
			escaped = true
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			} else {
				current.WriteRune(r)
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			} else {
				current.WriteRune(r)
			}
		case ' ', '\t':
			if inSingleQuote || inDoubleQuote {
				current.WriteRune(r)
			} else if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// isURL checks if a string looks like a URL.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "{{")
}

// generateName generates a request name from the URL and method.
func generateName(url, method string) string {
	// Extract the path from the URL
	urlPattern := regexp.MustCompile(`https?://[^/]+(/[^?#]*)?`)
	matches := urlPattern.FindStringSubmatch(url)

	path := "/"
	if len(matches) > 1 && matches[1] != "" {
		path = matches[1]
	}

	// Clean up the path for a name
	path = strings.Trim(path, "/")
	if path == "" {
		path = "root"
	}

	// Replace path separators and other characters
	path = strings.ReplaceAll(path, "/", "_")
	path = strings.ReplaceAll(path, "-", "_")

	return strings.ToLower(method) + "_" + path
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

	return result
}
