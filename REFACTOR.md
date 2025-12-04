# Refactoring Plan for REST Client

This document outlines identified code quality issues, DRY violations, and best practice concerns discovered during a comprehensive codebase analysis.

## Table of Contents

1. [Critical DRY Violations](#critical-dry-violations)
2. [Architectural Issues](#architectural-issues)
3. [Best Practice Violations](#best-practice-violations)
4. [Code Organization Issues](#code-organization-issues)
5. [Refactoring Plan](#refactoring-plan)

---

## Critical DRY Violations

### 1. Duplicate String Truncation Functions

**Locations:**
- `cmd/history.go:377-387` - `truncateString()`
- `cmd/session.go:278-290` - `truncateValue()`
- `pkg/tui/selector.go:459-473` - `truncateString()`
- `cmd/send.go:749-761` - `truncateURL()`

**Issue:** Four nearly identical functions that truncate strings with ellipsis.

**Solution:** Create a single `pkg/utils/strings.go` with:
```go
func Truncate(s string, maxLen int) string
func TruncateMiddle(s string, maxLen int) string  // for URLs
```

---

### 2. Duplicate Home Directory Resolution

**Locations:**
- `pkg/config/config.go:49-62` - `getDefaultConfigDir()`
- `pkg/history/history.go:31-43` - `getDefaultHistoryPath()`
- `pkg/session/session.go:44-56` - `getDefaultSessionDir()`

**Issue:** Same pattern repeated: get home dir, handle error, join path.

**Solution:** Create `pkg/paths/paths.go`:
```go
func HomeDir() (string, error)
func AppDataDir(subdir string) (string, error)
func DefaultConfigPath() string
func DefaultHistoryPath() string
func DefaultSessionDir() string
```

---

### 3. Dual Configuration Systems

**Locations:**
- `cmd/root.go:40-89` - Viper-based config initialization
- `pkg/config/config.go:1-432` - Separate Config struct with its own file loading

**Issue:** Two parallel configuration systems that don't integrate well:
- `root.go` uses viper directly for CLI flags
- `pkg/config` has its own YAML loading, defaults, and validation

**Solution:** Unify into single config system:
1. Keep `pkg/config` as the source of truth
2. Have `root.go` use `pkg/config` instead of direct viper calls
3. Remove duplicate default value definitions

---

### 4. Duplicate Color Decision Logic

**Locations:**
- `cmd/send.go:103` - `useColors()` check
- `cmd/history.go:78,142,215` - `useColors()` checks
- `cmd/session.go:89,160` - `useColors()` checks
- `cmd/env.go:85,156,230` - `useColors()` checks
- `pkg/output/formatter.go:47-55` - `shouldUseColors()`

**Issue:** Color decision logic scattered across files with slightly different implementations.

**Solution:** 
1. Create `pkg/terminal/colors.go` with centralized color decision
2. Pass color config through context or config struct
3. Use a single `ColorConfig` that's initialized once

---

### 5. Duplicate Header Case-Insensitive Lookup

**Locations:**
- `pkg/client/http_client.go:194-200` - Manual loop for header lookup
- `pkg/parser/http_parser.go:678-684` - Similar pattern
- `pkg/variables/processor.go:425-431` - Same pattern again

**Issue:** Go's `http.Header` already has `Get()` which is case-insensitive, but code manually iterates.

**Solution:** Use `http.Header.Get()` or create helper if custom behavior needed:
```go
func GetHeaderValue(headers map[string]string, key string) (string, bool)
```

---

### 6. Duplicate JSON Formatting Logic

**Locations:**
- `cmd/session.go:254-265` - `formatValue()` for display
- `pkg/scripting/context.go:78-89` - Similar JSON pretty-print
- `pkg/output/formatter.go:156-180` - JSON formatting

**Issue:** Multiple places format JSON for display with slight variations.

**Solution:** Create `pkg/format/json.go`:
```go
func PrettyJSON(v interface{}) string
func CompactJSON(v interface{}) string
func FormatForDisplay(v interface{}, maxLen int) string
```

---

### 7. Duplicate File Existence Checks

**Locations:**
- `pkg/config/config.go:167-175` - `fileExists()`
- `pkg/parser/http_parser.go:95-103` - Similar check
- `pkg/session/session.go:178-186` - Same pattern
- `pkg/history/history.go:89-97` - Same pattern

**Issue:** File existence check repeated in multiple packages.

**Solution:** Add to `pkg/paths/paths.go`:
```go
func Exists(path string) bool
func IsFile(path string) bool
func IsDir(path string) bool
```

---

## Architectural Issues

### 1. `send.go` is Too Large (761 lines)

**Issue:** Single file handles:
- Command definition and flags
- Request building
- Variable resolution
- Script execution coordination
- Response handling
- Output formatting
- Interactive mode

**Solution:** Split into:
- `cmd/send.go` - Command definition, flags (~150 lines)
- `cmd/send_execute.go` - Request execution logic (~200 lines)
- `cmd/send_interactive.go` - Interactive mode (~150 lines)
- `cmd/send_output.go` - Output handling (~150 lines)

---

### 2. Global Variables in cmd Package

**Locations:**
- `cmd/root.go:22-38` - Multiple `var` declarations
- `cmd/send.go:25-45` - Flag variables
- `cmd/colors.go:15-25` - Color instances

**Issue:** Global mutable state makes testing difficult and can cause race conditions.

**Solution:**
1. Create `cmdContext` struct to hold command state
2. Pass context through command execution
3. Use closures or receiver methods instead of globals

---

### 3. Missing Interfaces for External Dependencies

**Locations:**
- `pkg/client/http_client.go` - Direct `http.Client` usage
- `pkg/history/history.go` - Direct file system access
- `pkg/session/session.go` - Direct file system access

**Issue:** Hard to test without real HTTP calls or file system.

**Solution:** Define interfaces:
```go
type HTTPDoer interface {
    Do(*http.Request) (*http.Response, error)
}

type FileSystem interface {
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, data []byte, perm os.FileMode) error
    Exists(path string) bool
}
```

---

### 4. Tight Coupling Between Packages

**Issue:** Packages directly depend on concrete implementations:
- `cmd/send.go` directly creates `client.HTTPClient`
- `parser` directly uses `variables.Processor`
- `scripting` directly accesses file system

**Solution:**
1. Define interfaces at package boundaries
2. Use dependency injection
3. Create a `wire.go` or `app.go` for wiring dependencies

---

### 5. Inconsistent Error Handling

**Locations:**
- Some functions return `error` with context: `fmt.Errorf("failed to X: %w", err)`
- Others return raw errors: `return err`
- Some use `log.Fatal()` in library code
- Some silently ignore errors

**Solution:**
1. Create `pkg/errors/errors.go` with custom error types
2. Always wrap errors with context
3. Never use `log.Fatal()` in library code
4. Document which errors can be returned

---

## Best Practice Violations

### 1. Magic Strings Throughout Codebase

**Examples:**
- `"Content-Type"` appears 15+ times
- `"application/json"` appears 10+ times
- `"Authorization"` appears 8+ times
- `".env"` appears 5+ times
- `"Bearer "` appears 4+ times

**Solution:** Create `pkg/constants/constants.go`:
```go
const (
    HeaderContentType   = "Content-Type"
    HeaderAuthorization = "Authorization"
    HeaderAccept        = "Accept"
    
    MIMEApplicationJSON = "application/json"
    MIMEFormURLEncoded  = "application/x-www-form-urlencoded"
    
    AuthSchemeBearer = "Bearer"
    AuthSchemeBasic  = "Basic"
)
```

---

### 2. Inconsistent Naming Conventions

**Examples:**
- `getDefaultConfigDir()` vs `GetConfig()` - unexported vs exported inconsistency
- `HTTPClient` vs `HttpParser` - acronym casing
- `ProcessVariables()` vs `processVariable()` - similar names, different visibility
- `req` vs `request` vs `httpReq` - inconsistent variable naming

**Solution:** Establish naming conventions document and apply consistently:
- Acronyms: `HTTP`, `URL`, `JSON`, `API` (all caps)
- Variables: Use full names for exported, abbreviations OK for local scope
- Consistency within each file

---

### 3. Missing Context Support

**Locations:**
- `pkg/client/http_client.go:Execute()` - No context parameter
- `pkg/scripting/engine.go:Run()` - No context for cancellation
- Most long-running operations

**Issue:** Can't cancel operations, no timeout propagation, no request tracing.

**Solution:** Add `context.Context` as first parameter to:
- `HTTPClient.Execute(ctx context.Context, ...)`
- `ScriptEngine.Run(ctx context.Context, ...)`
- `Parser.Parse(ctx context.Context, ...)`

---

### 4. Lack of Structured Logging

**Current State:** Uses `fmt.Printf` and `log.Printf` inconsistently.

**Solution:** 
1. Add `log/slog` (Go 1.21+) or a logging library
2. Create `pkg/logging/logger.go`
3. Use structured logging with levels

---

### 5. No Input Validation Layer

**Issue:** Validation scattered throughout code, inconsistent error messages.

**Solution:** Create `pkg/validation/validation.go`:
```go
func ValidateURL(url string) error
func ValidateMethod(method string) error
func ValidateHeaders(headers map[string]string) error
func ValidateRequest(req *models.Request) error
```

---

## Code Organization Issues

### 1. Package Responsibilities Unclear

**Current Structure:**
```
pkg/
  client/      # HTTP execution
  config/      # Configuration
  history/     # History storage
  models/      # Data structures
  output/      # Formatting
  parser/      # .http parsing
  postman/     # Postman integration
  scripting/   # JS execution
  session/     # Session management
  tui/         # Terminal UI
  variables/   # Variable processing
```

**Issues:**
- `variables/` and `parser/` have overlapping concerns
- `output/` and `tui/` could be combined
- No clear layering (what depends on what?)

**Proposed Structure:**
```
pkg/
  core/           # Core domain models
    request.go
    response.go
    errors.go
  
  http/           # HTTP layer
    client.go
    auth.go
    transport.go
  
  parser/         # Parsing layer
    http_parser.go
    variable_processor.go
  
  storage/        # Persistence layer
    history.go
    session.go
    config.go
  
  format/         # Output formatting
    json.go
    response.go
    colors.go
  
  ui/             # User interface
    selector.go
    prompt.go
  
  script/         # Scripting
    engine.go
    context.go
  
  postman/        # External integrations
    import.go
    export.go
  
  internal/       # Shared utilities (not exported)
    paths/
    strings/
    constants/
```

---

### 2. Test Organization

**Current:** Tests alongside implementation files.

**Recommendation:** Keep current structure but ensure:
- Every exported function has tests
- Use table-driven tests consistently
- Add integration tests in `test/` directory
- Add test fixtures in `testdata/` directories

---

## Refactoring Plan

### Phase 1: Foundation (Low Risk)

1. **Create utility packages** (1-2 hours)
   - [ ] Create `pkg/internal/strings/truncate.go`
   - [ ] Create `pkg/internal/paths/paths.go`
   - [ ] Create `pkg/internal/constants/constants.go`
   - [ ] Update imports throughout codebase

2. **Unify color handling** (1 hour)
   - [ ] Create `pkg/internal/terminal/colors.go`
   - [ ] Centralize color decision logic
   - [ ] Update all color usage

3. **Add missing tests** (2-3 hours)
   - [ ] Add tests for new utility packages
   - [ ] Improve coverage on existing packages

### Phase 2: Configuration Unification (Medium Risk)

4. **Unify configuration** (2-3 hours)
   - [ ] Audit all viper usage in `cmd/root.go`
   - [ ] Move config logic to `pkg/config`
   - [ ] Create single source of truth for defaults
   - [ ] Update all config access points

### Phase 3: Code Structure (Medium Risk)

5. **Split large files** (2-3 hours)
   - [ ] Split `cmd/send.go` into logical components
   - [ ] Split `pkg/parser/http_parser.go` if beneficial
   - [ ] Ensure each file has single responsibility

6. **Add interfaces** (2-3 hours)
   - [ ] Define `HTTPDoer` interface
   - [ ] Define `FileSystem` interface
   - [ ] Update implementations to use interfaces
   - [ ] Add mock implementations for testing

### Phase 4: Error Handling (Low-Medium Risk)

7. **Standardize error handling** (2 hours)
   - [ ] Create custom error types
   - [ ] Add error wrapping throughout
   - [ ] Remove any `log.Fatal` from library code
   - [ ] Document error contracts

### Phase 5: Advanced Improvements (Higher Risk)

8. **Add context support** (3-4 hours)
   - [ ] Add `context.Context` to main interfaces
   - [ ] Propagate context through call chains
   - [ ] Add timeout support
   - [ ] Add cancellation support

9. **Add structured logging** (2 hours)
   - [ ] Choose logging approach
   - [ ] Add logging throughout
   - [ ] Add debug/verbose modes

10. **Reorganize packages** (4-6 hours)
    - [ ] Plan new package structure
    - [ ] Move files incrementally
    - [ ] Update all imports
    - [ ] Ensure backward compatibility

---

## Priority Matrix

| Issue | Impact | Effort | Priority |
|-------|--------|--------|----------|
| Duplicate truncation functions | Low | Low | P2 |
| Duplicate home dir resolution | Low | Low | P2 |
| Dual config systems | High | Medium | P1 |
| Duplicate color logic | Medium | Low | P2 |
| Magic strings | Medium | Low | P1 |
| `send.go` too large | Medium | Medium | P2 |
| Global variables | Medium | High | P3 |
| Missing interfaces | High | Medium | P1 |
| No context support | High | High | P3 |
| Inconsistent errors | Medium | Medium | P2 |

---

## Quick Wins (Can Do Today)

1. Create `pkg/internal/constants/http.go` with header/MIME constants
2. Replace magic strings with constants (find/replace)
3. Create shared `truncate()` function
4. Create shared `homeDir()` function
5. Add `//go:generate` comments for future code generation

---

## Notes

- All refactoring should maintain backward compatibility
- Each phase should have its own PR for easier review
- Run full test suite after each change
- Consider adding benchmarks before performance-related changes
- Update documentation as code changes
