package scripting

import (
	"context"
	"regexp"
	"testing"

	"github.com/ideaspaper/restclient/pkg/models"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine()
	if engine == nil {
		t.Fatal("Expected engine to be created")
	}
}

func TestExecuteEmptyScript(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})

	result, err := engine.Execute("", ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result to be returned")
	}
	if len(result.Tests) != 0 {
		t.Errorf("Expected no tests, got %d", len(result.Tests))
	}
}

func TestClientLog(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})

	script := `client.log("Hello, World!");`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result.Logs) != 1 {
		t.Errorf("Expected 1 log, got %d", len(result.Logs))
	}
	if result.Logs[0] != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", result.Logs[0])
	}
}

func TestClientTest(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})
	ctx.SetResponse(&models.HttpResponse{
		StatusCode:    200,
		StatusMessage: "200 OK",
		Body:          `{"id": 1, "name": "Test"}`,
		Headers:       map[string][]string{"Content-Type": {"application/json"}},
	})

	script := `
		client.test("Status is 200", function() {
			client.assert(response.status === 200);
		});
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result.Tests) != 1 {
		t.Errorf("Expected 1 test, got %d", len(result.Tests))
	}
	if !result.Tests[0].Passed {
		t.Errorf("Expected test to pass")
	}
	if result.Tests[0].Name != "Status is 200" {
		t.Errorf("Expected test name 'Status is 200', got '%s'", result.Tests[0].Name)
	}
}

func TestClientTestFailure(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})
	ctx.SetResponse(&models.HttpResponse{
		StatusCode:    404,
		StatusMessage: "404 Not Found",
		Body:          `{"error": "not found"}`,
		Headers:       map[string][]string{},
	})

	script := `
		client.test("Status is 200", function() {
			client.assert(response.status === 200, "Expected 200");
		});
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result.Tests) != 1 {
		t.Errorf("Expected 1 test, got %d", len(result.Tests))
	}
	if result.Tests[0].Passed {
		t.Errorf("Expected test to fail")
	}
	// Check that error message contains "Expected 200"
	if result.Tests[0].Error == "" {
		t.Errorf("Expected error message, got empty string")
	}
}

func TestGlobalVariables(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})
	ctx.SetResponse(&models.HttpResponse{
		StatusCode: 200,
		Body:       `{"token": "abc123"}`,
		Headers:    map[string][]string{},
	})

	script := `
		client.global.set("myToken", response.body.token);
		client.log("Token set: " + client.global.get("myToken"));
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.GlobalVars["myToken"] != "abc123" {
		t.Errorf("Expected globalVars['myToken'] to be 'abc123', got '%v'", result.GlobalVars["myToken"])
	}
	if len(result.Logs) != 1 {
		t.Errorf("Expected 1 log, got %d", len(result.Logs))
	}
}

func TestResponseHeaders(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})
	ctx.SetResponse(&models.HttpResponse{
		StatusCode: 200,
		Body:       `{}`,
		Headers: map[string][]string{
			"Content-Type": {"application/json; charset=utf-8"},
			"X-Custom":     {"custom-value"},
		},
	})

	script := `
		var ct = response.headers.valueOf("Content-Type");
		client.log("Content-Type: " + ct);
		
		client.test("Has Content-Type", function() {
			client.assert(ct !== null);
		});
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result.Tests) != 1 {
		t.Errorf("Expected 1 test, got %d", len(result.Tests))
	}
	if !result.Tests[0].Passed {
		t.Errorf("Expected test to pass: %s", result.Tests[0].Error)
	}
}

func TestRequestObject(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method:  "POST",
		URL:     "https://example.com/api",
		Headers: map[string]string{"Authorization": "Bearer token123"},
		RawBody: `{"name": "test"}`,
	})

	script := `
		client.log("Method: " + request.method);
		client.log("URL: " + request.url);
		
		var authHeader = request.headers.findByName("Authorization");
		client.log("Auth: " + authHeader);
		
		client.test("Method is POST", function() {
			client.assert(request.method === "POST");
		});
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result.Tests) != 1 {
		t.Errorf("Expected 1 test, got %d", len(result.Tests))
	}
	if !result.Tests[0].Passed {
		t.Errorf("Expected test to pass: %s", result.Tests[0].Error)
	}
	if len(result.Logs) != 3 {
		t.Errorf("Expected 3 logs, got %d", len(result.Logs))
	}
}

func TestConsoleLog(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})

	script := `
		console.log("Test", "message", 123);
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result.Logs) != 1 {
		t.Errorf("Expected 1 log, got %d", len(result.Logs))
	}
	if result.Logs[0] != "Test message 123" {
		t.Errorf("Expected 'Test message 123', got '%s'", result.Logs[0])
	}
}

