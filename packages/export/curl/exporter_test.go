package curl

import (
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

func TestExport_BasicGET(t *testing.T) {
	e := New()
	req := &parser.Request{
		Method: "GET",
		URL:    "https://api.example.com/users",
	}

	result := e.Export(req)

	if !strings.Contains(result, "curl") {
		t.Error("expected curl command")
	}
	if !strings.Contains(result, "-X GET") {
		t.Error("expected GET method")
	}
	if !strings.Contains(result, "'https://api.example.com/users'") {
		t.Error("expected URL")
	}
}

func TestExport_POSTWithJSONBody(t *testing.T) {
	e := New()
	req := &parser.Request{
		Method: "POST",
		URL:    "https://api.example.com/users",
		Headers: []*parser.Header{
			{Key: "Content-Type", Value: "application/json"},
		},
		Body: &parser.Body{
			ContentType: parser.BodyJSON,
			Raw:         `{"name": "John", "email": "john@example.com"}`,
		},
	}

	result := e.Export(req)

	if !strings.Contains(result, "-X POST") {
		t.Error("expected POST method")
	}
	if !strings.Contains(result, "-H 'Content-Type: application/json'") {
		t.Error("expected Content-Type header")
	}
	if !strings.Contains(result, `-d '{"name": "John", "email": "john@example.com"}'`) {
		t.Error("expected JSON body")
	}
}

func TestExport_WithHeaders(t *testing.T) {
	e := New()
	req := &parser.Request{
		Method: "GET",
		URL:    "https://api.example.com/users",
		Headers: []*parser.Header{
			{Key: "Accept", Value: "application/json"},
			{Key: "X-Custom-Header", Value: "custom-value"},
		},
	}

	result := e.Export(req)

	if !strings.Contains(result, "-H 'Accept: application/json'") {
		t.Error("expected Accept header")
	}
	if !strings.Contains(result, "-H 'X-Custom-Header: custom-value'") {
		t.Error("expected X-Custom-Header")
	}
}

func TestExport_AuthBearer(t *testing.T) {
	e := New()
	req := &parser.Request{
		Method: "GET",
		URL:    "https://api.example.com/profile",
		Metadata: &parser.RequestMetadata{
			Auth: &parser.AuthConfig{
				Type:   parser.AuthBearer,
				Params: []string{"token123"},
			},
		},
	}

	result := e.Export(req)

	if !strings.Contains(result, "-H 'Authorization: Bearer token123'") {
		t.Error("expected Bearer auth header")
	}
}

func TestExport_AuthBasic(t *testing.T) {
	e := New()
	req := &parser.Request{
		Method: "GET",
		URL:    "https://api.example.com/secure",
		Metadata: &parser.RequestMetadata{
			Auth: &parser.AuthConfig{
				Type:   parser.AuthBasic,
				Params: []string{"user", "pass"},
			},
		},
	}

	result := e.Export(req)

	if !strings.Contains(result, "-u 'user:pass'") {
		t.Error("expected basic auth flag")
	}
}

func TestExport_AuthAPIKey(t *testing.T) {
	e := New()
	req := &parser.Request{
		Method: "GET",
		URL:    "https://api.example.com/data",
		Metadata: &parser.RequestMetadata{
			Auth: &parser.AuthConfig{
				Type:   parser.AuthAPIKey,
				Params: []string{"X-API-Key", "key123"},
			},
		},
	}

	result := e.Export(req)

	if !strings.Contains(result, "-H 'X-API-Key: key123'") {
		t.Error("expected API key header")
	}
}

func TestExport_AuthDigest(t *testing.T) {
	e := New()
	req := &parser.Request{
		Method: "GET",
		URL:    "https://api.example.com/digest",
		Metadata: &parser.RequestMetadata{
			Auth: &parser.AuthConfig{
				Type:   parser.AuthDigest,
				Params: []string{"user", "pass"},
			},
		},
	}

	result := e.Export(req)

	if !strings.Contains(result, "--digest") {
		t.Error("expected --digest flag")
	}
	if !strings.Contains(result, "-u 'user:pass'") {
		t.Error("expected user:pass")
	}
}

func TestExport_QueryParams(t *testing.T) {
	e := New()
	req := &parser.Request{
		Method: "GET",
		URL:    "https://api.example.com/search",
		QueryParams: []*parser.QueryParam{
			{Key: "q", Value: "hello world"},
			{Key: "page", Value: "1"},
		},
	}

	result := e.Export(req)

	if !strings.Contains(result, "q=hello") || !strings.Contains(result, "page=1") {
		t.Errorf("expected query params in URL, got: %s", result)
	}
}

func TestExport_MultipartBody(t *testing.T) {
	e := New()
	req := &parser.Request{
		Method: "POST",
		URL:    "https://api.example.com/upload",
		Body: &parser.Body{
			ContentType: parser.BodyMultipart,
			Multipart: []*parser.MultipartField{
				{Type: parser.MultipartFieldValue, Name: "name", Value: "John"},
				{Type: parser.MultipartFieldFile, Name: "file", Path: "/path/to/file.txt"},
			},
		},
	}

	result := e.Export(req)

	if !strings.Contains(result, "-F 'name=John'") {
		t.Error("expected form field")
	}
	if !strings.Contains(result, "-F 'file=@/path/to/file.txt'") {
		t.Error("expected file field")
	}
}

func TestExport_FormBody(t *testing.T) {
	e := New()
	req := &parser.Request{
		Method: "POST",
		URL:    "https://api.example.com/login",
		Body: &parser.Body{
			ContentType: parser.BodyForm,
			Raw:         "username=john&password=secret",
		},
	}

	result := e.Export(req)

	if !strings.Contains(result, "--data-urlencode") {
		t.Error("expected --data-urlencode flag")
	}
}

func TestExport_VerboseFlag(t *testing.T) {
	e := New(WithVerbose(true))
	req := &parser.Request{
		Method: "GET",
		URL:    "https://api.example.com/users",
	}

	result := e.Export(req)

	if !strings.Contains(result, "-v") {
		t.Error("expected verbose flag")
	}
}

func TestExport_WithResolver(t *testing.T) {
	resolver := func(s string) string {
		s = strings.ReplaceAll(s, "{{baseUrl}}", "https://api.example.com")
		s = strings.ReplaceAll(s, "{{token}}", "resolved-token")
		return s
	}

	e := New(WithResolver(resolver))
	req := &parser.Request{
		Method: "GET",
		URL:    "{{baseUrl}}/users",
		Metadata: &parser.RequestMetadata{
			Auth: &parser.AuthConfig{
				Type:   parser.AuthBearer,
				Params: []string{"{{token}}"},
			},
		},
	}

	result := e.Export(req)

	if !strings.Contains(result, "https://api.example.com/users") {
		t.Error("expected resolved baseUrl")
	}
	if !strings.Contains(result, "Bearer resolved-token") {
		t.Error("expected resolved token")
	}
}

func TestExportAll(t *testing.T) {
	e := New()
	reqs := []*parser.Request{
		{Method: "GET", URL: "https://api.example.com/users"},
		{Method: "POST", URL: "https://api.example.com/users"},
	}

	results := e.ExportAll(reqs)

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if !strings.Contains(results[0], "-X GET") {
		t.Error("expected GET in first result")
	}
	if !strings.Contains(results[1], "-X POST") {
		t.Error("expected POST in second result")
	}
}

func TestExportFormatted(t *testing.T) {
	e := New()
	reqs := []*parser.Request{
		{Name: "GetUsers", Method: "GET", URL: "https://api.example.com/users"},
		{Name: "CreateUser", Method: "POST", URL: "https://api.example.com/users"},
	}

	result := e.ExportFormatted(reqs)

	if !strings.Contains(result, "# Request: GetUsers") {
		t.Error("expected GetUsers comment")
	}
	if !strings.Contains(result, "# Request: CreateUser") {
		t.Error("expected CreateUser comment")
	}
}

func TestExport_QuoteEscaping(t *testing.T) {
	e := New()
	req := &parser.Request{
		Method: "POST",
		URL:    "https://api.example.com/users",
		Body: &parser.Body{
			ContentType: parser.BodyJSON,
			Raw:         `{"message": "it's a test"}`,
		},
	}

	result := e.Export(req)

	// Single quotes inside should be properly escaped
	if !strings.Contains(result, "'\"'\"'") {
		t.Errorf("expected escaped single quotes, got: %s", result)
	}
}

func TestExport_DefaultMethod(t *testing.T) {
	e := New()
	req := &parser.Request{
		URL: "https://api.example.com/users",
	}

	result := e.Export(req)

	if !strings.Contains(result, "-X GET") {
		t.Error("expected default GET method")
	}
}

func TestExport_GraphQL(t *testing.T) {
	e := New()
	req := &parser.Request{
		Method: "POST",
		URL:    "https://api.example.com/graphql",
		Body: &parser.Body{
			ContentType: parser.BodyGraphQL,
			GraphQL: &parser.GraphQLBody{
				Query:     "query { users { id name } }",
				Variables: `{"limit": 10}`,
			},
		},
	}

	result := e.Export(req)

	if !strings.Contains(result, "-d") {
		t.Error("expected -d flag for GraphQL body")
	}
	if !strings.Contains(result, "query") {
		t.Error("expected query in body")
	}
}
