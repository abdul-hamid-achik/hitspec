package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	maxToolPages           = 1000
	httpCloseTimeout       = 2 * time.Second
	stdioTerminateDuration = 2 * time.Second
)

var recommendedToolName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

var (
	// ErrToolNotFound reports a requested tool that the server did not advertise.
	ErrToolNotFound = errors.New("MCP tool not found")
	// ErrContractViolation reports arguments or structured output that do not
	// satisfy an advertised tool schema.
	ErrContractViolation = errors.New("MCP tool contract violation")
)

type clientSession interface {
	InitializeResult() *sdkmcp.InitializeResult
	ListTools(context.Context, *sdkmcp.ListToolsParams) (*sdkmcp.ListToolsResult, error)
	CallTool(context.Context, *sdkmcp.CallToolParams) (*sdkmcp.CallToolResult, error)
	Close() error
}

// Client owns one negotiated MCP session.
type Client struct {
	session    clientSession
	transport  string
	httpClient *http.Client
}

// Connect validates target and negotiates an MCP session.
func Connect(ctx context.Context, target Target, clientVersion string) (*Client, error) {
	if err := target.Validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(clientVersion) == "" {
		clientVersion = "dev"
	}

	var (
		transport  sdkmcp.Transport
		httpClient *http.Client
	)
	transportName := "stdio"
	if target.URL != "" {
		headers, err := parseHeaders(target.Headers)
		if err != nil {
			return nil, err
		}
		httpClient = &http.Client{
			Transport: &headerTransport{base: http.DefaultTransport, headers: headers},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		if deadline, ok := ctx.Deadline(); ok {
			httpClient.Timeout = time.Until(deadline)
			if httpClient.Timeout <= 0 {
				return nil, context.DeadlineExceeded
			}
		}
		transport = &sdkmcp.StreamableClientTransport{
			Endpoint:             target.URL,
			HTTPClient:           httpClient,
			DisableStandaloneSSE: true,
		}
		transportName = "streamable-http"
	} else {
		child := exec.CommandContext(ctx, target.Command[0], target.Command[1:]...) // #nosec G204 -- argv is the explicit CLI target; no shell is used.
		child.Dir = target.Dir
		child.Stderr = target.Stderr
		if len(target.Env) > 0 {
			child.Env = mergeEnvironment(os.Environ(), target.Env)
		}
		transport = &sdkmcp.CommandTransport{
			Command:           child,
			TerminateDuration: stdioTerminateDuration,
		}
	}

	sdkClient := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "hitspec-mcp-client", Title: "Hitspec MCP client", Version: clientVersion},
		nil,
	)
	session, err := sdkClient.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect over %s: %w", transportName, err)
	}
	return &Client{session: session, transport: transportName, httpClient: httpClient}, nil
}

// Validate ensures the target selects one transport and contains only safe,
// unambiguous connection settings.
func (t Target) Validate() error {
	hasURL := strings.TrimSpace(t.URL) != ""
	hasCommand := len(t.Command) > 0
	if hasURL == hasCommand {
		return errors.New("select exactly one MCP target: --url or a server command after --")
	}
	if hasURL {
		parsed, err := url.Parse(t.URL)
		if err != nil {
			return errors.New("invalid MCP URL")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return errors.New("MCP URL scheme must be http or https")
		}
		if parsed.Host == "" {
			return errors.New("MCP URL must include a host")
		}
		if parsed.User != nil {
			return errors.New("MCP URL must not contain credentials; use --header for authorization")
		}
		if parsed.Fragment != "" {
			return errors.New("MCP URL must not contain a fragment")
		}
		if t.Dir != "" || len(t.Env) > 0 {
			return errors.New("--cwd and --env are only valid for stdio targets")
		}
		if _, err := parseHeaders(t.Headers); err != nil {
			return err
		}
		return nil
	}
	if strings.TrimSpace(t.Command[0]) == "" {
		return errors.New("MCP server command must not be empty")
	}
	if len(t.Headers) > 0 {
		return errors.New("--header is only valid with --url")
	}
	for _, item := range t.Env {
		key, _, ok := strings.Cut(item, "=")
		if !ok || key == "" || strings.ContainsRune(item, '\x00') {
			return errors.New("invalid environment override; expected KEY=VALUE")
		}
	}
	return nil
}

// Close gracefully closes the MCP session and its underlying transport.
func (c *Client) Close() error {
	if c == nil || c.session == nil {
		return nil
	}
	if c.httpClient != nil {
		c.httpClient.Timeout = httpCloseTimeout
	}
	return c.session.Close()
}

