package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/fetch"
)

const (
	tavilyEndpoint       = "https://api.tavily.com/search"
	tavilyResponseLimit  = 512 << 10
	tavilyRequestTimeout = 15 * time.Second
)

type fetcher interface {
	Fetch(context.Context, fetch.Request) (*fetch.Result, error)
}

// Tavily is a provider adapter that exposes only Hitspec's bounded projection.
type Tavily struct {
	apiKey  string
	fetcher fetcher
}

// NewTavily constructs the Tavily provider. The endpoint and key are owned by
// the server and are never accepted through the provider-neutral tool input.
func NewTavily(apiKey string) (*Tavily, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("TAVILY_API_KEY is required for the tavily search provider")
	}
	return &Tavily{apiKey: apiKey, fetcher: fetch.NewService()}, nil
}

func (t *Tavily) Name() string { return "tavily" }

type tavilyRequest struct {
	Query             string   `json:"query"`
	SearchDepth       string   `json:"search_depth"`
	MaxResults        int      `json:"max_results"`
	TimeRange         string   `json:"time_range,omitempty"`
	IncludeDomains    []string `json:"include_domains,omitempty"`
	ExcludeDomains    []string `json:"exclude_domains,omitempty"`
	IncludeAnswer     bool     `json:"include_answer"`
	IncludeRawContent bool     `json:"include_raw_content"`
	IncludeImages     bool     `json:"include_images"`
	IncludeUsage      bool     `json:"include_usage"`
	SafeSearch        bool     `json:"safe_search"`
}

type tavilyResponse struct {
	Results []struct {
		Title         string `json:"title"`
		URL           string `json:"url"`
		Content       string `json:"content"`
		PublishedDate string `json:"published_date"`
	} `json:"results"`
}

func (t *Tavily) Search(ctx context.Context, input Request) (Response, error) {
	input, err := NormalizeRequest(input)
	if err != nil {
		return Response{}, err
	}
	if input.Language != "" {
		return Response{}, errors.New("tavily does not support the language filter")
	}

	timeRange := input.Freshness
	if timeRange == "any" {
		timeRange = ""
	}
	body, err := json.Marshal(tavilyRequest{
		Query: input.Query, SearchDepth: "basic", MaxResults: input.MaxResults,
		TimeRange: timeRange, IncludeDomains: input.IncludeDomains, ExcludeDomains: input.ExcludeDomains,
		IncludeAnswer: false, IncludeRawContent: false, IncludeImages: false,
		IncludeUsage: false, SafeSearch: true,
	})
	if err != nil {
		return Response{}, fmt.Errorf("encode tavily search request: %w", err)
	}
	result, err := t.fetcher.Fetch(ctx, fetch.Request{
		Method: http.MethodPost, URL: tavilyEndpoint,
		Headers: http.Header{"Authorization": {"Bearer " + t.apiKey}, "Content-Type": {"application/json"}},
		Body:    body, Timeout: tavilyRequestTimeout, FollowRedirects: false,
		MaxBodyBytes: tavilyResponseLimit, UserAgent: "hitspec-search",
		NetworkPolicy: fetch.NetworkPublicOnly,
	})
	if err != nil {
		return Response{}, fmt.Errorf("tavily search request failed: %w", err)
	}
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		return Response{}, tavilyStatusError(result.StatusCode)
	}
	var upstream tavilyResponse
	if err := json.Unmarshal(result.Body, &upstream); err != nil {
		return Response{}, errors.New("tavily returned malformed JSON")
	}
	results := make([]Result, 0, len(upstream.Results))
	for _, candidate := range upstream.Results {
		results = append(results, Result{
			Title: candidate.Title, URL: candidate.URL, Snippet: candidate.Content,
			PublishedAt: candidate.PublishedDate,
		})
	}
	response, err := NormalizeResponse(input, Response{Results: results})
	if err != nil {
		return Response{}, err
	}
	_, bounded, err := MarshalBounded(response, MaximumResponseBytes)
	if err != nil {
		return Response{}, err
	}
	return bounded, nil
}

func tavilyStatusError(status int) error {
	switch status {
	case http.StatusBadRequest:
		return errors.New("tavily rejected the bounded search request")
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.New("tavily authentication or plan authorization failed")
	case http.StatusTooManyRequests, 432, 433:
		return errors.New("tavily quota or rate limit was reached")
	default:
		if status >= 500 {
			return errors.New("tavily is temporarily unavailable")
		}
		return fmt.Errorf("tavily search failed with HTTP status %d", status)
	}
}
