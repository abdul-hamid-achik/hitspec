# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/abdul-hamid-achik/hitspec/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/abdul-hamid-achik/hitspec/releases/tag/v0.1.0
