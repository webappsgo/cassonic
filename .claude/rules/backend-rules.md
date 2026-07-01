# Backend Rules (PART 9, 10, 11, 32)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Expose stack traces in production error responses
- Use `DROP COLUMN`, `DROP TABLE`, or `DELETE` in schema updates
- Add a new column without a `DEFAULT` value or nullable
- Expose database credentials, internal IPs, or session tokens in any API response
- Expose per-user data in public endpoints (account-existence signals are forbidden)
- Use `ROLLBACK` on schema updates — use idempotent `IF NOT EXISTS` instead

## CRITICAL - ALWAYS DO

- Use `CREATE TABLE IF NOT EXISTS` for all schema definitions
- Use idempotent `ALTER TABLE` for schema updates (ignore "column already exists" errors)
- Return `{"ok": true, "data": {...}}` for success responses
- Return `{"ok": false, "error": "CODE", "message": "..."}` for errors
- Log all errors with context for debugging
- Use appropriate HTTP status codes matching error semantics
- Tier-1 secrets: NEVER expose (passwords, tokens, internal IPs, PII)
- Tier-2 operational info: always public (version, commit, mode, uptime, db_type)
- Tier-3 debug info: gate behind `DEBUG=true`

## API Response Format

Success:
```json
{"ok": true, "data": {}}
```

Error:
```json
{"ok": false, "error": "ERROR_CODE", "message": "Human readable message"}
```

## Database Rules (PART 10)

| Rule | Description |
|------|-------------|
| Self-creating | `CREATE TABLE IF NOT EXISTS` on startup |
| Idempotent updates | `ALTER TABLE ADD COLUMN IF NOT EXISTS` |
| No migrations table | Keep it simple |
| Never destructive | No DROP COLUMN, DROP TABLE, DELETE in schema |
| Defaults required | New columns need DEFAULT or nullable |

## Security Tiers (PART 11)

| Tier | Visibility | Examples |
|------|-----------|---------|
| 1 — NEVER public | Secrets, PII, tokens | DB password, API tokens, user emails, internal IPs |
| 2 — Always public | Operational | version, commit, mode, uptime, db_type |
| 3 — Debug only | Diagnostics | Stack traces, SQL queries, CSRF failure details |

## Tor Hidden Service (PART 32)

Tor is installed in Docker image but the binary controls it. When enabled, `.onion` address is exposed via `/api/autodiscover`.

For complete details, see AI.md PART 9, 10, 11, 32
