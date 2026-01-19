# hitspec CLI Reference

Complete reference for the hitspec command-line interface.

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

---

## Commands

### hitspec run

Run HTTP tests from `.http` files.

```bash
hitspec run <file|directory> [flags]
```

**Arguments:**
- `<file>` - Single `.http` file to run
- `<directory>` - Directory containing `.http` files (recursive)

**Examples:**

```bash
# Run a single file
hitspec run tests/api.http

# Run all tests in a directory
hitspec run tests/

# Run with specific environment
hitspec run tests/ --env staging

# Run only smoke tests
hitspec run tests/ --tags smoke

# Run specific test by name
hitspec run tests/ --name "Login"

# Run in parallel
hitspec run tests/ --parallel --concurrency 10

# Watch mode (re-run on file changes)
hitspec run tests/ --watch

# Output as JSON
hitspec run tests/ --output json

# Stop on first failure
hitspec run tests/ --bail

# Dry run (show what would run)
hitspec run tests/ --dry-run
```

**Flags:**

| Flag | Short | Description | Default | Env Var |
|------|-------|-------------|---------|---------|
| `--env` | `-e` | Environment name | `dev` | `HITSPEC_ENV` |
| `--env-file` | | Path to .env file | | `HITSPEC_ENV_FILE` |
| `--config` | | Path to config file | | `HITSPEC_CONFIG` |
| `--name` | `-n` | Filter by request name (pattern match) | | |
| `--tags` | `-t` | Filter by tags (comma-separated) | | `HITSPEC_TAGS` |
| `--verbose` | `-v` | Show detailed output (use -vv or -vvv for more) | `false` | |
| `--quiet` | `-q` | Suppress all output except errors | `false` | `HITSPEC_QUIET` |
| `--bail` | | Stop on first failure | `false` | `HITSPEC_BAIL` |
| `--timeout` | | Request timeout (e.g., 30s, 1m) | `30s` | `HITSPEC_TIMEOUT` |
| `--no-color` | | Disable colored output | `false` | `HITSPEC_NO_COLOR` |
| `--dry-run` | | Parse and show what would run | `false` | |
| `--output` | `-o` | Output format: `console`, `json`, `junit`, `tap` | `console` | `HITSPEC_OUTPUT` |
| `--output-file` | | Write output to file | | `HITSPEC_OUTPUT_FILE` |
| `--parallel` | `-p` | Run requests in parallel | `false` | `HITSPEC_PARALLEL` |
| `--concurrency` | | Max concurrent requests | `5` | `HITSPEC_CONCURRENCY` |
| `--watch` | `-w` | Watch files and re-run on changes | `false` | |
| `--proxy` | | Proxy URL for HTTP requests | | `HITSPEC_PROXY` |
| `--insecure` | `-k` | Disable SSL certificate validation | `false` | `HITSPEC_INSECURE` |

---

### hitspec validate

Validate `.http` files without running them.

```bash
hitspec validate <file|directory>
```

**Examples:**

```bash
# Validate a single file
hitspec validate tests/api.http

# Validate all files in directory
hitspec validate tests/
```

**Output:**
- Reports syntax errors
- Reports invalid assertions
- Reports undefined variables
- Reports circular dependencies

---

### hitspec list

List all requests in `.http` files.

```bash
hitspec list <file|directory>
```

**Examples:**

```bash
# List requests in a file
hitspec list tests/api.http

# List all requests in directory
hitspec list tests/
```

**Output:**
```
tests/api.http:
  - healthCheck (tags: smoke)
  - login (tags: auth)
  - getProfile (tags: users, depends: login)
  - updateProfile (tags: users, depends: getProfile)
```

---

### hitspec init

Initialize a new hitspec project with example files.

```bash
hitspec init
hitspec init --force  # Overwrite existing files
```

**Creates:**
- `hitspec.yaml` - Configuration file with environments
- `example.http` - Example test file

---

### hitspec version

Show version information.

```bash
hitspec version
```

---

### hitspec completion

Generate shell completion scripts for bash, zsh, fish, or PowerShell.

```bash
hitspec completion [bash|zsh|fish|powershell]
```

**Examples:**

