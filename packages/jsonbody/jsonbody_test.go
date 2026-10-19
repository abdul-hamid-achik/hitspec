package jsonbody

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsJSONMediaType(t *testing.T) {
	yes := []string{
		"application/json",
		"APPLICATION/JSON",
		"application/json; charset=utf-8",
		"application/json;charset=UTF-8",
		"application/problem+json",
		"application/problem+json; charset=utf-8",
		"application/vnd.api+json",
		"application/ld+json",
		"application/hal+json; profile=x",
		"text/json",
	}
	no := []string{"", "text/plain", "text/html; charset=utf-8", "application/xml", "application/jsonp", "application/x-ndjson", "image/png", "json"}
	for _, ct := range yes {
		assert.True(t, IsJSONMediaType(ct), ct)
	}
	for _, ct := range no {
		assert.False(t, IsJSONMediaType(ct), ct)
	}
}

func TestBodyErr(t *testing.T) {
	assert.NoError(t, New("application/problem+json", []byte(`{"a":1}`)).Err())
	assert.ErrorContains(t, New("text/html", []byte(`{"a":1}`)).Err(), "not JSON")
	assert.ErrorContains(t, New("", []byte(`{"a":1}`)).Err(), "no Content-Type")
	assert.ErrorContains(t, New("application/json", []byte(`{oops`)).Err(), "not valid JSON")
	assert.ErrorContains(t, New("application/json", nil).Err(), "empty")
}

func TestLookup(t *testing.T) {
	item := map[string]any{"id": float64(1), "tags": []any{"a", "b"}}
	v, ok := Lookup(item, "")
	assert.True(t, ok)
	assert.Equal(t, item, v)
	v, ok = Lookup(item, "tags[1]")
	assert.True(t, ok)
	assert.Equal(t, "b", v)
	v, ok = Lookup(item, "tags.#")
	assert.True(t, ok)
	assert.Equal(t, float64(2), v)
	_, ok = Lookup(item, "nope")
	assert.False(t, ok)
}
