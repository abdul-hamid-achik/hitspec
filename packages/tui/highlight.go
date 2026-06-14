package tui

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
)

// highlightStyle is the chroma style used for terminal syntax highlighting.
// It mirrors the TUI's Nord palette; chroma falls back gracefully if absent.
const highlightStyle = "nord"

// prettyJSON pretty-prints a JSON document with two-space indentation. If the
// input is not valid JSON it is returned unchanged so callers can pipe any body
// through it safely.
func prettyJSON(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return body
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(trimmed), "", "  "); err != nil {
		return body
	}
	return buf.String()
}

// lexerName maps a content-type (and, failing that, a quick body sniff) to a
// chroma lexer name. Returns "" when no confident match is found.
func lexerName(contentType, body string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "json"):
		return "json"
	case strings.Contains(ct, "html"):
		return "html"
	case strings.Contains(ct, "xml"):
		return "xml"
	case strings.Contains(ct, "yaml"), strings.Contains(ct, "yml"):
		return "yaml"
	case strings.Contains(ct, "graphql"):
		return "graphql"
	case strings.Contains(ct, "javascript"), strings.Contains(ct, "ecmascript"):
		return "javascript"
	case strings.Contains(ct, "css"):
		return "css"
	}
	switch c := firstNonSpace(body); c {
	case '{', '[':
		return "json"
	case '<':
		return "xml"
	}
	return ""
}

func firstNonSpace(s string) byte {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\n' && s[i] != '\t' && s[i] != '\r' {
			return s[i]
		}
	}
	return 0
}

// highlight renders code with chroma's terminal formatter. When color is false
// (golden tests, NO_COLOR terminals) or no lexer matches, the code is returned
// unchanged so output stays deterministic.
func highlight(code, lexerHint string, color bool) string {
	if !color || strings.TrimSpace(code) == "" {
		return code
	}
	lexer := lexers.Get(lexerHint)
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		return code
	}
	lexer = chroma.Coalesce(lexer)

	style := chromastyles.Get(highlightStyle)
	if style == nil {
		style = chromastyles.Fallback
	}
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}
	var buf strings.Builder
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return code
	}
	return buf.String()
}

// formatBody pretty-prints JSON bodies and syntax-highlights any recognized
// body, honoring the color flag. It is the single entry point response views use
// to render a payload.
func formatBody(body, contentType string, color bool) string {
	name := lexerName(contentType, body)
	out := body
	if name == "json" {
		out = prettyJSON(body)
	}
	return highlight(out, name, color)
}

// headerValue does a case-insensitive lookup of an HTTP header.
func headerValue(headers map[string]string, key string) string {
	if headers == nil {
		return ""
	}
	if v, ok := headers[key]; ok {
		return v
	}
	lower := strings.ToLower(key)
	for k, v := range headers {
		if strings.ToLower(k) == lower {
			return v
		}
	}
	return ""
}
