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
cassonic --port 4040 --debug --mode development
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MODE` | `production` or `development` | `production` |
| `DEBUG` | Enable debug mode (`true`/`false`/`1`/`0`/`yes`/`no`) | `false` |
| `PORT` | Listen port | `4040` (host), `80` (container) |
| `ADDRESS` | Listen address | `0.0.0.0` |
| `TZ` | Timezone | `America/New_York` |
| `CONFIG_DIR` | Config directory override | platform default |
| `DATA_DIR` | Data directory override | platform default |

## Full `server.yml` Reference

```yaml
server:
  # Bind address for the HTTP listener
  address: 0.0.0.0
  # Listen port (default 4040; container default 80)
  port: 4040
  # Base URL path when running behind a reverse proxy
  baseurl: /
  # Application mode: production or development
  mode: production
  # Enable debug logging (temporary; use for diagnostics only)
  debug: false
  # Server timezone (e.g. America/New_York, Europe/London, UTC)
  timezone: America/New_York

tls:
  # Enable automatic TLS via Let's Encrypt
  enabled: false
  # Domain name for ACME certificate
  domain: ""
  # Email address for ACME registration and renewal alerts
  email: ""
  # Directory to store certificates (relative to config_dir when not absolute)
  cert_dir: ssl/letsencrypt
  # Path to a manually supplied cert file (overrides ACME)
  cert_file: ""
  # Path to a manually supplied key file (overrides ACME)
  key_file: ""

library:
  # List of paths to scan for music files
  paths:
    - /music
  # Scan interval (scheduler; use scheduler section to override)
  scan_interval: 1h
  # Follow symlinks during library scan
  follow_symlinks: false
  # File extensions to include (lowercase, without dot)
  extensions:
    - mp3
    - flac
    - ogg
    - opus
    - m4a
    - aac
    - wav
    - wv
    - ape
    - mpc

database:
  # Directory for SQLite databases (server.db and users.db)
  dir: ""
  # Valkey/Redis URL for caching and clustering (optional)
  # Format: redis://[:password@]host[:port][/db]
  valkey_url: ""

smtp:
  # SMTP host; leave empty to disable email features
  host: ""
  port: 587
  username: ""
  # Password stored securely; do not set here — use the admin panel
  password: ""
  from: ""
  # true = STARTTLS, false = plain (TLS is auto-detected from port 465/587)
  tls: true

metrics:
  # Prometheus metrics endpoint path
  path: /metrics
  # Bearer token required to access /metrics; empty = no auth
  token: ""

geoip:
  # Two-letter ISO 3166-1 alpha-2 country codes to deny
  deny_countries: []
  # Two-letter ISO 3166-1 alpha-2 country codes to allow (takes precedence over deny)
  allow_countries: []

blocklist:
  # IP addresses, CIDR ranges, or hostnames to block
  blocked_ips: []
  # IP addresses or CIDR ranges that bypass all IP and country blocks
  allowlisted_ips: []

rate_limit:
  # Request limit per window for the native API
  native_rps: 100
  # Request limit per window for the Subsonic API
  subsonic_rps: 60
  # Request limit per window for the Ampache API
  ampache_rps: 60
  # Request limit per window for login endpoints
  login_rps: 5

scrobbling:
  lastfm:
    enabled: false
    api_key: ""
    api_secret: ""
  librefm:
    enabled: false
  listenbrainz:
    enabled: false
    token: ""
  maloja:
    enabled: false
    url: ""
    api_key: ""
  funkwhale:
    enabled: false
    url: ""
    token: ""

icecast:
  # Enable the Icecast relay source
  enabled: false
  host: ""
  port: 8000
  mount: /cassonic
  source_password: ""
  # Format: mp3 or ogg
  format: mp3
  # Bitrate in kbps
  bitrate: 128

podcasts:
  # Enable podcast support
  enabled: true
  # Directory to store downloaded episodes (defaults to data_dir/podcasts)
  download_dir: ""
  # Maximum age of episodes to keep; 0 = keep forever
  max_age_days: 90

backup:
  # Directory to write backup archives
  dir: ""
  # Include SSL certificates in backups
  include_ssl: false
  # Include the data directory (covers art, uploads) in backups
  include_data: false

tor:
  # true = always enable; false = disable; auto = enable when tor binary found
  mode: auto
  # Path to tor binary (auto-detected when empty)
  tor_binary: ""

update:
  # Release channel: stable, beta, or daily
  channel: stable
  # Check for updates automatically (never installs without --update yes)
  auto_check: true
```

## Boolean Values

Cassonic accepts all of the following as `true`: `yes`, `true`, `1`, `on`, `enable`, `enabled`, `allow`, `allow`, and their uppercase variants. Anything else is `false`. This applies to both `server.yml` and environment variables.

## Data Directory Layout

```
~/.local/share/local/cassonic/      # Linux user (non-root)
/var/lib/local/cassonic/            # Linux root
/data/cassonic/                     # Container

├── db/
│   ├── server.db                   # Main database
│   └── users.db                    # User accounts
├── covers/                         # Cached cover art
├── geoip/                          # GeoIP databases
├── podcasts/                       # Downloaded podcast episodes
├── uploads/                        # Uploaded music files
├── backup/                         # Local backup archives
└── tor/                            # Tor hidden service keys
```
