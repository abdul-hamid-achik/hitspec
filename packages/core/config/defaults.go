package config

const (
	// DefaultTimeoutMs is the default request timeout in milliseconds
	DefaultTimeoutMs = 30000
	// DefaultRetryDelayMs is the default delay between retries in milliseconds
	DefaultRetryDelayMs = 1000
	// DefaultMaxRedirects is the default maximum number of redirects to follow
	DefaultMaxRedirects = 10
	// DefaultConcurrency is the default number of concurrent requests in parallel mode
	DefaultConcurrency = 5
)

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		DefaultEnvironment: "dev",
		Timeout:            DefaultTimeoutMs,
		Retries:            0,
		RetryDelay:         DefaultRetryDelayMs,
		FollowRedirects:    boolPtr(true),
		MaxRedirects:       DefaultMaxRedirects,
		ValidateSSL:        boolPtr(true),
		Proxy:              "",
		Headers:            nil,
		Reporters:          []string{"console"},
		OutputDir:          "",
		Parallel:           boolPtr(false),
		Concurrency:        DefaultConcurrency,
		Bail:               boolPtr(false),
		Verbose:            boolPtr(false),
		NoColor:            boolPtr(false),
	}
}

// IsDefault returns true if the config matches defaults
func (c *Config) IsDefault() bool {
	defaults := DefaultConfig()
	return c.DefaultEnvironment == defaults.DefaultEnvironment &&
		c.Timeout == defaults.Timeout &&
		c.Retries == defaults.Retries &&
		c.RetryDelay == defaults.RetryDelay &&
		getBool(c.FollowRedirects, true) == getBool(defaults.FollowRedirects, true) &&
		c.MaxRedirects == defaults.MaxRedirects &&
		getBool(c.ValidateSSL, true) == getBool(defaults.ValidateSSL, true) &&
		c.Proxy == defaults.Proxy &&
		len(c.Headers) == 0 &&
		c.OutputDir == defaults.OutputDir &&
		getBool(c.Parallel, false) == getBool(defaults.Parallel, false) &&
		c.Concurrency == defaults.Concurrency &&
		getBool(c.Bail, false) == getBool(defaults.Bail, false) &&
		getBool(c.Verbose, false) == getBool(defaults.Verbose, false) &&
		getBool(c.NoColor, false) == getBool(defaults.NoColor, false)
}
