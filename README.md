# hitspec

Plain text API tests. No magic.

hitspec is a file-based HTTP API testing tool that emphasizes simplicity and readability.
Write your API tests in plain text files that look like actual HTTP requests.

## Why hitspec?

| Tool | hitspec Advantage |
|------|-------------------|
| **Postman** | Plain text files, version-controllable, no GUI needed |
| **REST Client (VSCode)** | Adds assertions, captures, dependencies, stress testing |
| **Hurl** | Built-in stress testing, DB assertions, mock server, import/export |
| **newman** | Simpler file format, no collection export needed |
| **curl** | Full test framework on top of HTTP requests |
| **k6** | Simpler syntax, no JavaScript required for basic tests |

## Installation

### Homebrew (macOS/Linux)

```bash
brew install abdul-hamid-achik/tap/hitspec
```

### Go Install

```bash
go install github.com/abdul-hamid-achik/hitspec/apps/cli@latest
```

### Download Binary

Download from [GitHub Releases](https://github.com/abdul-hamid-achik/hitspec/releases).

## Quick Start

Create a test file `api.http`:

```http
@baseUrl = https://jsonplaceholder.typicode.com

### Get all posts
# @name getPosts

GET {{baseUrl}}/posts

>>>
expect status 200
expect body type array
expect body[0].id exists
<<<

### Create a post
# @name createPost

POST {{baseUrl}}/posts
Content-Type: application/json

{
  "title": "Hello hitspec",
  "body": "Testing made simple",
  "userId": 1
}

>>>
expect status 201
expect body.id exists
<<<

>>>capture
postId from body.id
<<<

### Get the created post
# @name getCreatedPost
# @depends createPost

GET {{baseUrl}}/posts/{{createPost.postId}}

>>>
expect status 200
expect body.title == "Hello hitspec"
<<<
```

Run it:

```bash
hitspec run api.http
```

## Documentation

| Document | Description |
|----------|-------------|
| [Full Documentation](https://hitspec.dev) | Mintlify-hosted docs |
| [Examples](examples/) | Example test files |

## Features

- **Plain text test files** - `.http` format, readable and version-controllable
- **Web API Client Manager** - Browser-based UI via `hitspec serve` (like Postman, but file-backed)
- **26 assertion operators** - `==`, `!=`, `>`, `<`, `contains`, `matches`, `exists`, `length`, `type`, `schema`, `snapshot`, and more
- **22 metadata directives** - `@name`, `@tags`, `@depends`, `@timeout`, `@retry`, `@auth`, `@waitFor`, and more
- **17 built-in functions** - `$uuid()`, `$timestamp()`, `$random()`, `$base64()`, `$sha256()`, `$env()`, and more
- **8 authentication types** - Bearer, Basic, API Key, Digest, AWS Signature v4, OAuth2
- **Built-in stress testing** - Load test with `--stress` flag, real-time metrics dashboard
- **Mock server** - Generate mock APIs from `.http` files
- **Variable captures** - Chain requests by capturing response values
- **Request dependencies** - Control execution order with `@depends`
- **Multiple environments** - Dev, staging, prod configurations
- **Parallel execution** - Run independent tests concurrently
- **Multiple output formats** - Console, JSON, JUnit, TAP, HTML
- **Watch mode** - Re-run on file changes
- **Snapshot testing** - Capture and compare response bodies against baselines
- **Database assertions** - Verify database state after HTTP requests
- **Shell commands** - Run scripts before/after requests
- **API coverage reporting** - Measure test coverage against OpenAPI specs
- **curl/Insomnia/OpenAPI import** - Convert existing tests
- **Export to curl** - Export hitspec tests as executable curl commands
- **SSE support** - Test Server-Sent Events endpoints
- **Contract testing** - Verify API contracts against providers

## Editor Support

### VSCode

Install the official hitspec VSCode extension for full syntax support:

**From VSIX (manual install):**
1. Build the extension: `cd apps/vscode && npm run package`
2. In VSCode, open Command Palette (`Cmd+Shift+P` / `Ctrl+Shift+P`)
3. Run "Extensions: Install from VSIX..."
4. Select the generated `.vsix` file

**Features:**
- Full syntax highlighting for `.http` and `.hitspec` files
- Code snippets for requests, assertions, captures
- Support for hitspec-specific syntax: `>>>`, `<<<`, `[[[`, `]]]`
- Variable interpolation highlighting (`{{variable}}`)
- Built-in function highlighting (`$uuid()`, `$timestamp()`, etc.)

Alternatively, the [REST Client extension](https://marketplace.visualstudio.com/items?itemName=humao.rest-client) provides basic HTTP syntax highlighting.

### Shell Completion

Generate shell completion scripts for bash, zsh, fish, or PowerShell:

```bash
# Bash
hitspec completion bash > /etc/bash_completion.d/hitspec

# Zsh
hitspec completion zsh > "${fpath[1]}/_hitspec"

# Fish
hitspec completion fish > ~/.config/fish/completions/hitspec.fish
```

## Quick Reference

### Assertion Operators

```http
>>>
expect status == 200              # Equals
expect status != 404              # Not equals
expect body.count > 0             # Greater than
expect duration < 1000            # Less than
expect body contains "success"    # Contains substring
expect body.id matches /^\d+$/    # Regex match
expect body.error !exists         # Does not exist
expect body.items length 10       # Array length equals
expect body.items length > 0      # Array length greater than
expect body.tags includes "api"   # Array contains
expect status in [200, 201]       # Value in array
expect body.data type object      # Type check
expect body schema ./schema.json  # JSON Schema validation
expect body snapshot "response"   # Snapshot comparison
<<<
```

### Built-in Functions

```http
{{$uuid()}}              # Generate UUID v4
{{$timestamp()}}         # Unix timestamp
{{$now()}}               # RFC3339 datetime
{{$date(2006-01-02)}}    # Custom date format
{{$random(1, 100)}}      # Random integer
{{$randomEmail()}}       # Random email
{{$base64(value)}}       # Base64 encode
{{$sha256(value)}}       # SHA256 hash
{{$env(API_TOKEN)}}      # Environment variable
```

### Metadata Directives

```http
# @name myRequest           # Request identifier
# @tags smoke, auth         # Tags for filtering
# @depends login            # Run after login request
# @timeout 5000             # Timeout in ms
# @retry 3                  # Retry on failure
# @auth bearer {{token}}    # Authentication
```

### Captures

```http
>>>capture
token from body.access_token
userId from body.user.id
<<<

# Use in subsequent requests:
GET {{baseUrl}}/users/{{login.userId}}
Authorization: Bearer {{login.token}}
```

## API Client Manager (`hitspec serve`)

Launch a browser-based API Client Manager for visual test editing, execution, and debugging:

```bash
hitspec serve                         # Open UI at http://localhost:4000
hitspec serve ./tests/                # Serve specific directory
hitspec serve --port 8080             # Custom port
hitspec serve --api-only              # REST API only, no UI
hitspec serve --read-only --cors      # Safe mode for sharing
```

**Features:**
- Three-panel workspace: file tree, request editor, response viewer
- **Source editor** with CodeMirror syntax highlighting and `Cmd+S` save to disk
- **Stress test results panel** with latency percentiles, per-request breakdown, and threshold pass/fail
- **Stress profile CRUD** -- create, edit, and delete profiles from the UI (persisted to `hitspec.yaml`)
- **Settings persistence** -- config changes save to `hitspec.yaml` on disk
- Real-time file watching with WebSocket updates (self-write suppression)
- Stress testing dashboard with live metrics
- Mock server management
- Import from curl/Insomnia/OpenAPI
- Environment switcher
- Persistent execution history with run comparison

**Serve Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--port, -p` | 4000 | Server port |
| `--host` | localhost | Bind address |
| `--open` | true | Auto-open browser |
| `--watch, -w` | true | Watch for file changes |
| `--cors` | false | Enable CORS headers |
| `--api-only` | false | REST API only, no SPA |
| `--read-only` | false | Disallow file mutations |
| `--env, -e` | dev | Default environment |
| `--allow-shell` | false | Allow shell command execution |
| `--allow-db` | false | Allow database assertions |

**Development:**

```bash
# Two-terminal dev workflow
task serve:dev                        # Go API server on :4000
task client:dev                       # Vite dev server on :5173

# Production build (single binary with embedded UI)
task build:full
```

## CLI Usage

```bash
hitspec run tests/                    # Run all tests in directory
hitspec run tests/ --env prod         # Use production environment
hitspec run tests/ --tags smoke       # Run only smoke tests
hitspec run tests/ --parallel         # Run in parallel
hitspec run tests/ --watch            # Watch mode
hitspec run tests/ -o json            # JSON output
hitspec run tests/ --update-snapshots # Update snapshot files
hitspec validate tests/               # Validate syntax
hitspec list tests/                   # List all requests
hitspec import curl "curl ..."        # Import from curl
hitspec import insomnia export.json   # Import from Insomnia
hitspec import openapi spec.yaml     # Import from OpenAPI spec
hitspec export curl tests/api.http   # Export as curl commands
hitspec diff baseline.json current.json  # Compare test results
hitspec diff baseline.json current.json --threshold 10%
hitspec serve                         # Launch API Client Manager
hitspec serve ./tests/ --port 8080    # Serve specific directory
```

## Examples

See the [examples](examples/) directory:

- [Basic CRUD](examples/basic/crud.http) - GET, POST, PUT, DELETE operations
- [Petstore API](examples/petstore/petstore.http) - Real-world API example with dependencies
- [Auth Flow](examples/auth-flow/auth.http) - Bearer, Basic, API Key, Digest authentication
- [GraphQL](examples/graphql/graphql.http) - GraphQL queries with variables and assertions
- [Stress Testing](examples/stress-test/stress.http) - Load testing with weights, setup, teardown
- [Database](examples/database/database.http) - DB assertions with `>>>db` blocks
- [Contract Testing](examples/contract/contract.http) - Schema/type validation against API contracts
- [Conditional](examples/conditional/conditional.http) - `@if`/`@unless` conditional execution
- [Retry](examples/retry/retry.http) - `@retry`, `@retryOn`, `@retryDelay` for flaky endpoints

---

## Complete Syntax Reference

### Variables

```http
@baseUrl = https://api.example.com
@token = your-api-token
@userId = 123
```

Use `{{variableName}}` syntax:

```http
GET {{baseUrl}}/users/{{userId}}
Authorization: Bearer {{token}}
```

### All Built-in Functions

| Function | Description | Example |
|----------|-------------|---------|
| `$uuid()` | Generate UUID v4 | `{{$uuid()}}` → `550e8400-e29b-41d4-a716-446655440000` |
| `$timestamp()` | Unix timestamp (seconds) | `{{$timestamp()}}` → `1705612800` |
| `$timestampMs()` | Unix timestamp (milliseconds) | `{{$timestampMs()}}` → `1705612800000` |
| `$now()` | Current datetime (RFC3339) | `{{$now()}}` → `2024-01-18T12:00:00Z` |
| `$date(format)` | Custom date format (default: YYYY-MM-DD) | `{{$date(2006-01-02)}}` → `2024-01-18` |
| `$random(min, max)` | Random integer | `{{$random(1, 100)}}` → `42` |
| `$randomString(len)` | Random alphanumeric | `{{$randomString(8)}}` → `aB3kL9mN` |
| `$randomEmail()` | Random email | `{{$randomEmail()}}` → `user_abc123@example.com` |
| `$randomAlphanumeric(len)` | Random alphanumeric | `{{$randomAlphanumeric(10)}}` → `K8mNp2qRsT` |
| `$base64(value)` | Base64 encode | `{{$base64(hello)}}` → `aGVsbG8=` |
| `$base64Decode(value)` | Base64 decode | `{{$base64Decode(aGVsbG8=)}}` → `hello` |
| `$md5(value)` | MD5 hash | `{{$md5(hello)}}` → `5d41402abc4b2a76...` |
| `$sha256(value)` | SHA256 hash | `{{$sha256(hello)}}` → `2cf24dba5fb0a30e...` |
| `$urlEncode(value)` | URL encode | `{{$urlEncode(hello world)}}` → `hello%20world` |
| `$urlDecode(value)` | URL decode | `{{$urlDecode(hello%20world)}}` → `hello world` |
| `$json(value)` | JSON passthrough | `{{$json({"key": "value"})}}` |
| `$env(name, default)` | System environment variable | `{{$env(API_TOKEN)}}` |

### Query Parameters

Inline in URL:
```http
GET {{baseUrl}}/search?query=test&limit=10
```

Explicit syntax:
```http
GET {{baseUrl}}/search
? query = test
? limit = 10
```

### Request Bodies

**JSON Body:**
```http
POST {{baseUrl}}/users
Content-Type: application/json

{
  "name": "John Doe",
  "email": "john@example.com"
}
```

**Form URL-Encoded:**
```http
POST {{baseUrl}}/login
Content-Type: application/x-www-form-urlencoded

& username = john
& password = secret123
```

**Multipart Form Data:**
```http
POST {{baseUrl}}/upload

>>>multipart
field name = John Doe
file @./photo.jpg
<<<
```

**GraphQL:**
```http
POST {{baseUrl}}/graphql
Content-Type: application/json

>>>graphql
query GetUser($id: ID!) {
  user(id: $id) {
    id
    name
  }
}

>>>variables
{
  "id": "123"
}
<<<
```

### All Assertion Operators

#### Equality & Comparison
| Operator | Syntax | Description |
|----------|--------|-------------|
| `==` | `expect status == 200` | Equals |
| `!=` | `expect body.error != null` | Not equals |
| `>` | `expect body.count > 0` | Greater than |
| `>=` | `expect body.count >= 1` | Greater than or equal |
| `<` | `expect duration < 1000` | Less than |
| `<=` | `expect duration <= 500` | Less than or equal |

#### String Operators
| Operator | Syntax | Description |
|----------|--------|-------------|
| `contains` | `expect body contains "success"` | Contains substring |
| `!contains` | `expect body !contains "error"` | Does not contain |
| `startsWith` | `expect body.url startsWith "https"` | Starts with prefix |
| `endsWith` | `expect body.email endsWith ".com"` | Ends with suffix |
| `matches` | `expect body.id matches /^\d+$/` | Matches regex |

#### Existence & Type
| Operator | Syntax | Description |
|----------|--------|-------------|
| `exists` | `expect body.id exists` | Value is not null |
| `!exists` | `expect body.error !exists` | Value is null |
| `type` | `expect body.items type array` | Check value type (`null`, `boolean`, `number`, `string`, `array`, `object`) |

#### Length & Arrays
| Operator | Syntax | Description |
|----------|--------|-------------|
| `length` | `expect body.items length 10` | Array/string length equals |
| `length >` | `expect body.items length > 0` | Array/string length greater than |
| `length >=` | `expect body.items length >= 1` | Array/string length greater than or equal |
| `length <` | `expect body.items length < 100` | Array/string length less than |
| `length <=` | `expect body.items length <= 50` | Array/string length less than or equal |
| `includes` | `expect body.tags includes "admin"` | Array contains value |
| `!includes` | `expect body.tags !includes "test"` | Array does not contain |
| `in` | `expect status in [200, 201, 204]` | Value is in array |
| `!in` | `expect status !in [400, 404, 500]` | Value is not in array |
| `each` | `expect body.items each type object` | Apply assertion to each element |

#### Schema Validation
| Operator | Syntax | Description |
|----------|--------|-------------|
| `schema` | `expect body schema ./schema.json` | Validate against JSON Schema |

#### Snapshot Testing
| Operator | Syntax | Description |
|----------|--------|-------------|
| `snapshot` | `expect body snapshot "responseName"` | Compare against saved snapshot |

### Assertion Subjects

| Subject | Description | Example |
|---------|-------------|---------|
| `status` | HTTP status code | `expect status 200` |
| `duration` | Response time (ms) | `expect duration < 1000` |
| `header <name>` | Response header | `expect header Content-Type contains json` |
| `body` | Full response body | `expect body contains "success"` |
| `body.<path>` | JSON path | `expect body.user.name == "John"` |
| `body[n]` | Array index | `expect body[0].id exists` |

### All Metadata Directives

| Directive | Description | Example |
|-----------|-------------|---------|
| `@name` | Request identifier | `# @name createUser` |
| `@description` | Human-readable description | `# @description Creates a user` |
| `@tags` | Tags for filtering | `# @tags smoke, auth` |
| `@skip` | Skip request | `# @skip Temporarily disabled` |
| `@only` | Run only this request | `# @only` |
| `@timeout` | Timeout in ms | `# @timeout 5000` |
| `@retry` | Retry attempts | `# @retry 3` |
| `@retryDelay` | Delay between retries (ms) | `# @retryDelay 1000` |
| `@retryOn` | Status codes that trigger retry | `# @retryOn 500, 502, 503` |
| `@depends` | Dependencies | `# @depends login, setupData` |
| `@if` | Conditional execution | `# @if {{runTests}}` |
| `@unless` | Conditional skip | `# @unless {{skipAuth}}` |
| `@auth` | Authentication method | `# @auth bearer {{token}}` |
| `@before` | Run script before request | `# @before ./setup.sh` |
| `@after` | Run script after request | `# @after ./cleanup.sh` |
| `@db` | Database connection string | `# @db sqlite://./test.db` |
| `@waitFor` | Poll URL until ready | `# @waitFor {{baseUrl}}/health 200 30000 1000` |
| `@stress.weight` | Relative weight for stress testing | `# @stress.weight 3` |
| `@stress.think` | Think time in ms after request | `# @stress.think 500` |
| `@stress.skip` | Exclude from stress tests | `# @stress.skip` |
| `@stress.setup` | Run once before stress test starts | `# @stress.setup` |
| `@stress.teardown` | Run once after stress test ends | `# @stress.teardown` |

### Authentication Methods

```http
# Bearer Token
# @auth bearer {{token}}

# Basic Auth
# @auth basic {{username}}, {{password}}

# API Key (Header)
# @auth apiKey X-API-Key, {{apiKey}}

# API Key (Query String)
# @auth apiKeyQuery api_key, {{apiKey}}

# Digest Auth
# @auth digest {{username}}, {{password}}

# AWS Signature v4
# @auth aws {{accessKey}}, {{secretKey}}, {{region}}, {{service}}

# OAuth2 Client Credentials
# @auth oauth2 client_credentials {{tokenUrl}}, {{clientId}}, {{clientSecret}}, scope1,scope2

# OAuth2 Password Grant
# @auth oauth2 password {{tokenUrl}}, {{clientId}}, {{clientSecret}}, {{username}}, {{password}}, scope1,scope2
```

### Captures

Capture values from responses for use in subsequent requests:

```http
### Login
# @name login
POST {{baseUrl}}/auth/login
Content-Type: application/json

{"username": "john", "password": "secret"}

>>>capture
token from body.access_token
userId from body.user.id
<<<

### Get Profile
# @depends login
GET {{baseUrl}}/users/{{login.userId}}
Authorization: Bearer {{login.token}}

>>>
expect status 200
expect body.id == "{{login.userId}}"
<<<
```

**Note:** Captured variables can be used in expect clauses with `{{requestName.captureName}}` syntax.

**Capture Sources:**
| Source | Syntax | Description |
|--------|--------|-------------|
| Body JSON path | `token from body.access_token` | Capture from response body |
| Header | `contentType from header Content-Type` | Capture from response header |
| Status | `code from status` | Capture status code |
| Duration | `time from duration` | Capture response time (ms) |

## CI/CD Integration

### GitHub Actions

Use the official hitspec action to run API tests in your CI pipeline:

```yaml
- uses: abdul-hamid-achik/hitspec@v1
  with:
    files: tests/
    output: junit
    output-file: test-results.xml

- uses: actions/upload-artifact@v4
  with:
    name: test-results
    path: test-results.xml
```

#### Action Inputs

| Input | Description | Default |
|-------|-------------|---------|
| `files` | Files/directories to test | (required) |
| `env` | Environment name | `dev` |
| `env-file` | Path to .env file | - |
| `output` | Output format (console, json, junit, tap, html) | `junit` |
| `output-file` | Write output to file | - |
| `tags` | Filter by tags | - |
| `parallel` | Run in parallel | `false` |
| `bail` | Stop on first failure | `false` |
| `stress` | Enable stress testing | `false` |
| `duration` | Stress test duration | `30s` |
| `rate` | Requests per second | `10` |
| `threshold` | Pass/fail thresholds | - |
| `version` | hitspec version | `latest` |

#### Stress Testing in CI

```yaml
- uses: abdul-hamid-achik/hitspec@v1
  with:
    files: api.http
    stress: true
    duration: 2m
    rate: 100
    threshold: 'p95<200ms,errors<0.5%'
```

#### With Environment Variables

```yaml
- name: Create .env file
  run: |
    echo "API_TOKEN=${{ secrets.API_TOKEN }}" >> .env.ci

- uses: abdul-hamid-achik/hitspec@v1
  with:
    files: tests/
    env: staging
    env-file: .env.ci
```

#### Using hitspec Environment Variables

All major CLI flags can be set via environment variables with the `HITSPEC_` prefix:

| Environment Variable | CLI Flag | Description |
|---------------------|----------|-------------|
| `HITSPEC_ENV` | `--env` | Environment to use |
| `HITSPEC_ENV_FILE` | `--env-file` | Path to .env file |
| `HITSPEC_CONFIG` | `--config` | Path to config file |
| `HITSPEC_TIMEOUT` | `--timeout` | Request timeout |
| `HITSPEC_TAGS` | `--tags` | Filter by tags |
| `HITSPEC_OUTPUT` | `--output` | Output format |
| `HITSPEC_OUTPUT_FILE` | `--output-file` | Output file path |
| `HITSPEC_BAIL` | `--bail` | Stop on first failure |
| `HITSPEC_PARALLEL` | `--parallel` | Run in parallel |
| `HITSPEC_CONCURRENCY` | `--concurrency` | Concurrent requests |
| `HITSPEC_QUIET` | `--quiet` | Suppress output |
| `HITSPEC_NO_COLOR` | `--no-color` | Disable colors |
| `HITSPEC_PROXY` | `--proxy` | Proxy URL |
| `HITSPEC_INSECURE` | `--insecure` | Skip SSL verification |

Example:
```bash
export HITSPEC_ENV=staging
export HITSPEC_TIMEOUT=60s
export HITSPEC_BAIL=true
hitspec run tests/
```

See [.github/workflows/example-hitspec.yml](.github/workflows/example-hitspec.yml) for more examples.

## Project Structure

```
hitspec/
├── apps/
│   ├── cli/              # CLI binary (Go + Cobra)
│   ├── client/           # Web API Client Manager (Vue 3 + TypeScript)
│   ├── docs/             # Mintlify documentation site
│   ├── vscode/           # VSCode extension
│   └── nvim/             # Neovim plugin
├── packages/
│   ├── core/             # Parser + runner
│   ├── serve/            # HTTP API server (embeds client SPA)
│   ├── stress/           # Stress testing engine
│   ├── mock/             # Mock server
│   ├── http/             # HTTP client
│   ├── assertions/       # Assertion evaluation
│   ├── output/           # Output formatters
│   ├── import/           # Importers (curl, Insomnia, OpenAPI)
│   └── ...               # Other feature packages
├── examples/             # Example .http files
```

## Development

```bash
# Install dependencies
task deps                             # Go dependencies
task client:install                   # Frontend dependencies (bun)

# Run tests
task test                             # Go tests
task client:test                      # Frontend tests (vitest)

# Build
task build                            # CLI binary only
task build:full                       # CLI + embedded web UI

# Development servers
task serve:dev                        # Go API server on :4000
task client:dev                       # Vite dev server on :5173
task docs:dev                         # Mintlify docs locally

# Run with example
task dev
```

## License

MIT
