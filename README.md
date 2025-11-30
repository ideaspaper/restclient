# rest-client

A powerful command-line HTTP client inspired by the [VS Code REST Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client) extension. Send HTTP requests directly from `.http` and `.rest` files, manage environments, and more.

## Features

- Parse and execute requests from `.http` and `.rest` files
- Multiple environments with variable support
- System variables (UUID, timestamps, random values, etc.)
- File variables and `.env` file support
- Request history with search and replay
- Multipart form data and file uploads
- GraphQL support (queries, mutations, subscriptions)
- Basic, Digest, and AWS Signature v4 authentication
- Cookie jar with persistence
- Colored output with syntax highlighting
- Shell completion for bash, zsh, fish, and PowerShell

## Installation

### Using `go install`

```bash
go install github.com/ideaspaper/restclient@latest
```

This will install the binary as `rest-client` in your `$GOPATH/bin` directory. Make sure `$GOPATH/bin` is in your `PATH`.

### From Source

```bash
# Clone the repository
git clone https://github.com/ideaspaper/restclient.git
cd rest-client

# Build the binary
go build -o restclient .

# Move to PATH (optional)
sudo mv restclient /usr/local/bin/
```

### Dependencies

Requires Go 1.24 or later.

```bash
go mod download
```

## Quick Start

### Basic Usage

```bash
# Send a request from a .http file
restclient send api.http

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

```bash
restclient send <file.http> [flags]
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--name` | `-n` | Select request by name (from `@name` metadata) |
| `--index` | `-i` | Select request by index (0-based) |
| `--headers` | | Only show response headers |
| `--body` | | Only show response body |
| `--output` | `-o` | Save response body to file |
| `--no-history` | | Don't save request to history |

**Examples:**
```bash
# Send first request in file
restclient send api.http

# Send request by name
restclient send api.http --name getUsers

# Send request by index
restclient send api.http --index 2

# Save response to file
restclient send api.http --output response.json

# Only show response body
restclient send api.http --body
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

View and manage request history.

```bash
restclient history <subcommand> [args]
```

**Subcommands:**
| Command | Description |
|---------|-------------|
| `list` | List recent requests |
| `show <index>` | Show details of a specific request |
| `search <query>` | Search request history |
| `replay <index>` | Replay a request from history |
| `stats` | Show history statistics |
| `clear` | Clear all history |

**Examples:**
```bash
# List last 10 requests
restclient history list

# List all requests
restclient history list --all

# Search history
restclient history search "api.example.com"

# Replay a request
restclient history replay 0

# View statistics
restclient history stats
```

### completion

Generate shell completion scripts.

```bash
restclient completion [bash|zsh|fish|powershell]
```

**Setup:**
```bash
# Bash
source <(restclient completion bash)

# Zsh
restclient completion zsh > "${fpath[1]}/_restclient"

# Fish
restclient completion fish > ~/.config/fish/completions/restclient.fish

# PowerShell
restclient completion powershell | Out-String | Invoke-Expression
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

| Metadata | Description |
|----------|-------------|
| `@name` | Name the request for reference |
| `@note` | Add a description |
| `@no-redirect` | Don't follow redirects |
| `@no-cookie-jar` | Don't use cookie jar |
| `@prompt` | Define prompt variables |

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

| Variable | Description | Example |
|----------|-------------|---------|
| `{{$guid}}` | UUID v4 | `550e8400-e29b-41d4-a716-446655440000` |
| `{{$timestamp}}` | Unix timestamp | `1234567890` |
| `{{$timestamp offset unit}}` | Timestamp with offset | `{{$timestamp -1 d}}` |
| `{{$datetime format}}` | Formatted datetime | `{{$datetime iso8601}}` |
| `{{$datetime format offset unit}}` | Datetime with offset | `{{$datetime rfc1123 1 h}}` |
| `{{$localDatetime format}}` | Local datetime | `{{$localDatetime YYYY-MM-DD}}` |
| `{{$randomInt min max}}` | Random integer | `{{$randomInt 1 100}}` |
| `{{$processEnv VAR}}` | OS environment variable | `{{$processEnv HOME}}` |
| `{{$dotenv VAR}}` | Variable from `.env` file | `{{$dotenv DATABASE_URL}}` |
| `{{$prompt name}}` | Prompt for input | `{{$prompt username}}` |
| `{{$prompt name description}}` | Prompt with description | `{{$prompt apiKey Enter your API key}}` |

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

| Option | Description | Default |
|--------|-------------|---------|
| `followRedirect` | Follow HTTP redirects | `true` |
| `timeoutInMilliseconds` | Request timeout (0 = no timeout) | `0` |
| `rememberCookiesForSubsequentRequests` | Persist cookies between requests | `true` |
| `defaultHeaders` | Headers added to all requests | `{"User-Agent": "restclient-cli"}` |
| `insecureSSL` | Skip SSL certificate verification | `false` |
| `proxy` | HTTP proxy URL | `""` |
| `excludeHostsForProxy` | Hosts to bypass proxy | `[]` |
| `showColors` | Colorized output | `true` |

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Config file path |
| `--env` | `-e` | Environment to use |
| `--verbose` | `-v` | Verbose output |
| `--no-color` | | Disable colored output |
| `--version` | | Show version |
| `--help` | `-h` | Show help |

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

# Replay last request
restclient history replay 0
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

# Run with no color for CI logs
restclient send test.http --no-color --body

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

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Acknowledgments

- Inspired by [VS Code REST Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client)
- Built with [Cobra](https://github.com/spf13/cobra) and [Viper](https://github.com/spf13/viper)
