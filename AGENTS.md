# AGENTS.md - AI Assistant Guide for hitspec

## Project Overview

hitspec is a file-based HTTP API testing tool written in Go. Users write `.http` files with requests and assertions, and hitspec executes them.

## Architecture

```
hitspec/
├── apps/cli/           # CLI application (cobra commands)
│   └── cmd/            # Command implementations (run, validate, list, init)
├── packages/
│   ├── core/
│   │   ├── parser/     # .http file parser (lexer, AST, parser)
│   │   ├── runner/     # Test execution engine
│   │   ├── env/        # Variable resolution
│   │   └── config/     # Configuration loading
│   ├── http/           # HTTP client and request/response types
│   ├── assertions/     # Assertion evaluation (19 operators)
│   ├── capture/        # Response value capturing
│   ├── output/         # Output formatters (console, JSON, JUnit, TAP)
│   └── builtin/        # Built-in functions ($uuid, $timestamp, etc.)
├── examples/           # Example .http files
└── docs/               # Documentation
```

## Key Files

| Task | Files |
|------|-------|
| Add assertion operator | `packages/core/parser/ast.go` (add to OpXxx), `packages/assertions/evaluator.go` (implement) |
| Add built-in function | `packages/builtin/functions.go` |
| Add CLI flag | `apps/cli/cmd/run.go` |
| Fix request parsing | `packages/core/parser/parser.go`, `packages/core/parser/lexer.go` |
| Fix HTTP client | `packages/http/client.go`, `packages/http/request.go` |
| Add output format | `packages/output/` |

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

## Assertion Operators

`==`, `!=`, `>`, `>=`, `<`, `<=`, `contains`, `!contains`, `startsWith`, `endsWith`, `matches`, `exists`, `!exists`, `length`, `includes`, `!includes`, `in`, `!in`, `type`, `schema`, `each`

## Running Tests

```bash
go test ./...                        # Run all unit tests
go run ./apps/cli run examples/      # Run example integration tests
task test                            # Run tests via Taskfile
task build                           # Build binary
```

## Coding Conventions

- Standard Go formatting (gofmt)
- Error handling: return errors, don't panic
- Package-level doc.go files for documentation
- Test files: `*_test.go` in same package

## Common Patterns

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

## Documentation

- [Syntax Reference](README.md#complete-syntax-reference) - Complete .http syntax
- [CLI Reference](docs/cli.md) - All commands and flags
- [Environment Config](docs/environments.md) - Environment setup