// Probe completes tools discovery, resolves advertised schemas, and checks
// that every required tool is present.
func (c *Client) Probe(ctx context.Context, requiredTools []string) (*ProbeReport, error) {
	initResult, err := c.initializeResult()
	if err != nil {
		return nil, err
	}
	if initResult.Capabilities == nil || initResult.Capabilities.Tools == nil {
		return nil, errors.New("MCP server did not declare the tools capability")
	}
	tools, err := collectTools(ctx, c.session)
	if err != nil {
		return nil, err
	}

	report := &ProbeReport{
		OK:              true,
		Transport:       c.transport,
		ProtocolVersion: initResult.ProtocolVersion,
		Server:          serverInfo(initResult.ServerInfo),
		Capabilities:    capabilityNames(initResult.Capabilities),
		Tools:           make([]Tool, 0, len(tools)),
	}
	available := make(map[string]struct{}, len(tools))
	for _, advertised := range tools {
		if advertised == nil {
			return nil, errors.New("tools/list returned a null tool")
		}
		if _, err := compileSchema(advertised.Name, "input", advertised.InputSchema); err != nil {
			return nil, err
		}
		if advertised.OutputSchema != nil {
			if _, err := compileSchema(advertised.Name, "output", advertised.OutputSchema); err != nil {
				return nil, err
			}
		}
		if _, exists := available[advertised.Name]; exists {
			report.Warnings = append(report.Warnings, fmt.Sprintf("server advertised duplicate tool %q", advertised.Name))
		}
		available[advertised.Name] = struct{}{}
		if !recommendedToolName.MatchString(advertised.Name) {
			report.Warnings = append(report.Warnings, fmt.Sprintf("tool name %q is outside the recommended MCP character set or length", advertised.Name))
		}
		report.Tools = append(report.Tools, Tool{
			Name:         advertised.Name,
			Title:        advertised.Title,
			Description:  advertised.Description,
			InputSchema:  advertised.InputSchema,
			OutputSchema: advertised.OutputSchema,
		})
	}
	sort.SliceStable(report.Tools, func(i, j int) bool { return report.Tools[i].Name < report.Tools[j].Name })
	report.Warnings = uniqueSorted(report.Warnings)

	required := uniqueSorted(requiredTools)
	for _, name := range required {
		if _, exists := available[name]; !exists {
			report.MissingTools = append(report.MissingTools, name)
		}
	}
	report.OK = len(report.MissingTools) == 0
	return report, nil
}

// Call invokes one advertised tool. Tool execution failures are returned in the
// report with IsError set; protocol and transport failures are returned as Go
// errors.
func (c *Client) Call(ctx context.Context, toolName string, arguments map[string]any) (*CallReport, error) {
	probe, err := c.Probe(ctx, nil)
	if err != nil {
		return nil, err
	}
	var selected *Tool
	for _, tool := range probe.Tools {
		if tool.Name == toolName {
			toolCopy := tool
			selected = &toolCopy
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("%w: server does not advertise %q", ErrToolNotFound, toolName)
	}
	inputSchema, err := compileSchema(toolName, "input", selected.InputSchema)
	if err != nil {
		return nil, err
	}
	if err := inputSchema.Validate(schemaValidationValue(arguments)); err != nil {
		return nil, fmt.Errorf("%w: arguments for tool %q do not match inputSchema: %v", ErrContractViolation, toolName, err)
	}

	result, err := c.session.CallTool(ctx, &sdkmcp.CallToolParams{Name: toolName, Arguments: arguments})
	if err != nil {
		return nil, fmt.Errorf("call MCP tool %q: %w", toolName, err)
	}
	if result == nil {
		return nil, fmt.Errorf("MCP tool %q returned no result", toolName)
	}
	if result.Content == nil {
		return nil, fmt.Errorf("MCP tool %q result omitted content", toolName)
	}
	if selected.OutputSchema != nil && !result.IsError {
		if result.StructuredContent == nil {
			return nil, fmt.Errorf("%w: tool %q advertised outputSchema but omitted structuredContent", ErrContractViolation, toolName)
		}
		outputSchema, err := compileSchema(toolName, "output", selected.OutputSchema)
		if err != nil {
			return nil, err
		}
		if err := outputSchema.Validate(schemaValidationValue(result.StructuredContent)); err != nil {
			return nil, fmt.Errorf("%w: structuredContent from tool %q does not match outputSchema: %v", ErrContractViolation, toolName, err)
		}
	}
	content := make([]json.RawMessage, 0, len(result.Content))
	for index, item := range result.Content {
		if item == nil {
			return nil, fmt.Errorf("MCP tool %q returned null content at index %d", toolName, index)
		}
		encoded, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("encode content item %d from MCP tool %q: %w", index, toolName, err)
		}
		content = append(content, encoded)
	}
	return &CallReport{
		OK:                !result.IsError,
		Transport:         c.transport,
		ProtocolVersion:   probe.ProtocolVersion,
		Server:            probe.Server,
		Tool:              toolName,
		Content:           content,
		StructuredContent: result.StructuredContent,
		IsError:           result.IsError,
	}, nil
}

func (c *Client) initializeResult() (*sdkmcp.InitializeResult, error) {
	result := c.session.InitializeResult()
	if result == nil {
		return nil, errors.New("MCP session has no initialize result")
	}
	if result.ServerInfo == nil {
		return nil, errors.New("MCP initialize result omitted serverInfo")
	}
	if strings.TrimSpace(result.ProtocolVersion) == "" {
		return nil, errors.New("MCP initialize result omitted protocolVersion")
	}
	return result, nil
}

