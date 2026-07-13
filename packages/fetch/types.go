// Package fetch executes one HTTP request and renders its response body for
// humans, files, and machine-to-machine transports.
package fetch

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultMaxBodyBytes bounds responses when no explicit limit is supplied.
	DefaultMaxBodyBytes int64 = 64 << 20
	// DefaultTimeout bounds an HTTP exchange including its response body.
	DefaultTimeout = 30 * time.Second
	// DefaultMaxRedirects limits redirect chains.
	DefaultMaxRedirects = 10
)

// Format names a response representation. Raw is byte-exact.
type Format string

const (
	FormatRaw      Format = "raw"
	FormatText     Format = "text"
	FormatMarkdown Format = "markdown"
	FormatJSON     Format = "json"
)

// ParseFormat validates a response format name.
func ParseFormat(value string) (Format, error) {
	format := Format(strings.ToLower(strings.TrimSpace(value)))
	switch format {
	case FormatRaw, FormatText, FormatMarkdown, FormatJSON:
		return format, nil
	default:
		return "", fmt.Errorf("unknown response format %q (valid formats: raw, text, markdown, json)", value)
	}
}

// NetworkPolicy determines which address classes may be reached.
type NetworkPolicy int

const (
	// NetworkAny permits private targets for an explicit CLI invocation.
	NetworkAny NetworkPolicy = iota
	// NetworkPublicOnly blocks private, loopback, link-local, and reserved IPs.
	NetworkPublicOnly
)

// Request describes one ad-hoc HTTP exchange.
type Request struct {
	Method          string
	URL             string
	Headers         http.Header
	Body            []byte
	Timeout         time.Duration
	FollowRedirects bool
	MaxRedirects    int
	Insecure        bool
	Proxy           string
	MaxBodyBytes    int64
	UserAgent       string
	NetworkPolicy   NetworkPolicy
}

// Result is a byte-safe response plus renderer metadata.
type Result struct {
	RequestedURL string
	FinalURL     string
	Status       string
	StatusCode   int
	Headers      http.Header
	Body         []byte
	ContentType  string
	Duration     time.Duration
}

// Success reports whether the response has a 2xx status.
func (r *Result) Success() bool {
	return r != nil && r.StatusCode >= http.StatusOK && r.StatusCode < http.StatusMultipleChoices
}

// BodyTooLargeError reports an explicit size-bound violation.
type BodyTooLargeError struct{ Limit int64 }

func (e *BodyTooLargeError) Error() string {
	return fmt.Sprintf("response body exceeds %d-byte limit", e.Limit)
}

// IsBodyTooLarge reports whether err is a response-size violation.
func IsBodyTooLarge(err error) bool {
	var target *BodyTooLargeError
	return errors.As(err, &target)
}
