package http

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Response struct {
	StatusCode int
	Status     string
	// Headers has one string per header name. Multi-value headers are joined
	// with ", " (RFC 9110 5.3), Set-Cookie included, so every value is
	// searchable with `contains`; use HeaderValues to get them separately.
	Headers map[string]string
	// HeaderValues holds every value of every header, unjoined (the right
	// source for Set-Cookie, whose values may themselves contain commas).
	HeaderValues map[string][]string
	Body         []byte
	Duration     time.Duration
	FinalURL     string
	// Proto is the negotiated protocol, e.g. "HTTP/1.1" or "HTTP/2.0".
	Proto string
	// RawBodySize is the number of body bytes read off the connection, before
	// hitspec decoded a gzip/deflate Content-Encoding (len(Body) is the
	// decoded size). Bodies that Go's transport already decompressed report
	// the decompressed size.
	RawBodySize int64
}

// ResponseTooLargeError reports a configured response-body limit violation.
type ResponseTooLargeError struct{ Limit int64 }

func (e *ResponseTooLargeError) Error() string {
	return fmt.Sprintf("response body exceeds %d-byte limit", e.Limit)
}

func (r *Response) BodyString() string {
	return string(r.Body)
}

func (r *Response) BodyJSON() (any, error) {
	var result any
	if err := json.Unmarshal(r.Body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Values returns every value of a header (case-insensitive name), unjoined.
func (r *Response) Values(key string) []string {
	for k, v := range r.HeaderValues {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	if v := r.Header(key); v != "" {
		return []string{v}
	}
	return nil
}

func (r *Response) Header(key string) string {
	for k, v := range r.Headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

func (r *Response) ContentType() string {
	return r.Header("Content-Type")
}

func (r *Response) IsJSON() bool {
	ct := r.ContentType()
	return strings.Contains(ct, "application/json")
}

func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

func (r *Response) IsRedirect() bool {
	return r.StatusCode >= 300 && r.StatusCode < 400
}

func (r *Response) IsClientError() bool {
	return r.StatusCode >= 400 && r.StatusCode < 500
}

func (r *Response) IsServerError() bool {
	return r.StatusCode >= 500
}

func (r *Response) DurationMs() int64 {
	return r.Duration.Milliseconds()
}
