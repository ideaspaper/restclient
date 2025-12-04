// Package errors provides custom error types and utilities for the REST client.
package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors that can be checked with errors.Is()
var (
	// ErrNotFound indicates a resource was not found.
	ErrNotFound = errors.New("not found")

	// ErrInvalidInput indicates invalid user input.
	ErrInvalidInput = errors.New("invalid input")

	// ErrTimeout indicates an operation timed out.
	ErrTimeout = errors.New("operation timed out")

	// ErrCanceled indicates an operation was canceled.
	ErrCanceled = errors.New("operation canceled")

	// ErrAuth indicates an authentication failure.
	ErrAuth = errors.New("authentication failed")

	// ErrNetwork indicates a network-related error.
	ErrNetwork = errors.New("network error")

	// ErrParse indicates a parsing error.
	ErrParse = errors.New("parse error")

	// ErrScript indicates a script execution error.
	ErrScript = errors.New("script error")

	// ErrConfig indicates a configuration error.
	ErrConfig = errors.New("configuration error")

	// ErrFileSystem indicates a file system error.
	ErrFileSystem = errors.New("file system error")
)

// RequestError represents an error that occurred during HTTP request processing.
type RequestError struct {
	Op      string // Operation that failed (e.g., "send", "build", "parse")
	URL     string // URL of the request, if applicable
	Method  string // HTTP method, if applicable
	Wrapped error  // Underlying error
}

func (e *RequestError) Error() string {
	if e.URL != "" {
		return fmt.Sprintf("%s %s: %s: %v", e.Method, e.URL, e.Op, e.Wrapped)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Wrapped)
}

func (e *RequestError) Unwrap() error {
	return e.Wrapped
}

// NewRequestError creates a new RequestError.
func NewRequestError(op string, err error) *RequestError {
	return &RequestError{Op: op, Wrapped: err}
}

// NewRequestErrorWithURL creates a new RequestError with URL context.
func NewRequestErrorWithURL(op, method, url string, err error) *RequestError {
	return &RequestError{Op: op, Method: method, URL: url, Wrapped: err}
}

// ParseError represents an error that occurred during parsing.
type ParseError struct {
	File    string // File being parsed
	Line    int    // Line number where error occurred (0 if unknown)
	Message string // Error message
	Wrapped error  // Underlying error, if any
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Message)
	}
	if e.File != "" {
		return fmt.Sprintf("%s: %s", e.File, e.Message)
	}
	return e.Message
}

func (e *ParseError) Unwrap() error {
	return e.Wrapped
}

// Is implements errors.Is for ParseError.
func (e *ParseError) Is(target error) bool {
	return target == ErrParse
}

// NewParseError creates a new ParseError.
func NewParseError(file string, line int, message string) *ParseError {
	return &ParseError{File: file, Line: line, Message: message}
}

// NewParseErrorWithCause creates a new ParseError with an underlying cause.
func NewParseErrorWithCause(file string, line int, message string, cause error) *ParseError {
	return &ParseError{File: file, Line: line, Message: message, Wrapped: cause}
}

// ValidationError represents a validation error.
type ValidationError struct {
	Field   string // Field that failed validation
	Value   string // The invalid value (may be redacted for sensitive fields)
	Message string // Description of what's wrong
}

func (e *ValidationError) Error() string {
	if e.Field != "" && e.Value != "" {
		return fmt.Sprintf("invalid %s %q: %s", e.Field, e.Value, e.Message)
	}
	if e.Field != "" {
		return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
	}
	return e.Message
}

// Is implements errors.Is for ValidationError.
func (e *ValidationError) Is(target error) bool {
	return target == ErrInvalidInput
}

// NewValidationError creates a new ValidationError.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

// NewValidationErrorWithValue creates a new ValidationError with the invalid value.
func NewValidationErrorWithValue(field, value, message string) *ValidationError {
	return &ValidationError{Field: field, Value: value, Message: message}
}

// ScriptError represents an error from script execution.
type ScriptError struct {
	Script  string // Script name or phase (e.g., "pre-request", "post-response")
	Message string // Error message
	Wrapped error  // Underlying error
}

func (e *ScriptError) Error() string {
	if e.Script != "" {
		return fmt.Sprintf("script %s: %s", e.Script, e.Message)
	}
	return e.Message
}

func (e *ScriptError) Unwrap() error {
	return e.Wrapped
}

// Is implements errors.Is for ScriptError.
func (e *ScriptError) Is(target error) bool {
	return target == ErrScript
}

// NewScriptError creates a new ScriptError.
func NewScriptError(script, message string) *ScriptError {
	return &ScriptError{Script: script, Message: message}
}

// NewScriptErrorWithCause creates a new ScriptError with an underlying cause.
func NewScriptErrorWithCause(script, message string, cause error) *ScriptError {
	return &ScriptError{Script: script, Message: message, Wrapped: cause}
}

// Wrap wraps an error with a message, using %w for proper error chaining.
// Returns nil if err is nil.
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// Wrapf wraps an error with a formatted message.
// Returns nil if err is nil.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// Is reports whether any error in err's chain matches target.
// This is a convenience re-export of errors.Is.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target.
// This is a convenience re-export of errors.As.
func As(err error, target any) bool {
	return errors.As(err, target)
}
