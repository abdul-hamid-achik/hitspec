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

```bash
hitspec run examples/auth-flow/auth.http

# Run only bearer tests
hitspec run examples/auth-flow/auth.http --tags bearer

# Run the full auth flow chain
hitspec run examples/auth-flow/auth.http --tags flow
```

## Key Concepts

- Use `# @auth <type> <params>` to set authentication per-request
- Combine `>>>capture` with `# @depends` to chain login flows
- API keys can go in headers or query strings depending on the provider
