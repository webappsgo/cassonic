# API Rules (PART 13, 14, 15)

Read: AI.md PART 13, 14, 15

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Unversioned API routes
- Plural-only nouns without versioning
- Trailing slashes in routes
- Verbs in route paths (use HTTP methods)

## CRITICAL - ALWAYS DO
- Versioned routes: /api/v1/...
- Plural nouns in paths
- JSON response: {"ok":true,"data":{...}}
- RFC 7807 error body
- X-Request-ID propagation
- Health endpoint: /health (always public)
- Version endpoint: /version or /api/version
- Swagger: /swagger/
- GraphQL: /graphql/

## MIDDLEWARE ORDER
Allowlist → Blocklist → RateLimit → GeoIP → Auth

## SSL/TLS
- Let's Encrypt integration (auto-cert)
- TLS 1.2+ minimum
- Cert paths: {config_dir}/ssl/letsencrypt/ and ssl/local/

---
For complete details, see AI.md PART 13, 14, 15
