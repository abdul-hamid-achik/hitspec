package coverage

import (
	"strings"
	"testing"
)

func TestAnalyzer_Analyze_BasicCoverage(t *testing.T) {
	analyzer := NewAnalyzer()

	// Manually set endpoints (normally loaded from OpenAPI)
	analyzer.endpoints = []Endpoint{
		{Method: "GET", Path: "/users", OperationID: "getUsers", Tags: []string{"users"}},
		{Method: "POST", Path: "/users", OperationID: "createUser", Tags: []string{"users"}},
		{Method: "GET", Path: "/users/{id}", OperationID: "getUser", Tags: []string{"users"}},
		{Method: "DELETE", Path: "/users/{id}", OperationID: "deleteUser", Tags: []string{"users"}},
	}

	requests := []ExecutedRequest{
		{Method: "GET", Path: "/users"},
		{Method: "POST", Path: "/users"},
	}

	report := analyzer.Analyze(requests)

	if report.TotalEndpoints != 4 {
		t.Errorf("expected 4 total endpoints, got %d", report.TotalEndpoints)
	}

	if report.CoveredEndpoints != 2 {
		t.Errorf("expected 2 covered endpoints, got %d", report.CoveredEndpoints)
	}

	if report.CoveragePercent != 50.0 {
		t.Errorf("expected 50%% coverage, got %.1f%%", report.CoveragePercent)
	}
}

func TestAnalyzer_Analyze_PathParameters(t *testing.T) {
	analyzer := NewAnalyzer()

	analyzer.endpoints = []Endpoint{
		{Method: "GET", Path: "/users/{id}"},
		{Method: "GET", Path: "/users/{id}/posts/{postId}"},
	}

	requests := []ExecutedRequest{
		{Method: "GET", Path: "/users/123"},
		{Method: "GET", Path: "/users/456/posts/789"},
	}

	report := analyzer.Analyze(requests)

	if report.CoveredEndpoints != 2 {
		t.Errorf("expected 2 covered endpoints, got %d", report.CoveredEndpoints)
	}

	if report.CoveragePercent != 100.0 {
		t.Errorf("expected 100%% coverage, got %.1f%%", report.CoveragePercent)
	}
}

func TestAnalyzer_Analyze_TagCoverage(t *testing.T) {
	analyzer := NewAnalyzer()

	analyzer.endpoints = []Endpoint{
		{Method: "GET", Path: "/users", Tags: []string{"users"}},
		{Method: "POST", Path: "/users", Tags: []string{"users"}},
		{Method: "GET", Path: "/posts", Tags: []string{"posts"}},
		{Method: "POST", Path: "/posts", Tags: []string{"posts"}},
	}

	requests := []ExecutedRequest{
		{Method: "GET", Path: "/users"},
		{Method: "POST", Path: "/users"},
		{Method: "GET", Path: "/posts"},
	}

	report := analyzer.Analyze(requests)

	usersReport := report.ByTag["users"]
	if usersReport == nil {
		t.Fatal("expected users tag report")
	}
	if usersReport.CoveragePercent != 100.0 {
		t.Errorf("expected 100%% users coverage, got %.1f%%", usersReport.CoveragePercent)
	}

	postsReport := report.ByTag["posts"]
	if postsReport == nil {
		t.Fatal("expected posts tag report")
	}
	if postsReport.CoveragePercent != 50.0 {
		t.Errorf("expected 50%% posts coverage, got %.1f%%", postsReport.CoveragePercent)
	}
}

func TestAnalyzer_Analyze_TestCount(t *testing.T) {
	analyzer := NewAnalyzer()

	analyzer.endpoints = []Endpoint{
		{Method: "GET", Path: "/users"},
	}

	requests := []ExecutedRequest{
		{Method: "GET", Path: "/users"},
		{Method: "GET", Path: "/users"},
		{Method: "GET", Path: "/users"},
	}

	report := analyzer.Analyze(requests)

	if len(report.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(report.Endpoints))
	}

	if report.Endpoints[0].TestCount != 3 {
		t.Errorf("expected test count 3, got %d", report.Endpoints[0].TestCount)
	}
}

