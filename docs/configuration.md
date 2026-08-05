# Configuration

Cassonic generates `server.yml` on first run. No manual editing is required to get started. Every value below has a safe default.

## Configuration File Locations

| Context | Path |
|---------|------|
| Root / privileged | `/etc/local/cassonic/server.yml` |
| User (non-root) | `~/.config/local/cassonic/server.yml` |
| Container | `/config/cassonic/server.yml` |
| Override | `--config {dir}` flag |

## CLI Flag Overrides

Every setting in `server.yml` can be overridden with a CLI flag or environment variable. CLI flags take precedence over environment variables, which take precedence over the config file.

```bash
cassonic --port 4533 --debug --mode development
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MODE` | `production` or `development` | `production` |
| `DEBUG` | Enable debug mode (`true`/`false`/`1`/`0`/`yes`/`no`) | `false` |
| `PORT` | Listen port | `4533` (host), `80` (container) |
| `ADDRESS` | Listen address | `0.0.0.0` |
| `DOMAIN` | Comma-separated `{fqdn}` list (first is primary); skips auto-detection | auto-detected |
| `CONFIG_DIR` | Config directory override | platform default |
| `DATA_DIR` | Data directory override | platform default |

TLS/Let's Encrypt (`--tls`, `--tls-domain`, `--tls-email`, `--tls-cert`, `--tls-key`) and the backup archive location (`--backup`) are CLI-flag-only settings — they are not persisted to `server.yml`.

## Full `server.yml` Reference

```yaml
server:
  # Bind address for the HTTP listener; empty means all interfaces
  address: ""
  # Listen port (default 4533; container default 80)
  port: 4533
  # URL path prefix when running behind a reverse proxy (e.g. /cassonic)
  base_url: ""
  # Application mode: production or development
  mode: production
  # Enable verbose request logging and debug endpoints
  debug: false
  # Logger verbosity: error, warn, info, debug
  log_level: info
  # Explicit {fqdn} list (first is primary); empty auto-detects via reverse-proxy
  # headers, then os.Hostname(), then public IP, then localhost. Same as DOMAIN env var.
  domain: []
  # Additional CIDRs (beyond private/loopback/link-local ranges) trusted to set
  # reverse-proxy headers (X-Forwarded-Host, X-Forwarded-Proto, etc.)
  trusted_proxies: []
  # Smart FQDN detection/live-reload subsystem (AI.md PART 8)
  url_detection:
    # Infer domain patterns from reverse-proxy headers
    learning: true
    # Minimum observed requests before inferring a wildcard pattern
    min_samples: 3
    # Time window for pattern analysis (Go duration string)
    sample_window: 5m
    # Log detected domain/proto changes to the application log
    log_changes: true
    # Allow resolved URL variables to update without a restart
    live_reload: true

database:
  # Path to the SQLite database file; resolved relative to the data dir when empty
  path: ""

paths:
  # Directory for server.yml and other config files
  config: ""
  # Directory for databases, cover art, and other persistent data
  data: ""
  # Directory for application log files
  log: ""
  # Root directories for the music library
  music: []
  # Directory for ephemeral cached files
  cache: ""

auth:
  # HMAC secret for signing session tokens; auto-generated on first run if empty
  jwt_secret: ""
  # Hours a session token remains valid
  session_duration: 168
  # Failed login attempts before the account is locked
  max_login_attempts: 5
  # Minutes a locked account remains inaccessible
  lockout_minutes: 15

scanner:
  # Enable periodic rescanning of music directories
  auto_scan: true
  # Seconds between automatic scans
  scan_interval: 3600
  # Allow the scanner to traverse symbolic links
  follow_symlinks: true
  # Glob patterns; matching paths are skipped during scanning
  exclude_patterns: []

icecast:
  # Activate the built-in Icecast-compatible relay listener
  enabled: false
  # Maximum number of concurrent mount points
  max_mounts: 10

scrobble:
  # Activate play-history/scrobble recording
  enabled: true
  # Seconds to wait before recording a scrobble
  delay: 30

ffmpeg:
  # Absolute path to the ffmpeg binary; auto-detected when empty
  path: ""
  # Allow cassonic to download ffmpeg automatically if not found on the system
  download_auto: true

email:
  # Activate SMTP email delivery; all email features are hidden when false
  enabled: false
  # SMTP server hostname
  host: ""
  # SMTP server port
  port: 587
  # SMTP authentication username
  username: ""
  # SMTP authentication password; set via the admin panel, not this file
  password: ""
  # Sender address used in outgoing mail
  from: ""
  # Enable STARTTLS or implicit TLS depending on port
  tls: true

features:
  # Enable podcast directory and subscription management
  podcasts: true
  # Enable unauthenticated access to shared resources
  public_shares: true
  # Enable self-registration for new users
  user_signup: false
  # Enable country-based access control via built-in GeoIP
  geo_ip: false
  # Enable the Tor hidden service when the tor binary is present
  tor: false
  # Enable on-the-fly audio transcoding via FFmpeg
  transcoding: true
  # Enable automatic metadata enrichment from MusicBrainz
  music_brainz: true

web:
  # Double-submit cookie CSRF protection (PART 16 "CSRF Protection")
  csrf:
    # Activate CSRF validation; set false only for API-only deployments
    # with no browser forms at all
    enabled: true
    # Glob patterns exempt from CSRF validation, e.g. OAuth callbacks
    # and webhook receivers
    exempt_paths: []
```

## Boolean Values

Cassonic accepts all of the following as `true`: `yes`, `true`, `1`, `on`, `enable`, `enabled`, `allow`, and their uppercase variants. Anything else is `false`. This applies to both `server.yml` and environment variables.

## Data Directory Layout

```
~/.local/share/local/cassonic/      # Linux user (non-root)
/var/lib/local/cassonic/            # Linux root
/data/cassonic/                     # Container

├── server.db                       # Main database
├── users.db                        # User accounts
├── security/geoip/                 # GeoIP databases
├── podcasts/{channel_id}/          # Downloaded podcast episodes
└── tor/hidden_service.key          # Tor hidden service key (when enabled)
```

Cover art thumbnails are cached under the cache directory (`{cache}/thumbs/`), not the data directory. Backup archives are written to the directory given by `--backup {dir}` — there is no default location.
