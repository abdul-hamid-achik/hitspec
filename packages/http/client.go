package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

const (
	// DefaultTimeout is the default HTTP request timeout
	DefaultTimeout = 30 * time.Second
	// DefaultMaxRedirects is the maximum number of redirects to follow
	DefaultMaxRedirects = 10
	// DefaultMaxIdleConns is the maximum number of idle connections in the pool
	DefaultMaxIdleConns = 100
	// DefaultMaxIdleConnsPerHost is the maximum number of idle connections per host
	DefaultMaxIdleConnsPerHost = 10
	// DefaultIdleConnTimeout is how long idle connections stay in the pool
	DefaultIdleConnTimeout = 90 * time.Second
)

type Client struct {
	httpClient     *http.Client
	timeout        time.Duration
	followRedirect bool
	maxRedirects   int
	validateSSL    bool
	proxyURL       string
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
		timeout:        DefaultTimeout,
		followRedirect: true,
		maxRedirects:   DefaultMaxRedirects,
		validateSSL:    true,
		defaultHeaders: make(map[string]string),
	}

	for _, opt := range opts {
		opt(c)
	}

	transport := &http.Transport{
		MaxIdleConns:        DefaultMaxIdleConns,
		MaxIdleConnsPerHost: DefaultMaxIdleConnsPerHost,
		IdleConnTimeout:     DefaultIdleConnTimeout,
	}

	// Configure TLS verification
	if !c.validateSSL {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	// Configure proxy if specified
	if c.proxyURL != "" {
		proxyURL, err := neturl.Parse(c.proxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
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

// WithDefaultHeaders sets multiple default headers for all requests
func WithDefaultHeaders(headers map[string]string) ClientOption {
	return func(c *Client) {
		for k, v := range headers {
			c.defaultHeaders[k] = v
		}
	}
}

// WithValidateSSL enables or disables SSL certificate validation
func WithValidateSSL(validate bool) ClientOption {
	return func(c *Client) {
		c.validateSSL = validate
	}
}

// WithProxy sets the proxy URL for all requests
func WithProxy(proxyURL string) ClientOption {
	return func(c *Client) {
		c.proxyURL = proxyURL
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

	// Handle OAuth2 auth
	if req.OAuth2Auth != nil {
		return c.doWithOAuth2Auth(ctx, req)
	}

	return c.doRequest(ctx, req, "")
}

func (c *Client) doRequest(ctx context.Context, req *Request, authHeader string) (*Response, error) {
	// Validate URL before making request
	if err := ValidateURL(req.URL); err != nil {
		return nil, err
	}

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

func (c *Client) doWithOAuth2Auth(ctx context.Context, req *Request) (*Response, error) {
	// Fetch OAuth2 token
	token, err := c.fetchOAuth2Token(req.OAuth2Auth)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth2 token: %w", err)
	}

	// Add Bearer token to Authorization header
	authHeader := "Bearer " + token
	return c.doRequest(ctx, req, authHeader)
}

func (c *Client) fetchOAuth2Token(auth *OAuth2AuthCredentials) (string, error) {
	// Build token request
	data := neturl.Values{}
	data.Set("grant_type", auth.GrantType)

	if auth.GrantType == "password" {
		data.Set("username", auth.Username)
		data.Set("password", auth.Password)
	}

	if len(auth.Scopes) > 0 {
		data.Set("scope", strings.Join(auth.Scopes, " "))
	}

	tokenReq, err := http.NewRequest("POST", auth.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Add client credentials via Basic auth
	if auth.ClientID != "" && auth.ClientSecret != "" {
		tokenReq.SetBasicAuth(auth.ClientID, auth.ClientSecret)
	}

	resp, err := c.httpClient.Do(tokenReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse token response - simple extraction of access_token
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	// Simple JSON parsing for access_token
	// Using strings to avoid importing encoding/json in this file
	accessTokenStart := strings.Index(string(body), `"access_token"`)
	if accessTokenStart == -1 {
		return "", fmt.Errorf("no access_token in response: %s", string(body))
	}

	// Find the value after "access_token": "
	valueStart := strings.Index(string(body)[accessTokenStart:], `"`) + accessTokenStart + 1
	valueStart = strings.Index(string(body)[valueStart:], `"`) + valueStart + 1
	valueEnd := strings.Index(string(body)[valueStart:], `"`) + valueStart

	if valueEnd <= valueStart {
		return "", fmt.Errorf("invalid token response format: %s", string(body))
	}

	tokenResp.AccessToken = string(body)[valueStart:valueEnd]
	return tokenResp.AccessToken, nil
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

// validatePathWithinBase checks that the resolved path stays within the base directory
// to prevent path traversal attacks
func validatePathWithinBase(path, baseDir string) error {
	if baseDir == "" {
		return nil
	}

	// Clean and resolve both paths
	cleanBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("failed to resolve base directory: %v", err)
	}

	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %v", err)
	}

	// Ensure the path starts with the base directory
	if !strings.HasPrefix(cleanPath, cleanBase+string(filepath.Separator)) && cleanPath != cleanBase {
		return fmt.Errorf("path traversal detected: %s is outside allowed directory %s", path, baseDir)
	}

	return nil
}

// ValidateURL checks that a URL is well-formed and uses an allowed scheme
func ValidateURL(rawURL string) error {
	u, err := neturl.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}

	// Check for valid scheme
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s (only http and https are allowed)", u.Scheme)
	}

	// Check for valid host
	if u.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	return nil
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

			// Validate path doesn't escape base directory (prevent path traversal)
			if err := validatePathWithinBase(filePath, baseDir); err != nil {
				return nil, "", err
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
