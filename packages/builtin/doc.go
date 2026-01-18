// Package builtin provides built-in functions for use in hitspec test files.
//
// Available functions:
//   - uuid(): Generate a random UUID v4
//   - timestamp(): Current Unix timestamp
//   - isodate(): Current date in ISO 8601 format
//   - random(min, max): Random integer in range
//   - randomString(length): Random alphanumeric string
//   - base64(value): Base64 encode a string
//   - env(name): Get environment variable value
//
// Functions are invoked using the {{$functionName(args)}} syntax in test files.
package builtin
