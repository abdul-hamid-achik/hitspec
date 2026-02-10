package serve

// ServeConfig holds configuration for the serve command.
type ServeConfig struct {
	Port       int
	Host       string
	WorkDir    string
	Open       bool
	Watch      bool
	CORS       bool
	APIOnly    bool
	ReadOnly   bool
	Env        string
	ConfigPath string
	Verbose    bool
	AllowShell    bool
	AllowDB       bool
	HistoryDBPath string // Path to persistent history SQLite database
	LogFormat     string // "json" or "text" (default: "text")
	LogLevel      string // "debug", "info", "warn", "error" (default: "info")
}

// Option configures ServeConfig.
type Option func(*ServeConfig)

// WithPort sets the server port.
func WithPort(port int) Option {
	return func(c *ServeConfig) { c.Port = port }
}

// WithHost sets the bind address.
func WithHost(host string) Option {
	return func(c *ServeConfig) { c.Host = host }
}

// WithWorkDir sets the working directory.
func WithWorkDir(dir string) Option {
	return func(c *ServeConfig) { c.WorkDir = dir }
}

// WithOpen enables auto-opening the browser.
func WithOpen(open bool) Option {
	return func(c *ServeConfig) { c.Open = open }
}

// WithWatch enables file watching.
func WithWatch(watch bool) Option {
	return func(c *ServeConfig) { c.Watch = watch }
}

// WithCORS enables CORS headers.
func WithCORS(cors bool) Option {
	return func(c *ServeConfig) { c.CORS = cors }
}

// WithAPIOnly disables the SPA frontend.
func WithAPIOnly(apiOnly bool) Option {
	return func(c *ServeConfig) { c.APIOnly = apiOnly }
}

// WithReadOnly disallows file mutations.
func WithReadOnly(readOnly bool) Option {
	return func(c *ServeConfig) { c.ReadOnly = readOnly }
}

// WithEnv sets the default environment.
func WithEnv(env string) Option {
	return func(c *ServeConfig) { c.Env = env }
}

// WithConfigPath sets the path to hitspec.yaml.
func WithConfigPath(path string) Option {
	return func(c *ServeConfig) { c.ConfigPath = path }
}

// WithVerbose enables verbose logging.
func WithVerbose(verbose bool) Option {
	return func(c *ServeConfig) { c.Verbose = verbose }
}

// WithAllowShell allows shell command execution.
func WithAllowShell(allow bool) Option {
	return func(c *ServeConfig) { c.AllowShell = allow }
}

// WithAllowDB allows database assertions.
func WithAllowDB(allow bool) Option {
	return func(c *ServeConfig) { c.AllowDB = allow }
}

// WithHistoryDBPath sets the path to the persistent history database.
func WithHistoryDBPath(path string) Option {
	return func(c *ServeConfig) { c.HistoryDBPath = path }
}

// WithLogFormat sets the log output format ("json" or "text").
func WithLogFormat(format string) Option {
	return func(c *ServeConfig) { c.LogFormat = format }
}

// WithLogLevel sets the minimum log level ("debug", "info", "warn", "error").
func WithLogLevel(level string) Option {
	return func(c *ServeConfig) { c.LogLevel = level }
}

// DefaultConfig returns a ServeConfig with default values.
func DefaultConfig() *ServeConfig {
	return &ServeConfig{
		Port:      4000,
		Host:      "localhost",
		Open:      true,
		Watch:     true,
		Env:       "dev",
		WorkDir:   ".",
		LogFormat: "text",
		LogLevel:  "info",
	}
}