```bash
# Bash (Linux)
hitspec completion bash > /etc/bash_completion.d/hitspec

# Bash (macOS with Homebrew)
hitspec completion bash > $(brew --prefix)/etc/bash_completion.d/hitspec

# Zsh
hitspec completion zsh > "${fpath[1]}/_hitspec"

# Fish
hitspec completion fish > ~/.config/fish/completions/hitspec.fish

# PowerShell
hitspec completion powershell > hitspec.ps1
```

---

## Output Formats

### Console (default)

Human-readable output with colors:

```
Running: tests/api.http

  ✓ healthCheck (45ms)
  ✓ login (120ms)
  ✗ getProfile (89ms)
    → status ==
      Expected: 200
      Actual:   401
      expected 200, got 401

Tests: 2 passed, 1 failed, 3 total
Time:  254ms
```

### JSON

Machine-readable JSON output:

```bash
hitspec run tests/ --output json
```

```json
{
  "file": "tests/api.http",
  "passed": 2,
  "failed": 1,
  "skipped": 0,
  "duration": 254,
  "results": [
    {
      "name": "healthCheck",
      "passed": true,
      "duration": 45
    }
  ]
}
```

### JUnit XML

For CI/CD integration:

```bash
hitspec run tests/ --output junit --output-file results.xml
```

### TAP (Test Anything Protocol)

```bash
hitspec run tests/ --output tap
```

```
TAP version 13
1..3
ok 1 - healthCheck
ok 2 - login
not ok 3 - getProfile
  ---
  message: expected 200, got 401
  ---
```

---

## Filtering Tests

### By Tags

Run only requests with specific tags:

```bash
# Single tag
hitspec run tests/ --tags smoke

# Multiple tags (OR logic)
hitspec run tests/ --tags smoke,critical

# Exclude by using @skip in test file
```

### By Name

Run requests matching a name pattern:

```bash
# Exact match
hitspec run tests/ --name "Login"

# Pattern match
hitspec run tests/ --name "User*"
hitspec run tests/ --name "*Profile"
```

### Combining Filters

```bash
hitspec run tests/ --tags smoke --name "*Auth*"
```

---

## Parallel Execution

Run independent requests in parallel:

```bash
hitspec run tests/ --parallel --concurrency 10
```

**Notes:**
- Requests with dependencies run sequentially
- Only independent requests run in parallel
- Default concurrency is 5
- Captures are not shared between parallel requests

---

## Watch Mode

Re-run tests when files change:

```bash
hitspec run tests/ --watch
```

**Watches:**
- `.http` files in the specified directory
- `.hitspec.env.json` environment file

---

## Environment Selection

Select which environment to use:

```bash
# Use dev environment (default)
hitspec run tests/

# Use staging environment
hitspec run tests/ --env staging

# Use production environment
hitspec run tests/ --env prod
```

Environment variables are loaded from `.hitspec.env.json`. See [environments.md](environments.md).

---

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | All tests passed |
| 1 | One or more tests failed |
| 2 | Parse error (invalid .http file syntax) |
| 3 | Configuration error |
| 4 | Network/connection error |
| 64 | Invalid CLI usage |

---

## Examples

### CI/CD Pipeline

```yaml
# GitHub Actions
- name: Run API Tests
  run: |
    hitspec run tests/ \
      --env staging \
      --output junit \
      --output-file test-results.xml \
      --bail

- name: Upload Test Results
  uses: actions/upload-artifact@v3
  with:
    name: test-results
    path: test-results.xml
```

### Local Development

```bash
# Quick smoke test
hitspec run tests/ --tags smoke

# Full test suite with details
hitspec run tests/ --verbose

# Debug specific test
hitspec run tests/ --name "Login" --verbose

# Watch mode during development
hitspec run tests/ --watch --tags dev
```

### Debugging

```bash
# Dry run to see what would execute
hitspec run tests/ --dry-run

# Validate syntax without running
hitspec validate tests/

# List all requests
hitspec list tests/

# Verbose output for debugging
hitspec run tests/ --verbose
```

---

## See Also

- [Syntax Reference](README.md) - Complete .http file syntax
- [Environment Configuration](environments.md) - Environment setup
