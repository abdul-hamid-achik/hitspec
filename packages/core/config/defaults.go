package config

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		DefaultEnvironment: "dev",
		Timeout:            30000,  // 30 seconds
		Retries:            0,
		RetryDelay:         1000,   // 1 second
		FollowRedirects:    true,
		MaxRedirects:       10,
		ValidateSSL:        true,
		Proxy:              "",
		Headers:            nil,
		Reporters:          []string{"console"},
		OutputDir:          "",
		Parallel:           false,
		Concurrency:        5,
		Bail:               false,
		Verbose:            false,
		NoColor:            false,
	}
}

// IsDefault returns true if the config matches defaults
func (c *Config) IsDefault() bool {
	defaults := DefaultConfig()
	return c.DefaultEnvironment == defaults.DefaultEnvironment &&
		c.Timeout == defaults.Timeout &&
		c.Retries == defaults.Retries &&
		c.RetryDelay == defaults.RetryDelay &&
		c.FollowRedirects == defaults.FollowRedirects &&
		c.MaxRedirects == defaults.MaxRedirects &&
		c.ValidateSSL == defaults.ValidateSSL &&
		c.Proxy == defaults.Proxy &&
		len(c.Headers) == 0 &&
		c.OutputDir == defaults.OutputDir &&
		c.Parallel == defaults.Parallel &&
		c.Concurrency == defaults.Concurrency &&
		c.Bail == defaults.Bail &&
		c.Verbose == defaults.Verbose &&
		c.NoColor == defaults.NoColor
}
