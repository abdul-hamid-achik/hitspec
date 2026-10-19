// Package snippet renders hitspec requests as runnable client code: curl,
// fetch (JavaScript), wget, python (requests), httpie, go and ruby. It is the
// single implementation behind both `hitspec export <format>` and the Studio
// export action, so the two always agree on format names and output.
package snippet

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/export/curl"
)

// Formats lists the supported format names in display order.
var Formats = []string{"curl", "fetch", "wget", "python", "httpie", "go", "ruby"}

// Normalize maps a user-supplied format name (case-insensitive, with a few
// aliases) to one of Formats. The second result is false for unknown names.
func Normalize(format string) (string, bool) {
	f := strings.ToLower(strings.TrimSpace(format))
	switch f {
	case "":
		return "curl", true
	case "javascript", "js":
		f = "fetch"
	case "py":
		f = "python"
	case "http":
		f = "httpie"
	case "golang":
		f = "go"
	}
	for _, known := range Formats {
		if f == known {
			return f, true
		}
	}
	return "", false
}

// Option configures Render.
type Option func(*options)

type options struct {
	resolve func(string) string
	verbose bool
}

// WithResolver interpolates {{variables}} in the request before rendering.
func WithResolver(r func(string) string) Option {
	return func(o *options) { o.resolve = r }
}

// WithVerbose adds -v to curl output (ignored by other formats).
func WithVerbose(v bool) Option {
	return func(o *options) { o.verbose = v }
}

// Render renders one request in the given format. Unknown formats and request
// shapes a format cannot express (multipart or file bodies outside curl) are
// errors rather than silently wrong snippets.
func Render(format string, req *parser.Request, opts ...Option) (string, error) {
	f, ok := Normalize(format)
	if !ok {
		return "", fmt.Errorf("unknown export format %q; supported formats: %s", format, strings.Join(Formats, ", "))
	}
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	if f == "curl" {
		var co []curl.Option
		if o.resolve != nil {
			co = append(co, curl.WithResolver(o.resolve))
		}
		if o.verbose {
			co = append(co, curl.WithVerbose(true))
		}
		return curl.New(co...).Export(req), nil
	}

	p, err := prepare(req, o.resolve)
	if err != nil {
		return "", fmt.Errorf("cannot export %s as %s: %w", describe(req), f, err)
	}
	var body string
	switch f {
	case "fetch":
		body = fetchSnippet(p)
	case "wget":
		body = wgetSnippet(p)
	case "python":
		body = pythonSnippet(p)
	case "httpie":
		body = httpieSnippet(p)
	case "go":
		body = goSnippet(p)
	case "ruby":
		body = rubySnippet(p)
	}
	var sb strings.Builder
	for _, n := range p.notes {
		sb.WriteString(Comment(f) + " NOTE: " + n + "\n")
	}
	sb.WriteString(body)
	return sb.String(), nil
}

// Comment returns the line-comment prefix of the given format's language.
func Comment(format string) string {
	if f, _ := Normalize(format); f == "fetch" || f == "go" {
		return "//"
	}
	return "#"
}

func describe(r *parser.Request) string {
	if r.Name != "" {
		return fmt.Sprintf("request %q", r.Name)
	}
	return fmt.Sprintf("%s %s", r.Method, r.URL)
}

type header struct{ key, value string }

// prepared is a request flattened to what every snippet needs.
type prepared struct {
	method  string
	url     string
	headers []header
	body    string
	notes   []string
}

func (p *prepared) hasHeader(k string) bool {
	for _, h := range p.headers {
		if strings.EqualFold(h.key, k) {
			return true
		}
	}
	return false
}

