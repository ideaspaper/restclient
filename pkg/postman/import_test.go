package postman

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Sample Postman Collection v2.1.0 for testing
const samplePostmanCollection = `{
	"info": {
		"_postman_id": "test-collection-id",
		"name": "Test Collection",
		"description": "A test collection for import testing",
		"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
	},
	"item": [
		{
			"name": "Get Users",
			"request": {
				"method": "GET",
				"header": [
					{
						"key": "Accept",
						"value": "application/json"
					}
				],
				"url": {
					"raw": "{{baseUrl}}/users?page=1&limit=10",
					"host": ["{{baseUrl}}"],
					"path": ["users"],
					"query": [
						{"key": "page", "value": "1"},
						{"key": "limit", "value": "10"}
					]
				}
			}
		},
		{
			"name": "Create User",
			"request": {
				"method": "POST",
				"header": [
					{
						"key": "Content-Type",
						"value": "application/json"
					}
				],
				"body": {
					"mode": "raw",
					"raw": "{\"name\": \"John Doe\", \"email\": \"john@example.com\"}"
				},
				"url": {
					"raw": "{{baseUrl}}/users",
					"host": ["{{baseUrl}}"],
					"path": ["users"]
				}
			}
		},
		{
			"name": "User Operations",
			"item": [
				{
					"name": "Update User",
					"request": {
						"method": "PUT",
						"header": [
							{
								"key": "Content-Type",
								"value": "application/json"
							}
						],
						"body": {
							"mode": "raw",
							"raw": "{\"name\": \"Jane Doe\"}"
						},
						"url": {
							"raw": "{{baseUrl}}/users/{{userId}}",
							"host": ["{{baseUrl}}"],
							"path": ["users", "{{userId}}"]
						}
					}
				},
				{
					"name": "Delete User",
					"request": {
						"method": "DELETE",
						"url": {
							"raw": "{{baseUrl}}/users/{{userId}}",
							"host": ["{{baseUrl}}"],
							"path": ["users", "{{userId}}"]
						}
					}
				}
			]
		}
	],
	"variable": [
		{
			"key": "baseUrl",
			"value": "https://api.example.com"
		},
		{
			"key": "userId",
			"value": "123"
		}
	]
}`

const sampleCollectionWithAuth = `{
	"info": {
		"name": "Auth Test Collection",
		"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
	},
	"item": [
		{
			"name": "Basic Auth Request",
			"request": {
				"method": "GET",
				"url": "https://api.example.com/protected",
				"auth": {
					"type": "basic",
					"basic": [
						{"key": "username", "value": "testuser", "type": "string"},
						{"key": "password", "value": "testpass", "type": "string"}
					]
				}
			}
		},
		{
			"name": "Bearer Token Request",
			"request": {
				"method": "GET",
				"url": "https://api.example.com/secure",
				"auth": {
					"type": "bearer",
					"bearer": [
						{"key": "token", "value": "{{accessToken}}", "type": "string"}
					]
				}
			}
		}
	]
}`

const sampleCollectionWithScripts = `{
	"info": {
		"name": "Scripts Test Collection",
		"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
	},
	"item": [
		{
			"name": "Request With Scripts",
			"event": [
				{
					"listen": "prerequest",
					"script": {
						"type": "text/javascript",
						"exec": [
							"console.log('Pre-request script');",
							"pm.environment.set('timestamp', Date.now());"
						]
					}
				},
				{
					"listen": "test",
					"script": {
						"type": "text/javascript",
						"exec": [
							"pm.test('Status code is 200', function() {",
							"    pm.response.to.have.status(200);",
							"});"
						]
					}
				}
			],
			"request": {
				"method": "GET",
				"url": "https://api.example.com/test"
			}
		}
	]
}`

const sampleCollectionWithFormData = `{
	"info": {
		"name": "Form Data Collection",
		"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
	},
	"item": [
		{
			"name": "URL Encoded Form",
			"request": {
				"method": "POST",
				"header": [
					{
						"key": "Content-Type",
						"value": "application/x-www-form-urlencoded"
					}
				],
				"body": {
					"mode": "urlencoded",
					"urlencoded": [
						{"key": "username", "value": "john"},
						{"key": "password", "value": "secret123"}
					]
				},
				"url": "https://api.example.com/login"
			}
		},
		{
			"name": "Multipart Form",
			"request": {
				"method": "POST",
				"body": {
					"mode": "formdata",
					"formdata": [
						{"key": "name", "value": "My File", "type": "text"},
						{"key": "file", "type": "file", "src": "data.csv"}
					]
				},
				"url": "https://api.example.com/upload"
			}
		}
	]
}`

