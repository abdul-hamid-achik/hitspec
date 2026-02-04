// Package curl provides functionality to export hitspec requests as curl commands.
package curl

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

// Exporter converts hitspec requests to curl commands.
type Exporter struct {
	resolver func(string) string
	verbose  bool
}

// Option configures the Exporter.
type Option func(*Exporter)

// WithResolver sets a variable resolver function for interpolating {{variables}}.
func WithResolver(r func(string) string) Option {
	return func(e *Exporter) {
		e.resolver = r
	}
}

// WithVerbose enables the -v flag in curl output.
func WithVerbose(v bool) Option {
	return func(e *Exporter) {
		e.verbose = v
	}
}

// New creates a new Exporter with the given options.
func New(opts ...Option) *Exporter {
	e := &Exporter{}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Export converts a single request to a curl command string.
func (e *Exporter) Export(req *parser.Request) string {
	var parts []string

	parts = append(parts, "curl")

	// Verbose flag
	if e.verbose {
		parts = append(parts, "-v")
	}

	// Method
	method := strings.ToUpper(req.Method)
	if method == "" {
		method = "GET"
	}
	parts = append(parts, "-X", method)

	// URL with query parameters
	urlStr := e.resolve(req.URL)
	if len(req.QueryParams) > 0 {
		parsedURL, err := url.Parse(urlStr)
		if err == nil {
			q := parsedURL.Query()
			for _, qp := range req.QueryParams {
				q.Add(e.resolve(qp.Key), e.resolve(qp.Value))
			}
			parsedURL.RawQuery = q.Encode()
			urlStr = parsedURL.String()
		}
	}
	parts = append(parts, quote(urlStr))

	// Auth handling (before headers to avoid duplicates)
	authHeader := ""
	if req.Metadata != nil && req.Metadata.Auth != nil {
		authParts := e.buildAuth(req.Metadata.Auth)
		if len(authParts) > 0 {
			// Check if it's a header-based auth
			if authParts[0] == "-H" && len(authParts) > 1 {
				authHeader = authParts[1]
			}
			parts = append(parts, authParts...)
		}
	}

	// Headers
	for _, h := range req.Headers {
		key := e.resolve(h.Key)
		value := e.resolve(h.Value)
		headerStr := fmt.Sprintf("%s: %s", key, value)
		// Skip Authorization header if already set by auth
		if authHeader != "" && strings.HasPrefix(strings.ToLower(key), "authorization") {
			continue
		}
		parts = append(parts, "-H", quote(headerStr))
	}

	// Body
	if req.Body != nil {
		bodyParts := e.buildBody(req.Body)
		parts = append(parts, bodyParts...)
	}

	return strings.Join(parts, " ")
}

// ExportAll converts multiple requests to curl commands.
func (e *Exporter) ExportAll(reqs []*parser.Request) []string {
	result := make([]string, 0, len(reqs))
	for _, req := range reqs {
		result = append(result, e.Export(req))
	}
	return result
}

// ExportFormatted returns a formatted string with all requests, including comments.
func (e *Exporter) ExportFormatted(reqs []*parser.Request) string {
	var sb strings.Builder

	for i, req := range reqs {
		if i > 0 {
			sb.WriteString("\n")
		}

		// Request name as comment
		name := req.Name
		if name == "" {
			name = fmt.Sprintf("%s %s", req.Method, req.URL)
		}
		sb.WriteString("# Request: ")
		sb.WriteString(name)
		sb.WriteString("\n")

		// The curl command
		sb.WriteString(e.Export(req))
		sb.WriteString("\n")
	}

	return sb.String()
}

// buildAuth converts hitspec auth config to curl flags.
func (e *Exporter) buildAuth(auth *parser.AuthConfig) []string {
	if auth == nil || auth.Type == parser.AuthNone {
		return nil
	}

	switch auth.Type {
	case parser.AuthBasic:
		// @auth basic user pass -> -u 'user:pass'
		if len(auth.Params) >= 2 {
			user := e.resolve(auth.Params[0])
			pass := e.resolve(auth.Params[1])
			return []string{"-u", quote(user + ":" + pass)}
		}
	case parser.AuthBearer:
		// @auth bearer token -> -H 'Authorization: Bearer token'
		if len(auth.Params) >= 1 {
			token := e.resolve(auth.Params[0])
			return []string{"-H", quote("Authorization: Bearer " + token)}
		}
	case parser.AuthAPIKey:
		// @auth apiKey headerName key -> -H 'headerName: key'
		if len(auth.Params) >= 2 {
			headerName := e.resolve(auth.Params[0])
			key := e.resolve(auth.Params[1])
			return []string{"-H", quote(headerName + ": " + key)}
		}
	case parser.AuthAPIKeyQuery:
		// @auth apiKeyQuery paramName key -> handled in URL, return empty
		return nil
	case parser.AuthDigest:
		// @auth digest user pass -> --digest -u 'user:pass'
		if len(auth.Params) >= 2 {
			user := e.resolve(auth.Params[0])
			pass := e.resolve(auth.Params[1])
			return []string{"--digest", "-u", quote(user + ":" + pass)}
		}
	}

	return nil
}

// buildBody converts hitspec body to curl flags.
func (e *Exporter) buildBody(body *parser.Body) []string {
	if body == nil {
		return nil
	}

	switch body.ContentType {
	case parser.BodyNone:
		return nil

	case parser.BodyJSON, parser.BodyRaw, parser.BodyXML:
		if body.Raw != "" {
			return []string{"-d", quote(e.resolve(body.Raw))}
		}

	case parser.BodyForm, parser.BodyFormBlock:
		if body.Raw != "" {
			return []string{"--data-urlencode", quote(e.resolve(body.Raw))}
		}

	case parser.BodyMultipart:
		var parts []string
		for _, field := range body.Multipart {
			switch field.Type {
			case parser.MultipartFieldValue:
				// -F 'name=value'
				parts = append(parts, "-F", quote(e.resolve(field.Name)+"="+e.resolve(field.Value)))
			case parser.MultipartFieldFile:
				// -F 'name=@filepath'
				parts = append(parts, "-F", quote(e.resolve(field.Name)+"=@"+e.resolve(field.Path)))
			}
		}
		return parts

	case parser.BodyGraphQL:
		if body.GraphQL != nil {
			// Convert GraphQL to JSON body
			gqlBody := map[string]string{
				"query": body.GraphQL.Query,
			}
			if body.GraphQL.Variables != "" {
				gqlBody["variables"] = body.GraphQL.Variables
			}
			jsonBytes, err := json.Marshal(gqlBody)
			if err == nil {
				return []string{"-d", quote(string(jsonBytes))}
			}
		}
	}

	return nil
}

// resolve applies the resolver function if set, otherwise returns input unchanged.
func (e *Exporter) resolve(input string) string {
	if e.resolver != nil {
		return e.resolver(input)
	}
	return input
}

// quote wraps a string in single quotes, escaping internal single quotes.
func quote(s string) string {
	// Escape single quotes by ending the string, adding escaped quote, starting string again
	escaped := strings.ReplaceAll(s, "'", "'\"'\"'")
	return "'" + escaped + "'"
}
