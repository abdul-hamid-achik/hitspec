# AGENTS.md - AI Assistant Guide for hitspec

## Project Overview

hitspec is a file-based HTTP API testing tool written in Go. Users write `.http`
(or `.hitspec`) files with requests and assertions, and hitspec executes them. It
also provides a **native terminal API Client Manager (TUI)** via `hitspec serve`
— a keyboard-first, Postman-like interface that stays true to the plain-text,
git-friendly file format.

> History: the API Client Manager used to be a Vue.js single-page app served over
> HTTP. That web client was removed and replaced by an in-process facade
> (`packages/clientmgr`) plus a Charm Bubble Tea v2 TUI (`packages/tui`). The
> legacy REST/WebSocket server still exists for `hitspec serve --api-only`.

## Architecture

```
hitspec/
├── apps/
│   ├── cli/                # CLI application (Cobra commands)
│   │   └── cmd/            # Commands: run, validate, list, init, serve, mock, record, etc.
│   ├── docs/               # Mintlify documentation site
│   ├── vscode/             # VSCode extension (syntax highlighting + snippets)
│   └── nvim/               # Neovim plugin (syntax + ftdetect)
├── packages/
│   ├── core/
│   │   ├── parser/         # .http file parser (lexer, AST, parser)
│   │   ├── runner/         # Test execution engine
│   │   ├── env/            # Variable/environment resolution
│   │   └── config/         # Configuration loading (hitspec.yaml)
│   ├── clientmgr/          # Transport-independent API Client Manager facade (drives the TUI)
│   ├── tui/                # Native Charm Bubble Tea v2 terminal UI (hitspec serve)
│   ├── serve/              # REST/WebSocket API server for `hitspec serve --api-only`
│   ├── http/               # HTTP client and request/response types
│   ├── assertions/         # Assertion evaluation (26 operators)
│   ├── capture/            # Response value capturing
│   ├── output/             # Output formatters (console, JSON, JUnit, TAP, HTML)
│   ├── stress/             # Stress testing engine
│   ├── mock/               # Mock server from .http files
│   ├── proxy/              # Recording proxy
│   ├── import/             # Importers (curl, Insomnia, OpenAPI, Postman)
│   ├── export/             # Exporters (curl + fetch/wget/python/httpie/go/ruby snippets)
│   ├── builtin/            # Built-in functions ($uuid, $timestamp, etc.)
│   ├── snapshot/           # Snapshot testing
│   ├── sse/                # Server-Sent Events support
│   ├── contract/           # Contract testing
│   ├── history/            # SQLite-backed persistent run history (sqlc-generated)
│   ├── db/                 # Database assertion support
│   ├── auth/oauth2/        # OAuth2 token acquisition
│   └── notify/             # Slack/Teams notifications
├── internal/
│   ├── pathutil/           # Path validation helpers
│   └── conv/               # Numeric conversion helpers
└── examples/               # Example .http files
```

## Key Files

| Task | Files |
|------|-------|
| Add assertion operator | `packages/core/parser/ast.go` (add to OpXxx), `packages/assertions/evaluator.go` (implement) |
| Add built-in function | `packages/builtin/functions.go` |
| Add CLI command | `apps/cli/cmd/` (new file), `apps/cli/cmd/root.go` (register) |
| Add CLI flag to run | `apps/cli/cmd/run.go` |
| Fix request parsing | `packages/core/parser/parser.go`, `packages/core/parser/lexer.go` |
| Fix HTTP client | `packages/http/client.go`, `packages/http/request.go` |
| Add output format | `packages/output/` |
| Add a Manager operation (TUI capability) | `packages/clientmgr/*.go` (method + DTO in `types.go`) |
| Add a TUI screen/command | `packages/tui/app.go` (model, keys, `executeCommand`, `secondary*`) |
| Add a TUI component | `packages/tui/<component>.go` (own file, e.g. `responseviewer.go`, `toast.go`) |
| Regenerate TUI golden snapshots | `go test ./packages/tui/ -run Golden -update` |
| Add serve (`--api-only`) endpoint | `packages/serve/handler_*.go`, `packages/serve/routes.go`, `packages/serve/types.go` |
| Update docs | `apps/docs/` (.mdx files, docs.json) |
| Add stress feature | `packages/stress/stress.go`, `packages/stress/metrics.go` |
| Add mock feature | `packages/mock/server.go`, `packages/mock/router.go` |

## Monorepo Structure

| Directory | Purpose | Language | Build Tool |
|-----------|---------|----------|------------|
| `apps/cli` | CLI binary (entry point) | Go | `go build` |
| `apps/docs` | Documentation site | MDX | Mintlify |
| `packages/tui` | Native terminal UI (Bubble Tea v2) | Go | `go build` |
| `packages/clientmgr` | In-process API Client Manager facade | Go | - |
| `packages/serve` | REST/WebSocket API (`--api-only`) | Go | `go build` |
| `packages/core` | Parser + runner | Go | - |
| `packages/*` | Feature packages | Go | - |