const sampleCollectionWithGraphQL = `{
	"info": {
		"name": "GraphQL Collection",
		"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
	},
	"item": [
		{
			"name": "GraphQL Query",
			"request": {
				"method": "POST",
				"header": [
					{
						"key": "Content-Type",
						"value": "application/json"
					}
				],
				"body": {
					"mode": "graphql",
					"graphql": {
						"query": "query GetUser($id: ID!) { user(id: $id) { id name email } }",
						"variables": "{\"id\": \"123\"}"
					}
				},
				"url": "https://api.example.com/graphql"
			}
		}
	]
}`

func TestImportBasicCollection(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "postman-import-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write sample collection
	collectionPath := filepath.Join(tmpDir, "collection.json")
	if err := os.WriteFile(collectionPath, []byte(samplePostmanCollection), 0644); err != nil {
		t.Fatalf("Failed to write collection file: %v", err)
	}

	// Import with default options
	opts := DefaultImportOptions()
	opts.OutputDir = tmpDir
	opts.SingleFile = true

	result, err := Import(collectionPath, opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify results
	if result.RequestsCount != 4 {
		t.Errorf("Expected 4 requests, got %d", result.RequestsCount)
	}

	if result.FoldersCount != 1 {
		t.Errorf("Expected 1 folder, got %d", result.FoldersCount)
	}

	if result.VariablesCount != 2 {
		t.Errorf("Expected 2 variables, got %d", result.VariablesCount)
	}

	if len(result.FilesCreated) != 1 {
		t.Errorf("Expected 1 file created, got %d", len(result.FilesCreated))
	}

	// Read the created file and verify content
	content, err := os.ReadFile(result.FilesCreated[0])
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)

	// Check for variables
	if !strings.Contains(contentStr, "@baseUrl = https://api.example.com") {
		t.Error("Missing baseUrl variable")
	}

	// Check for request names
	if !strings.Contains(contentStr, "# @name Get Users") {
		t.Error("Missing 'Get Users' request")
	}

	// Check for methods and URLs
	if !strings.Contains(contentStr, "GET {{baseUrl}}/users") {
		t.Error("Missing GET request to /users")
	}

	if !strings.Contains(contentStr, "POST {{baseUrl}}/users") {
		t.Error("Missing POST request to /users")
	}
}

func TestImportWithFolderStructure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-import-folders-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	collectionPath := filepath.Join(tmpDir, "collection.json")
	if err := os.WriteFile(collectionPath, []byte(samplePostmanCollection), 0644); err != nil {
		t.Fatalf("Failed to write collection file: %v", err)
	}

	opts := DefaultImportOptions()
	opts.OutputDir = tmpDir
	opts.SingleFile = false

	result, err := Import(collectionPath, opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Should create multiple files
	if len(result.FilesCreated) < 2 {
		t.Errorf("Expected at least 2 files created, got %d", len(result.FilesCreated))
	}

	// Check collection folder was created (output is now inside collection name folder)
	collectionFolder := filepath.Join(tmpDir, "Test Collection")
	if _, err := os.Stat(collectionFolder); os.IsNotExist(err) {
		t.Error("Collection folder 'Test Collection' was not created")
	}

	// Check subfolder was created inside collection folder
	folderPath := filepath.Join(collectionFolder, "User Operations")
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		t.Error("Folder 'User Operations' was not created inside collection folder")
	}
}

func TestImportWithAuth(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-import-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	collectionPath := filepath.Join(tmpDir, "collection.json")
	if err := os.WriteFile(collectionPath, []byte(sampleCollectionWithAuth), 0644); err != nil {
		t.Fatalf("Failed to write collection file: %v", err)
	}

	opts := DefaultImportOptions()
	opts.OutputDir = tmpDir
	opts.SingleFile = true

	result, err := Import(collectionPath, opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	content, err := os.ReadFile(result.FilesCreated[0])
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)

	// Check for Basic auth
	if !strings.Contains(contentStr, "Authorization: Basic testuser:testpass") {
		t.Error("Missing Basic auth header")
	}

	// Check for Bearer auth
	if !strings.Contains(contentStr, "Authorization: Bearer {{accessToken}}") {
		t.Error("Missing Bearer auth header")
	}
}

