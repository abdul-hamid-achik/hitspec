# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
  - Available at `vscode-hitspec/` directory

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

[Unreleased]: https://github.com/abdul-hamid-achik/hitspec/compare/v1.0.1...HEAD
[1.0.1]: https://github.com/abdul-hamid-achik/hitspec/compare/v0.1.0...v1.0.1
[0.1.0]: https://github.com/abdul-hamid-achik/hitspec/releases/tag/v0.1.0