func prepare(r *parser.Request, resolve func(string) string) (*prepared, error) {
	rs := func(s string) string {
		if resolve != nil {
			return resolve(s)
		}
		return s
	}
	p := &prepared{method: strings.ToUpper(r.Method), url: rs(r.URL)}
	if p.method == "" {
		p.method = "GET"
	}

	for _, h := range r.Headers {
		p.headers = append(p.headers, header{rs(h.Key), rs(h.Value)})
	}

	// Query parameters from `? key = value` lines.
	if len(r.QueryParams) > 0 {
		if u, err := url.Parse(p.url); err == nil {
			q := u.Query()
			for _, qp := range r.QueryParams {
				q.Add(rs(qp.Key), rs(qp.Value))
			}
			u.RawQuery = q.Encode()
			p.url = u.String()
		}
	}

	if r.Metadata != nil && r.Metadata.Auth != nil {
		a := r.Metadata.Auth
		param := func(i int) string {
			if i < len(a.Params) {
				return rs(a.Params[i])
			}
			return ""
		}
		switch a.Type {
		case parser.AuthBearer:
			if !p.hasHeader("Authorization") {
				p.headers = append(p.headers, header{"Authorization", "Bearer " + param(0)})
			}
		case parser.AuthBasic:
			user, pass := param(0), param(1)
			if strings.Contains(user+pass, "{{") {
				p.notes = append(p.notes, "@auth basic credentials contain unresolved variables; add the Authorization header yourself")
			} else if !p.hasHeader("Authorization") {
				p.headers = append(p.headers, header{"Authorization", "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))})
			}
		case parser.AuthAPIKey:
			p.headers = append(p.headers, header{param(0), param(1)})
		case parser.AuthAPIKeyQuery:
			if u, err := url.Parse(p.url); err == nil {
				q := u.Query()
				q.Add(param(0), param(1))
				u.RawQuery = q.Encode()
				p.url = u.String()
			}
		case parser.AuthNone:
		default:
			p.notes = append(p.notes, "the @auth annotation on this request is not exported; add the credentials yourself")
		}
	}

	if b := r.Body; b != nil {
		switch b.ContentType {
		case parser.BodyNone:
		case parser.BodyMultipart:
			return nil, fmt.Errorf("multipart bodies are only supported by the curl format")
		case parser.BodyFile:
			return nil, fmt.Errorf("file bodies (< %s) are only supported by the curl format", b.FilePath)
		case parser.BodyGraphQL:
			if b.GraphQL != nil {
				payload := map[string]any{"query": rs(b.GraphQL.Query)}
				if v := strings.TrimSpace(b.GraphQL.Variables); v != "" {
					var parsed any
					if json.Unmarshal([]byte(rs(v)), &parsed) == nil {
						payload["variables"] = parsed
					} else {
						payload["variables"] = rs(v)
					}
				}
				enc, err := json.Marshal(payload)
				if err != nil {
					return nil, err
				}
				p.body = string(enc)
				if !p.hasHeader("Content-Type") {
					p.headers = append(p.headers, header{"Content-Type", "application/json"})
				}
			}
		default:
			p.body = strings.TrimSpace(rs(b.Raw))
			if p.body != "" && !p.hasHeader("Content-Type") {
				switch b.ContentType {
				case parser.BodyJSON:
					p.headers = append(p.headers, header{"Content-Type", "application/json"})
				case parser.BodyForm, parser.BodyFormBlock:
					p.headers = append(p.headers, header{"Content-Type", "application/x-www-form-urlencoded"})
				}
			}
		}
	}
	return p, nil
}

// shellQuote wraps s in single quotes, safely escaping embedded single quotes,
// for shell-based snippets (httpie, wget).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func fetchSnippet(p *prepared) string {
	var b strings.Builder
	fmt.Fprintf(&b, "fetch(%s, {\n  method: %s", strconv.Quote(p.url), strconv.Quote(p.method))
	if len(p.headers) > 0 {
		b.WriteString(",\n  headers: {")
		for i, h := range p.headers {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, "\n    %s: %s", strconv.Quote(h.key), strconv.Quote(h.value))
		}
		b.WriteString("\n  }")
	}
	if p.body != "" {
		fmt.Fprintf(&b, ",\n  body: %s", strconv.Quote(p.body))
	}
	b.WriteString("\n})")
	return b.String()
}

