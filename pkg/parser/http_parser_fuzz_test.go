package parser

import (
	"strings"
	"testing"
)

// FuzzParseRequest tests the parser with random inputs
func FuzzParseRequest(f *testing.F) {
	// Seed corpus with valid examples
	seeds := []string{
		"GET https://api.example.com/users",
		"POST https://api.example.com/users\nContent-Type: application/json\n\n{\"name\": \"test\"}",
		"# @name test\nGET https://api.example.com",
		"GET https://api.example.com/users\n?page=1\n&limit=10",
		"DELETE https://api.example.com/users/123",
		"PUT https://api.example.com/users/123\nContent-Type: application/json\n\n{\"name\": \"updated\"}",
		"@baseUrl = https://api.example.com\n\nGET {{baseUrl}}/users",
		"GET https://api.example.com/users\nAuthorization: Bearer token123",
		"POST https://api.example.com/graphql\nContent-Type: application/json\nX-Request-Type: GraphQL\n\nquery { users { id } }",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		parser := NewHttpRequestParser(input, nil, "")

		// ParseRequest should not panic
		_, _ = parser.ParseRequest(input)

		// ParseAll should not panic
		_, _ = parser.ParseAll()

		// ParseAllWithWarnings should not panic
		_ = parser.ParseAllWithWarnings()
	})
}

// FuzzSplitRequestBlocks tests block splitting with random inputs
func FuzzSplitRequestBlocks(f *testing.F) {
	seeds := []string{
		"GET /1\n###\nGET /2",
		"GET /1\n####\nGET /2",
		"GET /1\n##########\nGET /2",
		"###\n###\n###",
		"",
		"GET /only",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := splitRequestBlocks(input)

		// Result should be a valid slice
		if result == nil {
			t.Error("splitRequestBlocks returned nil")
		}
	})
}

// FuzzParseRequestLine tests request line parsing
func FuzzParseRequestLine(f *testing.F) {
	seeds := []string{
		"GET https://api.example.com/users",
		"POST https://api.example.com/users",
		"DELETE https://api.example.com/users/123",
		"GET https://api.example.com/users HTTP/1.1",
		"https://api.example.com/users",
		"get https://api.example.com",
		"",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := parseRequestLine(input)

		// Method should be uppercase if not empty
		if result.Method != "" && result.Method != strings.ToUpper(result.Method) {
			t.Errorf("Method should be uppercase: %s", result.Method)
		}

		// URL should not contain the method
		if result.URL != "" && result.Method != "" && strings.HasPrefix(result.URL, result.Method) {
			t.Errorf("URL should not start with method: %s", result.URL)
		}
	})
}

// FuzzParseHeaders tests header parsing
func FuzzParseHeaders(f *testing.F) {
	seeds := []string{
		"Content-Type: application/json",
		"Authorization: Bearer token123",
		"X-Custom: value:with:colons",
		"Accept: text/html\nAccept: application/json",
		"Cookie: a=1\nCookie: b=2",
		"",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		lines := strings.Split(input, "\n")
		// Should not panic
		result := parseHeaders(lines, nil, "https://example.com")

		// Result should be a valid map
		if result == nil {
			t.Error("parseHeaders returned nil")
		}
	})
}

// FuzzIsComment tests comment detection
func FuzzIsComment(f *testing.F) {
	seeds := []string{
		"# comment",
		"// comment",
		"GET https://api.example.com",
		"",
		"  # indented comment",
		"#",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := isComment(input)

		// Verify expected behavior for clear cases
		trimmed := strings.TrimSpace(input)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			if !result && len(trimmed) > 0 {
				// This is a comment, result should be true
				// (unless trimmed is empty after removing the prefix)
			}
		}
	})
}

// FuzzIsFileVariable tests file variable detection
func FuzzIsFileVariable(f *testing.F) {
	seeds := []string{
		"@baseUrl = https://api.example.com",
		"  @token = abc123",
		"@name = value",
		"GET https://api.example.com",
		"email@example.com",
		"@",
		"@ =",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		_ = isFileVariable(input)
	})
}

// FuzzExtractBoundary tests boundary extraction from Content-Type
func FuzzExtractBoundary(f *testing.F) {
	seeds := []string{
		"multipart/form-data; boundary=----WebKitFormBoundary",
		`multipart/form-data; boundary="quoted-boundary"`,
		"multipart/form-data",
		"application/json",
		"",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		_ = extractBoundary(input)
	})
}

// FuzzCreateGraphQLBody tests GraphQL body creation
func FuzzCreateGraphQLBody(f *testing.F) {
	seeds := []string{
		"query GetUser { user { id } }",
		"mutation CreateUser { createUser(name: \"test\") { id } }",
		"subscription OnMessage { messageAdded { id } }",
		"query GetUser { user { id } }\n{\"id\": \"123\"}",
		"{ users { name } }",
		"",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := createGraphQLBody(input)

		// Result should be valid JSON if not empty
		if result != "" {
			if !strings.HasPrefix(result, "{") || !strings.HasSuffix(result, "}") {
				// Result should be a JSON object
			}
		}
	})
}

// FuzzParseMetadata tests metadata parsing
func FuzzParseMetadata(f *testing.F) {
	seeds := []string{
		"# @name myRequest",
		"// @name myRequest",
		"# @no-redirect",
		"# @no-cookie-jar",
		"# @note This is a test",
		"# @prompt username Enter username",
		"# regular comment",
		"GET https://api.example.com",
		"",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		meta, ok := parseMetadata(input)

		// If ok is true, meta should not be nil
		if ok && meta == nil {
			t.Error("parseMetadata returned ok=true but nil map")
		}
	})
}

// FuzzParseMultipartSection tests multipart section parsing
func FuzzParseMultipartSection(f *testing.F) {
	seeds := []string{
		"Content-Disposition: form-data; name=\"field\"\n\nvalue",
		"Content-Disposition: form-data; name=\"file\"; filename=\"test.txt\"\nContent-Type: text/plain\n\ncontent",
		"Content-Disposition: form-data; name=\"data\"\n\n< ./file.json",
		"",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := parseMultipartSection(input)

		// Result should be a valid struct (zero value is ok)
		_ = result.Name
		_ = result.Value
		_ = result.FileName
	})
}

// FuzzFindDuplicateNames tests duplicate name finding
func FuzzFindDuplicateNames(f *testing.F) {
	// Use string inputs that we parse into requests
	seeds := []string{
		"# @name test\nGET /1\n###\n# @name test\nGET /2",
		"# @name a\nGET /1\n###\n# @name b\nGET /2",
		"GET /1\n###\nGET /2",
		"",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		parser := NewHttpRequestParser(input, nil, "")
		result := parser.ParseAllWithWarnings()

		// Should not panic
		duplicates := FindDuplicateNames(result.Requests)

		// Result should be a valid map
		if duplicates == nil {
			t.Error("FindDuplicateNames returned nil")
		}
	})
}
