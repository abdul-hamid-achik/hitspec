package curl

import (
	"strings"
	"testing"
)

func TestParse_SimpleGet(t *testing.T) {
	converter := NewConverter()

	parsed, err := converter.Parse(`curl https://api.example.com/users`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.Method != "GET" {
		t.Errorf("expected method GET, got %s", parsed.Method)
	}
	if parsed.URL != "https://api.example.com/users" {
		t.Errorf("expected URL https://api.example.com/users, got %s", parsed.URL)
	}
}

func TestParse_PostWithData(t *testing.T) {
	converter := NewConverter()

	parsed, err := converter.Parse(`curl -X POST https://api.example.com/users -d '{"name":"John"}'`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.Method != "POST" {
		t.Errorf("expected method POST, got %s", parsed.Method)
	}
	if parsed.Body != `{"name":"John"}` {
		t.Errorf("expected body {\"name\":\"John\"}, got %s", parsed.Body)
	}
}

func TestParse_WithHeaders(t *testing.T) {
	converter := NewConverter()

	parsed, err := converter.Parse(`curl -H "Content-Type: application/json" -H "Authorization: Bearer token123" https://api.example.com/users`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.Headers["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type: application/json, got %s", parsed.Headers["Content-Type"])
	}
	if parsed.Headers["Authorization"] != "Bearer token123" {
		t.Errorf("expected Authorization: Bearer token123, got %s", parsed.Headers["Authorization"])
	}
}

func TestParse_WithBasicAuth(t *testing.T) {
	converter := NewConverter()

	parsed, err := converter.Parse(`curl -u admin:password123 https://api.example.com/admin`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.BasicAuth != "admin:password123" {
		t.Errorf("expected basicAuth admin:password123, got %s", parsed.BasicAuth)
	}
}

func TestParse_ImplicitPost(t *testing.T) {
	converter := NewConverter()

	// Without -X, -d should imply POST
	parsed, err := converter.Parse(`curl -d "name=John" https://api.example.com/users`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.Method != "POST" {
		t.Errorf("expected implicit POST method, got %s", parsed.Method)
	}
}

func TestParse_Flags(t *testing.T) {
	converter := NewConverter()

	parsed, err := converter.Parse(`curl -k -L https://api.example.com`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !parsed.Insecure {
		t.Error("expected Insecure to be true")
	}
	if !parsed.FollowRedirects {
		t.Error("expected FollowRedirects to be true")
	}
}

func TestToHitspec(t *testing.T) {
	converter := NewConverter()

	parsed := &ParsedCurl{
		Method: "POST",
		URL:    "https://api.example.com/users",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: `{"name":"John"}`,
		Name: "create_user",
	}

	result := converter.ToHitspec(parsed)

	if !strings.Contains(result, "POST https://api.example.com/users") {
		t.Error("expected hitspec to contain method and URL")
	}
	if !strings.Contains(result, "Content-Type: application/json") {
		t.Error("expected hitspec to contain Content-Type header")
	}
	if !strings.Contains(result, `{"name":"John"}`) {
		t.Error("expected hitspec to contain body")
	}
	if !strings.Contains(result, "expect status >= 200") {
		t.Error("expected hitspec to contain default assertion")
	}
}

func TestToHitspec_NoAssertions(t *testing.T) {
	converter := NewConverter(WithAssertions(false))

	parsed := &ParsedCurl{
		Method: "GET",
		URL:    "https://api.example.com/users",
		Name:   "get_users",
	}

	result := converter.ToHitspec(parsed)

	if strings.Contains(result, ">>>") {
		t.Error("expected hitspec to NOT contain assertions when disabled")
	}
}

func TestToHitspec_BasicAuth(t *testing.T) {
	converter := NewConverter()

	parsed := &ParsedCurl{
		Method:    "GET",
		URL:       "https://api.example.com/admin",
		BasicAuth: "admin:secret",
		Name:      "admin_access",
	}

	result := converter.ToHitspec(parsed)

	if !strings.Contains(result, "# @auth basic admin secret") {
		t.Errorf("expected hitspec to contain auth annotation, got: %s", result)
	}
}

func TestConvertCommand(t *testing.T) {
	converter := NewConverter()

	result, err := converter.ConvertCommand(`curl -X POST -H "Content-Type: application/json" -d '{"name":"John"}' https://api.example.com/users`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "POST https://api.example.com/users") {
		t.Error("expected converted command to contain POST and URL")
	}
	if !strings.Contains(result, "Content-Type: application/json") {
		t.Error("expected converted command to contain header")
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    `-X POST -d "hello world"`,
			expected: []string{"-X", "POST", "-d", "hello world"},
		},
		{
			input:    `-H 'Content-Type: application/json'`,
			expected: []string{"-H", "Content-Type: application/json"},
		},
		{
			input:    `-d '{"key": "value"}'`,
			expected: []string{"-d", `{"key": "value"}`},
		},
	}

	for _, tt := range tests {
		tokens := tokenize(tt.input)
		if len(tokens) != len(tt.expected) {
			t.Errorf("tokenize(%q): got %d tokens, expected %d", tt.input, len(tokens), len(tt.expected))
			continue
		}
		for i, tok := range tokens {
			if tok != tt.expected[i] {
				t.Errorf("tokenize(%q)[%d]: got %q, expected %q", tt.input, i, tok, tt.expected[i])
			}
		}
	}
}

func TestGenerateName(t *testing.T) {
	tests := []struct {
		url    string
		method string
		expect string
	}{
		{"https://api.example.com/users", "GET", "get_users"},
		{"https://api.example.com/users/123", "GET", "get_users_123"},
		{"https://api.example.com/", "POST", "post_root"},
		{"https://api.example.com/api/v1/users", "PUT", "put_api_v1_users"},
	}

	for _, tt := range tests {
		result := generateName(tt.url, tt.method)
		if result != tt.expect {
			t.Errorf("generateName(%q, %q): got %q, expected %q", tt.url, tt.method, result, tt.expect)
		}
	}
}
