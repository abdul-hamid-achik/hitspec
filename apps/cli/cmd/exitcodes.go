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
