package http

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptrace"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/http/httpproxy"
	"golang.org/x/net/publicsuffix"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/pathutil"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/auth/oauth2"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
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
	// DefaultMaxBodyBytes bounds a response body (raw and decoded) when no
	// explicit limit is configured. Pass a negative value to WithMaxBodyBytes
	// for no limit.
	DefaultMaxBodyBytes int64 = 100 << 20
)

// sensitiveRedirectKeep lists request headers that are forwarded on a
// cross-origin redirect even though the user set them: they describe the
// payload or content negotiation, not credentials.
var sensitiveRedirectKeep = map[string]bool{
	"Accept": true, "Accept-Encoding": true, "Accept-Language": true,
	"User-Agent": true, "Content-Type": true, "Content-Length": true,
	"Cache-Control": true, "Pragma": true,
}

type Client struct {
	httpClient     *http.Client
	timeout        time.Duration
	followRedirect bool
	maxRedirects   int
	validateSSL    bool
	proxyURL       string
	defaultHeaders map[string]string
	maxBodyBytes   int64

	caCertPath      string
	clientCertPath  string
	clientKeyPath   string
	disableHTTP2    bool
	jar             http.CookieJar
	enableCookies   bool
	forwardRedirect bool // forward user headers on cross-origin redirects
	oauthCache      *oauth2.TokenCache
	configErr       error // deferred option error, returned by Do
}

// DigestAuthCredentials holds credentials for digest auth
type DigestAuthCredentials struct {
	Username string
	Password string
}

type ClientOption func(*Client)

