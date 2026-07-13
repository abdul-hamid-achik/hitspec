package fetch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	hitspechttp "github.com/abdul-hamid-achik/hitspec/packages/http"
)

// SavedRequest selects and configures one request from a hitspec file.
type SavedRequest struct {
	Path             string
	Name             string
	Index            int // one-based; zero means unset
	Environment      string
	EnvFile          string
	ConfigFile       string
	Timeout          time.Duration
	MaxBodyBytes     int64
	MaxRedirects     int
	NoFollow         bool
	Insecure         bool
	Proxy            string
	DefaultUserAgent string
	NetworkPolicy    NetworkPolicy
	WorkspaceRoot    string // optional file-read boundary for agent-facing callers
}

// FetchSaved executes exactly one saved request. Assertions, hooks, shell
// commands, database assertions, and dependencies are not executed.
func FetchSaved(ctx context.Context, input SavedRequest) (*Result, error) {
	parsed, err := parser.ParseFile(input.Path)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", input.Path, err)
	}
	selected, err := selectRequest(parsed, input.Name, input.Index)
	if err != nil {
		return nil, err
	}
	if selected.Metadata != nil && len(selected.Metadata.Depends) > 0 {
		return nil, fmt.Errorf("request %q declares @depends; use hitspec run or fetch a dependency-free request", selected.Name)
	}
	var fileConfig *config.Config
	if input.ConfigFile == "" {
		fileConfig, err = config.FindAndLoadConfig(filepath.Dir(input.Path))
	} else {
		fileConfig, err = config.LoadConfig(input.ConfigFile)
	}
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	environmentName := input.Environment
	if environmentName == "" {
		environmentName = fileConfig.DefaultEnvironment
	}
	if environmentName == "" {
		environmentName = "dev"
	}
	resolver := env.NewResolver()
	if input.EnvFile != "" {
		if err := resolver.LoadDotEnv(input.EnvFile); err != nil {
			return nil, fmt.Errorf("load env file: %w", err)
		}
	}
	environment, err := env.LoadEnvironment(filepath.Dir(input.Path), environmentName, fileConfig.Environments)
	if err != nil {
		return nil, fmt.Errorf("load environment %q: %w", environmentName, err)
	}
	resolver.SetVariables(environment.Variables)
	for _, variable := range parsed.Variables {
		resolver.SetVariable(variable.Name, variable.Value)
	}
	if input.WorkspaceRoot != "" {
		if err := validateBodyFiles(input.WorkspaceRoot, filepath.Dir(input.Path), selected, resolver.Resolve); err != nil {
			return nil, err
		}
	}
	request := hitspechttp.BuildRequestFromASTWithBaseDir(selected, resolver.Resolve, filepath.Dir(input.Path))
	if input.DefaultUserAgent != "" && request.Headers["User-Agent"] == "" {
		request.Headers["User-Agent"] = input.DefaultUserAgent
	}
	effectiveTimeout := request.Timeout
	if fileConfig.Timeout > 0 && effectiveTimeout <= 0 {
		effectiveTimeout = time.Duration(fileConfig.Timeout) * time.Millisecond
	}
	if input.Timeout > 0 {
		effectiveTimeout = input.Timeout
	}
	if effectiveTimeout <= 0 {
		effectiveTimeout = DefaultTimeout
	}
	maxBytes := input.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}
	maxRedirects := input.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = fileConfig.MaxRedirects
	}
	if maxRedirects <= 0 {
		maxRedirects = DefaultMaxRedirects
	}
	proxy := fileConfig.Proxy
	if input.Proxy != "" {
		proxy = input.Proxy
	}
	if input.NetworkPolicy == NetworkPublicOnly {
		if request.DigestAuth != nil || request.AWSAuth != nil || request.OAuth2Auth != nil || len(request.Multipart) > 0 {
			return nil, errors.New("saved request uses an auth or multipart mode unavailable under the public-only network policy")
		}
		headers := make(http.Header, len(fileConfig.Headers)+len(request.Headers))
		for key, value := range fileConfig.Headers {
			headers.Set(key, value)
		}
		for key, value := range request.Headers {
			headers.Set(key, value)
		}
		return NewService().Fetch(ctx, Request{
			Method: request.Method, URL: request.URL, Headers: headers, Body: []byte(request.Body),
			Timeout: effectiveTimeout, FollowRedirects: fileConfig.GetFollowRedirects() && !input.NoFollow,
			MaxRedirects: maxRedirects, Insecure: input.Insecure || !fileConfig.GetValidateSSL(),
			Proxy: proxy, MaxBodyBytes: maxBytes, UserAgent: input.DefaultUserAgent,
			NetworkPolicy: NetworkPublicOnly,
		})
	}
	client := hitspechttp.NewClient(
		hitspechttp.WithTimeout(effectiveTimeout),
		hitspechttp.WithFollowRedirects(fileConfig.GetFollowRedirects() && !input.NoFollow),
		hitspechttp.WithMaxRedirects(maxRedirects),
		hitspechttp.WithValidateSSL(fileConfig.GetValidateSSL() && !input.Insecure),
		hitspechttp.WithProxy(proxy),
		hitspechttp.WithDefaultHeaders(fileConfig.Headers),
		hitspechttp.WithMaxBodyBytes(maxBytes),
	)
	response, err := client.DoWithContext(ctx, request)
	if err != nil {
		return nil, err
	}
	headers := make(http.Header, len(response.Headers))
	for key, value := range response.Headers {
		headers.Set(key, value)
	}
	finalURL := response.FinalURL
	if finalURL == "" {
		finalURL = request.URL
	}
	return &Result{
		RequestedURL: request.URL, FinalURL: finalURL, Status: response.Status,
		StatusCode: response.StatusCode, Headers: headers, Body: response.Body,
		ContentType: response.ContentType(), Duration: response.Duration,
	}, nil
}

