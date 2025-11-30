package parser

import (
	"strings"
	"testing"
)

func TestParseRequestLine(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantMethod string
		wantURL    string
	}{
		{
			name:       "simple GET",
			input:      "GET https://api.example.com/users",
			wantMethod: "GET",
			wantURL:    "https://api.example.com/users",
		},
		{
			name:       "POST request",
			input:      "POST https://api.example.com/users",
			wantMethod: "POST",
			wantURL:    "https://api.example.com/users",
		},
		{
			name:       "with HTTP version",
			input:      "GET https://api.example.com/users HTTP/1.1",
			wantMethod: "GET",
			wantURL:    "https://api.example.com/users",
		},
		{
			name:       "with HTTP/2",
			input:      "GET https://api.example.com/users HTTP/2",
			wantMethod: "GET",
			wantURL:    "https://api.example.com/users",
		},
		{
			name:       "URL only (default to GET)",
			input:      "https://api.example.com/users",
			wantMethod: "GET",
			wantURL:    "https://api.example.com/users",
		},
		{
			name:       "DELETE request",
			input:      "DELETE https://api.example.com/users/123",
			wantMethod: "DELETE",
			wantURL:    "https://api.example.com/users/123",
		},
		{
			name:       "PATCH request",
			input:      "PATCH https://api.example.com/users/123",
			wantMethod: "PATCH",
			wantURL:    "https://api.example.com/users/123",
		},
		{
			name:       "PUT request",
			input:      "PUT https://api.example.com/users/123",
			wantMethod: "PUT",
			wantURL:    "https://api.example.com/users/123",
		},
		{
			name:       "OPTIONS request",
			input:      "OPTIONS https://api.example.com/users",
			wantMethod: "OPTIONS",
			wantURL:    "https://api.example.com/users",
		},
		{
			name:       "lowercase method (converted to uppercase)",
			input:      "get https://api.example.com/users",
			wantMethod: "GET",
			wantURL:    "https://api.example.com/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, url := parseRequestLine(tt.input)
			if method != tt.wantMethod {
				t.Errorf("parseRequestLine() method = %v, want %v", method, tt.wantMethod)
			}
			if url != tt.wantURL {
				t.Errorf("parseRequestLine() url = %v, want %v", url, tt.wantURL)
			}
		})
	}
}

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		name           string
		lines          []string
		defaultHeaders map[string]string
		url            string
		wantHeaders    map[string]string
	}{
		{
			name:           "single header",
			lines:          []string{"Content-Type: application/json"},
			defaultHeaders: nil,
			url:            "https://example.com",
			wantHeaders:    map[string]string{"Content-Type": "application/json"},
		},
		{
			name:           "multiple headers",
			lines:          []string{"Content-Type: application/json", "Authorization: Bearer token123"},
			defaultHeaders: nil,
			url:            "https://example.com",
			wantHeaders: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer token123",
			},
		},
		{
			name:  "with default headers",
			lines: []string{"Content-Type: application/json"},
			defaultHeaders: map[string]string{
				"User-Agent": "restclient",
			},
			url: "https://example.com",
			wantHeaders: map[string]string{
				"Content-Type": "application/json",
				"User-Agent":   "restclient",
			},
		},
		{
			name:           "header with extra spaces",
			lines:          []string{"Content-Type:   application/json  "},
			defaultHeaders: nil,
			url:            "https://example.com",
			wantHeaders:    map[string]string{"Content-Type": "application/json"},
		},
		{
			name:           "header with colon in value",
			lines:          []string{"X-Custom: value:with:colons"},
			defaultHeaders: nil,
			url:            "https://example.com",
			wantHeaders:    map[string]string{"X-Custom": "value:with:colons"},
		},
		{
			name:           "combined cookie headers",
			lines:          []string{"Cookie: a=1", "Cookie: b=2"},
			defaultHeaders: nil,
			url:            "https://example.com",
			wantHeaders:    map[string]string{"Cookie": "a=1;b=2"},
		},
		{
			name:           "combined other headers",
			lines:          []string{"Accept: text/html", "Accept: application/json"},
			defaultHeaders: nil,
			url:            "https://example.com",
			wantHeaders:    map[string]string{"Accept": "text/html,application/json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHeaders(tt.lines, tt.defaultHeaders, tt.url)
			for k, v := range tt.wantHeaders {
				if got[k] != v {
					t.Errorf("parseHeaders()[%s] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestParseMetadata(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMeta map[string]string
		wantOK   bool
	}{
		{
			name:     "name metadata with #",
			input:    "# @name myRequest",
			wantMeta: map[string]string{"name": "myRequest"},
			wantOK:   true,
		},
		{
			name:     "name metadata with //",
			input:    "// @name myRequest",
			wantMeta: map[string]string{"name": "myRequest"},
			wantOK:   true,
		},
		{
			name:     "no-redirect metadata",
			input:    "# @no-redirect",
			wantMeta: map[string]string{"no-redirect": ""},
			wantOK:   true,
		},
		{
			name:     "note metadata",
			input:    "# @note This is a test request",
			wantMeta: map[string]string{"note": "This is a test request"},
			wantOK:   true,
		},
		{
			name:     "regular comment",
			input:    "# This is just a comment",
			wantMeta: nil,
			wantOK:   false,
		},
		{
			name:     "not a comment",
			input:    "GET https://api.example.com",
			wantMeta: nil,
			wantOK:   false,
		},
		{
			name:     "prompt metadata",
			input:    "# @prompt username Enter your username",
			wantMeta: map[string]string{"prompt": "username Enter your username"},
			wantOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMeta, gotOK := parseMetadata(tt.input)
			if gotOK != tt.wantOK {
				t.Errorf("parseMetadata() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if tt.wantOK {
				for k, v := range tt.wantMeta {
					if gotMeta[k] != v {
						t.Errorf("parseMetadata()[%s] = %v, want %v", k, gotMeta[k], v)
					}
				}
			}
		})
	}
}

func TestIsComment(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "hash comment", input: "# this is a comment", want: true},
		{name: "double slash comment", input: "// this is a comment", want: true},
		{name: "not a comment", input: "GET https://api.example.com", want: false},
		{name: "hash at start with spaces", input: "  # comment", want: true},
		{name: "empty string", input: "", want: false},
		{name: "just hash", input: "#", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isComment(tt.input); got != tt.want {
				t.Errorf("isComment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsFileVariable(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "file variable", input: "@baseUrl = https://api.example.com", want: true},
		{name: "file variable with spaces", input: "  @token = abc123", want: true},
		{name: "not a variable", input: "GET https://api.example.com", want: false},
		{name: "@ in body", input: "email@example.com", want: false},
		{name: "just @", input: "@", want: false},
		{name: "@ without equals", input: "@name something", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFileVariable(tt.input); got != tt.want {
				t.Errorf("isFileVariable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsQueryStringContinuation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "starts with ?", input: "?param=value", want: true},
		{name: "starts with &", input: "&param=value", want: true},
		{name: "not a continuation", input: "param=value", want: false},
		{name: "spaces before ?", input: "  ?param=value", want: true},
		{name: "spaces before &", input: "  &param=value", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isQueryStringContinuation(tt.input); got != tt.want {
				t.Errorf("isQueryStringContinuation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRequest(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantMethod  string
		wantURL     string
		wantHeaders map[string]string
		wantBody    string
		wantName    string
		wantErr     bool
	}{
		{
			name:       "simple GET",
			input:      "GET https://api.example.com/users",
			wantMethod: "GET",
			wantURL:    "https://api.example.com/users",
		},
		{
			name: "GET with headers",
			input: `GET https://api.example.com/users
Authorization: Bearer token123
Accept: application/json`,
			wantMethod:  "GET",
			wantURL:     "https://api.example.com/users",
			wantHeaders: map[string]string{"Authorization": "Bearer token123", "Accept": "application/json"},
		},
		{
			name: "POST with body",
			input: `POST https://api.example.com/users
Content-Type: application/json

{"name": "John", "email": "john@example.com"}`,
			wantMethod:  "POST",
			wantURL:     "https://api.example.com/users",
			wantHeaders: map[string]string{"Content-Type": "application/json"},
			wantBody:    `{"name": "John", "email": "john@example.com"}`,
		},
		{
			name: "with name metadata",
			input: `# @name getUsers
GET https://api.example.com/users`,
			wantMethod: "GET",
			wantURL:    "https://api.example.com/users",
			wantName:   "getUsers",
		},
		{
			name: "with query string continuation",
			input: `GET https://api.example.com/users
?page=1
&limit=10`,
			wantMethod: "GET",
			wantURL:    "https://api.example.com/users?page=1&limit=10",
		},
		{
			name: "with comments",
			input: `# This is a request to get users
# It fetches all users from the API
GET https://api.example.com/users`,
			wantMethod: "GET",
			wantURL:    "https://api.example.com/users",
		},
		{
			name: "with file variables at top",
			input: `@baseUrl = https://api.example.com

GET {{baseUrl}}/users`,
			wantMethod: "GET",
			wantURL:    "{{baseUrl}}/users",
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "only comments",
			input:   "# Just a comment\n// Another comment",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewHttpRequestParser(tt.input, nil, "")
			got, err := parser.ParseRequest(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.Method != tt.wantMethod {
				t.Errorf("ParseRequest() Method = %v, want %v", got.Method, tt.wantMethod)
			}
			if got.URL != tt.wantURL {
				t.Errorf("ParseRequest() URL = %v, want %v", got.URL, tt.wantURL)
			}
			for k, v := range tt.wantHeaders {
				if got.Headers[k] != v {
					t.Errorf("ParseRequest() Headers[%s] = %v, want %v", k, got.Headers[k], v)
				}
			}
			if tt.wantBody != "" && got.RawBody != tt.wantBody {
				t.Errorf("ParseRequest() RawBody = %v, want %v", got.RawBody, tt.wantBody)
			}
			if tt.wantName != "" && got.Metadata.Name != tt.wantName {
				t.Errorf("ParseRequest() Name = %v, want %v", got.Metadata.Name, tt.wantName)
			}
		})
	}
}

func TestParseAll(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
	}{
		{
			name:      "single request",
			input:     "GET https://api.example.com/users",
			wantCount: 1,
		},
		{
			name: "multiple requests",
			input: `GET https://api.example.com/users

###

POST https://api.example.com/users
Content-Type: application/json

{"name": "John"}

###

DELETE https://api.example.com/users/123`,
			wantCount: 3,
		},
		{
			name: "requests with names",
			input: `# @name getUsers
GET https://api.example.com/users

###

# @name createUser
POST https://api.example.com/users`,
			wantCount: 2,
		},
		{
			name:      "empty input",
			input:     "",
			wantCount: 0,
		},
		{
			name:      "only separators",
			input:     "###\n###\n###",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewHttpRequestParser(tt.input, nil, "")
			got, err := parser.ParseAll()
			if err != nil {
				t.Errorf("ParseAll() error = %v", err)
				return
			}
			if len(got) != tt.wantCount {
				t.Errorf("ParseAll() count = %v, want %v", len(got), tt.wantCount)
			}
		})
	}
}

func TestSplitRequestBlocks(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
	}{
		{
			name:      "single block",
			input:     "GET https://api.example.com",
			wantCount: 1,
		},
		{
			name:      "two blocks",
			input:     "GET https://api.example.com\n###\nPOST https://api.example.com",
			wantCount: 2,
		},
		{
			name:      "three hashes",
			input:     "GET /1\n###\nGET /2",
			wantCount: 2,
		},
		{
			name:      "four hashes",
			input:     "GET /1\n####\nGET /2",
			wantCount: 2,
		},
		{
			name:      "many hashes",
			input:     "GET /1\n##########\nGET /2",
			wantCount: 2,
		},
		{
			name:      "hashes with spaces",
			input:     "GET /1\n###   \nGET /2",
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitRequestBlocks(tt.input)
			if len(got) != tt.wantCount {
				t.Errorf("splitRequestBlocks() count = %v, want %v", len(got), tt.wantCount)
			}
		})
	}
}

func TestFormURLEncodedBody(t *testing.T) {
	input := `POST https://api.example.com/login
Content-Type: application/x-www-form-urlencoded

username=john
&password=secret123
&remember=true`

	parser := NewHttpRequestParser(input, nil, "")
	req, err := parser.ParseRequest(input)
	if err != nil {
		t.Fatalf("ParseRequest() error = %v", err)
	}

	// Form body should have & separators, not newlines
	if !strings.Contains(req.RawBody, "&") {
		t.Errorf("Form body should contain &, got: %v", req.RawBody)
	}
	if strings.Contains(req.RawBody, "\n") {
		t.Errorf("Form body should not contain newlines, got: %v", req.RawBody)
	}
}

func TestGraphQLBody(t *testing.T) {
	input := `POST https://api.example.com/graphql
Content-Type: application/json
X-Request-Type: GraphQL

query GetUser {
  user(id: "123") {
    name
    email
  }
}

{"id": "123"}`

	parser := NewHttpRequestParser(input, nil, "")
	req, err := parser.ParseRequest(input)
	if err != nil {
		t.Fatalf("ParseRequest() error = %v", err)
	}

	// Body should be JSON with query field
	if !strings.Contains(req.RawBody, `"query"`) {
		t.Errorf("GraphQL body should contain query field, got: %v", req.RawBody)
	}
	if !strings.Contains(req.RawBody, `"variables"`) {
		t.Errorf("GraphQL body should contain variables field, got: %v", req.RawBody)
	}
}

func TestRelativeURLWithHostHeader(t *testing.T) {
	input := `GET /users
Host: api.example.com`

	parser := NewHttpRequestParser(input, nil, "")
	req, err := parser.ParseRequest(input)
	if err != nil {
		t.Fatalf("ParseRequest() error = %v", err)
	}

	if req.URL != "http://api.example.com/users" {
		t.Errorf("URL should be absolute, got: %v", req.URL)
	}
}

func TestRelativeURLWithHTTPSHost(t *testing.T) {
	input := `GET /users
Host: api.example.com:443`

	parser := NewHttpRequestParser(input, nil, "")
	req, err := parser.ParseRequest(input)
	if err != nil {
		t.Fatalf("ParseRequest() error = %v", err)
	}

	if req.URL != "https://api.example.com:443/users" {
		t.Errorf("URL should use https for port 443, got: %v", req.URL)
	}
}

func TestGraphQLMutation(t *testing.T) {
	input := `POST https://api.example.com/graphql
Content-Type: application/json
X-Request-Type: GraphQL

mutation CreateUser($input: CreateUserInput!) {
  createUser(input: $input) {
    id
    name
  }
}

{"input": {"name": "John", "email": "john@example.com"}}`

	parser := NewHttpRequestParser(input, nil, "")
	req, err := parser.ParseRequest(input)
	if err != nil {
		t.Fatalf("ParseRequest() error = %v", err)
	}

	// Body should contain query field with mutation
	if !strings.Contains(req.RawBody, `"query"`) {
		t.Errorf("GraphQL body should contain query field, got: %v", req.RawBody)
	}
	// Should contain operationName extracted from mutation
	if !strings.Contains(req.RawBody, `"operationName":"CreateUser"`) {
		t.Errorf("GraphQL body should contain operationName, got: %v", req.RawBody)
	}
}

func TestGraphQLSubscription(t *testing.T) {
	input := `POST https://api.example.com/graphql
Content-Type: application/json
X-Request-Type: GraphQL

subscription OnMessage {
  messageAdded {
    id
    content
  }
}`

	parser := NewHttpRequestParser(input, nil, "")
	req, err := parser.ParseRequest(input)
	if err != nil {
		t.Fatalf("ParseRequest() error = %v", err)
	}

	// Should contain operationName extracted from subscription
	if !strings.Contains(req.RawBody, `"operationName":"OnMessage"`) {
		t.Errorf("GraphQL body should contain operationName, got: %v", req.RawBody)
	}
}

func TestGraphQLAutoDetectByURL(t *testing.T) {
	input := `POST https://api.example.com/graphql
Content-Type: application/json

query GetUsers {
  users {
    id
    name
  }
}`

	parser := NewHttpRequestParser(input, nil, "")
	req, err := parser.ParseRequest(input)
	if err != nil {
		t.Fatalf("ParseRequest() error = %v", err)
	}

	// Body should be JSON with query field (auto-detected by URL)
	if !strings.Contains(req.RawBody, `"query"`) {
		t.Errorf("GraphQL body should contain query field, got: %v", req.RawBody)
	}
}

func TestExtractBoundary(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        string
	}{
		{
			name:        "simple boundary",
			contentType: "multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW",
			want:        "----WebKitFormBoundary7MA4YWxkTrZu0gW",
		},
		{
			name:        "quoted boundary",
			contentType: `multipart/form-data; boundary="----WebKitFormBoundary7MA4YWxkTrZu0gW"`,
			want:        "----WebKitFormBoundary7MA4YWxkTrZu0gW",
		},
		{
			name:        "no boundary",
			contentType: "multipart/form-data",
			want:        "",
		},
		{
			name:        "not multipart",
			contentType: "application/json",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBoundary(tt.contentType)
			if got != tt.want {
				t.Errorf("extractBoundary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseMultipartSection(t *testing.T) {
	tests := []struct {
		name            string
		section         string
		wantName        string
		wantValue       string
		wantFileName    string
		wantContentType string
		wantIsFile      bool
	}{
		{
			name: "simple text field",
			section: `Content-Disposition: form-data; name="username"

john_doe`,
			wantName:  "username",
			wantValue: "john_doe",
		},
		{
			name: "file field",
			section: `Content-Disposition: form-data; name="avatar"; filename="photo.jpg"
Content-Type: image/jpeg

binary content here`,
			wantName:        "avatar",
			wantFileName:    "photo.jpg",
			wantContentType: "image/jpeg",
			wantIsFile:      true,
		},
		{
			name: "field with file reference value",
			section: `Content-Disposition: form-data; name="document"

< ./document.pdf`,
			wantName:   "document",
			wantValue:  "< ./document.pdf",
			wantIsFile: false, // parseMultipartSection doesn't detect file references; that's done in parseMultipartParts
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMultipartSection(tt.section)
			if got.Name != tt.wantName {
				t.Errorf("parseMultipartSection() Name = %v, want %v", got.Name, tt.wantName)
			}
			if tt.wantValue != "" && got.Value != tt.wantValue {
				t.Errorf("parseMultipartSection() Value = %v, want %v", got.Value, tt.wantValue)
			}
			if got.FileName != tt.wantFileName {
				t.Errorf("parseMultipartSection() FileName = %v, want %v", got.FileName, tt.wantFileName)
			}
			if got.ContentType != tt.wantContentType {
				t.Errorf("parseMultipartSection() ContentType = %v, want %v", got.ContentType, tt.wantContentType)
			}
			if got.IsFile != tt.wantIsFile {
				t.Errorf("parseMultipartSection() IsFile = %v, want %v", got.IsFile, tt.wantIsFile)
			}
		})
	}
}

func TestMultipartFormDataParsing(t *testing.T) {
	input := `POST https://api.example.com/upload
Content-Type: multipart/form-data; boundary=----FormBoundary123

------FormBoundary123
Content-Disposition: form-data; name="title"

My Document
------FormBoundary123
Content-Disposition: form-data; name="file"; filename="test.txt"
Content-Type: text/plain

file content here
------FormBoundary123--`

	parser := NewHttpRequestParser(input, nil, "")
	req, err := parser.ParseRequest(input)
	if err != nil {
		t.Fatalf("ParseRequest() error = %v", err)
	}

	if len(req.MultipartParts) != 2 {
		t.Errorf("Expected 2 multipart parts, got %d", len(req.MultipartParts))
		return
	}

	// Check first part (title)
	if req.MultipartParts[0].Name != "title" {
		t.Errorf("First part name = %v, want 'title'", req.MultipartParts[0].Name)
	}
	if req.MultipartParts[0].Value != "My Document" {
		t.Errorf("First part value = %v, want 'My Document'", req.MultipartParts[0].Value)
	}

	// Check second part (file)
	if req.MultipartParts[1].Name != "file" {
		t.Errorf("Second part name = %v, want 'file'", req.MultipartParts[1].Name)
	}
	if req.MultipartParts[1].FileName != "test.txt" {
		t.Errorf("Second part filename = %v, want 'test.txt'", req.MultipartParts[1].FileName)
	}
	if !req.MultipartParts[1].IsFile {
		t.Errorf("Second part should be a file")
	}
}

func TestCreateGraphQLBody(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		wantQuery     bool
		wantOpName    string
		wantVariables bool
	}{
		{
			name: "simple query",
			body: `query GetUser {
  user(id: "1") {
    name
  }
}`,
			wantQuery:     true,
			wantOpName:    "GetUser",
			wantVariables: true,
		},
		{
			name: "mutation",
			body: `mutation CreateUser {
  createUser(name: "John") {
    id
  }
}`,
			wantQuery:     true,
			wantOpName:    "CreateUser",
			wantVariables: true,
		},
		{
			name: "subscription",
			body: `subscription OnUserCreated {
  userCreated {
    id
  }
}`,
			wantQuery:     true,
			wantOpName:    "OnUserCreated",
			wantVariables: true,
		},
		{
			name: "query with variables",
			body: `query GetUser($id: ID!) {
  user(id: $id) {
    name
  }
}

{"id": "123"}`,
			wantQuery:     true,
			wantOpName:    "GetUser",
			wantVariables: true,
		},
		{
			name: "anonymous query",
			body: `{
  users {
    name
  }
}`,
			wantQuery:     true,
			wantOpName:    "",
			wantVariables: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createGraphQLBody(tt.body)
			if tt.wantQuery && !strings.Contains(result, `"query"`) {
				t.Errorf("createGraphQLBody() should contain 'query', got: %v", result)
			}
			if tt.wantOpName != "" && !strings.Contains(result, `"operationName":"`+tt.wantOpName+`"`) {
				t.Errorf("createGraphQLBody() should contain operationName '%s', got: %v", tt.wantOpName, result)
			}
			if tt.wantOpName == "" && strings.Contains(result, `"operationName"`) {
				t.Errorf("createGraphQLBody() should not contain operationName for anonymous query, got: %v", result)
			}
			if tt.wantVariables && !strings.Contains(result, `"variables"`) {
				t.Errorf("createGraphQLBody() should contain 'variables', got: %v", result)
			}
		})
	}
}
