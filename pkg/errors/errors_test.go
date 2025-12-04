package errors

import (
	"errors"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", ErrNotFound},
		{"ErrInvalidInput", ErrInvalidInput},
		{"ErrTimeout", ErrTimeout},
		{"ErrCanceled", ErrCanceled},
		{"ErrAuth", ErrAuth},
		{"ErrNetwork", ErrNetwork},
		{"ErrParse", ErrParse},
		{"ErrScript", ErrScript},
		{"ErrConfig", ErrConfig},
		{"ErrFileSystem", ErrFileSystem},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s should not be nil", tt.name)
			}
			if tt.err.Error() == "" {
				t.Errorf("%s.Error() should not be empty", tt.name)
			}
		})
	}
}

func TestRequestError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := NewRequestError("send", errors.New("connection failed"))
		if err.Op != "send" {
			t.Errorf("expected Op 'send', got %q", err.Op)
		}
		expected := "send: connection failed"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("error with URL", func(t *testing.T) {
		err := NewRequestErrorWithURL("send", "GET", "https://api.example.com", errors.New("timeout"))
		if err.Method != "GET" {
			t.Errorf("expected Method 'GET', got %q", err.Method)
		}
		if err.URL != "https://api.example.com" {
			t.Errorf("expected URL 'https://api.example.com', got %q", err.URL)
		}
		expected := "GET https://api.example.com: send: timeout"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("unwrap", func(t *testing.T) {
		cause := errors.New("root cause")
		err := NewRequestError("build", cause)
		if err.Unwrap() != cause {
			t.Error("Unwrap should return the wrapped error")
		}
	})
}

func TestParseError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := NewParseError("api.http", 10, "invalid header")
		if err.File != "api.http" {
			t.Errorf("expected File 'api.http', got %q", err.File)
		}
		if err.Line != 10 {
			t.Errorf("expected Line 10, got %d", err.Line)
		}
		expected := "api.http:10: invalid header"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("error without line", func(t *testing.T) {
		err := NewParseError("api.http", 0, "no requests found")
		expected := "api.http: no requests found"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("error without file", func(t *testing.T) {
		err := NewParseError("", 0, "syntax error")
		expected := "syntax error"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("error with cause", func(t *testing.T) {
		cause := errors.New("EOF")
		err := NewParseErrorWithCause("api.http", 5, "unexpected end of file", cause)
		if err.Unwrap() != cause {
			t.Error("Unwrap should return the wrapped error")
		}
	})

	t.Run("Is ErrParse", func(t *testing.T) {
		err := NewParseError("api.http", 1, "test")
		if !errors.Is(err, ErrParse) {
			t.Error("ParseError should match ErrParse with errors.Is")
		}
	})
}

func TestValidationError(t *testing.T) {
	t.Run("field only", func(t *testing.T) {
		err := NewValidationError("url", "cannot be empty")
		expected := "invalid url: cannot be empty"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("field with value", func(t *testing.T) {
		err := NewValidationErrorWithValue("method", "INVALID", "must be a valid HTTP method")
		expected := `invalid method "INVALID": must be a valid HTTP method`
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("message only", func(t *testing.T) {
		err := &ValidationError{Message: "validation failed"}
		expected := "validation failed"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("Is ErrInvalidInput", func(t *testing.T) {
		err := NewValidationError("field", "test")
		if !errors.Is(err, ErrInvalidInput) {
			t.Error("ValidationError should match ErrInvalidInput with errors.Is")
		}
	})
}

func TestScriptError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := NewScriptError("pre-request", "syntax error at line 5")
		expected := "script pre-request: syntax error at line 5"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("error without script name", func(t *testing.T) {
		err := &ScriptError{Message: "execution failed"}
		expected := "execution failed"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("error with cause", func(t *testing.T) {
		cause := errors.New("undefined variable")
		err := NewScriptErrorWithCause("post-response", "runtime error", cause)
		if err.Unwrap() != cause {
			t.Error("Unwrap should return the wrapped error")
		}
	})

	t.Run("Is ErrScript", func(t *testing.T) {
		err := NewScriptError("test", "test")
		if !errors.Is(err, ErrScript) {
			t.Error("ScriptError should match ErrScript with errors.Is")
		}
	})
}

func TestWrap(t *testing.T) {
	t.Run("wraps error", func(t *testing.T) {
		cause := errors.New("file not found")
		err := Wrap(cause, "failed to load config")
		expected := "failed to load config: file not found"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
		if !errors.Is(err, cause) {
			t.Error("wrapped error should match cause with errors.Is")
		}
	})

	t.Run("nil error returns nil", func(t *testing.T) {
		if Wrap(nil, "message") != nil {
			t.Error("Wrap(nil, ...) should return nil")
		}
	})
}

func TestWrapf(t *testing.T) {
	t.Run("wraps with format", func(t *testing.T) {
		cause := errors.New("permission denied")
		err := Wrapf(cause, "failed to read %s", "config.json")
		expected := "failed to read config.json: permission denied"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
		if !errors.Is(err, cause) {
			t.Error("wrapped error should match cause with errors.Is")
		}
	})

	t.Run("nil error returns nil", func(t *testing.T) {
		if Wrapf(nil, "message %s", "arg") != nil {
			t.Error("Wrapf(nil, ...) should return nil")
		}
	})
}

func TestIs(t *testing.T) {
	err := Wrap(ErrNotFound, "resource lookup failed")
	if !Is(err, ErrNotFound) {
		t.Error("Is should find wrapped sentinel error")
	}
	if Is(err, ErrTimeout) {
		t.Error("Is should not match different sentinel error")
	}
}

func TestAs(t *testing.T) {
	parseErr := NewParseError("test.http", 5, "invalid syntax")
	err := Wrap(parseErr, "parsing failed")

	var target *ParseError
	if !As(err, &target) {
		t.Error("As should find wrapped ParseError")
	}
	if target.Line != 5 {
		t.Errorf("expected Line 5, got %d", target.Line)
	}
}
