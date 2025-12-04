package postman

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Sample .http file content for export testing
const sampleHttpFile = `@baseUrl = https://api.example.com
@userId = 123

###

# @name GetUser
GET {{baseUrl}}/users/{{userId}}
Accept: application/json

###

# @name CreateUser
# @note Creates a new user in the system
POST {{baseUrl}}/users
Content-Type: application/json

{
  "name": "John Doe",
  "email": "john@example.com"
}

###

# @name UpdateUser
PUT {{baseUrl}}/users/{{userId}}
Content-Type: application/json

{
  "name": "Jane Doe"
}

###

# @name DeleteUser
DELETE {{baseUrl}}/users/{{userId}}
`

const sampleHttpWithAuth = `# @name BasicAuthRequest
GET https://api.example.com/protected
Authorization: Basic testuser:testpass

###

# @name BearerAuthRequest
GET https://api.example.com/secure
Authorization: Bearer my-jwt-token

###

# @name DigestAuthRequest
GET https://api.example.com/digest
Authorization: Digest admin password123
`

const sampleHttpWithFormData = `# @name UrlEncodedForm
POST https://api.example.com/login
Content-Type: application/x-www-form-urlencoded

username=john&password=secret123

###

# @name JsonBody
POST https://api.example.com/api/data
Content-Type: application/json

{
  "data": "test",
  "enabled": true
}
`

const sampleHttpWithScripts = `# @name ScriptedRequest
< {%
console.log('Pre-request script');
pm.environment.set('timestamp', Date.now());
%}
GET https://api.example.com/test

> {%
pm.test('Status code is 200', function() {
    pm.response.to.have.status(200);
});
%}
`

