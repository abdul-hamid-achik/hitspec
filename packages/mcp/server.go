// Package mcp exposes a bounded, workspace-scoped Hitspec MCP server.
package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/artifact"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/fetch"
	"github.com/abdul-hamid-achik/hitspec/packages/search"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const instructions = `hitspec discovers and fetches public web content through bounded tools.
Use hitspec_search_web for live discovery and treat its snippets as candidates, not verified evidence.
Use hitspec_capture_webpage only when a durable file.cheap artifact is wanted.
Use hitspec_list_requests to discover saved requests and hitspec_validate before execution when a file
may be malformed. hitspec_fetch can use a direct public URL or one request in the fixed workspace. It
never executes shell hooks or database assertions and never persists a hidden artifact.`

// Options configures server-owned limits and network authority.
type Options struct {
	MaxBodyBytes        int64
	Timeout             time.Duration
	AllowPrivateNetwork bool
	SearchProvider      search.Provider
	ArtifactSink        artifact.Sink
}

// Server wraps the stdio MCP transport and its fixed workspace.
type Server struct {
	workspace    string
	maxBodyBytes int64
	timeout      time.Duration
	allowPrivate bool
	search       search.Provider
	artifacts    artifact.Sink
	webFetcher   webFetcher
	srv          *sdkmcp.Server
}

// NewServer validates the workspace and constructs the MCP surface.
func NewServer(version, workspace string, options Options) (*Server, error) {
	root, err := canonicalWorkspace(workspace)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(version) == "" {
		version = "dev"
	}
	if options.MaxBodyBytes <= 0 {
		options.MaxBodyBytes = 1 << 20
	}
	if options.Timeout <= 0 {
		options.Timeout = fetch.DefaultTimeout
	}
	server := &Server{
		workspace: root, maxBodyBytes: options.MaxBodyBytes,
		timeout: options.Timeout, allowPrivate: options.AllowPrivateNetwork,
		search: options.SearchProvider, artifacts: options.ArtifactSink,
		webFetcher: fetch.NewService(),
	}
	server.srv = sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "hitspec", Title: "hitspec HTTP tools", Version: version},
		&sdkmcp.ServerOptions{Instructions: instructions},
	)
	server.register()
	return server, nil
}

// Run serves MCP JSON-RPC over stdin/stdout until cancellation.
func (s *Server) Run(ctx context.Context) error {
	return s.serve(ctx, &sdkmcp.StdioTransport{})
}

func (s *Server) serve(ctx context.Context, transport sdkmcp.Transport) error {
	return s.srv.Run(ctx, transport)
}

type fetchInput struct {
	URL         string            `json:"url,omitempty" jsonschema:"direct HTTP(S) URL; mutually exclusive with file"`
	File        string            `json:"file,omitempty" jsonschema:"workspace-relative .http or .hitspec file; mutually exclusive with url"`
	Name        string            `json:"name,omitempty" jsonschema:"saved request name"`
	Index       int               `json:"index,omitempty" jsonschema:"one-based saved request index"`
	Environment string            `json:"environment,omitempty" jsonschema:"saved request environment"`
	EnvFile     string            `json:"env_file,omitempty" jsonschema:"workspace-relative dotenv file"`
	ConfigFile  string            `json:"config_file,omitempty" jsonschema:"workspace-relative hitspec config file"`
	Method      string            `json:"method,omitempty" jsonschema:"direct URL HTTP method; defaults to GET or POST when body is set"`
	Headers     map[string]string `json:"headers,omitempty" jsonschema:"direct URL request headers"`
	Body        string            `json:"body,omitempty" jsonschema:"direct URL UTF-8 request body"`
	Format      string            `json:"format,omitempty" jsonschema:"raw, text, markdown, or json; defaults to text"`
	NoFollow    bool              `json:"no_follow,omitempty" jsonschema:"do not follow redirects"`
}

type listInput struct {
	Path string `json:"path,omitempty" jsonschema:"workspace-relative directory or file; defaults to workspace root"`
}

type listedRequest struct {
	Name   string   `json:"name,omitempty"`
	Method string   `json:"method"`
	Line   int      `json:"line"`
	Tags   []string `json:"tags,omitempty"`
}

type listedFile struct {
	File     string          `json:"file"`
	Requests []listedRequest `json:"requests"`
}

type listOutput struct {
	Files []listedFile `json:"files"`
}

type validateInput struct {
	File string `json:"file" jsonschema:"required,workspace-relative .http or .hitspec file"`
}

type validateOutput struct {
	File     string   `json:"file"`
	Valid    bool     `json:"valid"`
	Requests int      `json:"requests"`
	Errors   []string `json:"errors,omitempty"`
}

