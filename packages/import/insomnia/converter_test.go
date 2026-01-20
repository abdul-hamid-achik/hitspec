package insomnia

import (
	"strings"
	"testing"
)

func TestConvert_SimpleRequest(t *testing.T) {
	converter := NewConverter()

	export := `{
		"_type": "export",
		"__export_format": 4,
		"resources": [
			{
				"_id": "req_1",
				"_type": "request",
				"parentId": "wrk_1",
				"name": "Get Users",
				"method": "GET",
				"url": "https://api.example.com/users"
			}
		]
	}`

	result, err := converter.Convert([]byte(export))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "GET https://api.example.com/users") {
		t.Error("expected result to contain GET request")
	}
	if !strings.Contains(result, "### Get Users") {
		t.Error("expected result to contain request name")
	}
}

func TestConvert_RequestWithHeaders(t *testing.T) {
	converter := NewConverter()

	export := `{
		"_type": "export",
		"__export_format": 4,
		"resources": [
			{
				"_id": "req_1",
				"_type": "request",
				"parentId": "wrk_1",
				"name": "Create User",
				"method": "POST",
				"url": "https://api.example.com/users",
				"headers": [
					{"name": "Content-Type", "value": "application/json"},
					{"name": "Authorization", "value": "Bearer token123"}
				]
			}
		]
	}`

	result, err := converter.Convert([]byte(export))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Content-Type: application/json") {
		t.Error("expected result to contain Content-Type header")
	}
	if !strings.Contains(result, "Authorization: Bearer token123") {
		t.Error("expected result to contain Authorization header")
	}
}

func TestConvert_RequestWithBody(t *testing.T) {
	converter := NewConverter()

	export := `{
		"_type": "export",
		"__export_format": 4,
		"resources": [
			{
				"_id": "req_1",
				"_type": "request",
				"parentId": "wrk_1",
				"name": "Create User",
				"method": "POST",
				"url": "https://api.example.com/users",
				"body": {
					"mimeType": "application/json",
					"text": "{\"name\":\"John\"}"
				}
			}
		]
	}`

	result, err := converter.Convert([]byte(export))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, `{"name":"John"}`) {
		t.Error("expected result to contain body")
	}
}

func TestConvert_RequestWithVariables(t *testing.T) {
	converter := NewConverter()

	export := `{
		"_type": "export",
		"__export_format": 4,
		"resources": [
			{
				"_id": "req_1",
				"_type": "request",
				"parentId": "wrk_1",
				"name": "Get User",
				"method": "GET",
				"url": "{{ _.baseUrl }}/users/{{ userId }}"
			}
		]
	}`

	result, err := converter.Convert([]byte(export))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should convert {{ _.varName }} to {{varName}}
	if !strings.Contains(result, "{{baseUrl}}/users/{{userId}}") {
		t.Errorf("expected variables to be converted, got: %s", result)
	}
}

func TestConvert_RequestWithBasicAuth(t *testing.T) {
	converter := NewConverter()

	export := `{
		"_type": "export",
		"__export_format": 4,
		"resources": [
			{
				"_id": "req_1",
				"_type": "request",
				"parentId": "wrk_1",
				"name": "Admin Endpoint",
				"method": "GET",
				"url": "https://api.example.com/admin",
				"authentication": {
					"type": "basic",
					"username": "admin",
					"password": "secret123"
				}
			}
		]
	}`

	result, err := converter.Convert([]byte(export))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "# @auth basic admin secret123") {
		t.Errorf("expected basic auth annotation, got: %s", result)
	}
}

func TestConvert_RequestWithQueryParams(t *testing.T) {
	converter := NewConverter()

	export := `{
		"_type": "export",
		"__export_format": 4,
		"resources": [
			{
				"_id": "req_1",
				"_type": "request",
				"parentId": "wrk_1",
				"name": "Search Users",
				"method": "GET",
				"url": "https://api.example.com/users",
				"parameters": [
					{"name": "q", "value": "john"},
					{"name": "limit", "value": "10"}
				]
			}
		]
	}`

	result, err := converter.Convert([]byte(export))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "? q = john") {
		t.Errorf("expected query param q, got: %s", result)
	}
	if !strings.Contains(result, "? limit = 10") {
		t.Errorf("expected query param limit, got: %s", result)
	}
}

func TestConvert_RequestInFolder(t *testing.T) {
	converter := NewConverter()

	export := `{
		"_type": "export",
		"__export_format": 4,
		"resources": [
			{
				"_id": "fld_1",
				"_type": "request_group",
				"parentId": "wrk_1",
				"name": "Users"
			},
			{
				"_id": "req_1",
				"_type": "request",
				"parentId": "fld_1",
				"name": "Get Users",
				"method": "GET",
				"url": "https://api.example.com/users"
			}
		]
	}`

	result, err := converter.Convert([]byte(export))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "### Users - Get Users") {
		t.Errorf("expected folder path in request name, got: %s", result)
	}
}

func TestConvert_NoAssertions(t *testing.T) {
	converter := NewConverter(WithAssertions(false))

	export := `{
		"_type": "export",
		"__export_format": 4,
		"resources": [
			{
				"_id": "req_1",
				"_type": "request",
				"parentId": "wrk_1",
				"name": "Get Users",
				"method": "GET",
				"url": "https://api.example.com/users"
			}
		]
	}`

	result, err := converter.Convert([]byte(export))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(result, ">>>") {
		t.Error("expected no assertions when disabled")
	}
}

func TestConvertVariable(t *testing.T) {
	converter := NewConverter()

	tests := []struct {
		input    string
		expected string
	}{
		{"{{ _.baseUrl }}", "{{baseUrl}}"},
		{"{{ baseUrl }}", "{{baseUrl}}"},
		{"{{baseUrl}}", "{{baseUrl}}"},
		{"{{ _.host }}/{{ _.path }}", "{{host}}/{{path}}"},
		{"no variables", "no variables"},
	}

	for _, tt := range tests {
		result := converter.convertVariable(tt.input)
		if result != tt.expected {
			t.Errorf("convertVariable(%q): got %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Get Users", "get_users"},
		{"Create User (Admin)", "create_user_admin"},
		{"  spaces  ", "spaces"},
		{"Multiple---Dashes", "multiple_dashes"},
	}

	for _, tt := range tests {
		result := sanitizeName(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeName(%q): got %q, expected %q", tt.input, result, tt.expected)
		}
	}
}
