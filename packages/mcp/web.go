package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/abdul-hamid-achik/hitspec/packages/artifact"
	"github.com/abdul-hamid-achik/hitspec/packages/fetch"
	"github.com/abdul-hamid-achik/hitspec/packages/search"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/net/html"
)

const (
	searchResponseLimit     = 64 << 10
	maximumCaptureTags      = 20
	maximumCaptureName      = 80
	maximumCaptureTitle     = 300
	maximumReceiptFailures  = 16
	maximumReceiptErrorSize = 1024
)

type webFetcher interface {
	Fetch(context.Context, fetch.Request) (*fetch.Result, error)
}

type searchInput struct {
	Query          string   `json:"query" jsonschema:"required,live public-web search query; maximum 512 characters"`
	MaxResults     int      `json:"max_results,omitempty" jsonschema:"maximum results from 1 to 10; defaults to 5"`
	Language       string   `json:"language,omitempty" jsonschema:"optional BCP 47 language hint; unsupported provider mappings fail explicitly"`
	Freshness      string   `json:"freshness,omitempty" jsonschema:"any, day, week, month, or year; defaults to any"`
	IncludeDomains []string `json:"include_domains,omitempty" jsonschema:"at most 20 public domains"`
	ExcludeDomains []string `json:"exclude_domains,omitempty" jsonschema:"at most 20 public domains"`
}

// captureWebpageInput preserves the public schema exposed by the capture-only
// binary that preceded the consolidated MCP server.
type captureWebpageInput struct {
	URL   string   `json:"url" jsonschema:"required,absolute http(s) URL of the public static webpage to fetch"`
	Name  string   `json:"name,omitempty" jsonschema:"display name for the file.cheap stash; defaults to the HTML title"`
	Tags  []string `json:"tags,omitempty" jsonschema:"extra file.cheap tags; web and markdown are always included"`
	TTL   string   `json:"ttl,omitempty" jsonschema:"optional file.cheap retention such as 24h, 7d, 30d, or 2026-12-31; omitted uses file.cheap policy"`
	Index *bool    `json:"index,omitempty" jsonschema:"index the Markdown immediately for fcheap_search; defaults to true"`
}

type captureFailureOutput struct {
	ID    string `json:"id"`
	Stage string `json:"stage"`
	Error string `json:"error"`
}

