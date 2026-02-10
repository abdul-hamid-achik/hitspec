# Retry and Resilience Examples

Demonstrates how to handle flaky or unreliable endpoints with retry logic.

## Features Covered

- `@retry <n>` -- Retry up to n times on failure
- `@retryDelay <ms>` -- Wait between retry attempts
- `@retryOn <status codes>` -- Only retry on specific HTTP status codes
- `@timeout <ms>` -- Per-request timeout
- `@waitFor <url> <status> <timeout> <interval>` -- Poll until a service is ready
- Idempotent POST retries with `$uuid()` for idempotency keys

## Running

```bash
hitspec run examples/retry/retry.http

# Run only timeout-related retry tests
hitspec run examples/retry/retry.http --tags timeout

# Run with verbose output to see retry attempts
hitspec run examples/retry/retry.http --tags retry
```

## Retry Behavior

| Directive | Effect |
|-----------|--------|
| `@retry 3` | Retry up to 3 times (4 total attempts) |
| `@retryDelay 2000` | Wait 2 seconds between retries |
| `@retryOn 500, 502, 503` | Only retry on these status codes |
| `@timeout 5000` | Fail the request if it takes longer than 5s |
| `@waitFor <url> 200 10000 1000` | Poll URL every 1s for up to 10s until 200 |

## Key Concepts

- Without `@retryOn`, any non-2xx response triggers a retry
- With `@retryOn`, only the listed status codes trigger retries
- Use `@waitFor` to block until a dependency service is healthy
- For POST retries, use an idempotency key to prevent duplicate side effects
- Combine `@timeout` with `@retry` to handle both slow and failing responses
