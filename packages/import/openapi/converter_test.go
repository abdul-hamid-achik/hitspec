package openapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// ---------------------------------------------------------------------------
// Helper: write a YAML string to a temp file and return its path.
// ---------------------------------------------------------------------------

func writeTempSpec(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp spec: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------------
// Minimal valid OpenAPI 3.0 spec used by many tests.
// ---------------------------------------------------------------------------

const minimalSpec = `openapi: "3.0.3"
info:
  title: Minimal API
  version: "1.0.0"
servers:
  - url: https://api.example.com
paths:
  /health:
    get:
      operationId: healthCheck
      summary: Health check
      responses:
        "200":
          description: OK
`

// ---------------------------------------------------------------------------
// 1. NewConverter – creates converter with defaults
// ---------------------------------------------------------------------------

func TestNewConverter_Defaults(t *testing.T) {
	c := NewConverter()
	if c == nil {
		t.Fatal("NewConverter returned nil")
	}
	if c.baseURL != "" {
		t.Errorf("expected empty baseURL, got %q", c.baseURL)
	}
	if !c.generateTests {
		t.Error("expected generateTests to be true by default")
	}
	if len(c.includeTags) != 0 {
		t.Errorf("expected empty includeTags, got %v", c.includeTags)
	}
	if len(c.excludeTags) != 0 {
		t.Errorf("expected empty excludeTags, got %v", c.excludeTags)
	}
	if len(c.includeOnly) != 0 {
		t.Errorf("expected empty includeOnly, got %v", c.includeOnly)
	}
}

// ---------------------------------------------------------------------------
// 2. WithBaseURL – overrides server URL
// ---------------------------------------------------------------------------

func TestWithBaseURL(t *testing.T) {
	c := NewConverter(WithBaseURL("http://localhost:9090"))
	if c.baseURL != "http://localhost:9090" {
		t.Errorf("expected baseURL http://localhost:9090, got %q", c.baseURL)
	}

	// The overridden URL should appear in the output
	specPath := writeTempSpec(t, "spec.yaml", minimalSpec)
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "@baseUrl = http://localhost:9090") {
		t.Errorf("output should contain overridden baseUrl, got:\n%s", out)
	}
	// Must NOT contain the spec's server URL
	if strings.Contains(out, "@baseUrl = https://api.example.com") {
		t.Error("output should not contain original server URL when overridden")
	}
}

// ---------------------------------------------------------------------------
// 3. WithTags – filters operations by tag
// ---------------------------------------------------------------------------

func TestWithTags(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Tag Test
  version: "1.0.0"
paths:
  /users:
    get:
      operationId: listUsers
      tags: [users]
      summary: List users
      responses:
        "200":
          description: OK
  /admin:
    get:
      operationId: listAdmins
      tags: [admin]
      summary: List admins
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "tags.yaml", spec)

	c := NewConverter(WithTags([]string{"users"}))
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "listUsers") {
		t.Error("expected listUsers to be included")
	}
	if strings.Contains(out, "listAdmins") {
		t.Error("expected listAdmins to be excluded")
	}
}

// ---------------------------------------------------------------------------
// 4. WithExcludeTags – excludes operations by tag
// ---------------------------------------------------------------------------

func TestWithExcludeTags(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Exclude Test
  version: "1.0.0"
paths:
  /users:
    get:
      operationId: listUsers
      tags: [users]
      summary: List users
      responses:
        "200":
          description: OK
  /internal:
    get:
      operationId: internalOp
      tags: [internal]
      summary: Internal endpoint
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "exclude.yaml", spec)

	c := NewConverter(WithExcludeTags([]string{"internal"}))
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "listUsers") {
		t.Error("expected listUsers to be included")
	}
	if strings.Contains(out, "internalOp") {
		t.Error("expected internalOp to be excluded")
	}
}

// ---------------------------------------------------------------------------
// 5. WithOperations – filters by operation ID
// ---------------------------------------------------------------------------

func TestWithOperations(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Ops Filter
  version: "1.0.0"
paths:
  /a:
    get:
      operationId: opA
      summary: Operation A
      responses:
        "200":
          description: OK
  /b:
    get:
      operationId: opB
      summary: Operation B
      responses:
        "200":
          description: OK
  /c:
    get:
      operationId: opC
      summary: Operation C
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "ops.yaml", spec)

	c := NewConverter(WithOperations([]string{"opA", "opC"}))
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "opA") {
		t.Error("expected opA")
	}
	if !strings.Contains(out, "opC") {
		t.Error("expected opC")
	}
	if strings.Contains(out, "opB") {
		t.Error("expected opB to be filtered out")
	}
}

