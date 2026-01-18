// Package runner executes hitspec test files and manages test execution.
//
// It provides functionality for:
//   - Running individual test files
//   - Managing test dependencies and execution order
//   - Parallel test execution with configurable concurrency
//   - Request retry handling
//   - Environment variable resolution
//   - Capturing and storing response values
//
// The runner supports both sequential and parallel execution modes,
// with automatic dependency resolution using topological sorting.
package runner
