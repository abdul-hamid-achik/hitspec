package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config represents the hitspec configuration
type Config struct {
	DefaultEnvironment string            `json:"defaultEnvironment,omitempty"`
	Timeout            int               `json:"timeout,omitempty"`           // milliseconds
	Retries            int               `json:"retries,omitempty"`
	RetryDelay         int               `json:"retryDelay,omitempty"`        // milliseconds
	FollowRedirects    *bool             `json:"followRedirects,omitempty"`
	MaxRedirects       int               `json:"maxRedirects,omitempty"`
	ValidateSSL        *bool             `json:"validateSSL,omitempty"`
	Proxy              string            `json:"proxy,omitempty"`
	Headers            map[string]string `json:"headers,omitempty"`           // Default headers for all requests
	Reporters          []string          `json:"reporters,omitempty"`         // Output reporters
	OutputDir          string            `json:"outputDir,omitempty"`         // Directory for output files
	Parallel           *bool             `json:"parallel,omitempty"`
	Concurrency        int               `json:"concurrency,omitempty"`       // Number of parallel requests
	Bail               *bool             `json:"bail,omitempty"`
	Verbose            *bool             `json:"verbose,omitempty"`
	NoColor            *bool             `json:"noColor,omitempty"`
}

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

// BoolPtr is exported version of boolPtr for external use
func BoolPtr(b bool) *bool {
	return &b
}

// getBool returns the value of a bool pointer, or the default if nil
func getBool(b *bool, defaultVal bool) bool {
	if b == nil {
		return defaultVal
	}
	return *b
}

// GetFollowRedirects returns the follow redirects setting, defaulting to true
func (c *Config) GetFollowRedirects() bool {
	return getBool(c.FollowRedirects, true)
}

// GetValidateSSL returns the validate SSL setting, defaulting to true
func (c *Config) GetValidateSSL() bool {
	return getBool(c.ValidateSSL, true)
}

// GetParallel returns the parallel setting, defaulting to false
func (c *Config) GetParallel() bool {
	return getBool(c.Parallel, false)
}

// GetBail returns the bail setting, defaulting to false
func (c *Config) GetBail() bool {
	return getBool(c.Bail, false)
}

// GetVerbose returns the verbose setting, defaulting to false
func (c *Config) GetVerbose() bool {
	return getBool(c.Verbose, false)
}

// GetNoColor returns the no color setting, defaulting to false
func (c *Config) GetNoColor() bool {
	return getBool(c.NoColor, false)
}

// ConfigFilenames contains the possible config file names
var ConfigFilenames = []string{
	".hitspec.config.json",
	"hitspec.config.json",
	".hitspecrc",
	".hitspecrc.json",
}

// LoadConfig loads configuration from the specified path or searches for config files
func LoadConfig(path string) (*Config, error) {
	if path != "" {
		return loadConfigFromFile(path)
	}

	// Search for config file in current directory
	return FindAndLoadConfig(".")
}

// FindAndLoadConfig searches for a config file in the given directory
func FindAndLoadConfig(dir string) (*Config, error) {
	for _, filename := range ConfigFilenames {
		configPath := filepath.Join(dir, filename)
		if _, err := os.Stat(configPath); err == nil {
			return loadConfigFromFile(configPath)
		}
	}

	// Return defaults if no config file found
	return DefaultConfig(), nil
}

// loadConfigFromFile loads configuration from a specific file
func loadConfigFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// Merge merges another config into this one, with other taking precedence
func (c *Config) Merge(other *Config) *Config {
	if other == nil {
		return c
	}

	result := *c // Copy

	if other.DefaultEnvironment != "" {
		result.DefaultEnvironment = other.DefaultEnvironment
	}
	if other.Timeout > 0 {
		result.Timeout = other.Timeout
	}
	if other.Retries > 0 {
		result.Retries = other.Retries
	}
	if other.RetryDelay > 0 {
		result.RetryDelay = other.RetryDelay
	}
	if other.MaxRedirects > 0 {
		result.MaxRedirects = other.MaxRedirects
	}
	if other.Proxy != "" {
		result.Proxy = other.Proxy
	}
	if other.OutputDir != "" {
		result.OutputDir = other.OutputDir
	}
	if other.Concurrency > 0 {
		result.Concurrency = other.Concurrency
	}

	// Boolean flags - only override if explicitly set in other config
	if other.FollowRedirects != nil {
		result.FollowRedirects = other.FollowRedirects
	}
	if other.ValidateSSL != nil {
		result.ValidateSSL = other.ValidateSSL
	}
	if other.Parallel != nil {
		result.Parallel = other.Parallel
	}
	if other.Bail != nil {
		result.Bail = other.Bail
	}
	if other.Verbose != nil {
		result.Verbose = other.Verbose
	}
	if other.NoColor != nil {
		result.NoColor = other.NoColor
	}

	// Merge headers
	if len(other.Headers) > 0 {
		if result.Headers == nil {
			result.Headers = make(map[string]string)
		}
		for k, v := range other.Headers {
			result.Headers[k] = v
		}
	}

	// Merge reporters
	if len(other.Reporters) > 0 {
		result.Reporters = other.Reporters
	}

	return &result
}

// SaveConfig saves the configuration to a file
func (c *Config) SaveConfig(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
