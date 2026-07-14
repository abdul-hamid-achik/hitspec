package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/fetch"
)

type fakeFetcher struct {
	result  *fetch.Result
	err     error
	request fetch.Request
	calls   int
}

func (f *fakeFetcher) Fetch(_ context.Context, request fetch.Request) (*fetch.Result, error) {
	f.calls++
	f.request = request
	return f.result, f.err
}

func TestTavilySearchNormalizesProviderResponse(t *testing.T) {
	fake := &fakeFetcher{result: &fetch.Result{StatusCode: 200, Body: []byte(`{
  "request_id":"must-not-leak","response_time":"0.12","results":[
    {"title":"One\nTitle","url":"https://example.com/page?id=7&utm_source=test#section","content":"useful snippet","published_date":"2026-07-01T12:00:00Z","score":0.9},
    {"title":"Duplicate","url":"https://example.com/page?id=7&utm_source=other","content":"duplicate","score":0.8},
    {"title":"Bad","url":"javascript:alert(1)","content":"bad","score":1}
  ]}`)}}
	provider := &Tavily{apiKey: "sentinel-key", fetcher: fake}
	response, err := provider.Search(context.Background(), Request{
		Query: " test ", MaxResults: 5, Freshness: "week",
		IncludeDomains: []string{"Example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Kind != DiscoveryKind || response.Query != "test" {
		t.Fatalf("response envelope = %#v", response)
	}
	wantResults := []Result{{
		Title: "One Title", URL: "https://example.com/page?id=7", Domain: "example.com",
		Snippet: "useful snippet", PublishedAt: "2026-07-01T12:00:00Z", CitationID: "source-01",
	}}
	if fmt.Sprintf("%#v", response.Results) != fmt.Sprintf("%#v", wantResults) {
		t.Fatalf("results = %#v, want %#v", response.Results, wantResults)
	}
	if fake.request.URL != tavilyEndpoint || fake.request.Headers.Get("Authorization") != "Bearer sentinel-key" || fake.request.NetworkPolicy != fetch.NetworkPublicOnly {
		t.Fatalf("request = %#v", fake.request)
	}
	var payload map[string]any
	if err := json.Unmarshal(fake.request.Body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["time_range"] != "week" || payload["include_answer"] != false || payload["include_raw_content"] != false || payload["safe_search"] != true {
		t.Fatalf("unsafe or incomplete tavily payload = %#v", payload)
	}
	for _, forbidden := range []string{"language", "freshness", "topic", "country", "provider", "endpoint", "api_key"} {
		if _, ok := payload[forbidden]; ok {
			t.Errorf("provider payload unexpectedly contains %q: %#v", forbidden, payload)
		}
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"schema_version", "provider", "outcome", "retrieved_at", "rank", "request_id", "score", "duration", "response_time"} {
		if strings.Contains(string(encoded), `"`+forbidden+`"`) {
			t.Errorf("provider metadata leaked in %s", encoded)
		}
	}
}

func TestTavilyMapsAnyFreshnessToNoTimeRange(t *testing.T) {
	fake := &fakeFetcher{result: &fetch.Result{StatusCode: 200, Body: []byte(`{"results":[]}`)}}
	provider := &Tavily{apiKey: "key", fetcher: fake}
	response, err := provider.Search(context.Background(), Request{Query: "test"})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(fake.request.Body, &payload); err != nil {
		t.Fatal(err)
	}
	if _, present := payload["time_range"]; present {
		t.Fatalf("any freshness must omit time_range: %#v", payload)
	}
	if response.Kind != DiscoveryKind || response.Query != "test" || response.Results == nil || len(response.Results) != 0 {
		t.Fatalf("response = %#v", response)
	}
}

func TestTavilyExplicitlyRejectsLanguage(t *testing.T) {
	fake := &fakeFetcher{result: &fetch.Result{StatusCode: 200, Body: []byte(`{"results":[]}`)}}
	provider := &Tavily{apiKey: "key", fetcher: fake}
	_, err := provider.Search(context.Background(), Request{Query: "test", Language: "es-MX"})
	if err == nil || !strings.Contains(err.Error(), "does not support the language filter") {
		t.Fatalf("error = %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("unsupported language made %d provider calls", fake.calls)
	}
}

func TestTavilyErrorsNeverContainSecretOrUpstreamBody(t *testing.T) {
	fake := &fakeFetcher{result: &fetch.Result{StatusCode: http.StatusUnauthorized, Body: []byte("sentinel-upstream-body")}}
	provider := &Tavily{apiKey: "sentinel-key", fetcher: fake}
	_, err := provider.Search(context.Background(), Request{Query: "test"})
	if err == nil || strings.Contains(err.Error(), "sentinel") {
		t.Fatalf("unsafe error = %v", err)
	}
	fake.err = errors.New("network unavailable")
	fake.result = nil
	_, err = provider.Search(context.Background(), Request{Query: "test"})
	if err == nil || strings.Contains(err.Error(), "sentinel-key") {
		t.Fatalf("secret leaked in transport error = %v", err)
	}
}

func TestNewTavilyRequiresServerOwnedKey(t *testing.T) {
	if _, err := NewTavily(" \t"); err == nil {
		t.Fatal("NewTavily unexpectedly accepted an empty key")
	}
}

func TestNormalizeRequest(t *testing.T) {
	input, err := NormalizeRequest(Request{
		Query: " hello\nworld ", Language: "ZH-hant-tw",
		IncludeDomains: []string{"Example.com", "example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if input.Query != "hello world" || input.MaxResults != DefaultMaxResults || input.Language != "zh-Hant-TW" || input.Freshness != "any" || len(input.IncludeDomains) != 1 {
		t.Fatalf("input = %#v", input)
	}

	for _, value := range []string{"en", "es-MX", "zh-Hant-TW", "de-CH-1901", "en-US-u-ca-gregory", "en-x-private"} {
		if _, err := NormalizeRequest(Request{Query: "x", Language: value}); err != nil {
			t.Errorf("valid language %q rejected: %v", value, err)
		}
	}

	twentyDomains := make([]string, MaximumDomains)
	twentyExcludedDomains := make([]string, MaximumDomains)
	for index := range twentyDomains {
		twentyDomains[index] = fmt.Sprintf("d%d.example.com", index)
		twentyExcludedDomains[index] = fmt.Sprintf("d%d.example.net", index)
	}
	if _, err := NormalizeRequest(Request{Query: "x", IncludeDomains: twentyDomains, ExcludeDomains: twentyExcludedDomains}); err != nil {
		t.Fatalf("maximum domain lists rejected: %v", err)
	}

	tooManyDomains := append(append([]string(nil), twentyDomains...), "extra.example.com")
	tests := []Request{
		{Query: ""},
		{Query: strings.Repeat("世", MaximumQueryRunes+1)},
		{Query: "x", MaxResults: -1},
		{Query: "x", MaxResults: MaximumResults + 1},
		{Query: "x", Freshness: "hour"},
		{Query: "x", Language: "english"},
		{Query: "x", Language: "en_US"},
		{Query: "x", Language: "en--US"},
		{Query: "x", Language: "en-x"},
		{Query: "x", IncludeDomains: []string{"https://example.com"}},
		{Query: "x", IncludeDomains: tooManyDomains},
		{Query: "x", IncludeDomains: []string{"example.com"}, ExcludeDomains: []string{"EXAMPLE.COM"}},
	}
	for _, request := range tests {
		if _, err := NormalizeRequest(request); err == nil {
			t.Errorf("NormalizeRequest(%#v) unexpectedly succeeded", request)
		}
	}
}

func TestNormalizeResponseEnforcesStableProviderBoundary(t *testing.T) {
	response, err := NormalizeResponse(Request{
		Query: " stable\nquery ", MaxResults: 2, IncludeDomains: []string{"example.com"},
	}, Response{Kind: "provider", Query: "wrong", Results: []Result{
		{Title: " One\nTitle ", URL: "https://sub.example.com/a?utm_source=test#fragment", Domain: "forged.invalid", Snippet: strings.Repeat("s", MaximumSnippet+100), CitationID: "provider-id"},
		{Title: "duplicate", URL: "https://sub.example.com/a?utm_source=other"},
		{Title: "outside", URL: "https://outside.example.net/"},
		{Title: "Two", URL: "https://example.com/b"},
		{Title: "Three", URL: "https://example.com/c"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if response.Kind != DiscoveryKind || response.Query != "stable query" || !response.Truncated || len(response.Results) != 2 {
		t.Fatalf("response = %#v", response)
	}
	first := response.Results[0]
	if first.Title != "One Title" || first.URL != "https://sub.example.com/a" || first.Domain != "sub.example.com" || first.CitationID != "source-01" || len(first.Snippet) != MaximumSnippet {
		t.Fatalf("first = %#v", first)
	}
	if response.Results[1].CitationID != "source-02" || response.Results[1].Domain != "example.com" {
		t.Fatalf("second = %#v", response.Results[1])
	}
}

func TestMarshalBoundedRemovesResultsFromEnd(t *testing.T) {
	input := Response{Kind: "provider-specific", Query: "bounded", Results: []Result{
		{Title: "one", URL: "https://one.example/", Domain: "one.example", Snippet: strings.Repeat("<", 1000), CitationID: "s1"},
		{Title: "two", URL: "https://two.example/", Domain: "two.example", Snippet: strings.Repeat("<", 1000), CitationID: "s2"},
		{Title: "three", URL: "https://three.example/", Domain: "three.example", Snippet: strings.Repeat("<", 1000), CitationID: "s3"},
	}}
	encoded, bounded, err := MarshalBounded(input, 7000)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 7000 || !bounded.Truncated || bounded.Kind != DiscoveryKind || len(bounded.Results) != 1 || bounded.Results[0].CitationID != "s1" {
		t.Fatalf("len=%d bounded=%#v", len(encoded), bounded)
	}
	if len(input.Results) != 3 || input.Truncated || input.Kind != "provider-specific" {
		t.Fatalf("input was mutated: %#v", input)
	}
	var decoded Response
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%#v", decoded) != fmt.Sprintf("%#v", bounded) {
		t.Fatalf("decoded=%#v bounded=%#v", decoded, bounded)
	}
}

func TestMarshalResponseNeverExceeds64KiB(t *testing.T) {
	response := Response{Query: strings.Repeat("q", MaximumQueryRunes)}
	for index := 0; index < MaximumResults; index++ {
		response.Results = append(response.Results, Result{
			Title:  strings.Repeat("<", 300),
			URL:    "https://example.com/" + strings.Repeat("a", MaximumURLBytes-20),
			Domain: "example.com", Snippet: strings.Repeat("<", MaximumSnippet),
			CitationID: fmt.Sprintf("s%d", index+1),
		})
	}
	encoded, err := MarshalResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > MaximumResponseBytes {
		t.Fatalf("encoded response = %d bytes", len(encoded))
	}
	var bounded Response
	if err := json.Unmarshal(encoded, &bounded); err != nil {
		t.Fatal(err)
	}
	if !bounded.Truncated || len(bounded.Results) >= len(response.Results) {
		t.Fatalf("response was not bounded: %#v", bounded)
	}
}

func TestMarshalBoundedRejectsImpossibleLimit(t *testing.T) {
	if _, _, err := MarshalBounded(Response{Query: "x"}, 0); err == nil {
		t.Fatal("zero limit unexpectedly succeeded")
	}
	if _, _, err := MarshalBounded(Response{Query: strings.Repeat("x", 100)}, 16); err == nil {
		t.Fatal("fixed fields unexpectedly fit an impossible limit")
	}
}

func TestCanonicalURLRejectsOversizeURL(t *testing.T) {
	if _, _, ok := canonicalURL("https://example.com/" + strings.Repeat("a", MaximumURLBytes)); ok {
		t.Fatal("oversize URL unexpectedly accepted")
	}
	canonical, domain, ok := canonicalURL("https://Example.com:443/path?utm_source=test&id=1#fragment")
	if !ok || canonical != "https://example.com/path?id=1" || domain != "example.com" {
		t.Fatalf("canonical=%q domain=%q ok=%t", canonical, domain, ok)
	}
}
