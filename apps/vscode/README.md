# hitspec VSCode Extension

Syntax highlighting and snippets for hitspec HTTP API testing files.

## Features

- **Syntax Highlighting** for `.http` and `.hitspec` files
- **Code Snippets** for common patterns
- **Language Configuration** for comments, brackets, and folding
- Support for all HTTP methods including **WebSocket** (`WS`)
- Typed block highlighting: `>>>capture`, `>>>graphql`, `>>>db`, `>>>shell`, `>>>multipart`, `>>>variables`
- Comment support for both `#` and `//` styles

## Supported File Extensions

- `.http`
- `.hitspec`

## Syntax Highlighting

The extension provides highlighting for:

- HTTP methods (GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS, WS, etc.)
- Request separators (`###`)
- Annotations (`@name`, `@tags`, `@auth`, `@if`, `@unless`, `@retryOn`, `@description`, `@import`, etc.)
- Headers
- Assertion blocks (`>>>` ... `<<<`)
- Typed blocks (`>>>capture`, `>>>graphql`, `>>>db`, `>>>shell`, `>>>multipart`)
- Variable interpolation (`{{variable}}`)
- Built-in functions (`$uuid()`, `$timestamp()`, etc.)
- Comments (`#` and `//`)

## Snippets

### Request Snippets

| Prefix | Description |
|--------|-------------|
| `get` | GET request template |
| `post` | POST request with JSON body |
| `put` | PUT request template |
| `patch` | PATCH request template |
| `delete` | DELETE request template |
| `head` | HEAD request template |
| `options` | OPTIONS request template |
| `ws` | WebSocket request template |
| `jsonrequest` | Complete request with assertions and captures |

### Block Snippets

| Prefix | Description |
|--------|-------------|
| `assert` | Assertion block |
| `capture` | Capture block (`>>>capture`) |
| `graphql` | GraphQL body with variables |
| `multipart` | Multipart form body |
| `shell` | Shell command block |
| `db` | Database assertion block |

### Assertion Snippets

| Prefix | Description |
|--------|-------------|
| `expectstatus` | Status code assertion |
| `expectbody` | Body field assertion |
| `expectheader` | Header assertion |
| `expectduration` | Response time assertion |
| `expectexists` | Field existence assertion |
| `expecttype` | Type assertion |
| `expectlength` | Array length assertion |
| `expectschema` | JSON schema assertion |
| `expectsnapshot` | Snapshot assertion |

### Annotation Snippets

| Prefix | Description |
|--------|-------------|
| `var` | Variable definition |
| `authbasic` | Basic authentication |
| `authbearer` | Bearer token authentication |
| `authoauth` | OAuth2 authentication |
| `skip` | Skip annotation |
| `timeout` | Timeout annotation |
| `retry` | Retry annotation |
| `retryon` | Retry on specific status codes |
| `depends` | Dependency annotation |
| `tags` | Tags annotation |
| `if` | Conditional execution |
| `unless` | Skip if truthy |
| `description` | Request description |
| `stress` | Stress test annotations |
| `contract` | Contract test annotations |
| `waitfor` | Wait for service |

### Function Snippets

| Prefix | Description |
|--------|-------------|
| `uuid` | Generate UUID |
| `timestamp` | Generate timestamp |
| `random` | Random number |
| `env` | Environment variable |
| `file` | File content |
| `base64` | Base64 encode |

## Installation

### From VSIX

1. Download the `.vsix` file
2. In VSCode, open Command Palette (`Cmd+Shift+P` / `Ctrl+Shift+P`)
3. Run "Extensions: Install from VSIX..."
4. Select the downloaded file

### From Marketplace

Search for "hitspec" in the VSCode Extensions marketplace.

## Development

```bash
# Package the extension
npm run package
```

## License

MIT
