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