func pythonSnippet(p *prepared) string {
	var b strings.Builder
	b.WriteString("import requests\n\n")
	if len(p.headers) > 0 {
		b.WriteString("headers = {\n")
		for _, h := range p.headers {
			fmt.Fprintf(&b, "    %s: %s,\n", strconv.Quote(h.key), strconv.Quote(h.value))
		}
		b.WriteString("}\n")
	}
	if p.body != "" {
		fmt.Fprintf(&b, "data = %s\n", strconv.Quote(p.body))
	}
	fmt.Fprintf(&b, "response = requests.request(%s, %s", strconv.Quote(p.method), strconv.Quote(p.url))
	if len(p.headers) > 0 {
		b.WriteString(", headers=headers")
	}
	if p.body != "" {
		b.WriteString(", data=data")
	}
	b.WriteString(")")
	return b.String()
}

func httpieSnippet(p *prepared) string {
	var b strings.Builder
	if p.body != "" {
		fmt.Fprintf(&b, "echo %s | ", shellQuote(p.body))
	}
	fmt.Fprintf(&b, "http %s %s", p.method, shellQuote(p.url))
	for _, h := range p.headers {
		fmt.Fprintf(&b, " %s", shellQuote(h.key+":"+h.value))
	}
	return b.String()
}

func goSnippet(p *prepared) string {
	var b strings.Builder
	if p.body != "" {
		bodyLit := "`" + p.body + "`"
		if strings.Contains(p.body, "`") {
			bodyLit = strconv.Quote(p.body)
		}
		fmt.Fprintf(&b, "req, _ := http.NewRequest(%s, %s, strings.NewReader(%s))\n", strconv.Quote(p.method), strconv.Quote(p.url), bodyLit)
	} else {
		fmt.Fprintf(&b, "req, _ := http.NewRequest(%s, %s, nil)\n", strconv.Quote(p.method), strconv.Quote(p.url))
	}
	for _, h := range p.headers {
		fmt.Fprintf(&b, "req.Header.Set(%s, %s)\n", strconv.Quote(h.key), strconv.Quote(h.value))
	}
	b.WriteString("resp, _ := http.DefaultClient.Do(req)\ndefer resp.Body.Close()")
	return b.String()
}

func rubySnippet(p *prepared) string {
	var b strings.Builder
	b.WriteString("require 'net/http'\nrequire 'uri'\n\n")
	fmt.Fprintf(&b, "uri = URI(%s)\n", strconv.Quote(p.url))
	fmt.Fprintf(&b, "req = Net::HTTP::%s.new(uri)\n", rubyMethodClass(p.method))
	for _, h := range p.headers {
		fmt.Fprintf(&b, "req[%s] = %s\n", strconv.Quote(h.key), strconv.Quote(h.value))
	}
	if p.body != "" {
		fmt.Fprintf(&b, "req.body = %s\n", strconv.Quote(p.body))
	}
	b.WriteString("res = Net::HTTP.start(uri.hostname, uri.port, use_ssl: uri.scheme == 'https') { |http| http.request(req) }")
	return b.String()
}

func wgetSnippet(p *prepared) string {
	var b strings.Builder
	fmt.Fprintf(&b, "wget --method=%s", p.method)
	for _, h := range p.headers {
		fmt.Fprintf(&b, " --header=%s", shellQuote(h.key+": "+h.value))
	}
	if p.body != "" {
		fmt.Fprintf(&b, " --body-data=%s", shellQuote(p.body))
	}
	fmt.Fprintf(&b, " %s", shellQuote(p.url))
	return b.String()
}

// rubyMethodClass title-cases an HTTP method for Ruby's Net::HTTP class names
// (get -> Get, post -> Post).
func rubyMethodClass(method string) string {
	m := strings.ToLower(method)
	if m == "" {
		return m
	}
	return strings.ToUpper(m[:1]) + m[1:]
}
