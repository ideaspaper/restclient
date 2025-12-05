package userinput

import (
	"testing"

	"github.com/ideaspaper/restclient/internal/filesystem"
	"github.com/ideaspaper/restclient/pkg/session"
)

func TestNewPrompter(t *testing.T) {
	tempDir := t.TempDir()
	sm, _ := session.NewSessionManager(tempDir, "", "test")

	p := NewPrompter(sm, false, true)

	if p == nil {
		t.Fatal("NewPrompter() returned nil")
	}
	if p.session != sm {
		t.Error("Prompter session not set correctly")
	}
	if p.forcePrompt != false {
		t.Error("Prompter forcePrompt not set correctly")
	}
	if p.useColors != true {
		t.Error("Prompter useColors not set correctly")
	}
	if p.detector == nil {
		t.Error("Prompter detector should not be nil")
	}
}

func TestPrompter_HasPatterns(t *testing.T) {
	p := NewPrompter(nil, false, false)

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "URL with patterns",
			url:  "https://api.example.com/posts/{{:id}}",
			want: true,
		},
		{
			name: "URL without patterns",
			url:  "https://api.example.com/posts/123",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.HasPatterns(tt.url)
			if got != tt.want {
				t.Errorf("HasPatterns() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrompter_GenerateKey(t *testing.T) {
	p := NewPrompter(nil, false, false)

	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "simple URL",
			url:  "https://api.example.com/posts/{{:id}}",
			want: "api.example.com/posts/{{:id}}",
		},
		{
			name: "URL with query",
			url:  "https://api.example.com/posts?page={{:page}}",
			want: "api.example.com/posts?page={{:page}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.GenerateKey(tt.url)
			if got != tt.want {
				t.Errorf("GenerateKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrompter_ProcessURL_NoPatterns(t *testing.T) {
	p := NewPrompter(nil, false, false)

	url := "https://api.example.com/posts/123"
	result, err := p.ProcessURL(url)

	if err != nil {
		t.Errorf("ProcessURL() error = %v", err)
	}
	if result.URL != url {
		t.Errorf("ProcessURL() URL = %v, want %v", result.URL, url)
	}
	if result.Prompted {
		t.Error("ProcessURL() Prompted should be false for no patterns")
	}
	if len(result.Patterns) != 0 {
		t.Errorf("ProcessURL() Patterns = %v, want empty", result.Patterns)
	}
}

func TestPrompter_ProcessURL_WithStoredValues(t *testing.T) {
	tempDir := t.TempDir()

	// Create a mock file system
	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Pre-populate session with values
	urlKey := "api.example.com/posts/{{:id}}"
	sm.SetUserInputs(urlKey, map[string]string{"id": "42"})

	p := NewPrompter(sm, false, false)

	url := "https://api.example.com/posts/{{:id}}"
	result, err := p.ProcessURL(url)

	if err != nil {
		t.Errorf("ProcessURL() error = %v", err)
	}
	if result.URL != "https://api.example.com/posts/42" {
		t.Errorf("ProcessURL() URL = %v, want %v", result.URL, "https://api.example.com/posts/42")
	}
	if result.Prompted {
		t.Error("ProcessURL() Prompted should be false when using stored values")
	}
	if len(result.Patterns) != 1 {
		t.Errorf("ProcessURL() Patterns length = %v, want 1", len(result.Patterns))
	}
	if result.Values["id"] != "42" {
		t.Errorf("ProcessURL() Values[id] = %v, want 42", result.Values["id"])
	}
}

func TestPrompter_ProcessURL_WithMultipleStoredValues(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Pre-populate session with values
	urlKey := "api.example.com/users/{{:userId}}/posts/{{:postId}}"
	sm.SetUserInputs(urlKey, map[string]string{"userId": "1", "postId": "99"})

	p := NewPrompter(sm, false, false)

	url := "https://api.example.com/users/{{:userId}}/posts/{{:postId}}"
	result, err := p.ProcessURL(url)

	if err != nil {
		t.Errorf("ProcessURL() error = %v", err)
	}
	if result.URL != "https://api.example.com/users/1/posts/99" {
		t.Errorf("ProcessURL() URL = %v, want %v", result.URL, "https://api.example.com/users/1/posts/99")
	}
	if result.Prompted {
		t.Error("ProcessURL() Prompted should be false when using stored values")
	}
	if len(result.Patterns) != 2 {
		t.Errorf("ProcessURL() Patterns length = %v, want 2", len(result.Patterns))
	}
}

func TestPrompter_ProcessContent_NoPatterns(t *testing.T) {
	p := NewPrompter(nil, false, false)

	content := "Hello World"
	got, err := p.ProcessContent(content, "test-key")

	if err != nil {
		t.Errorf("ProcessContent() error = %v", err)
	}
	if got != content {
		t.Errorf("ProcessContent() = %v, want %v", got, content)
	}
}

func TestPrompter_ProcessContent_WithStoredValues(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Pre-populate session with values
	urlKey := "test-key"
	sm.SetUserInputs(urlKey, map[string]string{"name": "John"})

	p := NewPrompter(sm, false, false)

	content := "Hello {{:name}}!"
	got, err := p.ProcessContent(content, urlKey)

	if err != nil {
		t.Errorf("ProcessContent() error = %v", err)
	}
	if got != "Hello John!" {
		t.Errorf("ProcessContent() = %v, want %v", got, "Hello John!")
	}
}

func TestPrompter_NilSession(t *testing.T) {
	// Test that prompter works without a session (should fall through to prompting)
	p := NewPrompter(nil, false, false)

	// URL without patterns should work fine
	url := "https://api.example.com/posts/123"
	result, err := p.ProcessURL(url)

	if err != nil {
		t.Errorf("ProcessURL() error = %v", err)
	}
	if result.URL != url {
		t.Errorf("ProcessURL() URL = %v, want %v", result.URL, url)
	}
}

func TestProcessResult_PatternsOrder(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Pre-populate session with values
	urlKey := "api.example.com/users/{{:userId}}/posts/{{:postId}}?page={{:page}}"
	sm.SetUserInputs(urlKey, map[string]string{"userId": "1", "postId": "99", "page": "5"})

	p := NewPrompter(sm, false, false)

	url := "https://api.example.com/users/{{:userId}}/posts/{{:postId}}?page={{:page}}"
	result, err := p.ProcessURL(url)

	if err != nil {
		t.Fatalf("ProcessURL() error = %v", err)
	}

	// Verify patterns are in order of appearance
	if len(result.Patterns) != 3 {
		t.Fatalf("ProcessURL() Patterns length = %v, want 3", len(result.Patterns))
	}
	if result.Patterns[0].Name != "userId" {
		t.Errorf("ProcessURL() Patterns[0].Name = %v, want userId", result.Patterns[0].Name)
	}
	if result.Patterns[1].Name != "postId" {
		t.Errorf("ProcessURL() Patterns[1].Name = %v, want postId", result.Patterns[1].Name)
	}
	if result.Patterns[2].Name != "page" {
		t.Errorf("ProcessURL() Patterns[2].Name = %v, want page", result.Patterns[2].Name)
	}
}

func TestProcessResult_ValuesMap(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	urlKey := "api.example.com/posts/{{:id}}?limit={{:limit}}"
	sm.SetUserInputs(urlKey, map[string]string{"id": "123", "limit": "10"})

	p := NewPrompter(sm, false, false)

	url := "https://api.example.com/posts/{{:id}}?limit={{:limit}}"
	result, err := p.ProcessURL(url)

	if err != nil {
		t.Fatalf("ProcessURL() error = %v", err)
	}

	// Verify values map contains all values
	if len(result.Values) != 2 {
		t.Errorf("ProcessURL() Values length = %v, want 2", len(result.Values))
	}
	if result.Values["id"] != "123" {
		t.Errorf("ProcessURL() Values[id] = %v, want 123", result.Values["id"])
	}
	if result.Values["limit"] != "10" {
		t.Errorf("ProcessURL() Values[limit] = %v, want 10", result.Values["limit"])
	}
}

func TestProcessResult_PromptedFlag(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	p := NewPrompter(sm, false, false)

	// Test 1: No patterns - Prompted should be false
	url1 := "https://api.example.com/posts/123"
	result1, _ := p.ProcessURL(url1)
	if result1.Prompted {
		t.Error("Prompted should be false when no patterns exist")
	}

	// Test 2: Patterns with stored values - Prompted should be false
	urlKey := "api.example.com/posts/{{:id}}"
	sm.SetUserInputs(urlKey, map[string]string{"id": "42"})
	url2 := "https://api.example.com/posts/{{:id}}"
	result2, _ := p.ProcessURL(url2)
	if result2.Prompted {
		t.Error("Prompted should be false when using stored values")
	}
}

func TestProcessResult_NilValuesWhenNoPatterns(t *testing.T) {
	p := NewPrompter(nil, false, false)

	url := "https://api.example.com/posts/123"
	result, err := p.ProcessURL(url)

	if err != nil {
		t.Fatalf("ProcessURL() error = %v", err)
	}

	if result.Values != nil {
		t.Errorf("ProcessURL() Values should be nil when no patterns, got %v", result.Values)
	}
	if result.Patterns != nil {
		t.Errorf("ProcessURL() Patterns should be nil when no patterns, got %v", result.Patterns)
	}
}

func TestProcessResult_URLCorrectlyProcessed(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	tests := []struct {
		name      string
		urlKey    string
		values    map[string]string
		inputURL  string
		wantURL   string
		wantCount int
	}{
		{
			name:      "single path parameter",
			urlKey:    "api.example.com/posts/{{:id}}",
			values:    map[string]string{"id": "42"},
			inputURL:  "https://api.example.com/posts/{{:id}}",
			wantURL:   "https://api.example.com/posts/42",
			wantCount: 1,
		},
		{
			name:      "multiple path parameters",
			urlKey:    "api.example.com/users/{{:userId}}/posts/{{:postId}}",
			values:    map[string]string{"userId": "1", "postId": "99"},
			inputURL:  "https://api.example.com/users/{{:userId}}/posts/{{:postId}}",
			wantURL:   "https://api.example.com/users/1/posts/99",
			wantCount: 2,
		},
		{
			name:      "query parameters",
			urlKey:    "api.example.com/posts?page={{:page}}&limit={{:limit}}",
			values:    map[string]string{"page": "1", "limit": "10"},
			inputURL:  "https://api.example.com/posts?page={{:page}}&limit={{:limit}}",
			wantURL:   "https://api.example.com/posts?page=1&limit=10",
			wantCount: 2,
		},
		{
			name:      "mixed path and query",
			urlKey:    "api.example.com/posts/{{:id}}?format={{:format}}",
			values:    map[string]string{"id": "123", "format": "json"},
			inputURL:  "https://api.example.com/posts/{{:id}}?format={{:format}}",
			wantURL:   "https://api.example.com/posts/123?format=json",
			wantCount: 2,
		},
		{
			name:      "duplicate parameter name",
			urlKey:    "api.example.com/posts/{{:id}}/related/{{:id}}",
			values:    map[string]string{"id": "42"},
			inputURL:  "https://api.example.com/posts/{{:id}}/related/{{:id}}",
			wantURL:   "https://api.example.com/posts/42/related/42",
			wantCount: 1, // Unique patterns only
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm.SetUserInputs(tt.urlKey, tt.values)
			p := NewPrompter(sm, false, false)

			result, err := p.ProcessURL(tt.inputURL)
			if err != nil {
				t.Fatalf("ProcessURL() error = %v", err)
			}

			if result.URL != tt.wantURL {
				t.Errorf("ProcessURL() URL = %v, want %v", result.URL, tt.wantURL)
			}

			if len(result.Patterns) != tt.wantCount {
				t.Errorf("ProcessURL() Patterns count = %v, want %v", len(result.Patterns), tt.wantCount)
			}
		})
	}
}

func TestProcessResult_EmptyValues(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Test with empty string value
	urlKey := "api.example.com/posts/{{:id}}"
	sm.SetUserInputs(urlKey, map[string]string{"id": ""})

	p := NewPrompter(sm, false, false)

	url := "https://api.example.com/posts/{{:id}}"
	result, err := p.ProcessURL(url)

	if err != nil {
		t.Fatalf("ProcessURL() error = %v", err)
	}

	// Empty value should still be used
	if result.URL != "https://api.example.com/posts/" {
		t.Errorf("ProcessURL() URL = %v, want %v", result.URL, "https://api.example.com/posts/")
	}
	if result.Values["id"] != "" {
		t.Errorf("ProcessURL() Values[id] = %v, want empty string", result.Values["id"])
	}
}

func TestPrompter_ProcessContent_MultipartFieldValues(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Pre-populate session with values
	urlKey := "api.example.com/upload"
	sm.SetUserInputs(urlKey, map[string]string{
		"username":    "john_doe",
		"description": "My file",
		"tag":         "important",
	})

	p := NewPrompter(sm, false, false)

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "simple field value",
			content: "{{:username}}",
			want:    "john_doe",
		},
		{
			name:    "field value with surrounding text",
			content: "Uploaded by {{:username}}",
			want:    "Uploaded by john_doe",
		},
		{
			name:    "multiple patterns in field value",
			content: "{{:description}} - tagged as {{:tag}}",
			want:    "My file - tagged as important",
		},
		{
			name:    "field value without patterns",
			content: "static value",
			want:    "static value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.ProcessContent(tt.content, urlKey)
			if err != nil {
				t.Errorf("ProcessContent() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ProcessContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrompter_ProcessContent_SharedValuesAcrossFields(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Pre-populate session with values - simulating values used across URL, headers, body, and multipart
	urlKey := "api.example.com/users/{{:userId}}"
	sm.SetUserInputs(urlKey, map[string]string{
		"userId":   "42",
		"username": "john",
		"token":    "abc123",
	})

	p := NewPrompter(sm, false, false)

	// Simulate processing different parts of a request that share the same urlKey
	// This mimics how URL, headers, body, and multipart all use the same session values

	// URL pattern
	urlResult, err := p.ProcessURL("https://api.example.com/users/{{:userId}}")
	if err != nil {
		t.Fatalf("ProcessURL() error = %v", err)
	}
	if urlResult.URL != "https://api.example.com/users/42" {
		t.Errorf("URL = %v, want https://api.example.com/users/42", urlResult.URL)
	}

	// Header value
	headerValue, err := p.ProcessContent("Bearer {{:token}}", urlKey)
	if err != nil {
		t.Fatalf("ProcessContent(header) error = %v", err)
	}
	if headerValue != "Bearer abc123" {
		t.Errorf("Header = %v, want Bearer abc123", headerValue)
	}

	// Body content
	bodyContent, err := p.ProcessContent(`{"user": "{{:username}}", "id": "{{:userId}}"}`, urlKey)
	if err != nil {
		t.Fatalf("ProcessContent(body) error = %v", err)
	}
	if bodyContent != `{"user": "john", "id": "42"}` {
		t.Errorf("Body = %v, want {\"user\": \"john\", \"id\": \"42\"}", bodyContent)
	}

	// Multipart field value
	multipartValue, err := p.ProcessContent("Created by {{:username}}", urlKey)
	if err != nil {
		t.Fatalf("ProcessContent(multipart) error = %v", err)
	}
	if multipartValue != "Created by john" {
		t.Errorf("Multipart = %v, want Created by john", multipartValue)
	}
}

// Edge Case Tests

func TestPrompter_ProcessContent_PatternInHeaderButNotInURL(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// URL has no patterns, but we store values for headers
	urlKey := "api.example.com/profile"
	sm.SetUserInputs(urlKey, map[string]string{
		"token": "secret-token-123",
	})

	p := NewPrompter(sm, false, false)

	// URL without patterns - should return as-is
	url := "https://api.example.com/profile"
	urlResult, err := p.ProcessURL(url)
	if err != nil {
		t.Fatalf("ProcessURL() error = %v", err)
	}
	if urlResult.URL != url {
		t.Errorf("URL = %v, want %v", urlResult.URL, url)
	}
	if len(urlResult.Patterns) != 0 {
		t.Errorf("Patterns count = %v, want 0", len(urlResult.Patterns))
	}

	// Header with pattern - should use stored value
	headerValue, err := p.ProcessContent("Bearer {{:token}}", urlKey)
	if err != nil {
		t.Fatalf("ProcessContent() error = %v", err)
	}
	if headerValue != "Bearer secret-token-123" {
		t.Errorf("Header = %v, want Bearer secret-token-123", headerValue)
	}
}

func TestPrompter_ProcessContent_PatternInBodyButNotInURL(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// URL has no patterns, but we store values for body
	urlKey := "api.example.com/login"
	sm.SetUserInputs(urlKey, map[string]string{
		"username": "admin",
		"password": "secret",
	})

	p := NewPrompter(sm, false, false)

	// Body with patterns - should use stored values
	body := `{"username": "{{:username}}", "password": "{{:password}}"}`
	processedBody, err := p.ProcessContent(body, urlKey)
	if err != nil {
		t.Fatalf("ProcessContent() error = %v", err)
	}
	expected := `{"username": "admin", "password": "secret"}`
	if processedBody != expected {
		t.Errorf("Body = %v, want %v", processedBody, expected)
	}
}

func TestPrompter_SameParameterSharedAcrossURLAndHeaders(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Same parameter name used in both URL and header
	urlKey := "api.example.com/users/{{:id}}"
	sm.SetUserInputs(urlKey, map[string]string{
		"id": "42",
	})

	p := NewPrompter(sm, false, false)

	// URL with pattern
	urlResult, err := p.ProcessURL("https://api.example.com/users/{{:id}}")
	if err != nil {
		t.Fatalf("ProcessURL() error = %v", err)
	}
	if urlResult.URL != "https://api.example.com/users/42" {
		t.Errorf("URL = %v, want https://api.example.com/users/42", urlResult.URL)
	}

	// Header with same parameter - should use same value
	headerValue, err := p.ProcessContent("X-Request-Id: {{:id}}", urlKey)
	if err != nil {
		t.Fatalf("ProcessContent() error = %v", err)
	}
	if headerValue != "X-Request-Id: 42" {
		t.Errorf("Header = %v, want X-Request-Id: 42", headerValue)
	}

	// Body with same parameter - should use same value
	body, err := p.ProcessContent(`{"requestId": "{{:id}}"}`, urlKey)
	if err != nil {
		t.Fatalf("ProcessContent() error = %v", err)
	}
	if body != `{"requestId": "42"}` {
		t.Errorf("Body = %v, want {\"requestId\": \"42\"}", body)
	}
}

func TestPrompter_EmptyStringValue(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Use the correct URL key that matches what GenerateKey would produce
	urlKey := "api.example.com/search?q={{:query}}"
	sm.SetUserInputs(urlKey, map[string]string{
		"query": "", // Empty string
	})

	p := NewPrompter(sm, false, false)

	// URL with empty value
	urlResult, err := p.ProcessURL("https://api.example.com/search?q={{:query}}")
	if err != nil {
		t.Fatalf("ProcessURL() error = %v", err)
	}
	// Empty value should result in empty parameter
	if urlResult.URL != "https://api.example.com/search?q=" {
		t.Errorf("URL = %v, want https://api.example.com/search?q=", urlResult.URL)
	}

	// Body with empty value
	body, err := p.ProcessContent(`{"filter": "{{:query}}"}`, urlKey)
	if err != nil {
		t.Fatalf("ProcessContent() error = %v", err)
	}
	if body != `{"filter": ""}` {
		t.Errorf("Body = %v, want {\"filter\": \"\"}", body)
	}
}

func TestPrompter_SpecialCharactersInBody(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	urlKey := "api.example.com/message"

	tests := []struct {
		name  string
		value string
		body  string
		want  string
	}{
		{
			name:  "value with double quotes",
			value: `Hello "world"`,
			body:  `{"message": "{{:msg}}"}`,
			want:  `{"message": "Hello "world""}`, // Invalid JSON - user responsibility
		},
		{
			name:  "value with newlines",
			value: "line1\nline2",
			body:  `{"text": "{{:msg}}"}`,
			want:  "{\"text\": \"line1\nline2\"}", // Raw newline in JSON
		},
		{
			name:  "value with backslash",
			value: `path\to\file`,
			body:  `{"path": "{{:msg}}"}`,
			want:  `{"path": "path\to\file"}`,
		},
		{
			name:  "value with unicode",
			value: "日本語テスト",
			body:  `{"text": "{{:msg}}"}`,
			want:  `{"text": "日本語テスト"}`,
		},
		{
			name:  "value with emoji",
			value: "Hello 👋 World 🌍",
			body:  `{"greeting": "{{:msg}}"}`,
			want:  `{"greeting": "Hello 👋 World 🌍"}`,
		},
		{
			name:  "value with HTML tags",
			value: "<script>alert('xss')</script>",
			body:  `{"html": "{{:msg}}"}`,
			want:  `{"html": "<script>alert('xss')</script>"}`,
		},
		{
			name:  "value with SQL injection attempt",
			value: "'; DROP TABLE users; --",
			body:  `{"query": "{{:msg}}"}`,
			want:  `{"query": "'; DROP TABLE users; --"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm.SetUserInputs(urlKey, map[string]string{"msg": tt.value})
			p := NewPrompter(sm, false, false)

			result, err := p.ProcessContent(tt.body, urlKey)
			if err != nil {
				t.Fatalf("ProcessContent() error = %v", err)
			}
			if result != tt.want {
				t.Errorf("ProcessContent() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestPrompter_ContentTypeHeaderWithPattern(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	urlKey := "api.example.com/upload"
	sm.SetUserInputs(urlKey, map[string]string{
		"contentType": "application/json",
	})

	p := NewPrompter(sm, false, false)

	// Content-Type header with pattern
	headerValue, err := p.ProcessContent("{{:contentType}}", urlKey)
	if err != nil {
		t.Fatalf("ProcessContent() error = %v", err)
	}
	if headerValue != "application/json" {
		t.Errorf("Header = %v, want application/json", headerValue)
	}
}

func TestPrompter_URLEncodingVsRawReplacement(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	urlKey := "api.example.com/test/{{:value}}"
	valueWithSpaces := "hello world"
	sm.SetUserInputs(urlKey, map[string]string{
		"value": valueWithSpaces,
	})

	p := NewPrompter(sm, false, false)

	// URL should be URL-encoded
	urlResult, err := p.ProcessURL("https://api.example.com/test/{{:value}}")
	if err != nil {
		t.Fatalf("ProcessURL() error = %v", err)
	}
	if urlResult.URL != "https://api.example.com/test/hello%20world" {
		t.Errorf("URL = %v, want https://api.example.com/test/hello%%20world", urlResult.URL)
	}

	// Header should NOT be URL-encoded
	headerValue, err := p.ProcessContent("X-Value: {{:value}}", urlKey)
	if err != nil {
		t.Fatalf("ProcessContent(header) error = %v", err)
	}
	if headerValue != "X-Value: hello world" {
		t.Errorf("Header = %v, want X-Value: hello world", headerValue)
	}

	// Body should NOT be URL-encoded
	body, err := p.ProcessContent(`{"value": "{{:value}}"}`, urlKey)
	if err != nil {
		t.Fatalf("ProcessContent(body) error = %v", err)
	}
	if body != `{"value": "hello world"}` {
		t.Errorf("Body = %v, want {\"value\": \"hello world\"}", body)
	}
}

func TestPrompter_DifferentSessionKeysForDifferentURLs(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Two different URLs with same parameter name but different values
	urlKey1 := "api.example.com/users/{{:id}}"
	urlKey2 := "api.example.com/posts/{{:id}}"

	sm.SetUserInputs(urlKey1, map[string]string{"id": "user-42"})
	sm.SetUserInputs(urlKey2, map[string]string{"id": "post-99"})

	p := NewPrompter(sm, false, false)

	// First URL
	result1, err := p.ProcessURL("https://api.example.com/users/{{:id}}")
	if err != nil {
		t.Fatalf("ProcessURL() error = %v", err)
	}
	if result1.URL != "https://api.example.com/users/user-42" {
		t.Errorf("URL1 = %v, want https://api.example.com/users/user-42", result1.URL)
	}

	// Second URL - should have different value
	result2, err := p.ProcessURL("https://api.example.com/posts/{{:id}}")
	if err != nil {
		t.Fatalf("ProcessURL() error = %v", err)
	}
	if result2.URL != "https://api.example.com/posts/post-99" {
		t.Errorf("URL2 = %v, want https://api.example.com/posts/post-99", result2.URL)
	}
}

func TestPrompter_MultiplePatternsSameNameInContent(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	urlKey := "api.example.com/test"
	sm.SetUserInputs(urlKey, map[string]string{
		"id": "123",
	})

	p := NewPrompter(sm, false, false)

	// Same pattern used multiple times in body
	body := `{"id": "{{:id}}", "relatedId": "{{:id}}", "refId": "{{:id}}"}`
	result, err := p.ProcessContent(body, urlKey)
	if err != nil {
		t.Fatalf("ProcessContent() error = %v", err)
	}
	expected := `{"id": "123", "relatedId": "123", "refId": "123"}`
	if result != expected {
		t.Errorf("Body = %v, want %v", result, expected)
	}
}

func TestPrompter_FormURLEncodedBody(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	urlKey := "api.example.com/login"
	sm.SetUserInputs(urlKey, map[string]string{
		"username": "john",
		"password": "secret123",
	})

	p := NewPrompter(sm, false, false)

	// Form URL-encoded body
	body := `username={{:username}}&password={{:password}}`
	result, err := p.ProcessContent(body, urlKey)
	if err != nil {
		t.Fatalf("ProcessContent() error = %v", err)
	}
	expected := `username=john&password=secret123`
	if result != expected {
		t.Errorf("Body = %v, want %v", result, expected)
	}
}

func TestPrompter_MultipartFieldValue(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	urlKey := "api.example.com/upload"
	sm.SetUserInputs(urlKey, map[string]string{
		"description": "My awesome file",
		"tags":        "important,urgent",
	})

	p := NewPrompter(sm, false, false)

	// Multipart text field values
	desc, err := p.ProcessContent("{{:description}}", urlKey)
	if err != nil {
		t.Fatalf("ProcessContent(description) error = %v", err)
	}
	if desc != "My awesome file" {
		t.Errorf("Description = %v, want My awesome file", desc)
	}

	tags, err := p.ProcessContent("{{:tags}}", urlKey)
	if err != nil {
		t.Fatalf("ProcessContent(tags) error = %v", err)
	}
	if tags != "important,urgent" {
		t.Errorf("Tags = %v, want important,urgent", tags)
	}
}

func TestPrompter_MultipartFilePath(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	urlKey := "api.example.com/upload"
	sm.SetUserInputs(urlKey, map[string]string{
		"filePath": "/home/user/documents/report.pdf",
	})

	p := NewPrompter(sm, false, false)

	// File path with pattern - should be processed with ReplaceRaw (no encoding)
	filePath, err := p.ProcessContent("{{:filePath}}", urlKey)
	if err != nil {
		t.Fatalf("ProcessContent(filePath) error = %v", err)
	}
	if filePath != "/home/user/documents/report.pdf" {
		t.Errorf("FilePath = %v, want /home/user/documents/report.pdf", filePath)
	}
}

func TestPrompter_MultipartFilePathWithSpaces(t *testing.T) {
	tempDir := t.TempDir()

	mockFS := filesystem.NewMockFileSystem()
	sm, err := session.NewSessionManagerWithFS(mockFS, tempDir, "", "test")
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	urlKey := "api.example.com/upload"
	sm.SetUserInputs(urlKey, map[string]string{
		"filePath": "/home/user/My Documents/report.pdf",
	})

	p := NewPrompter(sm, false, false)

	// File path with spaces - should NOT be URL encoded
	filePath, err := p.ProcessContent("{{:filePath}}", urlKey)
	if err != nil {
		t.Fatalf("ProcessContent(filePath) error = %v", err)
	}
	if filePath != "/home/user/My Documents/report.pdf" {
		t.Errorf("FilePath = %v, want /home/user/My Documents/report.pdf", filePath)
	}
}
