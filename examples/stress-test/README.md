# Stress Testing Examples

Demonstrates hitspec's built-in load testing capabilities.

## Features Covered

- `@stress.weight` -- Control how often each request runs relative to others
- `@stress.setup` -- Run once before the stress test starts
- `@stress.teardown` -- Run once after the stress test ends
- `@stress.skip` -- Exclude a request from stress testing
- Duration assertions (`expect duration < 5000`)
- Dynamic data with `$uuid()`, `$now()`, `$random()`

## Running

The file carries `>>>mock` blocks, so it runs offline against the built-in mock
server. Load-testing the in-process mock measures hitspec and your machine, not
an API; for realistic numbers set `@baseUrl` to a staging service you own.

```bash
hitspec mock examples/stress-test/stress.http &    # serves http://localhost:3000

# One functional pass (no load)
hitspec run examples/stress-test/stress.http

# Basic stress test (30 seconds, 10 req/s)
hitspec run examples/stress-test/stress.http --stress

# Custom duration and rate
hitspec run examples/stress-test/stress.http --stress --duration 2m --rate 50

# With pass/fail thresholds
hitspec run examples/stress-test/stress.http --stress --duration 1m --rate 100 \
  --threshold 'p95<2000ms,errors<1%'

# JSON output for CI integration
hitspec run examples/stress-test/stress.http --stress --duration 30s -o json

kill %1
```

## Weight Distribution

With the weights in this example:
- `healthCheck` (weight 5) -- ~50% of requests
- `getWithParams` (weight 3) -- ~30% of requests
- `createResource` (weight 1) -- ~10% of requests
- `delayedEndpoint` (weight 1) -- ~10% of requests
- `setupData` -- runs once before
- `teardown` -- runs once after
- `debugEndpoint` -- skipped entirely
