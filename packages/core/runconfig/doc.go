// Package runconfig resolves the settings a request runs with.
//
// hitspec reads settings from four places and every surface that executes
// requests (hitspec run, the studio, serve --api-only, the stress and contract
// engines) must agree on how they combine. Precedence, highest first:
//
//  1. an explicit override (a command-line flag, or a per-request option),
//  2. a HITSPEC_* environment variable (see FromEnv),
//  3. hitspec.yaml,
//  4. the built-in default.
//
// Build is the single place that applies it. It yields a runner.Config for
// surfaces that drive the runner and, via runner.Config.ClientOptions, the
// matching HTTP client options for surfaces that drive the HTTP layer
// themselves.
//
// A numeric or duration setting at or below zero counts as unset, because
// hitspec.yaml is decoded over the defaults and zero cannot be told apart from
// "not written" (use followRedirects: false rather than maxRedirects: 0).
package runconfig
