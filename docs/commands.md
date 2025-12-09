# Commands

## send

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

## env

Manage environments and variables.

```bash
restclient env <subcommand> [args]
```

Environment variables are stored **per-session** in `~/.restclient/session/<kind>/<id>/environments.json`. Each session automatically gets `$shared` and `development` environments on creation. The `$shared` environment contains variables available to all environments within that session.

Sessions are scoped by the directory of your `.http` file (via a hash) or by an explicit `--session` name. This keeps projects isolated automatically.

**Subcommands:**
| Command | Description |
|---------|-------------|
| `list` | List all environments in the current session |
| `current` | Show current environment |
| `use <env>` | Switch to an environment |
| `show [env]` | Show variables in an environment |
| `set <env> <var> <value>` | Set a variable |
| `unset <env> <var>` | Remove a variable |
| `create <env>` | Create a new environment |
| `delete <env>` | Delete an environment |

**Flags:**
| Flag | Description |
|------|-------------|
| `--session` | Use a named session instead of directory-based |
| `--dir` | Use session for a specific directory |

**Examples:**

```bash
# Create environments in your current session
restclient env create development
restclient env create production

# Set variables (stored in that session's environments.json)
restclient env set development API_URL https://dev.api.example.com
restclient env set production API_URL https://api.example.com
restclient env set '$shared' API_KEY my-api-key  # Shared across all environments

# Switch environment (persists to session config)
restclient env use production

# View variables
restclient env show production

# Use a named session (shared across directories)
restclient env set production API_KEY secret123 --session my-shared-api
```

## history

View and manage request history. History stores the exact request that was sent, including all headers (such as cookies from the session), so `replay` reproduces the original request exactly. Each invocation resolves the same session scoping rules as `send`, meaning history entries are separated per directory hash or `--session` name.

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

## session

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

## completion

Generate shell completion scripts.

```bash
restclient completion [bash|zsh|fish|powershell]
```

## postman

Import and export Postman Collection v2.1.0 files.

### postman import

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

### postman export

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

### Round-Trip Compatibility

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

### Secret Variable Interoperability

restclient preserves secret variable metadata when importing from and exporting to Postman collections:

**Import:** Postman variables marked as `type: "secret"` or with `[secret]` in their description are converted to the `{{:varName!secret}}` syntax in `.http` files:

```http
# Postman variable with type: "secret"
@apiKey = {{:apiKey!secret}}
```

**Export:** File variables using `{{:varName!secret}}` syntax are exported with `type: "secret"` and `[secret]` in the description, ensuring the secret status is preserved when re-importing:

```http
# This .http file variable
@password = {{:password!secret}}

# Becomes a Postman variable with:
# - type: "secret"
# - description: "[secret]"
```

This enables full round-trip compatibility for secret variables between restclient and Postman.