func TestAnalyzer_matchEndpoint(t *testing.T) {
	analyzer := NewAnalyzer()

	tests := []struct {
		request  ExecutedRequest
		endpoint Endpoint
		expected bool
	}{
		{
			request:  ExecutedRequest{Method: "GET", Path: "/users"},
			endpoint: Endpoint{Method: "GET", Path: "/users"},
			expected: true,
		},
		{
			request:  ExecutedRequest{Method: "POST", Path: "/users"},
			endpoint: Endpoint{Method: "GET", Path: "/users"},
			expected: false,
		},
		{
			request:  ExecutedRequest{Method: "GET", Path: "/users/123"},
			endpoint: Endpoint{Method: "GET", Path: "/users/{id}"},
			expected: true,
		},
		{
			request:  ExecutedRequest{Method: "GET", Path: "/users/123/posts/456"},
			endpoint: Endpoint{Method: "GET", Path: "/users/{userId}/posts/{postId}"},
			expected: true,
		},
		{
			request:  ExecutedRequest{Method: "GET", Path: "/users/123"},
			endpoint: Endpoint{Method: "GET", Path: "/users"},
			expected: false,
		},
		{
			request:  ExecutedRequest{Method: "GET", Path: "/users"},
			endpoint: Endpoint{Method: "GET", Path: "/users/{id}"},
			expected: false,
		},
	}

	for _, tt := range tests {
		result := analyzer.matchEndpoint(tt.request, tt.endpoint)
		if result != tt.expected {
			t.Errorf("matchEndpoint(%v, %v): got %v, expected %v",
				tt.request, tt.endpoint, result, tt.expected)
		}
	}
}

func TestReport_FormatConsole(t *testing.T) {
	report := &Report{
		TotalEndpoints:   4,
		CoveredEndpoints: 2,
		CoveragePercent:  50.0,
		Endpoints: []EndpointStatus{
			{Method: "GET", Path: "/users", Covered: true, TestCount: 1},
			{Method: "POST", Path: "/users", Covered: true, TestCount: 1},
			{Method: "GET", Path: "/users/{id}", Covered: false, TestCount: 0},
			{Method: "DELETE", Path: "/users/{id}", Covered: false, TestCount: 0},
		},
	}

	output := report.FormatConsole()

	if !strings.Contains(output, "50.0%") {
		t.Error("expected console output to contain coverage percentage")
	}

	if !strings.Contains(output, "[x] GET /users") {
		t.Error("expected console output to show covered endpoint")
	}

	if !strings.Contains(output, "[ ] GET /users/{id}") {
		t.Error("expected console output to show uncovered endpoint")
	}
}

func TestReport_FormatJSON(t *testing.T) {
	report := &Report{
		TotalEndpoints:   2,
		CoveredEndpoints: 1,
		CoveragePercent:  50.0,
		Endpoints: []EndpointStatus{
			{Method: "GET", Path: "/users", Covered: true, TestCount: 1},
		},
	}

	output, err := report.FormatJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, `"coveragePercent": 50`) {
		t.Error("expected JSON to contain coverage percentage")
	}

	if !strings.Contains(output, `"method": "GET"`) {
		t.Error("expected JSON to contain method")
	}
}

func TestReport_FormatHTML(t *testing.T) {
	report := &Report{
		TotalEndpoints:   2,
		CoveredEndpoints: 1,
		CoveragePercent:  50.0,
		Endpoints: []EndpointStatus{
			{Method: "GET", Path: "/users", Covered: true, Tags: []string{"users"}, TestCount: 1},
		},
	}

	output := report.FormatHTML()

	if !strings.Contains(output, "<!DOCTYPE html>") {
		t.Error("expected HTML to start with DOCTYPE")
	}

	if !strings.Contains(output, "50.0%") {
		t.Error("expected HTML to contain coverage percentage")
	}

	if !strings.Contains(output, `<span class="tag">users</span>`) {
		t.Error("expected HTML to contain tag")
	}
}
