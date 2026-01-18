package capture

import (
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/http"
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
		return e.bodyJSON.Value(), true
	}

	result := e.bodyJSON.Get(path)
	if !result.Exists() {
		return nil, false
	}
	return result.Value(), true
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