## `hitspec serve` Architecture

```
hitspec serve [file|dir]
     │
     ├── (default)    packages/tui  → Charm Bubble Tea v2 TUI
     │                                drives packages/clientmgr.Manager (in-process)
     │
     └── --api-only   packages/serve → net/http server (REST + WebSocket), no UI
                                       wraps the same clientmgr operations
```

Both surfaces (TUI and `--api-only` REST) sit on top of the same
`clientmgr.Manager`, so behavior stays consistent. The `Manager` is
transport-independent: ~50 methods covering files, execute/run, environments,
config, stress, mock, contract, record, import/export, cookies, and history.
It publishes realtime `Event`s (file_changed, request_progress, stress_update,
mock_request, environment_changed) that the TUI bridges into the Bubble Tea
message loop.

### REST API Endpoints (`--api-only`)

These are served only when `hitspec serve --api-only` is used.

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/v1/workspace | Workspace info and file stats |
| GET | /api/v1/files | List .http/.hitspec files |
| POST | /api/v1/files | Create a new file |
| GET | /api/v1/files/raw/{path...} | Get raw file content (text/plain) |
| GET | /api/v1/files/{path...} | Parse file as structured JSON |
| PUT | /api/v1/files/{path...} | Save raw file content |
| DELETE | /api/v1/files/{path...} | Delete a file |
| POST | /api/v1/execute | Execute single request |
| POST | /api/v1/run | Run all requests in a file |
| GET | /api/v1/environments | List environments |
| PUT | /api/v1/environments/active | Set active environment |
| GET | /api/v1/environments/{name} | Get environment variables |
| PUT | /api/v1/environments/{name} | Update environment |
| GET | /api/v1/config | Get hitspec.yaml config |
| PUT | /api/v1/config | Update config (persists to hitspec.yaml) |
| GET | /api/v1/history/runs | List persistent runs (SQLite) |
| GET | /api/v1/history/runs/{id} | Get run details with results |
| DELETE | /api/v1/history/runs | Delete all persistent runs |
| DELETE | /api/v1/history/runs/{id} | Delete a specific run |
| POST | /api/v1/stress/start | Start stress test |
| POST | /api/v1/stress/stop | Stop stress test |
| GET | /api/v1/stress/status | Stress test status/metrics |
| GET | /api/v1/stress/result | Last stress test result |
| GET/POST/PUT/DELETE | /api/v1/stress/profiles | Manage stress profiles |
| POST | /api/v1/mock/start, /stop | Mock server control |
| GET | /api/v1/mock/routes | List mock routes |
| POST | /api/v1/contract/verify | Verify API contracts |
| POST | /api/v1/record/start, /stop | Recording proxy control |
| GET | /api/v1/record/status | Recording proxy status |
| POST | /api/v1/record/export | Export recordings as .http |
| DELETE | /api/v1/record/clear | Clear recordings |
| POST | /api/v1/import/{curl,insomnia,openapi} | Importers |
| POST | /api/v1/export/curl | Export as curl |
| GET | /api/v1/system/info | Version and build info |
| GET | /api/v1/ws | WebSocket connection |

## TUI Architecture (`packages/tui`)

Idiomatic Bubble Tea v2 (Elm architecture). Side effects live in `tea.Cmd`
closures; `View()` is pure (no manager I/O — status is cached on the Update path
via `loadScreenState`).

| File | Responsibility |
|------|----------------|
| `app.go` | Root model, `Update`, `View`/`render`, keymap, per-screen logic, commands |
| `run.go` | `Run(ctx, mgr, Options)` entry point; starts the `tea.Program` |
| `theme.go` | Nord palette + `styles` struct |
| `responseviewer.go` | Tabbed response viewer (Body/Headers/Assertions/Timing/Captures) |
| `highlight.go` | chroma syntax highlighting + JSON pretty-print helpers |
| `clipboard.go` | Copy/export request as curl/httpie/python/fetch/go (via `Manager.Export`) |
| `toast.go` | Severity-colored auto-dismissing notifications |
| `overlay.go` | lipgloss v2 `Compositor` helpers for floating overlays (preserve background) |
| `confirm.go` | Modal yes/no dialog for destructive actions |
| `history_screen.go` | Interactive run history: list → `GetRun` detail → `DeleteRun` |
| `*_test.go`, `testdata/*.golden` | Unit + golden-snapshot tests |

### TUI keybindings (default)

- **Screens**: `1`-`9` (workspace, stress, mock, contract, record, history, import, cookies, settings)
- **Global**: `ctrl+p` command palette · `ctrl+e` environment switcher · `?` help · `q`/`ctrl+c` quit
- **Workspace**: `tab`/`shift+tab` cycle panes · `enter` open file · `e` edit source · `ctrl+s` save · `r` run request · `R` run file · `n` new file · `D` delete file (confirmed) · `ctrl+r` refresh
- **Response pane**: `←`/`→` or `[`/`]` switch tabs
- **Secondary forms**: `e` edit fields · `tab`/`shift+tab` move field · `enter`/`s` submit · `esc` cancel · `x` stop · `E` export (record screen)
- **History**: `enter` details · `D` delete (confirmed) · `esc` back · `ctrl+r` refresh
- **Confirm dialog**: `y`/`enter` confirm · `n`/`esc` cancel

