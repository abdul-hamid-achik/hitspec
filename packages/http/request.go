package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
)

type Request struct {
	Method      string
	URL         string
	Headers     map[string]string
	Body        string
	Timeout     time.Duration
	Auth        *parser.AuthConfig
	QueryParams []QueryParam // appended to URL in order by BuildURL
	Multipart   []*parser.MultipartField
	BaseDir     string // Base directory for resolving relative file paths
	DigestAuth  *DigestAuthCredentials
	AWSAuth     *AWSAuthCredentials
	OAuth2Auth  *OAuth2AuthCredentials

	// Err is set when the request could not be built from its source (for
	// example a "< ./body.json" file that cannot be read). The client returns
	// it instead of sending a request with the wrong body.
	Err error
}

// QueryParam is one name/value pair of a request's query string.
type QueryParam struct {
	Key, Value string
}

// OAuth2AuthCredentials holds OAuth2 authentication configuration
type OAuth2AuthCredentials struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	GrantType    string // "client_credentials" or "password"
	Username     string // For password grant
	Password     string // For password grant
}

// AWSAuthCredentials holds credentials for AWS Signature v4 authentication
type AWSAuthCredentials struct {
	AccessKey string
	SecretKey string
	Region    string
	Service   string
	// SessionToken is the optional temporary-credentials token sent as
	// X-Amz-Security-Token. When empty and AccessKey equals
	// $AWS_ACCESS_KEY_ID, $AWS_SESSION_TOKEN is used.
	SessionToken string
}

func NewRequest(method, requestURL string) *Request {
	return &Request{
		Method:  method,
		URL:     requestURL,
		Headers: make(map[string]string),
	}
}

func (r *Request) SetHeader(key, value string) *Request {
	r.Headers[key] = value
	return r
}

func (r *Request) SetBody(body string) *Request {
	r.Body = body
	return r
}

func (r *Request) SetTimeout(d time.Duration) *Request {
	r.Timeout = d
	return r
}

// SetQueryParam sets a query parameter: it replaces the value of an existing
// parameter with that name (dropping any repeats) or appends a new one.
func (r *Request) SetQueryParam(key, value string) *Request {
	out := r.QueryParams[:0:0]
	replaced := false
	for _, q := range r.QueryParams {
		if q.Key != key {
			out = append(out, q)
			continue
		}
		if !replaced {
			out = append(out, QueryParam{Key: key, Value: value})
			replaced = true
		}
	}
	if !replaced {
		out = append(out, QueryParam{Key: key, Value: value})
	}
	r.QueryParams = out
	return r
}

// AddQueryParam appends a query parameter, keeping earlier ones with the same
// name.
func (r *Request) AddQueryParam(key, value string) *Request {
	r.QueryParams = append(r.QueryParams, QueryParam{Key: key, Value: value})
	return r
}

// BuildURL returns URL with QueryParams appended, in order and with repeated
// names kept. Names and values are percent-encoded (a space is %20). Characters
// that cannot appear in a URL (spaces, quotes, non-ASCII, ...) in URL itself are
// percent-encoded too; existing %XX escapes are left alone. The query already in
// URL keeps its order.
func (r *Request) BuildURL() string {
	u := escapeURL(r.URL)
	if len(r.QueryParams) == 0 {
		return u
	}
	fragment := ""
	if i := strings.IndexByte(u, '#'); i >= 0 {
		u, fragment = u[:i], u[i:]
	}
	sep := "&"
	switch {
	case !strings.Contains(u, "?"):
		sep = "?"
	case strings.HasSuffix(u, "?"), strings.HasSuffix(u, "&"):
		sep = ""
	}
	var sb strings.Builder
	sb.WriteString(u)
	for _, q := range r.QueryParams {
		sb.WriteString(sep)
		sb.WriteString(rfc3986Escape(q.Key))
		sb.WriteByte('=')
		sb.WriteString(rfc3986Escape(q.Value))
		sep = "&"
	}
	sb.WriteString(fragment)
	return sb.String()
}

// dropURLQueryParam removes every parameter called key from the query string of
// rawURL, leaving the others in order.
func dropURLQueryParam(rawURL, key string) string {
	rest, fragment := rawURL, ""
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		rest, fragment = rest[:i], rest[i:]
	}
	base, query, ok := strings.Cut(rest, "?")
	if !ok {
		return rawURL
	}
	var kept []string
	for _, pair := range strings.Split(query, "&") {
		name, _, _ := strings.Cut(pair, "=")
		if decoded, err := url.QueryUnescape(name); err == nil {
			name = decoded
		}
		if name != key {
			kept = append(kept, pair)
		}
	}
	if len(kept) == 0 {
		return base + fragment
	}
	return base + "?" + strings.Join(kept, "&") + fragment
}

