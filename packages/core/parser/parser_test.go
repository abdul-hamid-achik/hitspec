package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_SimpleGET(t *testing.T) {
	input := `### Get User
GET https://api.example.com/users/1

>>>
expect status 200
<<<`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)

	req := file.Requests[0]
	assert.Equal(t, "Get User", req.Name)
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "https://api.example.com/users/1", req.URL)
	require.Len(t, req.Assertions, 1)
	assert.Equal(t, "status", req.Assertions[0].Subject)
	assert.Equal(t, OpEquals, req.Assertions[0].Operator)
	assert.Equal(t, 200, req.Assertions[0].Expected)
}

func TestParser_POSTWithBody(t *testing.T) {
	input := `### Create User
POST https://api.example.com/users
Content-Type: application/json

{
  "name": "John",
  "email": "john@example.com"
}

>>>
expect status 201
expect body.id exists
<<<`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)

	req := file.Requests[0]
	assert.Equal(t, "Create User", req.Name)
	assert.Equal(t, "POST", req.Method)
	require.Len(t, req.Headers, 1)
	assert.Equal(t, "Content-Type", req.Headers[0].Key)
	assert.Equal(t, "application/json", req.Headers[0].Value)
	require.NotNil(t, req.Body)
	assert.Equal(t, BodyJSON, req.Body.ContentType)
	require.Len(t, req.Assertions, 2)
}

func TestParser_Variables(t *testing.T) {
	input := `@baseUrl = https://api.example.com
@token = secret123

### Get User
GET {{baseUrl}}/users
Authorization: Bearer {{token}}`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Variables, 2)
	assert.Equal(t, "baseUrl", file.Variables[0].Name)
	assert.Equal(t, "https://api.example.com", file.Variables[0].Value)
	assert.Equal(t, "token", file.Variables[1].Name)
	assert.Equal(t, "secret123", file.Variables[1].Value)

	require.Len(t, file.Requests, 1)
	req := file.Requests[0]
	assert.Equal(t, "{{baseUrl}}/users", req.URL)
	assert.Equal(t, "Bearer {{token}}", req.Headers[0].Value)
}

func TestParser_Captures(t *testing.T) {
	input := `### Login
POST https://api.example.com/auth/login

>>>
expect status 200
<<<

>>>capture
token from body.access_token
userId from body.user.id
<<<`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)

	req := file.Requests[0]
	require.Len(t, req.Captures, 2)
	assert.Equal(t, "token", req.Captures[0].Name)
	assert.Equal(t, CaptureBody, req.Captures[0].Source)
	assert.Equal(t, "access_token", req.Captures[0].Path)
	assert.Equal(t, "userId", req.Captures[1].Name)
	assert.Equal(t, "user.id", req.Captures[1].Path)
}

func TestParser_Annotations(t *testing.T) {
	input := `### Test Request
# @name myTest
# @description This is a test request
# @tags smoke, auth
# @timeout 5000
# @retry 3

GET https://api.example.com/test`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)

	req := file.Requests[0]
	// @name overrides the separator name for better DX
	assert.Equal(t, "myTest", req.Name)
	assert.Equal(t, "This is a test request", req.Description)
	assert.Contains(t, req.Tags, "smoke")
	assert.Contains(t, req.Tags, "auth")
	assert.Equal(t, 5000, req.Metadata.Timeout)
	assert.Equal(t, 3, req.Metadata.Retry)
}

func TestParser_MultipleRequests(t *testing.T) {
	input := `### First Request
GET https://api.example.com/first

### Second Request
POST https://api.example.com/second

### Third Request
DELETE https://api.example.com/third`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 3)
	assert.Equal(t, "First Request", file.Requests[0].Name)
	assert.Equal(t, "GET", file.Requests[0].Method)
	assert.Equal(t, "Second Request", file.Requests[1].Name)
	assert.Equal(t, "POST", file.Requests[1].Method)
	assert.Equal(t, "Third Request", file.Requests[2].Name)
	assert.Equal(t, "DELETE", file.Requests[2].Method)
}

