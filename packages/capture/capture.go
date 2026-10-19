package capture

import (
	"bytes"
	"encoding/json"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
	"github.com/tidwall/gjson"
)

type Extractor struct {
	response *http.Response
	bodyJSON gjson.Result
}

func NewExtractor(resp *http.Response) *Extractor {
	e := &Extractor{
		response: resp,
	}
	if resp.IsJSON() {
		e.bodyJSON = gjson.ParseBytes(resp.Body)
	}
	return e
}

func (e *Extractor) Extract(capture *parser.Capture) (any, bool) {
	switch capture.Source {
	case parser.CaptureBody:
		return e.extractFromBody(capture.Path)
	case parser.CaptureHeader:
		return e.extractFromHeader(capture.Path)
	case parser.CaptureStatus:
		return e.response.StatusCode, true
	case parser.CaptureDuration:
		return e.response.DurationMs(), true
	default:
		return nil, false
	}
}

func (e *Extractor) extractFromBody(path string) (any, bool) {
	if !e.bodyJSON.Exists() {
		if path == "" {
			return e.response.BodyString(), true
		}
		return nil, false
	}

	if path == "" {
		return jsonValue(e.bodyJSON), true
	}

	result := e.bodyJSON.Get(path)
	if !result.Exists() {
		return nil, false
	}
	return jsonValue(result), true
}

// jsonValue converts a gjson result to a Go value without losing numeric
// precision. gjson's Value() turns every number into a float64, so
// 123456789012 later formats as "1.23456789012e+11" and integers above 2^53
// are silently rounded. Numbers are returned as json.Number carrying the
// exact JSON text; objects and arrays are decoded with the same guarantee.
func jsonValue(r gjson.Result) any {
	switch r.Type {
	case gjson.Number:
		return json.Number(r.Raw)
	case gjson.JSON:
		dec := json.NewDecoder(bytes.NewReader([]byte(r.Raw)))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err == nil {
			return v
		}
	}
	return r.Value()
}

func (e *Extractor) extractFromHeader(name string) (any, bool) {
	value := e.response.Header(name)
	if value == "" {
		return nil, false
	}
	return value, true
}

func ExtractAll(resp *http.Response, captures []*parser.Capture) map[string]any {
	extractor := NewExtractor(resp)
	results := make(map[string]any)

	for _, c := range captures {
		if value, ok := extractor.Extract(c); ok {
			results[c.Name] = value
		}
	}

	return results
}
