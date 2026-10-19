package runconfig

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

// Overrides are the settings a caller pins above hitspec.yaml. The zero value
// overrides nothing; a nil pointer or zero number/string means "not set".
type Overrides struct {
	Environment     string
	EnvFile         string
	Timeout         time.Duration
	Retries         *int
	RetryDelay      time.Duration
	FollowRedirects *bool
	MaxRedirects    int
	ValidateSSL     *bool
	Proxy           string
	// Headers are merged over the default headers from hitspec.yaml.
	Headers     map[string]string
	Parallel    *bool
	Concurrency int
	Bail        *bool

	// HTTP client settings. Paths are used as given (relative to the working
	// directory); only paths from hitspec.yaml are resolved against that file.
	CACert                 string
	Cert                   string
	Key                    string
	HTTP1                  *bool
	Cookies                *bool
	MaxResponseBytes       *int64 // 0 = default, -1 = unlimited
	RedirectForwardHeaders *bool
}

// Environment variables read by FromEnv.
const (
	EnvTimeout     = "HITSPEC_TIMEOUT"     // duration, e.g. 30s
	EnvProxy       = "HITSPEC_PROXY"       // proxy URL
	EnvInsecure    = "HITSPEC_INSECURE"    // true disables certificate validation
	EnvParallel    = "HITSPEC_PARALLEL"    // run requests in parallel
	EnvConcurrency = "HITSPEC_CONCURRENCY" // parallel request limit
	EnvBail        = "HITSPEC_BAIL"        // stop on first failure

	EnvCACert                 = "HITSPEC_CACERT"                   // PEM CA bundle to trust
	EnvCert                   = "HITSPEC_CERT"                     // PEM client certificate (mTLS)
	EnvKey                    = "HITSPEC_KEY"                      // PEM client key (mTLS)
	EnvHTTP1                  = "HITSPEC_HTTP1"                    // true disables HTTP/2
	EnvCookies                = "HITSPEC_COOKIES"                  // keep a cookie jar for the run
	EnvMaxResponseBytes       = "HITSPEC_MAX_RESPONSE_BYTES"       // response body bound; -1 = unlimited
	EnvRedirectForwardHeaders = "HITSPEC_REDIRECT_FORWARD_HEADERS" // keep user headers on cross-origin redirects
)

// FromEnv reads the HITSPEC_* variables that map to run settings. A variable
// that is unset or empty overrides nothing. An unparsable value is reported in
// the error but the remaining variables are still returned, so a surface with
// nowhere to print can ignore the error.
func FromEnv() (Overrides, error) {
	var o Overrides
	var errs []string

	if v := os.Getenv(EnvTimeout); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			errs = append(errs, fmt.Sprintf("invalid %s %q: %v (use format like 30s, 1m, 500ms)", EnvTimeout, v, err))
		} else {
			o.Timeout = d
		}
	}
	o.Proxy = os.Getenv(EnvProxy)
	if v := os.Getenv(EnvInsecure); v != "" {
		validate := !envBool(v)
		o.ValidateSSL = &validate
	}
	if v := os.Getenv(EnvParallel); v != "" {
		b := envBool(v)
		o.Parallel = &b
	}
	if v := os.Getenv(EnvBail); v != "" {
		b := envBool(v)
		o.Bail = &b
	}
	fromEnvHTTP(&o, &errs)
	if v := os.Getenv(EnvConcurrency); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			errs = append(errs, fmt.Sprintf("invalid %s %q: %v", EnvConcurrency, v, err))
		} else {
			o.Concurrency = n
		}
	}

	if len(errs) > 0 {
		return o, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return o, nil
}

// fromEnvHTTP reads the HTTP client variables into o.
func fromEnvHTTP(o *Overrides, errs *[]string) {
	o.CACert = os.Getenv(EnvCACert)
	o.Cert = os.Getenv(EnvCert)
	o.Key = os.Getenv(EnvKey)
	for _, b := range []struct {
		name string
		dst  **bool
	}{{EnvHTTP1, &o.HTTP1}, {EnvCookies, &o.Cookies}, {EnvRedirectForwardHeaders, &o.RedirectForwardHeaders}} {
		if v := os.Getenv(b.name); v != "" {
			val := envBool(v)
			*b.dst = &val
		}
	}
	if v := os.Getenv(EnvMaxResponseBytes); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			*errs = append(*errs, fmt.Sprintf("invalid %s %q: %v", EnvMaxResponseBytes, v, err))
		} else {
			o.MaxResponseBytes = &n
		}
	}
}

func envBool(v string) bool { return v == "true" || v == "1" || v == "yes" }

