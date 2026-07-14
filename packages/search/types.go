package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	DefaultMaxResults    = 5
	MaximumResults       = 10
	MaximumQueryRunes    = 512
	MaximumDomains       = 20
	MaximumSnippet       = 1024
	MaximumURLBytes      = 4096
	MaximumResponseBytes = 64 << 10
	maximumCandidates    = 100

	// DiscoveryKind identifies search responses as candidates rather than
	// fetched, verified, or durable evidence.
	DiscoveryKind = "discovery"
)

// Request is the provider-neutral, bounded web discovery contract.
type Request struct {
	Query          string   `json:"query"`
	MaxResults     int      `json:"max_results,omitempty"`
	Language       string   `json:"language,omitempty"`
	Freshness      string   `json:"freshness,omitempty"`
	IncludeDomains []string `json:"include_domains,omitempty"`
	ExcludeDomains []string `json:"exclude_domains,omitempty"`
}

// Result is one discovery candidate. It is not verified evidence.
type Result struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Domain      string `json:"domain"`
	Snippet     string `json:"snippet"`
	PublishedAt string `json:"published_at,omitempty"`
	CitationID  string `json:"citation_id"`
}

// Response is the provider-neutral Hitspec discovery response.
type Response struct {
	Kind      string   `json:"kind"`
	Query     string   `json:"query"`
	Results   []Result `json:"results"`
	Truncated bool     `json:"truncated"`
}

// Provider discovers public URLs without fetching or persisting their bodies.
type Provider interface {
	Name() string
	Search(context.Context, Request) (Response, error)
}

// NormalizeRequest validates and fills provider-neutral defaults.
func NormalizeRequest(input Request) (Request, error) {
	input.Query = strings.TrimSpace(input.Query)
	if input.Query == "" {
		return Request{}, errors.New("search query is required")
	}
	if utf8.RuneCountInString(input.Query) > MaximumQueryRunes {
		return Request{}, fmt.Errorf("search query exceeds %d characters", MaximumQueryRunes)
	}
	if strings.ContainsRune(input.Query, '\x00') {
		return Request{}, errors.New("search query contains a NUL byte")
	}
	input.Query = cleanInline(input.Query, MaximumQueryRunes)
	if input.MaxResults == 0 {
		input.MaxResults = DefaultMaxResults
	}
	if input.MaxResults < 1 || input.MaxResults > MaximumResults {
		return Request{}, fmt.Errorf("max_results must be between 1 and %d", MaximumResults)
	}

	var err error
	input.Language, err = normalizeLanguage(input.Language)
	if err != nil {
		return Request{}, err
	}
	input.Freshness = strings.ToLower(strings.TrimSpace(input.Freshness))
	if input.Freshness == "" {
		input.Freshness = "any"
	}
	switch input.Freshness {
	case "any", "day", "week", "month", "year":
	default:
		return Request{}, errors.New("freshness must be any, day, week, month, or year")
	}
	input.IncludeDomains, err = normalizeDomains(input.IncludeDomains)
	if err != nil {
		return Request{}, fmt.Errorf("include_domains: %w", err)
	}
	input.ExcludeDomains, err = normalizeDomains(input.ExcludeDomains)
	if err != nil {
		return Request{}, fmt.Errorf("exclude_domains: %w", err)
	}
	included := make(map[string]bool, len(input.IncludeDomains))
	for _, domain := range input.IncludeDomains {
		included[domain] = true
	}
	for _, domain := range input.ExcludeDomains {
		if included[domain] {
			return Request{}, fmt.Errorf("domain %q cannot be both included and excluded", domain)
		}
	}
	return input, nil
}

// NormalizeResponse projects an arbitrary provider response onto Hitspec's
// small, provider-neutral discovery contract. This boundary is applied by the
// MCP handler even for trusted in-process providers so a replacement adapter
// cannot leak metadata or bypass request limits.
func NormalizeResponse(request Request, input Response) (Response, error) {
	request, err := NormalizeRequest(request)
	if err != nil {
		return Response{}, err
	}
	response := Response{
		Kind:      DiscoveryKind,
		Query:     request.Query,
		Results:   make([]Result, 0, min(request.MaxResults, len(input.Results))),
		Truncated: input.Truncated,
	}
	candidateCount := min(len(input.Results), maximumCandidates)
	if candidateCount < len(input.Results) {
		response.Truncated = true
	}
	seen := make(map[string]bool, candidateCount)
	for _, candidate := range input.Results[:candidateCount] {
		canonical, domain, ok := canonicalURL(candidate.URL)
		if !ok || seen[canonical] || !domainAllowed(domain, request.IncludeDomains, request.ExcludeDomains) {
			response.Truncated = true
			continue
		}
		if len(response.Results) == request.MaxResults {
			response.Truncated = true
			break
		}
		seen[canonical] = true
		response.Results = append(response.Results, Result{
			Title:       cleanInline(candidate.Title, 300),
			URL:         canonical,
			Domain:      domain,
			Snippet:     cleanInline(candidate.Snippet, MaximumSnippet),
			PublishedAt: cleanInline(candidate.PublishedAt, 128),
			CitationID:  fmt.Sprintf("source-%02d", len(response.Results)+1),
		})
	}
	return response, nil
}

// MarshalResponse serializes a discovery response within the stable 64 KiB
// response ceiling. If necessary it removes results from the end, preserving
// their order, and marks the returned document as truncated.
func MarshalResponse(input Response) ([]byte, error) {
	encoded, _, err := MarshalBounded(input, MaximumResponseBytes)
	return encoded, err
}

