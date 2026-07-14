package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/artifact"
	"github.com/abdul-hamid-achik/hitspec/packages/fetch"
	"github.com/abdul-hamid-achik/hitspec/packages/search"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeSearchProvider struct {
	request  search.Request
	response *search.Response
	err      error
	calls    int
}

func (p *fakeSearchProvider) Name() string { return "fake" }

func (p *fakeSearchProvider) Search(_ context.Context, request search.Request) (search.Response, error) {
	p.calls++
	p.request = request
	if p.err != nil {
		return search.Response{}, p.err
	}
	if p.response != nil {
		return *p.response, nil
	}
	return search.Response{Kind: search.DiscoveryKind, Query: request.Query, Results: []search.Result{{
		CitationID: "source-01", Title: "Example", URL: "https://example.com/",
		Domain: "example.com", Snippet: "snippet", PublishedAt: "2026-07-13T00:00:00Z",
	}}}, nil
}

func TestSearchRejectsInvalidStableInputBeforeProviderCall(t *testing.T) {
	provider := &fakeSearchProvider{}
	server, err := NewServer("test", t.TempDir(), Options{SearchProvider: provider})
	if err != nil {
		t.Fatal(err)
	}
	result, err := connect(t, server).CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "hitspec_search_web",
		Arguments: map[string]any{
			"query": "test", "max_results": 11,
		},
	})
	if err != nil || !result.IsError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if provider.calls != 0 {
		t.Fatalf("invalid stable input made %d provider calls", provider.calls)
	}
}

type fakeArtifactSink struct {
	input   artifact.Input
	receipt *artifact.Receipt
	err     error
	calls   int
}

func (s *fakeArtifactSink) Save(_ context.Context, input artifact.Input) (artifact.Receipt, error) {
	s.calls++
	s.input = input
	if s.err != nil {
		return artifact.Receipt{Store: "fcheap", Storage: artifact.StorageUnknown}, s.err
	}
	if s.receipt != nil {
		return *s.receipt, nil
	}
	indexRequested := input.Index
	index := artifact.IndexSkipped
	if input.Index {
		index = artifact.IndexSucceeded
	}
	return artifact.Receipt{
		Store: "fcheap", OperationID: "operation-1", Storage: artifact.StorageSucceeded,
		Index: index, StashID: "stash-1", ContentHash: "abc", Name: input.Name,
		Status: "saved", CreatedAt: "2026-07-13T00:00:00Z", Tags: append([]string(nil), input.Tags...),
		IndexRequested: &indexRequested, FileCount: 1, TotalSize: int64(len(input.Content)),
	}, nil
}

type fakeWebFetcher struct {
	request fetch.Request
	result  *fetch.Result
	err     error
	calls   int
}

func (f *fakeWebFetcher) Fetch(_ context.Context, request fetch.Request) (*fetch.Result, error) {
	f.calls++
	f.request = request
	return f.result, f.err
}

