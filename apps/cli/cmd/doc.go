// Package cmd implements the hitspec CLI commands using Cobra.
//
// Available commands:
//   - run:        Execute API tests from hitspec files
//   - validate:   Check test file syntax without executing
//   - list:       Display all tests defined in files
//   - init:       Create a new hitspec project with example files
//   - serve:      Start the web-based API Client Manager
//   - import:     Import from OpenAPI, Postman, curl, or Insomnia
//   - export:     Export requests to curl format
//   - mock:       Start a mock server from hitspec files
//   - record:     Start a recording proxy to capture requests
//   - contract:   Verify API contracts against providers
//   - diff:       Compare two JSON test result files
//   - docs:       Output AI-readable documentation
//   - completion: Generate shell completion scripts
//   - version:    Show hitspec version information
package cmd