// MarshalBounded serializes a discovery response within limit bytes and also
// returns the exact response represented by those bytes. It removes results
// only from the end, making truncation deterministic for callers that need to
// persist or otherwise reuse the bounded response.
func MarshalBounded(input Response, limit int) ([]byte, Response, error) {
	if limit <= 0 {
		return nil, Response{}, errors.New("search response limit must be positive")
	}
	response := input
	response.Kind = DiscoveryKind
	resultCount := min(len(input.Results), MaximumResults)
	response.Results = append([]Result(nil), input.Results[:resultCount]...)
	if resultCount < len(input.Results) {
		response.Truncated = true
	}
	if response.Results == nil {
		response.Results = []Result{}
	}
	for {
		encoded, err := json.Marshal(response)
		if err != nil {
			return nil, Response{}, fmt.Errorf("encode search response: %w", err)
		}
		if len(encoded) <= limit {
			return encoded, response, nil
		}
		if len(response.Results) == 0 {
			return nil, Response{}, fmt.Errorf("search response exceeds %d bytes without results", limit)
		}
		response.Results = response.Results[:len(response.Results)-1]
		response.Truncated = true
	}
}

func domainAllowed(domain string, included, excluded []string) bool {
	for _, blocked := range excluded {
		if domain == blocked || strings.HasSuffix(domain, "."+blocked) {
			return false
		}
	}
	if len(included) == 0 {
		return true
	}
	for _, allowed := range included {
		if domain == allowed || strings.HasSuffix(domain, "."+allowed) {
			return true
		}
	}
	return false
}

func normalizeLanguage(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len(value) > 63 || strings.Contains(value, "_") {
		return "", errors.New("language must be a valid BCP 47 language tag")
	}
	parts := strings.Split(value, "-")
	if len(parts) == 0 || len(parts[0]) < 2 || len(parts[0]) > 3 || !asciiLetters(parts[0]) {
		return "", errors.New("language must be a valid BCP 47 language tag")
	}
	parts[0] = strings.ToLower(parts[0])
	index := 1
	if index < len(parts) && len(parts[index]) == 4 && asciiLetters(parts[index]) {
		parts[index] = strings.ToUpper(parts[index][:1]) + strings.ToLower(parts[index][1:])
		index++
	}
	if index < len(parts) && ((len(parts[index]) == 2 && asciiLetters(parts[index])) || (len(parts[index]) == 3 && asciiDigits(parts[index]))) {
		parts[index] = strings.ToUpper(parts[index])
		index++
	}
	for index < len(parts) {
		part := parts[index]
		if part == "" || len(part) > 8 || !asciiAlphaNumeric(part) {
			return "", errors.New("language must be a valid BCP 47 language tag")
		}
		if len(part) == 1 {
			// Extension singletons require at least one following subtag. Private
			// use accepts 1-8 characters; other extensions accept 2-8.
			privateUse := strings.EqualFold(part, "x")
			parts[index] = strings.ToLower(part)
			index++
			if index == len(parts) {
				return "", errors.New("language must be a valid BCP 47 language tag")
			}
			for index < len(parts) && len(parts[index]) != 1 {
				minimum := 2
				if privateUse {
					minimum = 1
				}
				if len(parts[index]) < minimum || len(parts[index]) > 8 || !asciiAlphaNumeric(parts[index]) {
					return "", errors.New("language must be a valid BCP 47 language tag")
				}
				parts[index] = strings.ToLower(parts[index])
				index++
			}
			continue
		}
		validVariant := (len(part) >= 5 && len(part) <= 8) || (len(part) == 4 && part[0] >= '0' && part[0] <= '9')
		if !validVariant {
			return "", errors.New("language must be a valid BCP 47 language tag")
		}
		parts[index] = strings.ToLower(part)
		index++
	}
	return strings.Join(parts, "-"), nil
}

func asciiLetters(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') {
			return false
		}
	}
	return true
}

func asciiDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func asciiAlphaNumeric(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}

func normalizeDomains(values []string) ([]string, error) {
	if len(values) > MaximumDomains {
		return nil, fmt.Errorf("at most %d domains are allowed", MaximumDomains)
	}
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.TrimSuffix(value, ".")
		if value == "" || len(value) > 253 || strings.ContainsAny(value, "/:@?#[]") || net.ParseIP(value) != nil {
			return nil, fmt.Errorf("invalid public domain %q", value)
		}
		parsed, err := url.Parse("https://" + value)
		if err != nil || parsed.Hostname() != value || !strings.Contains(value, ".") {
			return nil, fmt.Errorf("invalid public domain %q", value)
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result, nil
}

func cleanInline(value string, max int) string {
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > max {
		value = string(runes[:max])
	}
	return value
}

func canonicalURL(raw string) (string, string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) == 0 || len(raw) > MaximumURLBytes {
		return "", "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return "", "", false
	}
	parsed.Fragment = ""
	hostname := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if hostname == "" {
		return "", "", false
	}
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return "", "", false
	}
	if address := net.ParseIP(hostname); address != nil && (!address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast()) {
		return "", "", false
	}
	query := parsed.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "gclid" || lower == "fbclid" || lower == "mc_cid" || lower == "mc_eid" {
			query.Del(key)
		}
	}
	port := parsed.Port()
	if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		port = ""
	}
	parsed.Host = hostname
	if port != "" {
		parsed.Host = net.JoinHostPort(hostname, port)
	}
	parsed.RawQuery = query.Encode()
	canonical := parsed.String()
	if len(canonical) > MaximumURLBytes {
		return "", "", false
	}
	return canonical, hostname, true
}
