package clientmgr

// Config holds configuration for the API Client Manager.
type Config struct {
	WorkDir       string
	Watch         bool
	ReadOnly      bool
	Env           string
	ConfigPath    string
	Verbose       bool
	AllowShell    bool
	AllowDB       bool
	HistoryDBPath string
	LogFormat     string
	LogLevel      string
}

// Option configures Config.
type Option func(*Config)

// WithWorkDir sets the workspace directory.
func WithWorkDir(dir string) Option {
	return func(c *Config) { c.WorkDir = dir }
}

// WithWatch enables file watching.
func WithWatch(watch bool) Option {
	return func(c *Config) { c.Watch = watch }
}

// WithReadOnly disallows mutating actions.
func WithReadOnly(readOnly bool) Option {
	return func(c *Config) { c.ReadOnly = readOnly }
}

// WithEnv sets the active environment.
func WithEnv(env string) Option {
	return func(c *Config) { c.Env = env }
}

// WithConfigPath sets the path to hitspec.yaml.
func WithConfigPath(path string) Option {
	return func(c *Config) { c.ConfigPath = path }
}

// WithVerbose enables verbose feature engines.
func WithVerbose(verbose bool) Option {
	return func(c *Config) { c.Verbose = verbose }
}

// WithAllowShell allows shell command execution.
func WithAllowShell(allow bool) Option {
	return func(c *Config) { c.AllowShell = allow }
}

// WithAllowDB allows database assertions.
func WithAllowDB(allow bool) Option {
	return func(c *Config) { c.AllowDB = allow }
}

// WithHistoryDBPath sets the path to the persistent history database.
func WithHistoryDBPath(path string) Option {
	return func(c *Config) { c.HistoryDBPath = path }
}

// WithLogFormat sets the log output format.
func WithLogFormat(format string) Option {
	return func(c *Config) { c.LogFormat = format }
}

// WithLogLevel sets the minimum log level.
func WithLogLevel(level string) Option {
	return func(c *Config) { c.LogLevel = level }
}

// DefaultConfig returns the default manager configuration.
func DefaultConfig() *Config {
	return &Config{
		Watch:     true,
		Env:       "dev",
		WorkDir:   ".",
		LogFormat: "text",
		LogLevel:  "info",
	}
}
