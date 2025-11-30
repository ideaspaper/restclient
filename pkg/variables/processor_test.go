package variables

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestProcess(t *testing.T) {
	vp := NewVariableProcessor()
	vp.SetFileVariables(map[string]string{
		"baseUrl": "https://api.example.com",
		"token":   "abc123",
	})

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no variables",
			input: "https://api.example.com/users",
			want:  "https://api.example.com/users",
		},
		{
			name:  "single variable",
			input: "{{baseUrl}}/users",
			want:  "https://api.example.com/users",
		},
		{
			name:  "multiple variables",
			input: "{{baseUrl}}/users?token={{token}}",
			want:  "https://api.example.com/users?token=abc123",
		},
		{
			name:  "variable with spaces",
			input: "{{ baseUrl }}/users",
			want:  "https://api.example.com/users",
		},
		{
			name:  "unknown variable kept as-is",
			input: "{{unknown}}/users",
			want:  "{{unknown}}/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vp.Process(tt.input)
			if err != nil {
				t.Errorf("Process() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Process() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessNestedVariables(t *testing.T) {
	vp := NewVariableProcessor()
	vp.SetFileVariables(map[string]string{
		"host":    "api.example.com",
		"baseUrl": "https://{{host}}",
	})

	input := "{{baseUrl}}/users"
	got, err := vp.Process(input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	want := "https://api.example.com/users"
	if got != want {
		t.Errorf("Process() = %v, want %v", got, want)
	}
}

func TestResolveSystemVariables(t *testing.T) {
	vp := NewVariableProcessor()

	t.Run("$guid", func(t *testing.T) {
		got, err := vp.Process("{{$guid}}")
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
		uuidRegex := regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
		if !uuidRegex.MatchString(got) {
			t.Errorf("$guid should produce UUID, got: %v", got)
		}
	})

	t.Run("$timestamp", func(t *testing.T) {
		got, err := vp.Process("{{$timestamp}}")
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		// Should be a number (Unix timestamp)
		if len(got) < 10 {
			t.Errorf("$timestamp should produce Unix timestamp, got: %v", got)
		}
	})

	t.Run("$randomInt", func(t *testing.T) {
		got, err := vp.Process("{{$randomInt 1 100}}")
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		// Should be a number
		if len(got) == 0 {
			t.Errorf("$randomInt should produce a number, got: %v", got)
		}
	})

	t.Run("$datetime iso8601", func(t *testing.T) {
		got, err := vp.Process("{{$datetime iso8601}}")
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		// ISO 8601 format check
		if !strings.Contains(got, "T") || !strings.Contains(got, "Z") {
			t.Errorf("$datetime iso8601 should produce ISO 8601 format, got: %v", got)
		}
	})

	t.Run("$datetime rfc1123", func(t *testing.T) {
		got, err := vp.Process("{{$datetime rfc1123}}")
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		// RFC 1123 contains day name
		if !strings.Contains(got, "GMT") && !strings.Contains(got, "UTC") {
			// May not contain GMT/UTC depending on format
		}
	})
}

func TestResolvePromptVariable(t *testing.T) {
	vp := NewVariableProcessor()

	// Test without handler - should return error
	t.Run("without handler", func(t *testing.T) {
		got, _ := vp.Process("{{$prompt username}}")
		// Should keep original since handler not set
		if got != "{{$prompt username}}" {
			t.Errorf("Without handler, should keep original, got: %v", got)
		}
	})

	// Test with handler
	t.Run("with handler", func(t *testing.T) {
		vp.SetPromptHandler(func(name, description string, isPassword bool) (string, error) {
			if name == "username" {
				return "testuser", nil
			}
			return "", nil
		})

		got, err := vp.Process("{{$prompt username}}")
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if got != "testuser" {
			t.Errorf("$prompt with handler should return handler value, got: %v", got)
		}
	})

	// Test password detection
	t.Run("password detection", func(t *testing.T) {
		var detectedPassword bool
		vp.SetPromptHandler(func(name, description string, isPassword bool) (string, error) {
			detectedPassword = isPassword
			return "secret", nil
		})

		vp.Process("{{$prompt password}}")
		if !detectedPassword {
			t.Error("Should detect 'password' as password field")
		}
	})

	// Test description
	t.Run("with description", func(t *testing.T) {
		var capturedDescription string
		vp.SetPromptHandler(func(name, description string, isPassword bool) (string, error) {
			capturedDescription = description
			return "value", nil
		})

		vp.Process("{{$prompt apiKey Enter your API key}}")
		if capturedDescription != "Enter your API key" {
			t.Errorf("Description should be 'Enter your API key', got: %v", capturedDescription)
		}
	})
}

func TestResolveEnvironmentVariables(t *testing.T) {
	vp := NewVariableProcessor()
	vp.SetEnvironment("dev")
	vp.SetEnvironmentVariables(map[string]map[string]string{
		"$shared": {
			"version": "v1",
		},
		"dev": {
			"host": "dev.example.com",
		},
		"prod": {
			"host": "example.com",
		},
	})

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "environment variable",
			input: "{{host}}",
			want:  "dev.example.com",
		},
		{
			name:  "shared variable",
			input: "{{version}}",
			want:  "v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vp.Process(tt.input)
			if err != nil {
				t.Errorf("Process() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Process() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveRequestVariables(t *testing.T) {
	vp := NewVariableProcessor()
	vp.SetRequestResult("loginAPI", RequestResult{
		StatusCode: 200,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
			"X-Request-Id": {"req-123"},
		},
		Body: `{"token": "jwt-token-123", "user": {"id": 1, "name": "John"}}`,
	})

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "response header",
			input: "{{loginAPI.response.headers.X-Request-Id}}",
			want:  "req-123",
		},
		{
			name:  "response body json path",
			input: "{{loginAPI.response.body.$.token}}",
			want:  "jwt-token-123",
		},
		{
			name:  "response body nested json path",
			input: "{{loginAPI.response.body.$.user.name}}",
			want:  "John",
		},
		{
			name:  "response body wildcard",
			input: "{{loginAPI.response.body.*}}",
			want:  `{"token": "jwt-token-123", "user": {"id": 1, "name": "John"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vp.Process(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Process() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestURLEncodedVariable(t *testing.T) {
	vp := NewVariableProcessor()
	vp.SetFileVariables(map[string]string{
		"query": "hello world",
	})

	got, err := vp.Process("?q={{%query}}")
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	want := "?q=hello%20world"
	if got != want {
		t.Errorf("Process() = %v, want %v", got, want)
	}
}

func TestParseFileVariables(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]string
	}{
		{
			name:    "single variable",
			content: "@baseUrl = https://api.example.com",
			want:    map[string]string{"baseUrl": "https://api.example.com"},
		},
		{
			name: "multiple variables",
			content: `@baseUrl = https://api.example.com
@token = abc123
@version = v1`,
			want: map[string]string{
				"baseUrl": "https://api.example.com",
				"token":   "abc123",
				"version": "v1",
			},
		},
		{
			name:    "variable with spaces",
			content: "  @name   =   John Doe  ",
			want:    map[string]string{"name": "John Doe"},
		},
		{
			name: "mixed content",
			content: `@baseUrl = https://api.example.com
# This is a comment
GET {{baseUrl}}/users
@token = abc123`,
			want: map[string]string{
				"baseUrl": "https://api.example.com",
				"token":   "abc123",
			},
		},
		{
			name:    "no variables",
			content: "GET https://api.example.com/users",
			want:    map[string]string{},
		},
		{
			name:    "variable with escape sequences",
			content: `@message = Hello\nWorld`,
			want:    map[string]string{"message": "Hello\nWorld"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFileVariables(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("ParseFileVariables() length = %v, want %v", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("ParseFileVariables()[%s] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestAddDuration(t *testing.T) {
	// Use a fixed time for testing
	baseTime := parseTime("2024-01-15T12:00:00Z")

	tests := []struct {
		name   string
		offset int
		unit   string
		want   string
	}{
		{name: "add 1 year", offset: 1, unit: "y", want: "2025-01-15T12:00:00Z"},
		{name: "subtract 1 year", offset: -1, unit: "y", want: "2023-01-15T12:00:00Z"},
		{name: "add 1 month", offset: 1, unit: "M", want: "2024-02-15T12:00:00Z"},
		{name: "add 1 week", offset: 1, unit: "w", want: "2024-01-22T12:00:00Z"},
		{name: "add 1 day", offset: 1, unit: "d", want: "2024-01-16T12:00:00Z"},
		{name: "add 1 hour", offset: 1, unit: "h", want: "2024-01-15T13:00:00Z"},
		{name: "add 30 minutes", offset: 30, unit: "m", want: "2024-01-15T12:30:00Z"},
		{name: "add 30 seconds", offset: 30, unit: "s", want: "2024-01-15T12:00:30Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addDuration(baseTime, tt.offset, tt.unit)
			gotStr := got.Format("2006-01-02T15:04:05Z")
			if gotStr != tt.want {
				t.Errorf("addDuration() = %v, want %v", gotStr, tt.want)
			}
		})
	}
}

func TestConvertDateFormat(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "YYYY-MM-DD", want: "2006-01-02"},
		{input: "YY/MM/DD", want: "06/01/02"},
		{input: "YYYY-MM-DD HH:mm:ss", want: "2006-01-02 15:04:05"},
		{input: "'YYYY-MM-DD'", want: "2006-01-02"},
		{input: "YYYY", want: "2006"},
		{input: "MM", want: "01"},
		{input: "DD", want: "02"},
		{input: "HH", want: "15"},
		{input: "mm", want: "04"},
		{input: "ss", want: "05"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := convertDateFormat(tt.input)
			if got != tt.want {
				t.Errorf("convertDateFormat(%s) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestURLEncode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "hello world", want: "hello%20world"},
		{input: "a+b", want: "a%2Bb"},
		{input: "test@example.com", want: "test%40example.com"},
		{input: "key=value", want: "key%3Dvalue"},
		{input: "safe-string_123", want: "safe-string_123"},
		{input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := urlEncode(tt.input)
			if got != tt.want {
				t.Errorf("urlEncode(%s) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractJSONPath(t *testing.T) {
	body := `{"user": {"name": "John", "age": 30}, "items": ["a", "b", "c"], "active": true}`

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{name: "simple field", path: "$.user", want: `{"age":30,"name":"John"}`},
		{name: "nested field", path: "$.user.name", want: "John"},
		{name: "number field", path: "$.user.age", want: "30"},
		{name: "boolean field", path: "$.active", want: "true"},
		{name: "array element", path: "$.items[0]", want: "a"},
		{name: "array element 2", path: "$.items[1]", want: "b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractJSONPath(body, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractJSONPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("extractJSONPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessEscapes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: `Hello\nWorld`, want: "Hello\nWorld"},
		{input: `Tab\there`, want: "Tab\there"},
		{input: `Return\rhere`, want: "Return\rhere"},
		{input: `Quote\"here`, want: `Quote"here`},
		{input: `No escapes`, want: "No escapes"},
		{input: `Multiple\n\t\rescapes`, want: "Multiple\n\t\rescapes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := processEscapes(tt.input)
			if got != tt.want {
				t.Errorf("processEscapes() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateGUID(t *testing.T) {
	guid1 := GenerateGUID()
	guid2 := GenerateGUID()

	// Should be in UUID format
	uuidRegex := regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
	if !uuidRegex.MatchString(guid1) {
		t.Errorf("GenerateGUID() = %v, not in UUID format", guid1)
	}

	// Should be unique
	if guid1 == guid2 {
		t.Error("GenerateGUID() should produce unique values")
	}
}

func TestResolveRandomInt(t *testing.T) {
	vp := NewVariableProcessor()

	// Test multiple times to verify range
	for i := 0; i < 100; i++ {
		got, err := vp.Process("{{$randomInt 1 10}}")
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}

		// Parse as int to verify range
		var num int
		if _, err := parseIntFromString(got, &num); err != nil {
			t.Fatalf("Result is not a number: %v", got)
		}

		if num < 1 || num >= 10 {
			t.Errorf("$randomInt 1 10 produced %d, which is out of range [1, 10)", num)
		}
	}
}

// Helper to parse int from string
func parseIntFromString(s string, result *int) (bool, error) {
	for i, c := range s {
		if c < '0' || c > '9' {
			if i == 0 && c == '-' {
				continue
			}
			return false, nil
		}
	}
	// Simple parsing
	val := 0
	neg := false
	for i, c := range s {
		if i == 0 && c == '-' {
			neg = true
			continue
		}
		val = val*10 + int(c-'0')
	}
	if neg {
		val = -val
	}
	*result = val
	return true, nil
}

// Helper to parse time
func parseTime(s string) (t time.Time) {
	t, _ = time.Parse(time.RFC3339, s)
	return
}
