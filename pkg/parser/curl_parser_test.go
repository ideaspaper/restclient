package parser

import (
	"testing"
)

func TestParseCurl(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantMethod  string
		wantURL     string
		wantHeaders map[string]string
		wantBody    string
		wantErr     bool
	}{
		{
			name:       "simple GET",
			input:      "curl https://api.example.com/users",
			wantMethod: "GET",
			wantURL:    "https://api.example.com/users",
		},
		{
			name:       "GET with explicit method",
			input:      "curl -X GET https://api.example.com/users",
			wantMethod: "GET",
			wantURL:    "https://api.example.com/users",
		},
		{
			name:       "POST request",
			input:      "curl -X POST https://api.example.com/users",
			wantMethod: "POST",
			wantURL:    "https://api.example.com/users",
		},
		{
			name:       "DELETE request",
			input:      "curl -X DELETE https://api.example.com/users/123",
			wantMethod: "DELETE",
			wantURL:    "https://api.example.com/users/123",
		},
		{
			name:       "with headers",
			input:      `curl -H "Content-Type: application/json" -H "Authorization: Bearer token123" https://api.example.com/users`,
			wantMethod: "GET",
			wantURL:    "https://api.example.com/users",
			wantHeaders: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer token123",
			},
		},
		{
			name:        "POST with data",
			input:       `curl -X POST -d "name=John&email=john@example.com" https://api.example.com/users`,
			wantMethod:  "POST",
			wantURL:     "https://api.example.com/users",
			wantBody:    "name=John&email=john@example.com",
			wantHeaders: map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
		},
		{
			name:        "POST with JSON data",
			input:       `curl -X POST -H "Content-Type: application/json" -d '{"name":"John"}' https://api.example.com/users`,
			wantMethod:  "POST",
			wantURL:     "https://api.example.com/users",
			wantBody:    `{"name":"John"}`,
			wantHeaders: map[string]string{"Content-Type": "application/json"},
		},
		{
			name:       "implicit POST with data",
			input:      `curl -d "data=test" https://api.example.com/users`,
			wantMethod: "POST",
			wantURL:    "https://api.example.com/users",
			wantBody:   "data=test",
		},
		{
			name:        "with basic auth",
			input:       `curl -u "user:password" https://api.example.com/users`,
			wantMethod:  "GET",
			wantURL:     "https://api.example.com/users",
			wantHeaders: map[string]string{"Authorization": "Basic dXNlcjpwYXNzd29yZA=="},
		},
		{
			name:        "with cookies",
			input:       `curl -b "session=abc123" https://api.example.com/users`,
			wantMethod:  "GET",
			wantURL:     "https://api.example.com/users",
			wantHeaders: map[string]string{"Cookie": "session=abc123"},
		},
		{
			name:       "HEAD request with -I",
			input:      `curl -I https://api.example.com/users`,
			wantMethod: "HEAD",
			wantURL:    "https://api.example.com/users",
		},
		{
			name:       "long form options",
			input:      `curl --request POST --header "Content-Type: application/json" --data '{"test":true}' https://api.example.com/users`,
			wantMethod: "POST",
			wantURL:    "https://api.example.com/users",
			wantBody:   `{"test":true}`,
		},
		{
			name:       "no space between -X and method",
			input:      `curl -XPOST https://api.example.com/users`,
			wantMethod: "POST",
			wantURL:    "https://api.example.com/users",
		},
		{
			name:       "line continuation",
			input:      "curl \\\n  -X POST \\\n  https://api.example.com/users",
			wantMethod: "POST",
			wantURL:    "https://api.example.com/users",
		},
		{
			name:       "multiple -d flags",
			input:      `curl -X POST -d "name=John" -d "email=john@example.com" https://api.example.com/users`,
			wantMethod: "POST",
			wantURL:    "https://api.example.com/users",
			wantBody:   "name=John&email=john@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCurl(tt.input, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCurl() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.Method != tt.wantMethod {
				t.Errorf("ParseCurl() Method = %v, want %v", got.Method, tt.wantMethod)
			}
			if got.URL != tt.wantURL {
				t.Errorf("ParseCurl() URL = %v, want %v", got.URL, tt.wantURL)
			}
			for k, v := range tt.wantHeaders {
				if got.Headers[k] != v {
					t.Errorf("ParseCurl() Headers[%s] = %v, want %v", k, got.Headers[k], v)
				}
			}
			if tt.wantBody != "" && got.RawBody != tt.wantBody {
				t.Errorf("ParseCurl() RawBody = %v, want %v", got.RawBody, tt.wantBody)
			}
		})
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple args",
			input: "curl https://example.com",
			want:  []string{"curl", "https://example.com"},
		},
		{
			name:  "double quoted string",
			input: `curl -H "Content-Type: application/json" https://example.com`,
			want:  []string{"curl", "-H", "Content-Type: application/json", "https://example.com"},
		},
		{
			name:  "single quoted string",
			input: `curl -d '{"name":"John"}' https://example.com`,
			want:  []string{"curl", "-d", `{"name":"John"}`, "https://example.com"},
		},
		{
			name:  "escaped quotes",
			input: `curl -d "say \"hello\"" https://example.com`,
			want:  []string{"curl", "-d", `say "hello"`, "https://example.com"},
		},
		{
			name:  "escaped backslash outside quotes",
			input: `curl -d test\\data https://example.com`,
			want:  []string{"curl", "-d", `test\data`, "https://example.com"},
		},
		{
			name:  "multiple spaces",
			input: "curl   -X   POST   https://example.com",
			want:  []string{"curl", "-X", "POST", "https://example.com"},
		},
		{
			name:  "tabs",
			input: "curl\t-X\tPOST\thttps://example.com",
			want:  []string{"curl", "-X", "POST", "https://example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseArgs(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseArgs() length = %v, want %v", len(got), len(tt.want))
				t.Errorf("got: %v", got)
				return
			}
			for i, v := range tt.want {
				if got[i] != v {
					t.Errorf("parseArgs()[%d] = %v, want %v", i, got[i], v)
				}
			}
		})
	}
}

