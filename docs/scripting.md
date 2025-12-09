# Scripting

restclient supports JavaScript scripting for testing responses and sharing data between requests, similar to Postman.

## Session Persistence

By default, restclient persists certain data between CLI invocations. Sessions are scoped by directory (based on the `.http` file location) or an explicit `--session` name.

| Data Type                                 | Persisted? | Notes                                   |
| ----------------------------------------- | ---------- | --------------------------------------- |
| Cookies (`Set-Cookie` headers)            | Yes        | Stored in `<session>/cookies.json`      |
| Script variables (`client.global.set()`)  | Yes        | Stored in `<session>/variables.json`    |
| Environment variables                     | Yes        | Stored in `<session>/environments.json` |
| User input values (`{{:param}}`)          | No         | Prompted fresh on every run             |
| File variables (`@var = value`)           | No         | Re-read each invocation                 |
| Request variables (`{{req.response...}}`) | No         | Single execution only                   |

```bash
# Use a named session for isolation
restclient send api.http --session my-test

# Disable session persistence entirely
restclient send api.http --no-session
```

## Post-Response Scripts

Add a script block after your request to test the response:

```http
### Get user with tests
# @name getUser
GET https://api.example.com/users/1
Accept: application/json

> {%
client.test("Status is 200", function() {
    client.assert(response.status === 200, "Expected status 200");
});

client.test("User has name", function() {
    client.assert(response.body.name !== undefined, "Expected name field");
});

// Log information
client.log("User name: " + response.body.name);

// Store value for later requests
client.global.set("userId", response.body.id);
%}
```

## Pre-Request Scripts

Run JavaScript before the request is sent:

```http
### Request with pre-script
# @name myRequest
< {%
client.log("Preparing request...");
client.global.set("timestamp", Date.now());
%}
GET https://api.example.com/data
Accept: application/json
```

## External Script Files

You can also reference external JavaScript files instead of inline scripts:

```http
### Request with external scripts
< ./scripts/pre-request.js
GET https://api.example.com/data
Accept: application/json

> ./scripts/post-response.js
```

The script file path can be relative to the `.http` file or absolute:

```http
### Using relative path
< ./pre-script.js
GET https://api.example.com/users

> ../shared/validate-response.js
```

## Script API Reference

### client Object

| Method                              | Description                            |
| ----------------------------------- | -------------------------------------- |
| `client.test(name, fn)`             | Define a test with a name and function |
| `client.assert(condition, message)` | Assert a condition is true             |
| `client.log(text)`                  | Log text to the console                |
| `client.global.set(name, value)`    | Store a global variable                |
| `client.global.get(name)`           | Retrieve a global variable             |
| `client.global.clear(name)`         | Remove a global variable               |
| `client.global.clearAll()`          | Remove all global variables            |
| `client.global.isEmpty()`           | Check if global storage is empty       |

### response Object (post-response scripts only)

| Property                          | Description                                  |
| --------------------------------- | -------------------------------------------- |
| `response.status`                 | HTTP status code (e.g., 200)                 |
| `response.statusText`             | Status message (e.g., "200 OK")              |
| `response.body`                   | Response body (parsed as JSON if applicable) |
| `response.headers.valueOf(name)`  | Get header value by name                     |
| `response.headers.valuesOf(name)` | Get all header values by name                |
| `response.contentType.mimeType`   | Response MIME type                           |
| `response.contentType.charset`    | Response charset                             |

### request Object

| Property                           | Description                   |
| ---------------------------------- | ----------------------------- |
| `request.method`                   | HTTP method (GET, POST, etc.) |
| `request.url`                      | Request URL                   |
| `request.body`                     | Request body                  |
| `request.headers.all`              | Array of all headers          |
| `request.headers.findByName(name)` | Get header value by name      |
| `request.environment.get(name)`    | Get environment variable      |

### Built-in Utility Functions

These functions are available globally in your scripts:

| Function                | Description                        | Example                                                |
| ----------------------- | ---------------------------------- | ------------------------------------------------------ |
| `$uuid()`               | Generate a UUID v4                 | `"550e8400-e29b-41d4-a716-446655440000"`               |
| `$guid()`               | Alias for `$uuid()`                | `"550e8400-e29b-41d4-a716-446655440000"`               |
| `$timestamp()`          | Unix timestamp in milliseconds     | `1704067200000`                                        |
| `$isoTimestamp()`       | ISO 8601 timestamp                 | `"2024-01-01T00:00:00Z"`                               |
| `$randomInt(min, max)`  | Random integer between min and max | `$randomInt(1, 100)` → `42`                            |
| `$randomString(length)` | Random alphanumeric string         | `$randomString(16)` → `"a1B2c3D4e5F6g7H8"`             |
| `$base64(text)`         | Base64 encode a string             | `$base64("hello")` → `"aGVsbG8="`                      |
| `$base64Decode(text)`   | Base64 decode a string             | `$base64Decode("aGVsbG8=")` → `"hello"`                |
| `$md5(text)`            | MD5 hash of a string               | `$md5("hello")` → `"5d41402abc4b2a76b9719d911017c592"` |
| `$sha256(text)`         | SHA256 hash of a string            | `$sha256("hello")` → `"2cf24dba..."`                   |
| `$sha512(text)`         | SHA512 hash of a string            | `$sha512("hello")` → `"9b71d224..."`                   |

**Using Utility Functions:**

```http
### Generate dynamic data
POST https://api.example.com/users
Content-Type: application/json

< {%
client.global.set("requestId", $uuid());
client.global.set("timestamp", $timestamp());
%}

{"requestId": "{{requestId}}"}

> {%
// Validate and hash the response
var hash = $sha256(JSON.stringify(response.body));
client.log("Response hash: " + hash);

// Generate a random token
var token = $randomString(32);
client.global.set("sessionToken", token);
%}
```

## Scripting Examples

### Testing Response Status and Body

```http
### Create user and validate
POST https://api.example.com/users
Content-Type: application/json

{"name": "John", "email": "john@example.com"}

> {%
client.test("Status is 201", function() {
    client.assert(response.status === 201);
});

client.test("Response has ID", function() {
    client.assert(response.body.id !== undefined);
});

client.test("Email matches", function() {
    client.assert(response.body.email === "john@example.com");
});

// Store for next request
client.global.set("newUserId", response.body.id);
%}
```

### Chaining Requests with Global Variables

```http
### Login
# @name login
POST https://api.example.com/auth/login
Content-Type: application/json

{"username": "admin", "password": "secret"}

> {%
client.test("Login successful", function() {
    client.assert(response.status === 200);
    client.assert(response.body.token !== undefined);
});

client.global.set("authToken", response.body.token);
client.log("Logged in successfully");
%}

### Get protected resource
# @name getProtected
GET https://api.example.com/protected
Authorization: Bearer {{authToken}}

> {%
client.test("Access granted", function() {
    client.assert(response.status === 200);
});
%}
```

### Validating Headers

```http
### Check content type
GET https://api.example.com/data

> {%
client.test("Content-Type is JSON", function() {
    var contentType = response.headers.valueOf("Content-Type");
    client.assert(contentType.includes("application/json"));
});

client.test("Has cache control", function() {
    var cacheControl = response.headers.valueOf("Cache-Control");
    client.assert(cacheControl !== null);
});
%}
```
