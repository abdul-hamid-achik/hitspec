// Package cmd implements the hitspec CLI commands using Cobra.
//
// Available commands:
//   - run: Execute API tests from hitspec files
//   - validate: Check test file syntax without executing
//   - list: Display all tests defined in files
//   - init: Create a new hitspec project with example files
//   - version: Show hitspec version information
//
// The CLI supports various flags for filtering, output formatting,
// parallel execution, and watch mode for development workflows.
package cmd
