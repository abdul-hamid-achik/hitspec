package http

import (
	"encoding/json"
	"strings"
	"time"
)

type Response struct {
	StatusCode int
	Status     string
	Headers    map[string]string
	Body       []byte
	Duration   time.Duration
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
