# Security

## Authentication

### Passwords

All passwords are hashed with **Argon2id** using parameters tuned for interactive login latency on commodity hardware. bcrypt is never used.

```
Argon2id parameters:
  memory:     64 MiB
  iterations: 3
  parallelism: 4
  key length: 32 bytes
  salt length: 16 bytes (random per password)
```

Admins cannot view, set, or reset passwords for other accounts directly. Password changes use a time-limited invite/reset link — only the account holder can set their own password.

### API Tokens

API tokens are generated as cryptographically random 32-byte values, base64url-encoded. Only the **SHA-256 hash** of the token is stored in the database. The plaintext token is shown once at creation and never again.

Tokens carry scopes (e.g. `read:library`, `write:library`, `admin`) enforced on every request.

### Sessions

Web sessions use HTTP-only, `SameSite=Strict` cookies with a 24-hour default TTL. Session IDs are cryptographically random 32-byte values. Sessions are stored server-side and invalidated on logout.

The `session_cleanup` scheduler job removes expired sessions every hour.

### Subsonic Authentication

Subsonic clients authenticate using a username and a token derived as `MD5(password + salt)`. Cassonic validates this token by re-hashing the stored Argon2id password and comparing the MD5 result. Plaintext passwords are never stored.

---

## Transport Security

### TLS

Cassonic enforces **TLS 1.2** as the minimum version. TLS 1.0 and 1.1 are disabled.

Supported cipher suites (Go TLS defaults, AEAD-only):

- `TLS_AES_128_GCM_SHA256` (TLS 1.3)
- `TLS_AES_256_GCM_SHA384` (TLS 1.3)
- `TLS_CHACHA20_POLY1305_SHA256` (TLS 1.3)
- `TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256` (TLS 1.2)
- `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256` (TLS 1.2)
- `TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384` (TLS 1.2)
- `TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384` (TLS 1.2)
- `TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305` (TLS 1.2)
- `TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305` (TLS 1.2)

### Let's Encrypt

Cassonic integrates with Let's Encrypt via the ACME protocol. Certificates are auto-provisioned and auto-renewed (the `ssl_renewal` scheduler job runs daily and renews certificates with fewer than 30 days remaining).

```yaml
tls:
  enabled: true
  domain: music.example.com
  email: admin@example.com
```

Local certificates (self-signed or from your own CA) are also supported:

```yaml
tls:
  enabled: true
  cert_file: /etc/local/cassonic/ssl/local/cert.pem
  key_file: /etc/local/cassonic/ssl/local/key.pem
```

### HSTS

When TLS is enabled and mode is `production`, cassonic sends:

```
Strict-Transport-Security: max-age=31536000; includeSubDomains
```

---

## HTTP Security Headers

All security headers are active in `production` mode:

| Header | Value |
|--------|-------|
| `Content-Security-Policy` | `default-src 'self'; script-src 'self' 'nonce-{random}'; style-src 'self' 'nonce-{random}'; img-src 'self' data: blob:; media-src 'self' blob:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'` |
| `X-Frame-Options` | `DENY` |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `geolocation=(), microphone=(), camera=()` |
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` (HTTPS only) |

In `development` mode, `Content-Security-Policy` and `Strict-Transport-Security` are relaxed to avoid blocking local development workflows.

---

## Rate Limiting

Rate limiting is applied per IP address to all endpoint groups:

| Endpoint Group | Default Limit | Burst |
|----------------|--------------|-------|
| Native REST API | 100 req/s | 200 |
| Subsonic API | 60 req/s | 120 |
| Ampache API | 60 req/s | 120 |
| Login / auth | 5 req/s | 5 |

Requests over the limit receive `429 Too Many Requests`. Configure limits in `server.yml` under `rate_limit`.

---

## IP Filtering

### Blocklist

Individual IPs, CIDR ranges, and hostnames can be blocked. Blocked requests receive `403 Forbidden`.

```yaml
blocklist:
  blocked_ips:
    - 203.0.113.0/24
    - 198.51.100.42
```

### Allowlist

Allowlisted IPs bypass all IP blocking and country filtering:

```yaml
blocklist:
  allowlisted_ips:
    - 192.168.1.0/24
    - 10.0.0.0/8
```

RFC 1918 private addresses are never blocked regardless of configuration.

---

## GeoIP Country Filtering

Cassonic downloads the free sapics/ip-location-db database on first run (no API key required). The `geoip_update` scheduler job refreshes the database weekly.

```yaml
geoip:
  # Block all traffic from these countries
  deny_countries:
    - RU
    - CN

  # Or: allow only these countries (takes precedence over deny_countries)
  allow_countries:
    - US
    - CA
    - GB
```

Allowlisted IPs always bypass country filtering.

---

## Tor Hidden Service

When the `tor` binary is found on the system (or installed in the container), cassonic automatically starts a Tor hidden service. The `.onion` address is displayed in the startup banner and on the admin dashboard.

```yaml
tor:
  mode: auto   # auto | true | false
```

No manual `torrc` editing is required. Cassonic manages the hidden service directory, key generation, and the `torrc` fragment entirely.

---

## Metrics Endpoint

The Prometheus-compatible `/metrics` endpoint is intended for internal monitoring systems only. Secure it with:

1. **Network-level control** — bind to an internal interface or use a reverse proxy with IP restrictions
2. **Bearer token authentication** — set `metrics.token` in `server.yml`

```yaml
metrics:
  path: /metrics
  token: your-secret-metrics-token
```

```bash
curl http://localhost:4040/metrics \
  -H "Authorization: Bearer your-secret-metrics-token"
```

---

## Middleware Order

The request processing order follows the spec (PART 13):

1. **IP Allowlist** — allowlisted IPs pass through unconditionally
2. **IP Blocklist** — blocked IPs receive `403`
3. **Rate Limiting** — per-IP sliding window
4. **GeoIP Filter** — country-based allow/deny
5. **Authentication** — session cookie or Bearer token
6. **Authorization** — role and scope enforcement

---

## Security Contact

See `/.well-known/security.txt` for the vulnerability disclosure policy and contact details.

---

## Vulnerability Reporting

To report a security vulnerability:

1. Do **not** open a public GitHub issue.
2. Email `security@cassonic.local` with a description of the vulnerability and reproduction steps.
3. Expect a response within 72 hours.

We follow responsible disclosure: vulnerabilities are fixed before public announcement.
