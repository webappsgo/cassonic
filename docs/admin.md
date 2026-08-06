# Admin Panel

The cassonic admin panel is at `/server/admin/`. It provides full control over every aspect of the server without requiring CLI access.

## First-Time Setup

On first run, cassonic prints a one-time, 32-character hexadecimal **setup token** in the startup banner:

```
╔══════════════════════════════════════════════╗
║  cassonic — setup required                   ║
║  Visit http://localhost:64xxx to continue     ║
║  Setup token: 3f9a1c2b7d8e405fa1b2c3d4e5f60718 ║
╚══════════════════════════════════════════════╝
```

Open the URL in a browser and enter the setup token. On success you're taken through a 6-step setup wizard at `/config/setup`:

1. **Create Admin Account** — username (defaults to `administrator`) and password (leave blank to generate a random password)
2. **API Token** — an admin API token is generated and shown once; copy it now, it cannot be retrieved again
3. **Server Configuration** — app name, domain/FQDN, mode (production/development), timezone
4. **Security Settings** — optional backup-archive encryption password (AES-256-GCM) and a 2FA-enabled toggle for this admin
5. **Optional Services** — confirms the HTTPS/certificate configuration has been reviewed
6. **Complete** — saves `server.yml`, creates the admin account, marks the setup token consumed, logs the new admin in, and redirects to the admin panel

The wizard is stateless: each step's form carries every previously entered value forward as hidden fields, so there is no server-side "in-progress setup" record beyond the setup token itself. The setup token is invalidated the moment step 6 completes and cannot be re-used; if it's lost before setup finishes, resetting `server.db` is required to generate a new one.

!!! note "Primary Admin"
    The admin account created by the setup wizard is the Primary Admin — the admin with the lowest ID. It cannot be deleted except via `--maintenance setup`. All other admin accounts can be removed normally.

## Accessing the Admin Panel

Navigate to `http://your-server:4040/server/admin/` and log in with your admin credentials.

The admin panel path changes if you set `--baseurl`. For `--baseurl /music`, the admin panel is at `/music/server/admin/`.

## Dashboard

The dashboard shows:

- Server version, uptime, and mode
- Library statistics (artists, albums, songs, total duration, disk usage)
- Active library scan progress (if a scan is running)
- Recent scrobbling activity
- Scheduler job status (last run, next run, success/failure)
- System resource usage (memory, CPU, goroutines)

## Library Management

### Adding a Library Path

1. Go to **Admin → Library → Paths**
2. Click **Add Path**
3. Enter the absolute path to your music directory
4. Choose whether to follow symlinks
5. Save — cassonic begins scanning immediately

Multiple library paths are supported. Each path is scanned independently.

### Triggering a Scan

Click **Scan Now** on any library path to start an immediate scan. Scan progress is shown in real time on the dashboard.

The scheduler runs incremental scans automatically (configurable interval, default: 1 hour). The first full scan after adding a new path indexes all files.

### File Upload

Navigate to **Admin → Library → Upload** to upload music files directly through the browser. Files are placed in the configured upload directory and indexed during the next scan.

## User Management

Admin accounts and regular user accounts are managed separately.

### Admin Accounts

Navigate to **Admin → Admins**:

- **Create admin** — enter username, email, and an invite link is sent (or shown if SMTP is not configured)
- **Edit admin** — change username or email
- **Delete admin** — removes the account (Primary Admin cannot be deleted)
- **Reset password** — generates a new invite link; the admin sets their own password

!!! warning "Admins cannot set passwords for other admins"
    Only the account holder can set their own password via the invite/reset link. Admins see no password fields for other accounts.

### Regular Users

Navigate to **Admin → Users** (requires multi-user mode to be enabled in settings):

- **Registration mode** — open, invite-only, admin-only, or disabled
- **Create user** — sends an invite email (or shows the link)
- **Edit user** — change username, email, or role
- **Suspend user** — block access without deleting data
- **Delete user** — permanently removes the account and associated data
- **API tokens** — view and revoke user API tokens (token values are never shown)

## Music Metadata

### Tag Editor

Navigate to **Admin → Library → Tags** or click any song's edit button:

- Edit title, artist, album, album artist, year, track number, disc number, genre, comment
- Changes write through to the music file on disk using the appropriate tag format (ID3v2, Vorbis Comment, etc.)
- Changes re-index the file immediately

### MusicBrainz Lookup

