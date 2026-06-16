# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.14.1] - 2026-06-15

### Fixed

- **`studio` TUI layout and input**: bounded the workspace layout so panes no longer overflow on small terminals, fixed form-field input handling, and made `ctrl+c` quit reliably.

### Changed

- Hardened the end-to-end suite: reorganized glyphrun specs into `flows`/`actions`/`fixtures`, isolated `HOME` from real user data, fixed file-watcher status-clobber flakes, and added stress/record/import/clear-history flows. Dropped the redundant cairntrace browser suite. Raised TUI unit coverage to ~85%.

## [2.14.0] - 2026-06-14

### Changed

- **API Client Manager is now a native interactive terminal app** launched with **`hitspec studio`** (the Vue.js web SPA was removed and replaced by a keyboard-first Charm Bubble Tea v2 app). `hitspec serve --api-only` runs the REST/WebSocket API server; `hitspec serve` without `--api-only` still opens the app for backward compatibility but prints a hint to use `hitspec studio`. The app is driven by a new in-process `clientmgr` facade shared by both surfaces.
- **Removed the deprecated `--open` flag** from `serve`, and scrubbed user-facing "TUI"/"web-based" jargon from CLI help, `init` next-steps, README, and docs.

### Added

- **Tabbed response viewer**: Body / Headers / Assertions / Timing / Captures, with pretty-printed and syntax-highlighted JSON (chroma) and status colored by class.
- **Copy/export as code**: render the selected request as curl / HTTPie / Python / fetch / Go to the system clipboard, plus copy the response body (command palette).
- **Toast notifications**: severity-colored, auto-dismissing — errors and successes no longer vanish on the next keypress.
- **Confirm dialogs**: destructive actions (delete file/cookie/run, clear recordings/history) now require confirmation.
- **Environment switcher** (`ctrl+e`) and **environment-variable editing** on the settings screen (read-modify-write so other variables are preserved).
- **Interactive run history**: select a run, drill into per-request details (`GetRun`), and delete runs.
- **Full stress results**: percentiles and per-request breakdown shown after a load test completes.
- **Color themes**: choose between Nord, Catppuccin Mocha, Dracula, Tokyo Night, and Gruvbox Dark via the theme picker (`ctrl+t`), the command palette, or the `--theme` flag. Per-method color badges and status-by-class coloring track the active theme.
- **Welcoming first-run experience**: an empty workspace now shows a centered welcome card with next steps, including a one-key "generate sample project" (`g`) that scaffolds `hitspec.yaml` + `example.http` (shared with `hitspec init`).
- **Centered keyboard help overlay** (`?`): a sectioned, aligned shortcut reference replaces the cramped single-line help.
- **Screen navigation strip + context-aware hints**: a numbered (1–9) screen tab strip and a status bar that shows the keys relevant to the focused pane.

### Fixed

- **studio input routing**: a key that opened a modal or moved focus into an editor is no longer re-delivered to that widget; modals now capture all input.
- **Pure render path**: the stress/mock/record screens no longer perform manager I/O during `View()` (status is cached on the update path).
- **Errors persist until resolved**: a status-bar error no longer vanishes on the next keypress — it stays until you navigate or the next operation succeeds, and is truncated to fit instead of overflowing.
- **Quit protects unsaved work**: pressing `q` with unsaved edits now asks for confirmation (`ctrl+c` still hard-quits); the command palette and overlays render consistently over the workspace; mock/record screens and the search overlay show actionable guidance and result counts; secondary form fields and viewports adapt to small terminals.
- **Cookie capture**: `Set-Cookie` headers from responses are now stored correctly (capture previously read redacted headers and never recorded a cookie).
- **Stale selection**: deleting the selected file no longer leaves its name in the top bar/source pane.
- **History database**: enabled `busy_timeout` and a single connection to avoid `SQLITE_BUSY` when run recording overlaps reads/deletes.
- **Stress metrics**: fixed a data race between `StressStatus` and a running test.

## [2.13.0] - 2026-02-27

### Fixed

- **Neovim: Operators not highlighted**: `syn keyword` silently ignored non-keyword chars (`==`, `!=`); switched to `syn match`
- **Neovim: Assertion subjects never highlighted**: `containedin=` on a `syn keyword` is ineffective; redesigned as full-line `syn match` with `contains=`
- **Neovim: `contains` keyword error**: `contains` is a reserved Vim syntax argument; switched operator keywords to `syn match`
- **VSCode: Separators rendered as comments**: Comment pattern matched `###` lines before separator pattern could; fixed pattern ordering
- **VSCode: Symbolic operators never matched**: `\b` word boundary fails on `==`, `!=`, `>=`, `<=`, `>`, `<`; split into separate patterns
- **Both: `@waitFor` not recognized**: Annotation pattern used lowercase `waitfor`; fixed to match camelCase `waitFor`
- **Both: Query params/form data not highlighted**: Pattern required leading whitespace; real files use column 0
- **VSCode: Global operator false positives**: Removed top-level operators rule that matched `=`, `>`, `<` everywhere

