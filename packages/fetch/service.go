package fetch

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// HostResolver is the DNS subset used by the public-only policy.
type HostResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

// Service executes bounded HTTP requests.
type Service struct{ resolver HostResolver }

// Option configures Service.
type Option func(*Service)

// WithResolver replaces DNS resolution, primarily for tests.
func WithResolver(resolver HostResolver) Option {
	return func(service *Service) {
		if resolver != nil {
			service.resolver = resolver
		}
	}
}

// NewService constructs a fetch service.
func NewService(options ...Option) *Service {
	service := &Service{resolver: net.DefaultResolver}
	for _, option := range options {
		option(service)
	}
	return service
}

// Fetch executes one request and rejects oversized bodies without truncation.
func (s *Service) Fetch(ctx context.Context, input Request) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	requestedURL, err := validateURL(input.URL)
	if err != nil {
		return nil, err
	}
	if input.NetworkPolicy == NetworkPublicOnly {
		if strings.TrimSpace(input.Proxy) != "" {
			return nil, errors.New("a proxy is not allowed with the public-only network policy")
		}
		if _, err := resolvePublic(ctx, s.resolver, requestedURL.Hostname()); err != nil {
			return nil, fmt.Errorf("reject request target: %w", err)
		}
	}
	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method == "" {
		method = http.MethodGet
	}
	timeout := input.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	maxBytes := input.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}
	maxRedirects := input.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = DefaultMaxRedirects
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	}
	transport.TLSClientConfig.InsecureSkipVerify = input.Insecure //nolint:gosec // explicit curl-compatible flag
	if input.NetworkPolicy == NetworkPublicOnly {
		transport.Proxy = nil
		transport.DialContext = (&publicDialer{resolver: s.resolver}).DialContext
	} else if strings.TrimSpace(input.Proxy) != "" {
		proxyURL, err := url.Parse(input.Proxy)
		if err != nil {
			return nil, fmt.Errorf("parse proxy URL: %w", err)
		}
		if proxyURL.Scheme != "http" && proxyURL.Scheme != "https" && proxyURL.Scheme != "socks5" {
			return nil, fmt.Errorf("unsupported proxy scheme %q", proxyURL.Scheme)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if !input.FollowRedirects {
				return http.ErrUseLastResponse
			}
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			if _, err := validateURL(next.URL.String()); err != nil {
				return fmt.Errorf("reject redirect target: %w", err)
			}
			if input.NetworkPolicy == NetworkPublicOnly {
				if _, err := resolvePublic(next.Context(), s.resolver, next.URL.Hostname()); err != nil {
					return fmt.Errorf("reject redirect target: %w", err)
				}
			}
			return nil
		},
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(requestCtx, method, requestedURL.String(), bytes.NewReader(input.Body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpRequest.Header = input.Headers.Clone()
	if input.UserAgent != "" && httpRequest.Header.Get("User-Agent") == "" {
		httpRequest.Header.Set("User-Agent", input.UserAgent)
	}
	started := time.Now()
	response, err := client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, &BodyTooLargeError{Limit: maxBytes}
	}
	finalURL := requestedURL.String()
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
	}
	return &Result{
		RequestedURL: requestedURL.String(), FinalURL: finalURL,
		Status: response.Status, StatusCode: response.StatusCode,
		Headers: response.Header.Clone(), Body: body,
		ContentType: response.Header.Get("Content-Type"), Duration: time.Since(started),
	}, nil
}

func validateURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("URL is required")
	}
	if len(raw) > 8192 {
		return nil, errors.New("URL exceeds 8192 characters")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("URL scheme must be http or https")
	}
	if parsed.Host == "" {
		return nil, errors.New("URL must include a host")
	}
	if parsed.User != nil {
		return nil, errors.New("URL must not contain embedded credentials; use an Authorization header")
	}
	parsed.Fragment = ""
	return parsed, nil
}

type publicDialer struct {
	resolver HostResolver
	dialer   net.Dialer
}

func (d *publicDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("parse target address: %w", err)
	}
	addresses, err := resolvePublic(ctx, d.resolver, host)
	if err != nil {
		return nil, err
	}
	var dialErrors []error
	for _, resolved := range addresses {
		connection, err := d.dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
		if err == nil {
			return connection, nil
		}
		dialErrors = append(dialErrors, err)
	}
	return nil, fmt.Errorf("dial public target: %w", errors.Join(dialErrors...))
}

var deniedPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"),
}

func resolvePublic(ctx context.Context, resolver HostResolver, host string) ([]net.IPAddr, error) {
	var addresses []net.IPAddr
	if parsed := net.ParseIP(host); parsed != nil {
		addresses = []net.IPAddr{{IP: parsed}}
	} else {
		resolved, err := resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve target host %q: %w", host, err)
		}
		addresses = resolved
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("resolve target host %q: no addresses", host)
	}
	for _, address := range addresses {
		parsed, ok := netip.AddrFromSlice(address.IP)
		if !ok || !publicIP(parsed.Unmap()) {
			return nil, fmt.Errorf("target host %q resolves to non-public address %s", host, address.IP)
		}
	}
	return addresses, nil
}

func publicIP(address netip.Addr) bool {
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
		address.IsUnspecified() || address.IsLinkLocalUnicast() || address.IsMulticast() {
		return false
	}
	for _, prefix := range deniedPublicPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}
