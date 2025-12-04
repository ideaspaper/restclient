# Refactoring Plan Phase 3 for REST Client

This document provides an honest review of progress made on REFACTOR2.md and identifies remaining issues and improvements.

## Table of Contents

1. [Progress Summary](#progress-summary)
2. [Completed Items](#completed-items)
3. [Incomplete Items from REFACTOR2.md](#incomplete-items-from-refactor2md)
4. [New Issues Discovered](#new-issues-discovered)
5. [Remaining Refactoring Plan](#remaining-refactoring-plan)
6. [Priority Matrix](#priority-matrix)

---

## Progress Summary

### What Was Done Well

1. **Mock Implementations** - Created comprehensive mocks:
   - `pkg/client/http_client_mock.go` - Thread-safe, fluent API, records requests
   - `internal/filesystem/mock.go` - In-memory FS with error injection

2. **FileSystem Interface Adoption** - Session and History now use the interface:
   - `NewSessionManagerWithFS()` and `NewHistoryManagerWithFS()` added
   - Core CRUD operations use `fs.ReadFile()`, `fs.WriteFile()`, `fs.MkdirAll()`
   - Backward compatibility maintained

3. **Context Propagation** - Full implementation:
   - `ExecuteWithContext()` in executor package
   - Signal handling (Ctrl+C) with cancellation
   - Timeout support from config
   - Scripts use `ExecuteWithContext()`

4. **Removed Unused Code**:
   - Removed `setupScriptContext()` and `applyScriptGlobalVars()` from `cmd/send_execute.go`

5. **Test Coverage**:
   - `pkg/errors/errors_test.go` - Comprehensive tests
   - `internal/filesystem/filesystem_test.go` - Tests for OSFileSystem and helpers

---

## Completed Items

### From REFACTOR2.md Phase 2 (Testing Infrastructure)
- [x] Create `pkg/client/http_client_mock.go`
- [x] Create `internal/filesystem/mock.go`
- [x] Add `internal/filesystem/filesystem_test.go`
- [x] Add `pkg/errors/errors_test.go`

### From REFACTOR2.md Phase 3 (Interface Adoption)
- [x] Add `FileSystem` parameter to `NewSessionManager` (`NewSessionManagerWithFS`)
- [x] Replace `os.ReadFile`/`os.WriteFile` with interface calls in session
- [x] Add `FileSystem` parameter to `NewHistoryManager` (`NewHistoryManagerWithFS`)
- [x] Replace direct `os` calls with interface in history

### From REFACTOR2.md Phase 5 (Context Propagation)
- [x] Create context with timeout in `sendRequest()`
- [x] Pass context through to `SendWithContext()`
- [x] Add cancellation signal handling (SIGINT/SIGTERM)

### From REFACTOR2.md Appendix (Go Modernization)
- [x] Already using `any` instead of `interface{}` (only in test files now)
- [x] Using `maps.Copy` in executor package

---

## Incomplete Items from REFACTOR2.md

### 5.1 Constants Not Adopted Throughout Codebase

**Status:** NOT DONE

Magic strings still exist in test files and some production code:

| File | Issue |
|------|-------|
| `pkg/client/http_client_test.go` | Uses `"Content-Type"`, `"User-Agent"` |
| `pkg/parser/http_parser_test.go` | Uses `"Content-Type"`, `"User-Agent"` |
| `pkg/output/formatter_test.go` | Uses `"Content-Type"` |
| `pkg/scripting/engine_test.go` | Uses `"Content-Type"` |
| `pkg/postman/import_test.go` | Uses `"Content-Type"` |
| `pkg/postman/export_test.go` | Uses `"Content-Type"` |

**Note:** Production code is clean; only test files have magic strings. This is lower priority but should be addressed for consistency.

---

### 6.1 Deprecated Wrapper Function

**Status:** DONE

The `truncateString()` function was removed. `cmd/send.go` now uses `stringutil.Truncate()` directly.

---

### 6.2 Inconsistent Color Handling

**Status:** PARTIALLY DONE

The `printTestResults()` function in `cmd/send_execute.go` now uses the centralized `successColor` and `errorColor` from `cmd/colors.go` instead of creating new color instances.

However, there's no `printTestPass()` and `printTestFail()` helper functions as suggested. The current implementation is acceptable but could be cleaner.

---

### 6.4 Session ListAllSessions Still Uses os.ReadDir

**Status:** INCOMPLETE

`ListAllSessionsWithFS()` in `pkg/session/session.go:409,418` still uses `os.ReadDir()` directly instead of the filesystem interface.

**Problem:** The `FileSystem` interface doesn't include a `ReadDir` method.

**Current Code:**
```go
func ListAllSessionsWithFS(fs filesystem.FileSystem, baseDir string) ([]string, error) {
    // ...
    if entries, err := os.ReadDir(namedPath); err == nil {  // BUG: ignores fs parameter
        // ...
    }
}
```

**Solution Options:**
1. Add `ReadDir(name string) ([]fs.DirEntry, error)` to FileSystem interface
2. Use a workaround with `Stat()` to check if paths exist

---

### 6.5 Missing Test Coverage

**Status:** PARTIALLY DONE

| Item | Status |
|------|--------|
| `internal/filesystem/filesystem_test.go` | DONE |
| `pkg/errors/errors_test.go` | DONE |
| `cmd/colors.go` tests | NOT DONE |

---

### 6.6 Global Variables in cmd Package

**Status:** NOT DONE (Lower priority)

Global variables still exist in:
- `cmd/root.go:15-26` - version, viper, cfgFile, environment, verbose
- `cmd/send.go:55-67` - requestName, requestIndex, showHeaders, etc.

This makes testing difficult but is a larger refactoring effort.

---

### 6.7 Parser Package Too Large

**Status:** NOT DONE (Lower priority)

`pkg/parser/http_parser.go` is still 904 lines. No splitting was performed.

---

### 6.8 Inconsistent Header Case-Insensitive Lookup

**Status:** NOT DONE

No shared header utility was created. The `getHeaderCaseInsensitive()` pattern is still duplicated in:
- `pkg/parser/http_parser.go` - has the function
- `pkg/client/http_client.go` - manual loop
- `pkg/variables/processor.go` - manual loop
- `pkg/models/request.go` - manual loop
- `pkg/models/response.go` - manual loop

---

## New Issues Discovered

### 7.1 FileSystem Interface Missing ReadDir

**Location:** `internal/filesystem/filesystem.go`

The interface lacks `ReadDir` which is needed by `ListAllSessionsWithFS()`.

**Solution:**
```go
type FileSystem interface {
    // ... existing methods ...
    
    // Directory operations
    ReadDir(name string) ([]fs.DirEntry, error)
}
```

---

### 7.2 Mock FileSystem Doesn't Support ReadDir

**Location:** `internal/filesystem/mock.go`

When `ReadDir` is added to the interface, the mock needs implementation:

```go
func (m *MockFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    if err := m.pathError(name); err != nil {
        return nil, err
    }
    
    var entries []fs.DirEntry
    prefix := name + "/"
    seen := make(map[string]bool)
    
    for path := range m.Files {
        if strings.HasPrefix(path, prefix) {
            rest := path[len(prefix):]
            if idx := strings.Index(rest, "/"); idx >= 0 {
                rest = rest[:idx]
            }
            if !seen[rest] {
                seen[rest] = true
                entries = append(entries, &mockDirEntry{name: rest, isDir: strings.Contains(path[len(prefix):], "/")})
            }
        }
    }
    
    for path := range m.Dirs {
        if strings.HasPrefix(path, prefix) {
            rest := path[len(prefix):]
            if !seen[rest] {
                seen[rest] = true
                entries = append(entries, &mockDirEntry{name: rest, isDir: true})
            }
        }
    }
    
    return entries, nil
}
```

---

### 7.3 Old-Style For Loops (Go 1.22+ Modernization)

**Status:** Could use `range over int`

| File | Line | Current |
|------|------|---------|
| `pkg/variables/processor.go` | varies | `for i := 0; i < len(path); i++` |
| `pkg/output/formatter.go` | varies | `for i := 0; i < len(jsonStr); i++` |
| `pkg/parser/http_parser.go` | varies | `for i := 0; i < len(lines); i++` |

**Note:** Many of these need index manipulation (`i++` or `i--` inside loop), so not all can be converted. Only simple counting loops should use `range`.

---

### 7.4 Test Files Still Use interface{}

**Location:** `pkg/postman/export_test.go`, `pkg/postman/import_test.go`

These test files still use `interface{}` instead of `any`. While not critical, it's inconsistent.

---

### 7.5 Unused fs Parameter in ListAllSessionsWithFS

**Location:** `pkg/session/session.go:397`

The `fs` parameter is passed but never used because `os.ReadDir` is called directly.

---

### 7.6 Missing Context in PreScript Execution (cmd/send.go)

**Location:** `cmd/send.go:253`

```go
result, err := executor.ExecutePreScript(request.Metadata.PreScript, cfg, request, varProcessor)
```

This uses `ExecutePreScript` (which defaults to `context.Background()`) instead of `ExecutePreScriptWithContext`. The pre-script won't be cancelled if the user presses Ctrl+C before the request is sent.

**Solution:**
```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
// ... set up signal handling ...
result, err := executor.ExecutePreScriptWithContext(ctx, request.Metadata.PreScript, cfg, request, varProcessor)
```

---

## Remaining Refactoring Plan

### Phase 1: Quick Fixes (30 min)

1. **Add ReadDir to FileSystem interface**
   - [ ] Add `ReadDir(name string) ([]fs.DirEntry, error)` to interface
   - [ ] Implement in `OSFileSystem`
   - [ ] Implement in `MockFileSystem`
   - [ ] Update `ListAllSessionsWithFS` to use `fs.ReadDir()`

2. **Fix pre-script context propagation**
   - [ ] Update `cmd/send.go` to create context before pre-script execution
   - [ ] Use `ExecutePreScriptWithContext` instead of `ExecutePreScript`

### Phase 2: Consistency (1-2 hours)

3. **Create shared header utility**
   - [ ] Create `internal/httputil/headers.go`
   - [ ] Add `GetHeader()`, `SetHeader()`, `HasHeader()`
   - [ ] Update parser, client, models, variables to use shared utility

4. **Update test files to use constants**
   - [ ] Replace magic strings in test files with constants
   - [ ] This is optional but improves consistency

### Phase 3: Code Quality (2-3 hours) - Lower Priority

5. **Add color helper tests**
   - [ ] Add `cmd/colors_test.go`
   - [ ] Test `useColors()`, `getMethodColor()`, formatting functions

6. **Add printTestPass/printTestFail helpers**
   - [ ] Add helpers to `cmd/colors.go`
   - [ ] Update `printTestResults()` to use them

### Phase 4: Large Refactoring (4+ hours) - Optional

7. **Split parser package** (If desired)
   - [ ] Extract body parsing to `body_parser.go`
   - [ ] Extract metadata to `metadata.go`

8. **Refactor cmd global variables** (If desired)
   - [ ] Create `cmdContext` struct
   - [ ] Pass through command execution

---

## Priority Matrix

| Issue | Impact | Effort | Priority |
|-------|--------|--------|----------|
| Add ReadDir to FileSystem | High | Low | P1 |
| Fix pre-script context | Medium | Low | P1 |
| Create header utility | Medium | Medium | P2 |
| Test file constants | Low | Low | P3 |
| Color helper tests | Low | Medium | P3 |
| Split parser | Low | High | P4 |
| Refactor globals | Medium | High | P4 |

---

## Summary

**Done Well:**
- Mock implementations are comprehensive
- Context propagation is complete (except pre-script)
- FileSystem interface adoption is mostly complete
- Backward compatibility maintained throughout

**Needs Attention:**
- FileSystem interface missing `ReadDir` method
- Pre-script doesn't use context (can't be cancelled)
- No shared header utility (code duplication)

**Lower Priority:**
- Test file consistency (magic strings)
- Parser splitting
- Global variable refactoring

---

## Quick Command Reference

```bash
# Check for remaining issues
rg 'os\.ReadDir' pkg/
rg '"Content-Type"' --type go -l
rg 'interface\{\}' --type go -l
rg 'getHeaderCaseInsensitive|strings\.EqualFold' --type go

# Run tests
go test ./...
go build ./...
```
