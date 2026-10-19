# Authentication Flow Examples

Demonstrates the various authentication methods supported by hitspec.

## Auth Types Covered

- **Bearer Token** -- `@auth bearer <token>`
- **Basic Auth** -- `@auth basic <user>, <password>`
- **API Key (Header)** -- `@auth apiKey <header>, <value>`
- **API Key (Query)** -- `@auth apiKeyQuery <param>, <value>`
- **Digest Auth** -- `@auth digest <user>, <password>`
- **Chained auth flow** -- Login, capture token, use in next request

## Running

The file is self-contained: `>>>mock` blocks let `hitspec mock` serve canned
httpbin-style responses, so it runs offline. The mock does **not** verify the
credentials it receives, so offline runs prove the request/assertion plumbing,
not the auth schemes. Set `@baseUrl = https://httpbin.org` to test the real
thing.

```bash
hitspec mock examples/auth-flow/auth.http &     # serves http://localhost:3000
hitspec run examples/auth-flow/auth.http

# Run only bearer tests
hitspec run examples/auth-flow/auth.http --tags bearer

# Run the full auth flow chain
hitspec run examples/auth-flow/auth.http --tags flow

kill %1
```

## Key Concepts

- Use `# @auth <type> <params>` to set authentication per-request
- Combine `>>>capture` with `# @depends` to chain login flows
- API keys can go in headers or query strings depending on the provider
