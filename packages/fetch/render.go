package fetch

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// Render converts a Result to raw, readable text, Markdown, or JSON.
func Render(ctx context.Context, result *Result, format Format) ([]byte, error) {
	if result == nil {
		return nil, errors.New("cannot render a nil response")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	switch format {
	case FormatRaw:
		return append([]byte(nil), result.Body...), nil
	case FormatText:
		return renderText(result)
	case FormatMarkdown:
		return renderMarkdown(ctx, result)
	case FormatJSON:
		return renderJSON(result)
	default:
		return nil, fmt.Errorf("unsupported response format %q", format)
	}
}

func renderText(result *Result) ([]byte, error) {
	kind := bodyKind(result.ContentType, result.Body)
	if kind == "binary" {
		return nil, fmt.Errorf("response media type %q is binary or not readable as UTF-8; use --format raw", mediaType(result.ContentType, result.Body))
	}
	decoded, err := decodeText(result.Body, result.ContentType, kind == "html")
	if err != nil {
		return nil, err
	}
	switch kind {
	case "html":
		return []byte(visibleHTMLText(decoded)), nil
	case "json":
		var pretty bytes.Buffer
		if json.Indent(&pretty, decoded, "", "  ") == nil {
			return pretty.Bytes(), nil
		}
	}
	return decoded, nil
}

func renderMarkdown(ctx context.Context, result *Result) ([]byte, error) {
	kind := bodyKind(result.ContentType, result.Body)
	if kind == "binary" {
		return nil, fmt.Errorf("response media type %q is binary; use --format raw", mediaType(result.ContentType, result.Body))
	}
	decoded, err := decodeText(result.Body, result.ContentType, kind == "html")
	if err != nil {
		return nil, err
	}
	var body string
	switch kind {
	case "html":
		body, err = htmltomarkdown.ConvertString(string(decoded), converter.WithContext(ctx), converter.WithDomain(result.FinalURL))
	case "markdown":
		body = string(decoded)
	case "json":
		var pretty bytes.Buffer
		if json.Indent(&pretty, decoded, "", "  ") == nil {
			decoded = pretty.Bytes()
		}
		body = fenced("json", string(decoded))
	case "xml":
		body = fenced("xml", string(decoded))
	default:
		body = fenced("text", string(decoded))
	}
	if err != nil {
		return nil, fmt.Errorf("convert response to Markdown: %w", err)
	}
	requested, finalURL := SanitizeURL(result.RequestedURL), SanitizeURL(result.FinalURL)
	if finalURL == "" {
		finalURL = requested
	}
	status := cleanInline(result.Status)
	if status == "" {
		status = fmt.Sprintf("%d", result.StatusCode)
	}
	contentType := cleanInline(result.ContentType)
	if contentType == "" {
		contentType = mediaType(result.ContentType, result.Body)
	}
	var output strings.Builder
	output.WriteString("# HTTP response\n\n")
	fmt.Fprintf(&output, "- Source: <%s>\n", requested)
	if finalURL != requested {
		fmt.Fprintf(&output, "- Final URL: <%s>\n", finalURL)
	}
	fmt.Fprintf(&output, "- Status: `%s`\n- Content-Type: `%s`\n", status, contentType)
	fmt.Fprintf(&output, "- Duration: `%s`\n- Size: `%d bytes`\n\n## Body\n\n", humanDuration(result.Duration), len(result.Body))
	output.WriteString(strings.TrimSpace(body))
	output.WriteByte('\n')
	return []byte(output.String()), nil
}

type jsonResponse struct {
	Source       string              `json:"source"`
	FinalURL     string              `json:"final_url"`
	Status       string              `json:"status"`
	StatusCode   int                 `json:"status_code"`
	ContentType  string              `json:"content_type,omitempty"`
	DurationMS   float64             `json:"duration_ms"`
	Size         int                 `json:"size"`
	Headers      map[string][]string `json:"headers,omitempty"`
	Body         string              `json:"body"`
	BodyEncoding string              `json:"body_encoding"`
}

func renderJSON(result *Result) ([]byte, error) {
	body, encoding := string(result.Body), "utf-8"
	if bodyKind(result.ContentType, result.Body) == "binary" || !utf8.Valid(result.Body) {
		body, encoding = base64.StdEncoding.EncodeToString(result.Body), "base64"
	}
	document := jsonResponse{
		Source: SanitizeURL(result.RequestedURL), FinalURL: SanitizeURL(result.FinalURL),
		Status: cleanInline(result.Status), StatusCode: result.StatusCode,
		ContentType: cleanInline(result.ContentType), DurationMS: float64(result.Duration) / float64(time.Millisecond),
		Size: len(result.Body), Headers: sanitizedHeaders(result.Headers), Body: body, BodyEncoding: encoding,
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode response JSON: %w", err)
	}
	return append(encoded, '\n'), nil
}

// SanitizeURL removes credentials, query parameters, and fragments before a
// URL is used as provenance or diagnostics.
func SanitizeURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.User, parsed.RawQuery, parsed.ForceQuery, parsed.Fragment = nil, "", false, ""
	return parsed.String()
}

