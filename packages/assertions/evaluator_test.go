package assertions

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createResponse(statusCode int, body string, headers map[string]string) *http.Response {
	if headers == nil {
		headers = make(map[string]string)
	}
	if _, ok := headers["Content-Type"]; !ok {
		headers["Content-Type"] = "application/json"
	}
	return &http.Response{
		StatusCode: statusCode,
		Status:     "",
		Headers:    headers,
		Body:       []byte(body),
		Duration:   100 * time.Millisecond,
	}
}

func TestEvaluator_StatusCode(t *testing.T) {
	resp := createResponse(200, `{}`, nil)
	e := NewEvaluator(resp)

	result := e.Evaluate(&parser.Assertion{
		Subject:  "status",
		Operator: parser.OpEquals,
		Expected: 200,
	})

	assert.True(t, result.Passed)
	assert.Equal(t, 200, result.Actual)
}

func TestEvaluator_StatusCodeNotEquals(t *testing.T) {
	resp := createResponse(404, `{}`, nil)
	e := NewEvaluator(resp)

	result := e.Evaluate(&parser.Assertion{
		Subject:  "status",
		Operator: parser.OpNotEquals,
		Expected: 200,
	})

	assert.True(t, result.Passed)
}

func TestEvaluator_Body_JSONPath(t *testing.T) {
	resp := createResponse(200, `{"user": {"name": "John", "age": 30}}`, nil)
	e := NewEvaluator(resp)

	tests := []struct {
		name     string
		subject  string
		operator parser.AssertionOperator
		expected any
		passed   bool
	}{
		{
			name:     "nested path equals",
			subject:  "body.user.name",
			operator: parser.OpEquals,
			expected: "John",
			passed:   true,
		},
		{
			name:     "nested path numeric",
			subject:  "body.user.age",
			operator: parser.OpEquals,
			expected: 30,
			passed:   true,
		},
		{
			name:     "greater than",
			subject:  "body.user.age",
			operator: parser.OpGreaterThan,
			expected: 25,
			passed:   true,
		},
		{
			name:     "less than",
			subject:  "body.user.age",
			operator: parser.OpLessThan,
			expected: 35,
			passed:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(&parser.Assertion{
				Subject:  tt.subject,
				Operator: tt.operator,
				Expected: tt.expected,
			})
			assert.Equal(t, tt.passed, result.Passed, "Message: %s", result.Message)
		})
	}
}

func TestEvaluator_Body_Array(t *testing.T) {
	resp := createResponse(200, `{"items": [1, 2, 3, 4, 5]}`, nil)
	e := NewEvaluator(resp)

	t.Run("array length", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "body.items",
			Operator: parser.OpLength,
			Expected: 5,
		})
		assert.True(t, result.Passed)
	})

	t.Run("array includes", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "body.items",
			Operator: parser.OpIncludes,
			Expected: 3,
		})
		assert.True(t, result.Passed)
	})

	t.Run("array not includes", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "body.items",
			Operator: parser.OpNotIncludes,
			Expected: 10,
		})
		assert.True(t, result.Passed)
	})

	t.Run("bracket notation access", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "body.items[0]",
			Operator: parser.OpEquals,
			Expected: 1,
		})
		assert.True(t, result.Passed, "Message: %s", result.Message)
	})
}

func TestEvaluator_Exists(t *testing.T) {
	resp := createResponse(200, `{"name": "test", "value": null}`, nil)
	e := NewEvaluator(resp)

	t.Run("exists - present", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "body.name",
			Operator: parser.OpExists,
		})
		assert.True(t, result.Passed)
	})

	t.Run("exists - null value", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "body.value",
			Operator: parser.OpExists,
		})
		// null exists but is nil
		assert.False(t, result.Passed)
	})

	t.Run("not exists - missing", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "body.missing",
			Operator: parser.OpNotExists,
		})
		assert.True(t, result.Passed)
	})
}

func TestEvaluator_Contains(t *testing.T) {
	resp := createResponse(200, `{"message": "Hello, World!"}`, nil)
	e := NewEvaluator(resp)

	t.Run("contains - match", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "body.message",
			Operator: parser.OpContains,
			Expected: "World",
		})
		assert.True(t, result.Passed)
	})

	t.Run("not contains - no match", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "body.message",
			Operator: parser.OpNotContains,
			Expected: "Goodbye",
		})
		assert.True(t, result.Passed)
	})
}

func TestEvaluator_StartsWithEndsWith(t *testing.T) {
	resp := createResponse(200, `{"url": "https://example.com/api/v1"}`, nil)
	e := NewEvaluator(resp)

	t.Run("starts with", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "body.url",
			Operator: parser.OpStartsWith,
			Expected: "https://",
		})
		assert.True(t, result.Passed)
	})

	t.Run("ends with", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "body.url",
			Operator: parser.OpEndsWith,
			Expected: "/v1",
		})
		assert.True(t, result.Passed)
	})
}

func TestEvaluator_Matches(t *testing.T) {
	resp := createResponse(200, `{"email": "test@example.com"}`, nil)
	e := NewEvaluator(resp)

	result := e.Evaluate(&parser.Assertion{
		Subject:  "body.email",
		Operator: parser.OpMatches,
		Expected: `^[a-z]+@[a-z]+\.[a-z]+$`,
	})
	assert.True(t, result.Passed)
}

