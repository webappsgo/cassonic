# API Rules (PART 13, 14, 15)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use unversioned API routes (e.g. `/api/users`) — always versioned
- Use singular resource names (e.g. `/api/v1/user`) — always plural
- Use verbs in routes (e.g. `/api/v1/getUsers`) — use HTTP methods
- Use trailing slashes on routes
- Use uppercase or underscores in routes (use lowercase hyphens)
- Keep "legacy" endpoints (old changed routes) — DELETE them
- Keep old and new route trees alive in parallel
- Redirect between versioned and unversioned aliases — mount SAME handler

## CRITICAL - ALWAYS DO

- Version all API routes: `/api/{api_version}/...`
- Use plural nouns for resources: `/api/{api_version}/users`
- Use HTTP methods for actions: `GET`, `POST`, `PATCH`, `DELETE`
- Use hyphens for multi-word routes: `/api/{api_version}/api-keys`
- Mount versioned and unversioned aliases on the SAME handler (no redirect)
- Expose both Web (HTML) and API (JSON) for every user-facing feature
- Delete superseded routes completely when migrating

## Route Pattern

| Web Route (HTML) | API Route (JSON) |
|------------------|------------------|
| `/` | `/api/{api_version}/` |
| `/server/healthz` | `/api/{api_version}/server/healthz` |
| `/server/{admin_path}` | `/api/{api_version}/server/{admin_path}` |
| `/server/docs/swagger` | `/api/{api_version}/server/swagger` (+ `/api/swagger` alias) |

## Route Compliance Rules

| Rule | Wrong | Correct |
|------|-------|---------|
| Versioning | `/api/users` | `/api/{api_version}/users` |
| Plural nouns | `/api/v1/user` | `/api/v1/users` |
| Lowercase | `/api/v1/Users` | `/api/v1/users` |
| Hyphens | `/api/v1/api_keys` | `/api/v1/api-keys` |
| No trailing slash | `/api/v1/users/` | `/api/v1/users` |
| No verbs | `/api/v1/getUsers` | `GET /api/v1/users` |

## SSL/TLS (PART 15)

- Supports Let's Encrypt automatic cert provisioning
- Self-signed cert fallback for development
- Redirect HTTP → HTTPS when TLS enabled
- HSTS header when in production with TLS

For complete details, see AI.md PART 13, 14, 15