func TestParser_AssertionOperators(t *testing.T) {
	tests := []struct {
		input    string
		expected AssertionOperator
	}{
		{"expect status == 200", OpEquals},
		{"expect status != 404", OpNotEquals},
		{"expect body.count > 0", OpGreaterThan},
		{"expect body.count >= 1", OpGreaterOrEqual},
		{"expect duration < 1000", OpLessThan},
		{"expect duration <= 500", OpLessOrEqual},
		{"expect body.name contains \"test\"", OpContains},
		{"expect body.name !contains \"error\"", OpNotContains},
		{"expect body.id exists", OpExists},
		{"expect body.error !exists", OpNotExists},
		{"expect body.items length 10", OpLength},
		{"expect body.items length > 0", OpLengthGt},
		{"expect body.items length >= 1", OpLengthGte},
		{"expect body.items length < 100", OpLengthLt},
		{"expect body.items length <= 50", OpLengthLte},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			input := "### Test\nGET http://test.com\n\n>>>\n" + tt.input + "\n<<<"
			file, err := Parse(input, "test.http")
			require.NoError(t, err)
			require.Len(t, file.Requests, 1)
			require.Len(t, file.Requests[0].Assertions, 1)
			assert.Equal(t, tt.expected, file.Requests[0].Assertions[0].Operator)
		})
	}
}

func TestParser_QueryParams(t *testing.T) {
	input := `### Search
GET https://api.example.com/search
? query = test
? limit = 10
? offset = 0`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)

	req := file.Requests[0]
	require.Len(t, req.QueryParams, 3)
	assert.Equal(t, "query", req.QueryParams[0].Key)
	assert.Equal(t, "test", req.QueryParams[0].Value)
}

func TestParser_FormBody(t *testing.T) {
	input := `### Login
POST https://api.example.com/login
Content-Type: application/x-www-form-urlencoded

& username = john
& password = secret123`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)

	req := file.Requests[0]
	require.NotNil(t, req.Body)
	assert.Equal(t, BodyFormBlock, req.Body.ContentType)
	assert.Contains(t, req.Body.Raw, "username=john")
	assert.Contains(t, req.Body.Raw, "password=secret123")
}

func TestParser_Auth(t *testing.T) {
	input := `### Protected Resource
# @auth bearer {{token}}

GET https://api.example.com/protected`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)

	req := file.Requests[0]
	require.NotNil(t, req.Metadata)
	require.NotNil(t, req.Metadata.Auth)
	assert.Equal(t, AuthBearer, req.Metadata.Auth.Type)
	require.Len(t, req.Metadata.Auth.Params, 1)
	assert.Equal(t, "{{token}}", req.Metadata.Auth.Params[0])
}

func TestParser_Skip(t *testing.T) {
	input := `### Skipped Test
# @skip This test is temporarily disabled

GET https://api.example.com/skip`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)

	req := file.Requests[0]
	assert.Equal(t, "This test is temporarily disabled", req.Metadata.Skip)
}

func TestParser_EscapedQuotesInBody(t *testing.T) {
	input := `### Escaped quotes test
POST https://api.example.com/test
Content-Type: application/json

{"content": "{\"test\": true}", "nested": "{\"key\": \"value\"}"}`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)

	req := file.Requests[0]
	require.NotNil(t, req.Body)
	assert.Equal(t, BodyJSON, req.Body.ContentType)
	// Verify escaped quotes are preserved
	assert.Contains(t, req.Body.Raw, `"{\"test\": true}"`)
	assert.Contains(t, req.Body.Raw, `"{\"key\": \"value\"}"`)
}

func TestParser_EscapeSequencesInBody(t *testing.T) {
	input := `### Escape sequences test
POST https://api.example.com/test
Content-Type: application/json

{"message": "line1\nline2\ttab", "path": "C:\\Users\\test"}`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)

	req := file.Requests[0]
	require.NotNil(t, req.Body)
	// Verify escape sequences are preserved
	assert.Contains(t, req.Body.Raw, `\n`)
	assert.Contains(t, req.Body.Raw, `\t`)
	assert.Contains(t, req.Body.Raw, `\\`)
}

func TestParser_RetryOn(t *testing.T) {
	input := `### Retry On Status
# @retry 3
# @retryOn 500, 502, 503

GET https://api.example.com/test`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)

	req := file.Requests[0]
	assert.Equal(t, 3, req.Metadata.Retry)
	require.Len(t, req.Metadata.RetryOn, 3)
	assert.Equal(t, 500, req.Metadata.RetryOn[0])
	assert.Equal(t, 502, req.Metadata.RetryOn[1])
	assert.Equal(t, 503, req.Metadata.RetryOn[2])
}