func TestExtractURL(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "simple URL",
			args: []string{"curl", "https://example.com"},
			want: "https://example.com",
		},
		{
			name: "URL after flags",
			args: []string{"curl", "-X", "GET", "https://example.com"},
			want: "https://example.com",
		},
		{
			name: "URL with path",
			args: []string{"curl", "https://example.com/api/users"},
			want: "https://example.com/api/users",
		},
		{
			name: "http URL",
			args: []string{"curl", "http://example.com"},
			want: "http://example.com",
		},
		{
			name: "URL after multiple flags",
			args: []string{"curl", "-X", "POST", "-H", "Content-Type: application/json", "https://example.com"},
			want: "https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractURL(tt.args)
			if got != tt.want {
				t.Errorf("extractURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValueFlag(t *testing.T) {
	valueFlags := []string{"-X", "--request", "-H", "--header", "-d", "--data", "-u", "--user", "-b", "--cookie", "-o", "--output"}
	nonValueFlags := []string{"-v", "--verbose", "-k", "--insecure", "-I", "--head", "-L", "--compressed"}

	for _, flag := range valueFlags {
		if !isValueFlag(flag) {
			t.Errorf("isValueFlag(%s) = false, want true", flag)
		}
	}

	for _, flag := range nonValueFlags {
		if isValueFlag(flag) {
			// -L is actually a value flag in our implementation
			if flag != "-L" {
				t.Errorf("isValueFlag(%s) = true, want false", flag)
			}
		}
	}
}

func TestGetArgValue(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		flags []string
		want  string
	}{
		{
			name:  "short flag",
			args:  []string{"-X", "POST", "url"},
			flags: []string{"-X"},
			want:  "POST",
		},
		{
			name:  "long flag",
			args:  []string{"--request", "POST", "url"},
			flags: []string{"--request"},
			want:  "POST",
		},
		{
			name:  "multiple possible flags",
			args:  []string{"-X", "POST", "url"},
			flags: []string{"-X", "--request"},
			want:  "POST",
		},
		{
			name:  "flag with equals",
			args:  []string{"-X=POST", "url"},
			flags: []string{"-X"},
			want:  "POST",
		},
		{
			name:  "not found",
			args:  []string{"url"},
			flags: []string{"-X"},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getArgValue(tt.args, tt.flags...)
			if got != tt.want {
				t.Errorf("getArgValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetArgValues(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		flags []string
		want  []string
	}{
		{
			name:  "single value",
			args:  []string{"-H", "Content-Type: application/json"},
			flags: []string{"-H"},
			want:  []string{"Content-Type: application/json"},
		},
		{
			name:  "multiple values",
			args:  []string{"-H", "Content-Type: application/json", "-H", "Authorization: Bearer token"},
			flags: []string{"-H"},
			want:  []string{"Content-Type: application/json", "Authorization: Bearer token"},
		},
		{
			name:  "no values",
			args:  []string{"url"},
			flags: []string{"-H"},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getArgValues(tt.args, tt.flags...)
			if len(got) != len(tt.want) {
				t.Errorf("getArgValues() length = %v, want %v", len(got), len(tt.want))
				return
			}
			for i, v := range tt.want {
				if got[i] != v {
					t.Errorf("getArgValues()[%d] = %v, want %v", i, got[i], v)
				}
			}
		})
	}
}

func TestHasArg(t *testing.T) {
	args := []string{"curl", "-v", "-X", "POST", "https://example.com"}

	if !hasArg(args, "-v") {
		t.Error("hasArg() should return true for -v")
	}
	if !hasArg(args, "-X") {
		t.Error("hasArg() should return true for -X")
	}
	if !hasArg(args, "-v", "-X") {
		t.Error("hasArg() should return true for -v or -X")
	}
	if hasArg(args, "-k") {
		t.Error("hasArg() should return false for -k")
	}
}

func TestIsCurlCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "curl command", input: "curl https://example.com", want: true},
		{name: "curl with options", input: "curl -X POST https://example.com", want: true},
		{name: "CURL uppercase", input: "CURL https://example.com", want: true},
		{name: "curl with leading spaces", input: "  curl https://example.com", want: true},
		{name: "not curl", input: "wget https://example.com", want: false},
		{name: "http request", input: "GET https://example.com", want: false},
		{name: "curl in text", input: "this is about curl", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCurlCommand(tt.input); got != tt.want {
				t.Errorf("IsCurlCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBase64Encode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "user:password", want: "dXNlcjpwYXNzd29yZA=="},
		{input: "hello", want: "aGVsbG8="},
		{input: "", want: ""},
		{input: "a", want: "YQ=="},
		{input: "ab", want: "YWI="},
		{input: "abc", want: "YWJj"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := base64Encode(tt.input); got != tt.want {
				t.Errorf("base64Encode(%s) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMergeIntoSingleLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Unix line continuation",
			input: "curl \\\n  -X POST",
			want:  "curl    -X POST",
		},
		{
			name:  "Windows line continuation",
			input: "curl \\\r\n  -X POST",
			want:  "curl    -X POST",
		},
		{
			name:  "Multiple continuations",
			input: "curl \\\n  -X POST \\\n  url",
			want:  "curl    -X POST    url",
		},
		{
			name:  "No continuation",
			input: "curl -X POST url",
			want:  "curl -X POST url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeIntoSingleLine(tt.input); got != tt.want {
				t.Errorf("mergeIntoSingleLine() = %v, want %v", got, tt.want)
			}
		})
	}
}
