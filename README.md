# hitspec

Plain text API tests. No magic.

hitspec is a file-based HTTP API testing tool that emphasizes simplicity and readability.
Write your API tests in plain text files that look like actual HTTP requests.

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
| [Syntax Reference](docs/README.md) | Complete .http file syntax, all operators, directives, functions |
| [CLI Reference](docs/cli.md) | All commands and flags |
| [Environment Configuration](docs/environments.md) | Setting up environments |
| [Examples](examples/) | Example test files |

## Features

- **Plain text test files** - `.http` format, readable and version-controllable
- **19 assertion operators** - `==`, `!=`, `>`, `<`, `contains`, `matches`, `exists`, `length`, `type`, `schema`, and more
- **16 metadata directives** - `@name`, `@tags`, `@depends`, `@timeout`, `@retry`, `@auth`, and more
- **17 built-in functions** - `$uuid()`, `$timestamp()`, `$random()`, `$base64()`, `$sha256()`, and more
- **6 authentication types** - Bearer, Basic, API Key, Digest, AWS Signature v4
- **Variable captures** - Chain requests by capturing response values
- **Request dependencies** - Control execution order with `@depends`
- **Multiple environments** - Dev, staging, prod configurations
- **Parallel execution** - Run independent tests concurrently
- **Multiple output formats** - Console, JSON, JUnit, TAP
- **Watch mode** - Re-run on file changes

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
expect body.items length 10       # Array length
expect body.tags includes "api"   # Array contains
expect status in [200, 201]       # Value in array
expect body.data type object      # Type check
expect body schema ./schema.json  # JSON Schema validation
<<<
```

### Built-in Functions

```http
{{$uuid()}}              # Generate UUID v4
{{$timestamp()}}         # Unix timestamp
{{$now()}}               # ISO datetime
{{$random(1, 100)}}      # Random integer
{{$randomEmail()}}       # Random email
{{$base64(value)}}       # Base64 encode
{{$sha256(value)}}       # SHA256 hash
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

## CLI Usage

```bash
hitspec run tests/              # Run all tests in directory
hitspec run tests/ --env prod   # Use production environment
hitspec run tests/ --tags smoke # Run only smoke tests
hitspec run tests/ --parallel   # Run in parallel
hitspec run tests/ --watch      # Watch mode
hitspec run tests/ -o json      # JSON output
hitspec validate tests/         # Validate syntax
hitspec list tests/             # List all requests
```

## Examples

See the [examples](examples/) directory:

- [Basic CRUD](examples/basic/crud.http) - GET, POST, PUT, DELETE operations
- [Petstore API](examples/petstore/petstore.http) - Real-world API example with dependencies

## Development

```bash
# Install dependencies
task deps

# Run tests
task test

# Build
task build

# Run with example
task dev
```

## License

MIT