### TUI gotcha: the `transitioned` guard

`Update` resets `m.transitioned = false` each tick. Any `handleKey` branch that
*consumes* a key to open/close an overlay or move focus into a text widget must
set `m.transitioned = true` (and modals also short-circuit via `m.confirm != nil`).
The guard runs **before** the overlay-forwarding blocks so an open overlay still
receives navigation keys, but the key that closes it does not leak through to a
background widget. Forgetting this re-introduces the "stray key in the editor"
class of bugs.

## .http File Syntax (Quick Reference)

```http
@variable = value                    # Variable definition
{{variable}}                         # Variable interpolation
{{$uuid()}}                          # Built-in function

### Request Name                     # Request separator
# @name identifier                   # Request identifier (for captures/deps)
# @tags smoke, auth                  # Tags for filtering
# @depends otherRequest              # Dependency
# @timeout 5000                      # Timeout in ms
# @auth bearer {{token}}             # Authentication

GET {{baseUrl}}/path                 # HTTP method and URL
Header: Value                        # Headers

{json body}                          # Request body

>>>                                  # Assertion block start
expect status 200                    # Assertions
expect body.field == "value"
expect body[0].id exists
<<<                                  # Assertion block end

>>>capture                           # Capture block
token from body.access_token
<<<
```

## Assertion Operators (26)

`==`, `!=`, `>`, `>=`, `<`, `<=`, `contains`, `!contains`, `startsWith`, `endsWith`, `matches`, `exists`, `!exists`, `length`, `length >`, `length >=`, `length <`, `length <=`, `includes`, `!includes`, `in`, `!in`, `type`, `schema`, `each`, `snapshot`

## Running Tests

```bash
# All Go tests
go test ./...
go test ./packages/tui/...           # TUI package only
go test -race ./packages/tui/...     # With the race detector

# Regenerate TUI golden snapshots after intentional render changes
go test ./packages/tui/ -run Golden -update

# Lint (the repo config is golangci-lint v2; a v1 binary cannot read it)
golangci-lint run ./...

# Build
task build                           # CLI binary
go build ./...                       # Everything

# Development
task serve:dev                       # Run the native TUI locally
task docs:dev                        # Mintlify docs locally
task dev                             # Run with the petstore example
go run ./apps/cli run examples/      # Run example files
```

## Coding Conventions

- Standard Go formatting (gofmt); keep the package lint-clean (`golangci-lint run`).
- Error handling: return errors, don't panic.
- Functional options pattern for config: `type Option func(*Config)`.
- Package-level `doc.go` files for documentation.
- Test files: `*_test.go` in the same package.
- TUI: `Update` mutates a value-receiver copy — pointer-receiver helpers that
  mutate `m` work because the local is addressable; keep `View()` pure (no I/O);
  isolate side effects in `tea.Cmd` closures; build reusable widgets in their own
  files; honor the `transitioned` guard (see above).

## Common Patterns

### Adding a TUI capability + surfacing it

1. Add the operation to `packages/clientmgr` (a `Manager` method + DTOs in `types.go`).
2. Wire a `tea.Cmd` in `packages/tui/app.go` that calls it and returns a typed `Msg`.
3. Handle the `Msg` in `Update` (update state, push a toast on error/success).
4. Add a command-palette entry (`buildCommandItems` + `executeCommand`) and/or a key binding.
5. Add a test (direct `Update`/command test; golden snapshot for new rendering).

### Adding an `--api-only` REST endpoint

1. Add DTO types to `packages/serve/types.go` (or reuse `clientmgr` DTOs).
2. Create a handler in `packages/serve/handler_*.go`.
3. Register the route in `packages/serve/routes.go`.

### Adding a New Assertion Operator

1. Add a constant to `packages/core/parser/ast.go`:
   ```go
   OpNewOperator AssertionOperator = "newoperator"
   ```
2. Add parsing in `packages/core/parser/parser.go` `parseAssertion()`.
3. Implement in `packages/assertions/evaluator.go`:
   ```go
   case parser.OpNewOperator:
       return e.newOperator(actual, expected)
   ```

### Adding a Built-in Function

1. Add to `packages/builtin/functions.go`:
   ```go
   "newFunc": func(args ...string) string {
       // implementation
   }
   ```

## Documentation

- [Mintlify Docs](apps/docs/) - Full documentation site
- [CLI Reference](apps/docs/reference/) - All commands and flags (Mintlify MDX)
- [LLM Reference](llms.txt) - Complete syntax reference for AI assistants