func TestImportWithScripts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-import-scripts-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	collectionPath := filepath.Join(tmpDir, "collection.json")
	if err := os.WriteFile(collectionPath, []byte(sampleCollectionWithScripts), 0644); err != nil {
		t.Fatalf("Failed to write collection file: %v", err)
	}

	opts := DefaultImportOptions()
	opts.OutputDir = tmpDir
	opts.SingleFile = true
	opts.IncludeScripts = true

	result, err := Import(collectionPath, opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	content, err := os.ReadFile(result.FilesCreated[0])
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)

	// Check for pre-request script
	if !strings.Contains(contentStr, "< {%") {
		t.Error("Missing pre-request script block")
	}

	// Check for test script
	if !strings.Contains(contentStr, "> {%") {
		t.Error("Missing test script block")
	}

	if !strings.Contains(contentStr, "client.log('Pre-request script')") {
		t.Error("Pre-request script content not found")
	}
}

func TestImportWithFormData(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-import-formdata-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	collectionPath := filepath.Join(tmpDir, "collection.json")
	if err := os.WriteFile(collectionPath, []byte(sampleCollectionWithFormData), 0644); err != nil {
		t.Fatalf("Failed to write collection file: %v", err)
	}

	opts := DefaultImportOptions()
	opts.OutputDir = tmpDir
	opts.SingleFile = true

	result, err := Import(collectionPath, opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	content, err := os.ReadFile(result.FilesCreated[0])
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)

	// Check for URL-encoded form
	if !strings.Contains(contentStr, "username=john") {
		t.Error("Missing URL-encoded form data")
	}
}

func TestImportWithGraphQL(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "postman-import-graphql-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	collectionPath := filepath.Join(tmpDir, "collection.json")
	if err := os.WriteFile(collectionPath, []byte(sampleCollectionWithGraphQL), 0644); err != nil {
		t.Fatalf("Failed to write collection file: %v", err)
	}

	opts := DefaultImportOptions()
	opts.OutputDir = tmpDir
	opts.SingleFile = true

	result, err := Import(collectionPath, opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	content, err := os.ReadFile(result.FilesCreated[0])
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)

	// Check for GraphQL query
	if !strings.Contains(contentStr, "query GetUser") {
		t.Error("Missing GraphQL query")
	}
}

func TestCollectionParsing(t *testing.T) {
	var collection Collection
	if err := json.Unmarshal([]byte(samplePostmanCollection), &collection); err != nil {
		t.Fatalf("Failed to parse collection: %v", err)
	}

	if collection.Info.Name != "Test Collection" {
		t.Errorf("Expected collection name 'Test Collection', got '%s'", collection.Info.Name)
	}

	if len(collection.Item) != 3 {
		t.Errorf("Expected 3 top-level items, got %d", len(collection.Item))
	}

	// Check folder
	folder := collection.Item[2]
	if !folder.IsFolder() {
		t.Error("Expected third item to be a folder")
	}

	if len(folder.Item) != 2 {
		t.Errorf("Expected folder to have 2 items, got %d", len(folder.Item))
	}

	// Check variables
	if len(collection.Variable) != 2 {
		t.Errorf("Expected 2 variables, got %d", len(collection.Variable))
	}
}

func TestURLParsing(t *testing.T) {
	urlJSON := `{
		"raw": "https://api.example.com:8080/users/123?page=1&limit=10#section",
		"protocol": "https",
		"host": ["api", "example", "com"],
		"port": "8080",
		"path": ["users", "123"],
		"query": [
			{"key": "page", "value": "1"},
			{"key": "limit", "value": "10"}
		],
		"hash": "section"
	}`

	var url URL
	if err := json.Unmarshal([]byte(urlJSON), &url); err != nil {
		t.Fatalf("Failed to parse URL: %v", err)
	}

	if url.GetRaw() != "https://api.example.com:8080/users/123?page=1&limit=10#section" {
		t.Errorf("Unexpected raw URL: %s", url.GetRaw())
	}

	if url.GetHost() != "api.example.com" {
		t.Errorf("Unexpected host: %s", url.GetHost())
	}

	if url.GetPath() != "/users/123" {
		t.Errorf("Unexpected path: %s", url.GetPath())
	}
}