func TestEvaluator_Type(t *testing.T) {
	resp := createResponse(200, `{
		"string": "hello",
		"number": 42,
		"boolean": true,
		"array": [1, 2, 3],
		"object": {"key": "value"},
		"null": null
	}`, nil)
	e := NewEvaluator(resp)

	tests := []struct {
		subject  string
		expected string
		passed   bool
	}{
		{"body.string", "string", true},
		{"body.number", "number", true},
		{"body.boolean", "boolean", true},
		{"body.array", "array", true},
		{"body.object", "object", true},
		{"body.null", "null", true},
	}

	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			result := e.Evaluate(&parser.Assertion{
				Subject:  tt.subject,
				Operator: parser.OpType,
				Expected: tt.expected,
			})
			assert.Equal(t, tt.passed, result.Passed, "Message: %s", result.Message)
		})
	}
}

func TestEvaluator_In(t *testing.T) {
	resp := createResponse(200, `{"status": "active"}`, nil)
	e := NewEvaluator(resp)

	t.Run("in - match", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "body.status",
			Operator: parser.OpIn,
			Expected: []any{"active", "pending", "completed"},
		})
		assert.True(t, result.Passed)
	})

	t.Run("not in - no match", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "body.status",
			Operator: parser.OpNotIn,
			Expected: []any{"deleted", "archived"},
		})
		assert.True(t, result.Passed)
	})
}

func TestEvaluator_Duration(t *testing.T) {
	resp := createResponse(200, `{}`, nil)
	resp.Duration = 50 * time.Millisecond
	e := NewEvaluator(resp)

	result := e.Evaluate(&parser.Assertion{
		Subject:  "duration",
		Operator: parser.OpLessThan,
		Expected: 100,
	})
	assert.True(t, result.Passed)
}

func TestEvaluator_Header(t *testing.T) {
	resp := createResponse(200, `{}`, map[string]string{
		"Content-Type":  "application/json",
		"X-Custom":      "custom-value",
		"Cache-Control": "no-cache",
	})
	e := NewEvaluator(resp)

	t.Run("header equals", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "header X-Custom",
			Operator: parser.OpEquals,
			Expected: "custom-value",
		})
		assert.True(t, result.Passed)
	})

	t.Run("header contains", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "header Content-Type",
			Operator: parser.OpContains,
			Expected: "json",
		})
		assert.True(t, result.Passed)
	})
}

func TestEvaluator_Schema(t *testing.T) {
	// Create a temporary schema file
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "user.schema.json")
	schema := `{
		"type": "object",
		"required": ["name", "email"],
		"properties": {
			"name": {"type": "string"},
			"email": {"type": "string", "format": "email"}
		}
	}`
	err := os.WriteFile(schemaPath, []byte(schema), 0644)
	require.NoError(t, err)

	resp := createResponse(200, `{"name": "John", "email": "john@example.com"}`, nil)
	e := NewEvaluatorWithBaseDir(resp, tmpDir)

	result := e.Evaluate(&parser.Assertion{
		Subject:  "body",
		Operator: parser.OpSchema,
		Expected: "user.schema.json",
	})
	assert.True(t, result.Passed, "Message: %s", result.Message)
}

func TestEvaluator_Schema_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	resp := createResponse(200, `{}`, nil)
	e := NewEvaluatorWithBaseDir(resp, tmpDir)

	result := e.Evaluate(&parser.Assertion{
		Subject:  "body",
		Operator: parser.OpSchema,
		Expected: "../../../etc/passwd",
	})
	assert.False(t, result.Passed)
	assert.Contains(t, result.Message, "path traversal")
}

func TestEvaluator_Each(t *testing.T) {
	resp := createResponse(200, `{"users": [{"name": "a"}, {"name": "b"}, {"name": "c"}]}`, nil)
	e := NewEvaluator(resp)

	t.Run("each with operator map", func(t *testing.T) {
		result := e.Evaluate(&parser.Assertion{
			Subject:  "body.users",
			Operator: parser.OpEach,
			Expected: map[string]any{
				"operator": "exists",
				"value":    nil,
			},
		})
		// Each element should have the structure but exists checks the element itself
		assert.True(t, result.Passed, "Message: %s", result.Message)
	})
}

func TestEvaluateAll(t *testing.T) {
	resp := createResponse(200, `{"status": "ok", "count": 5}`, nil)

	assertions := []*parser.Assertion{
		{Subject: "status", Operator: parser.OpEquals, Expected: 200},
		{Subject: "body.status", Operator: parser.OpEquals, Expected: "ok"},
		{Subject: "body.count", Operator: parser.OpGreaterThan, Expected: 0},
	}

	results := EvaluateAll(resp, assertions)

	assert.Len(t, results, 3)
	for _, r := range results {
		assert.True(t, r.Passed, "Failed: %s - %s", r.Subject, r.Message)
	}
}

func TestValidatePathWithinBase(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		baseDir string
		wantErr bool
	}{
		{
			name:    "path within base",
			path:    "/home/user/project/schema.json",
			baseDir: "/home/user/project",
			wantErr: false,
		},
		{
			name:    "path traversal",
			path:    "/home/user/project/../../../etc/passwd",
			baseDir: "/home/user/project",
			wantErr: true,
		},
		{
			name:    "empty base",
			path:    "/any/path",
			baseDir: "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePathWithinBase(tt.path, tt.baseDir)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
