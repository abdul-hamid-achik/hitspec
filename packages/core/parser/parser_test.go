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
