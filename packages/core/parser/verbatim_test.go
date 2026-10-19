package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestLine(t *testing.T) {
	tests := []struct {
		name, line, url, version string
	}{
		{"plain", "GET http://x/api", "http://x/api", ""},
		{"version", "GET http://x/api HTTP/1.1", "http://x/api", "HTTP/1.1"},
		{"version http2", "POST http://x/api HTTP/2", "http://x/api", "HTTP/2"},
		{"space in builtin arg", "GET {{base}}/q?x={{$urlEncode(hello world)}}", "{{base}}/q?x={{$urlEncode(hello world)}}", ""},
		{"comma and space in builtin", "GET {{base}}/q?n={{$random(1, 100)}} HTTP/1.1", "{{base}}/q?n={{$random(1, 100)}}", "HTTP/1.1"},
		{"spaces inside braces", "GET {{ base }}/users/{{ id }}", "{{ base }}/users/{{ id }}", ""},
		{"fragment", "GET http://x/a#section", "http://x/a#section", ""},
		{"trailing comment", "GET http://x/a # list them", "http://x/a", ""},
		{"extra spaces", "GET    http://x/a   ", "http://x/a", ""},
		{"no url", "GET", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := Parse("### r\n"+tt.line+"\n", "t.http")
			require.NoError(t, err)
			require.Len(t, file.Requests, 1)
			assert.Equal(t, tt.url, file.Requests[0].URL)
			assert.Equal(t, tt.version, file.Requests[0].HTTPVersion)
		})
	}
}

func TestHeaders_Verbatim(t *testing.T) {
	input := "### h\nGET http://x/\n" +
		"If-None-Match: \"abc\"\n" +
		"Content-Disposition: attachment; filename=\"a.txt\"\n" +
		"X-Color: #fff\n" +
		"X-Note: it's fine\n" +
		"X-Json: {\"a\": 'b', \"c\": [1, 2]}\n" +
		"X-Url: http://y/z?a=b:c\n" +
		"X-Spaces:    padded value   \n" +
		"X-Tpl: {{ a }} and {{$random(1, 5)}}\n" +
		"X-Empty:\n" +
		"\n" +
		"body\n"
	file, err := Parse(input, "t.http")
	require.NoError(t, err)
	req := file.Requests[0]

	got := map[string]string{}
	var order []string
	for _, h := range req.Headers {
		got[h.Key] = h.Value
		order = append(order, h.Key)
	}
	assert.Equal(t, []string{"If-None-Match", "Content-Disposition", "X-Color", "X-Note", "X-Json", "X-Url", "X-Spaces", "X-Tpl", "X-Empty"}, order)
	assert.Equal(t, `"abc"`, got["If-None-Match"])
	assert.Equal(t, `attachment; filename="a.txt"`, got["Content-Disposition"])
	assert.Equal(t, "#fff", got["X-Color"])
	assert.Equal(t, "it's fine", got["X-Note"])
	assert.Equal(t, `{"a": 'b', "c": [1, 2]}`, got["X-Json"])
	assert.Equal(t, "http://y/z?a=b:c", got["X-Url"])
	assert.Equal(t, "padded value", got["X-Spaces"])
	assert.Equal(t, "{{ a }} and {{$random(1, 5)}}", got["X-Tpl"])
	assert.Equal(t, "", got["X-Empty"])
	require.NotNil(t, req.Body)
	assert.Equal(t, "body", req.Body.Raw)
}

func TestHeaders_CommentsInHeaderBlock(t *testing.T) {
	input := "### h\nPOST http://x/\n" +
		"X-First: one\n" +
		"# plain comment\n" +
		"// slash comment\n" +
		"# @name looks-like-a-directive\n" +
		"   # indented comment\n" +
		"X-Second: two\n" +
		"Content-Type: application/json\n" +
		"\n" +
		"{\"a\": 1}\n"
	file, err := Parse(input, "t.http")
	require.NoError(t, err)
	req := file.Requests[0]
	require.Len(t, req.Headers, 3)
	assert.Equal(t, "X-First", req.Headers[0].Key)
	assert.Equal(t, "X-Second", req.Headers[1].Key)
	assert.Equal(t, "two", req.Headers[1].Value)
	assert.Equal(t, "Content-Type", req.Headers[2].Key)
	assert.Equal(t, "h", req.Name, "an annotation-looking comment in the header block is only a comment")
	require.NotNil(t, req.Body)
	assert.Equal(t, `{"a": 1}`, req.Body.Raw)
	assert.Equal(t, BodyJSON, req.Body.ContentType)
}

func TestHeaders_CommentRightAfterRequestLine(t *testing.T) {
	input := "### h\nGET http://x/\n# note\nX-A: 1\n\n>>>\nexpect status 200\n<<<\n"
	file, err := Parse(input, "t.http")
	require.NoError(t, err)
	req := file.Requests[0]
	require.Len(t, req.Headers, 1)
	assert.Nil(t, req.Body)
	require.Len(t, req.Assertions, 1)
}