func TestWebToolsExposeStableSurfaceAndSchemas(t *testing.T) {
	provider, sink := &fakeSearchProvider{}, &fakeArtifactSink{}
	server, err := NewServer("test", t.TempDir(), Options{SearchProvider: provider, ArtifactSink: sink})
	if err != nil {
		t.Fatal(err)
	}
	session := connect(t, server)
	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 5 {
		t.Fatalf("tools = %d, want 5", len(listed.Tools))
	}
	byName := make(map[string]*sdkmcp.Tool, len(listed.Tools))
	for _, tool := range listed.Tools {
		byName[tool.Name] = tool
	}
	for _, name := range []string{"hitspec_fetch", "hitspec_search_web", "hitspec_capture_webpage", "hitspec_list_requests", "hitspec_validate"} {
		if byName[name] == nil {
			t.Errorf("tool %q is missing", name)
		}
	}
	if byName["hitspec_capture_search"] != nil {
		t.Fatal("hitspec_capture_search must not be model-visible")
	}
	searchTool := byName["hitspec_search_web"]
	if searchTool.Annotations == nil || !searchTool.Annotations.ReadOnlyHint {
		t.Fatalf("search annotations = %#v", searchTool.Annotations)
	}
	assertSchemaProperties(t, searchTool.InputSchema,
		[]string{"query", "max_results", "language", "freshness", "include_domains", "exclude_domains"},
		[]string{"topic", "time_range", "country", "provider", "endpoint", "api_key", "headers"},
	)
	captureTool := byName["hitspec_capture_webpage"]
	assertExactSchema(t, captureTool.InputSchema, `{"additionalProperties":false,"properties":{"index":{"description":"index the Markdown immediately for fcheap_search; defaults to true","type":["null","boolean"]},"name":{"description":"display name for the file.cheap stash; defaults to the HTML title","type":"string"},"tags":{"description":"extra file.cheap tags; web and markdown are always included","items":{"type":"string"},"type":["null","array"]},"ttl":{"description":"optional file.cheap retention such as 24h, 7d, 30d, or 2026-12-31; omitted uses file.cheap policy","type":"string"},"url":{"description":"required,absolute http(s) URL of the public static webpage to fetch","type":"string"}},"required":["url"],"type":"object"}`)
	assertExactSchema(t, captureTool.OutputSchema, `{"additionalProperties":false,"properties":{"content_type":{"type":"string"},"final_url":{"type":"string"},"http_status":{"type":"integer"},"markdown_bytes":{"type":"integer"},"stash":{"additionalProperties":false,"properties":{"content_hash":{"type":"string"},"created_at":{"type":"string"},"expires_at":{"type":"string"},"failed":{"items":{"additionalProperties":false,"properties":{"error":{"type":"string"},"id":{"type":"string"},"stage":{"type":"string"}},"required":["id","stage","error"],"type":"object"},"type":["null","array"]},"file_count":{"type":"integer"},"id":{"type":"string"},"index_requested":{"type":"boolean"},"indexed":{"type":"boolean"},"name":{"type":"string"},"status":{"type":"string"},"tags":{"items":{"type":"string"},"type":["null","array"]},"total_size":{"type":"integer"}},"required":["id","status","file_count","total_size","indexed","index_requested"],"type":"object"},"title":{"type":"string"},"url":{"type":"string"}},"required":["url","final_url","title","http_status","content_type","markdown_bytes","stash"],"type":"object"}`)
	if captureTool.Annotations == nil || captureTool.Annotations.ReadOnlyHint || captureTool.Annotations.IdempotentHint {
		t.Fatalf("capture annotations = %#v", captureTool.Annotations)
	}
	assertSchemaProperties(t, captureTool.InputSchema,
		[]string{"url", "name", "tags", "ttl", "index"}, nil,
	)
	assertSchemaProperties(t, captureTool.OutputSchema,
		[]string{"url", "final_url", "title", "http_status", "content_type", "markdown_bytes", "stash"},
		[]string{"content", "artifact", "requested_url", "status_code"},
	)
	assertNestedSchemaProperties(t, captureTool.OutputSchema, "stash",
		[]string{"id", "name", "status", "created_at", "expires_at", "tags", "content_hash", "file_count", "total_size", "indexed", "index_requested", "failed"},
	)
	assertSchemaProperties(t, byName["hitspec_fetch"].InputSchema, nil, []string{"artifact"})
}

func TestSearchReturnsBoundedDiscoveryWithoutPersistence(t *testing.T) {
	provider, sink := &fakeSearchProvider{}, &fakeArtifactSink{}
	server, err := NewServer("test", t.TempDir(), Options{SearchProvider: provider, ArtifactSink: sink})
	if err != nil {
		t.Fatal(err)
	}
	searched, err := connect(t, server).CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "hitspec_search_web",
		Arguments: map[string]any{
			"query": "current docs", "max_results": 3, "language": "es-MX",
			"freshness": "week", "include_domains": []string{"example.com"},
		},
	})
	if err != nil || searched.IsError {
		t.Fatalf("search=%#v err=%v", searched, err)
	}
	if sink.calls != 0 {
		t.Fatalf("discovery persisted %d artifacts", sink.calls)
	}
	if provider.request.Query != "current docs" || provider.request.Language != "es-MX" || provider.request.Freshness != "week" {
		t.Fatalf("provider request = %#v", provider.request)
	}
	text := searched.Content[0].(*sdkmcp.TextContent).Text
	if len(text) > searchResponseLimit {
		t.Fatalf("search response = %d bytes", len(text))
	}
	var response search.Response
	if err := json.Unmarshal([]byte(text), &response); err != nil {
		t.Fatal(err)
	}
	if response.Kind != search.DiscoveryKind || response.Query != "current docs" || len(response.Results) != 1 || response.Results[0].CitationID != "source-01" {
		t.Fatalf("response = %#v", response)
	}
	for _, forbidden := range []string{"schema_version", "provider", "outcome", "retrieved_at", "rank", "request_id", "score", "duration_ms", "api_key"} {
		if strings.Contains(text, `"`+forbidden+`"`) {
			t.Errorf("provider field %q leaked in %s", forbidden, text)
		}
	}
}

