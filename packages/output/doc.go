// Package output provides formatters for displaying test results.
//
// Supported output formats:
//   - Console: Human-readable colored terminal output
//   - JSON: Machine-readable JSON output
//   - JUnit: JUnit XML format for CI integration
//   - TAP: Test Anything Protocol format
//
// Each formatter implements the Formatter interface and can optionally
// implement Flushable for formats that accumulate results before output.
package output
