package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

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

	// ExitInterrupted indicates the run was stopped by SIGINT/SIGTERM
	// (128 + SIGINT, the shell convention for Ctrl-C).
	ExitInterrupted = 130
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

// usageError is a command-line mistake found by the argument parser: an
// unknown flag or command, a bad flag value, a missing required flag or the
// wrong number of arguments. Unlike exitError, the root command prints it.
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }
func (e usageError) ExitCode() int { return ExitUsageError }

// markUsageErrors makes cobra's own parse and argument-count errors usage
// errors (exit 64) for c and every command below it.
func markUsageErrors(c *cobra.Command) {
	c.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{err} })
	if validate := c.Args; validate != nil {
		c.Args = func(cmd *cobra.Command, args []string) error {
			if err := validate(cmd, args); err != nil {
				return usageError{err}
			}
			return nil
		}
	}
	for _, sub := range c.Commands() {
		markUsageErrors(sub)
	}
}

// isCobraUsageError recognizes the parse errors cobra returns as plain
// errors, before any command runs: an unknown command and a required flag
// that was not set.
func isCobraUsageError(err error) bool {
	msg := err.Error()
	return strings.HasPrefix(msg, "unknown command ") ||
		strings.HasPrefix(msg, `required flag(s) "`)
}