func TestQueryParams_Verbatim(t *testing.T) {
	input := "### q\nGET http://x/\n? name = it's \"x\" # y\n? page = {{ p }}\nAccept: */*\n"
	file, err := Parse(input, "t.http")
	require.NoError(t, err)
	req := file.Requests[0]
	require.Len(t, req.QueryParams, 2)
	assert.Equal(t, "name", req.QueryParams[0].Key)
	assert.Equal(t, `it's "x" # y`, req.QueryParams[0].Value)
	assert.Equal(t, "{{ p }}", req.QueryParams[1].Value)
	require.Len(t, req.Headers, 1)
}

func TestVariables_Verbatim(t *testing.T) {
	input := "@pw = abc#123\n@greeting = \"hi there\"\n@quote = it's\n@url = http://x/a#frag\n@empty =\n\n### r\nGET http://x/\n@mid = a # b\n"
	file, err := Parse(input, "t.http")
	require.NoError(t, err)
	got := map[string]string{}
	for _, v := range file.Variables {
		got[v.Name] = v.Value
	}
	// No quote stripping and no '#' truncation: the docs never describe either.
	assert.Equal(t, "abc#123", got["pw"])
	assert.Equal(t, `"hi there"`, got["greeting"])
	assert.Equal(t, "it's", got["quote"])
	assert.Equal(t, "http://x/a#frag", got["url"])
	assert.Equal(t, "", got["empty"])
	assert.Equal(t, "a # b", got["mid"])
	require.Len(t, file.Requests, 1)
}

