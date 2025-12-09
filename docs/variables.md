# Variables

## File Variables

Define variables at the top of your `.http` file:

```http
@baseUrl = https://api.example.com
@token = my-secret-token

GET {{baseUrl}}/users
Authorization: Bearer {{token}}
```

> **Strict resolution:** File variables must resolve successfully. Missing or misspelled variable names will stop the request with a validation error.

## Environment Variables

Variables defined in environments via `restclient env set`:

```http
GET {{API_URL}}/users
Authorization: Bearer {{API_KEY}}
```

If a variable is missing from the active environment (or `$shared`), processing will fail with a descriptive error.

## System Variables

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

> **Note:** `$prompt` requires a prompt handler. When none is configured (e.g., during CI runs), resolution will fail.

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

## Request Variables

Reference values from previous named requests:

```http
### Login
# @name login
POST https://api.example.com/auth/login
Content-Type: application/json

{"username": "john", "password": "secret"}

### Get Profile
# @name getProfile
GET https://api.example.com/users/me
Authorization: Bearer {{login.response.body.$.token}}
```

**Syntax:**

- `{{requestName.response.body.$.jsonPath}}` - Extract from JSON response
- `{{requestName.response.headers.Header-Name}}` - Extract response header

If the referenced request or path is missing, resolution will fail immediately.

## URL Encoding

Prefix with `%` to URL-encode a variable:

```http
GET https://api.example.com/search?q={{%searchTerm}}
```

## User Input Variables

User input variables use the `{{:paramName}}` syntax and are ideal for dynamic path parameters, query parameters, headers, and request bodies. You'll be prompted to enter values each time you run a request.

**Syntax:**

```http
{{:paramName}}           # Regular input
{{:paramName!secret}}    # Secret input (masked in UI and logs)
```

**Secret Inputs:**

Use the `!secret` suffix for sensitive values like passwords, API keys, or tokens:

```http
### Login with secret password
POST https://api.example.com/login
Content-Type: application/json

{
  "username": "{{:username}}",
  "password": "{{:password!secret}}"
}

### API request with secret token
GET https://api.example.com/data
Authorization: Bearer {{:apiToken!secret}}
```

Secret inputs have special handling:

- **Masked during entry**: Password-style input field (characters hidden)
- **Masked in output**: Displayed as `<secret>` in CLI output and TUI summaries

**Supported Locations:**

| Location              | URL Encoding | Example                                     |
| --------------------- | ------------ | ------------------------------------------- |
| URL                   | Yes          | `https://api.example.com/users/{{:userId}}` |
| Header values         | No           | `Authorization: Bearer {{:token!secret}}`   |
| Request body          | No           | `{"password": "{{:password!secret}}"}`      |
| Multipart form fields | No           | Text field value: `{{:description}}`        |
| Multipart file paths  | No           | `< {{:filePath}}`                           |

**Features:**

- **Interactive prompting**: Each run prompts for all parameters via a TUI form
- **Secret inputs**: Use `{{:param!secret}}` for masked password-style input
- **Unique per request**: Values are collected fresh on every execution
- **Duplicate handling**: Same parameter name used multiple times prompts once

**Basic Examples:**

```http
### Get post by ID (prompts for id)
GET https://api.example.com/posts/{{:id}}

### Get posts with pagination (prompts for page and limit)
GET https://api.example.com/posts?page={{:page}}&limit={{:limit}}

### Authentication header (prompts for token, masked input, no URL encoding)
GET https://api.example.com/protected
Authorization: Bearer {{:apiToken!secret}}

### JSON body (prompts for username and password, password is masked)
POST https://api.example.com/login
Content-Type: application/json

{
  "username": "{{:username}}",
  "password": "{{:password!secret}}"
}

### Form URL-encoded body (password is masked)
POST https://api.example.com/login
Content-Type: application/x-www-form-urlencoded

username={{:username}}&password={{:password!secret}}
```

**Multipart Form Data Examples:**

```http
### Multipart with dynamic text field
POST https://api.example.com/upload
Content-Type: multipart/form-data; boundary=----FormBoundary

------FormBoundary
Content-Disposition: form-data; name="description"

{{:fileDescription}}
------FormBoundary
Content-Disposition: form-data; name="file"; filename="document.pdf"
Content-Type: application/pdf

< ./document.pdf
------FormBoundary--

### Multipart with dynamic file path
POST https://api.example.com/upload
Content-Type: multipart/form-data; boundary=----FormBoundary

------FormBoundary
Content-Disposition: form-data; name="file"; filename="upload.pdf"
Content-Type: application/pdf

< {{:filePath}}
------FormBoundary--
```

**Shared Values:**

When the same parameter name is used multiple times, you only get prompted once:

```http
GET https://api.example.com/users/{{:userId}}/posts
Authorization: Bearer {{:token}}
X-User-Id: {{:userId}}
```

The `userId` value is shared between the URL and the `X-User-Id` header.

**Command Line Examples:**

```bash
# Prompts for :id value
restclient send api.http --name getPostById
# Enter value for id: 123

# Prompts again for new value (values are not cached)
restclient send api.http --name getPostById
# Enter value for id: 456
```

**Notes:**

- User input variables are processed **before** regular `{{varName}}` variables
- Values are automatically URL-encoded when replaced in URLs (not in headers/body)
- Use `{{:param!secret}}` for sensitive values - they are masked in the TUI and CLI output
- Values are collected fresh on every run (no caching between invocations)