// NewClient builds a Client. Option problems that cannot be reported here
// (an unreadable CA file, a bad proxy URL, ...) are returned by every request;
// use NewClientE to get them up front.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		timeout:        DefaultTimeout,
		followRedirect: true,
		maxRedirects:   DefaultMaxRedirects,
		validateSSL:    true,
		defaultHeaders: make(map[string]string),
		oauthCache:     oauth2.NewTokenCache(),
	}

	for _, opt := range opts {
		opt(c)
	}

	switch {
	case c.maxBodyBytes == 0:
		c.maxBodyBytes = DefaultMaxBodyBytes
	case c.maxBodyBytes < 0:
		c.maxBodyBytes = 0 // explicitly unlimited
	}

	transport := &http.Transport{
		MaxIdleConns:        DefaultMaxIdleConns,
		MaxIdleConnsPerHost: DefaultMaxIdleConnsPerHost,
		IdleConnTimeout:     DefaultIdleConnTimeout,
		DialContext:         (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		// A custom TLSClientConfig disables HTTP/2 unless this is set.
		ForceAttemptHTTP2: !c.disableHTTP2,
	}
	if c.disableHTTP2 {
		transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	}

	tlsCfg, err := c.tlsConfig()
	if err != nil {
		c.setConfigErr(err)
	}
	transport.TLSClientConfig = tlsCfg

	// Proxy: an explicit URL wins; otherwise HTTP_PROXY/HTTPS_PROXY/NO_PROXY.
	if c.proxyURL != "" {
		proxyURL, err := parseProxyURL(c.proxyURL)
		if err != nil {
			c.setConfigErr(err)
		} else {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	} else {
		transport.Proxy = proxyFromEnvironment()
	}

	redirectPolicy := func(req *http.Request, via []*http.Request) error {
		if !c.followRedirect {
			return http.ErrUseLastResponse
		}
		// via holds the requests already sent, so len(via) is the number of
		// this redirect: follow up to maxRedirects of them, like curl --max-redirs.
		if len(via) > c.maxRedirects {
			return http.ErrUseLastResponse
		}
		if !c.forwardRedirect && len(via) > 0 && !sameOrigin(via[0].URL, req.URL) {
			// Go only drops Authorization/Cookie (and only across domains);
			// API keys and other user headers would leak to the new origin.
			for k := range via[0].Header {
				if !sensitiveRedirectKeep[http.CanonicalHeaderKey(k)] {
					req.Header.Del(k)
				}
			}
		}
		return nil
	}

	jar := c.jar
	if jar == nil && c.enableCookies {
		jar, err = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		if err != nil {
			c.setConfigErr(err)
		}
	}
	c.jar = jar

	c.httpClient = &http.Client{
		Transport:     transport,
		Jar:           jar,
		Timeout:       0, // no client-level cap; the per-request context deadline in Do() is the single source of truth
		CheckRedirect: redirectPolicy,
	}

	return c
}

// NewClientE is NewClient that reports invalid options (unreadable or invalid
// CA/client-certificate files, malformed proxy URL) as an error.
func NewClientE(opts ...ClientOption) (*Client, error) {
	c := NewClient(opts...)
	if c.configErr != nil {
		return nil, c.configErr
	}
	return c, nil
}

func (c *Client) setConfigErr(err error) {
	if c.configErr == nil {
		c.configErr = err
	}
}

// tlsConfig builds the TLS settings from the CA / client-certificate /
// validate-SSL options.
func (c *Client) tlsConfig() (*tls.Config, error) {
	cfg := &tls.Config{}
	if !c.validateSSL {
		cfg.InsecureSkipVerify = true
	}
	var err error
	if c.caCertPath != "" {
		pem, rerr := os.ReadFile(c.caCertPath)
		if rerr != nil {
			err = fmt.Errorf("reading CA certificate %q: %w", c.caCertPath, rerr)
		} else {
			pool, perr := x509.SystemCertPool()
			if perr != nil || pool == nil {
				pool = x509.NewCertPool()
			}
			if !pool.AppendCertsFromPEM(pem) {
				err = fmt.Errorf("CA certificate %q contains no valid PEM certificates", c.caCertPath)
			} else {
				cfg.RootCAs = pool
			}
		}
	}
	if err == nil && (c.clientCertPath != "" || c.clientKeyPath != "") {
		switch {
		case c.clientCertPath == "" || c.clientKeyPath == "":
			err = errors.New("client certificate and key must be configured together")
		default:
			cert, lerr := tls.LoadX509KeyPair(c.clientCertPath, c.clientKeyPath)
			if lerr != nil {
				err = fmt.Errorf("loading client certificate %q / key %q: %w", c.clientCertPath, c.clientKeyPath, lerr)
			} else {
				cfg.Certificates = []tls.Certificate{cert}
			}
		}
	}
	return cfg, err
}

// parseProxyURL parses an explicit proxy URL; a bare host:port means http://.
func parseProxyURL(raw string) (*neturl.URL, error) {
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := neturl.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("invalid proxy URL %q", raw)
	}
	return u, nil
}

// proxyFromEnvironment resolves HTTP_PROXY/HTTPS_PROXY/NO_PROXY (and their
// lower-case forms) as of client creation. Loopback destinations are never
// proxied, matching net/http.
func proxyFromEnvironment() func(*http.Request) (*neturl.URL, error) {
	proxy := httpproxy.FromEnvironment().ProxyFunc()
	return func(r *http.Request) (*neturl.URL, error) { return proxy(r.URL) }
}

// sameOrigin reports whether a and b share scheme, host name and port.
func sameOrigin(a, b *neturl.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) &&
		strings.EqualFold(a.Hostname(), b.Hostname()) &&
		effectivePort(a) == effectivePort(b)
}