func TestScriptContext(t *testing.T) {
	ctx := NewScriptContext()

	// Test global vars
	ctx.SetGlobalVar("key1", "value1")
	if ctx.GetGlobalVar("key1") != "value1" {
		t.Errorf("Expected 'value1', got '%v'", ctx.GetGlobalVar("key1"))
	}

	// Test env vars
	ctx.SetEnvVar("API_KEY", "secret123")
	if ctx.GetEnvVar("API_KEY") != "secret123" {
		t.Errorf("Expected 'secret123', got '%s'", ctx.GetEnvVar("API_KEY"))
	}

	// Test GetGlobalVarAsString
	ctx.SetGlobalVar("number", 42.0)
	if ctx.GetGlobalVarAsString("number") != "42" {
		t.Errorf("Expected '42', got '%s'", ctx.GetGlobalVarAsString("number"))
	}
}

func TestMultipleTests(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})
	ctx.SetResponse(&models.HttpResponse{
		StatusCode: 200,
		Body:       `{"id": 1, "name": "Test", "active": true}`,
		Headers:    map[string][]string{},
	})

	script := `
		client.test("Status is 200", function() {
			client.assert(response.status === 200);
		});
		
		client.test("Has id", function() {
			client.assert(response.body.id === 1);
		});
		
		client.test("Has name", function() {
			client.assert(response.body.name === "Test");
		});
		
		client.test("Is active", function() {
			client.assert(response.body.active === true);
		});
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result.Tests) != 4 {
		t.Errorf("Expected 4 tests, got %d", len(result.Tests))
	}
	for _, test := range result.Tests {
		if !test.Passed {
			t.Errorf("Expected test '%s' to pass", test.Name)
		}
	}
}

func TestEnvironmentAccess(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})
	ctx.SetEnvVar("API_KEY", "test-api-key")

	script := `
		var apiKey = request.environment.get("API_KEY");
		client.log("API Key: " + apiKey);
		
		client.test("API Key exists", function() {
			client.assert(apiKey === "test-api-key");
		});
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result.Tests) != 1 {
		t.Errorf("Expected 1 test, got %d", len(result.Tests))
	}
	if !result.Tests[0].Passed {
		t.Errorf("Expected test to pass: %s", result.Tests[0].Error)
	}
}

func TestUuidFunction(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})

	script := `
		var id = $uuid();
		client.global.set("generatedUuid", id);
		client.log("UUID: " + id);
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	uuid, ok := result.GlobalVars["generatedUuid"].(string)
	if !ok {
		t.Fatal("Expected UUID to be a string")
	}

	// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	if !uuidRegex.MatchString(uuid) {
		t.Errorf("Expected valid UUID format, got '%s'", uuid)
	}
}

func TestGuidFunction(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})

	script := `
		var id = $guid();
		client.global.set("generatedGuid", id);
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	guid, ok := result.GlobalVars["generatedGuid"].(string)
	if !ok {
		t.Fatal("Expected GUID to be a string")
	}

	// GUID is an alias for UUID
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	if !uuidRegex.MatchString(guid) {
		t.Errorf("Expected valid UUID format, got '%s'", guid)
	}
}

func TestTimestampFunction(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})

	script := `
		var ts = $timestamp();
		client.global.set("timestamp", ts);
		client.log("Timestamp: " + ts);
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ts, ok := result.GlobalVars["timestamp"].(int64)
	if !ok {
		t.Fatalf("Expected timestamp to be int64, got %T", result.GlobalVars["timestamp"])
	}

	// Timestamp should be greater than 0 and reasonable (after year 2020)
	if ts < 1577836800000 { // 2020-01-01 in milliseconds
		t.Errorf("Timestamp seems too old: %d", ts)
	}
}

func TestIsoTimestampFunction(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})

	script := `
		var ts = $isoTimestamp();
		client.global.set("isoTimestamp", ts);
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	isoTs, ok := result.GlobalVars["isoTimestamp"].(string)
	if !ok {
		t.Fatal("Expected ISO timestamp to be a string")
	}

	// ISO 8601 format: 2024-01-15T10:30:00Z
	isoRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)
	if !isoRegex.MatchString(isoTs) {
		t.Errorf("Expected valid ISO 8601 format, got '%s'", isoTs)
	}
}

