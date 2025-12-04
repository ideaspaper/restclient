# Refactoring Plan Phase 2 for REST Client

This document outlines remaining code quality improvements and best practice enhancements following the initial refactoring (REFACTOR.md).

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Completed from REFACTOR.md](#completed-from-refactormd)
3. [Remaining Issues from Point 5](#remaining-issues-from-point-5)
4. [New Issues Discovered](#new-issues-discovered)
5. [Refactoring Plan](#refactoring-plan)
6. [Priority Matrix](#priority-matrix)

---

## Executive Summary

The initial refactoring was largely successful. Key achievements include:
- Utility packages created (`internal/stringutil`, `internal/paths`, `internal/constants`, `internal/filesystem`)
- Configuration system unified with `SetViperDefaults()`
- `cmd/send.go` split into two files (~350 lines each)
- Interfaces defined (`HTTPDoer`, `FileSystem`)
- Custom error types created in `pkg/errors`
- Context support added to HTTP client and scripting engine

However, several items remain incomplete, particularly around adopting the new abstractions and maintaining consistency across the codebase.

---

## Completed from REFACTOR.md

### Phase 1: Foundation
- [x] `internal/stringutil/truncate.go` - `Truncate()` and `TruncateMiddle()`
- [x] `internal/paths/paths.go` - Path utilities with comprehensive functions
- [x] `internal/constants/constants.go` - HTTP headers, MIME types, auth schemes
- [x] `internal/filesystem/filesystem.go` - `FileSystem` interface and `OSFileSystem`
- [x] Tests for utility packages

### Phase 2: Configuration
- [x] Unified config with `config.SetViperDefaults(v)` in `cmd/root.go:100`
- [x] Single source of truth via `DefaultConfig()` function

### Phase 3: Code Structure
- [x] `cmd/send.go` split into `send.go` (350 lines) + `send_execute.go` (373 lines)
- [x] `HTTPDoer` interface defined in `pkg/client/http_client.go:32-40`
- [x] `FileSystem` interface defined in `internal/filesystem/filesystem.go:13-27`

### Phase 4: Error Handling
- [x] Custom error types: `RequestError`, `ParseError`, `ValidationError`, `ScriptError`
- [x] Sentinel errors: `ErrNotFound`, `ErrTimeout`, `ErrCanceled`, etc.
- [x] Helper functions: `Wrap()`, `Wrapf()`, `Is()`, `As()`

### Phase 5: Advanced
- [x] Context support in `HTTPClient.SendWithContext()`
- [x] Context support in `ScriptEngine.ExecuteWithContext()`

---

## Remaining Issues from Point 5

### 5.1 Constants Not Adopted Throughout Codebase

**Status:** Constants defined but NOT used

The constants in `internal/constants/constants.go` are defined but the codebase still uses magic strings extensively.

**Violations Found:**

| File | Line | Magic String | Should Use |
|------|------|--------------|------------|
| `pkg/client/http_client.go` | 73 | `"User-Agent": "restclient-cli"` | `constants.HeaderUserAgent`, `constants.DefaultUserAgent` |
| `pkg/client/http_client.go` | 212 | `"Content-Type"` | `constants.HeaderContentType` |
| `pkg/parser/http_parser.go` | 49-51 | `"User-Agent": "restclient-cli"` | `constants.HeaderUserAgent`, `constants.DefaultUserAgent` |
| `pkg/parser/http_parser.go` | 391, 401, 427, 600 | `"Content-Type"` | `constants.HeaderContentType` |
| `pkg/parser/http_parser.go` | 537, 538 | `"host"`, `"Host"` | `constants.HeaderHost` |
| `pkg/parser/http_parser.go` | 560 | `"cookie"` | `constants.HeaderCookie` |
| `pkg/output/formatter.go` | Multiple | `"Content-Type"`, `"application/json"` | Constants |
| `pkg/variables/processor.go` | Multiple | `".env"` | `constants.ExtEnv` |
| `pkg/scripting/context.go` | Multiple | `"Content-Type"`, `"application/json"` | Constants |

**Solution:**
```bash
# Find and replace magic strings with constants
rg '"Content-Type"' --type go
rg '"User-Agent"' --type go
rg '"Authorization"' --type go
rg '"application/json"' --type go
```

---

### 5.2 Further File Splitting Not Done

**Status:** Partial - only 2 files instead of proposed 4

REFACTOR.md proposed splitting `cmd/send.go` into:
- `cmd/send.go` - Command definition, flags (~150 lines)
- `cmd/send_execute.go` - Request execution logic (~200 lines)
- `cmd/send_interactive.go` - Interactive mode (~150 lines)
- `cmd/send_output.go` - Output handling (~150 lines)

**Current State:**
- `cmd/send.go` (350 lines) - Command + variable processing + request selection
- `cmd/send_execute.go` (373 lines) - Execution + output + dry-run

**Recommendation:** The current split is acceptable. Files are under 400 lines each. Further splitting would add complexity without significant benefit. **Mark as COMPLETE.**

---

### 5.3 Mock Implementations Missing

**Status:** Not implemented

Interfaces exist but no mock implementations for testing:
- `HTTPDoer` - No mock for testing HTTP calls
- `FileSystem` - No mock for testing file operations

**Solution:** Create mock implementations:

```go
// pkg/client/http_client_mock.go
type MockHTTPClient struct {
    Response *models.HttpResponse
    Error    error
    Requests []*models.HttpRequest // Record calls for assertions
}

func (m *MockHTTPClient) Send(req *models.HttpRequest) (*models.HttpResponse, error) {
    m.Requests = append(m.Requests, req)
    return m.Response, m.Error
}
```

```go
// internal/filesystem/mock.go
type MockFileSystem struct {
    Files map[string][]byte
    Dirs  map[string]bool
    Err   error
}
```

---

### 5.4 Dependency Injection Not Fully Implemented

**Status:** Partial

The interfaces exist but direct instantiation is still common:

| File | Line | Issue |
|------|------|-------|
| `cmd/send_execute.go` | 57 | `client.NewHttpClient(clientCfg)` - Direct instantiation |
| `cmd/send_execute.go` | 87 | `history.NewHistoryManager("")` - Direct instantiation |
| `pkg/session/session.go` | 114, 140 | Uses `os.ReadFile`, `os.WriteFile` directly |
| `pkg/history/history.go` | Multiple | Uses `os` directly instead of `FileSystem` interface |

**Solution:** 
1. Add factory functions that accept interfaces
2. Create a simple DI container or use constructor injection
3. Update session and history to use `filesystem.Default`

---

### 5.5 Context Propagation Incomplete

**Status:** Partial - infrastructure exists, not fully used

Context support exists in:
- `HTTPClient.SendWithContext(ctx, request)`
- `ScriptEngine.ExecuteWithContext(ctx, script, scriptCtx)`

But context is NOT propagated from:
- `cmd/send.go` - Uses `Send()` not `SendWithContext()`
- `pkg/parser/http_parser.go` - No context parameter
- `pkg/session/session.go` - No context parameter

**Solution:**
```go
// In cmd/send_execute.go
ctx, cancel := context.WithTimeout(context.Background(), cfg.GetTimeout())
defer cancel()
resp, err := httpClient.SendWithContext(ctx, request)
```

---

## New Issues Discovered

### 6.1 Deprecated Wrapper Function Still Present

**Location:** `cmd/colors.go:121-125`

```go
// truncateString truncates a string to maxLen and adds ellipsis
// Deprecated: Use stringutil.Truncate from pkg/internal/stringutil instead
func truncateString(s string, maxLen int) string {
    return stringutil.Truncate(s, maxLen)
}
```

**Callers:** `cmd/send.go:41` (RequestItem.Title)

**Solution:** 
1. Update callers to use `stringutil.Truncate()` directly
2. Remove deprecated wrapper

---

### 6.2 Inconsistent Color Handling in send_execute.go

**Location:** `cmd/send_execute.go:215-228`

```go
func printTestResults(tests []scripting.TestResult) {
    passColor := color.New(color.FgGreen)  // Creates new color instance
    failColor := color.New(color.FgRed)    // Creates new color instance
    ...
}
```

This violates the centralized color handling in `cmd/colors.go`.

**Solution:** Add helpers to `cmd/colors.go`:
```go
func printTestPass(name string) {
    if useColors() {
        fmt.Printf("  %s %s\n", successColor.Sprint("✓"), name)
    } else {
        fmt.Printf("  [PASS] %s\n", name)
    }
}

func printTestFail(name, err string) {
    if useColors() {
        fmt.Printf("  %s %s: %s\n", errorColor.Sprint("✗"), name, err)
    } else {
        fmt.Printf("  [FAIL] %s: %s\n", name, err)
    }
}
```

---

### 6.3 Custom Error Types Not Widely Adopted

**Status:** Error types defined but rarely used

The `pkg/errors` package has excellent error types, but most code still uses `fmt.Errorf()`:

| Package | Error Type Available | Currently Uses |
|---------|---------------------|----------------|
| `pkg/parser` | `ParseError` | `fmt.Errorf()` |
| `pkg/client` | `RequestError` | `fmt.Errorf()` |
| `pkg/scripting` | `ScriptError` | `fmt.Errorf()` |
| `pkg/config` | `ErrConfig` | `fmt.Errorf()` |

**Example Fix for parser:**
```go
// Before
return nil, fmt.Errorf("no request line found")

// After
return nil, errors.NewParseError(filename, lineNum, "no request line found")
```

---

### 6.4 Session and History Don't Use FileSystem Interface

**Location:** `pkg/session/session.go`, `pkg/history/history.go`

Both packages use `os.ReadFile`/`os.WriteFile` directly instead of the `FileSystem` interface:

```go
// session.go:114
data, err := os.ReadFile(path)

// session.go:140
return os.WriteFile(path, data, 0644)
```

**Solution:** Inject `FileSystem` interface or use `filesystem.Default`:
```go
type SessionManager struct {
    fs          filesystem.FileSystem
    // ...
}

func NewSessionManager(fs filesystem.FileSystem, ...) (*SessionManager, error) {
    if fs == nil {
        fs = filesystem.Default
    }
    // ...
}
```

---

### 6.5 Missing Test Coverage

**Files Without Tests:**
- `internal/filesystem/filesystem.go` - No `filesystem_test.go`
- `pkg/errors/errors.go` - No `errors_test.go`
- `cmd/colors.go` - No tests for color helpers

**Recommendation:** Add tests for these packages to ensure reliability.

---

### 6.6 Global Variables in cmd Package

**Status:** Unchanged from REFACTOR.md

Still present in multiple files:

```go
// cmd/root.go:15-26
var (
    version     = "dev"
    v           *viper.Viper
    cfgFile     string
    environment string
    verbose     bool
)

// cmd/send.go:53-65
var (
    requestName  string
    requestIndex int
    showHeaders  bool
    // ... more flags
)
```

**Impact:** Makes testing difficult, potential for race conditions in concurrent usage.

**Solution:** (Lower priority) Create `cmdContext` struct:
```go
type cmdContext struct {
    config      *config.Config
    verbose     bool
    environment string
    // ...
}
```

---

### 6.7 Parser Package Too Large

**Location:** `pkg/parser/http_parser.go` (901 lines)

The parser file handles too many responsibilities:
- Request block splitting
- Request line parsing
- Header parsing
- Body parsing
- Multipart parsing
- GraphQL handling
- File reading
- Metadata extraction

**Proposed Split:**
```
pkg/parser/
  http_parser.go      (~200 lines) - Main parser, block splitting
  request_parser.go   (~200 lines) - Request line, headers
  body_parser.go      (~200 lines) - Body, multipart, GraphQL
  metadata.go         (~100 lines) - Metadata extraction
  file_reader.go      (~100 lines) - File reading utilities
```

---

### 6.8 Inconsistent Header Case-Insensitive Lookup

**Issue from REFACTOR.md Point 5:** Still present in some places

The `getHeaderCaseInsensitive()` function exists in `pkg/parser/http_parser.go:573-580` but similar patterns are repeated:

| File | Line | Pattern |
|------|------|---------|
| `pkg/client/http_client.go` | 267-271 | Manual loop for "authorization" |
| `pkg/variables/processor.go` | 441-445 | Manual loop for headers |

**Solution:** Create shared header utility:
```go
// internal/httputil/headers.go
func GetHeader(headers map[string]string, key string) (string, bool) {
    for k, v := range headers {
        if strings.EqualFold(k, key) {
            return v, true
        }
    }
    return "", false
}
```

---

## Refactoring Plan

### Phase 1: Quick Wins (1-2 hours)

1. **Adopt constants throughout codebase**
   - [ ] Replace magic strings with `constants.HeaderContentType`, etc.
   - [ ] Replace `"restclient-cli"` with `constants.DefaultUserAgent`
   - [ ] Replace MIME type strings with constants

2. **Remove deprecated wrapper**
   - [ ] Update `cmd/send.go:41` to use `stringutil.Truncate()`
   - [ ] Remove `truncateString()` from `cmd/colors.go`

3. **Fix color handling in send_execute.go**
   - [ ] Add `printTestPass()` and `printTestFail()` to `cmd/colors.go`
   - [ ] Update `printTestResults()` to use new helpers

### Phase 2: Testing Infrastructure (2-3 hours)

4. **Add mock implementations**
   - [ ] Create `pkg/client/http_client_mock.go`
   - [ ] Create `internal/filesystem/mock.go`

5. **Add missing tests**
   - [ ] Add `internal/filesystem/filesystem_test.go`
   - [ ] Add `pkg/errors/errors_test.go`
   - [ ] Add basic tests for `cmd/colors.go` helpers

### Phase 3: Interface Adoption (3-4 hours)

6. **Update session to use FileSystem**
   - [ ] Add `FileSystem` parameter to `NewSessionManager`
   - [ ] Replace `os.ReadFile`/`os.WriteFile` with interface calls
   - [ ] Update tests to use mock filesystem

7. **Update history to use FileSystem**
   - [ ] Add `FileSystem` parameter to `NewHistoryManager`
   - [ ] Replace direct `os` calls with interface

8. **Create shared header utility**
   - [ ] Create `internal/httputil/headers.go`
   - [ ] Add `GetHeader()`, `SetHeader()`, `DeleteHeader()`
   - [ ] Update parser, client, variables to use shared utility

### Phase 4: Error Handling Adoption (2-3 hours)

9. **Adopt custom error types in parser**
   - [ ] Use `ParseError` for parsing failures
   - [ ] Add file and line context to errors

10. **Adopt custom error types in client**
    - [ ] Use `RequestError` for HTTP failures
    - [ ] Include method and URL in error context

11. **Adopt custom error types in scripting**
    - [ ] Use `ScriptError` for script execution failures

### Phase 5: Context Propagation (2-3 hours)

12. **Propagate context from cmd to client**
    - [ ] Create context with timeout in `sendRequest()`
    - [ ] Pass context through to `SendWithContext()`
    - [ ] Add cancellation signal handling

13. **Add context to parser (optional)**
    - [ ] Add `ParseWithContext()` methods
    - [ ] Support cancellation during large file parsing

### Phase 6: Code Organization (4-6 hours) - Optional

14. **Split parser package**
    - [ ] Extract body parsing to `body_parser.go`
    - [ ] Extract metadata to `metadata.go`
    - [ ] Keep main orchestration in `http_parser.go`

15. **Refactor global variables** (Lower priority)
    - [ ] Create `cmdContext` struct
    - [ ] Pass context through command execution
    - [ ] Update tests to use context

---

## Priority Matrix

| Issue | Impact | Effort | Priority | Phase |
|-------|--------|--------|----------|-------|
| Adopt constants | Medium | Low | P1 | 1 |
| Remove deprecated wrapper | Low | Low | P1 | 1 |
| Fix color handling | Low | Low | P1 | 1 |
| Add mock implementations | High | Medium | P1 | 2 |
| Add missing tests | Medium | Medium | P2 | 2 |
| Session use FileSystem | Medium | Medium | P2 | 3 |
| History use FileSystem | Medium | Medium | P2 | 3 |
| Shared header utility | Low | Low | P2 | 3 |
| Adopt ParseError | Medium | Medium | P2 | 4 |
| Adopt RequestError | Medium | Medium | P2 | 4 |
| Context propagation | High | Medium | P2 | 5 |
| Split parser | Low | High | P3 | 6 |
| Refactor globals | Medium | High | P3 | 6 |

---

## Quick Reference: Import Paths

When adopting utilities, use these import paths:

```go
import (
    "github.com/ideaspaper/restclient/internal/constants"
    "github.com/ideaspaper/restclient/internal/filesystem"
    "github.com/ideaspaper/restclient/internal/paths"
    "github.com/ideaspaper/restclient/internal/stringutil"
    "github.com/ideaspaper/restclient/pkg/errors"
)
```

---

---

## Appendix: Go Modernization Hints

The following minor improvements can be made to use modern Go idioms:

### Replace `interface{}` with `any` (Go 1.18+)

| File | Lines |
|------|-------|
| `pkg/errors/errors.go` | 184, 199 |
| `pkg/session/session.go` | 43, 77, 279, 305, 310, 326 |
| `pkg/scripting/engine.go` | Multiple (38, 71, 126, etc.) |
| `cmd/session.go` | 269, 278 |
| `pkg/output/formatter.go` | 184 |
| `pkg/variables/processor.go` | 81 |
| `cmd/send_execute.go` | 203 |

### Use `maps.Copy` for map copying (Go 1.21+)

| File | Line | Current | Replacement |
|------|------|---------|-------------|
| `cmd/send_execute.go` | 93 | `for k, v := range {...}` | `maps.Copy(dest, src)` |
| `pkg/variables/processor.go` | 81 | Loop assignment | `maps.Copy(vp.fileVariables, vars)` |

### Use `min`/`max` built-ins (Go 1.21+)

| File | Line | Current | Replacement |
|------|------|---------|-------------|
| `pkg/tui/selector.go` | 214 | `if x > y { x = y }` | `x = max(x, y)` |
| `pkg/tui/selector.go` | 315 | `if x < y { x = y }` | `x = min(x, y)` |

### Use range over int (Go 1.22+)

| File | Line | Current | Replacement |
|------|------|---------|-------------|
| `pkg/variables/processor.go` | 518 | `for i := 0; i < len(s); i++` | `for i := range len(s)` |

### Use `strings.SplitSeq` for iteration (Go 1.24+)

| File | Line | Note |
|------|------|------|
| `pkg/parser/http_parser.go` | 668, 703 | Can use `SplitSeq` for more efficient iteration |

### Remove unused code

| File | Line | Issue |
|------|------|-------|
| `cmd/colors.go` | 93, 102 | `printSuccess()`, `printError()` are defined but unused |
| `pkg/parser/http_parser.go` | 786 | Parameter `encoding` in `readFile()` is unused |

---

## Notes

- All refactoring should maintain backward compatibility
- Run `go test ./...` after each change
- Update documentation as code changes
- Consider adding benchmarks for performance-critical paths (HTTP client, parser)
- The `internal/` packages cannot be imported by external consumers, which is intentional for utilities
- Go version modernization hints require minimum Go version; check `go.mod` before applying