func TestSearchProjectsUntrustedProviderOutputAtTheHandlerBoundary(t *testing.T) {
	results := []search.Result{
		{Title: " One\nTitle ", URL: "https://sub.example.com/a?utm_source=provider#fragment", Domain: "forged.invalid", Snippet: strings.Repeat("s", search.MaximumSnippet+100), CitationID: "provider-id"},
		{Title: "duplicate", URL: "https://sub.example.com/a?utm_source=duplicate"},
		{Title: "outside filter", URL: "https://other.example.net/outside"},
		{Title: "Two", URL: "https://example.com/b"},
		{Title: "Three", URL: "https://example.com/c"},
		{Title: "Four", URL: "https://example.com/d"},
	}
	provider := &fakeSearchProvider{response: &search.Response{Kind: "provider-kind", Query: "provider-query", Results: results}}
	server, err := NewServer("test", t.TempDir(), Options{SearchProvider: provider})
	if err != nil {
		t.Fatal(err)
	}
	searched, err := connect(t, server).CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "hitspec_search_web",
		Arguments: map[string]any{
			"query": " stable query ", "max_results": 3, "include_domains": []string{"example.com"},
		},
	})
	if err != nil || searched.IsError {
		t.Fatalf("search=%#v err=%v", searched, err)
	}
	text := searched.Content[0].(*sdkmcp.TextContent).Text
	var response search.Response
	if err := json.Unmarshal([]byte(text), &response); err != nil {
		t.Fatal(err)
	}
	if response.Kind != search.DiscoveryKind || response.Query != "stable query" || !response.Truncated || len(response.Results) != 3 {
		t.Fatalf("response = %#v", response)
	}
	first := response.Results[0]
	if first.Title != "One Title" || first.URL != "https://sub.example.com/a" || first.Domain != "sub.example.com" || first.CitationID != "source-01" || len(first.Snippet) != search.MaximumSnippet {
		t.Fatalf("first result = %#v", first)
	}
	for _, result := range response.Results {
		if !strings.HasSuffix(result.Domain, "example.com") {
			t.Fatalf("provider bypassed domain filter: %#v", result)
		}
	}
}

