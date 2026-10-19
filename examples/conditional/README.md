# Conditional Execution Examples

Demonstrates `@if` and `@unless` directives for controlling which requests execute.

## Features Covered

- `@if {{variable}}` -- Run request only when variable is truthy
- `@unless {{variable}}` -- Skip request when variable is truthy
- Combining conditions with `@depends` for conditional chains
- Environment-based conditional execution

## Running

The file carries `>>>mock` blocks, so it runs offline against the built-in mock
server (set `@baseUrl = https://jsonplaceholder.typicode.com` to use the real
API instead):

```bash
hitspec mock examples/conditional/conditional.http &    # serves http://localhost:3000

# Run with default variables (runAuthTests=true, skipSlowTests=false)
hitspec run examples/conditional/conditional.http

# Run only conditional tests
hitspec run examples/conditional/conditional.http --tags conditional

kill %1
```

Edit the `@runAuthTests`, `@skipSlowTests` and `@debugMode` variables at the
top of the file to see requests being skipped.

## How Conditions Work

| Directive | Variable Value | Request Runs? |
|-----------|---------------|---------------|
| `@if {{var}}` | `true`, non-empty string | Yes |
| `@if {{var}}` | `false`, empty, undefined | No |
| `@unless {{var}}` | `true`, non-empty string | No |
| `@unless {{var}}` | `false`, empty, undefined | Yes |

## Key Concepts

- Use `@if` to gate tests behind feature flags or environment variables
- Use `@unless` to skip tests in certain environments (e.g., skip destructive tests in prod)
- Combine with `@depends` to build conditional request chains
- Undefined variables evaluate as falsy, so `@if {{debugMode}}` skips when debugMode is not set