func effectivePort(u *neturl.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	return "80"
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

// WithMaxBodyBytes bounds response bodies (the bytes on the wire and, for
// gzip/deflate responses, the decoded bytes). n > 0 sets the limit, n < 0
// removes it, and 0 (the default) means DefaultMaxBodyBytes.
func WithMaxBodyBytes(maxBytes int64) ClientOption {
	return func(c *Client) { c.maxBodyBytes = maxBytes }
}

// WithCACert trusts the PEM CA bundle at path in addition to the system roots
// (for private CAs). An unreadable or certificate-less file is an error
// reported by NewClientE and by every request.
func WithCACert(path string) ClientOption {
	return func(c *Client) { c.caCertPath = path }
}

// WithClientCert presents the PEM certificate/key pair for mutual TLS.
func WithClientCert(certPath, keyPath string) ClientOption {
	return func(c *Client) { c.clientCertPath, c.clientKeyPath = certPath, keyPath }
}

// WithHTTP2 enables or disables HTTP/2 negotiation over TLS (default on).
func WithHTTP2(enabled bool) ClientOption {
	return func(c *Client) { c.disableHTTP2 = !enabled }
}

// WithCookieJar makes the client store Set-Cookie responses in jar and send
// them on later requests (also across redirects). Default: no cookie handling.
func WithCookieJar(jar http.CookieJar) ClientOption {
	return func(c *Client) { c.jar = jar }
}

// WithCookies enables a private in-memory cookie jar (public-suffix aware)
// when enabled is true. WithCookieJar takes precedence.
func WithCookies(enabled bool) ClientOption {
	return func(c *Client) { c.enableCookies = enabled }
}

// WithForwardRedirectHeaders keeps user-set headers (API keys, Authorization,
// Cookie, ...) on redirects that change origin. The default is to drop them.
func WithForwardRedirectHeaders(forward bool) ClientOption {
	return func(c *Client) { c.forwardRedirect = forward }
}

// CookieJar returns the client's cookie jar, or nil when cookies are disabled.
func (c *Client) CookieJar() http.CookieJar { return c.jar }

func (c *Client) Do(req *Request) (*Response, error) {
	return c.DoWithContext(context.Background(), req)
}

// DoWithContext executes the request bound to the caller's context so that
// cancellation or a deadline from the caller (e.g. a stress run's duration
// timeout) stops in-flight requests instead of running to completion.
func (c *Client) DoWithContext(ctx context.Context, req *Request) (*Response, error) {
	if c.configErr != nil {
		return nil, c.configErr
	}
	if req.Err != nil {
		return nil, req.Err
	}
	// The effective timeout is the per-request @timeout when set, otherwise the
	// client-level timeout. A per-request timeout longer than the client default
	// (e.g. @timeout 60000 with a 30s default) must not be silently capped, so the
	// deadline is enforced only via this context (http.Client.Timeout is 0).
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = c.timeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx = context.WithValue(ctx, timeoutKey{}, timeout)
		ctx, cancel = context.WithTimeout(ctx, timeout)
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

// timeoutKey carries the effective per-request timeout through the context so
// body-read errors can report it.
type timeoutKey struct{}

// isEventStream reports whether a Content-Type header names a Server-Sent
// Events stream (parameters and case are ignored).
func isEventStream(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	}
	return strings.EqualFold(mediaType, "text/event-stream")
}

// bodyReadError describes a failure while reading a response body, including
// how much of the body arrived before it failed.
func bodyReadError(ctx context.Context, err error, read int, contentLength int64) error {
	progress := fmt.Sprintf("read %d bytes", read)
	if contentLength >= 0 {
		progress = fmt.Sprintf("read %d of %d bytes", read, contentLength)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		if timeout, ok := ctx.Value(timeoutKey{}).(time.Duration); ok {
			return &BodyReadError{msg: fmt.Sprintf("timeout reading response body after %s (%s)", timeout, progress), err: err}
		}
		return &BodyReadError{msg: fmt.Sprintf("timeout reading response body (%s)", progress), err: err}
	}
	return &BodyReadError{msg: fmt.Sprintf("error reading response body (%s)", progress), err: err}
}

// BodyReadError reports a failure while reading a response body, after the
// response headers arrived. It unwraps to the underlying read error, and
// IsNetworkError never classifies it as a network error: a response did arrive.
type BodyReadError struct {
	msg string
	err error
}

func (e *BodyReadError) Error() string { return e.msg + ": " + e.err.Error() }
func (e *BodyReadError) Unwrap() error { return e.err }

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
		setRequestHeader(httpReq, k, v)
	}

	for k, v := range req.Headers {
		setRequestHeader(httpReq, k, v)
	}

	// Set multipart content type if present (must be after headers to override)
	if contentType != "" {
		httpReq.Header.Set("Content-Type", contentType)
	}

	// Set auth header if provided (for digest auth retry)
	if authHeader != "" {
		httpReq.Header.Set("Authorization", authHeader)
	}

	// Remember whether a connection was ever obtained, to tell a deadline that
	// expired while connecting (unreachable host) from one after connecting.
	var connected atomic.Bool
	httpReq = httpReq.WithContext(httptrace.WithClientTrace(httpReq.Context(), &httptrace.ClientTrace{
		GotConn: func(httptrace.GotConnInfo) { connected.Store(true) },
	}))

	start := time.Now()
	httpResp, err := c.httpClient.Do(httpReq)

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && !connected.Load() {
			err = &ConnectTimeoutError{err: err}
		}
		return nil, err
	}
	defer httpResp.Body.Close()

	// Fail fast on a declared length over the limit instead of downloading it.
	if c.maxBodyBytes > 0 && httpReq.Method != http.MethodHead && httpResp.ContentLength > c.maxBodyBytes {
		return nil, &ResponseTooLargeError{Limit: c.maxBodyBytes}
	}
	var responseReader io.Reader = httpResp.Body
	if c.maxBodyBytes > 0 {
		responseReader = io.LimitReader(httpResp.Body, c.maxBodyBytes+1)
	}
	respBody, err := io.ReadAll(responseReader)
	truncatedEventStream := false
	if err != nil {
		// A context deadline while reading the body is the expected termination
		// for a time-bounded SSE/event stream: the caller set @timeout to bound
		// how long to listen, so the partial body collected so far is the result,
		// not an error. For any other response a deadline (or any read error)
		// means the body is truncated and must fail the request rather than let
		// assertions run against partial data.
		if !(errors.Is(err, context.DeadlineExceeded) && isEventStream(httpResp.Header.Get("Content-Type"))) {
			return nil, bodyReadError(ctx, err, len(respBody), httpResp.ContentLength)
		}
		truncatedEventStream = true
	}
	if c.maxBodyBytes > 0 && int64(len(respBody)) > c.maxBodyBytes {
		return nil, &ResponseTooLargeError{Limit: c.maxBodyBytes}
	}
	rawSize := int64(len(respBody))

	// net/http only decompresses when it added Accept-Encoding itself; when the
	// request asked for gzip explicitly the body arrives still compressed.
	if !truncatedEventStream && !httpResp.Uncompressed {
		decoded, derr := decodeContentEncoding(respBody, httpResp.Header.Values("Content-Encoding"), c.maxBodyBytes)
		if derr != nil {
			return nil, derr
		}
		respBody = decoded
	}

	// Duration covers the full request including body download (previously it was
	// measured right after the headers arrived, so slow/large bodies reported an
	// artificially short duration).
	duration := time.Since(start)

	headers := make(map[string]string)
	headerValues := make(map[string][]string, len(httpResp.Header))
	for k := range httpResp.Header {
		values := httpResp.Header.Values(k)
		headerValues[k] = append([]string(nil), values...)
		switch len(values) {
		case 0:
			headers[k] = ""
		case 1:
			headers[k] = values[0]
		default:
			// Multi-value headers (Set-Cookie, Vary, ...) collapse with ", "
			// per RFC 9110 5.3; HeaderValues keeps the individual values.
			headers[k] = strings.Join(values, ", ")
		}
	}

	return &Response{
		StatusCode:   httpResp.StatusCode,
		Status:       httpResp.Status,
		Headers:      headers,
		HeaderValues: headerValues,
		Body:         respBody,
		Duration:     duration,
		FinalURL:     httpResp.Request.URL.String(),
		Proto:        httpResp.Proto,
		RawBodySize:  rawSize,
	}, nil
}

