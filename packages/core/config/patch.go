package config

import "fmt"

// Patch is a partial update of the settings that the API Client Manager and
// `hitspec serve --api-only` edit. A nil field is "not part of the update",
// which is what lets a caller set `retries` to 0 or clear `proxy`: a plain
// int/string cannot tell "absent" from "zero". It decodes straight from the
// JSON body of PUT /config, so key presence is the field mask.
type Patch struct {
	DefaultEnvironment *string           `json:"defaultEnvironment"`
	Timeout            *int              `json:"timeout"`
	Retries            *int              `json:"retries"`
	RetryDelay         *int              `json:"retryDelay"`
	FollowRedirects    *bool             `json:"followRedirects"`
	MaxRedirects       *int              `json:"maxRedirects"`
	ValidateSSL        *bool             `json:"validateSSL"`
	Proxy              *string           `json:"proxy"`
	Headers            map[string]string `json:"headers"` // non-nil replaces all default headers ({} clears them)
	Parallel           *bool             `json:"parallel"`
	Concurrency        *int              `json:"concurrency"`
}

// Apply validates p and then applies it to c; on error c is untouched.
//
// Numeric settings other than retries cannot be zero (a zero timeout or retry
// delay would not mean "none" and hitspec.yaml does not persist zero for
// them), so an explicit 0 resets them to their built-in default. Retries may
// be 0 (no retries). An empty defaultEnvironment is ignored; an empty proxy
// clears the proxy.
func (c *Config) Apply(p Patch) error {
	for _, f := range []struct {
		name string
		v    *int
	}{
		{"timeout", p.Timeout}, {"retries", p.Retries}, {"retryDelay", p.RetryDelay},
		{"maxRedirects", p.MaxRedirects}, {"concurrency", p.Concurrency},
	} {
		if f.v != nil && *f.v < 0 {
			return fmt.Errorf("%s must not be negative", f.name)
		}
	}

	if p.DefaultEnvironment != nil && *p.DefaultEnvironment != "" {
		c.DefaultEnvironment = *p.DefaultEnvironment
	}
	setOrDefault := func(dst *int, v *int, def int) {
		if v == nil {
			return
		}
		if *v == 0 {
			*dst = def
			return
		}
		*dst = *v
	}
	setOrDefault(&c.Timeout, p.Timeout, DefaultTimeoutMs)
	if p.Retries != nil {
		c.Retries = *p.Retries
	}
	setOrDefault(&c.RetryDelay, p.RetryDelay, DefaultRetryDelayMs)
	if p.FollowRedirects != nil {
		c.FollowRedirects = BoolPtr(*p.FollowRedirects)
	}
	setOrDefault(&c.MaxRedirects, p.MaxRedirects, DefaultMaxRedirects)
	if p.ValidateSSL != nil {
		c.ValidateSSL = BoolPtr(*p.ValidateSSL)
	}
	if p.Proxy != nil {
		c.Proxy = *p.Proxy
	}
	if p.Headers != nil {
		c.Headers = p.Headers
	}
	if p.Parallel != nil {
		c.Parallel = BoolPtr(*p.Parallel)
	}
	setOrDefault(&c.Concurrency, p.Concurrency, DefaultConcurrency)
	return nil
}