// ---------------------------------------------------------------------------
// 6. ConvertFile – reads and converts local OpenAPI spec file
// ---------------------------------------------------------------------------

func TestConvertFile_Basic(t *testing.T) {
	specPath := writeTempSpec(t, "basic.yaml", minimalSpec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}

	// Header comment
	if !strings.Contains(out, "# Generated from OpenAPI spec: Minimal API") {
		t.Error("missing title in header")
	}
	if !strings.Contains(out, "# Version: 1.0.0") {
		t.Error("missing version in header")
	}
	// Base URL from servers
	if !strings.Contains(out, "@baseUrl = https://api.example.com") {
		t.Error("missing baseUrl")
	}
	// Request line
	if !strings.Contains(out, "GET {{baseUrl}}/health") {
		t.Error("missing GET /health request")
	}
	// Name annotation
	if !strings.Contains(out, "# @name healthCheck") {
		t.Error("missing @name annotation")
	}
}

// ---------------------------------------------------------------------------
// 7. Convert – converts OpenAPI doc to hitspec (via programmatic doc)
// ---------------------------------------------------------------------------

func TestConvert_ProgrammaticDoc(t *testing.T) {
	// Load from YAML to build a proper openapi3.T
	spec := `openapi: "3.0.3"
info:
  title: Programmatic
  version: "2.0.0"
servers:
  - url: https://prog.example.com
paths:
  /ping:
    get:
      operationId: ping
      summary: Ping
      responses:
        "200":
          description: OK
`
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(spec))
	if err != nil {
		t.Fatalf("LoadFromData: %v", err)
	}

	c := NewConverter()
	out, err := c.Convert(doc)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !strings.Contains(out, "GET {{baseUrl}}/ping") {
		t.Errorf("expected GET /ping, got:\n%s", out)
	}
	if !strings.Contains(out, "@baseUrl = https://prog.example.com") {
		t.Errorf("expected baseUrl from spec servers, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// 8. Path parameter conversion ({id} → {{id}})
// ---------------------------------------------------------------------------

func TestPathParameterConversion(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Path Params
  version: "1.0.0"
paths:
  /users/{userId}/posts/{postId}:
    get:
      operationId: getUserPost
      summary: Get user post
      parameters:
        - name: userId
          in: path
          required: true
          schema:
            type: string
        - name: postId
          in: path
          required: true
          schema:
            type: integer
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "path_params.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "GET {{baseUrl}}/users/{{userId}}/posts/{{postId}}") {
		t.Errorf("expected double-braced path params, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// 9. Query parameter generation (? syntax)
// ---------------------------------------------------------------------------

func TestQueryParameterGeneration(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Query Params
  version: "1.0.0"
paths:
  /search:
    get:
      operationId: search
      summary: Search
      parameters:
        - name: q
          in: query
          schema:
            type: string
        - name: limit
          in: query
          schema:
            type: integer
        - name: active
          in: query
          schema:
            type: boolean
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "query.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "? q = example") {
		t.Errorf("expected query param q, got:\n%s", out)
	}
	if !strings.Contains(out, "? limit = 1") {
		t.Errorf("expected query param limit, got:\n%s", out)
	}
	if !strings.Contains(out, "? active = true") {
		t.Errorf("expected query param active, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// 10. Request body generation from JSON schema (object, array, nested)
// ---------------------------------------------------------------------------

func TestRequestBody_Object(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Body Object
  version: "1.0.0"
paths:
  /users:
    post:
      operationId: createUser
      summary: Create user
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                age:
                  type: integer
                email:
                  type: string
                  format: email
      responses:
        "201":
          description: Created
`
	specPath := writeTempSpec(t, "body_object.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "Content-Type: application/json") {
		t.Error("missing Content-Type header")
	}
	if !strings.Contains(out, `"name": "example"`) {
		t.Errorf("expected name field in body, got:\n%s", out)
	}
	if !strings.Contains(out, `"age": 1`) {
		t.Errorf("expected age field in body, got:\n%s", out)
	}
	if !strings.Contains(out, `"email": "user@example.com"`) {
		t.Errorf("expected email field in body, got:\n%s", out)
	}
}

func TestRequestBody_Array(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Body Array
  version: "1.0.0"
paths:
  /items:
    post:
      operationId: createItems
      summary: Create items
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: array
              items:
                type: string
      responses:
        "201":
          description: Created
`
	specPath := writeTempSpec(t, "body_array.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, `["example"]`) {
		t.Errorf("expected array body with string item, got:\n%s", out)
	}
}

func TestRequestBody_Nested(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Body Nested
  version: "1.0.0"
paths:
  /orders:
    post:
      operationId: createOrder
      summary: Create order
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                customer:
                  type: object
                  properties:
                    name:
                      type: string
                items:
                  type: array
                  items:
                    type: object
                    properties:
                      product:
                        type: string
                      quantity:
                        type: integer
      responses:
        "201":
          description: Created
`
	specPath := writeTempSpec(t, "body_nested.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	// Nested object
	if !strings.Contains(out, `"customer"`) {
		t.Errorf("expected customer field, got:\n%s", out)
	}
	if !strings.Contains(out, `"name": "example"`) {
		t.Errorf("expected name field inside customer, got:\n%s", out)
	}
	// Array of objects
	if !strings.Contains(out, `"items"`) {
		t.Errorf("expected items field, got:\n%s", out)
	}
	if !strings.Contains(out, `"product": "example"`) {
		t.Errorf("expected product inside items, got:\n%s", out)
	}
	if !strings.Contains(out, `"quantity": 1`) {
		t.Errorf("expected quantity inside items, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// 11. Assertion generation from response schema
// ---------------------------------------------------------------------------

func TestAssertionGeneration(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Assertions
  version: "1.0.0"
paths:
  /data:
    get:
      operationId: getData
      summary: Get data
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: integer
`
	specPath := writeTempSpec(t, "assertions.yaml", spec)

	c := NewConverter() // generateTests defaults to true
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, ">>>") {
		t.Error("missing assertion block start >>>")
	}
	if !strings.Contains(out, "expect status == 200") {
		t.Errorf("missing status assertion, got:\n%s", out)
	}
	if !strings.Contains(out, "expect header Content-Type contains application/json") {
		t.Errorf("missing Content-Type assertion, got:\n%s", out)
	}
	if !strings.Contains(out, "<<<") {
		t.Error("missing assertion block end <<<")
	}
}

func TestAssertionGeneration_Disabled(t *testing.T) {
	specPath := writeTempSpec(t, "no_assert.yaml", minimalSpec)

	c := NewConverter(WithTests(false))
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if strings.Contains(out, ">>>") {
		t.Error("assertions should be disabled but found >>>")
	}
	if strings.Contains(out, "expect status") {
		t.Error("assertions should be disabled but found expect")
	}
}

func TestAssertionGeneration_201(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: "201 Test"
  version: "1.0.0"
paths:
  /items:
    post:
      operationId: createItem
      summary: Create item
      responses:
        "201":
          description: Created
`
	specPath := writeTempSpec(t, "assert201.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "expect status == 201") {
		t.Errorf("expected status 201 assertion, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// 12. Multiple HTTP methods for the same path
// ---------------------------------------------------------------------------

func TestMultipleMethodsSamePath(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Multi Method
  version: "1.0.0"
paths:
  /resources:
    get:
      operationId: listResources
      summary: List resources
      responses:
        "200":
          description: OK
    post:
      operationId: createResource
      summary: Create resource
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
      responses:
        "201":
          description: Created
    delete:
      operationId: deleteResource
      summary: Delete resource
      responses:
        "204":
          description: Deleted
`
	specPath := writeTempSpec(t, "multi_method.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "GET {{baseUrl}}/resources") {
		t.Error("expected GET /resources")
	}
	if !strings.Contains(out, "POST {{baseUrl}}/resources") {
		t.Error("expected POST /resources")
	}
	if !strings.Contains(out, "DELETE {{baseUrl}}/resources") {
		t.Error("expected DELETE /resources")
	}
}

// ---------------------------------------------------------------------------
// 13. Error cases
// ---------------------------------------------------------------------------

func TestConvertFile_InvalidPath(t *testing.T) {
	c := NewConverter()
	_, err := c.ConvertFile("/nonexistent/path/does_not_exist.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to load OpenAPI spec") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestConvertFile_InvalidSpec(t *testing.T) {
	specPath := writeTempSpec(t, "invalid.yaml", `this is not valid openapi at all: [[[`)
	c := NewConverter()
	_, err := c.ConvertFile(specPath)
	if err == nil {
		t.Fatal("expected error for invalid spec")
	}
}

func TestConvertFile_EmptyFile(t *testing.T) {
	// An empty YAML file is parsed as an empty OpenAPI doc by kin-openapi.
	// The converter prints a validation warning but still produces output
	// (just the header comment). Verify it does not panic.
	specPath := writeTempSpec(t, "empty.yaml", "")
	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		// Depending on kin-openapi version, empty file may error or not.
		// Either outcome is acceptable.
		return
	}
	// If no error, output should at least contain the generated header
	if !strings.Contains(out, "# Generated from OpenAPI spec") {
		t.Errorf("expected header comment in output, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Additional coverage: server URL fallback, header params, annotations, etc.
// ---------------------------------------------------------------------------

func TestBaseURL_DefaultFallback(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: No Server
  version: "1.0.0"
paths:
  /test:
    get:
      operationId: test
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "no_server.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "@baseUrl = http://localhost:3000") {
		t.Errorf("expected default baseUrl fallback, got:\n%s", out)
	}
}

func TestHeaderParameterGeneration(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Header Params
  version: "1.0.0"
paths:
  /protected:
    get:
      operationId: protectedEndpoint
      summary: Protected
      parameters:
        - name: X-API-Key
          in: header
          required: true
          schema:
            type: string
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "headers.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "X-API-Key: example") {
		t.Errorf("expected header parameter, got:\n%s", out)
	}
}

func TestDescriptionAnnotation(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Description Test
  version: "1.0.0"
paths:
  /desc:
    get:
      operationId: descOp
      summary: Described operation
      description: "This endpoint does something very useful"
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "desc.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "# @description This endpoint does something very useful") {
		t.Errorf("expected description annotation, got:\n%s", out)
	}
}

func TestTagsAnnotation(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Tags Annotation
  version: "1.0.0"
paths:
  /tagged:
    get:
      operationId: taggedOp
      summary: Tagged
      tags: [users, admin]
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "tags_annot.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "# @tags users,admin") {
		t.Errorf("expected tags annotation, got:\n%s", out)
	}
}

func TestRequestSeparator(t *testing.T) {
	specPath := writeTempSpec(t, "sep.yaml", minimalSpec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "### Health check") {
		t.Errorf("expected ### separator with summary, got:\n%s", out)
	}
}

func TestConvertToFile(t *testing.T) {
	specPath := writeTempSpec(t, "tofile.yaml", minimalSpec)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "subdir", "output.hitspec")

	c := NewConverter()
	err := c.ConvertToFile(specPath, outPath)
	if err != nil {
		t.Fatalf("ConvertToFile: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "GET {{baseUrl}}/health") {
		t.Error("output file missing expected content")
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"get-user-by-id", "get_user_by_id"},
		{"create user", "create_user"},
		{"op/with/slashes", "op_with_slashes"},
		{"multi---dashes", "multi_dashes"},
		{"__leading__", "leading"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/users", "Users"},
		{"/users/posts", "UsersPosts"},
		{"hello-world", "HelloWorld"},
		{"under_score", "UnderScore"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toTitle(tt.input)
			if got != tt.want {
				t.Errorf("toTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParamExamples(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Param Examples
  version: "1.0.0"
paths:
  /test:
    get:
      operationId: testExamples
      summary: Test examples
      parameters:
        - name: dateParam
          in: query
          schema:
            type: string
            format: date
        - name: dateTimeParam
          in: query
          schema:
            type: string
            format: date-time
        - name: emailParam
          in: query
          schema:
            type: string
            format: email
        - name: uuidParam
          in: query
          schema:
            type: string
            format: uuid
        - name: numberParam
          in: query
          schema:
            type: number
        - name: customExample
          in: query
          example: "myValue"
          schema:
            type: string
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "param_examples.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "? dateParam = 2024-01-01\n") {
		t.Errorf("expected date param, got:\n%s", out)
	}
	if !strings.Contains(out, "? dateTimeParam = 2024-01-01T00:00:00Z") {
		t.Errorf("expected dateTime param, got:\n%s", out)
	}
	if !strings.Contains(out, "? emailParam = user@example.com") {
		t.Errorf("expected email param, got:\n%s", out)
	}
	if !strings.Contains(out, "? uuidParam = {{$uuid()}}") {
		t.Errorf("expected uuid param, got:\n%s", out)
	}
	if !strings.Contains(out, "? numberParam = 1.0") {
		t.Errorf("expected number param, got:\n%s", out)
	}
	if !strings.Contains(out, "? customExample = myValue") {
		t.Errorf("expected custom example param, got:\n%s", out)
	}
}

func TestJSONValueTypes(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: JSON Value Types
  version: "1.0.0"
paths:
  /types:
    post:
      operationId: typeTest
      summary: Test types
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                strField:
                  type: string
                intField:
                  type: integer
                numField:
                  type: number
                boolField:
                  type: boolean
                dateField:
                  type: string
                  format: date
                dtField:
                  type: string
                  format: date-time
                emailField:
                  type: string
                  format: email
                uuidField:
                  type: string
                  format: uuid
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "json_values.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, `"strField": "example"`) {
		t.Errorf("expected string value, got:\n%s", out)
	}
	if !strings.Contains(out, `"intField": 1`) {
		t.Errorf("expected int value, got:\n%s", out)
	}
	if !strings.Contains(out, `"numField": 1.0`) {
		t.Errorf("expected number value, got:\n%s", out)
	}
	if !strings.Contains(out, `"boolField": true`) {
		t.Errorf("expected bool value, got:\n%s", out)
	}
	if !strings.Contains(out, `"dateField": "2024-01-01"`) {
		t.Errorf("expected date value, got:\n%s", out)
	}
	if !strings.Contains(out, `"dtField": "2024-01-01T00:00:00Z"`) {
		t.Errorf("expected date-time value, got:\n%s", out)
	}
	if !strings.Contains(out, `"emailField": "user@example.com"`) {
		t.Errorf("expected email value, got:\n%s", out)
	}
	if !strings.Contains(out, `"uuidField": "{{$uuid()}}"`) {
		t.Errorf("expected uuid value, got:\n%s", out)
	}
}

func TestSortedPaths(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Sorted Paths
  version: "1.0.0"
paths:
  /zebra:
    get:
      operationId: zebra
      responses:
        "200":
          description: OK
  /alpha:
    get:
      operationId: alpha
      responses:
        "200":
          description: OK
  /middle:
    get:
      operationId: middle
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "sorted.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}

	alphaIdx := strings.Index(out, "alpha")
	middleIdx := strings.Index(out, "middle")
	zebraIdx := strings.Index(out, "zebra")

	if alphaIdx >= middleIdx || middleIdx >= zebraIdx {
		t.Errorf("expected paths in sorted order (alpha, middle, zebra), got:\n%s", out)
	}
}

func TestOperationWithoutID_FallbackName(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: No OperationID
  version: "1.0.0"
paths:
  /users:
    get:
      summary: List all users
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "no_opid.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	// When operationId is missing, the summary is used for ###, and the
	// @name is generated from method+path via toTitle
	if !strings.Contains(out, "### List all users") {
		t.Errorf("expected summary in separator, got:\n%s", out)
	}
	if !strings.Contains(out, "# @name getUsers") {
		t.Errorf("expected generated @name fallback, got:\n%s", out)
	}
}

func TestPathLevelParameters(t *testing.T) {
	// Parameters defined at the path level (not operation level)
	spec := `openapi: "3.0.3"
info:
  title: Path-level Params
  version: "1.0.0"
paths:
  /orgs/{orgId}/members:
    parameters:
      - name: orgId
        in: path
        required: true
        schema:
          type: string
    get:
      operationId: listMembers
      summary: List members
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "path_level_params.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "/orgs/{{orgId}}/members") {
		t.Errorf("expected path-level parameter to be converted, got:\n%s", out)
	}
}

func TestLongDescriptionTruncation(t *testing.T) {
	longDesc := strings.Repeat("a", 200)
	spec := `openapi: "3.0.3"
info:
  title: Long Desc
  version: "1.0.0"
paths:
  /long:
    get:
      operationId: longDesc
      description: "` + longDesc + `"
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "long_desc.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, "...") {
		t.Error("expected truncated description with ellipsis")
	}
	// The truncated description (100 chars) + "..." should be in one line
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "# @description") {
			descPart := strings.TrimPrefix(line, "# @description ")
			// 100 chars + "..." = 103 chars
			if len(descPart) > 110 {
				t.Errorf("description too long (%d chars): %s", len(descPart), descPart)
			}
			break
		}
	}
}

func TestEnumValues(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Enum Test
  version: "1.0.0"
paths:
  /status:
    post:
      operationId: setStatus
      summary: Set status
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                status:
                  type: string
                  enum: [active, inactive, pending]
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "enum.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	// Should use first enum value
	if !strings.Contains(out, `"status": "active"`) {
		t.Errorf("expected first enum value in body, got:\n%s", out)
	}
}

func TestSchemaExample(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Example Test
  version: "1.0.0"
paths:
  /example:
    post:
      operationId: withExample
      summary: With example
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                  example: "John Doe"
      responses:
        "200":
          description: OK
`
	specPath := writeTempSpec(t, "example.yaml", spec)

	c := NewConverter()
	out, err := c.ConvertFile(specPath)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !strings.Contains(out, `"name": "John Doe"`) {
		t.Errorf("expected schema example value, got:\n%s", out)
	}
}