func TestParser_RetryOnSingleCode(t *testing.T) {
	input := `### Retry On Single
# @retryOn 429

GET https://api.example.com/test`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)

	req := file.Requests[0]
	require.Len(t, req.Metadata.RetryOn, 1)
	assert.Equal(t, 429, req.Metadata.RetryOn[0])
}

func TestParser_ConditionIf(t *testing.T) {
	input := `### Conditional Request
# @if {{runTests}}

GET https://api.example.com/test`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)

	req := file.Requests[0]
	require.NotNil(t, req.Metadata.Condition)
	assert.Equal(t, ConditionIf, req.Metadata.Condition.Type)
	assert.Equal(t, "{{runTests}}", req.Metadata.Condition.Expression)
}

func TestParser_ConditionUnless(t *testing.T) {
	input := `### Skip Auth Test
# @unless {{skipAuth}}

GET https://api.example.com/auth/test`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)

	req := file.Requests[0]
	require.NotNil(t, req.Metadata.Condition)
	assert.Equal(t, ConditionUnless, req.Metadata.Condition.Type)
	assert.Equal(t, "{{skipAuth}}", req.Metadata.Condition.Expression)
}

func TestParser_EmptyFile(t *testing.T) {
	file, err := Parse("", "empty.http")
	require.NoError(t, err)
	assert.Empty(t, file.Requests)
	assert.Empty(t, file.Variables)
}

func TestParser_OnlyComments(t *testing.T) {
	input := `# This is a comment
# Another comment
// Also a comment`

	file, err := Parse(input, "comments.http")
	require.NoError(t, err)
	assert.Empty(t, file.Requests)
}

func TestParser_OnlyVariables(t *testing.T) {
	input := `@baseUrl = https://api.example.com
@token = secret`

	file, err := Parse(input, "vars.http")
	require.NoError(t, err)
	assert.Len(t, file.Variables, 2)
	assert.Empty(t, file.Requests)
}

func TestParser_MissingSeparator(t *testing.T) {
	// A request without ### separator should still parse if it starts with a method
	input := `GET https://api.example.com/test`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	assert.Equal(t, "GET", file.Requests[0].Method)
	assert.Equal(t, "https://api.example.com/test", file.Requests[0].URL)
}

func TestParser_ParseErrorIncludesSnippet(t *testing.T) {
	// A request separator followed by non-method text should produce a parse error with snippet
	input := `### Bad Request
not-a-method https://example.com`

	_, err := Parse(input, "bad.http")
	require.Error(t, err)
	pe, ok := err.(*ParseError)
	require.True(t, ok, "expected *ParseError, got %T", err)
	assert.Contains(t, pe.Message, "expected HTTP method")
	assert.NotEmpty(t, pe.Snippet, "parse error should include a source snippet")
	assert.Equal(t, "bad.http", pe.File)
}

func TestParser_WhitespaceOnlyFile(t *testing.T) {
	input := "   \n\n\t\n   "
	file, err := Parse(input, "ws.http")
	require.NoError(t, err)
	assert.Empty(t, file.Requests)
}

func TestParser_AssertionLineNumbers(t *testing.T) {
	input := `### Test
GET http://example.com

>>>
expect status 200
expect body.id exists
<<<`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	require.Len(t, file.Requests[0].Assertions, 2)
	// Line numbers should be > 0
	assert.True(t, file.Requests[0].Assertions[0].Line > 0, "assertion should have line number")
	assert.True(t, file.Requests[0].Assertions[1].Line > 0, "assertion should have line number")
	// Second assertion should be on a later line
	assert.True(t, file.Requests[0].Assertions[1].Line > file.Requests[0].Assertions[0].Line,
		"second assertion should be on a later line")
}

func TestParser_CustomAnnotations(t *testing.T) {
	input := `### Contract Test
# @contract.state user exists
# @x-custom foo

GET http://example.com`

	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	req := file.Requests[0]
	require.NotNil(t, req.Metadata.Custom)
	assert.Equal(t, "user exists", req.Metadata.Custom["contract.state"])
	assert.Equal(t, "foo", req.Metadata.Custom["x-custom"])
}

// --- UTF-8 multi-byte character tests ---

