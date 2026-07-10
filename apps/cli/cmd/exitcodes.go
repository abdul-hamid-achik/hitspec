package cmd

// Exit codes for hitspec CLI
const (
	// ExitSuccess indicates all tests passed
	ExitSuccess = 0

	// ExitTestFailure indicates one or more tests failed
	ExitTestFailure = 1

	// ExitParseError indicates a file parsing error
	ExitParseError = 2

	// ExitConfigError indicates a configuration error
	ExitConfigError = 3

	// ExitNetworkError indicates a network/connection error
	ExitNetworkError = 4

	// ExitUsageError indicates invalid CLI usage
	ExitUsageError = 64
)

// ExitCoder is an error that carries a process exit code. Commands that want
// to emit a semantically distinct exit code (parse, config, ...) return an
// ExitCoder from their RunE instead of calling os.Exit directly, so the exit
// code stays testable through rootCmd.Execute() (which surfaces the error)
// while the root command maps it to the right process exit code.
type ExitCoder interface {
	error
	ExitCode() int
}

// exitError wraps a plain message with a specific exit code.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }
func (e *exitError) ExitCode() int { return e.code }

// newExitError returns an error the root command maps to the given exit code.
func newExitError(code int, msg string) error {
	return &exitError{code: code, msg: msg}
}
