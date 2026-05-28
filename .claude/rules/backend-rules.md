# Backend Rules (PART 9, 10, 11, 32)

Read: AI.md PART 9, 10, 11, 32

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Use bcrypt — Argon2id only
- Store passwords or tokens in plaintext
- Use strconv.ParseBool() — use config.ParseBool()
- Use SELECT * in queries
- Run destructive schema ops (DROP TABLE etc.)
- Embed GeoIP/CVE/blocklist data in binary (download at runtime)

## CRITICAL - ALWAYS DO
- Argon2id for passwords
- SHA-256 hash for tokens
- Parameterized queries always
- SQLite default: server.db and users.db in db_dir
- Valkey/Redis support for caching/clustering
- Rate limiting on all endpoints
- Security headers in production mode
- Tor: auto-enable when tor binary found

## DATABASE PATHS
| Context | Path |
|---------|------|
| Container | `/data/db/sqlite/server.db` and `users.db` |
| Linux privileged | `/var/lib/local/cassonic/db/` |
| Linux user | `~/.local/share/local/cassonic/db/` |

## SECURITY HEADERS (production)
- Content-Security-Policy
- X-Frame-Options: DENY
- X-Content-Type-Options: nosniff
- Referrer-Policy: strict-origin-when-cross-origin
- Strict-Transport-Security (HTTPS only)

---
For complete details, see AI.md PART 9, 10, 11, 32
