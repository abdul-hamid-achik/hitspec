# Contributing to hitspec

Thank you for your interest in contributing to hitspec! This document provides guidelines and instructions for contributing.

## Prerequisites

- **Go 1.25+** — [golang.org/dl](https://go.dev/dl/)
- **Bun** (latest) — [bun.sh](https://bun.sh/) — used for the web client
- **Task** — [taskfile.dev](https://taskfile.dev/) — task runner for all dev commands

## Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/hitspec.git
   cd hitspec
   ```
3. Bootstrap everything:
   ```bash
   task setup
   ```
   This installs Go and frontend dependencies, builds the client, and runs code generation.

4. Verify the build:
   ```bash
   task build
   ```

5. Run tests:
   ```bash
   task test
   ```

## Code Style

- Run `task lint` before submitting (CI will check this)
- Run `task check` to run all checks (lint, vet, tests)
- Follow existing patterns in the codebase
- Keep functions focused and small
- Write descriptive variable and function names
- Add comments only for complex logic

## Testing

### Running Tests

```bash
# Run all tests (Go + client)
task test

# Run only Go tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with race detection
go test -race ./...

# Run specific package tests
go test ./packages/core/parser/...
```

### Writing Tests

- Place test files next to the code they test (`*_test.go`)
- Use table-driven tests for multiple cases
- Test edge cases and error conditions
- Use the `testdata/` directory for test fixtures

## Frontend Development

The web client lives in `apps/client/` and is built with Vue 3, Pinia, and Reka UI.

```bash
# Start the dev server (with hot reload)
task client:dev

# Build for production (output goes to packages/serve/dist/)
task client:build

# Type-check
cd apps/client && bun run type-check
```

The built client is embedded into the Go server binary via `go:embed`.

## Code Generation

Some packages use generated code (e.g. `sqlc` for the history package). After modifying SQL queries or schemas:

```bash
go generate ./...
# or
task generate
```

## Editor Extensions

- **VSCode** — `apps/vscode/` — syntax highlighting and snippets for `.http`/`.hitspec` files
- **Neovim** — `apps/nvim/` — Tree-sitter grammar and ftdetect for Neovim

## Pull Request Process

1. Create a branch for your changes:
   ```bash
   git checkout -b feature/my-feature
   ```

2. Make your changes following the code style guidelines

3. Write or update tests to cover your changes

4. Run all checks:
   ```bash
   task check
   ```

5. Commit your changes with a clear message:
   ```bash
   git commit -m "feat: description of the feature"
   ```

6. Push to your fork:
   ```bash
   git push origin feature/my-feature
   ```

7. Open a Pull Request against the `main` branch

### PR Guidelines

- Keep PRs focused on a single change
- Include a clear description of what and why
- Reference any related issues
- Ensure CI passes before requesting review
- Be responsive to feedback

## Project Structure

```
hitspec/
├── apps/
│   ├── cli/           # CLI application (Cobra commands)
│   ├── client/        # Vue 3 + Pinia web client (Bun/Vite)
│   ├── docs/          # Mintlify documentation
│   ├── vscode/        # VSCode extension
│   └── nvim/          # Neovim plugin
├── packages/
│   ├── core/          # Parser, runner, config, env
│   ├── http/          # HTTP client
│   ├── output/        # Formatters (console, json, junit, tap, html)
│   ├── assertions/    # Test assertions
│   ├── capture/       # Response capture
│   ├── builtin/       # Built-in functions
│   ├── serve/         # HTTP server (embeds web client)
│   ├── stress/        # Stress/load testing
│   ├── mock/          # Mock server
│   ├── proxy/         # Recording proxy
│   ├── import/        # Importers (curl, insomnia, openapi)
│   ├── export/        # Exporters (curl, wget, httpie)
│   ├── snapshot/      # Snapshot testing
│   ├── sse/           # Server-Sent Events
│   ├── contract/      # Contract testing
│   ├── coverage/      # API coverage
│   ├── history/       # Run history (SQLite)
│   ├── db/            # Database assertions
│   ├── notify/        # Notifications
│   └── auth/          # Authentication (OAuth2)
├── internal/
│   ├── pathutil/      # Path validation
│   └── conv/          # Numeric conversion
├── examples/          # Example test files
└── testdata/          # Test fixtures
```

## Reporting Issues

- Search existing issues before creating a new one
- Use the issue template if available
- Include reproduction steps
- Include version information (`hitspec version`)
- Include relevant error messages

## Questions?

Feel free to open an issue for questions about contributing or the codebase.
