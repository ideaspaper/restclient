# Troubleshooting

## Request Not Found

```
Error: request with name 'myRequest' not found
```

Make sure the request has the `@name` metadata:

```http
# @name myRequest
GET https://api.example.com
```

## Variable Not Resolved

Variables resolve strictly, so missing values stop execution before any request is sent.

1. Check if the variable is defined in file variables, the active environment, or `$shared`
2. Verify the current environment with `restclient env current`
3. Check variable spelling (case-sensitive)
4. Ensure `$prompt` variables have a handler available (TTY session)

## Request Validation Errors

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

## Parsing Warnings

When using `--verbose`, restclient shows warnings for invalid request blocks that could not be parsed:

```bash
restclient send api.http --verbose
# Warning: block 2: skipped invalid request block: no request line found
```

Invalid blocks (e.g., blocks with only comments or missing request lines) will not appear in the selection menu. This helps identify syntax issues in multi-request `.http` files.

## SSL Certificate Errors

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

## Proxy Issues

Configure proxy in `~/.restclient/config.json`:

```json
{
  "proxy": "http://proxy.example.com:8080",
  "excludeHostsForProxy": ["localhost", "127.0.0.1"]
}
```
