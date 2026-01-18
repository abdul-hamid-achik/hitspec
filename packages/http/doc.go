// Package http provides HTTP client functionality for hitspec test execution.
//
// It wraps the standard library's http package with additional features:
//   - Configurable timeouts
//   - Redirect handling
//   - Request building from parsed AST
//   - Response handling and body reading
//   - Multipart form data support
package http