func TestParser_UTF8InURL(t *testing.T) {
	input := "### UTF8 URL\nGET https://api.example.com/search?q=\u00e9l\u00e8ve"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	assert.Contains(t, file.Requests[0].URL, "\u00e9l\u00e8ve")
}

func TestParser_UTF8InHeader(t *testing.T) {
	input := "### UTF8 Header\nGET https://api.example.com/test\nX-Custom: caf\u00e9 cr\u00e8me"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	require.Len(t, file.Requests[0].Headers, 1)
	assert.Equal(t, "caf\u00e9 cr\u00e8me", file.Requests[0].Headers[0].Value)
}

func TestParser_UTF8InBody(t *testing.T) {
	input := "### UTF8 Body\nPOST https://api.example.com/test\nContent-Type: application/json\n\n{\"name\": \"\u00fc\u00f6\u00e4\", \"city\": \"\u6771\u4eac\"}"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	require.NotNil(t, file.Requests[0].Body)
	assert.Contains(t, file.Requests[0].Body.Raw, "\u00fc\u00f6\u00e4")
	assert.Contains(t, file.Requests[0].Body.Raw, "\u6771\u4eac")
}

func TestParser_UTF8InStringLiteral(t *testing.T) {
	input := "### UTF8 Assert\nGET http://test.com\n\n>>>\nexpect body.name == \"\u00e9l\u00e8ve\"\n<<<"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	require.Len(t, file.Requests[0].Assertions, 1)
	assert.Equal(t, "\u00e9l\u00e8ve", file.Requests[0].Assertions[0].Expected)
}

func TestParser_UTF8InVariableValue(t *testing.T) {
	input := "@greeting = \u3053\u3093\u306b\u3061\u306f\n\n### UTF8 Var\nGET http://test.com"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Variables, 1)
	assert.Equal(t, "\u3053\u3093\u306b\u3061\u306f", file.Variables[0].Value)
}

func TestParser_UTF8InRequestSeparatorName(t *testing.T) {
	// Request separator names aren't identifiers, they're free-form text
	input := "### R\u00e9sum\u00e9 Upload\nGET http://test.com"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	assert.Equal(t, "R\u00e9sum\u00e9 Upload", file.Requests[0].Name)
}

func TestParser_UTF8Emoji(t *testing.T) {
	// Emoji are 4-byte UTF-8 sequences
	input := "### Emoji\nPOST http://test.com\nContent-Type: application/json\n\n{\"reaction\": \"\U0001f600\"}"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	require.NotNil(t, file.Requests[0].Body)
	assert.Contains(t, file.Requests[0].Body.Raw, "\U0001f600")
}

// --- Shell/DB block edge cases ---

func TestParser_EmptyShellBlock(t *testing.T) {
	input := "### Test\nGET http://test.com\n\n>>>shell\n<<<\n"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	assert.Empty(t, file.Requests[0].ShellCommands)
}

func TestParser_EmptyDBBlock(t *testing.T) {
	input := "### Test\n# @db sqlite3://test.db\nGET http://test.com\n\n>>>db\n<<<\n"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	assert.Empty(t, file.Requests[0].DBAssertions)
}

func TestParser_EmptyAssertionBlock(t *testing.T) {
	input := "### Test\nGET http://test.com\n\n>>>\n<<<\n"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	assert.Empty(t, file.Requests[0].Assertions)
}

func TestParser_ShellBlockMissingClose(t *testing.T) {
	// Missing <<< should parse until EOF without panic
	input := "### Test\nGET http://test.com\n\n>>>shell\necho hello"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	require.Len(t, file.Requests[0].ShellCommands, 1)
	assert.Equal(t, "echo hello", file.Requests[0].ShellCommands[0].Command)
}

func TestParser_DBBlockMissingClose(t *testing.T) {
	input := "### Test\n# @db sqlite3://test.db\nGET http://test.com\n\n>>>db\nquery SELECT count(*) as cnt FROM users\nexpect cnt > 0"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	require.Len(t, file.Requests[0].DBAssertions, 1)
}

func TestParser_AssertionBlockMissingClose(t *testing.T) {
	input := "### Test\nGET http://test.com\n\n>>>\nexpect status 200"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	require.Len(t, file.Requests[0].Assertions, 1)
}

// --- Parse error quality tests ---