func sanitizedHeaders(headers http.Header) map[string][]string {
	sensitive := map[string]bool{"authorization": true, "proxy-authorization": true, "set-cookie": true, "cookie": true}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string][]string, len(keys))
	for _, key := range keys {
		if sensitive[strings.ToLower(key)] {
			continue
		}
		for _, value := range headers.Values(key) {
			canonical := http.CanonicalHeaderKey(key)
			result[canonical] = append(result[canonical], cleanInline(value))
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func bodyKind(contentType string, body []byte) string {
	media := mediaType(contentType, body)
	switch {
	case media == "text/html" || media == "application/xhtml+xml":
		return "html"
	case media == "application/json" || strings.HasSuffix(media, "+json"):
		return "json"
	case media == "text/markdown" || media == "text/x-markdown" || media == "application/markdown":
		return "markdown"
	case media == "application/xml" || media == "text/xml" || strings.HasSuffix(media, "+xml"):
		return "xml"
	case strings.HasPrefix(media, "text/") || media == "application/javascript" || media == "application/x-www-form-urlencoded":
		return "text"
	case media == "" && utf8.Valid(body) && !bytes.ContainsRune(body, '\x00'):
		return "text"
	default:
		return "binary"
	}
}

func mediaType(contentType string, body []byte) string {
	if parsed, _, err := mime.ParseMediaType(contentType); err == nil && parsed != "" {
		parsed = strings.ToLower(parsed)
		if (parsed == "application/octet-stream" || parsed == "text/plain") && len(body) > 0 {
			detected, _, _ := mime.ParseMediaType(http.DetectContentType(body))
			if detected == "text/html" {
				return detected
			}
		}
		return parsed
	}
	if len(body) == 0 {
		return ""
	}
	detected, _, _ := mime.ParseMediaType(http.DetectContentType(body))
	return strings.ToLower(detected)
}

func decodeText(body []byte, contentType string, htmlDocument bool) ([]byte, error) {
	if len(body) == 0 {
		return []byte{}, nil
	}
	if htmlDocument {
		reader, err := charset.NewReader(bytes.NewReader(body), contentType)
		if err != nil {
			return nil, fmt.Errorf("decode HTML response: %w", err)
		}
		return io.ReadAll(reader)
	}
	_, params, _ := mime.ParseMediaType(contentType)
	label := strings.TrimSpace(params["charset"])
	if label != "" && !strings.EqualFold(label, "utf-8") && !strings.EqualFold(label, "utf8") {
		reader, err := charset.NewReaderLabel(label, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("decode response charset %q: %w", label, err)
		}
		return io.ReadAll(reader)
	}
	if !utf8.Valid(body) {
		return nil, errors.New("response body is not valid UTF-8; use --format raw")
	}
	return append([]byte(nil), body...), nil
}

func visibleHTMLText(body []byte) string {
	document, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return strings.TrimSpace(string(body))
	}
	var output strings.Builder
	var walk func(*html.Node, bool)
	walk = func(node *html.Node, hidden bool) {
		if node.Type == html.ElementNode {
			switch node.Data {
			case "script", "style", "template", "noscript", "svg", "head":
				hidden = true
			}
			if blockElement(node.Data) {
				output.WriteByte('\n')
			}
		}
		if node.Type == html.TextNode && !hidden {
			output.WriteString(node.Data)
			output.WriteByte(' ')
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, hidden)
		}
		if node.Type == html.ElementNode && blockElement(node.Data) {
			output.WriteByte('\n')
		}
	}
	walk(document, false)
	var lines []string
	for _, line := range strings.Split(output.String(), "\n") {
		if clean := strings.Join(strings.Fields(line), " "); clean != "" {
			lines = append(lines, clean)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func blockElement(name string) bool {
	switch name {
	case "address", "article", "aside", "blockquote", "br", "div", "dl", "fieldset", "figcaption", "figure", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hr", "li", "main", "nav", "ol", "p", "pre", "section", "table", "tr", "ul":
		return true
	default:
		return false
	}
}

func fenced(language, body string) string {
	fence := "```"
	for strings.Contains(body, fence) {
		fence += "`"
	}
	return fence + language + "\n" + strings.TrimSpace(body) + "\n" + fence
}

func cleanInline(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	return strings.Join(strings.Fields(value), " ")
}

func humanDuration(duration time.Duration) string {
	if duration <= 0 {
		return "0s"
	}
	return duration.Round(time.Microsecond).String()
}
