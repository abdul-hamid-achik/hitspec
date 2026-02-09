# AGENTS.md - AI Assistant Guide for hitspec

## Project Overview

hitspec is a file-based HTTP API testing tool written in Go. Users write `.http` files with requests and assertions, and hitspec executes them. It also provides a browser-based API Client Manager via `hitspec serve`.

## Architecture

```
hitspec/
├── apps/
│   ├── cli/                # CLI application (Cobra commands)
│   │   └── cmd/            # Commands: run, validate, list, init, serve, mock, record, etc.
│   ├── client/             # Vue.js 3.5 API Client Manager (SPA for hitspec serve)
│   │   └── src/            # TypeScript + Vue components, stores, API layer
│   └── docs/               # Mintlify documentation site
├── packages/
│   ├── core/
│   │   ├── parser/         # .http file parser (lexer, AST, parser)
│   │   ├── runner/         # Test execution engine
│   │   ├── env/            # Variable/environment resolution
│   │   └── config/         # Configuration loading (hitspec.yaml)
│   ├── serve/              # HTTP server + REST API for hitspec serve
│   ├── http/               # HTTP client and request/response types
│   ├── assertions/         # Assertion evaluation (24 operators)
│   ├── capture/            # Response value capturing
│   ├── output/             # Output formatters (console, JSON, JUnit, TAP, HTML)
│   ├── stress/             # Stress testing engine
│   ├── mock/               # Mock server from .http files
│   ├── proxy/              # Recording proxy
│   ├── import/             # Importers (curl, Insomnia, OpenAPI)
│   ├── export/             # Exporters (curl)
│   ├── builtin/            # Built-in functions ($uuid, $timestamp, etc.)
│   ├── snapshot/           # Snapshot testing
│   ├── sse/                # Server-Sent Events support
│   └── contract/           # Contract testing
├── examples/               # Example .http files
└── vscode-hitspec/         # VSCode extension (syntax highlighting)
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
| Add serve API endpoint | `packages/serve/handler_*.go`, `packages/serve/routes.go`, `packages/serve/types.go` |
| Add frontend component | `apps/client/src/components/` |
| Add frontend store | `apps/client/src/stores/` |
| Add Pinia API endpoint | `apps/client/src/api/endpoints/` |
| Update docs | `apps/docs/` (.mdx files, docs.json) |
| Add stress feature | `packages/stress/stress.go`, `packages/stress/metrics.go` |
| Add mock feature | `packages/mock/server.go`, `packages/mock/router.go` |

## Monorepo Structure

| Directory | Purpose | Language | Build Tool |
|-----------|---------|----------|------------|
| `apps/cli` | CLI binary | Go | `go build` |
| `apps/client` | Web API Client Manager | Vue 3 + TypeScript | Vite + Bun |
| `apps/docs` | Documentation site | MDX | Mintlify |
| `packages/serve` | HTTP API server (embeds client SPA) | Go | `go build` |
| `packages/core` | Parser + runner | Go | - |
| `packages/*` | Feature packages | Go | - |

## `hitspec serve` Architecture

```
hitspec serve [dir]
     │
     ▼
Go HTTP Server (net/http.ServeMux, port 4000)
     │
     ├── /api/v1/*        REST API (24 endpoints, JSON)
     ├── /api/v1/ws       WebSocket (real-time events)
     └── /*               Embedded Vue SPA (//go:embed)
```

### REST API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/v1/workspace | Workspace info and file stats |
| GET | /api/v1/files | List .http/.hitspec files |
| GET | /api/v1/files/{path...} | Parse file as structured JSON |
| POST | /api/v1/execute | Execute single request |
| POST | /api/v1/run | Run all requests in a file |
| GET | /api/v1/environments | List environments |
| GET | /api/v1/environments/{name} | Get environment variables |
| PUT | /api/v1/environments/{name} | Update environment |
| GET | /api/v1/config | Get hitspec.yaml config |
| PUT | /api/v1/config | Update config |
| GET | /api/v1/history | Execution history |
| POST | /api/v1/stress/start | Start stress test |
| POST | /api/v1/stress/stop | Stop stress test |
| GET | /api/v1/stress/status | Stress test status/metrics |
| POST | /api/v1/mock/start | Start mock server |
| POST | /api/v1/mock/stop | Stop mock server |
| GET | /api/v1/mock/routes | List mock routes |
| POST | /api/v1/import/curl | Import from curl |
| POST | /api/v1/import/insomnia | Import from Insomnia |
| POST | /api/v1/import/openapi | Import from OpenAPI |
| POST | /api/v1/export/curl | Export as curl |
| GET | /api/v1/system/info | Version and build info |
| GET | /api/v1/ws | WebSocket connection |

### WebSocket Events

| Type | Direction | Description |
|------|-----------|-------------|
| `file:changed` | server→client | File modified on disk |
| `file:created` | server→client | New file detected |
| `file:deleted` | server→client | File removed |
| `exec:started` | server→client | Request execution began |
| `exec:completed` | server→client | Request execution finished |
| `stress:metrics` | server→client | Stress test progress (every 500ms) |
| `mock:request` | server→client | Mock server received request |
| `ping` | client→server | Heartbeat |

### Frontend Stack (apps/client)

| Layer | Technology |
|-------|-----------|
| Framework | Vue 3.5 + TypeScript |
| State | Pinia 3 (5 stores: collection, request, environment, history, settings) |
| Routing | Vue Router 4 (6 routes) |
| Styling | TailwindCSS v4 + Nord color palette |
| Code Editor | CodeMirror 6 with Nord theme |
| Charts | ECharts 5 (stress dashboard) |
| Icons | Lucide Vue Next |
| Tests | Vitest 3 + Vue Test Utils |

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

## Assertion Operators (24)

`==`, `!=`, `>`, `>=`, `<`, `<=`, `contains`, `!contains`, `startsWith`, `endsWith`, `matches`, `exists`, `!exists`, `length`, `includes`, `!includes`, `in`, `!in`, `type`, `schema`, `each`, `snapshot`

## Running Tests

```bash
# Go backend tests
go test ./...                        # All tests
go test ./packages/serve/...         # Serve package only (14 tests)

# Frontend tests
cd apps/client && bun run test       # Vitest (28 tests)
cd apps/client && bun run type-check # TypeScript check

# Build
task build                           # CLI binary only
task build:full                      # CLI + embedded SPA

# Development
task serve:dev                       # Go API server (dev mode, no SPA)
task client:dev                      # Vite dev server on :5173
task docs:dev                        # Mintlify docs locally

# Run examples
task dev                             # Run with petstore example
go run ./apps/cli run examples/      # Run example files
```

## Coding Conventions

- Standard Go formatting (gofmt)
- Error handling: return errors, don't panic
- Functional options pattern for config: `type Option func(*Config)`
- Package-level doc.go files for documentation
- Test files: `*_test.go` in same package
- Frontend: Vue 3 Composition API + `<script setup>`
- Frontend stores: Pinia setup stores (not options)
- Frontend styling: TailwindCSS utility classes with Nord theme tokens

## Common Patterns

### Adding a New Serve API Endpoint

1. Add DTO types to `packages/serve/types.go`
2. Create handler in `packages/serve/handler_*.go`
3. Register route in `packages/serve/routes.go`
4. Add TypeScript types to `apps/client/src/types/api.ts`
5. Add API endpoint to `apps/client/src/api/endpoints/`
6. Wire into Pinia store or component

### Adding a New Assertion Operator

1. Add constant to `packages/core/parser/ast.go`:
   ```go
   OpNewOperator AssertionOperator = "newoperator"
   ```

2. Add parsing in `packages/core/parser/parser.go` parseAssertion()

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

## Build Pipeline

```
Development:
  Terminal 1: task serve:dev          # Go API on :4000 (no SPA)
  Terminal 2: task client:dev         # Vite on :5173, proxies /api → :4000

Production:
  task build:full                     # Builds client → embeds in Go binary
  ./bin/hitspec serve                 # Single binary serves everything on :4000
```

## Documentation

- [Mintlify Docs](apps/docs/) - Full documentation site
- [CLI Reference](docs/cli.md) - All commands and flags
- [LLM Reference](llms.txt) - Complete syntax reference for AI assistants