func TestExportBasic(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-export-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write sample .http file
	httpPath := filepath.Join(tmpDir, "api.http")
	if err := os.WriteFile(httpPath, []byte(sampleHttpFile), 0644); err != nil {
		t.Fatalf("Failed to write .http file: %v", err)
	}

	// Export
	outputPath := filepath.Join(tmpDir, "collection.json")
	opts := DefaultExportOptions()
	opts.CollectionName = "Test API"
	opts.CollectionDescription = "Test API collection"

	result, err := Export([]string{httpPath}, outputPath, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Verify result
	if result.RequestsCount != 4 {
		t.Errorf("Expected 4 requests, got %d", result.RequestsCount)
	}

	if result.VariablesCount != 2 {
		t.Errorf("Expected 2 variables, got %d", result.VariablesCount)
	}

	// Parse the exported collection
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var collection Collection
	if err := json.Unmarshal(data, &collection); err != nil {
		t.Fatalf("Failed to parse exported collection: %v", err)
	}

	// Verify collection structure
	if collection.Info.Name != "Test API" {
		t.Errorf("Expected collection name 'Test API', got '%s'", collection.Info.Name)
	}

	if collection.Info.Schema != SchemaV21 {
		t.Errorf("Expected schema '%s', got '%s'", SchemaV21, collection.Info.Schema)
	}

	if len(collection.Item) != 4 {
		t.Errorf("Expected 4 items, got %d", len(collection.Item))
	}

	// Verify first request
	getUser := collection.Item[0]
	if getUser.Name != "GetUser" {
		t.Errorf("Expected first request name 'GetUser', got '%s'", getUser.Name)
	}

	if getUser.Request.Method != "GET" {
		t.Errorf("Expected GET method, got '%s'", getUser.Request.Method)
	}

	// Verify variables
	if len(collection.Variable) != 2 {
		t.Errorf("Expected 2 collection variables, got %d", len(collection.Variable))
	}

	varMap := make(map[string]string)
	for _, v := range collection.Variable {
		varMap[v.Key] = v.GetValue()
	}

	if varMap["baseUrl"] != "https://api.example.com" {
		t.Errorf("Unexpected baseUrl value: %s", varMap["baseUrl"])
	}

	if varMap["userId"] != "123" {
		t.Errorf("Unexpected userId value: %s", varMap["userId"])
	}
}

func TestExportWithAuth(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-export-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	httpPath := filepath.Join(tmpDir, "auth.http")
	if err := os.WriteFile(httpPath, []byte(sampleHttpWithAuth), 0644); err != nil {
		t.Fatalf("Failed to write .http file: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "collection.json")
	opts := DefaultExportOptions()

	result, err := Export([]string{httpPath}, outputPath, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if result.RequestsCount != 3 {
		t.Errorf("Expected 3 requests, got %d", result.RequestsCount)
	}

	// Parse the exported collection
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var collection Collection
	if err := json.Unmarshal(data, &collection); err != nil {
		t.Fatalf("Failed to parse exported collection: %v", err)
	}

	// Verify Basic auth
	basicReq := collection.Item[0]
	if basicReq.Request.Auth == nil {
		t.Error("Expected Basic auth to be set")
	} else if basicReq.Request.Auth.Type != "basic" {
		t.Errorf("Expected auth type 'basic', got '%s'", basicReq.Request.Auth.Type)
	}

	// Verify Bearer auth
	bearerReq := collection.Item[1]
	if bearerReq.Request.Auth == nil {
		t.Error("Expected Bearer auth to be set")
	} else if bearerReq.Request.Auth.Type != "bearer" {
		t.Errorf("Expected auth type 'bearer', got '%s'", bearerReq.Request.Auth.Type)
	}

	// Verify Digest auth
	digestReq := collection.Item[2]
	if digestReq.Request.Auth == nil {
		t.Error("Expected Digest auth to be set")
	} else if digestReq.Request.Auth.Type != "digest" {
		t.Errorf("Expected auth type 'digest', got '%s'", digestReq.Request.Auth.Type)
	}
}

func TestExportWithFormData(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-export-form-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	httpPath := filepath.Join(tmpDir, "form.http")
	if err := os.WriteFile(httpPath, []byte(sampleHttpWithFormData), 0644); err != nil {
		t.Fatalf("Failed to write .http file: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "collection.json")
	opts := DefaultExportOptions()

	result, err := Export([]string{httpPath}, outputPath, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if result.RequestsCount != 2 {
		t.Errorf("Expected 2 requests, got %d", result.RequestsCount)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var collection Collection
	if err := json.Unmarshal(data, &collection); err != nil {
		t.Fatalf("Failed to parse exported collection: %v", err)
	}

	// Verify URL-encoded form
	formReq := collection.Item[0]
	if formReq.Request.Body == nil {
		t.Error("Expected body to be set")
	} else if formReq.Request.Body.Mode != "urlencoded" {
		t.Errorf("Expected body mode 'urlencoded', got '%s'", formReq.Request.Body.Mode)
	}

	// Verify JSON body
	jsonReq := collection.Item[1]
	if jsonReq.Request.Body == nil {
		t.Error("Expected body to be set")
	} else if jsonReq.Request.Body.Mode != "raw" {
		t.Errorf("Expected body mode 'raw', got '%s'", jsonReq.Request.Body.Mode)
	}
}

func TestExportWithScripts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-export-scripts-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	httpPath := filepath.Join(tmpDir, "scripts.http")
	if err := os.WriteFile(httpPath, []byte(sampleHttpWithScripts), 0644); err != nil {
		t.Fatalf("Failed to write .http file: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "collection.json")
	opts := DefaultExportOptions()
	opts.IncludeScripts = true

	result, err := Export([]string{httpPath}, outputPath, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if result.RequestsCount != 1 {
		t.Errorf("Expected 1 request, got %d", result.RequestsCount)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var collection Collection
	if err := json.Unmarshal(data, &collection); err != nil {
		t.Fatalf("Failed to parse exported collection: %v", err)
	}

	req := collection.Item[0]
	if len(req.Event) < 2 {
		t.Errorf("Expected at least 2 events, got %d", len(req.Event))
	}

	// Find pre-request event
	var preReq, test *Event
	for i := range req.Event {
		if req.Event[i].Listen == "prerequest" {
			preReq = &req.Event[i]
		} else if req.Event[i].Listen == "test" {
			test = &req.Event[i]
		}
	}

	if preReq == nil {
		t.Error("Expected prerequest event")
	}
	if test == nil {
		t.Error("Expected test event")
	}
}

func TestExportWithoutScripts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-export-noscripts-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	httpPath := filepath.Join(tmpDir, "scripts.http")
	if err := os.WriteFile(httpPath, []byte(sampleHttpWithScripts), 0644); err != nil {
		t.Fatalf("Failed to write .http file: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "collection.json")
	opts := DefaultExportOptions()
	opts.IncludeScripts = false

	_, err = Export([]string{httpPath}, outputPath, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var collection Collection
	if err := json.Unmarshal(data, &collection); err != nil {
		t.Fatalf("Failed to parse exported collection: %v", err)
	}

	req := collection.Item[0]
	if len(req.Event) != 0 {
		t.Errorf("Expected no events when scripts disabled, got %d", len(req.Event))
	}
}

func TestExportWithoutVariables(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-export-novars-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	httpPath := filepath.Join(tmpDir, "api.http")
	if err := os.WriteFile(httpPath, []byte(sampleHttpFile), 0644); err != nil {
		t.Fatalf("Failed to write .http file: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "collection.json")
	opts := DefaultExportOptions()
	opts.IncludeVariables = false

	result, err := Export([]string{httpPath}, outputPath, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if result.VariablesCount != 0 {
		t.Errorf("Expected 0 variables when disabled, got %d", result.VariablesCount)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var collection Collection
	if err := json.Unmarshal(data, &collection); err != nil {
		t.Fatalf("Failed to parse exported collection: %v", err)
	}

	if len(collection.Variable) != 0 {
		t.Errorf("Expected no variables in collection, got %d", len(collection.Variable))
	}
}

func TestExportMultipleFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-export-multi-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create first file
	httpPath1 := filepath.Join(tmpDir, "users.http")
	if err := os.WriteFile(httpPath1, []byte(sampleHttpFile), 0644); err != nil {
		t.Fatalf("Failed to write first .http file: %v", err)
	}

	// Create second file
	httpPath2 := filepath.Join(tmpDir, "auth.http")
	if err := os.WriteFile(httpPath2, []byte(sampleHttpWithAuth), 0644); err != nil {
		t.Fatalf("Failed to write second .http file: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "collection.json")
	opts := DefaultExportOptions()
	opts.CollectionName = "Combined API"

	result, err := Export([]string{httpPath1, httpPath2}, outputPath, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// 4 from first file + 3 from second file
	if result.RequestsCount != 7 {
		t.Errorf("Expected 7 requests, got %d", result.RequestsCount)
	}
}

func TestExportMinified(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-export-minify-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	httpPath := filepath.Join(tmpDir, "api.http")
	if err := os.WriteFile(httpPath, []byte(sampleHttpFile), 0644); err != nil {
		t.Fatalf("Failed to write .http file: %v", err)
	}

	// Export minified
	minifiedPath := filepath.Join(tmpDir, "collection-min.json")
	optsMin := DefaultExportOptions()
	optsMin.PrettyPrint = false

	_, err = Export([]string{httpPath}, minifiedPath, optsMin)
	if err != nil {
		t.Fatalf("Export minified failed: %v", err)
	}

	// Export pretty
	prettyPath := filepath.Join(tmpDir, "collection-pretty.json")
	optsPretty := DefaultExportOptions()
	optsPretty.PrettyPrint = true

	_, err = Export([]string{httpPath}, prettyPath, optsPretty)
	if err != nil {
		t.Fatalf("Export pretty failed: %v", err)
	}

	minInfo, _ := os.Stat(minifiedPath)
	prettyInfo, _ := os.Stat(prettyPath)

	// Minified should be smaller
	if minInfo.Size() >= prettyInfo.Size() {
		t.Errorf("Minified file should be smaller. Minified: %d, Pretty: %d", minInfo.Size(), prettyInfo.Size())
	}
}

func TestExportToCollection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-export-struct-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	httpPath := filepath.Join(tmpDir, "api.http")
	if err := os.WriteFile(httpPath, []byte(sampleHttpFile), 0644); err != nil {
		t.Fatalf("Failed to write .http file: %v", err)
	}

	opts := DefaultExportOptions()
	opts.CollectionName = "Direct Export Test"

	collection, err := ExportToCollection([]string{httpPath}, opts)
	if err != nil {
		t.Fatalf("ExportToCollection failed: %v", err)
	}

	if collection.Info.Name != "Direct Export Test" {
		t.Errorf("Expected collection name 'Direct Export Test', got '%s'", collection.Info.Name)
	}

	if len(collection.Item) != 4 {
		t.Errorf("Expected 4 items, got %d", len(collection.Item))
	}
}

func TestParseAuthHeader(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string // expected auth type
	}{
		{"Basic Auth", "Basic user:pass", "basic"},
		{"Bearer Token", "Bearer my-token", "bearer"},
		{"Digest Auth", "Digest admin secret", "digest"},
		{"AWS Signature", "AWS accessKey secretKey region:us-east-1", "awsv4"},
		{"Unknown", "Custom header", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := parseAuthHeader(tt.header)
			if tt.expected == "" {
				if auth != nil {
					t.Errorf("Expected nil auth, got %+v", auth)
				}
			} else {
				if auth == nil {
					t.Error("Expected non-nil auth")
				} else if auth.Type != tt.expected {
					t.Errorf("Expected auth type '%s', got '%s'", tt.expected, auth.Type)
				}
			}
		})
	}
}

func TestGenerateRequestName(t *testing.T) {
	tests := []struct {
		method   string
		url      string
		expected string
	}{
		{"GET", "https://api.example.com/users", "GET users"},
		{"POST", "https://api.example.com/users/123", "POST 123"},
		{"DELETE", "https://api.example.com/", "DELETE api.example.com"},
		{"GET", "invalid-url", "GET invalid-url"}, // Invalid URL is used as-is
	}

	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.url, func(t *testing.T) {
			result := generateRequestName(tt.method, tt.url)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestParseURLWithVariables(t *testing.T) {
	tests := []struct {
		name         string
		rawURL       string
		expectedHost any
		expectedPath []string
		expectedRaw  string
	}{
		{
			name:         "Variable host with path",
			rawURL:       "{{baseUrl}}/users/1/albums",
			expectedHost: "{{baseUrl}}",
			expectedPath: []string{"users", "1", "albums"},
			expectedRaw:  "{{baseUrl}}/users/1/albums",
		},
		{
			name:         "Variable host with single path segment",
			rawURL:       "{{baseUrl}}/posts",
			expectedHost: "{{baseUrl}}",
			expectedPath: []string{"posts"},
			expectedRaw:  "{{baseUrl}}/posts",
		},
		{
			name:         "Variable host only",
			rawURL:       "{{baseUrl}}",
			expectedHost: "{{baseUrl}}",
			expectedPath: nil,
			expectedRaw:  "{{baseUrl}}",
		},
		{
			name:         "Variable host with query params",
			rawURL:       "{{baseUrl}}/users?userId=1&active=true",
			expectedHost: "{{baseUrl}}",
			expectedPath: []string{"users"},
			expectedRaw:  "{{baseUrl}}/users?userId=1&active=true",
		},
		{
			name:         "Regular URL still works",
			rawURL:       "https://api.example.com/users",
			expectedHost: []string{"api", "example", "com"},
			expectedPath: []string{"users"},
			expectedRaw:  "https://api.example.com/users",
		},
		{
			name:         "Variable host with nested path",
			rawURL:       "{{baseUrl}}/api/v1/users/123/posts",
			expectedHost: "{{baseUrl}}",
			expectedPath: []string{"api", "v1", "users", "123", "posts"},
			expectedRaw:  "{{baseUrl}}/api/v1/users/123/posts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseURL(tt.rawURL)

			if result.Raw != tt.expectedRaw {
				t.Errorf("Expected raw '%s', got '%s'", tt.expectedRaw, result.Raw)
			}

			// Check host
			switch expected := tt.expectedHost.(type) {
			case string:
				if hostStr, ok := result.Host.(string); ok {
					if hostStr != expected {
						t.Errorf("Expected host '%s', got '%s'", expected, hostStr)
					}
				} else {
					t.Errorf("Expected host to be string '%s', got %v", expected, result.Host)
				}
			case []string:
				if hostArr, ok := result.Host.([]string); ok {
					if len(hostArr) != len(expected) {
						t.Errorf("Expected host %v, got %v", expected, hostArr)
					} else {
						for i, v := range expected {
							if hostArr[i] != v {
								t.Errorf("Expected host[%d] = '%s', got '%s'", i, v, hostArr[i])
							}
						}
					}
				} else {
					t.Errorf("Expected host to be []string %v, got %v", expected, result.Host)
				}
			}

			// Check path
			if tt.expectedPath == nil {
				if result.Path != nil {
					t.Errorf("Expected nil path, got %v", result.Path)
				}
			} else {
				pathArr, ok := result.Path.([]string)
				if !ok {
					t.Errorf("Expected path to be []string, got %v", result.Path)
				} else if len(pathArr) != len(tt.expectedPath) {
					t.Errorf("Expected path %v, got %v", tt.expectedPath, pathArr)
				} else {
					for i, v := range tt.expectedPath {
						if pathArr[i] != v {
							t.Errorf("Expected path[%d] = '%s', got '%s'", i, v, pathArr[i])
						}
					}
				}
			}
		})
	}
}

func TestParseURLQueryParams(t *testing.T) {
	result := parseURL("{{baseUrl}}/users?userId=1&status=active")

	if len(result.Query) != 2 {
		t.Fatalf("Expected 2 query params, got %d", len(result.Query))
	}

	queryMap := make(map[string]string)
	for _, q := range result.Query {
		if q.Key != nil && q.Value != nil {
			queryMap[*q.Key] = *q.Value
		}
	}

	if queryMap["userId"] != "1" {
		t.Errorf("Expected userId=1, got userId=%s", queryMap["userId"])
	}
	if queryMap["status"] != "active" {
		t.Errorf("Expected status=active, got status=%s", queryMap["status"])
	}
}

func TestConvertScriptToPostman(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Convert client.test to pm.test",
			input:    `client.test("Status is 200", function() {`,
			expected: `pm.test("Status is 200", function() {`,
		},
		{
			name:     "Convert client.assert with message",
			input:    `client.assert(response.status === 200, "Expected status 200");`,
			expected: `pm.expect(pm.response.code === 200, "Expected status 200").to.be.true;`,
		},
		{
			name:     "Convert client.assert without message",
			input:    `client.assert(response.status === 200);`,
			expected: `pm.expect(pm.response.code === 200).to.be.true;`,
		},
		{
			name:     "Convert client.log",
			input:    `client.log("Hello world");`,
			expected: `console.log("Hello world");`,
		},
		{
			name:     "Convert client.global.set",
			input:    `client.global.set("key", "value");`,
			expected: `pm.globals.set("key", "value");`,
		},
		{
			name:     "Convert client.global.get",
			input:    `var x = client.global.get("key");`,
			expected: `var x = pm.globals.get("key");`,
		},
		{
			name:     "Convert response.status",
			input:    `if (response.status === 200) {`,
			expected: `if (pm.response.code === 200) {`,
		},
		{
			name:     "Convert response.body property access",
			input:    `var id = response.body.id;`,
			expected: `var id = pm.response.json().id;`,
		},
		{
			name:     "Convert response.body in parentheses",
			input:    `client.assert(response.body)`,
			expected: `pm.expect(pm.response.json()).to.be.true`,
		},
		{
			name:     "Convert response.headers.valueOf",
			input:    `var ct = response.headers.valueOf("Content-Type");`,
			expected: `var ct = pm.response.headers.get("Content-Type");`,
		},
		{
			name:     "Convert $uuid()",
			input:    `var id = $uuid();`,
			expected: `var id = pm.variables.replaceIn("{{$guid}}");`,
		},
		{
			name:     "Convert $timestamp()",
			input:    `var ts = $timestamp();`,
			expected: `var ts = Date.now();`,
		},
		{
			name:     "Convert $isoTimestamp()",
			input:    `var iso = $isoTimestamp();`,
			expected: `var iso = new Date().toISOString();`,
		},
		{
			name:     "Convert $randomInt(min, max)",
			input:    `var num = $randomInt(1, 100);`,
			expected: `var num = (Math.floor(Math.random() * (100 - 1 + 1)) + 1);`,
		},
		{
			name:     "Convert $randomString(length)",
			input:    `var str = $randomString(10);`,
			expected: `var str = Array(10).fill(0).map(() => Math.random().toString(36).charAt(2)).join('');`,
		},
		{
			name:     "Convert $base64()",
			input:    `var encoded = $base64(data);`,
			expected: `var encoded = btoa(data);`,
		},
		{
			name:     "Convert $base64Decode()",
			input:    `var decoded = $base64Decode(encoded);`,
			expected: `var decoded = atob(encoded);`,
		},
		{
			name:     "Convert $md5()",
			input:    `var hash = $md5(data);`,
			expected: `var hash = CryptoJS.MD5(data).toString();`,
		},
		{
			name:     "Convert $sha256()",
			input:    `var hash = $sha256(data);`,
			expected: `var hash = CryptoJS.SHA256(data).toString();`,
		},
		{
			name:     "Convert $sha512()",
			input:    `var hash = $sha512(data);`,
			expected: `var hash = CryptoJS.SHA512(data).toString();`,
		},
		{
			name: "Convert complex script",
			input: `client.test("Status is 200", function() {
    client.assert(response.status === 200, "Expected status 200");
});
client.log("Post title: " + response.body.title);
client.global.set("postId", response.body.id);`,
			expected: `pm.test("Status is 200", function() {
    pm.expect(pm.response.code === 200, "Expected status 200").to.be.true;
});
console.log("Post title: " + pm.response.json().title);
pm.globals.set("postId", pm.response.json().id);`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertScriptToPostman(tt.input)
			if result != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestExportWithClientScripts(t *testing.T) {
	// Test that client.* scripts are properly converted to pm.* in export
	tmpDir, err := os.MkdirTemp("", "postman-export-client-scripts-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	httpContent := `# @name TestRequest
GET https://api.example.com/test

> {%
client.test("Status is 200", function() {
    client.assert(response.status === 200, "Expected status 200");
});
client.log("Response: " + response.body.message);
client.global.set("testId", response.body.id);
%}
`

	httpPath := filepath.Join(tmpDir, "client-scripts.http")
	if err := os.WriteFile(httpPath, []byte(httpContent), 0644); err != nil {
		t.Fatalf("Failed to write .http file: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "collection.json")
	opts := DefaultExportOptions()
	opts.IncludeScripts = true

	_, err = Export([]string{httpPath}, outputPath, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var collection Collection
	if err := json.Unmarshal(data, &collection); err != nil {
		t.Fatalf("Failed to parse exported collection: %v", err)
	}

	req := collection.Item[0]
	if len(req.Event) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(req.Event))
	}

	testEvent := req.Event[0]
	if testEvent.Listen != "test" {
		t.Errorf("Expected 'test' event, got '%s'", testEvent.Listen)
	}

	// Check that script was converted
	scriptContent := testEvent.Script.GetExec()

	// Verify conversions happened
	if !contains(scriptContent, "pm.test(") {
		t.Error("Expected pm.test() in converted script")
	}
	if !contains(scriptContent, "pm.expect(") {
		t.Error("Expected pm.expect() in converted script")
	}
	if !contains(scriptContent, "pm.response.code") {
		t.Error("Expected pm.response.code in converted script")
	}
	if !contains(scriptContent, "console.log(") {
		t.Error("Expected console.log() in converted script")
	}
	if !contains(scriptContent, "pm.globals.set(") {
		t.Error("Expected pm.globals.set() in converted script")
	}
	if !contains(scriptContent, "pm.response.json()") {
		t.Error("Expected pm.response.json() in converted script")
	}

	// Verify old syntax is NOT present
	if contains(scriptContent, "client.test(") {
		t.Error("Script should not contain client.test()")
	}
	if contains(scriptContent, "client.assert(") {
		t.Error("Script should not contain client.assert()")
	}
	if contains(scriptContent, "client.log(") {
		t.Error("Script should not contain client.log()")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestExportVariableConflicts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-export-conflicts-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create first file with variables
	httpContent1 := `@baseUrl = https://api.example.com
@timeout = 30

# @name GetUsers
GET {{baseUrl}}/users
`

	// Create second file with conflicting variables
	httpContent2 := `@baseUrl = https://api.staging.com
@timeout = 60
@newVar = value

# @name GetPosts
GET {{baseUrl}}/posts
`

	httpPath1 := filepath.Join(tmpDir, "users.http")
	if err := os.WriteFile(httpPath1, []byte(httpContent1), 0644); err != nil {
		t.Fatalf("Failed to write first .http file: %v", err)
	}

	httpPath2 := filepath.Join(tmpDir, "posts.http")
	if err := os.WriteFile(httpPath2, []byte(httpContent2), 0644); err != nil {
		t.Fatalf("Failed to write second .http file: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "collection.json")
	opts := DefaultExportOptions()
	opts.IncludeVariables = true

	result, err := Export([]string{httpPath1, httpPath2}, outputPath, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Should have 3 unique variables (baseUrl, timeout, newVar)
	if result.VariablesCount != 3 {
		t.Errorf("Expected 3 variables, got %d", result.VariablesCount)
	}

	// Should have 2 conflicts (baseUrl and timeout have different values)
	if len(result.VariableConflicts) != 2 {
		t.Errorf("Expected 2 variable conflicts, got %d", len(result.VariableConflicts))
	}

	// Check conflict details
	conflictMap := make(map[string]VariableConflict)
	for _, c := range result.VariableConflicts {
		conflictMap[c.Name] = c
	}

	// Check baseUrl conflict
	if baseUrlConflict, ok := conflictMap["baseUrl"]; ok {
		if baseUrlConflict.UsedValue != "https://api.example.com" {
			t.Errorf("Expected baseUrl to use first file value, got %s", baseUrlConflict.UsedValue)
		}
		if len(baseUrlConflict.Files) != 2 {
			t.Errorf("Expected 2 files in conflict, got %d", len(baseUrlConflict.Files))
		}
		if len(baseUrlConflict.Values) != 2 {
			t.Errorf("Expected 2 values in conflict, got %d", len(baseUrlConflict.Values))
		}
	} else {
		t.Error("Expected baseUrl conflict")
	}

	// Check timeout conflict
	if timeoutConflict, ok := conflictMap["timeout"]; ok {
		if timeoutConflict.UsedValue != "30" {
			t.Errorf("Expected timeout to use first file value '30', got %s", timeoutConflict.UsedValue)
		}
	} else {
		t.Error("Expected timeout conflict")
	}

	// Verify the collection has the first file's values
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var collection Collection
	if err := json.Unmarshal(data, &collection); err != nil {
		t.Fatalf("Failed to parse exported collection: %v", err)
	}

	varMap := make(map[string]string)
	for _, v := range collection.Variable {
		varMap[v.Key] = v.GetValue()
	}

	if varMap["baseUrl"] != "https://api.example.com" {
		t.Errorf("Expected baseUrl from first file, got %s", varMap["baseUrl"])
	}
	if varMap["timeout"] != "30" {
		t.Errorf("Expected timeout from first file, got %s", varMap["timeout"])
	}
	if varMap["newVar"] != "value" {
		t.Errorf("Expected newVar value, got %s", varMap["newVar"])
	}
}

func TestExportNoVariableConflicts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-export-no-conflicts-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create files with same variable values (no conflict)
	httpContent1 := `@baseUrl = https://api.example.com

# @name GetUsers
GET {{baseUrl}}/users
`

	httpContent2 := `@baseUrl = https://api.example.com
@timeout = 30

# @name GetPosts
GET {{baseUrl}}/posts
`

	httpPath1 := filepath.Join(tmpDir, "users.http")
	if err := os.WriteFile(httpPath1, []byte(httpContent1), 0644); err != nil {
		t.Fatalf("Failed to write first .http file: %v", err)
	}

	httpPath2 := filepath.Join(tmpDir, "posts.http")
	if err := os.WriteFile(httpPath2, []byte(httpContent2), 0644); err != nil {
		t.Fatalf("Failed to write second .http file: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "collection.json")
	opts := DefaultExportOptions()

	result, err := Export([]string{httpPath1, httpPath2}, outputPath, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// No conflicts since baseUrl has same value in both files
	if len(result.VariableConflicts) != 0 {
		t.Errorf("Expected 0 variable conflicts, got %d", len(result.VariableConflicts))
	}
}

func TestExportVariableConflictsDisabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-export-conflicts-disabled-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create files with conflicting variables
	httpContent1 := `@baseUrl = https://api.example.com

# @name GetUsers
GET {{baseUrl}}/users
`

	httpContent2 := `@baseUrl = https://api.staging.com

# @name GetPosts
GET {{baseUrl}}/posts
`

	httpPath1 := filepath.Join(tmpDir, "users.http")
	if err := os.WriteFile(httpPath1, []byte(httpContent1), 0644); err != nil {
		t.Fatalf("Failed to write first .http file: %v", err)
	}

	httpPath2 := filepath.Join(tmpDir, "posts.http")
	if err := os.WriteFile(httpPath2, []byte(httpContent2), 0644); err != nil {
		t.Fatalf("Failed to write second .http file: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "collection.json")
	opts := DefaultExportOptions()
	opts.IncludeVariables = false // Disable variables

	result, err := Export([]string{httpPath1, httpPath2}, outputPath, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// No conflicts when variables are disabled
	if len(result.VariableConflicts) != 0 {
		t.Errorf("Expected 0 variable conflicts when disabled, got %d", len(result.VariableConflicts))
	}

	if result.VariablesCount != 0 {
		t.Errorf("Expected 0 variables when disabled, got %d", result.VariablesCount)
	}
}

func TestRoundTrip(t *testing.T) {
	// Test that import -> export -> import produces equivalent results
	tmpDir, err := os.MkdirTemp("", "postman-roundtrip-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Start with a Postman collection
	collectionPath := filepath.Join(tmpDir, "original.json")
	if err := os.WriteFile(collectionPath, []byte(samplePostmanCollection), 0644); err != nil {
		t.Fatalf("Failed to write original collection: %v", err)
	}

	// Import to .http files
	importOpts := DefaultImportOptions()
	importOpts.OutputDir = tmpDir
	importOpts.SingleFile = true

	importResult, err := Import(collectionPath, importOpts)
	if err != nil {
		t.Fatalf("Initial import failed: %v", err)
	}

	// Export back to Postman collection
	exportPath := filepath.Join(tmpDir, "exported.json")
	exportOpts := DefaultExportOptions()
	exportOpts.CollectionName = "Test Collection"

	exportResult, err := Export(importResult.FilesCreated, exportPath, exportOpts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Verify request count is preserved
	if exportResult.RequestsCount != importResult.RequestsCount {
		t.Errorf("Request count mismatch: imported %d, exported %d",
			importResult.RequestsCount, exportResult.RequestsCount)
	}

	// Parse and verify the exported collection has valid structure
	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("Failed to read exported file: %v", err)
	}

	var collection Collection
	if err := json.Unmarshal(data, &collection); err != nil {
		t.Fatalf("Failed to parse exported collection: %v", err)
	}

	// Verify schema
	if collection.Info.Schema != SchemaV21 {
		t.Errorf("Expected schema '%s', got '%s'", SchemaV21, collection.Info.Schema)
	}
}
