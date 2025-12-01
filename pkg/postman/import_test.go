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

	if !strings.Contains(contentStr, "console.log('Pre-request script')") {
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