// escapeURL percent-encodes the bytes of the path, query and fragment of a URL
// that must not be sent as they are: spaces, control characters, non-ASCII
// bytes, quotes, angle brackets, a "%" that does not start an escape, and so on.
// Reserved characters and existing %XX escapes are kept. The scheme and
// authority are left to the URL validation that follows.
func escapeURL(raw string) string {
	i := strings.Index(raw, "://")
	if i < 0 {
		return raw
	}
	start := i + 3
	j := strings.IndexAny(raw[start:], "/?#")
	if j < 0 {
		return raw
	}
	start += j

	const hexDigits = "0123456789ABCDEF"
	var sb strings.Builder
	sb.WriteString(raw[:start])
	rest := raw[start:]
	for k := 0; k < len(rest); k++ {
		c := rest[k]
		switch {
		case c == '%' && k+2 < len(rest) && isHex(rest[k+1]) && isHex(rest[k+2]):
			sb.WriteByte(c)
		case c > ' ' && c < 0x7f && !strings.ContainsRune("\"%<>\\^`{|}", rune(c)):
			sb.WriteByte(c)
		default:
			sb.WriteByte('%')
			sb.WriteByte(hexDigits[c>>4])
			sb.WriteByte(hexDigits[c&15])
		}
	}
	return sb.String()
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func (r *Request) ApplyAuth() {
	if r.Auth == nil {
		return
	}

	switch r.Auth.Type {
	case parser.AuthBasic:
		if len(r.Auth.Params) >= 2 {
			creds := r.Auth.Params[0] + ":" + r.Auth.Params[1]
			encoded := base64.StdEncoding.EncodeToString([]byte(creds))
			r.Headers["Authorization"] = "Basic " + encoded
		}
	case parser.AuthBearer:
		if len(r.Auth.Params) >= 1 {
			r.Headers["Authorization"] = "Bearer " + r.Auth.Params[0]
		}
	case parser.AuthAPIKey:
		if len(r.Auth.Params) >= 2 {
			r.Headers[r.Auth.Params[0]] = r.Auth.Params[1]
		}
	case parser.AuthAPIKeyQuery:
		if len(r.Auth.Params) >= 2 {
			// The credential replaces a parameter of the same name that is
			// already in the URL, rather than being sent next to it.
			r.URL = dropURLQueryParam(r.URL, r.Auth.Params[0])
			r.SetQueryParam(r.Auth.Params[0], r.Auth.Params[1])
		}
	case parser.AuthDigest:
		// Digest auth requires challenge-response, handled by the client
		if len(r.Auth.Params) >= 2 {
			r.DigestAuth = &DigestAuthCredentials{
				Username: r.Auth.Params[0],
				Password: r.Auth.Params[1],
			}
		}
	case parser.AuthAWS:
		// AWS Signature v4 auth
		if len(r.Auth.Params) >= 4 {
			r.AWSAuth = &AWSAuthCredentials{
				AccessKey: r.Auth.Params[0],
				SecretKey: r.Auth.Params[1],
				Region:    r.Auth.Params[2],
				Service:   r.Auth.Params[3],
			}
			if len(r.Auth.Params) >= 5 {
				r.AWSAuth.SessionToken = r.Auth.Params[4]
			}
		}
	case parser.AuthOAuth2ClientCredentials:
		// OAuth2 client_credentials grant
		// Params: tokenUrl, clientId, clientSecret, [scopes]
		if len(r.Auth.Params) >= 3 {
			r.OAuth2Auth = &OAuth2AuthCredentials{
				TokenURL:     r.Auth.Params[0],
				ClientID:     r.Auth.Params[1],
				ClientSecret: r.Auth.Params[2],
				GrantType:    "client_credentials",
			}
			if len(r.Auth.Params) >= 4 {
				r.OAuth2Auth.Scopes = strings.Split(r.Auth.Params[3], ",")
			}
		}
	case parser.AuthOAuth2Password:
		// OAuth2 password grant
		// Params: tokenUrl, clientId, clientSecret, username, password, [scopes]
		if len(r.Auth.Params) >= 5 {
			r.OAuth2Auth = &OAuth2AuthCredentials{
				TokenURL:     r.Auth.Params[0],
				ClientID:     r.Auth.Params[1],
				ClientSecret: r.Auth.Params[2],
				Username:     r.Auth.Params[3],
				Password:     r.Auth.Params[4],
				GrantType:    "password",
			}
			if len(r.Auth.Params) >= 6 {
				r.OAuth2Auth.Scopes = strings.Split(r.Auth.Params[5], ",")
			}
		}
	}
}

func BuildRequestFromAST(req *parser.Request, resolver func(string) string) *Request {
	return BuildRequestFromASTWithBaseDir(req, resolver, "")
}

func BuildRequestFromASTWithBaseDir(req *parser.Request, resolver func(string) string, baseDir string) *Request {
	r := NewRequest(req.Method, resolver(req.URL))
	r.BaseDir = baseDir

	for _, h := range req.Headers {
		r.SetHeader(h.Key, resolver(h.Value))
	}

	for _, qp := range req.QueryParams {
		r.AddQueryParam(resolver(qp.Key), resolver(qp.Value))
	}

	if req.Body != nil {
		if req.Body.ContentType == parser.BodyFile && req.Body.FilePath != "" {
			// Resolve file path relative to the .http file's directory
			resolvedPath := resolver(req.Body.FilePath)
			if !filepath.IsAbs(resolvedPath) && baseDir != "" {
				resolvedPath = filepath.Join(baseDir, resolvedPath)
			}
			data, err := os.ReadFile(resolvedPath)
			if err != nil {
				// Sending the literal "< ./file" text would only hide the
				// problem behind a confusing server error.
				r.Err = fmt.Errorf("request body file %q: %w", req.Body.FilePath, err)
			} else {
				body := string(data)
				r.SetBody(body)
				// Auto-detect content type from file extension
				ext := strings.ToLower(filepath.Ext(resolvedPath))
				if r.Headers["Content-Type"] == "" {
					switch ext {
					case ".json":
						r.SetHeader("Content-Type", "application/json")
					case ".xml":
						r.SetHeader("Content-Type", "application/xml")
					case ".html", ".htm":
						r.SetHeader("Content-Type", "text/html")
					case ".txt":
						r.SetHeader("Content-Type", "text/plain")
					case ".csv":
						r.SetHeader("Content-Type", "text/csv")
					case ".yaml", ".yml":
						r.SetHeader("Content-Type", "application/yaml")
					}
				}
			}
		} else if req.Body.ContentType == parser.BodyMultipart && len(req.Body.Multipart) > 0 {
			// Handle multipart form data
			resolvedFields := make([]*parser.MultipartField, len(req.Body.Multipart))
			for i, field := range req.Body.Multipart {
				resolvedFields[i] = &parser.MultipartField{
					Type:  field.Type,
					Name:  resolver(field.Name),
					Value: resolver(field.Value),
					Path:  resolver(field.Path),
				}
			}
			r.Multipart = resolvedFields
			// Content-Type will be set by the client when building the multipart body
		} else if req.Body.ContentType == parser.BodyGraphQL && req.Body.GraphQL != nil {
			// Build JSON body from GraphQL query and optional variables
			gqlPayload := map[string]interface{}{
				"query": resolver(req.Body.GraphQL.Query),
			}
			if vars := strings.TrimSpace(req.Body.GraphQL.Variables); vars != "" {
				var parsed interface{}
				if err := json.Unmarshal([]byte(resolver(vars)), &parsed); err == nil {
					gqlPayload["variables"] = parsed
				} else {
					gqlPayload["variables"] = resolver(vars)
				}
			}
			if encoded, err := json.Marshal(gqlPayload); err == nil {
				r.SetBody(string(encoded))
			}
			if r.Headers["Content-Type"] == "" {
				r.SetHeader("Content-Type", "application/json")
			}
		} else if req.Body.ContentType == parser.BodyFormBlock && len(req.Body.FormFields) > 0 {
			// "& key = value" lines: encode each resolved name and value so
			// spaces, "&" and "=" inside a value cannot change the form.
			pairs := make([]string, len(req.Body.FormFields))
			for i, f := range req.Body.FormFields {
				pairs[i] = rfc3986Escape(resolver(f.Key)) + "=" + rfc3986Escape(resolver(f.Value))
			}
			r.SetBody(strings.Join(pairs, "&"))
			if r.Headers["Content-Type"] == "" {
				r.SetHeader("Content-Type", "application/x-www-form-urlencoded")
			}
		} else {
			body := resolver(req.Body.Raw)
			r.SetBody(body)

			if req.Body.ContentType == parser.BodyJSON && r.Headers["Content-Type"] == "" {
				r.SetHeader("Content-Type", "application/json")
			} else if (req.Body.ContentType == parser.BodyForm || req.Body.ContentType == parser.BodyFormBlock) && r.Headers["Content-Type"] == "" {
				r.SetHeader("Content-Type", "application/x-www-form-urlencoded")
			}
		}
	}

	if req.Metadata != nil && req.Metadata.Auth != nil {
		auth := &parser.AuthConfig{
			Type:   req.Metadata.Auth.Type,
			Params: make([]string, len(req.Metadata.Auth.Params)),
		}
		for i, p := range req.Metadata.Auth.Params {
			auth.Params[i] = resolver(p)
		}
		r.Auth = auth
		r.ApplyAuth()
	}

	if req.Metadata != nil && req.Metadata.Timeout > 0 {
		r.SetTimeout(time.Duration(req.Metadata.Timeout) * time.Millisecond)
	}

	r.URL = r.BuildURL()
	r.QueryParams = nil // folded into URL; a second BuildURL must not repeat them

	return r
}

func ParseFormBody(body string) map[string]string {
	result := make(map[string]string)
	pairs := strings.Split(body, "&")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			key, _ := url.QueryUnescape(kv[0])
			value, _ := url.QueryUnescape(kv[1])
			result[key] = value
		}
	}
	return result
}