func TestCaptureWebpagePreservesInstalledCompactContract(t *testing.T) {
	sink := &fakeArtifactSink{}
	server, err := NewServer("test", t.TempDir(), Options{ArtifactSink: sink})
	if err != nil {
		t.Fatal(err)
	}
	server.webFetcher = &fakeWebFetcher{result: &fetch.Result{
		RequestedURL: "https://example.com/docs?token=secret", FinalURL: "https://docs.example.com/guide?token=secret",
		Status: "200 OK", StatusCode: 200, ContentType: "text/html; charset=utf-8",
		Body: []byte(`<html><head><title>Hitspec Guide</title></head><body><a href="/next">Next</a></body></html>`),
	}}
	index := false
	captured, err := connect(t, server).CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "hitspec_capture_webpage",
		Arguments: map[string]any{
			"url": "https://example.com/docs?token=secret", "tags": []string{"docs"},
			"ttl": " 30d ", "index": index,
		},
	})
	if err != nil || captured.IsError {
		t.Fatalf("capture=%#v err=%v", captured, err)
	}
	if sink.calls != 1 || sink.input.Name != "Hitspec Guide" || sink.input.TTL != "30d" || sink.input.Index {
		t.Fatalf("artifact input = %#v calls=%d", sink.input, sink.calls)
	}
	if !reflect.DeepEqual(sink.input.Tags, []string{"web", "markdown", "docs"}) {
		t.Fatalf("artifact tags = %#v", sink.input.Tags)
	}
	if sink.input.Source != "https://docs.example.com/guide" || !strings.Contains(string(sink.input.Content), "https://docs.example.com/next") {
		t.Fatalf("source=%q content=%s", sink.input.Source, sink.input.Content)
	}
	text := captured.Content[0].(*sdkmcp.TextContent).Text
	if strings.Contains(text, `"content"`) || strings.Contains(text, "token=secret") {
		t.Fatalf("capture leaked full content or query: %s", text)
	}
	var output captureWebpageOutput
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		t.Fatal(err)
	}
	if output.URL != "https://example.com/docs" || output.FinalURL != "https://docs.example.com/guide" || output.Title != "Hitspec Guide" || output.HTTPStatus != 200 {
		t.Fatalf("output = %#v", output)
	}
	if output.MarkdownBytes != len(sink.input.Content) || output.Stash.ID != "stash-1" || output.Stash.ContentHash != "abc" || output.Stash.IndexRequested || output.Stash.Indexed {
		t.Fatalf("output = %#v", output)
	}
	if captured.StructuredContent == nil {
		t.Fatal("capture omitted the installed structured output contract")
	}
}

func TestCaptureStashBoundsUntrustedSinkReceipt(t *testing.T) {
	failures := make([]artifact.Failure, maximumReceiptFailures+20)
	for index := range failures {
		failures[index] = artifact.Failure{
			ID: strings.Repeat("i", 300), Stage: strings.Repeat("s", 100),
			Error: "bad\x00\n" + strings.Repeat("e", maximumReceiptErrorSize+500),
		}
	}
	tags := make([]string, maximumCaptureTags+20)
	for index := range tags {
		tags[index] = fmt.Sprintf("tag-%02d-%s", index, strings.Repeat("x", 100))
	}
	output := captureStash(artifact.Receipt{
		Storage: artifact.StorageSucceeded, Index: artifact.IndexSucceeded,
		StashID: strings.Repeat("i", 300), Name: strings.Repeat("n", 300),
		Status: strings.Repeat("s", 300), CreatedAt: strings.Repeat("c", 300),
		ExpiresAt: strings.Repeat("e", 300), ContentHash: strings.Repeat("h", 300),
		Tags: tags, Failures: failures, FileCount: -1, TotalSize: -1,
	}, "fallback", []string{"web", "markdown"}, true)
	encoded, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(output.ID) > 128 || len(output.Name) > maximumCaptureName || len(output.Status) > 64 || len(output.CreatedAt) > 64 || len(output.ExpiresAt) > 64 || len(output.ContentHash) > 128 {
		t.Fatalf("receipt strings were not bounded: %#v", output)
	}
	if len(output.Tags) != maximumCaptureTags || len(output.Failed) != maximumReceiptFailures || output.FileCount != 0 || output.TotalSize != 0 {
		t.Fatalf("receipt collections or counts were not bounded: %#v", output)
	}
	for _, failure := range output.Failed {
		if len(failure.ID) > 128 || len(failure.Stage) > 64 || len(failure.Error) > maximumReceiptErrorSize || strings.ContainsRune(failure.Error, '\x00') || strings.ContainsRune(failure.Error, '\n') {
			t.Fatalf("failure was not sanitized: %#v", failure)
		}
	}
	if len(encoded) > 32<<10 {
		t.Fatalf("compact receipt unexpectedly grew to %d bytes", len(encoded))
	}
}

