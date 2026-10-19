package jsonbody

import (
	"encoding/json"
	"fmt"
	"mime"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/tidwall/gjson"
)

// IsJSONMediaType reports whether a Content-Type header names a JSON body:
// application/json, text/json and any structured-syntax suffix
// (application/problem+json, application/vnd.api+json, ...), with or without
// parameters (; charset=utf-8).
func IsJSONMediaType(contentType string) bool {
	ct := strings.TrimSpace(contentType)
	if ct == "" {
		return false
	}
	media, _, err := mime.ParseMediaType(ct)
	if err != nil {
		media = strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0]))
	}
	switch media {
	case "application/json", "text/json":
		return true
	}
	return strings.HasSuffix(media, "+json")
}

// Body is a response body prepared for path lookups.
type Body struct {
	raw         []byte
	contentType string
	isJSON      bool
	valid       bool
}

// New wraps a response body. Only a JSON Content-Type makes path lookups
// possible; the bytes must also be valid JSON.
func New(contentType string, raw []byte) *Body {
	b := &Body{raw: raw, contentType: contentType, isJSON: IsJSONMediaType(contentType)}
	if b.isJSON {
		b.valid = gjson.ValidBytes(raw)
	}
	return b
}

// Raw returns the body bytes as received.
func (b *Body) Raw() []byte { return b.raw }

// Err explains why path lookups are impossible, or returns nil when they work.
func (b *Body) Err() error {
	switch {
	case !b.isJSON:
		ct := b.contentType
		if ct == "" {
			ct = "no Content-Type"
		}
		return fmt.Errorf("response body is not JSON (%s); a body path needs a JSON response", ct)
	case !b.valid && len(strings.TrimSpace(string(b.raw))) == 0:
		return fmt.Errorf("response body is empty, there is no JSON to read a path from")
	case !b.valid:
		return fmt.Errorf("response body has a JSON Content-Type but is not valid JSON")
	}
	return nil
}

// Root returns the parsed whole body.
func (b *Body) Root() any { return gjson.ParseBytes(b.raw).Value() }

// Get looks up a body path (already converted with parser.BodyPathToGJSON). The
// empty path is the whole body.
func (b *Body) Get(gpath string) gjson.Result {
	if gpath == "" {
		return gjson.ParseBytes(b.raw)
	}
	return gjson.GetBytes(b.raw, gpath)
}

// Lookup reads a body path from an already decoded JSON value (an array element
// for "each"). The boolean is false when the path matches nothing.
func Lookup(v any, path string) (any, bool) {
	if path == "" {
		return v, true
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	r := gjson.GetBytes(raw, parser.BodyPathToGJSON(path))
	if !r.Exists() {
		return nil, false
	}
	return r.Value(), true
}