On any song, album, or artist page, click **Lookup on MusicBrainz** to automatically fetch and apply metadata from MusicBrainz. Cassonic matches by MBID if present, or by title/artist/album for fuzzy matching.

## Scrobbling

Navigate to **Admin → Integrations → Scrobbling** to configure scrobbling services per user or server-wide:

| Service | Configuration |
|---------|--------------|
| Last.fm | API key + API secret (OAuth flow in browser) |
| LibreFM | No API key needed (compatible with Last.fm protocol) |
| ListenBrainz | User token from listenbrainz.org |
| Maloja | Server URL + API key |
| Funkwhale | Server URL + user token |

Scrobbling is triggered when a track has been played for at least 50% of its duration or 4 minutes, whichever is less.

## Podcasts

Navigate to **Admin → Podcasts**:

- **Add feed** — paste an RSS/Atom podcast URL; cassonic fetches and indexes episodes
- **Refresh all** — manually trigger a feed refresh (scheduler handles this automatically)
- **Delete feed** — removes the feed and all downloaded episodes
- **Episode management** — mark as played, download, or delete individual episodes

## Icecast Relay

Navigate to **Admin → Integrations → Icecast** to configure the built-in Icecast source:

- Enable/disable the relay
- Set the Icecast server address, port, mount point, and source password
- Choose format (MP3 or Ogg Vorbis) and bitrate
- Set the stream metadata (station name, genre, description, URL)

## Backup and Restore

Navigate to **Admin → Maintenance → Backup**:

### Create a Backup

Click **Backup Now** to create an immediate archive. Optionally enable:

- **Encrypt backup** — AES-256-GCM with Argon2id key derivation; you must remember the password — there is no recovery path
- **Include SSL certificates** — include Let's Encrypt certificates in the archive
- **Include data directory** — include cover art, uploads, and downloaded episodes

Backups are named `cassonic_backup_YYYY-MM-DD_HHMMSS.tar.gz[.enc]`.

### Restore

1. Upload a backup archive or select an existing one
2. Enter the decryption password if the backup is encrypted
3. Click **Restore** — cassonic restores config and databases, then restarts

!!! danger "Restore overwrites all current data"
    A restore replaces the current configuration and databases. Make sure you have a recent backup of the current state before restoring.

### Scheduled Backups

The built-in scheduler runs:

- **Hourly backup** — retains the last 24 hourly backups
- **Daily backup** — retains the last 30 daily backups

Configure retention and backup destination in `server.yml` or via **Admin → Settings → Backup**.

## Scheduler

Navigate to **Admin → Maintenance → Scheduler** to see all built-in jobs:

| Job | Default Interval | Description |
|-----|-----------------|-------------|
| `ssl_renewal` | Daily | Renew Let's Encrypt certificates |
| `geoip_update` | Weekly | Download updated GeoIP databases |
| `blocklist_update` | Daily | Refresh IP blocklists |
| `cve_update` | Daily | Check for known CVEs in dependencies |
| `session_cleanup` | Hourly | Remove expired sessions |
| `token_cleanup` | Daily | Remove expired API tokens |
| `log_rotation` | Daily | Rotate and compress log files |
| `backup_daily` | Daily | Create daily backup archive |
| `backup_hourly` | Hourly | Create hourly backup archive |
| `healthcheck_self` | Every 5 min | Internal self-health check |
| `tor_health` | Every 15 min | Verify Tor hidden service is reachable |
| `cluster_heartbeat` | Every 30 s | Cluster node heartbeat (cluster mode) |
| `library_scan` | Hourly | Incremental library scan |
| `podcast_refresh` | Every 4 h | Refresh podcast feeds |
| `scrobble_retry` | Every 15 min | Retry failed scrobbles |

Click **Run Now** to trigger any job immediately.

## Updates

Navigate to **Admin → Maintenance → Updates**:

- **Check for updates** — shows current and latest version without installing
- **Install update** — downloads, verifies SHA256, replaces the binary, and restarts
- **Release channel** — switch between stable, beta, and daily builds

## Settings

Navigate to **Admin → Settings** to manage all configuration without editing `server.yml`:

- **General** — server name, base URL, timezone, mode
- **Network** — address, port, TLS/Let's Encrypt
- **Email** — SMTP configuration and test button
- **Security** — rate limits, GeoIP filtering, IP blocklists, session timeout
- **Library** — scan paths, intervals, file extensions
- **Metrics** — path and optional Bearer token
- **Tor** — mode and binary path

All settings changes take effect immediately without a restart (where applicable).
