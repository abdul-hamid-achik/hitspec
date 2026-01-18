package http

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

type Client struct {
	httpClient     *http.Client
	timeout        time.Duration
	followRedirect bool
	maxRedirects   int
	validateSSL    bool
	defaultHeaders map[string]string
}

// DigestAuthCredentials holds credentials for digest auth
type DigestAuthCredentials struct {
	Username string
	Password string
}

type ClientOption func(*Client)

func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		timeout:        30 * time.Second,
		followRedirect: true,
		maxRedirects:   10,
		validateSSL:    true,
		defaultHeaders: make(map[string]string),
	}

	for _, opt := range opts {
		opt(c)
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	redirectPolicy := func(req *http.Request, via []*http.Request) error {
		if !c.followRedirect {
			return http.ErrUseLastResponse
		}
		if len(via) >= c.maxRedirects {
			return http.ErrUseLastResponse
		}
		return nil
	}

	c.httpClient = &http.Client{
		Transport:     transport,
		Timeout:       c.timeout,
		CheckRedirect: redirectPolicy,
	}

	return c
}

func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = d
	}
}

func WithFollowRedirects(follow bool) ClientOption {
	return func(c *Client) {
		c.followRedirect = follow
	}
}

func WithMaxRedirects(max int) ClientOption {
	return func(c *Client) {
		c.maxRedirects = max
	}
}

func WithDefaultHeader(key, value string) ClientOption {
	return func(c *Client) {
		c.defaultHeaders[key] = value
	}
}

func (c *Client) Do(req *Request) (*Response, error) {
	ctx := context.Background()
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	// Handle digest auth - requires challenge-response
	if req.DigestAuth != nil {
		return c.doWithDigestAuth(ctx, req)
	}

	// Handle AWS Signature auth
	if req.AWSAuth != nil {
		return c.doWithAWSAuth(ctx, req)
	}

	return c.doRequest(ctx, req, "")
}

func (c *Client) doRequest(ctx context.Context, req *Request, authHeader string) (*Response, error) {
	var body io.Reader
	var contentType string

	// Check if this is a multipart request
	if len(req.Multipart) > 0 {
		multipartBody, ct, err := BuildMultipartBody(req.Multipart, req.BaseDir)
		if err != nil {
			return nil, err
		}
		body = multipartBody
		contentType = ct
	} else if req.Body != "" {
		body = bytes.NewBufferString(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return nil, err
	}

	for k, v := range c.defaultHeaders {
		httpReq.Header.Set(k, v)
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Set multipart content type if present (must be after headers to override)
	if contentType != "" {
		httpReq.Header.Set("Content-Type", contentType)
	}

	// Set auth header if provided (for digest auth retry)
	if authHeader != "" {
		httpReq.Header.Set("Authorization", authHeader)
	}

	start := time.Now()
	httpResp, err := c.httpClient.Do(httpReq)
	duration := time.Since(start)

	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string)
	for k := range httpResp.Header {
		headers[k] = httpResp.Header.Get(k)
	}

	return &Response{
		StatusCode: httpResp.StatusCode,
		Status:     httpResp.Status,
		Headers:    headers,
		Body:       respBody,
		Duration:   duration,
	}, nil
}

func (c *Client) doWithDigestAuth(ctx context.Context, req *Request) (*Response, error) {
	// First request without auth to get the challenge
	resp, err := c.doRequest(ctx, req, "")
	if err != nil {
		return nil, err
	}

	// If not 401, return the response as-is
	if resp.StatusCode != 401 {
		return resp, nil
	}

	// Parse WWW-Authenticate header
	wwwAuth := resp.Header("WWW-Authenticate")
	if wwwAuth == "" {
		return resp, nil // Return original response if no challenge
	}

	params := ParseWWWAuthenticate(wwwAuth)

	// Build digest auth
	auth := &DigestAuth{
		Username: req.DigestAuth.Username,
		Password: req.DigestAuth.Password,
		Realm:    params["realm"],
		Nonce:    params["nonce"],
		URI:      req.URL,
		Qop:      params["qop"],
		Opaque:   params["opaque"],
		Method:   req.Method,
	}

	// Parse the URL to get just the path for the URI
	parsedURL, err := http.NewRequest("GET", req.URL, nil)
	if err == nil {
		auth.URI = parsedURL.URL.RequestURI()
	}

	if auth.Qop != "" {
		auth.Nc = "00000001"
		cnonce, err := GenerateCnonce()
		if err != nil {
			return nil, err
		}
		auth.Cnonce = cnonce
		// Prefer "auth" qop
		if strings.Contains(auth.Qop, "auth") {
			auth.Qop = "auth"
		}
	}

	authHeader := auth.BuildAuthorizationHeader()

	// Retry with authorization
	return c.doRequest(ctx, req, authHeader)
}

func (c *Client) doWithAWSAuth(ctx context.Context, req *Request) (*Response, error) {
	// Sign the request with AWS Signature v4
	authHeader, err := SignAWSRequest(req)
	if err != nil {
		return nil, err
	}

	return c.doRequest(ctx, req, authHeader)
}

func (c *Client) Get(url string, headers map[string]string) (*Response, error) {
	return c.Do(&Request{
		Method:  "GET",
		URL:     url,
		Headers: headers,
	})
}

func (c *Client) Post(url, body string, headers map[string]string) (*Response, error) {
	return c.Do(&Request{
		Method:  "POST",
		URL:     url,
		Body:    body,
		Headers: headers,
	})
}

func (c *Client) Put(url, body string, headers map[string]string) (*Response, error) {
	return c.Do(&Request{
		Method:  "PUT",
		URL:     url,
		Body:    body,
		Headers: headers,
	})
}

func (c *Client) Patch(url, body string, headers map[string]string) (*Response, error) {
	return c.Do(&Request{
		Method:  "PATCH",
		URL:     url,
		Body:    body,
		Headers: headers,
	})
}

func (c *Client) Delete(url string, headers map[string]string) (*Response, error) {
	return c.Do(&Request{
		Method:  "DELETE",
		URL:     url,
		Headers: headers,
	})
}

// BuildMultipartBody creates a multipart form data body from multipart fields
func BuildMultipartBody(fields []*parser.MultipartField, baseDir string) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for _, field := range fields {
		if field.Type == parser.MultipartFieldFile {
			// Resolve file path relative to base directory
			filePath := field.Path
			if !filepath.IsAbs(filePath) && baseDir != "" {
				filePath = filepath.Join(baseDir, filePath)
			}

			file, err := os.Open(filePath)
			if err != nil {
				return nil, "", err
			}

			part, err := writer.CreateFormFile(field.Name, filepath.Base(filePath))
			if err != nil {
				file.Close()
				return nil, "", err
			}

			_, err = io.Copy(part, file)
			file.Close()
			if err != nil {
				return nil, "", err
			}
		} else {
			// Regular form field
			err := writer.WriteField(field.Name, field.Value)
			if err != nil {
				return nil, "", err
			}
		}
	}

	err := writer.Close()
	if err != nil {
		return nil, "", err
	}

	return body, writer.FormDataContentType(), nil
}
