# Contributing to restclient

Thank you for your interest in contributing to restclient! This document provides guidelines and best practices for contributing to this project.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Code Style Guidelines](#code-style-guidelines)
- [Testing Guidelines](#testing-guidelines)
- [Error Handling](#error-handling)
- [Documentation](#documentation)
- [Pull Request Process](#pull-request-process)

## Getting Started

### Prerequisites

- Go 1.24.4 or later
- golangci-lint (for linting)
- Make (for build automation)

### Development Setup

```bash
# Clone the repository
git clone https://github.com/ideaspaper/restclient.git
cd restclient

# Install dependencies
go mod download

# Build the project
make build

# Run tests
make test

# Run linter
make lint
```

## Project Structure

```
restclient/
├── main.go              # Entry point (minimal code)
├── cmd/                 # CLI commands (Cobra-based)
├── internal/            # Private packages (not exposed outside module)
│   ├── constants/       # Shared constants (headers, MIME types)
│   ├── filesystem/      # File system abstraction for testability
│   ├── httputil/        # HTTP utility functions
│   ├── paths/           # Path resolution utilities
│   └── stringutil/      # String manipulation helpers
├── pkg/                 # Public, reusable packages
│   ├── auth/            # Authentication handlers
│   ├── client/          # HTTP client implementation
│   ├── config/          # Configuration management
│   ├── errors/          # Custom error types
│   ├── executor/        # Request execution logic
│   ├── history/         # Request history storage
│   ├── lastfile/        # Last used file tracking
│   ├── models/          # Data models (Request, Response)
│   ├── output/          # Response formatting
│   ├── parser/          # .http/.rest file parser
│   ├── postman/         # Postman import/export
│   ├── scripting/       # JavaScript scripting engine
│   ├── session/         # Session management
│   ├── tui/             # Terminal UI components
│   └── variables/       # Variable processing
└── examples/            # Example .http files
```

### Package Guidelines

- **`internal/`**: Private packages that cannot be imported outside the module. Use for implementation details.
- **`pkg/`**: Public, reusable packages. Use for functionality that could be used by external consumers.
- **`cmd/`**: CLI command definitions using Cobra framework.
- Each package should have a focused, single responsibility.

## Code Style Guidelines

### General Principles

1. Follow standard Go conventions and idioms
2. Keep functions small and focused
3. Prefer composition over inheritance
4. Use dependency injection for testability

### Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Package names | Short, lowercase, no underscores | `httputil`, `stringutil` |
| Exported identifiers | PascalCase | `HttpRequest`, `NewParser` |
| Unexported identifiers | camelCase | `parseMetadata`, `isComment` |
| Acronyms | All caps | `URL`, `HTTP`, `JSON`, `SSL` |
| File names | Lowercase with underscores | `http_parser.go`, `http_client.go` |
| Test files | `*_test.go` suffix | `config_test.go` |
| Mock files | `*_mock.go` suffix | `http_client_mock.go` |
| Fuzz test files | `*_fuzz_test.go` suffix | `http_parser_fuzz_test.go` |

### Function and Method Naming

| Pattern | Convention | Example |
|---------|------------|---------|
| Constructors | `New` prefix | `NewHttpClient`, `NewParser` |
| Getters | No `Get` prefix | `ContentType()`, not `GetContentType()` |
| Boolean getters | `Is`/`Has` prefix | `IsValid()`, `HasPrefix()` |
| Builder methods | `With` prefix | `WithResponse()`, `WithError()` |
| Parse functions | `Parse` prefix | `ParseAll`, `ParseRequest` |

### Variable Naming

- **Receivers**: Short, 1-2 letters (`c` for config, `p` for parser, `r` for request)
- **Loop variables**: `i`, `j` for indices; `k`, `v` for key-value pairs
- **Test cases**: `tt` for test table entries, `got`/`want` for assertions

### Import Organization

Organize imports in three groups, separated by blank lines:

```go
import (
    // Standard library
    "context"
    "fmt"
    "net/http"

    // External dependencies
    "github.com/spf13/cobra"
    "github.com/spf13/viper"

    // Internal packages
    "github.com/ideaspaper/restclient/internal/constants"
    "github.com/ideaspaper/restclient/pkg/models"
)
```

### Constants Organization

Group constants by category:

```go
const (
    // HTTP Headers
    HeaderContentType   = "Content-Type"
    HeaderAuthorization = "Authorization"
)

const (
    // MIME Types
    MIMEApplicationJSON = "application/json"
    MIMETextPlain       = "text/plain"
)
```

### Struct Tags

Use consistent struct tags for JSON and mapstructure:

```go
type Config struct {
    FollowRedirects bool `json:"followRedirect" mapstructure:"followRedirect"`
    TimeoutMs       int  `json:"timeoutInMilliseconds" mapstructure:"timeoutInMilliseconds"`
}
```

## Testing Guidelines

### Test Structure

Use table-driven tests as the primary pattern:

```go
func TestParseRequestLine(t *testing.T) {
    tests := []struct {
        name       string
        input      string
        wantMethod string
        wantURL    string
    }{
        {
            name:       "simple GET",
            input:      "GET https://api.example.com/users",
            wantMethod: "GET",
            wantURL:    "https://api.example.com/users",
        },
        {
            name:       "POST with path",
            input:      "POST https://api.example.com/users/create",
            wantMethod: "POST",
            wantURL:    "https://api.example.com/users/create",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            method, url := parseRequestLine(tt.input)
            if method != tt.wantMethod {
                t.Errorf("parseRequestLine() method = %v, want %v", method, tt.wantMethod)
            }
            if url != tt.wantURL {
                t.Errorf("parseRequestLine() url = %v, want %v", url, tt.wantURL)
            }
        })
    }
}
```

### Testing Conventions

1. **Subtest naming**: Use descriptive names for subtests
2. **Error messages**: Use format `"got X, want Y"` or `"field = X, want Y"`
3. **Test isolation**: Use `t.TempDir()` for temporary directories
4. **Cleanup**: Use `defer` for cleanup operations
5. **HTTP testing**: Use `httptest.NewServer()` for HTTP client tests

### Mock Pattern

Define interfaces for testability and create mock implementations:

```go
// Interface definition
type HTTPDoer interface {
    Send(request *models.HttpRequest) (*models.HttpResponse, error)
    SendWithContext(ctx context.Context, request *models.HttpRequest) (*models.HttpResponse, error)
}

// Mock implementation
type MockHTTPClient struct {
    Response     *models.HttpResponse
    Error        error
    Requests     []*models.HttpRequest
    ResponseFunc func(req *models.HttpRequest) (*models.HttpResponse, error)
}

// Ensure interface compliance at compile time
var _ HTTPDoer = (*MockHTTPClient)(nil)

// Fluent builder methods for test setup
func (m *MockHTTPClient) WithResponse(resp *models.HttpResponse) *MockHTTPClient {
    m.Response = resp
    return m
}
```

### Fuzz Testing

Use Go's native fuzzing for parser robustness:

```go
func FuzzParseRequest(f *testing.F) {
    // Add seed corpus
    f.Add("GET https://example.com\n")
    f.Add("POST https://example.com\nContent-Type: application/json\n\n{}")

    f.Fuzz(func(t *testing.T, input string) {
        // Parser should not panic on any input
        parser := NewHttpRequestParser(input, nil, "")
        _ = parser.ParseAll()
    })
}
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific package tests
go test -v ./pkg/parser/...

# Run fuzz tests
go test -fuzz=FuzzParseRequest ./pkg/parser/
```

## Error Handling

### Custom Error Types

Use the custom error package for consistent error handling:

```go
import "github.com/ideaspaper/restclient/pkg/errors"

// Use sentinel errors for comparison
if err != nil {
    return errors.ErrNotFound
}

// Wrap errors with context
if err != nil {
    return errors.Wrap(err, "failed to read config file")
}

// Use formatted wrapping
if err != nil {
    return errors.Wrapf(err, "failed to parse request at line %d", lineNum)
}

// Create validation errors
if name == "$shared" {
    return errors.NewValidationErrorWithValue("environment", "$shared", "cannot use reserved name")
}
```

### Error Handling Best Practices

1. **Always wrap errors** with context using `errors.Wrap()` or `errors.Wrapf()`
2. **Use sentinel errors** for errors that callers need to check with `errors.Is()`
3. **Use structured errors** when additional context is needed
4. **Nil-safe wrapping**: `errors.Wrap()` returns nil if the error is nil

```go
func LoadConfig() (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, errors.ErrNotFound
        }
        return nil, errors.Wrap(err, "failed to read config file")
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, errors.Wrap(err, "failed to parse config file")
    }

    return &cfg, nil
}
```

## Documentation

### Package Documentation

Add a package-level comment at the top of the main file:

```go
// Package errors provides custom error types and utilities for the REST client.
package errors
```

### Function Documentation

Document exported functions starting with the function name:

```go
// NewHttpRequestParser creates a new parser for HTTP request files.
// It accepts the file content, optional default headers, and the base directory
// for resolving relative file paths.
func NewHttpRequestParser(content string, defaultHeaders map[string]string, baseDir string) *HttpRequestParser {
    // ...
}
```

### Documentation Guidelines

1. Start documentation with the identifier name
2. Be concise but descriptive
3. Document exported items; unexported items are optional
4. Use complete sentences
5. Avoid redundant phrases like "This function..."

## Pull Request Process

### Before Submitting

1. **Run the linter**: `make lint`
2. **Run all tests**: `make test`
3. **Format code**: Code is auto-formatted by `gofmt` and `goimports`
4. **Update documentation** if adding new features

### Commit Messages

Write clear, concise commit messages:

- Use present tense ("Add feature" not "Added feature")
- Use imperative mood ("Move cursor to..." not "Moves cursor to...")
- Keep the first line under 72 characters
- Reference issues when applicable

```
Add GraphQL subscription support

- Implement WebSocket connection handling
- Add subscription query parsing
- Update documentation with examples

Fixes #123
```

### Code Review Checklist

Before requesting review, ensure:

- [ ] Code follows the style guidelines in this document
- [ ] All tests pass
- [ ] New code has appropriate test coverage
- [ ] Documentation is updated for new features
- [ ] No linter warnings
- [ ] Commit history is clean and logical

### Linting Configuration

The project uses golangci-lint v2 with the following key settings:

- **gocyclo**: Maximum cyclomatic complexity of 25
- **nakedret**: No naked returns in functions > 30 lines
- **bodyclose**: HTTP response bodies must be closed
- **gosec**: Security checks enabled (with some exceptions for CLI tool patterns)

See `.golangci.yml` for the complete configuration.

## Additional Resources

- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