type captureStashOutput struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name,omitempty"`
	Status         string                 `json:"status"`
	CreatedAt      string                 `json:"created_at,omitempty"`
	ExpiresAt      string                 `json:"expires_at,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
	ContentHash    string                 `json:"content_hash,omitempty"`
	FileCount      int                    `json:"file_count"`
	TotalSize      int64                  `json:"total_size"`
	Indexed        bool                   `json:"indexed"`
	IndexRequested bool                   `json:"index_requested"`
	Failed         []captureFailureOutput `json:"failed,omitempty"`
}

type captureWebpageOutput struct {
	URL           string             `json:"url"`
	FinalURL      string             `json:"final_url"`
	Title         string             `json:"title"`
	HTTPStatus    int                `json:"http_status"`
	ContentType   string             `json:"content_type"`
	MarkdownBytes int                `json:"markdown_bytes"`
	Stash         captureStashOutput `json:"stash"`
}

func (s *Server) registerWebTools(destructive, openWorld bool) {
	if s.search != nil {
		readOnly := true
		sdkmcp.AddTool(s.srv, &sdkmcp.Tool{
			Name: "hitspec_search_web", Title: "Search the live public web",
			Description: "Search the live public web through the server-configured provider and return at most 64 KiB of normalized discovery candidates. Snippets are discovery-only and are not verified evidence. Does not persist artifacts.",
			Annotations: &sdkmcp.ToolAnnotations{
				Title: "Search the live public web", ReadOnlyHint: readOnly,
				DestructiveHint: &destructive, IdempotentHint: false, OpenWorldHint: &openWorld,
			},
		}, s.handleSearchWeb)
	}
	if s.artifacts != nil {
		readOnly := false
		sdkmcp.AddTool(s.srv, &sdkmcp.Tool{
			Name: "hitspec_capture_webpage", Title: "Capture a webpage as Markdown",
			Description: "Fetch a public HTTP(S) URL, convert its static HTML to Markdown with absolute links, and persist it as a searchable file.cheap stash. Returns a compact stash receipt, not the full page.",
			Annotations: &sdkmcp.ToolAnnotations{
				Title: "Capture a webpage as Markdown", ReadOnlyHint: readOnly,
				DestructiveHint: &destructive, IdempotentHint: false, OpenWorldHint: &openWorld,
			},
		}, s.handleCaptureWebpage)
	}
}

func (s *Server) handleSearchWeb(ctx context.Context, _ *sdkmcp.CallToolRequest, input searchInput) (*sdkmcp.CallToolResult, any, error) {
	request, err := search.NormalizeRequest(search.Request{
		Query: input.Query, MaxResults: input.MaxResults, Language: input.Language,
		Freshness: input.Freshness, IncludeDomains: input.IncludeDomains, ExcludeDomains: input.ExcludeDomains,
	})
	if err != nil {
		return nil, nil, err
	}
	response, err := s.search.Search(ctx, request)
	if err != nil {
		return nil, nil, err
	}
	response, err = search.NormalizeResponse(request, response)
	if err != nil {
		return nil, nil, err
	}
	encoded, _, err := search.MarshalBounded(response, searchResponseLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("encode bounded search result: %w", err)
	}
	return textCallResult(encoded), nil, nil
}

func (s *Server) handleCaptureWebpage(ctx context.Context, _ *sdkmcp.CallToolRequest, input captureWebpageInput) (*sdkmcp.CallToolResult, *captureWebpageOutput, error) {
	if err := validateCaptureInput(input); err != nil {
		return nil, nil, err
	}
	result, err := s.webFetcher.Fetch(ctx, fetch.Request{
		Method: http.MethodGet, URL: input.URL, Timeout: s.timeout, FollowRedirects: true,
		MaxRedirects: fetch.DefaultMaxRedirects, MaxBodyBytes: s.maxBodyBytes,
		UserAgent: "hitspec-web-capture", NetworkPolicy: fetch.NetworkPublicOnly,
	})
	if err != nil {
		return nil, nil, err
	}
	if !result.Success() {
		return nil, nil, fmt.Errorf("webpage returned HTTP status %d", result.StatusCode)
	}
	markdown, err := fetch.Render(ctx, result, fetch.FormatMarkdown)
	if err != nil {
		return nil, nil, err
	}
	if int64(len(markdown)) > expandedBodyLimit(s.maxBodyBytes) {
		return nil, nil, errors.New("rendered webpage exceeds the artifact size limit")
	}
	title := captureTitle(result)
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = cleanCaptureBytes(title, maximumCaptureName)
	}
	if name == "" {
		name = "webpage"
	}
	tags := captureTags(input.Tags)
	index := true
	if input.Index != nil {
		index = *input.Index
	}
	receipt, err := s.artifacts.Save(ctx, artifact.Input{
		Filename: "response.md", Content: markdown, Name: name,
		Source: fetch.SanitizeURL(result.FinalURL), Tags: tags,
		TTL: strings.TrimSpace(input.TTL), Index: index,
	})
	if err != nil {
		return nil, nil, err
	}
	requestedURL := fetch.SanitizeURL(result.RequestedURL)
	if requestedURL == "" {
		requestedURL = fetch.SanitizeURL(input.URL)
	}
	finalURL := fetch.SanitizeURL(result.FinalURL)
	if finalURL == "" {
		finalURL = requestedURL
	}
	output := &captureWebpageOutput{
		URL: requestedURL, FinalURL: finalURL, Title: title,
		HTTPStatus: result.StatusCode, ContentType: cleanCaptureInline(result.ContentType, 256),
		MarkdownBytes: len(markdown),
		Stash:         captureStash(receipt, name, tags, index),
	}
	return nil, output, nil
}

func captureStash(receipt artifact.Receipt, fallbackName string, fallbackTags []string, indexRequested bool) captureStashOutput {
	name := cleanCaptureBytes(receipt.Name, maximumCaptureName)
	if name == "" {
		name = cleanCaptureBytes(fallbackName, maximumCaptureName)
	}
	tags := cleanReceiptTags(receipt.Tags)
	if len(tags) == 0 {
		tags = cleanReceiptTags(fallbackTags)
	}
	status := cleanCaptureBytes(receipt.Status, 64)
	if status == "" {
		switch {
		case receipt.Storage == artifact.StorageSucceeded && len(receipt.Failures) > 0:
			status = "saved_with_failures"
		case receipt.Storage == artifact.StorageSucceeded:
			status = "saved"
		case receipt.Storage == artifact.StorageFailed:
			status = "failed"
		default:
			status = "unknown"
		}
	}
	failureCount := len(receipt.Failures)
	if failureCount > maximumReceiptFailures {
		failureCount = maximumReceiptFailures
	}
	failed := make([]captureFailureOutput, 0, failureCount)
	for _, failure := range receipt.Failures[:failureCount] {
		failed = append(failed, captureFailureOutput{
			ID:    cleanCaptureBytes(failure.ID, 128),
			Stage: cleanCaptureBytes(failure.Stage, 64),
			Error: cleanCaptureBytes(failure.Error, maximumReceiptErrorSize),
		})
	}
	fileCount := receipt.FileCount
	if fileCount < 0 {
		fileCount = 0
	}
	totalSize := receipt.TotalSize
	if totalSize < 0 {
		totalSize = 0
	}
	return captureStashOutput{
		ID: cleanCaptureBytes(receipt.StashID, 128), Name: name, Status: status,
		CreatedAt: cleanCaptureBytes(receipt.CreatedAt, 64), ExpiresAt: cleanCaptureBytes(receipt.ExpiresAt, 64), Tags: tags,
		ContentHash: cleanCaptureBytes(receipt.ContentHash, 128),
		FileCount:   fileCount, TotalSize: totalSize,
		Indexed: receipt.Index == artifact.IndexSucceeded, IndexRequested: indexRequested,
		Failed: failed,
	}
}

func cleanReceiptTags(input []string) []string {
	result := make([]string, 0, min(len(input), maximumCaptureTags))
	seen := make(map[string]bool, min(len(input), maximumCaptureTags))
	for _, raw := range input {
		tag := cleanCaptureBytes(raw, 64)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		result = append(result, tag)
		if len(result) == maximumCaptureTags {
			break
		}
	}
	return result
}

func validateCaptureInput(input captureWebpageInput) error {
	return validateCaptureMetadata(input.Name, input.Tags, input.TTL)
}

func validateCaptureMetadata(rawName string, tags []string, ttl string) error {
	if err := artifact.ValidateTTL(ttl); err != nil {
		return err
	}
	if len(tags) > maximumCaptureTags-2 {
		return fmt.Errorf("at most %d additional capture tags are allowed", maximumCaptureTags-2)
	}
	name := strings.TrimSpace(rawName)
	if len(name) > maximumCaptureName || containsCaptureControl(name) {
		return fmt.Errorf("capture name must be one line and at most %d bytes", maximumCaptureName)
	}
	for _, tag := range tags {
		if tag == "" || len(tag) > 64 || containsCaptureControl(tag) || strings.HasPrefix(tag, "-") {
			return fmt.Errorf("invalid capture tag %q", tag)
		}
	}
	return nil
}

func containsCaptureControl(value string) bool {
	for _, char := range value {
		if unicode.IsControl(char) {
			return true
		}
	}
	return false
}

func captureTags(extra []string) []string {
	result := []string{"web", "markdown"}
	seen := map[string]bool{"web": true, "markdown": true}
	for _, tag := range extra {
		if !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}
	return result
}

func captureTitle(result *fetch.Result) string {
	if result != nil {
		if document, err := html.Parse(bytes.NewReader(result.Body)); err == nil {
			for node := document; node != nil; node = nextHTMLNode(node) {
				if node.Type == html.ElementNode && strings.EqualFold(node.Data, "title") {
					var text strings.Builder
					for child := node.FirstChild; child != nil; child = child.NextSibling {
						if child.Type == html.TextNode {
							text.WriteString(child.Data)
						}
					}
					if title := cleanCaptureInline(text.String(), maximumCaptureTitle); title != "" {
						return title
					}
				}
			}
		}
		for _, raw := range []string{result.FinalURL, result.RequestedURL} {
			if parsed, err := url.Parse(raw); err == nil && parsed.Hostname() != "" {
				return cleanCaptureInline(parsed.Hostname(), maximumCaptureTitle)
			}
		}
	}
	return "webpage"
}

func nextHTMLNode(node *html.Node) *html.Node {
	if node.FirstChild != nil {
		return node.FirstChild
	}
	for node != nil {
		if node.NextSibling != nil {
			return node.NextSibling
		}
		node = node.Parent
	}
	return nil
}

func cleanCaptureInline(value string, maximum int) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > maximum {
		value = string(runes[:maximum])
	}
	return value
}

func cleanCaptureBytes(value string, maximum int) string {
	value = cleanCaptureInline(value, maximum)
	for len(value) > maximum {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}

func textCallResult(content []byte) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(content)}}}
}
