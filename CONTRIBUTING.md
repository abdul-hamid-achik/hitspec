# Contributing to hitspec

Thank you for your interest in contributing to hitspec! This document provides guidelines and instructions for contributing.

## Prerequisites

- **Go 1.26** — [golang.org/dl](https://go.dev/dl/) (`go.mod` pins the `go1.26.9` toolchain, which Go downloads automatically; asdf users get it the same way through `.tool-versions`, which only needs a Go 1.26 release)
- **Task** — [taskfile.dev](https://taskfile.dev/) — task runner for all dev commands
- Optional: **Node.js 24** for the docs (`task docs:check`) and the desktop app (`apps/desktop`), `sqlite3` for the database example, [`glyph`](https://github.com/abdul-hamid-achik/glyphrun) for the studio e2e specs (`task e2e`)

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
   This downloads the Go dependencies and runs code generation.

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
# Run all Go tests (race detector + coverage)
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

### Binary smoke tests and docs checks

Unit tests never looked at what the built binary prints, which is how a silent
`hitspec run` shipped. CI therefore also runs the real binary:

```bash
# init -> validate -> mock -> run, then every example in examples/ against its
# own >>>mock blocks (same script CI runs)
task smoke

# Parse every ```http block in apps/docs, README.md, AGENTS.md and llms.txt, and
# check `hitspec <cmd> --flag` lines in shell blocks against the binary's --help
task docs:snippets

# Studio TUI specs through a real PTY (needs glyph)
task e2e
```

If a docs sample is intentionally partial or invalid, put `{/* docsnippets: skip */}`
(`.mdx`) or `<!-- docsnippets: skip -->` (`.md`) on the line above the fence.
When you change `llms.txt`, copy it to `apps/cli/cmd/llms.txt` (a test checks they match).

## Terminal UI Development

The API Client Manager is a native terminal UI (Charm Bubble Tea v2) in
`packages/tui/`, driven by the in-process `packages/clientmgr` facade. There is
no web build step — it ships inside the Go binary.

```bash
# Run the TUI locally
task studio:dev         # == go run ./apps/cli studio

# Test + lint the TUI
go test -race ./packages/tui/...
golangci-lint run ./packages/tui/...

# Regenerate golden snapshots after intentional render changes
go test ./packages/tui/ -run Golden -update
```

`hitspec serve --api-only` (`task serve:dev`) runs the legacy REST/WebSocket server
(`packages/serve`) for integrations. See `AGENTS.md` for the TUI architecture and keybindings.

## Code Generation

Some packages use generated code (e.g. `sqlc` for the history package). After modifying SQL queries or schemas:

```bash
go generate ./...
# or
task generate
```

## Editor Extensions

- **VSCode** — `apps/vscode/` — TextMate grammar and snippets for `.http`/`.hitspec` files (published by `.github/workflows/vscode-publish.yml` once the Marketplace tokens exist)
- **Neovim** — `apps/nvim/` — Vim syntax file, filetype detection and LuaSnip snippets (the plugin lives in this subdirectory; see its README for the `runtimepath` setup)
- **Desktop** — `apps/desktop/` — Electron client that spawns `hitspec serve --api-only`

Keep grammars and snippets to syntax the parser actually has: hitspec has no `WS`
method and no `@import` directive.

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
│   ├── docs/          # Mintlify documentation
│   ├── desktop/       # hitspec Studio (Electron)
│   ├── vscode/        # VSCode extension
│   └── nvim/          # Neovim plugin
├── packages/
│   ├── core/          # Parser, runner, config, env
│   ├── tui/           # Native terminal UI (Charm Bubble Tea v2)
│   ├── clientmgr/     # In-process API Client Manager facade
│   ├── http/          # HTTP client
│   ├── output/        # Formatters (console, json, junit, tap, html)
│   ├── assertions/    # Test assertions
│   ├── capture/       # Response capture
│   ├── builtin/       # Built-in functions
│   ├── serve/         # REST/WebSocket API (hitspec serve --api-only)
│   ├── stress/        # Stress/load testing
│   ├── mock/          # Mock server
│   ├── proxy/         # Recording proxy
│   ├── import/        # Importers (curl, insomnia, openapi, postman)
│   ├── export/        # Exporters (curl, fetch, wget, python, httpie, go, ruby)
│   ├── fetch/         # One-shot bounded HTTP fetch
│   ├── mcp/           # MCP server (hitspec mcp serve)
│   ├── mcpclient/     # MCP client (hitspec mcp probe/call)
│   ├── search/        # Web search providers for the MCP server
│   ├── artifact/      # Result artifact stores
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
│   ├── conv/          # Numeric conversion
│   ├── testhome/      # Isolates HOME for tests that touch ~/.hitspec
│   └── watchutil/     # File watching helpers
├── tools/
│   └── docsnippets/   # Docs snippet checker (go run ./tools/docsnippets)
├── e2e/               # glyphrun specs for the studio TUI
├── examples/          # Offline-runnable example test files
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