func collectTools(ctx context.Context, session clientSession) ([]*sdkmcp.Tool, error) {
	var tools []*sdkmcp.Tool
	cursor := ""
	seen := make(map[string]struct{})
	for page := 0; page < maxToolPages; page++ {
		var params *sdkmcp.ListToolsParams
		if cursor != "" {
			params = &sdkmcp.ListToolsParams{Cursor: cursor}
		}
		result, err := session.ListTools(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("list MCP tools: %w", err)
		}
		if result == nil {
			return nil, errors.New("tools/list returned no result")
		}
		if result.Tools == nil {
			return nil, errors.New("tools/list result omitted tools")
		}
		tools = append(tools, result.Tools...)
		if result.NextCursor == "" {
			return tools, nil
		}
		if _, exists := seen[result.NextCursor]; exists {
			return nil, fmt.Errorf("tools/list repeated pagination cursor %q", result.NextCursor)
		}
		seen[result.NextCursor] = struct{}{}
		cursor = result.NextCursor
	}
	return nil, fmt.Errorf("tools/list exceeded the %d-page safety limit", maxToolPages)
}

func compileSchema(toolName, kind string, schema any) (*jsonschema.Resolved, error) {
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("tool %q has an invalid %s schema: %w", toolName, kind, err)
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil || object == nil {
		return nil, fmt.Errorf("tool %q %s schema must be a JSON object", toolName, kind)
	}
	var parsed jsonschema.Schema
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		return nil, fmt.Errorf("tool %q has an invalid %s JSON Schema: %w", toolName, kind, err)
	}
	resolved, err := parsed.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("tool %q has an invalid or unresolved %s JSON Schema: %w", toolName, kind, err)
	}
	return resolved, nil
}

func schemaValidationValue(value any) any {
	switch typed := value.(type) {
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer
		}
		if number, err := typed.Float64(); err == nil {
			return number
		}
		return typed.String()
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, item := range typed {
			normalized[key] = schemaValidationValue(item)
		}
		return normalized
	case []any:
		normalized := make([]any, len(typed))
		for index, item := range typed {
			normalized[index] = schemaValidationValue(item)
		}
		return normalized
	default:
		return value
	}
}

func serverInfo(info *sdkmcp.Implementation) ServerInfo {
	return ServerInfo{Name: info.Name, Title: info.Title, Version: info.Version}
}

func capabilityNames(capabilities *sdkmcp.ServerCapabilities) []string {
	var names []string
	if capabilities.Completions != nil {
		names = append(names, "completions")
	}
	if capabilities.Logging != nil {
		names = append(names, "logging")
	}
	if capabilities.Prompts != nil {
		names = append(names, "prompts")
	}
	if capabilities.Resources != nil {
		names = append(names, "resources")
	}
	if capabilities.Tools != nil {
		names = append(names, "tools")
	}
	if len(capabilities.Extensions) > 0 {
		names = append(names, "extensions")
	}
	if len(capabilities.Experimental) > 0 {
		names = append(names, "experimental")
	}
	return names
}

func uniqueSorted(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

type headerTransport struct {
	base    http.RoundTripper
	headers http.Header
}

func (t *headerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	cloned := request.Clone(request.Context())
	cloned.Header = request.Header.Clone()
	for name, values := range t.headers {
		for _, value := range values {
			cloned.Header.Add(name, value)
		}
	}
	return t.base.RoundTrip(cloned)
}

func parseHeaders(values []string) (http.Header, error) {
	headers := make(http.Header)
	for _, raw := range values {
		name, value, ok := strings.Cut(raw, ":")
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if !ok || name == "" {
			return nil, errors.New("invalid HTTP header; expected 'Name: Value'")
		}
		if !validHeaderName(name) || !validHeaderValue(value) {
			return nil, errors.New("HTTP header name or value contains invalid characters")
		}
		switch strings.ToLower(name) {
		case "host", "content-length", "mcp-session-id", "mcp-protocol-version":
			return nil, fmt.Errorf("HTTP header %q is managed by the MCP transport", name)
		}
		headers.Add(name, value)
	}
	return headers, nil
}

func validHeaderName(value string) bool {
	if value == "" {
		return false
	}
	const separators = "()<>@,;:\\\"/[]?={} \t"
	for _, char := range value {
		if char <= ' ' || char >= '\x7f' || strings.ContainsRune(separators, char) {
			return false
		}
	}
	return true
}

func validHeaderValue(value string) bool {
	for _, char := range value {
		if (char < ' ' && char != '\t') || char == '\x7f' {
			return false
		}
	}
	return true
}

func mergeEnvironment(base, overrides []string) []string {
	result := append([]string(nil), base...)
	positions := make(map[string]int, len(result))
	for index, item := range result {
		key, _, _ := strings.Cut(item, "=")
		positions[key] = index
	}
	for _, item := range overrides {
		key, _, _ := strings.Cut(item, "=")
		if index, exists := positions[key]; exists {
			result[index] = item
		} else {
			positions[key] = len(result)
			result = append(result, item)
		}
	}
	return result
}
