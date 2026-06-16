package parser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A >>>mock block is captured verbatim into Request.MockBody, and assertion
// blocks that follow it still parse.
func TestParseMockBlock_GET(t *testing.T) {
	input := `### Get user
# @name getUser

GET http://localhost:3000/users/1

>>>mock
{
  "id": 1,
  "name": "Alice"
}
<<<

>>>
expect status 200
expect body.id == 1
<<<
`
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 1)
	req := file.Requests[0]
	require.Contains(t, req.MockBody, `"id": 1`)
	require.Contains(t, req.MockBody, `"name": "Alice"`)
	// the mock block must not be swallowed into the request body
	require.Nil(t, req.Body)
	// assertions after the mock block still parse
	require.Len(t, req.Assertions, 2)
}

// A request body followed by a >>>mock block: the body stops at the mock block,
// and the mock block is captured separately.
func TestParseMockBlock_AfterBody(t *testing.T) {
	input := `POST http://localhost:3000/users
Content-Type: application/json

{"name": "Bob"}

>>>mock
{"id": 3, "name": "Bob"}
<<<

>>>
expect status 201
<<<
`
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	req := file.Requests[0]
	require.NotNil(t, req.Body)
	require.Contains(t, req.Body.Raw, "Bob")
	// the mock block must not bleed into the request body
	require.NotContains(t, req.Body.Raw, "id")
	require.Contains(t, req.MockBody, `"id": 3`)
	require.Len(t, req.Assertions, 1)
}

// A >>>mock body may contain >>> / <<< inside its content (e.g. in a JSON
// string); only a delimiter at the start of a line ends the block.
func TestParseMockBlock_DelimitersInContent(t *testing.T) {
	input := "### a\n# @name a\nGET http://x/a\n\n>>>mock\n{\"note\": \"arrows <<< and >>> inside\", \"n\": 1}\n<<<\n\n>>>\nexpect status 200\n<<<\n"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	req := file.Requests[0]
	require.Contains(t, req.MockBody, "<<< and >>> inside")
	require.Len(t, req.Assertions, 1)
}

// An indented closing <<< still terminates the block.
func TestParseMockBlock_IndentedTerminator(t *testing.T) {
	input := "### a\n# @name a\nGET http://x/a\n\n>>>mock\n{\"id\": 1}\n  <<<\n\n>>>\nexpect status 200\n<<<\n"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	req := file.Requests[0]
	require.Contains(t, req.MockBody, `"id": 1`)
	require.NotContains(t, req.MockBody, "<<<")
	require.Len(t, req.Assertions, 1)
}

// An unclosed >>>mock block (no closing <<<, runs to EOF) is a parse error,
// not silently consumed to end-of-file.
func TestParseMockBlock_UnclosedEOF(t *testing.T) {
	input := "### a\n# @name a\nGET http://x/a\n\n>>>mock\n{\"id\": 1}\n"
	_, err := Parse(input, "test.http")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unclosed")
}

// A properly closed mock block at end-of-file parses fine (no false positive).
func TestParseMockBlock_ClosedAtEOF(t *testing.T) {
	input := "### a\n# @name a\nGET http://x/a\n\n>>>mock\n{\"id\": 1}\n<<<\n"
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Contains(t, file.Requests[0].MockBody, `"id": 1`)
}

// An unclosed >>>graphql block is also a parse error.
func TestParseGraphQL_UnclosedEOF(t *testing.T) {
	input := "POST http://x/g\n\n>>>graphql\nquery { me { id } }\n"
	_, err := Parse(input, "test.http")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unclosed")
}

// Requests without a >>>mock block leave MockBody empty (backward compatible).
func TestParseMockBlock_Absent(t *testing.T) {
	input := `GET http://localhost:3000/x

>>>
expect status 200
<<<
`
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	require.Equal(t, "", file.Requests[0].MockBody)
}
