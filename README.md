# hitspec

Plain text API tests. No magic.

hitspec is a file-based HTTP API testing tool that emphasizes simplicity and readability.
Write your API tests in plain text files that look like actual HTTP requests.

## Installation

```bash
# From source (requires Go 1.22+)
go install github.com/abdul-hamid-achik/hitspec/apps/cli@latest

# Or build locally
task build
```

## Quick Start

Create a test file `api.http`:

```http
@baseUrl = https://jsonplaceholder.typicode.com

### Get all posts
# @name getPosts

GET {{baseUrl}}/posts

>>>
expect status 200
expect body length 100
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
```

Run it:

```bash
hitspec run api.http
```

## Commands

```bash
hitspec run <file|dir>      # Run tests
hitspec validate <file|dir> # Validate syntax
hitspec list <file|dir>     # List all tests
hitspec init                # Initialize new project
hitspec version             # Show version
```

## Features

- Plain text test files (`.http` or `.hitspec`)
- Variable interpolation with `{{variable}}`
- Request chaining with captures
- Powerful assertions (status, headers, body, JSONPath)
- Multiple environments
- Built-in functions (uuid, timestamp, random, etc.)

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
