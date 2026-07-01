# Configuration Rules (PART 5, 6, 12)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Put YAML comments inline — ALWAYS put them ABOVE the setting
- Expose database passwords, connection strings, or internal IPs in any API response
- Expose stack traces in production error responses
- Expose per-user data or account-existence signals in public endpoints
- Allow path traversal (`..`) in any config value or HTTP path
- Default mode to anything other than `production`

## CRITICAL - ALWAYS DO

- Put YAML comments ABOVE the setting (never inline)
- Validate and normalize ALL paths before use (strip `..`, normalize slashes)
- Use mode priority: `--mode` flag > `MODE` env var > default `production`
- Use debug priority: `--debug` flag > `DEBUG` env var > default `false`
- Sanitize all user input before use — never execute directly
- Apply path security: `path.Clean()`, strip `..`, max length 2048

## Application Modes (PART 6)

| Mode | Debug | Behavior |
|------|-------|----------|
| production | false | Minimal logs, no debug endpoints, cached templates |
| production | true | Debug endpoints enabled, full logging |
| development | false | Verbose logs, hot reload, relaxed CORS |
| development | true | All debug features, admin auth bypassed |

**Default: `production` + `false`**

## YAML Comment Style

```yaml
# CORRECT — comment above
# Server port number
port: 8080

# WRONG — never inline
port: 8080  # Server port number
```

## Key Rules Summary

| Rule | Description |
|------|-------------|
| Mode default | `production` — never `development` in production |
| Path security | Validate all paths; reject `..` traversal |
| YAML comments | Always above; never inline |
| Config file | `server.yml` — YAML, comments above only |
| Secret exposure | Never expose creds, tokens, internal IPs |

For complete details, see AI.md PART 5, 6, 12