// setRequestHeader sets a header on r. net/http ignores a "Host" entry in
// Header, so it is applied to r.Host instead.
func setRequestHeader(r *http.Request, key, value string) {
	if strings.EqualFold(key, "Host") {
		r.Host = value
		return
	}
	r.Header.Set(key, value)
}

// decodeContentEncoding undoes gzip/deflate content codings (applied in the
// order listed, so undone in reverse). Unknown codings (br, zstd, ...) stop the
// decoding and the remaining body is returned as-is. limit bounds the decoded
// size when positive.
func decodeContentEncoding(body []byte, encodings []string, limit int64) ([]byte, error) {
	if len(body) == 0 {
		return body, nil
	}
	var codings []string
	for _, e := range encodings {
		for _, part := range strings.Split(e, ",") {
			if p := strings.ToLower(strings.TrimSpace(part)); p != "" && p != "identity" {
				codings = append(codings, p)
			}
		}
	}
	for i := len(codings) - 1; i >= 0; i-- {
		var r io.Reader
		switch codings[i] {
		case "gzip", "x-gzip":
			zr, err := gzip.NewReader(bytes.NewReader(body))
			if err != nil {
				return nil, fmt.Errorf("decoding gzip response body: %w", err)
			}
			r = zr
		case "deflate":
			// HTTP "deflate" is zlib-wrapped, but some servers send raw DEFLATE.
			if zr, err := zlib.NewReader(bytes.NewReader(body)); err == nil {
				r = zr
			} else {
				r = flate.NewReader(bytes.NewReader(body))
			}
		default:
			return body, nil
		}
		if limit > 0 {
			r = io.LimitReader(r, limit+1)
		}
		out, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("decoding %s response body: %w", codings[i], err)
		}
		if limit > 0 && int64(len(out)) > limit {
			return nil, &ResponseTooLargeError{Limit: limit}
		}
		body = out
	}
	return body, nil
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
	// SigV4 signs a hash of the exact bytes sent. A multipart body is normally
	// assembled at send time with a random boundary, so assemble it now and
	// sign (and send) those bytes.
	if len(req.Multipart) > 0 {
		body, contentType, err := BuildMultipartBody(req.Multipart, req.BaseDir)
		if err != nil {
			return nil, err
		}
		fixed := *req
		fixed.Body = body.String()
		fixed.Multipart = nil
		fixed.Headers = make(map[string]string, len(req.Headers)+1)
		for k, v := range req.Headers {
			if !strings.EqualFold(k, "Content-Type") {
				fixed.Headers[k] = v
			}
		}
		fixed.Headers["Content-Type"] = contentType
		req = &fixed
	}

	// Sign the request with AWS Signature v4
	authHeader, err := SignAWSRequest(req)
	if err != nil {
		return nil, err
	}

	return c.doRequest(ctx, req, authHeader)
}