func TestDescriptionUnmarshal(t *testing.T) {
	// Test string description
	stringDesc := `"This is a simple description"`
	var desc1 Description
	if err := json.Unmarshal([]byte(stringDesc), &desc1); err != nil {
		t.Fatalf("Failed to parse string description: %v", err)
	}
	if desc1.Content != "This is a simple description" {
		t.Errorf("Unexpected description content: %s", desc1.Content)
	}

	// Test object description
	objDesc := `{"content": "Detailed description", "type": "text/markdown"}`
	var desc2 Description
	if err := json.Unmarshal([]byte(objDesc), &desc2); err != nil {
		t.Fatalf("Failed to parse object description: %v", err)
	}
	if desc2.Content != "Detailed description" {
		t.Errorf("Unexpected description content: %s", desc2.Content)
	}
	if desc2.Type != "text/markdown" {
		t.Errorf("Unexpected description type: %s", desc2.Type)
	}
}

func TestScriptGetExec(t *testing.T) {
	// Test array exec
	script1 := Script{
		Exec: []interface{}{"line1", "line2", "line3"},
	}
	if script1.GetExec() != "line1\nline2\nline3" {
		t.Errorf("Unexpected exec: %s", script1.GetExec())
	}

	// Test string exec
	script2 := Script{
		Exec: "single line script",
	}
	if script2.GetExec() != "single line script" {
		t.Errorf("Unexpected exec: %s", script2.GetExec())
	}

	// Test nil exec
	var script3 Script
	if script3.GetExec() != "" {
		t.Error("Expected empty exec for nil script")
	}
}