func (s *Server) register() {
	destructive, openWorld := false, true
	sdkmcp.AddTool(s.srv, &sdkmcp.Tool{
		Name: "hitspec_fetch", Title: "Fetch one HTTP response",
		Description: "Fetch one direct HTTP(S) URL or one saved Hitspec request and return bounded raw base64, readable text, Markdown, or JSON. Does not persist artifacts or execute shell/database blocks.",
		Annotations: &sdkmcp.ToolAnnotations{
			Title: "Fetch one HTTP response", ReadOnlyHint: false, DestructiveHint: &destructive,
			IdempotentHint: false, OpenWorldHint: &openWorld,
		},
	}, s.handleFetch)

	s.registerWebTools(destructive, openWorld)

	readOnly, closedWorld, idempotent := true, false, true
	sdkmcp.AddTool(s.srv, &sdkmcp.Tool{
		Name: "hitspec_list_requests", Title: "List saved Hitspec requests",
		Description: "List request names, methods, lines, and tags in workspace .http and .hitspec files without executing them.",
		Annotations: &sdkmcp.ToolAnnotations{
			Title: "List saved Hitspec requests", ReadOnlyHint: readOnly, DestructiveHint: &destructive,
			IdempotentHint: idempotent, OpenWorldHint: &closedWorld,
		},
	}, s.handleList)
	sdkmcp.AddTool(s.srv, &sdkmcp.Tool{
		Name: "hitspec_validate", Title: "Validate a Hitspec file",
		Description: "Parse and structurally validate one workspace .http or .hitspec file without executing requests.",
		Annotations: &sdkmcp.ToolAnnotations{
			Title: "Validate a Hitspec file", ReadOnlyHint: readOnly, DestructiveHint: &destructive,
			IdempotentHint: idempotent, OpenWorldHint: &closedWorld,
		},
	}, s.handleValidate)
}

func (s *Server) handleFetch(ctx context.Context, _ *sdkmcp.CallToolRequest, input fetchInput) (*sdkmcp.CallToolResult, any, error) {
	if (input.URL == "") == (input.File == "") {
		return nil, nil, errors.New("provide exactly one of url or file")
	}
	formatName := input.Format
	if formatName == "" {
		formatName = string(fetch.FormatText)
	}
	format, err := fetch.ParseFormat(formatName)
	if err != nil {
		return nil, nil, err
	}
	var result *fetch.Result
	if input.URL != "" {
		if input.Name != "" || input.Index != 0 || input.Environment != "" || input.EnvFile != "" || input.ConfigFile != "" {
			return nil, nil, errors.New("saved-request fields cannot be combined with url")
		}
		headers := make(http.Header, len(input.Headers))
		for key, value := range input.Headers {
			headers.Set(key, value)
		}
		method := input.Method
		if method == "" && input.Body != "" {
			method = http.MethodPost
		}
		policy := fetch.NetworkPublicOnly
		if s.allowPrivate {
			policy = fetch.NetworkAny
		}
		result, err = fetch.NewService().Fetch(ctx, fetch.Request{
			Method: method, URL: input.URL, Headers: headers, Body: []byte(input.Body),
			Timeout: s.timeout, FollowRedirects: !input.NoFollow,
			MaxRedirects: fetch.DefaultMaxRedirects, MaxBodyBytes: s.maxBodyBytes,
			UserAgent: "hitspec-mcp", NetworkPolicy: policy,
		})
	} else {
		if input.Method != "" || len(input.Headers) != 0 || input.Body != "" {
			return nil, nil, errors.New("method, headers, and body apply only to url")
		}
		file, resolveErr := s.resolveFile(input.File, true)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}
		envFile, resolveErr := s.resolveOptionalFile(input.EnvFile, false)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}
		configFile, resolveErr := s.resolveOptionalFile(input.ConfigFile, false)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}
		policy := fetch.NetworkPublicOnly
		if s.allowPrivate {
			policy = fetch.NetworkAny
		}
		result, err = fetch.FetchSaved(ctx, fetch.SavedRequest{
			Path: file, Name: input.Name, Index: input.Index, Environment: input.Environment,
			EnvFile: envFile, ConfigFile: configFile, Timeout: s.timeout,
			MaxBodyBytes: s.maxBodyBytes, MaxRedirects: fetch.DefaultMaxRedirects, NoFollow: input.NoFollow,
			DefaultUserAgent: "hitspec-mcp", NetworkPolicy: policy, WorkspaceRoot: s.workspace,
		})
	}
	if err != nil {
		return nil, nil, err
	}
	content, err := renderToolContent(ctx, result, format)
	if err != nil {
		return nil, nil, err
	}
	if int64(len(content)) > expandedBodyLimit(s.maxBodyBytes) {
		return nil, nil, errors.New("rendered tool result exceeds the server response limit")
	}
	callResult := &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: content}}}
	return callResult, nil, nil
}

func expandedBodyLimit(maxBodyBytes int64) int64 {
	const maximumInt64 = int64(1<<63 - 1)
	if maxBodyBytes > (maximumInt64-4096)/2 {
		return maximumInt64
	}
	return maxBodyBytes*2 + 4096
}

