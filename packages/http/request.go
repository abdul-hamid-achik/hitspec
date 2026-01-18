package http

import (
	"encoding/base64"
	"net/url"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

type Request struct {
	Method      string
	URL         string
	Headers     map[string]string
	Body        string
	Timeout     time.Duration
	Auth        *parser.AuthConfig
	QueryParams map[string]string
	Multipart   []*parser.MultipartField
	BaseDir     string // Base directory for resolving relative file paths
	DigestAuth  *DigestAuthCredentials
	AWSAuth     *AWSAuthCredentials
}

// AWSAuthCredentials holds credentials for AWS Signature v4 authentication
type AWSAuthCredentials struct {
	AccessKey string
	SecretKey string
	Region    string
	Service   string
}

func NewRequest(method, requestURL string) *Request {
	return &Request{
		Method:      method,
		URL:         requestURL,
		Headers:     make(map[string]string),
		QueryParams: make(map[string]string),
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

func (r *Request) SetQueryParam(key, value string) *Request {
	r.QueryParams[key] = value
	return r
}

func (r *Request) BuildURL() string {
	if len(r.QueryParams) == 0 {
		return r.URL
	}

	u, err := url.Parse(r.URL)
	if err != nil {
		return r.URL
	}

	q := u.Query()
	for k, v := range r.QueryParams {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
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
			r.QueryParams[r.Auth.Params[0]] = r.Auth.Params[1]
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
		r.SetQueryParam(qp.Key, resolver(qp.Value))
	}

	if req.Body != nil {
		if req.Body.ContentType == parser.BodyMultipart && len(req.Body.Multipart) > 0 {
			// Handle multipart form data
			resolvedFields := make([]*parser.MultipartField, len(req.Body.Multipart))
			for i, field := range req.Body.Multipart {
				resolvedFields[i] = &parser.MultipartField{
					Type:  field.Type,
					Name:  field.Name,
					Value: resolver(field.Value),
					Path:  resolver(field.Path),
				}
			}
			r.Multipart = resolvedFields
			// Content-Type will be set by the client when building the multipart body
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
