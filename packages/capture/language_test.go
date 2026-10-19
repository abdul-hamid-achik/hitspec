package capture

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturesFrom parses a >>>capture block and returns its captures.
func capturesFrom(t *testing.T, lines string) []*parser.Capture {
	t.Helper()
	f, err := parser.Parse("GET http://x/\n\n>>>capture\n"+lines+"\n<<<\n", "t.http")
	require.NoError(t, err)
	return f.Requests[0].Captures
}

func TestCapturePaths(t *testing.T) {
	body := `{"items":[{"name":"Jane Doe","id":1,"tags":["a","b"]},{"name":"Bob","id":2,"tags":[]}],"id":123456789012,"user":{"email":"e@x.io"},"matrix":[[1,2],[3]]}`
	for _, ct := range []string{"application/json", "application/problem+json; charset=utf-8"} {
		resp := &http.Response{StatusCode: 200, Headers: map[string]string{"Content-Type": ct, "X-Id": "h1", "X-Empty": ""}, Body: []byte(body)}
		caps := capturesFrom(t, strings.Join([]string{
			"first from body.items[0].name",
			"second from body.items[1].id",
			"count from body.items.#",
			"jane from body.items.#(name==\"Jane Doe\").id",
			"names from body.items.#.name",
			"idx from body[0]",
			"cell from body.matrix[0][1]",
			"all from body.items[*].id",
			"bigid from body.id",
			"jp from jsonpath $.user.email",
			"jpall from jsonpath $.items[*].id",
			"hid from header X-Id",
			"hempty from header X-Empty",
			"code from status",
		}, "\n"))
		got, misses := ExtractAllReport(resp, caps)
		// body[0] is not an array element here
		assert.Contains(t, misses[0], `"idx"`)
		require.Len(t, misses, 1, "%v", misses)
		assert.Equal(t, "Jane Doe", got["first"])
		assert.Equal(t, json.Number("2"), got["second"])
		assert.Equal(t, json.Number("2"), got["count"])
		assert.Equal(t, json.Number("1"), got["jane"])
		assert.Equal(t, []any{"Jane Doe", "Bob"}, got["names"])
		assert.Equal(t, json.Number("2"), got["cell"])
		assert.Equal(t, []any{json.Number("1"), json.Number("2")}, got["all"])
		assert.Equal(t, json.Number("123456789012"), got["bigid"])
		assert.Equal(t, "e@x.io", got["jp"])
		assert.Equal(t, []any{json.Number("1"), json.Number("2")}, got["jpall"])
		assert.Equal(t, "h1", got["hid"])
		assert.Equal(t, "", got["hempty"])
		assert.Equal(t, 200, got["code"])
	}
}

func TestCaptureWholeBody(t *testing.T) {
	resp := &http.Response{Headers: map[string]string{"Content-Type": "application/json"}, Body: []byte(`{"a":1,"b":[true]}`)}
	got, misses := ExtractAllReport(resp, capturesFrom(t, "doc from body"))
	assert.Empty(t, misses)
	assert.Equal(t, map[string]any{"a": json.Number("1"), "b": []any{true}}, got["doc"])

	text := &http.Response{Headers: map[string]string{"Content-Type": "text/plain"}, Body: []byte("hello")}
	got, misses = ExtractAllReport(text, capturesFrom(t, "doc from body"))
	assert.Empty(t, misses)
	assert.Equal(t, "hello", got["doc"])
}

func TestCaptureMissesAreReported(t *testing.T) {
	json := &http.Response{Headers: map[string]string{"Content-Type": "application/json"}, Body: []byte(`{"a":1}`)}
	_, misses := ExtractAllReport(json, capturesFrom(t, "x from body.nope\ny from header X-Nope\nz from body.a"))
	require.Len(t, misses, 2)
	assert.Contains(t, misses[0], `capture "x"`)
	assert.Contains(t, misses[0], "matches nothing")
	assert.Contains(t, misses[1], `no header "X-Nope"`)

	html := &http.Response{Headers: map[string]string{"Content-Type": "text/html"}, Body: []byte(`<p>`)}
	_, misses = ExtractAllReport(html, capturesFrom(t, "x from body.a"))
	require.Len(t, misses, 1)
	assert.Contains(t, misses[0], "not JSON")

	// ExtractAll prints each miss once.
	var buf bytes.Buffer
	old := Warn
	Warn = &buf
	defer func() { Warn = old }()
	caps := capturesFrom(t, "uniqueMissName from body.nope")
	for i := 0; i < 3; i++ {
		ExtractAll(json, caps)
	}
	assert.Equal(t, 1, strings.Count(buf.String(), "uniqueMissName"))
	assert.Contains(t, buf.String(), "warning: capture")
}