func TestParser_ParseErrorIncludesColumnPointer(t *testing.T) {
	input := "### Bad Request\nnot-a-method https://example.com"
	_, err := Parse(input, "err.http")
	require.Error(t, err)
	pe, ok := err.(*ParseError)
	require.True(t, ok)
	errStr := pe.Error()
	// Should contain file:line:col, the snippet, and a caret
	assert.Contains(t, errStr, "err.http:")
	assert.Contains(t, errStr, "expected HTTP method")
	assert.Contains(t, errStr, "^")
}

// --- @waitFor parsing edge cases ---

func TestParser_WaitForDefaults(t *testing.T) {
	input := "### Wait\n# @waitFor http://localhost:8080/health\n\nGET http://test.com"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	wf := file.Requests[0].Metadata.WaitFor
	require.NotNil(t, wf)
	assert.Equal(t, "http://localhost:8080/health", wf.URL)
	assert.Equal(t, 200, wf.Status)     // default
	assert.Equal(t, 30000, wf.Timeout)  // default 30s
	assert.Equal(t, 1000, wf.Interval)  // default 1s
}

func TestParser_WaitForAllParams(t *testing.T) {
	input := "### Wait\n# @waitFor http://localhost:8080/ready 204 10000 500\n\nGET http://test.com"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	wf := file.Requests[0].Metadata.WaitFor
	require.NotNil(t, wf)
	assert.Equal(t, "http://localhost:8080/ready", wf.URL)
	assert.Equal(t, 204, wf.Status)
	assert.Equal(t, 10000, wf.Timeout)
	assert.Equal(t, 500, wf.Interval)
}

// --- Multipart parsing ---

func TestParser_MultipartBody(t *testing.T) {
	input := "### Upload\nPOST http://test.com/upload\n\n>>>multipart\nfield name = John Doe\nfile @./photo.jpg\n<<<\n"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	req := file.Requests[0]
	require.NotNil(t, req.Body)
	assert.Equal(t, BodyMultipart, req.Body.ContentType)
	require.Len(t, req.Body.Multipart, 2)
	assert.Equal(t, MultipartFieldValue, req.Body.Multipart[0].Type)
	assert.Equal(t, "name", req.Body.Multipart[0].Name)
	assert.Equal(t, "John Doe", req.Body.Multipart[0].Value)
	assert.Equal(t, MultipartFieldFile, req.Body.Multipart[1].Type)
	assert.Equal(t, "./photo.jpg", req.Body.Multipart[1].Path)
}

// --- Stress metadata parsing ---

func TestParser_StressAnnotations(t *testing.T) {
	input := "### Stress\n# @stress.weight 5\n# @stress.think 200\n# @stress.setup\n\nGET http://test.com"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	s := file.Requests[0].Metadata.Stress
	require.NotNil(t, s)
	assert.Equal(t, 5, s.Weight)
	assert.Equal(t, 200, s.Think)
	assert.True(t, s.Setup)
}

// --- @depends parsing ---

func TestParser_DependsAnnotation(t *testing.T) {
	input := "### Child\n# @depends parentA, parentB\n\nGET http://test.com"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	deps := file.Requests[0].Metadata.Depends
	require.Len(t, deps, 2)
	assert.Equal(t, "parentA", deps[0])
	assert.Equal(t, "parentB", deps[1])
}

// --- Capture edge cases ---

func TestParser_CaptureStatus(t *testing.T) {
	input := "### Test\nGET http://test.com\n\n>>>capture\ncode from status\n<<<"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests[0].Captures, 1)
	c := file.Requests[0].Captures[0]
	assert.Equal(t, "code", c.Name)
	assert.Equal(t, CaptureStatus, c.Source)
}

func TestParser_CaptureDuration(t *testing.T) {
	input := "### Test\nGET http://test.com\n\n>>>capture\nms from duration\n<<<"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests[0].Captures, 1)
	c := file.Requests[0].Captures[0]
	assert.Equal(t, "ms", c.Name)
	assert.Equal(t, CaptureDuration, c.Source)
}

func TestParser_CaptureHeader(t *testing.T) {
	input := "### Test\nGET http://test.com\n\n>>>capture\nloc from header Location\n<<<"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests[0].Captures, 1)
	c := file.Requests[0].Captures[0]
	assert.Equal(t, "loc", c.Name)
	assert.Equal(t, CaptureHeader, c.Source)
	assert.Equal(t, "Location", c.Path)
}
