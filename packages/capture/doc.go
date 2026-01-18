// Package capture extracts values from HTTP responses for use in subsequent requests.
//
// It supports capturing values from:
//   - Response body (JSON paths)
//   - Response headers
//   - Response status code
//
// Captured values can be used in later requests via the {{requestName.captureName}} syntax,
// enabling request chaining and dependent test scenarios.
package capture