func TestVariableGetValue(t *testing.T) {
	tests := []struct {
		name     string
		variable Variable
		expected string
	}{
		{
			name:     "string value",
			variable: Variable{Value: "test"},
			expected: "test",
		},
		{
			name:     "bool true",
			variable: Variable{Value: true},
			expected: "true",
		},
		{
			name:     "bool false",
			variable: Variable{Value: false},
			expected: "false",
		},
		{
			name:     "nil value",
			variable: Variable{Value: nil},
			expected: "",
		},
		{
			name:     "float64 integer",
			variable: Variable{Value: float64(123)},
			expected: "123",
		},
		{
			name:     "float64 decimal",
			variable: Variable{Value: float64(123.456)},
			expected: "123.456",
		},
		{
			name:     "float64 large number",
			variable: Variable{Value: float64(1000000)},
			expected: "1000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.variable.GetValue(); got != tt.expected {
				t.Errorf("GetValue() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with spaces", "with spaces"},
		{"with/slash", "with_slash"},
		{"with\\backslash", "with_backslash"},
		{"with:colon", "with_colon"},
		{"multiple///slashes", "multiple___slashes"},
		{"..hidden", "hidden"},
		{"   spaces   ", "spaces"},
		{"", "collection"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := sanitizeFileName(tt.input); got != tt.expected {
				t.Errorf("sanitizeFileName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNewCollection(t *testing.T) {
	collection := NewCollection("My API")

	if collection.Info.Name != "My API" {
		t.Errorf("Expected name 'My API', got '%s'", collection.Info.Name)
	}

	if collection.Info.Schema != SchemaV21 {
		t.Errorf("Expected schema '%s', got '%s'", SchemaV21, collection.Info.Schema)
	}

	if collection.Item == nil {
		t.Error("Item slice should not be nil")
	}
}

func TestConvertScriptFromPostman(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Convert pm.test to client.test",
			input:    `pm.test("Status is 200", function() {`,
			expected: `client.test("Status is 200", function() {`,
		},
		{
			name:     "Convert pm.expect with message",
			input:    `pm.expect(pm.response.code === 200, "Expected status 200").to.be.true;`,
			expected: `client.assert(response.status === 200, "Expected status 200");`,
		},
		{
			name:     "Convert pm.expect without message",
			input:    `pm.expect(pm.response.code === 200).to.be.true;`,
			expected: `client.assert(response.status === 200);`,
		},
		{
			name:     "Convert console.log",
			input:    `console.log("Hello world");`,
			expected: `client.log("Hello world");`,
		},
		{
			name:     "Convert pm.globals.set",
			input:    `pm.globals.set("key", "value");`,
			expected: `client.global.set("key", "value");`,
		},
		{
			name:     "Convert pm.globals.get",
			input:    `var x = pm.globals.get("key");`,
			expected: `var x = client.global.get("key");`,
		},
		{
			name:     "Convert pm.response.code",
			input:    `if (pm.response.code === 200) {`,
			expected: `if (response.status === 200) {`,
		},
		{
			name:     "Convert pm.response.json() property access",
			input:    `var id = pm.response.json().id;`,
			expected: `var id = response.body.id;`,
		},
		{
			name:     "Convert pm.response.json() in parentheses",
			input:    `client.assert(pm.response.json())`,
			expected: `client.assert(response.body)`,
		},
		{
			name:     "Convert pm.response.headers.get",
			input:    `var ct = pm.response.headers.get("Content-Type");`,
			expected: `var ct = response.headers.valueOf("Content-Type");`,
		},
		{
			name:     "Convert pm.variables.replaceIn guid",
			input:    `var id = pm.variables.replaceIn("{{$guid}}");`,
			expected: `var id = $uuid();`,
		},
		{
			name:     "Convert Date.now()",
			input:    `var ts = Date.now();`,
			expected: `var ts = $timestamp();`,
		},
		{
			name:     "Convert new Date().toISOString()",
			input:    `var iso = new Date().toISOString();`,
			expected: `var iso = $isoTimestamp();`,
		},
		{
			name:     "Convert random int expression",
			input:    `var num = (Math.floor(Math.random() * (100 - 1 + 1)) + 1);`,
			expected: `var num = $randomInt(1, 100);`,
		},
		{
			name:     "Convert random string expression",
			input:    `var str = Array(10).fill(0).map(() => Math.random().toString(36).charAt(2)).join('');`,
			expected: `var str = $randomString(10);`,
		},
		{
			name:     "Convert btoa",
			input:    `var encoded = btoa(data);`,
			expected: `var encoded = $base64(data);`,
		},
		{
			name:     "Convert atob",
			input:    `var decoded = atob(encoded);`,
			expected: `var decoded = $base64Decode(encoded);`,
		},
		{
			name:     "Convert CryptoJS.MD5",
			input:    `var hash = CryptoJS.MD5(data).toString();`,
			expected: `var hash = $md5(data);`,
		},
		{
			name:     "Convert CryptoJS.SHA256",
			input:    `var hash = CryptoJS.SHA256(data).toString();`,
			expected: `var hash = $sha256(data);`,
		},
		{
			name:     "Convert CryptoJS.SHA512",
			input:    `var hash = CryptoJS.SHA512(data).toString();`,
			expected: `var hash = $sha512(data);`,
		},
		{
			name: "Convert complex script",
			input: `pm.test("Status is 200", function() {
    pm.expect(pm.response.code === 200, "Expected status 200").to.be.true;
});
console.log("Post title: " + pm.response.json().title);
pm.globals.set("postId", pm.response.json().id);`,
			expected: `client.test("Status is 200", function() {
    client.assert(response.status === 200, "Expected status 200");
});
client.log("Post title: " + response.body.title);
client.global.set("postId", response.body.id);`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertScriptFromPostman(tt.input)
			if result != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestScriptRoundTrip(t *testing.T) {
	// Test that converting to Postman and back preserves the original script
	originalScripts := []string{
		`client.test("Status is 200", function() {
    client.assert(response.status === 200, "Expected status 200");
});`,
		`client.log("Hello world");`,
		`client.global.set("key", response.body.id);`,
		`var hash = $sha256(data);`,
		`var id = $uuid();`,
	}

	for _, original := range originalScripts {
		t.Run(original[:min(30, len(original))], func(t *testing.T) {
			// Convert to Postman format
			postman := convertScriptToPostman(original)
			// Convert back to rest-client format
			roundTripped := convertScriptFromPostman(postman)

			if roundTripped != original {
				t.Errorf("Roundtrip failed.\nOriginal:\n%s\nAfter Postman:\n%s\nAfter roundtrip:\n%s",
					original, postman, roundTripped)
			}
		})
	}
}

func TestImportWithPostmanScripts(t *testing.T) {
	// Test importing a collection with Postman-style scripts
	tmpDir, err := os.MkdirTemp("", "postman-import-scripts-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a Postman collection with pm.* scripts
	collectionWithScripts := `{
		"info": {
			"name": "Script Test",
			"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
		},
		"item": [
			{
				"name": "Test Request",
				"event": [
					{
						"listen": "test",
						"script": {
							"type": "text/javascript",
							"exec": [
								"pm.test('Status is 200', function() {",
								"    pm.expect(pm.response.code === 200, 'Expected 200').to.be.true;",
								"});",
								"console.log('Response: ' + pm.response.json().message);",
								"pm.globals.set('testId', pm.response.json().id);"
							]
						}
					}
				],
				"request": {
					"method": "GET",
					"url": "https://api.example.com/test"
				}
			}
		]
	}`

	collectionPath := filepath.Join(tmpDir, "collection.json")
	if err := os.WriteFile(collectionPath, []byte(collectionWithScripts), 0644); err != nil {
		t.Fatalf("Failed to write collection: %v", err)
	}

	opts := DefaultImportOptions()
	opts.OutputDir = tmpDir
	opts.SingleFile = true

	result, err := Import(collectionPath, opts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if len(result.FilesCreated) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(result.FilesCreated))
	}

	content, err := os.ReadFile(result.FilesCreated[0])
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)

	// Verify script was converted from Postman to rest-client format
	if !strings.Contains(contentStr, "client.test(") {
		t.Error("Expected client.test() in converted script")
	}
	if !strings.Contains(contentStr, "client.assert(") {
		t.Error("Expected client.assert() in converted script")
	}
	if !strings.Contains(contentStr, "client.log(") {
		t.Error("Expected client.log() in converted script")
	}
	if !strings.Contains(contentStr, "client.global.set(") {
		t.Error("Expected client.global.set() in converted script")
	}
	if !strings.Contains(contentStr, "response.body.") {
		t.Error("Expected response.body in converted script")
	}

	// Verify Postman syntax is NOT present
	if strings.Contains(contentStr, "pm.test(") {
		t.Error("Script should not contain pm.test()")
	}
	if strings.Contains(contentStr, "pm.expect(") {
		t.Error("Script should not contain pm.expect()")
	}
	if strings.Contains(contentStr, "console.log(") {
		t.Error("Script should not contain console.log()")
	}
	if strings.Contains(contentStr, "pm.globals.set(") {
		t.Error("Script should not contain pm.globals.set()")
	}
}

func TestFullRoundTrip(t *testing.T) {
	// Test full roundtrip: .http -> Postman -> .http
	// Verify that the essential parts are preserved
	tmpDir, err := os.MkdirTemp("", "postman-full-roundtrip-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a comprehensive .http file with various features
	originalHttp := `@baseUrl = https://api.example.com
@apiKey = test-key-123

# @name GetUsers
GET {{baseUrl}}/users
Accept: application/json

###

# @name GetUserById
GET {{baseUrl}}/users/1
Accept: application/json

###

# @name CreateUser
POST {{baseUrl}}/users
Content-Type: application/json

{
    "name": "John Doe",
    "email": "john@example.com"
}

###

# @name TestWithScript
GET {{baseUrl}}/test
Accept: application/json

> {%
client.test("Status is 200", function() {
    client.assert(response.status === 200, "Expected 200");
});
client.log("Response received");
client.global.set("testId", response.body.id);
%}

###

# @name RequestWithPreScript
< {%
var timestamp = $timestamp();
client.global.set("ts", timestamp);
client.log("Timestamp: " + timestamp);
%}
POST {{baseUrl}}/data
Content-Type: application/json

{
    "timestamp": "{{ts}}"
}

> {%
client.test("Created", function() {
    client.assert(response.status === 201, "Expected 201");
});
%}
`

	// Write original .http file
	httpPath := filepath.Join(tmpDir, "original.http")
	if err := os.WriteFile(httpPath, []byte(originalHttp), 0644); err != nil {
		t.Fatalf("Failed to write .http file: %v", err)
	}

	// Export to Postman
	exportPath := filepath.Join(tmpDir, "collection.json")
	exportOpts := DefaultExportOptions()
	exportOpts.IncludeVariables = true
	exportOpts.IncludeScripts = true

	exportResult, err := Export([]string{httpPath}, exportPath, exportOpts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if exportResult.RequestsCount != 5 {
		t.Errorf("Expected 5 requests exported, got %d", exportResult.RequestsCount)
	}

	if exportResult.VariablesCount != 2 {
		t.Errorf("Expected 2 variables exported, got %d", exportResult.VariablesCount)
	}

	// Import back from Postman
	importPath := filepath.Join(tmpDir, "reimported.http")
	importOpts := DefaultImportOptions()
	importOpts.OutputFile = importPath
	importOpts.SingleFile = true
	importOpts.IncludeVariables = true
	importOpts.IncludeScripts = true

	importResult, err := Import(exportPath, importOpts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if importResult.RequestsCount != 5 {
		t.Errorf("Expected 5 requests imported, got %d", importResult.RequestsCount)
	}

	// Read the reimported file
	reimportedContent, err := os.ReadFile(importPath)
	if err != nil {
		t.Fatalf("Failed to read reimported file: %v", err)
	}

	content := string(reimportedContent)

	// Verify variables are preserved
	if !strings.Contains(content, "@baseUrl = https://api.example.com") {
		t.Error("Variable baseUrl not preserved")
	}
	if !strings.Contains(content, "@apiKey = test-key-123") {
		t.Error("Variable apiKey not preserved")
	}

	// Verify request names are preserved
	for _, name := range []string{"GetUsers", "GetUserById", "CreateUser", "TestWithScript", "RequestWithPreScript"} {
		if !strings.Contains(content, "# @name "+name) {
			t.Errorf("Request name %s not preserved", name)
		}
	}

	// Verify URLs are preserved (without leading slash issue)
	if !strings.Contains(content, "GET {{baseUrl}}/users") {
		t.Error("URL {{baseUrl}}/users not preserved correctly")
	}
	if !strings.Contains(content, "GET {{baseUrl}}/users/1") {
		t.Error("URL {{baseUrl}}/users/1 not preserved correctly")
	}
	if !strings.Contains(content, "POST {{baseUrl}}/users") {
		t.Error("URL POST {{baseUrl}}/users not preserved correctly")
	}

	// Verify scripts are preserved with client.* syntax (not pm.*)
	if !strings.Contains(content, "client.test(") {
		t.Error("client.test() not preserved in scripts")
	}
	if !strings.Contains(content, "client.assert(") {
		t.Error("client.assert() not preserved in scripts")
	}
	if !strings.Contains(content, "client.log(") {
		t.Error("client.log() not preserved in scripts")
	}
	if !strings.Contains(content, "client.global.set(") {
		t.Error("client.global.set() not preserved in scripts")
	}
	if !strings.Contains(content, "response.status") {
		t.Error("response.status not preserved in scripts")
	}
	if !strings.Contains(content, "response.body.") {
		t.Error("response.body not preserved in scripts")
	}
	if !strings.Contains(content, "$timestamp()") {
		t.Error("$timestamp() not preserved in scripts")
	}

	// Verify Postman syntax is NOT present
	if strings.Contains(content, "pm.test(") {
		t.Error("Postman pm.test() should not be in reimported file")
	}
	if strings.Contains(content, "pm.expect(") {
		t.Error("Postman pm.expect() should not be in reimported file")
	}
	if strings.Contains(content, "pm.response.code") {
		t.Error("Postman pm.response.code should not be in reimported file")
	}

	// Verify pre-request and post-request script blocks are preserved
	if !strings.Contains(content, "< {%") {
		t.Error("Pre-request script block not preserved")
	}
	if !strings.Contains(content, "> {%") {
		t.Error("Post-request script block not preserved")
	}

	// Verify JSON body is preserved
	if !strings.Contains(content, `"name": "John Doe"`) {
		t.Error("JSON body not preserved")
	}
}