// Over returns o with every unset field filled from lower. Use it to layer a
// higher-precedence source (flags) over a lower one (FromEnv).
func (o Overrides) Over(lower Overrides) Overrides {
	if o.Environment == "" {
		o.Environment = lower.Environment
	}
	if o.EnvFile == "" {
		o.EnvFile = lower.EnvFile
	}
	if o.Timeout <= 0 {
		o.Timeout = lower.Timeout
	}
	if o.Retries == nil {
		o.Retries = lower.Retries
	}
	if o.RetryDelay <= 0 {
		o.RetryDelay = lower.RetryDelay
	}
	if o.FollowRedirects == nil {
		o.FollowRedirects = lower.FollowRedirects
	}
	if o.MaxRedirects <= 0 {
		o.MaxRedirects = lower.MaxRedirects
	}
	if o.ValidateSSL == nil {
		o.ValidateSSL = lower.ValidateSSL
	}
	if o.Proxy == "" {
		o.Proxy = lower.Proxy
	}
	if len(lower.Headers) > 0 {
		merged := make(map[string]string, len(lower.Headers)+len(o.Headers))
		for k, v := range lower.Headers {
			merged[k] = v
		}
		for k, v := range o.Headers {
			merged[k] = v
		}
		o.Headers = merged
	}
	if o.Parallel == nil {
		o.Parallel = lower.Parallel
	}
	if o.Concurrency <= 0 {
		o.Concurrency = lower.Concurrency
	}
	if o.Bail == nil {
		o.Bail = lower.Bail
	}
	o.CACert = firstNonEmpty(o.CACert, lower.CACert)
	o.Cert = firstNonEmpty(o.Cert, lower.Cert)
	o.Key = firstNonEmpty(o.Key, lower.Key)
	if o.HTTP1 == nil {
		o.HTTP1 = lower.HTTP1
	}
	if o.Cookies == nil {
		o.Cookies = lower.Cookies
	}
	if o.MaxResponseBytes == nil {
		o.MaxResponseBytes = lower.MaxResponseBytes
	}
	if o.RedirectForwardHeaders == nil {
		o.RedirectForwardHeaders = lower.RedirectForwardHeaders
	}
	return o
}

// ResolveEnvironment picks the environment name: the override, then
// hitspec.yaml's defaultEnvironment, then "dev". explicit reports whether the
// caller chose it rather than it being a default.
func ResolveEnvironment(override string, file *config.Config) (name string, explicit bool) {
	if override != "" {
		return override, true
	}
	if file != nil && file.DefaultEnvironment != "" {
		return file.DefaultEnvironment, false
	}
	return "dev", false
}

// Build resolves file (which may be nil) and o into a runner configuration and
// the HTTP client options that go with it. The caller adds what is specific to
// its run: name/tag/index filters, progress callback, history store, snapshot
// and shell/DB permissions, response-size bound.
//
// Nothing in the result aliases file, so a caller can build while holding the
// lock that guards a shared config and use the result after releasing it.
func Build(file *config.Config, o Overrides) (*runner.Config, []http.ClientOption) {
	if file == nil {
		file = config.DefaultConfig()
	}
	environment, _ := ResolveEnvironment(o.Environment, file)

	cfg := &runner.Config{
		Environment:        environment,
		EnvFile:            o.EnvFile,
		Timeout:            pickDuration(o.Timeout, ms(file.Timeout), http.DefaultTimeout),
		FollowRedirect:     pickBool(o.FollowRedirects, file.GetFollowRedirects()),
		MaxRedirects:       pickInt(o.MaxRedirects, file.MaxRedirects, config.DefaultMaxRedirects),
		ValidateSSL:        pickBool(o.ValidateSSL, file.GetValidateSSL()),
		Proxy:              firstNonEmpty(o.Proxy, file.Proxy),
		DefaultHeaders:     mergeHeaders(file.Headers, o.Headers),
		ConfigEnvironments: copyEnvironments(file.Environments),
		RetryDelay:         pickDuration(o.RetryDelay, ms(file.RetryDelay), config.DefaultRetryDelayMs*time.Millisecond),
		Parallel:           pickBool(o.Parallel, file.GetParallel()),
		Concurrency:        pickInt(o.Concurrency, file.Concurrency, config.DefaultConcurrency),
		Bail:               pickBool(o.Bail, file.GetBail()),

		CACert:                 firstNonEmpty(o.CACert, file.ResolvePath(file.CACert)),
		ClientCert:             firstNonEmpty(o.Cert, file.ResolvePath(file.Cert)),
		ClientKey:              firstNonEmpty(o.Key, file.ResolvePath(file.Key)),
		HTTP1:                  pickBool(o.HTTP1, file.GetHTTP1()),
		Cookies:                pickBool(o.Cookies, file.GetCookies()),
		MaxResponseBytes:       file.MaxResponseBytes,
		RedirectForwardHeaders: pickBool(o.RedirectForwardHeaders, file.GetRedirectForwardHeaders()),
	}
	if o.MaxResponseBytes != nil {
		cfg.MaxResponseBytes = *o.MaxResponseBytes
	}
	retries := file.Retries
	if o.Retries != nil {
		retries = *o.Retries
	}
	cfg.Retries = max(retries, 0)
	return cfg, cfg.ClientOptions()
}

func ms(v int) time.Duration { return time.Duration(v) * time.Millisecond }

func pickDuration(override, file, def time.Duration) time.Duration {
	switch {
	case override > 0:
		return override
	case file > 0:
		return file
	}
	return def
}

func pickInt(override, file, def int) int {
	switch {
	case override > 0:
		return override
	case file > 0:
		return file
	}
	return def
}

func pickBool(override *bool, fromFile bool) bool {
	if override != nil {
		return *override
	}
	return fromFile
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func mergeHeaders(layers ...map[string]string) map[string]string {
	var out map[string]string
	for _, layer := range layers {
		for k, v := range layer {
			if out == nil {
				out = make(map[string]string)
			}
			out[k] = v
		}
	}
	return out
}

func copyEnvironments(envs map[string]map[string]any) map[string]map[string]any {
	if envs == nil {
		return nil
	}
	out := make(map[string]map[string]any, len(envs))
	for name, vars := range envs {
		copied := make(map[string]any, len(vars))
		for k, v := range vars {
			copied[k] = v
		}
		out[name] = copied
	}
	return out
}
