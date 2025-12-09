# Configuration

restclient separates configuration into two layers:

- **Global config:** `~/.restclient/config.json` stores CLI display preferences only
- **Session config:** `~/.restclient/session/<kind>/<id>/config.json` stores all HTTP behavior settings

## Global Config

CLI display preferences are stored in `~/.restclient/config.json`:

```json
{
  "previewOption": "full",
  "showColors": true
}
```

| Option          | Description                               | Default |
| --------------- | ----------------------------------------- | ------- |
| `previewOption` | Output mode: `full`, `headers`, or `body` | `full`  |
| `showColors`    | Colorized terminal output                 | `true`  |

## Session Config

Sessions are scoped by directory (based on the `.http` file location) or by an explicit `--session` name. Each session directory (`~/.restclient/session/<kind>/<id>/`) contains:

| File                | Description                                    |
| ------------------- | ---------------------------------------------- |
| `config.json`       | All HTTP behavior settings for this session    |
| `environments.json` | Environment variables for this session         |
| `cookies.json`      | HTTP cookies from responses                    |
| `variables.json`    | Script variables set via `client.global.set()` |

The session `config.json` controls all HTTP behavior:

```json
{
  "version": 1,
  "environment": {
    "current": "",
    "rememberCookiesForSubsequentRequests": true,
    "defaultHeaders": {
      "User-Agent": "restclient-cli"
    }
  },
  "http": {
    "timeoutInMilliseconds": 0,
    "followRedirect": true,
    "cookieJar": "cookies.json"
  },
  "tls": {
    "insecureSSL": false,
    "proxy": "",
    "excludeHostsForProxy": [],
    "certificates": {}
  }
}
```

### Session Config Options

| Section       | Option                                 | Description                              | Default                            |
| ------------- | -------------------------------------- | ---------------------------------------- | ---------------------------------- |
| `environment` | `current`                              | Active environment name                  | `""`                               |
| `environment` | `rememberCookiesForSubsequentRequests` | Persist cookies between requests         | `true`                             |
| `environment` | `defaultHeaders`                       | Headers added to all requests            | `{"User-Agent": "restclient-cli"}` |
| `http`        | `timeoutInMilliseconds`                | Request timeout (0 = no timeout)         | `0`                                |
| `http`        | `followRedirect`                       | Follow HTTP redirects                    | `true`                             |
| `http`        | `cookieJar`                            | Cookie storage file name                 | `"cookies.json"`                   |
| `tls`         | `insecureSSL`                          | Skip SSL certificate verification        | `false`                            |
| `tls`         | `proxy`                                | HTTP proxy URL                           | `""`                               |
| `tls`         | `excludeHostsForProxy`                 | Hosts to bypass proxy                    | `[]`                               |
| `tls`         | `certificates`                         | Per-host client certificates (see below) | `{}`                               |

### Client Certificates

To configure client certificates for mutual TLS:

```json
{
  "tls": {
    "certificates": {
      "api.example.com": {
        "cert": "/path/to/client.crt",
        "key": "/path/to/client.key"
      }
    }
  }
}
```

## Environment Variables

Environment variables are stored **per-session** in `environments.json`:

```json
{
  "environmentVariables": {
    "$shared": {
      "API_KEY": "my-api-key"
    },
    "development": {
      "API_URL": "https://dev.api.example.com"
    },
    "production": {
      "API_URL": "https://api.example.com"
    }
  }
}
```

The `$shared` environment contains variables available to all environments within that session. New sessions automatically get `$shared` and `development` environments.

**Security:**

- Session files are written with `0600` permissions (owner read/write only)
- Session directories should NOT be committed to version control

## Global Flags

| Flag        | Short | Description                                |
| ----------- | ----- | ------------------------------------------ |
| `--config`  | `-c`  | Config file path                           |
| `--env`     | `-e`  | Environment to use                         |
| `--verbose` | `-v`  | Verbose output (includes parsing warnings) |
| `--version` |       | Show version                               |
| `--help`    | `-h`  | Show help                                  |
