# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.9.0] - 2026-02-10

### Fixed

- **GraphQL Variable Parsing**: Fix GraphQL variable parsing in the parser
- **Frontend UI Improvements**: Various UI polish in the web client

### Changed

- Build frontend in CI, remove dist from git tracking

## [2.8.0] - 2026-02-10

### Added

- **Live Execution Progress**: Per-request execution progress via WebSocket, showing real-time status during test runs

## [2.7.0] - 2026-02-10

### Fixed

- **WebSocket and SSE Support**: Delegate `Hijack`/`Flush` in `statusWriter` for proper WebSocket and SSE proxying
- Remove unused `requestIDFromContext` to pass lint

### Added

- **Single-Request Execution**: Fix and wire single-request execution from the UI
- **Export Wiring**: Wire export commands end-to-end
- **Structured Logging**: Add structured logging support

## [2.6.0] - 2026-02-10

### Added

- **Persistent Run History**: Record test runs to SQLite via `sqlc` + `modernc.org/sqlite` (pure Go, no CGo)
  - Schema: `runs` -> `results` -> `assertions` with CASCADE deletes
  - CLI flags: `--history-db`, `--no-history`
  - Server endpoints: `GET/DELETE /api/runs`, `GET /api/runs/:id`
  - Client: paginated history view with run/result/assertion drill-down
  - Non-blocking goroutine recording (doesn't slow test execution)

### Fixed

- File tree collapse not working due to missing reactivity trigger
- Rebuilt embedded client assets with history feature

## [2.5.0] - 2026-02-09

### Added

- **CLI DX Improvements**: Better CLI developer experience with improved defaults and help text
- **Parser Split**: Split parser into 6 focused files for maintainability
- **Runner Split**: Split runner into 4 files (runner, execute, filter, condition)

### Fixed

- Documentation accuracy and auth bug fixes
- UX polish across CLI and web client

## [2.4.1] - 2026-02-09

### Fixed

- **UTF-8 Lexer**: Lexer now handles multi-byte UTF-8 characters correctly (rune-based)
- **Dead Code Cleanup**: Remove unused code across packages
- **Contract Alignment**: Align contract testing with spec

### Added

- 112 new tests across packages

## [2.4.0] - 2026-02-09

### Added

- **Run Results Explorer**: Browse and inspect past test run results in the web client
- **Export Dialog**: UI dialog for exporting requests to curl, wget, and httpie
- **Sidebar Request Navigation**: Navigate requests directly from the sidebar

## [2.3.0] - 2026-02-09

### Fixed

- Add `doc.go` package comments to all internal packages for linter compliance

### Added

- **P0 Feature Wiring**: Wire all priority-0 features end-to-end
- **Security Hardening**: Input validation, path traversal protection, secure defaults
- **UX Polish**: Improved error messages, loading states, and feedback
- **Docs Accuracy**: Fix documentation to match actual behavior

## [2.2.1] - 2026-02-09

### Fixed

- **Race Conditions**: Fix data races in concurrent test execution
- **Dead Code**: Remove unused functions and types
- **Missing Endpoints**: Add endpoints that were documented but not implemented
- **E2E Fixes**: Fix end-to-end test reliability
- Resolve 63 bugs across client and server from two-pass audit

## [2.2.0] - 2026-02-09

### Added

- **Major Client Enhancement**: Redesigned web client with improved editor DX
- **Documentation Expansion**: Comprehensive docs for all features
- **Security Fixes**: Address OWASP top-10 issues in server

## [2.1.0] - 2026-02-09

### Added

- **Run Results Explorer**: Browse and inspect test run results
- **Export Dialog**: Export requests to curl, wget, httpie from the UI
- **Sidebar Request Navigation**: Quick navigation to requests

## [2.0.2] - 2026-02-09

### Fixed

- Use `defaultEnvironment` from `hitspec.yaml` when `--env` is not set
- Align execute API fields between client and server
- Add "Run All" button and clear history functionality

## [2.0.1] - 2026-02-09

### Fixed

- Remove unused `rootFile` type to satisfy linter
- Return file tree structure from workspace API so UI renders files correctly

## [2.0.0] - 2026-02-09

### Added

- **Web Client (hitspec serve)**: Browser-based API Client Manager
  - Built with Vue 3, Pinia, Reka UI, and CodeMirror
  - Embedded into Go binary via `go:embed`
  - Proxy to Vite dev server in dev mode
- **Mintlify Documentation Site**: Full documentation at `apps/docs/`
- **hitspec serve Command**: Start a local server with the web client

### Fixed

- Pin Go 1.25.7 to resolve crypto/tls vulnerability GO-2026-4337
- Resolve lint issues in serve package
- Fix `docs.json` for new Mintlify v2 schema

## [1.3.0] - 2026-02-05

### Added

- **Security Flags**: `--insecure` and TLS configuration options
- **DX Improvements**: Better error messages, auto-detection of file format
- **Mintlify Documentation**: Initial documentation site setup

## [1.2.3] - 2026-02-04

### Added

- **Export to Curl**: New `hitspec export curl` command to convert `.http` files to executable curl commands
  - Filter by request name with glob patterns (`--name "Login*"`)
  - Filter by tags (`--tags smoke,auth`)
  - Output to file (`-o commands.sh`) or stdout
  - Execute curl directly (`--exec`, requires single request)
  - Verbose curl output (`--verbose`)
  - Environment variable resolution (`--env staging`)
  - Handles all auth types (bearer, basic, apiKey, digest)
  - Handles all body types (JSON, form, multipart, GraphQL)

### Fixed

- **Escaped Quotes in JSON Body**: Escaped characters (like `\"`) inside JSON body values are now correctly preserved
  - Previously, `{"content": "{\"test\": true}"}` would become malformed JSON
  - Now escape sequences (`\"`, `\\`, `\n`, `\t`, etc.) are properly maintained when sent to the server
  - This fixes issues with APIs that accept JSON-in-JSON payloads

## [1.2.2] - 2026-02-04

### Changed

- Updated README with length comparison operators documentation
- Updated CHANGELOG with v1.2.1 release notes

## [1.2.1] - 2026-02-04

### Added

- **Length Comparison Operators**: Support comparison operators with `length` assertions
  - `expect body.items length > 0` - greater than
  - `expect body.items length >= 1` - greater than or equal
  - `expect body.items length < 100` - less than
  - `expect body.items length <= 50` - less than or equal

### Fixed

- **Variable Interpolation in Expect Clauses**: Captured variables like `{{createItem.itemId}}` now resolve in expect clause expected values
  - Previously, variables in expected values were compared as literal strings
  - Now `expect body.id == "{{login.userId}}"` correctly compares against the captured value

## [1.2.0] - 2026-01-20

### Added

- **Snapshot Testing**: Assert response bodies against saved snapshots with `expect body snapshot "name"` syntax
  - Store snapshots in `__snapshots__/` directory
  - Update with `--update-snapshots` flag
- **curl Import**: Import curl commands to hitspec format with `hitspec import curl "curl ..."`
  - Supports common flags: `-X`, `-H`, `-d`, `--data`, `-u`, `-k`, `-L`
  - Convert clipboard or file with `hitspec import curl @commands.txt`
- **Insomnia Import**: Import Insomnia export files with `hitspec import insomnia collection.json`
  - Supports Insomnia v4 export format
  - Converts variables, authentication, and request bodies
- **SSE (Server-Sent Events) Support**: Test SSE endpoints with dedicated syntax
  - Stream events with timeout configuration
  - Assert on event data, type, and count
- **API Coverage Reporting**: Measure API test coverage against OpenAPI specs
  - `--coverage` flag enables coverage tracking
  - `--openapi` flag specifies OpenAPI spec file
  - Generate HTML, JSON, or console reports with `--coverage-output`
- **Custom Annotations**: Define custom metadata with `@x-custom` or namespaced annotations like `@contract.state`
- **VSCode Extension**: Syntax highlighting and code snippets for `.http` and `.hitspec` files
  - TextMate grammar for syntax highlighting
  - Snippets for common patterns (requests, assertions, captures)

### Changed

- **Response Diff on Failure**: Console output now shows JSON diff for assertion failures in verbose mode
  - Added/removed/changed values highlighted with colors
  - Only differing paths displayed, not entire response bodies

## [1.0.1] - 2026-01-19

### Fixed

- Stress runner now extracts captures from setup requests, enabling variable chaining
- Requests with unresolved variables are skipped instead of being sent with literal `{{variable}}` strings
- Added `HasUnresolvedVariables` and `GetUnresolvedVariables` methods to env.Resolver for variable validation

## [0.1.0] - 2024-01-18

### Added

- Initial release of hitspec
- File-based HTTP API testing with `.http` and `.hitspec` files
- Variable interpolation with `{{variable}}` syntax
- Request chaining with captures
- Assertions for status, headers, body, and JSONPath
- Multiple environment support via `.env` files
- Built-in functions: `uuid()`, `timestamp()`, `random()`, and more
- Multiple output formats: console, JSON, JUnit, TAP
- Parallel test execution with `--parallel` flag
- Watch mode for automatic re-runs with `--watch` flag
- Request dependencies with `@depends` directive
- Request retry support with `@retry` directive
- Tag-based test filtering with `--tags` flag
- Name-based test filtering with `--name` flag
- Multipart form data support
- JSON Schema validation for responses
- Bail on first failure with `--bail` flag

### Commands

- `hitspec run` - Run API tests
- `hitspec validate` - Validate test file syntax
- `hitspec list` - List all tests in files
- `hitspec init` - Initialize a new project
- `hitspec version` - Show version information

[Unreleased]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.9.0...HEAD
[2.9.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.8.0...v2.9.0
[2.8.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.7.0...v2.8.0
[2.7.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.6.0...v2.7.0
[2.6.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.5.0...v2.6.0
[2.5.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.4.1...v2.5.0
[2.4.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.4.0...v2.4.1
[2.4.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.3.0...v2.4.0
[2.3.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.2.1...v2.3.0
[2.2.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.2.0...v2.2.1
[2.2.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.0.2...v2.1.0
[2.0.2]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.0.1...v2.0.2
[2.0.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v1.3.0...v2.0.0
[1.3.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v1.2.3...v1.3.0
[1.2.3]: https://github.com/abdul-hamid-achik/hitspec/compare/v1.2.2...v1.2.3
[1.2.2]: https://github.com/abdul-hamid-achik/hitspec/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v1.0.1...v1.2.0
[1.0.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v0.1.0...v1.0.1
[0.1.0]: https://github.com/abdul-hamid-achik/hitspec/releases/tag/v0.1.0
