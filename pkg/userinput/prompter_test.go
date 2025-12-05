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