func TestRandomIntFunction(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})

	script := `
		var r1 = $randomInt(1, 10);
		var r2 = $randomInt(100, 200);
		var r3 = $randomInt(); // Default 0-1000
		client.global.set("r1", r1);
		client.global.set("r2", r2);
		client.global.set("r3", r3);
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	r1, _ := result.GlobalVars["r1"].(int64)
	r2, _ := result.GlobalVars["r2"].(int64)
	r3, _ := result.GlobalVars["r3"].(int64)

	if r1 < 1 || r1 > 10 {
		t.Errorf("Expected r1 to be between 1 and 10, got %d", r1)
	}
	if r2 < 100 || r2 > 200 {
		t.Errorf("Expected r2 to be between 100 and 200, got %d", r2)
	}
	if r3 < 0 || r3 > 1000 {
		t.Errorf("Expected r3 to be between 0 and 1000, got %d", r3)
	}
}

func TestBase64Functions(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})

	script := `
		var original = "Hello, World!";
		var encoded = $base64(original);
		var decoded = $base64Decode(encoded);
		
		client.global.set("encoded", encoded);
		client.global.set("decoded", decoded);
		
		client.test("Base64 round-trip", function() {
			client.assert(decoded === original, "Decoded should match original");
		});
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	encoded := result.GlobalVars["encoded"].(string)
	decoded := result.GlobalVars["decoded"].(string)

	if encoded != "SGVsbG8sIFdvcmxkIQ==" {
		t.Errorf("Expected 'SGVsbG8sIFdvcmxkIQ==', got '%s'", encoded)
	}
	if decoded != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", decoded)
	}

	if !result.Tests[0].Passed {
		t.Errorf("Base64 round-trip test failed: %s", result.Tests[0].Error)
	}
}

func TestMd5Function(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})

	script := `
		var hash = $md5("hello");
		client.global.set("hash", hash);
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	hash := result.GlobalVars["hash"].(string)
	// MD5 of "hello" is well-known
	expected := "5d41402abc4b2a76b9719d911017c592"
	if hash != expected {
		t.Errorf("Expected '%s', got '%s'", expected, hash)
	}
}

func TestSha256Function(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})

	script := `
		var hash = $sha256("hello");
		client.global.set("hash", hash);
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	hash := result.GlobalVars["hash"].(string)
	// SHA256 of "hello" is well-known
	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if hash != expected {
		t.Errorf("Expected '%s', got '%s'", expected, hash)
	}
}

func TestSha512Function(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})

	script := `
		var hash = $sha512("hello");
		client.global.set("hash", hash);
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	hash := result.GlobalVars["hash"].(string)
	// SHA512 should be 128 hex characters
	if len(hash) != 128 {
		t.Errorf("Expected 128 character hash, got %d characters", len(hash))
	}
}

func TestRandomStringFunction(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})

	script := `
		var s1 = $randomString();      // Default 16 chars
		var s2 = $randomString(32);    // 32 chars
		var s3 = $randomString(8);     // 8 chars
		client.global.set("s1", s1);
		client.global.set("s2", s2);
		client.global.set("s3", s3);
	`
	result, err := engine.Execute(script, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	s1 := result.GlobalVars["s1"].(string)
	s2 := result.GlobalVars["s2"].(string)
	s3 := result.GlobalVars["s3"].(string)

	if len(s1) != 16 {
		t.Errorf("Expected s1 length 16, got %d", len(s1))
	}
	if len(s2) != 32 {
		t.Errorf("Expected s2 length 32, got %d", len(s2))
	}
	if len(s3) != 8 {
		t.Errorf("Expected s3 length 8, got %d", len(s3))
	}

	// Check that string contains only alphanumeric characters
	alphanumRegex := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	if !alphanumRegex.MatchString(s1) {
		t.Errorf("Expected alphanumeric string, got '%s'", s1)
	}
}

func TestEngineExecuteWithContextCancellation(t *testing.T) {
	engine := NewEngine()
	ctx := NewScriptContext()
	ctx.SetRequest(&models.HttpRequest{
		Method: "GET",
		URL:    "https://example.com",
	})

	script := `
		while (true) {
			// busy loop
		}
	`

	testCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := engine.ExecuteWithContext(testCtx, script, ctx)
	if err == nil {
		t.Fatal("expected cancellation error")
	}

	if err != nil && err.Error() != "script execution canceled: context canceled" {
		// Accept wrapped context cancellation errors
		if !regexp.MustCompile(`context canceled`).MatchString(err.Error()) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	}

	// Ensure we can run another script after cancellation without interference
	result, err := engine.Execute("client.log('ok');", ctx)
	if err != nil {
		t.Fatalf("unexpected error after cancellation: %v", err)
	}
	if len(result.Logs) != 1 {
		t.Fatalf("expected one log, got %d", len(result.Logs))
	}
}