func renderToolContent(ctx context.Context, result *fetch.Result, format fetch.Format) (string, error) {
	if format == fetch.FormatRaw {
		envelope := struct {
			Source      string `json:"source"`
			Status      string `json:"status"`
			StatusCode  int    `json:"status_code"`
			ContentType string `json:"content_type,omitempty"`
			Size        int    `json:"size"`
			Encoding    string `json:"encoding"`
			Data        string `json:"data"`
		}{
			Source: fetch.SanitizeURL(result.FinalURL), Status: result.Status,
			StatusCode: result.StatusCode, ContentType: result.ContentType, Size: len(result.Body),
			Encoding: "base64", Data: base64.StdEncoding.EncodeToString(result.Body),
		}
		encoded, err := json.MarshalIndent(envelope, "", "  ")
		return string(encoded), err
	}
	rendered, err := fetch.Render(ctx, result, format)
	if err != nil {
		return "", err
	}
	return string(rendered), nil
}

func (s *Server) handleList(_ context.Context, _ *sdkmcp.CallToolRequest, input listInput) (*sdkmcp.CallToolResult, any, error) {
	target := s.workspace
	if input.Path != "" {
		resolved, err := s.resolvePath(input.Path)
		if err != nil {
			return nil, nil, err
		}
		target = resolved
	}
	var paths []string
	info, err := os.Lstat(target)
	if err != nil {
		return nil, nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, errors.New("symlink paths are not listed")
	}
	if info.IsDir() {
		err = filepath.WalkDir(target, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !entry.IsDir() && hitspecFile(path) {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	} else if hitspecFile(target) {
		paths = append(paths, target)
	} else {
		return nil, nil, errors.New("path is not a .http or .hitspec file")
	}
	sort.Strings(paths)
	output := &listOutput{Files: make([]listedFile, 0, len(paths))}
	for _, path := range paths {
		relative, _ := filepath.Rel(s.workspace, path)
		parsed, parseErr := parser.ParseFile(path)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", filepath.ToSlash(relative), parseErr)
		}
		file := listedFile{File: filepath.ToSlash(relative), Requests: make([]listedRequest, 0, len(parsed.Requests))}
		for _, request := range parsed.Requests {
			file.Requests = append(file.Requests, listedRequest{Name: request.Name, Method: request.Method, Line: request.Line, Tags: request.Tags})
		}
		output.Files = append(output.Files, file)
	}
	result, err := JSONTextResult(output)
	return result, nil, err
}

func (s *Server) handleValidate(_ context.Context, _ *sdkmcp.CallToolRequest, input validateInput) (*sdkmcp.CallToolResult, any, error) {
	path, err := s.resolveFile(input.File, true)
	if err != nil {
		return nil, nil, err
	}
	relative, _ := filepath.Rel(s.workspace, path)
	output := &validateOutput{File: filepath.ToSlash(relative)}
	parsed, parseErr := parser.ParseFile(path)
	if parseErr != nil {
		output.Errors = []string{parseErr.Error()}
		result, encodeErr := JSONTextResult(output)
		return result, nil, encodeErr
	}
	output.Requests = len(parsed.Requests)
	if len(parsed.Requests) == 0 {
		output.Errors = append(output.Errors, "no requests found")
	}
	for _, request := range parsed.Requests {
		if strings.TrimSpace(request.URL) == "" {
			output.Errors = append(output.Errors, fmt.Sprintf("request at line %d has an empty URL", request.Line))
		}
	}
	output.Valid = len(output.Errors) == 0
	result, encodeErr := JSONTextResult(output)
	return result, nil, encodeErr
}

// JSONTextResult intentionally avoids StructuredContent until Local Agent
// preserves it losslessly for unrecognized tools.
func JSONTextResult(value any) (*sdkmcp.CallToolResult, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(encoded)}}}, nil
}

func canonicalWorkspace(workspace string) (string, error) {
	if strings.TrimSpace(workspace) == "" {
		workspace = "."
	}
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve workspace symlinks: %w", err)
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("workspace is not a directory: %s", canonical)
	}
	return filepath.Clean(canonical), nil
}

func (s *Server) resolveOptionalFile(path string, requireHitspec bool) (string, error) {
	if path == "" {
		return "", nil
	}
	return s.resolveFile(path, requireHitspec)
}

func (s *Server) resolveFile(path string, requireHitspec bool) (string, error) {
	resolved, err := s.resolvePath(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("path must identify a regular file")
	}
	if requireHitspec && !hitspecFile(resolved) {
		return "", errors.New("file must end in .http or .hitspec")
	}
	return resolved, nil
}

func (s *Server) resolvePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path is required")
	}
	if filepath.IsAbs(path) {
		return "", errors.New("path must be workspace-relative")
	}
	candidate := filepath.Join(s.workspace, filepath.Clean(path))
	canonical, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	relative, err := filepath.Rel(s.workspace, canonical)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes the workspace")
	}
	return canonical, nil
}

func hitspecFile(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".http" || extension == ".hitspec"
}
