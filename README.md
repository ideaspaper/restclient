# restclient

A powerful command-line HTTP client inspired by the [VS Code REST Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client) extension. Send HTTP requests directly from `.http` and `.rest` files, manage environments, and more.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Commands](#commands)
  - [send](#send)
  - [env](#env)
  - [history](#history)
  - [session](#session)
  - [completion](#completion)
  - [postman](#postman)
- [HTTP File Format](#http-file-format)
- [Variables](#variables)
- [Scripting](#scripting)
- [Authentication](#authentication)
- [Configuration](#configuration)
- [Global Flags](#global-flags)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)

## Features

- Parse and execute requests from `.http` and `.rest` files
- **Interactive fuzzy-search selector** for choosing requests from multi-request files or history
- **JavaScript scripting** for testing responses and chaining requests (like Postman)
- **Postman Collection v2.1.0 import/export** - full compatibility with Postman
- Multiple environments with variable support
- System variables (UUID, timestamps, random values, etc.)
- File variables and `.env` file support
- Request history with replay
- Multipart form data and file uploads
- GraphQL support (queries, mutations, subscriptions)
- Basic, Digest, and AWS Signature v4 authentication
- Cookie jar for subsequent requests within a session
- Colored output with syntax highlighting for JSON and XML
- Shell completion for bash, zsh, fish, and PowerShell

## Installation

### Using Homebrew

```bash
brew tap ideaspaper/tap
brew install --cask restclient
```

### Using `go install`

```bash
go install github.com/ideaspaper/restclient@latest
```

This will install the binary as `restclient` in your `$GOPATH/bin` directory. Make sure `$GOPATH/bin` is in your `PATH`.

### From Source

```bash
# Clone the repository
git clone https://github.com/ideaspaper/restclient.git
cd restclient

# Build the binary
go build -o restclient .

# Move to PATH (optional)
sudo mv restclient /usr/local/bin/
```

### Dependencies

Requires Go 1.24.4 or later.

```bash
go mod download
```

## Quick Start

### Basic Usage

```bash
# Send a request from a .http file
restclient send api.http

# Re-send from the last used file
restclient send

# Send with a specific environment
restclient send api.http -e production
```

### Create Your First Request File

Create a file named `api.http`:

```http
### Get all users
# @name getUsers
GET https://api.example.com/users
Authorization: Bearer {{token}}
Accept: application/json

### Create a new user
# @name createUser
POST https://api.example.com/users
Content-Type: application/json

{
    "name": "John Doe",
    "email": "john@example.com"
}

### Delete a user
DELETE https://api.example.com/users/{{userId}}
Authorization: Bearer {{token}}
```

## Commands

### send

Send HTTP requests from `.http` or `.rest` files.

When the file contains multiple requests and no `--name` or `--index` flag is provided, an interactive fuzzy-search selector is displayed.

**Last File Memory:** If no file is specified, restclient automatically uses the last opened file. The last file path is stored in `~/.restclient/lastfile`.

```bash
restclient send [file.http] [flags]
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--name` | `-n` | Select request by name (from `@name` metadata) |
| `--index` | `-i` | Select request by index |
| `--headers` | | Only show response headers |
| `--body` | | Only show response body |
| `--output` | `-o` | Save response body to file |
| `--no-history` | | Don't save request to history |
| `--dry-run` | | Preview request without sending |
| `--skip-validate` | | Skip request validation |
| `--session` | | Use a named session instead of directory-based |
| `--no-session` | | Don't load or save session state |
| `--strict` | | Error on duplicate `@name` values instead of warning |
| `--force-prompt` | `-p` | Force prompting for user input values (`{{:paramName}}`) |

**Examples:**

```bash
# Send first request in file
restclient send api.http

# Re-send from the last used file
restclient send

# Interactive selection (when file has multiple requests)
restclient send api.http

# Send request by name
restclient send api.http --name getUsers

# Send request by index
restclient send api.http --index 2

# Save response to file
restclient send api.http --output response.json

# Only show response body
restclient send api.http --body

# Preview request without sending (dry run)
restclient send api.http --dry-run
```

### env

Manage environments and variables.

```bash
restclient env <subcommand> [args]
```

**Subcommands:**
| Command | Description |
|---------|-------------|
| `list` | List all environments |
| `current` | Show current environment |
| `use <env>` | Switch to an environment |
| `show [env]` | Show variables in an environment |
| `set <env> <var> <value>` | Set a variable |
| `unset <env> <var>` | Remove a variable |
| `create <env>` | Create a new environment |
| `delete <env>` | Delete an environment |

**Examples:**

```bash
# Create environments
restclient env create development
restclient env create production

# Set variables
restclient env set development API_URL https://dev.api.example.com
restclient env set production API_URL https://api.example.com
restclient env set '$shared' API_KEY my-api-key  # Shared across all environments

# Switch environment
restclient env use production

# View variables
restclient env show production
```

### history

View and manage request history. History stores the exact request that was sent, including all headers (such as cookies from the session), so `replay` reproduces the original request exactly.

When no index is provided to `show` or `replay`, an interactive fuzzy-search selector is displayed.

```bash
restclient history <subcommand> [args]
```

**Subcommands:**
| Command | Description |
|---------|-------------|
| `show [index]` | Show details of a specific request, or interactive selection if no index |
| `replay [index]` | Replay a request exactly as it was sent, or interactive selection if no index |
| `stats` | Show history statistics |
| `clear` | Clear all history |

**Examples:**

```bash
# Interactive selection to show request details
restclient history show

# Show request at index 1
restclient history show 1

# Interactive selection to replay a request
restclient history replay

# Replay a specific request (sends exact same request including cookies)
restclient history replay 1

# View statistics
restclient history stats
```

### session

Manage session data including cookies and script variables. Sessions persist data between CLI invocations.

```bash
restclient session <subcommand> [args]
```

**Subcommands:**
| Command | Description |
|---------|-------------|
| `show` | Show session data (cookies, variables) |
| `clear` | Clear session data |
| `list` | List all sessions |

**How Sessions Work:**

Sessions are scoped by directory by default (based on the `.http` file location). This means different projects automatically have isolated sessions. You can also use named sessions with the `--session` flag.

**Flags for `show`:**
| Flag | Description |
|------|-------------|
| `--session` | Show a named session |
| `--dir` | Show session for a specific directory |

**Flags for `clear`:**
| Flag | Description |
|------|-------------|
| `--cookies` | Clear only cookies |
| `--variables` | Clear only variables |
| `--all` | Clear all sessions (not just current) |
| `--session` | Clear a named session |
| `--dir` | Clear session for a specific directory |

**Examples:**

```bash
# Show current directory's session
restclient session show

# Show a named session
restclient session show --session my-api

# List all sessions
restclient session list

# Clear current session
restclient session clear

# Clear only cookies
restclient session clear --cookies

# Clear only variables (script globals)
restclient session clear --variables

# Clear all sessions
restclient session clear --all
```

### completion

Generate shell completion scripts.

```bash
restclient completion [bash|zsh|fish|powershell]
```

### postman

Import and export Postman Collection v2.1.0 files.

#### postman import

Import a Postman collection to `.http` file(s).

```bash
restclient postman import <collection.json> [flags]
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--output` | `-o` | Output path (directory for multi-file, file path for `--single-file`) |
| `--single-file` | | Write all requests to a single .http file |
| `--no-variables` | | Don't include collection variables |
| `--no-scripts` | | Don't include pre-request and test scripts |

**Examples:**

```bash
# Import to current directory (creates Collection-Name/ folder with subfolders)
restclient postman import my-collection.json

# Import to specific directory
restclient postman import my-collection.json -o ./api-requests

# Import as single file (creates my-collection.http in current directory)
restclient postman import my-collection.json --single-file

# Import as single file with custom path
restclient postman import my-collection.json --single-file -o ./api/my-api.http

# Import without variables
restclient postman import my-collection.json --no-variables

# Import without scripts
restclient postman import my-collection.json --no-scripts
```

**Import Features:**

- Converts all request types (GET, POST, PUT, DELETE, PATCH, etc.)
- Preserves folder structure as directories
- Converts authentication (Basic, Bearer, Digest, AWS v4, API Key, OAuth1/2)
- Converts body types (raw, form-urlencoded, form-data, GraphQL)
- Converts pre-request and test scripts to `< {% %}` and `> {% %}` blocks
- Converts collection variables to file variables

#### postman export

Export `.http` file(s) to a Postman collection.

```bash
restclient postman export <file.http> [files...] [flags]
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--output` | `-o` | Output file path (default: collection.json) |
| `--name` | `-n` | Collection name (default: derived from filename) |
| `--description` | `-d` | Collection description |
| `--no-variables` | | Don't include file variables |
| `--no-scripts` | | Don't include scripts |
| `--minify` | | Minify JSON output |

**Examples:**

```bash
# Export single file
restclient postman export api.http

# Export with custom output path
restclient postman export api.http -o my-collection.json

# Export with custom name and description
restclient postman export api.http -n "My API" -d "API collection for testing"

# Export multiple files into one collection
restclient postman export users.http orders.http products.http -n "Full API"

# Export without variables and scripts
restclient postman export api.http --no-variables --no-scripts

# Export minified
restclient postman export api.http --minify
```

**Export Features:**

- Exports all requests with headers and body
- Converts authentication headers to Postman auth objects
- Converts file variables to collection variables
- Converts pre-request scripts (`< {% %}`) to Postman pre-request events
- Converts post-response scripts (`> {% %}`) to Postman test events
- Supports multiple input files merged into one collection

#### Round-Trip Compatibility

You can import a Postman collection, modify the `.http` files, and export back to Postman:

```bash
# Import from Postman (creates ./My-API/ folder with subfolders)
restclient postman import my-api.postman_collection.json

# Or import to a specific directory
restclient postman import my-api.postman_collection.json -o ./api

# Edit .http files as needed
# ...

# Export back to Postman
restclient postman export ./My-API/**/*.http -n "My API" -o updated-collection.json
```

## HTTP File Format

### Basic Request

```http
GET https://api.example.com/users
```

### With Headers

```http
GET https://api.example.com/users
Authorization: Bearer my-token
Accept: application/json
Content-Type: application/json
```

### With Body

```http
POST https://api.example.com/users
Content-Type: application/json

{
    "name": "John Doe",
    "email": "john@example.com"
}
```

### Multiple Requests

Separate requests with `###`:

```http
### Get users
GET https://api.example.com/users

###

### Create user
POST https://api.example.com/users
Content-Type: application/json

{"name": "John"}

###

### Delete user
DELETE https://api.example.com/users/123
```

### Request Metadata

Use comments with `@` prefix for metadata:

```http
# @name myRequest
# @note This is a test request
# @no-redirect
# @no-cookie-jar
GET https://api.example.com/users
```

| Metadata         | Description                    |
| ---------------- | ------------------------------ |
| `@name`          | Name the request for reference |
| `@note`          | Add a description              |
| `@no-redirect`   | Don't follow redirects         |
| `@no-cookie-jar` | Don't use cookie jar           |
| `@prompt`        | Define prompt variables        |

### Query Parameters

Multi-line query parameters:

```http
GET https://api.example.com/users
    ?page=1
    &limit=10
    &sort=name
    &order=asc
```

### Form URL Encoded

```http
POST https://api.example.com/login
Content-Type: application/x-www-form-urlencoded

username=john
&password=secret123
&remember=true
```

### Multipart Form Data

```http
POST https://api.example.com/upload
Content-Type: multipart/form-data; boundary=----FormBoundary

------FormBoundary
Content-Disposition: form-data; name="title"

My Document
------FormBoundary
Content-Disposition: form-data; name="file"; filename="document.pdf"
Content-Type: application/pdf

< ./document.pdf
------FormBoundary--
```

### File References

Include file contents in request body:

```http
POST https://api.example.com/upload
Content-Type: application/json

< ./data.json
```

### GraphQL

```http
POST https://api.example.com/graphql
Content-Type: application/json
X-Request-Type: GraphQL

query GetUser($id: ID!) {
    user(id: $id) {
        name
        email
    }
}

{"id": "123"}
```

GraphQL is auto-detected for URLs ending in `/graphql`:

```http
POST https://api.example.com/graphql
Content-Type: application/json

mutation CreateUser($input: CreateUserInput!) {
    createUser(input: $input) {
        id
        name
    }
}

{"input": {"name": "John", "email": "john@example.com"}}
```

## Variables

### File Variables

Define variables at the top of your `.http` file:

```http
@baseUrl = https://api.example.com
@token = my-secret-token

###

GET {{baseUrl}}/users
Authorization: Bearer {{token}}
```

### Environment Variables

Variables defined in environments via `restclient env set`:

```http
GET {{API_URL}}/users
Authorization: Bearer {{API_KEY}}
```

### System Variables

| Variable                           | Description               | Example                                 |
| ---------------------------------- | ------------------------- | --------------------------------------- |
| `{{$guid}}`                        | UUID v4                   | `550e8400-e29b-41d4-a716-446655440000`  |
| `{{$timestamp}}`                   | Unix timestamp            | `1234567890`                            |
| `{{$timestamp offset unit}}`       | Timestamp with offset     | `{{$timestamp -1 d}}`                   |
| `{{$datetime format}}`             | Formatted datetime        | `{{$datetime iso8601}}`                 |
| `{{$datetime format offset unit}}` | Datetime with offset      | `{{$datetime rfc1123 1 h}}`             |
| `{{$localDatetime format}}`        | Local datetime            | `{{$localDatetime YYYY-MM-DD}}`         |
| `{{$randomInt min max}}`           | Random integer            | `{{$randomInt 1 100}}`                  |
| `{{$processEnv VAR}}`              | OS environment variable   | `{{$processEnv HOME}}`                  |
| `{{$dotenv VAR}}`                  | Variable from `.env` file | `{{$dotenv DATABASE_URL}}`              |
| `{{$prompt name}}`                 | Prompt for input          | `{{$prompt username}}`                  |
| `{{$prompt name description}}`     | Prompt with description   | `{{$prompt apiKey Enter your API key}}` |

**Datetime Formats:**

- `iso8601` - ISO 8601 format (RFC3339)
- `rfc1123` - RFC 1123 format
- Custom: `YYYY-MM-DD`, `HH:mm:ss`, `YYYY-MM-DDTHH:mm:ssZ`, etc.

**Offset Units:**

- `y` - years
- `M` - months
- `w` - weeks
- `d` - days
- `h` - hours
- `m` - minutes
- `s` - seconds
- `ms` - milliseconds

**Examples:**

```http
@requestId = {{$guid}}

POST https://api.example.com/orders
Content-Type: application/json
X-Request-Id: {{requestId}}

{
    "id": "{{$guid}}",
    "timestamp": {{$timestamp}},
    "expiresAt": "{{$datetime iso8601 1 h}}",
    "randomCode": {{$randomInt 1000 9999}},
    "createdBy": "{{$processEnv USER}}"
}
```

### Request Variables

Reference values from previous named requests:

```http
# @name login
POST https://api.example.com/auth/login
Content-Type: application/json

{"username": "john", "password": "secret"}

###

# @name getProfile
GET https://api.example.com/users/me
Authorization: Bearer {{login.response.body.$.token}}
```

**Syntax:**

- `{{requestName.response.body.$.jsonPath}}` - Extract from JSON response
- `{{requestName.response.headers.Header-Name}}` - Extract response header

### URL Encoding

Prefix with `%` to URL-encode a variable:

```http
GET https://api.example.com/search?q={{%searchTerm}}
```

### User Input Variables

User input variables use the `{{:paramName}}` syntax and are ideal for dynamic path parameters and query parameters. On first use, you'll be prompted to enter values. These values are persisted in your session and reused automatically on subsequent requests.

**Syntax:**

```http
### Get post by ID
GET https://api.example.com/posts/{{:id}}

### Get posts with pagination
GET https://api.example.com/posts?page={{:page}}&limit={{:limit}}
```

**Features:**

- **Interactive prompting**: First run prompts for all undefined parameters via a TUI form
- **Session persistence**: Values are saved and reused automatically
- **Force re-prompting**: Use `--force-prompt` or `-p` flag to re-enter values
- **Unique per URL pattern**: Different endpoints maintain separate values
- **Duplicate handling**: Same parameter name used multiple times prompts once

**Examples:**

```bash
# First run - prompts for :id value
restclient send api.http --name getPostById
# Enter value for id: 123

# Second run - uses cached value (id=123)
restclient send api.http --name getPostById

# Force re-prompting to change values
restclient send api.http --name getPostById --force-prompt
# Enter value for id: 456
```

**Request File Example:**

```http
@baseUrl = https://jsonplaceholder.typicode.com

### Get post by ID (prompts for id)
# @name getPostById
GET {{baseUrl}}/posts/{{:id}}
Accept: application/json

### Get posts with pagination (prompts for page and limit)
# @name getPostsPaginated
GET {{baseUrl}}/posts?_page={{:page}}&_limit={{:limit}}
Accept: application/json

### Get comments for a post (prompts for postId)
# @name getCommentsForPost
GET {{baseUrl}}/posts/{{:postId}}/comments
Accept: application/json
```

**Notes:**

- User input variables are processed **before** regular `{{varName}}` variables
- Values are automatically URL-encoded when replaced in URLs
- Session values are cleared with `restclient session clear`
- Use `--no-session` to disable session storage (always prompts)

## Scripting

restclient supports JavaScript scripting for testing responses and sharing data between requests, similar to Postman.

### Session Persistence

By default, restclient persists certain data between CLI invocations. Sessions are scoped by directory (based on the `.http` file location), so different projects have isolated sessions automatically.

| Data Type                                 | Persisted? | Notes                         |
| ----------------------------------------- | ---------- | ----------------------------- |
| Cookies (`Set-Cookie` headers)            | Yes        | Automatic                     |
| Script variables (`client.global.set()`)  | Yes        | Use scripts to store/retrieve |
| File variables (`@var = value`)           | No         | Re-read each invocation       |
| Request variables (`{{req.response...}}`) | No         | Single execution only         |

```bash
# Use a named session for isolation
restclient send api.http --session my-test

# Disable session persistence
restclient send api.http --no-session
```

See [Chaining Requests with Global Variables](#chaining-requests-with-global-variables) for a complete example.

### Post-Response Scripts

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

### Pre-Request Scripts

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

### External Script Files

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

### Script API Reference

#### client Object

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

#### response Object (post-response scripts only)

| Property                          | Description                                  |
| --------------------------------- | -------------------------------------------- |
| `response.status`                 | HTTP status code (e.g., 200)                 |
| `response.statusText`             | Status message (e.g., "200 OK")              |
| `response.body`                   | Response body (parsed as JSON if applicable) |
| `response.headers.valueOf(name)`  | Get header value by name                     |
| `response.headers.valuesOf(name)` | Get all header values by name                |
| `response.contentType.mimeType`   | Response MIME type                           |
| `response.contentType.charset`    | Response charset                             |

#### request Object

| Property                           | Description                   |
| ---------------------------------- | ----------------------------- |
| `request.method`                   | HTTP method (GET, POST, etc.) |
| `request.url`                      | Request URL                   |
| `request.body`                     | Request body                  |
| `request.headers.all`              | Array of all headers          |
| `request.headers.findByName(name)` | Get header value by name      |
| `request.environment.get(name)`    | Get environment variable      |

#### Built-in Utility Functions

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

### Scripting Examples

#### Testing Response Status and Body

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

#### Chaining Requests with Global Variables

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

###

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

#### Validating Headers

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

## Authentication

### Basic Authentication

```http
GET https://api.example.com/protected
Authorization: Basic username:password
```

Or pre-encoded:

```http
GET https://api.example.com/protected
Authorization: Basic dXNlcm5hbWU6cGFzc3dvcmQ=
```

### Digest Authentication

```http
GET https://api.example.com/protected
Authorization: Digest username password
```

### AWS Signature v4

```http
GET https://s3.us-east-1.amazonaws.com/my-bucket
Authorization: AWS accessKeyId secretAccessKey
```

With optional parameters:

```http
GET https://api.example.com/resource
Authorization: AWS accessKeyId secretAccessKey token:sessionToken region:us-west-2 service:execute-api
```

## Configuration

Configuration is stored in `~/.restclient/config.json`:

```json
{
  "followRedirect": true,
  "timeoutInMilliseconds": 0,
  "rememberCookiesForSubsequentRequests": true,
  "defaultHeaders": {
    "User-Agent": "restclient-cli"
  },
  "environmentVariables": {
    "$shared": {
      "API_KEY": "shared-key"
    },
    "development": {
      "API_URL": "https://dev.api.example.com"
    },
    "production": {
      "API_URL": "https://api.example.com"
    }
  },
  "currentEnvironment": "development",
  "insecureSSL": false,
  "proxy": "",
  "excludeHostsForProxy": [],
  "certificates": {},
  "previewOption": "full",
  "showColors": true
}
```

### Configuration Options

| Option                                 | Description                       | Default                            |
| -------------------------------------- | --------------------------------- | ---------------------------------- |
| `followRedirect`                       | Follow HTTP redirects             | `true`                             |
| `timeoutInMilliseconds`                | Request timeout (0 = no timeout)  | `0`                                |
| `rememberCookiesForSubsequentRequests` | Persist cookies between requests  | `true`                             |
| `defaultHeaders`                       | Headers added to all requests     | `{"User-Agent": "restclient-cli"}` |
| `insecureSSL`                          | Skip SSL certificate verification | `false`                            |
| `proxy`                                | HTTP proxy URL                    | `""`                               |
| `excludeHostsForProxy`                 | Hosts to bypass proxy             | `[]`                               |
| `showColors`                           | Colorized output                  | `true`                             |

## Global Flags

| Flag        | Short | Description                                |
| ----------- | ----- | ------------------------------------------ |
| `--config`  | `-c`  | Config file path                           |
| `--env`     | `-e`  | Environment to use                         |
| `--verbose` | `-v`  | Verbose output (includes parsing warnings) |
| `--version` |       | Show version                               |
| `--help`    | `-h`  | Show help                                  |

## Examples

### Complete Workflow

```bash
# Create environments
restclient env create dev
restclient env create prod

# Set environment variables
restclient env set dev baseUrl https://dev.api.example.com
restclient env set prod baseUrl https://api.example.com
restclient env set '$shared' apiKey my-secret-key

# Use development environment
restclient env use dev

# Create request file
cat > api.http << 'EOF'
@userId = 123

### Get user
# @name getUser
GET {{baseUrl}}/users/{{userId}}
X-API-Key: {{apiKey}}

### Update user
# @name updateUser
PUT {{baseUrl}}/users/{{userId}}
Content-Type: application/json
X-API-Key: {{apiKey}}

{
    "name": "Updated Name",
    "updatedAt": "{{$datetime iso8601}}"
}
EOF

# Send request
restclient send api.http --name getUser

# Switch to production
restclient env use prod
restclient send api.http --name getUser

# View history
restclient history list

# Replay last request (1-based index)
restclient history replay 1
```

### CI/CD Usage

```bash
#!/bin/bash
# Run API tests in CI/CD

# Set environment
export API_KEY="${CI_API_KEY}"

# Create request file
cat > test.http << 'EOF'
GET https://api.example.com/health
Authorization: Bearer {{$processEnv API_KEY}}
EOF

# Run with colors disabled via config for CI logs
restclient send test.http --body

# Check response
if [ $? -eq 0 ]; then
    echo "API health check passed"
else
    echo "API health check failed"
    exit 1
fi
```

## Troubleshooting

### Request Not Found

```
Error: request with name 'myRequest' not found
```

Make sure the request has the `@name` metadata:

```http
# @name myRequest
GET https://api.example.com
```

### Variable Not Resolved

```
{{variableName}} appears in output
```

1. Check if the variable is defined in file variables or environment
2. Verify the current environment with `restclient env current`
3. Check variable spelling (case-sensitive)

### Request Validation Errors

restclient validates requests before sending to catch common issues early. If validation fails, you'll see errors like:

```
Request validation failed:
  - URL: URL contains unresolved variables (check your environment configuration)
  - Header:Authorization: header value contains unresolved variables
```

**Common validation errors:**

| Error                                                   | Cause                                     | Solution                                 |
| ------------------------------------------------------- | ----------------------------------------- | ---------------------------------------- |
| `URL is required`                                       | Empty URL                                 | Ensure request has a valid URL           |
| `URL must include scheme`                               | Missing `http://` or `https://`           | Add scheme to URL                        |
| `URL contains unresolved variables`                     | Variables like `{{baseUrl}}` not resolved | Check environment variables and spelling |
| `header value contains unresolved variables`            | Header has `{{variable}}` syntax          | Ensure variable is defined               |
| `Authorization header appears to contain a placeholder` | Auth header has `your-token` or similar   | Replace placeholder with actual token    |
| `URL contains spaces`                                   | Spaces in URL not encoded                 | URL-encode spaces as `%20`               |

**Bypassing validation:**

If you need to send a request with validation issues (e.g., testing error handling), use `--skip-validate`:

```bash
restclient send api.http --skip-validate
```

### Parsing Warnings

When using `--verbose`, restclient shows warnings for invalid request blocks that could not be parsed:

```bash
restclient send api.http --verbose
# Warning: block 2: skipped invalid request block: no request line found
```

Invalid blocks (e.g., blocks with only comments or missing request lines) will not appear in the selection menu. This helps identify syntax issues in multi-request `.http` files.

### SSL Certificate Errors

For self-signed certificates, you can either:

1. Set `insecureSSL` in config:

```bash
# Edit ~/.restclient/config.json
# Set "insecureSSL": true
```

2. Configure certificates:

```json
{
  "certificates": {
    "api.example.com": {
      "cert": "/path/to/cert.pem",
      "key": "/path/to/key.pem"
    }
  }
}
```

### Proxy Issues

Configure proxy in `~/.restclient/config.json`:

```json
{
  "proxy": "http://proxy.example.com:8080",
  "excludeHostsForProxy": ["localhost", "127.0.0.1"]
}
```

## License

MIT License - see [LICENSE](./LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Acknowledgments

- Inspired by [VS Code REST Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client)
- Built with [Cobra](https://github.com/spf13/cobra) and [Viper](https://github.com/spf13/viper)
