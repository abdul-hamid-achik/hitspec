# Contributing to hitspec

Thank you for your interest in contributing to hitspec! This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- Go 1.22 or later
- [Task](https://taskfile.dev/) (optional, for running common commands)

### Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/hitspec.git
   cd hitspec
   ```
3. Install dependencies:
   ```bash
   go mod download
   ```
4. Build the project:
   ```bash
   go build -o bin/hitspec ./apps/cli
   # or with Task:
   task build
   ```
5. Run tests:
   ```bash
   go test ./...
   # or with Task:
   task test
   ```

## Code Style

### Formatting

- Use `go fmt` to format your code
- Use `goimports` to organize imports
- Run `golangci-lint` before submitting (CI will check this)

```bash
go fmt ./...
goimports -w .
golangci-lint run
```

### Guidelines

- Keep functions focused and small
- Write descriptive variable and function names
- Add comments for complex logic
- Follow existing patterns in the codebase

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with race detection
go test -race ./...

# Run specific package tests
go test ./packages/core/parser/...
```

### Writing Tests

- Place test files next to the code they test (`*_test.go`)
- Use table-driven tests for multiple cases
- Test edge cases and error conditions
- Use the `testdata/` directory for test fixtures

## Pull Request Process

1. **Create a branch** for your changes:
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make your changes** following the code style guidelines

3. **Write/update tests** to cover your changes

4. **Run tests** to ensure everything passes:
   ```bash
   go test ./...
   golangci-lint run
   ```

5. **Commit your changes** with a clear message:
   ```bash
   git commit -m "Add feature: description of the feature"
   ```

6. **Push to your fork**:
   ```bash
   git push origin feature/my-feature
   ```

7. **Open a Pull Request** against the `main` branch

### PR Guidelines

- Keep PRs focused on a single change
- Include a clear description of what and why
- Reference any related issues
- Ensure CI passes before requesting review
- Be responsive to feedback

## Project Structure

```
hitspec/
├── apps/cli/           # CLI application
│   └── cmd/            # Cobra commands
├── packages/
│   ├── core/           # Core functionality
│   │   ├── config/     # Configuration handling
│   │   ├── env/        # Environment variables
│   │   ├── parser/     # File parser
│   │   └── runner/     # Test runner
│   ├── http/           # HTTP client
│   ├── output/         # Output formatters
│   ├── assertions/     # Test assertions
│   ├── capture/        # Response capture
│   └── builtin/        # Built-in functions
├── examples/           # Example test files
└── testdata/           # Test fixtures
```

## Reporting Issues

- Search existing issues before creating a new one
- Use the issue template if available
- Include reproduction steps
- Include version information (`hitspec version`)
- Include relevant error messages

## Questions?

Feel free to open an issue for questions about contributing or the codebase.