### Added

- **Neovim/VSCode: File include syntax**: Highlighting for `< ./path/to/file` body references
- **Neovim/VSCode: Type keywords**: `string`, `number`, `boolean`, `array`, `object` in assertions
- **Neovim: Multipart keywords**: `field` and `file` keywords in `>>>multipart` blocks
- **Neovim: Database keyword**: `query` keyword in `>>>db` blocks
- **Both: Negative number support**: `-42` and `-3.14` now highlighted as numbers
- **VSCode: Boolean and null literals**: `true`, `false`, `null` highlighting
- **VSCode: Legacy capture markers**: `[[[` and `]]]` block delimiters
- **VSCode: `headers` (plural)**: Added as assertion subject alongside `header`
- **Neovim: Variables in annotation values**: `{{var}}` inside `# @auth bearer {{token}}` now highlighted
- **Neovim: Assertion operators scoped**: Operators like `contains`, `type`, `in` now only match inside `expect` lines, preventing false positives in JSON bodies

### Changed

- VSCode built-in functions list trimmed to actual parser builtins (was bloated with 100+ nonexistent functions)

## [2.12.0] - 2026-02-27

### Fixed

- Format `types.go` with gofmt
- Update FileTree component tests for `treeitem` role

## [2.11.0] - 2026-02-11

### Added

- **File Editor Write-Back**: New Source tab in the request panel with a CodeMirror editor for editing `.http` files directly in the UI
  - `Cmd+S` / `Ctrl+S` save shortcut with dirty state indicators
  - PUT, POST, DELETE file endpoints with workspace path validation
  - File watcher self-write suppression to prevent reload loops
- **Stress Test Results Panel**: Full results dashboard shown after a stress test completes
  - Summary cards: total requests, avg RPS, success rate, errors
  - Latency percentiles: min, P50, P95, P99, max, mean, stddev
  - Per-request breakdown table with individual latency metrics
  - Threshold pass/fail indicators
  - "Run Again" button to return to config view
  - `GET /api/v1/stress/result` endpoint for fetching last test result
- **Settings Persistence**: Settings changes in the UI now persist to `hitspec.yaml` on disk
  - `PUT /api/v1/config` writes changes to the config file
  - Creates `hitspec.yaml` if none exists
- **Stress Profile CRUD**: Create, edit, and delete stress profiles from the UI
  - `POST /api/v1/stress/profiles` to create a profile
  - `PUT /api/v1/stress/profiles/{name}` to update a profile
  - `DELETE /api/v1/stress/profiles/{name}` to delete a profile
  - All changes persist to `hitspec.yaml`
  - Profile selection populates stress config fields in the UI

### Fixed

- **Diff Coloring**: Increase body preview limit from 512 bytes to 64KB so response diffs render correctly
- **Stress Test Latency**: Use fractional milliseconds (`float64(d.Microseconds()) / 1000.0`) instead of truncating integer milliseconds
- **Stop Button State**: WebSocket `stress_update` events now include a `running` boolean so the UI updates when a test finishes naturally
- **Stop Endpoint**: Return `200 OK` with `"already_stopped"` status instead of `400 Bad Request` when no test is running

## [2.10.1] - 2026-02-10

### Fixed

- Scope execution progress WebSocket events to current `execId` to prevent stale updates
- Add auto-scroll to execution progress list

## [2.10.0] - 2026-02-10

### Added

- **CI Pipeline Overhaul**: Parallel fan-out for lint, test, and security scan stages
- **Unified Taskfile**: Cross-stack `task build/test/lint/security/check` covering Go and TypeScript
- **Client Linting**: oxlint with Vue plugin (0 errors, 0 warnings on 93 files)
- **Editor Extensions**: VSCode extension at `apps/vscode/` with syntax highlighting and snippets, Neovim plugin at `apps/nvim/`
- **Neovim Support**: Vim syntax highlighting and filetype detection for `.http` and `.hitspec` files

### Changed

- Migrate Mintlify docs from `mint.json` to `docs.json` (v2 format)
- Pin Go version in `.tool-versions` for reproducible local builds
- Format all 29 Go files with `gofmt` for consistency

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

[Unreleased]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.11.0...HEAD
[2.11.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.10.1...v2.11.0
[2.10.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.10.0...v2.10.1
[2.10.0]: https://github.com/abdul-hamid-achik/hitspec/compare/v2.9.0...v2.10.0
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