func validateBodyFiles(workspace, baseDir string, request *parser.Request, resolve func(string) string) error {
	if request.Body == nil {
		return nil
	}
	var paths []string
	if request.Body.ContentType == parser.BodyFile && request.Body.FilePath != "" {
		paths = append(paths, request.Body.FilePath)
	}
	if request.Body.ContentType == parser.BodyMultipart {
		for _, field := range request.Body.Multipart {
			if field.Type == parser.MultipartFieldFile && field.Path != "" {
				paths = append(paths, field.Path)
			}
		}
	}
	workspace, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		return fmt.Errorf("resolve workspace: %w", err)
	}
	for _, raw := range paths {
		path := resolve(raw)
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		canonical, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("resolve request body file: %w", err)
		}
		relative, err := filepath.Rel(workspace, canonical)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errors.New("request body file escapes the workspace")
		}
	}
	return nil
}

func selectRequest(file *parser.File, name string, index int) (*parser.Request, error) {
	if name != "" && index != 0 {
		return nil, errors.New("name and index are mutually exclusive")
	}
	if index < 0 {
		return nil, errors.New("index must be greater than zero")
	}
	if index > 0 {
		if index > len(file.Requests) {
			return nil, fmt.Errorf("request index %d is out of range (file contains %d requests)", index, len(file.Requests))
		}
		return file.Requests[index-1], nil
	}
	if name != "" {
		var matches []*parser.Request
		for _, request := range file.Requests {
			if request.Name == name {
				matches = append(matches, request)
			}
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("request %q was not found in %s", name, file.Path)
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("request name %q is ambiguous in %s", name, file.Path)
		}
		return matches[0], nil
	}
	if len(file.Requests) != 1 {
		return nil, fmt.Errorf("%s contains %d requests; select exactly one with name or index", file.Path, len(file.Requests))
	}
	return file.Requests[0], nil
}
