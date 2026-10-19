package capture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/jsonbody"
	"github.com/tidwall/gjson"
)

type Extractor struct {
	response *http.Response
	body     *jsonbody.Body
}

func NewExtractor(resp *http.Response) *Extractor {
	return &Extractor{
		response: resp,
		body:     jsonbody.New(resp.ContentType(), resp.Body),
	}
}

// Extract returns the captured value. ok is false when the source has no value
// (a path that matches nothing, a missing header, a non-JSON body for a path);
// Miss explains why.
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

// Miss describes why a capture produced no value, for a warning. It is only
// meaningful after Extract returned ok == false for the same capture.
func (e *Extractor) Miss(capture *parser.Capture) string {
	switch capture.Source {
	case parser.CaptureBody:
		if err := e.body.Err(); err != nil && capture.Path != "" {
			return err.Error()
		}
		return fmt.Sprintf("the path %q matches nothing in the response body", capture.Path)
	case parser.CaptureHeader:
		return fmt.Sprintf("the response has no header %q", capture.Path)
	}
	return "no value"
}

func (e *Extractor) extractFromBody(path string) (any, bool) {
	if path == "" {
		// The whole body: parsed when it is JSON, the text otherwise.
		if e.body.Err() == nil {
			return jsonValue(e.body.Get("")), true
		}
		return e.response.BodyString(), true
	}
	if e.body.Err() != nil {
		return nil, false
	}

	result := e.body.Get(parser.BodyPathToGJSON(path))
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

// extractFromHeader reads a response header. A header that is present with an
// empty value is captured as "".
func (e *Extractor) extractFromHeader(name string) (any, bool) {
	for k, v := range e.response.Headers {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return nil, false
}

// Warn receives the warnings of ExtractAll. The default prints to whatever
// os.Stderr is at the time (so swapping os.Stderr, as the studio TUI does,
// silences it too); set it to nil to silence them.
var Warn io.Writer = stderrWriter{}

type stderrWriter struct{}

func (stderrWriter) Write(p []byte) (int, error) { return os.Stderr.Write(p) }

var warned sync.Map

// ExtractAll extracts every capture. A capture that finds nothing is left out
// of the result and reported once on Warn: a silent empty capture only shows up
// later as an unresolved {{variable}} in another request.
func ExtractAll(resp *http.Response, captures []*parser.Capture) map[string]any {
	values, misses := ExtractAllReport(resp, captures)
	if Warn != nil {
		for _, m := range misses {
			if _, dup := warned.LoadOrStore(m, true); dup {
				continue
			}
			fmt.Fprintf(Warn, "warning: %s\n", m)
		}
	}
	return values
}

// ExtractAllReport is ExtractAll that returns the miss descriptions instead of
// printing them.
func ExtractAllReport(resp *http.Response, captures []*parser.Capture) (map[string]any, []string) {
	extractor := NewExtractor(resp)
	results := make(map[string]any)
	var misses []string

	for _, c := range captures {
		if value, ok := extractor.Extract(c); ok {
			results[c.Name] = value
		} else {
			misses = append(misses, fmt.Sprintf("capture %q (line %d) captured nothing: %s", c.Name, c.Line, extractor.Miss(c)))
		}
	}

	return results, misses
}