func (c *Client) doWithOAuth2Auth(ctx context.Context, req *Request) (*Response, error) {
	auth := req.OAuth2Auth
	provider := oauth2.NewProvider(&oauth2.Config{
		TokenURL:     auth.TokenURL,
		ClientID:     auth.ClientID,
		ClientSecret: auth.ClientSecret,
		Scopes:       auth.Scopes,
		Username:     auth.Username,
		Password:     auth.Password,
		GrantType:    oauth2.GrantType(auth.GrantType),
	}).SetHTTPClient(c.httpClient).SetCache(c.oauthCache)

	// The token request shares the request's context (and so its deadline);
	// the provider additionally caps it at oauth2.DefaultTokenTimeout.
	token, fetched, err := provider.TokenContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth2 token: %w", err)
	}

	resp, err := c.doRequest(ctx, req, "Bearer "+token.AccessToken)
	if err != nil || resp.StatusCode != http.StatusUnauthorized || fetched {
		return resp, err
	}

	// A cached token may have been revoked or expired early: get a fresh one
	// and retry once.
	provider.Invalidate()
	token, _, err = provider.TokenContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth2 token: %w", err)
	}
	return c.doRequest(ctx, req, "Bearer "+token.AccessToken)
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
	if u.User != nil {
		return fmt.Errorf("URL must not contain embedded credentials; use an Authorization header")
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
			if err := pathutil.ValidateWithinBase(filePath, baseDir); err != nil {
				return nil, "", fmt.Errorf("multipart file %q: %w", field.Path, err)
			}

			file, err := os.Open(filePath)
			if err != nil {
				return nil, "", fmt.Errorf("multipart file %q: %w", field.Path, err)
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
