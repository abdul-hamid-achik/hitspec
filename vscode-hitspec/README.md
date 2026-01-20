# hitspec VSCode Extension

Syntax highlighting and snippets for hitspec HTTP API testing files.

## Features

- **Syntax Highlighting** for `.http` and `.hitspec` files
- **Code Snippets** for common patterns
- **Language Configuration** for comments, brackets, and folding

## Supported File Extensions

- `.http`
- `.hitspec`

## Syntax Highlighting

The extension provides highlighting for:

- HTTP methods (GET, POST, PUT, PATCH, DELETE, etc.)
- Request separators (`###`)
- Annotations (`@name`, `@tags`, `@auth`, etc.)
- Headers
- Assertion blocks (`>>>` ... `<<<`)
- Capture blocks (`[[[` ... `]]]`)
- Variable interpolation (`{{variable}}`)
- Built-in functions (`$uuid()`, `$timestamp()`, etc.)
- Comments (`#`)

## Snippets

### Request Snippets

| Prefix | Description |
|--------|-------------|
| `get` | GET request template |
| `post` | POST request with JSON body |
| `put` | PUT request template |
| `patch` | PATCH request template |
| `delete` | DELETE request template |
| `jsonrequest` | Complete request with assertions and captures |

### Assertion Snippets

| Prefix | Description |
|--------|-------------|
| `assert` | Assertion block |
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
| `depends` | Dependency annotation |
| `tags` | Tags annotation |
| `stress` | Stress test annotations |
| `contract` | Contract test annotations |
| `waitfor` | Wait for service |
| `db` | Database assertion block |

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
# Install dependencies
npm install

# Compile
npm run compile

# Package
npm run package
```

## License

MIT
