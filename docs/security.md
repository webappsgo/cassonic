# Security

## Authentication

- Passwords hashed with Argon2id (never bcrypt)
- Tokens stored as SHA-256 hashes
- Session management with secure HTTP-only cookies

## Transport Security

- TLS 1.2+ required for HTTPS
- Let's Encrypt integration for automatic certificate management
- HSTS enabled by default in production mode

## Headers

All security headers are enabled in production mode:
- `Content-Security-Policy`
- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: strict-origin-when-cross-origin`

## Rate Limiting

Rate limiting is applied to all endpoints to prevent abuse. Limits are configurable.

## Reporting Vulnerabilities

See `.github/SECURITY.md` for the vulnerability disclosure policy.

## Well-Known Endpoints

| Endpoint | Purpose |
|----------|---------|
| `/.well-known/security.txt` | Security contact information |