func TestCaptureDefaultsIndexAndRejectsBeforeFetch(t *testing.T) {
	sink := &fakeArtifactSink{}
	fetcher := &fakeWebFetcher{result: &fetch.Result{
		RequestedURL: "https://example.com/", FinalURL: "https://example.com/",
		StatusCode: 200, ContentType: "text/html", Body: []byte(`<title>Example</title><p>body</p>`),
	}}
	server, err := NewServer("test", t.TempDir(), Options{ArtifactSink: sink})
	if err != nil {
		t.Fatal(err)
	}
	server.webFetcher = fetcher
	session := connect(t, server)
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "hitspec_capture_webpage", Arguments: map[string]any{"url": "https://example.com/"},
	})
	if err != nil || result.IsError || !sink.input.Index {
		t.Fatalf("result=%#v input=%#v err=%v", result, sink.input, err)
	}

	tooManyTags := make([]string, maximumCaptureTags)
	for index := range tooManyTags {
		tooManyTags[index] = fmt.Sprintf("tag-%d", index)
	}
	fetchCalls, saveCalls := fetcher.calls, sink.calls
	result, err = session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "hitspec_capture_webpage",
		Arguments: map[string]any{"url": "https://example.com/", "tags": tooManyTags},
	})
	if err != nil || !result.IsError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if fetcher.calls != fetchCalls || sink.calls != saveCalls {
		t.Fatalf("invalid input caused effects: fetch=%d/%d save=%d/%d", fetcher.calls, fetchCalls, sink.calls, saveCalls)
	}

	result, err = session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "hitspec_capture_webpage",
		Arguments: map[string]any{
			"url": "https://example.com/", "name": strings.Repeat("é", maximumCaptureName),
		},
	})
	if err != nil || !result.IsError || fetcher.calls != fetchCalls || sink.calls != saveCalls {
		t.Fatalf("oversize name result=%#v err=%v fetch=%d save=%d", result, err, fetcher.calls, sink.calls)
	}
}

func TestCapturePropagatesFetchAndStorageErrors(t *testing.T) {
	sink := &fakeArtifactSink{}
	server, err := NewServer("test", t.TempDir(), Options{ArtifactSink: sink})
	if err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeWebFetcher{err: errors.New("network unavailable")}
	server.webFetcher = fetcher
	result, err := connect(t, server).CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "hitspec_capture_webpage", Arguments: map[string]any{"url": "https://example.com/"},
	})
	if err != nil || !result.IsError || sink.calls != 0 {
		t.Fatalf("result=%#v err=%v calls=%d", result, err, sink.calls)
	}
}

func TestCaptureRejectsOversizeRenderedArtifactBeforeSave(t *testing.T) {
	sink := &fakeArtifactSink{}
	server, err := NewServer("test", t.TempDir(), Options{ArtifactSink: sink, MaxBodyBytes: 16})
	if err != nil {
		t.Fatal(err)
	}
	server.webFetcher = &fakeWebFetcher{result: &fetch.Result{
		RequestedURL: "https://example.com/", FinalURL: "https://example.com/",
		StatusCode: 200, ContentType: "text/html",
		Body: []byte(strings.Repeat(`<a href="/next">Next</a>`, 512)),
	}}
	result, err := connect(t, server).CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "hitspec_capture_webpage", Arguments: map[string]any{"url": "https://example.com/"},
	})
	if err != nil || !result.IsError || sink.calls != 0 {
		t.Fatalf("result=%#v err=%v save_calls=%d", result, err, sink.calls)
	}
}

func assertSchemaProperties(t *testing.T, raw any, required, forbidden []string) {
	t.Helper()
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(encoded, &schema); err != nil {
		t.Fatal(err)
	}
	for _, name := range required {
		if _, ok := schema.Properties[name]; !ok {
			t.Errorf("schema is missing property %q: %s", name, encoded)
		}
	}
	for _, name := range forbidden {
		if _, ok := schema.Properties[name]; ok {
			t.Errorf("schema unexpectedly exposes property %q: %s", name, encoded)
		}
	}
}

func assertExactSchema(t *testing.T, raw any, expected string) {
	t.Helper()
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != expected {
		t.Fatalf("schema changed\n got: %s\nwant: %s", encoded, expected)
	}
}

func assertNestedSchemaProperties(t *testing.T, raw any, parent string, required []string) {
	t.Helper()
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(encoded, &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema.Properties[parent].Properties
	for _, name := range required {
		if _, ok := properties[name]; !ok {
			t.Errorf("schema property %q is missing nested property %q: %s", parent, name, encoded)
		}
	}
}
