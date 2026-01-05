# restclient

A powerful command-line HTTP client inspired by the [VS Code REST Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client) extension. Send HTTP requests directly from `.http` and `.rest` files, manage environments, and more.

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

## Documentation

- [**Commands**](docs/commands.md) - Usage of `send`, `env`, `history`, `session`, `completion`, `postman`
- [**HTTP File Format**](docs/file-format.md) - Syntax guide, request metadata, query params, form data, GraphQL
- [**Variables**](docs/variables.md) - File, environment, system, and user input variables
- [**Scripting**](docs/scripting.md) - Pre-request/post-response scripts, API reference, utility functions
- [**Authentication**](docs/authentication.md) - Basic, Digest, AWS Signature v4
- [**Configuration**](docs/configuration.md) - Global/session config, SSL/TLS, Proxy
- [**Troubleshooting**](docs/troubleshooting.md) - Common errors and solutions

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

### From Source

```bash
git clone https://github.com/ideaspaper/restclient.git
cd restclient
go build -o restclient .
```

## Quick Start

1.  **Create a request file** (`api.http`):

    ```http
    @baseUrl = https://api.example.com

    ### Get users
    # @name getUsers
    GET {{baseUrl}}/users
    Authorization: Bearer {{token}}

    ### Create user
    # @name createUser
    POST {{baseUrl}}/users
    Content-Type: application/json

    {
        "name": "John Doe"
    }
    ```

2.  **Send a request:**

    ```bash
    # Interactive selection
    restclient send api.http

    # Send by name
    restclient send api.http --name getUsers
    ```

## License

MIT License - see [LICENSE](./LICENSE) file for details.
