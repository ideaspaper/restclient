package userinput

import (
	"reflect"
	"testing"
)

func TestDetector_Detect(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want []Pattern
	}{
		{
			name: "no patterns",
			url:  "https://api.example.com/users",
			want: nil,
		},
		{
			name: "single path parameter",
			url:  "https://api.example.com/posts/{{:id}}",
			want: []Pattern{
				{Name: "id", Original: "{{:id}}", Position: 30},
			},
		},
		{
			name: "single query parameter",
			url:  "https://api.example.com/posts?page={{:page}}",
			want: []Pattern{
				{Name: "page", Original: "{{:page}}", Position: 35},
			},
		},
		{
			name: "multiple query parameters",
			url:  "https://api.example.com/posts?page={{:page}}&limit={{:limit}}",
			want: []Pattern{
				{Name: "page", Original: "{{:page}}", Position: 35},
				{Name: "limit", Original: "{{:limit}}", Position: 51},
			},
		},
		{
			name: "mixed path and query parameters",
			url:  "https://api.example.com/users/{{:userId}}/posts?page={{:page}}",
			want: []Pattern{
				{Name: "userId", Original: "{{:userId}}", Position: 30},
				{Name: "page", Original: "{{:page}}", Position: 53},
			},
		},
		{
			name: "parameter with underscore",
			url:  "https://api.example.com/users/{{:user_id}}",
			want: []Pattern{
				{Name: "user_id", Original: "{{:user_id}}", Position: 30},
			},
		},
		{
			name: "parameter with numbers",
			url:  "https://api.example.com/items/{{:id1}}/{{:id2}}",
			want: []Pattern{
				{Name: "id1", Original: "{{:id1}}", Position: 30},
				{Name: "id2", Original: "{{:id2}}", Position: 39},
			},
		},
		{
			name: "duplicate parameter names - only first returned",
			url:  "https://api.example.com/users/{{:id}}/posts/{{:id}}",
			want: []Pattern{
				{Name: "id", Original: "{{:id}}", Position: 30},
			},
		},
		{
			name: "nested paths",
			url:  "https://api.example.com/users/{{:userId}}/posts/{{:postId}}/comments/{{:commentId}}",
			want: []Pattern{
				{Name: "userId", Original: "{{:userId}}", Position: 30},
				{Name: "postId", Original: "{{:postId}}", Position: 48},
				{Name: "commentId", Original: "{{:commentId}}", Position: 69},
			},
		},
		{
			name: "URL with port",
			url:  "http://localhost:8080/api/{{:resource}}/{{:id}}",
			want: []Pattern{
				{Name: "resource", Original: "{{:resource}}", Position: 26},
				{Name: "id", Original: "{{:id}}", Position: 40},
			},
		},
		{
			name: "only colon without braces - not matched",
			url:  "https://api.example.com/:id",
			want: nil,
		},
		{
			name: "regular variable syntax - not matched",
			url:  "https://api.example.com/{{baseUrl}}/posts",
			want: nil,
		},
		{
			name: "mixed regular and user input variables",
			url:  "{{baseUrl}}/posts/{{:id}}?api_key={{apiKey}}",
			want: []Pattern{
				{Name: "id", Original: "{{:id}}", Position: 18},
			},
		},
	}

	detector := NewDetector()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.Detect(tt.url)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetector_HasPatterns(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "has patterns",
			url:  "https://api.example.com/posts/{{:id}}",
			want: true,
		},
		{
			name: "no patterns",
			url:  "https://api.example.com/posts/1",
			want: false,
		},
		{
			name: "regular variable - no user input patterns",
			url:  "{{baseUrl}}/posts",
			want: false,
		},
	}

	detector := NewDetector()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.HasPatterns(tt.url)
			if got != tt.want {
				t.Errorf("HasPatterns() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetector_Replace(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		values map[string]string
		want   string
	}{
		{
			name:   "single replacement",
			url:    "https://api.example.com/posts/{{:id}}",
			values: map[string]string{"id": "123"},
			want:   "https://api.example.com/posts/123",
		},
		{
			name:   "multiple replacements",
			url:    "https://api.example.com/posts?page={{:page}}&limit={{:limit}}",
			values: map[string]string{"page": "1", "limit": "10"},
			want:   "https://api.example.com/posts?page=1&limit=10",
		},
		{
			name:   "duplicate parameter - both replaced",
			url:    "https://api.example.com/users/{{:id}}/compare/{{:id}}",
			values: map[string]string{"id": "42"},
			want:   "https://api.example.com/users/42/compare/42",
		},
		{
			name:   "value with special characters - URL encoded",
			url:    "https://api.example.com/search?q={{:query}}",
			values: map[string]string{"query": "hello world"},
			want:   "https://api.example.com/search?q=hello%20world",
		},
		{
			name:   "value with slashes - URL encoded",
			url:    "https://api.example.com/files/{{:path}}",
			values: map[string]string{"path": "folder/file.txt"},
			want:   "https://api.example.com/files/folder%2Ffile.txt",
		},
		{
			name:   "empty value",
			url:    "https://api.example.com/posts?filter={{:filter}}",
			values: map[string]string{"filter": ""},
			want:   "https://api.example.com/posts?filter=",
		},
		{
			name:   "missing value - pattern unchanged",
			url:    "https://api.example.com/posts/{{:id}}",
			values: map[string]string{},
			want:   "https://api.example.com/posts/{{:id}}",
		},
		{
			name:   "partial values - only matching replaced",
			url:    "https://api.example.com/users/{{:userId}}/posts/{{:postId}}",
			values: map[string]string{"userId": "1"},
			want:   "https://api.example.com/users/1/posts/{{:postId}}",
		},
	}

	detector := NewDetector()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.Replace(tt.url, tt.values)
			if got != tt.want {
				t.Errorf("Replace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetector_ReplaceRaw(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		values map[string]string
		want   string
	}{
		{
			name:   "value with spaces - not encoded",
			url:    "https://api.example.com/search?q={{:query}}",
			values: map[string]string{"query": "hello world"},
			want:   "https://api.example.com/search?q=hello world",
		},
		{
			name:   "value with slashes - not encoded",
			url:    "https://api.example.com/files/{{:path}}",
			values: map[string]string{"path": "folder/file.txt"},
			want:   "https://api.example.com/files/folder/file.txt",
		},
	}

	detector := NewDetector()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.ReplaceRaw(tt.url, tt.values)
			if got != tt.want {
				t.Errorf("ReplaceRaw() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetector_GenerateKey(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "simple URL with path",
			url:  "https://api.example.com/posts/{{:id}}",
			want: "api.example.com/posts/{{:id}}",
		},
		{
			name: "URL with query parameters",
			url:  "https://api.example.com/posts?page={{:page}}&limit={{:limit}}",
			want: "api.example.com/posts?page={{:page}}&limit={{:limit}}",
		},
		{
			name: "URL with port",
			url:  "http://localhost:8080/api/{{:id}}",
			want: "localhost:8080/api/{{:id}}",
		},
		{
			name: "URL without path",
			url:  "https://api.example.com",
			want: "api.example.com",
		},
	}

	detector := NewDetector()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.GenerateKey(tt.url)
			if got != tt.want {
				t.Errorf("GenerateKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetector_ExtractPatternNames(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want []string
	}{
		{
			name: "no patterns",
			url:  "https://api.example.com/posts",
			want: []string{},
		},
		{
			name: "single pattern",
			url:  "https://api.example.com/posts/{{:id}}",
			want: []string{"id"},
		},
		{
			name: "multiple patterns",
			url:  "https://api.example.com/users/{{:userId}}/posts/{{:postId}}",
			want: []string{"userId", "postId"},
		},
	}

	detector := NewDetector()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.ExtractPatternNames(tt.url)
			// Handle nil vs empty slice comparison
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractPatternNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Edge case tests for user input variables

func TestDetector_Detect_EdgeCases(t *testing.T) {
	detector := NewDetector()

	tests := []struct {
		name      string
		url       string
		wantCount int
		wantNames []string
	}{
		{
			name:      "empty string",
			url:       "",
			wantCount: 0,
			wantNames: nil,
		},
		{
			name:      "only whitespace",
			url:       "   ",
			wantCount: 0,
			wantNames: nil,
		},
		{
			name:      "malformed pattern - missing closing braces",
			url:       "https://api.example.com/posts/{{:id",
			wantCount: 0,
			wantNames: nil,
		},
		{
			name:      "malformed pattern - missing opening braces",
			url:       "https://api.example.com/posts/:id}}",
			wantCount: 0,
			wantNames: nil,
		},
		{
			name:      "malformed pattern - missing colon",
			url:       "https://api.example.com/posts/{{id}}",
			wantCount: 0,
			wantNames: nil,
		},
		{
			name:      "pattern with only colon",
			url:       "https://api.example.com/posts/{{:}}",
			wantCount: 0,
			wantNames: nil,
		},
		{
			name:      "pattern with spaces in name",
			url:       "https://api.example.com/posts/{{:my id}}",
			wantCount: 0,
			wantNames: nil,
		},
		{
			name:      "pattern with special characters in name",
			url:       "https://api.example.com/posts/{{:id-value}}",
			wantCount: 0,
			wantNames: nil,
		},
		{
			name:      "pattern at start of URL",
			url:       "{{:protocol}}://api.example.com/posts",
			wantCount: 1,
			wantNames: []string{"protocol"},
		},
		{
			name:      "pattern at end of URL",
			url:       "https://api.example.com/posts/{{:id}}",
			wantCount: 1,
			wantNames: []string{"id"},
		},
		{
			name:      "consecutive patterns",
			url:       "https://api.example.com/{{:a}}{{:b}}{{:c}}",
			wantCount: 3,
			wantNames: []string{"a", "b", "c"},
		},
		{
			name:      "pattern with very long name",
			url:       "https://api.example.com/posts/{{:thisIsAVeryLongParameterNameThatShouldStillWork}}",
			wantCount: 1,
			wantNames: []string{"thisIsAVeryLongParameterNameThatShouldStillWork"},
		},
		{
			name:      "pattern with single character name",
			url:       "https://api.example.com/posts/{{:x}}",
			wantCount: 1,
			wantNames: []string{"x"},
		},
		{
			name:      "pattern starting with number - invalid",
			url:       "https://api.example.com/posts/{{:1id}}",
			wantCount: 0,
			wantNames: nil,
		},
		{
			name:      "pattern with all numbers - invalid",
			url:       "https://api.example.com/posts/{{:123}}",
			wantCount: 0,
			wantNames: nil,
		},
		{
			name:      "nested braces - partial match",
			url:       "https://api.example.com/posts/{{:{{:id}}}}",
			wantCount: 1,
			wantNames: []string{"id"},
		},
		{
			name:      "URL with fragment",
			url:       "https://api.example.com/posts/{{:id}}#section",
			wantCount: 1,
			wantNames: []string{"id"},
		},
		{
			name:      "URL with username and password",
			url:       "https://user:pass@api.example.com/posts/{{:id}}",
			wantCount: 1,
			wantNames: []string{"id"},
		},
		{
			name:      "multiple same patterns",
			url:       "https://api.example.com/{{:id}}/{{:id}}/{{:id}}",
			wantCount: 1,
			wantNames: []string{"id"},
		},
		{
			name:      "pattern in query value only",
			url:       "https://api.example.com/posts?id={{:id}}&name={{:name}}",
			wantCount: 2,
			wantNames: []string{"id", "name"},
		},
		{
			name:      "Unicode in URL path with pattern",
			url:       "https://api.example.com/用户/{{:userId}}/帖子",
			wantCount: 1,
			wantNames: []string{"userId"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := detector.Detect(tt.url)
			if len(patterns) != tt.wantCount {
				t.Errorf("Detect() count = %v, want %v", len(patterns), tt.wantCount)
			}
			if tt.wantNames != nil {
				for i, name := range tt.wantNames {
					if i < len(patterns) && patterns[i].Name != name {
						t.Errorf("Detect() pattern[%d].Name = %v, want %v", i, patterns[i].Name, name)
					}
				}
			}
		})
	}
}

func TestDetector_Replace_EdgeCases(t *testing.T) {
	detector := NewDetector()

	tests := []struct {
		name   string
		url    string
		values map[string]string
		want   string
	}{
		{
			name:   "nil values map",
			url:    "https://api.example.com/posts/{{:id}}",
			values: nil,
			want:   "https://api.example.com/posts/{{:id}}",
		},
		{
			name:   "value with ampersand - PathEscape does not encode",
			url:    "https://api.example.com/search?q={{:query}}",
			values: map[string]string{"query": "foo&bar"},
			want:   "https://api.example.com/search?q=foo&bar",
		},
		{
			name:   "value with equals sign - PathEscape does not encode",
			url:    "https://api.example.com/search?q={{:query}}",
			values: map[string]string{"query": "a=b"},
			want:   "https://api.example.com/search?q=a=b",
		},
		{
			name:   "value with question mark",
			url:    "https://api.example.com/search?q={{:query}}",
			values: map[string]string{"query": "what?"},
			want:   "https://api.example.com/search?q=what%3F",
		},
		{
			name:   "value with hash",
			url:    "https://api.example.com/tags/{{:tag}}",
			values: map[string]string{"tag": "#trending"},
			want:   "https://api.example.com/tags/%23trending",
		},
		{
			name:   "value with percent sign",
			url:    "https://api.example.com/discount/{{:percent}}",
			values: map[string]string{"percent": "50%"},
			want:   "https://api.example.com/discount/50%25",
		},
		{
			name:   "value with plus sign - PathEscape does not encode",
			url:    "https://api.example.com/math/{{:expr}}",
			values: map[string]string{"expr": "1+1"},
			want:   "https://api.example.com/math/1+1",
		},
		{
			name:   "value with newline",
			url:    "https://api.example.com/text/{{:content}}",
			values: map[string]string{"content": "line1\nline2"},
			want:   "https://api.example.com/text/line1%0Aline2",
		},
		{
			name:   "value with tab",
			url:    "https://api.example.com/text/{{:content}}",
			values: map[string]string{"content": "col1\tcol2"},
			want:   "https://api.example.com/text/col1%09col2",
		},
		{
			name:   "value with unicode",
			url:    "https://api.example.com/greet/{{:name}}",
			values: map[string]string{"name": "日本語"},
			want:   "https://api.example.com/greet/%E6%97%A5%E6%9C%AC%E8%AA%9E",
		},
		{
			name:   "value with emoji",
			url:    "https://api.example.com/emoji/{{:emoji}}",
			values: map[string]string{"emoji": "😀"},
			want:   "https://api.example.com/emoji/%F0%9F%98%80",
		},
		{
			name:   "very long value",
			url:    "https://api.example.com/data/{{:data}}",
			values: map[string]string{"data": "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
			want:   "https://api.example.com/data/abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		},
		{
			name:   "value with only spaces",
			url:    "https://api.example.com/search?q={{:query}}",
			values: map[string]string{"query": "   "},
			want:   "https://api.example.com/search?q=%20%20%20",
		},
		{
			name:   "extra values in map ignored",
			url:    "https://api.example.com/posts/{{:id}}",
			values: map[string]string{"id": "123", "extra": "ignored", "another": "also ignored"},
			want:   "https://api.example.com/posts/123",
		},
		{
			name:   "case sensitive parameter names",
			url:    "https://api.example.com/{{:ID}}/{{:id}}/{{:Id}}",
			values: map[string]string{"ID": "1", "id": "2", "Id": "3"},
			want:   "https://api.example.com/1/2/3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.Replace(tt.url, tt.values)
			if got != tt.want {
				t.Errorf("Replace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetector_GenerateKey_EdgeCases(t *testing.T) {
	detector := NewDetector()

	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "invalid URL - returns as is",
			url:  "not a valid url",
			want: "not a valid url",
		},
		{
			name: "URL with trailing slash",
			url:  "https://api.example.com/posts/",
			want: "api.example.com/posts/",
		},
		{
			name: "URL with multiple slashes",
			url:  "https://api.example.com//posts///{{:id}}",
			want: "api.example.com//posts///{{:id}}",
		},
		{
			name: "URL with fragment - fragment not included in key",
			url:  "https://api.example.com/posts/{{:id}}#section",
			want: "api.example.com/posts/{{:id}}",
		},
		{
			name: "URL with empty query - normalized without trailing ?",
			url:  "https://api.example.com/posts?",
			want: "api.example.com/posts",
		},
		{
			name: "URL with only query",
			url:  "https://api.example.com?{{:param}}={{:value}}",
			want: "api.example.com?{{:param}}={{:value}}",
		},
		{
			name: "localhost without port",
			url:  "http://localhost/api/{{:id}}",
			want: "localhost/api/{{:id}}",
		},
		{
			name: "IP address",
			url:  "http://192.168.1.1:8080/api/{{:id}}",
			want: "192.168.1.1:8080/api/{{:id}}",
		},
		{
			name: "IPv6 address",
			url:  "http://[::1]:8080/api/{{:id}}",
			want: "[::1]:8080/api/{{:id}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.GenerateKey(tt.url)
			if got != tt.want {
				t.Errorf("GenerateKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
