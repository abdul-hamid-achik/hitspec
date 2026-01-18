// Package parser provides parsing functionality for hitspec test files.
//
// It supports both .http and .hitspec file formats, parsing requests,
// assertions, captures, and metadata directives.
//
// The parser handles:
//   - HTTP request definitions (method, URL, headers, body)
//   - Variable declarations with @variable directive
//   - Test assertions in >>> ... <<< blocks
//   - Response captures for request chaining
//   - Metadata directives (@name, @tags, @depends, @retry, etc.)
//   - Multipart form data
package parser
