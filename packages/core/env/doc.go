// Package env handles environment variables and variable resolution for hitspec.
//
// It provides functionality for:
//   - Loading environment files (.env, .env.local, etc.)
//   - Variable interpolation using {{variable}} syntax
//   - Built-in function evaluation (uuid, timestamp, random, etc.)
//   - Capturing and resolving values from previous requests
//   - Environment-specific variable loading
package env