func TestApostropheDoesNotSwallowFile(t *testing.T) {
	input := "it's prose outside any request\n\n" +
		"### one\n# @description it's the first\nGET http://x/1\nX-Note: it's fine\n\n" +
		"### two\nPOST http://x/2\nContent-Type: text/plain\n\ndon't panic, it's \"fine\"\n\n>>>\nexpect status 200\nexpect body.msg == \"it's\"\n<<<\n\n" +
		"### three\nGET http://x/3\n"
	file, err := Parse(input, "t.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 3)
	assert.Equal(t, "it's the first", file.Requests[0].Description)
	assert.Equal(t, "it's fine", file.Requests[0].Headers[0].Value)
	assert.Equal(t, `don't panic, it's "fine"`, file.Requests[1].Body.Raw)
	require.Len(t, file.Requests[1].Assertions, 2)
	assert.Equal(t, "it's", file.Requests[1].Assertions[1].Expected)
	assert.Equal(t, "http://x/3", file.Requests[2].URL)
}

func TestAssertionQuotesStillWork(t *testing.T) {
	input := "### a\nGET http://x/\n\n>>>\nexpect body.name == \"Jane Doe\"\nexpect body.other == 'single quoted'\nexpect body.msg contains \"it's # not a comment\"\n<<<\n"
	file, err := Parse(input, "t.http")
	require.NoError(t, err)
	as := file.Requests[0].Assertions
	require.Len(t, as, 3)
	assert.Equal(t, "Jane Doe", as[0].Expected)
	assert.Equal(t, "single quoted", as[1].Expected)
	assert.Equal(t, "it's # not a comment", as[2].Expected)
}

func TestBody_Verbatim(t *testing.T) {
	input := "### b\nPOST http://x/\nContent-Type: text/plain\n\n" +
		"line one: it's a \"test\"\n" +
		"mail bob@example.com a >>> b\n" +
		"# not a comment\n" +
		"  indented   spaces\n" +
		"\n" +
		"# trailing note before the block\n" +
		">>>\nexpect status 200\n<<<\n\n" +
		"### next\nGET http://x/\n"
	file, err := Parse(input, "t.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 2)
	req := file.Requests[0]
	require.NotNil(t, req.Body)
	assert.Equal(t, "line one: it's a \"test\"\nmail bob@example.com a >>> b\n# not a comment\n  indented   spaces", req.Body.Raw)
	assert.Equal(t, BodyRaw, req.Body.ContentType)
	require.Len(t, req.Assertions, 1)
}

func TestBody_StopsAtVariableLine(t *testing.T) {
	input := "### a\nPOST http://x/\nContent-Type: application/json\n\n{\"a\": 1}\n\n@later = value\n\n### b\nGET http://x/{{later}}\n"
	file, err := Parse(input, "t.http")
	require.NoError(t, err)
	require.Len(t, file.Requests, 2)
	assert.Equal(t, `{"a": 1}`, file.Requests[0].Body.Raw)
	require.Len(t, file.Variables, 1)
	assert.Equal(t, "later", file.Variables[0].Name)
}

func TestCRLF_Request(t *testing.T) {
	input := "### a\r\nPOST http://x/ HTTP/1.1\r\nX-A: \"1\"\r\n# c\r\nX-B: 2\r\n\r\n{\"a\": 1}\r\n"
	file, err := Parse(input, "t.http")
	require.NoError(t, err)
	req := file.Requests[0]
	assert.Equal(t, "http://x/", req.URL)
	require.Len(t, req.Headers, 2)
	assert.Equal(t, `"1"`, req.Headers[0].Value)
	assert.Equal(t, "2", req.Headers[1].Value)
	assert.Equal(t, `{"a": 1}`, req.Body.Raw)
}

func TestAuth(t *testing.T) {
	tests := []struct {
		name, value string
		typ         AuthType
		params      []string
	}{
		{"bearer", "bearer {{token}}", AuthBearer, []string{"{{token}}"}},
		{"bearer spaced var", "bearer {{ token }}", AuthBearer, []string{"{{ token }}"}},
		{"basic comma", "basic alice, s3cret", AuthBasic, []string{"alice", "s3cret"}},
		{"basic comma vars", "basic {{username}}, {{password}}", AuthBasic, []string{"{{username}}", "{{password}}"}},
		{"basic spaces", "basic alice s3cret", AuthBasic, []string{"alice", "s3cret"}},
		{"basic comma password with space", "basic alice, pass word", AuthBasic, []string{"alice", "pass word"}},
		{"basic empty password", "basic alice,", AuthBasic, []string{"alice", ""}},
		{"basic spaced vars", "basic {{ user }} {{ pass }}", AuthBasic, []string{"{{ user }}", "{{ pass }}"}},
		{"basic var with comma inside braces", "basic {{$random(1, 5)}} pw", AuthBasic, []string{"{{$random(1, 5)}}", "pw"}},
		{"apiKey", "apiKey X-API-Key, k", AuthAPIKey, []string{"X-API-Key", "k"}},
		{"apiKey spaces", "apiKey X-API-Key k", AuthAPIKey, []string{"X-API-Key", "k"}},
		{"apiKeyQuery", "apiKeyQuery api_key, k", AuthAPIKeyQuery, []string{"api_key", "k"}},
		{"apikey-query", "apikey-query api_key k", AuthAPIKeyQuery, []string{"api_key", "k"}},
		{"digest", "digest user, passwd", AuthDigest, []string{"user", "passwd"}},
		{"aws", "aws AK, SK, us-east-1, s3", AuthAWS, []string{"AK", "SK", "us-east-1", "s3"}},
		{"type is case insensitive", "BASIC alice, s3cret", AuthBasic, []string{"alice", "s3cret"}},
		{"cc no scopes", "oauth2 client_credentials http://x/token, id, secret", AuthOAuth2ClientCredentials, []string{"http://x/token", "id", "secret"}},
		{"cc scopes", "oauth2 client_credentials {{tokenUrl}}, {{id}}, {{secret}}, scope1,scope2", AuthOAuth2ClientCredentials, []string{"{{tokenUrl}}", "{{id}}", "{{secret}}", "scope1,scope2"}},
		{"cc scopes spaces", "oauth2 client_credentials http://x/token id secret scope1 scope2", AuthOAuth2ClientCredentials, []string{"http://x/token", "id", "secret", "scope1,scope2"}},
		{"password", "oauth2 password http://x/t, id, secret, bob, pw", AuthOAuth2Password, []string{"http://x/t", "id", "secret", "bob", "pw"}},
		{"password scopes", "oauth2 password http://x/t, id, secret, bob, pw, a,b", AuthOAuth2Password, []string{"http://x/t", "id", "secret", "bob", "pw", "a,b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := Parse("### a\n# @auth "+tt.value+"\nGET http://x/\n", "t.http")
			require.NoError(t, err)
			auth := file.Requests[0].Metadata.Auth
			require.NotNil(t, auth)
			assert.Equal(t, tt.typ, auth.Type)
			assert.Equal(t, tt.params, auth.Params)
		})
	}
}

func TestAuth_Errors(t *testing.T) {
	tests := []struct{ name, value, msg string }{
		{"unknown type", "magic foo", `unknown @auth type "magic"`},
		{"empty", "", "@auth needs a type"},
		{"bearer none", "bearer", "@auth bearer expects 1 arguments (token), got 0"},
		{"bearer two", "bearer a b", "@auth bearer expects 1 arguments (token), got 2"},
		{"basic one", "basic alice", "@auth basic expects 2 arguments (username, password), got 1"},
		{"basic three", "basic a, b, c", "@auth basic expects 2 arguments (username, password), got 3"},
		{"apikey one", "apiKey X-API-Key", "@auth apiKey expects 2 arguments (headerName, value), got 1"},
		{"aws three", "aws a, b, c", "@auth aws expects 4 arguments"},
		{"oauth2 no grant", "oauth2", "@auth oauth2 needs a grant type"},
		{"oauth2 unknown grant", "oauth2 implicit http://x", `@auth oauth2 needs a grant type: client_credentials or password (got "implicit")`},
		{"cc two", "oauth2 client_credentials http://x/t, id", "@auth oauth2 client_credentials expects 3 or more arguments"},
		{"password four", "oauth2 password u, i, s, bob", "@auth oauth2 password expects 5 or more arguments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse("### a\n# @auth "+tt.value+"\nGET http://x/\n", "t.http")
			require.Error(t, err)
			var perr *ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, 2, perr.Line)
			assert.Contains(t, err.Error(), tt.msg)
		})
	}
}
