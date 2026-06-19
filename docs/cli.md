# CLI Reference

Cassonic ships two binaries:

- `cassonic` — the server binary, runs as a background service
- `cassonic-cli` — the companion CLI/TUI, connects to a running server

---

## `cassonic` — Server Binary

### Synopsis

```
cassonic [flags]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--help` | `-h` | Show help |
| `--version` | `-v` | Show version and build info |
| `--mode {production\|development}` | | Application mode (default: `production`) |
| `--config {dir}` | | Configuration directory override |
| `--data {dir}` | | Data directory override |
| `--log {dir}` | | Log directory override |
| `--pid {file}` | | Write PID to file |
| `--address {addr}` | | Bind address (default: `0.0.0.0`) |
| `--port {port}` | | Listen port (default: `4533`) |
| `--cache {dir}` | | Cache directory override |
| `--backup {dir}` | | Directory for backup archives (optional) |
| `--baseurl {path}` | | Base URL path for reverse proxy (e.g. `/music`) |
| `--debug` | | Enable debug logging |
| `--status` | | Show server status and exit |
| `--shell {completions\|init} [SHELL]` | | Print shell completions or init script; SHELL: bash, zsh, fish |

### Service Management

```
cassonic --service {start|restart|stop|reload|--install|--uninstall|--disable|--help}
```

| Sub-command | Description |
|-------------|-------------|
| `start` | Start the service |
| `restart` | Restart the service |
| `stop` | Stop the service |
| `reload` | Reload configuration without restarting |
| `--install` | Install cassonic as a system service |
| `--uninstall` | Stop, disable, and uninstall the service (deletes all data) |
| `--disable` | Disable auto-start without uninstalling |
| `--help` | Show service sub-command help |

### Maintenance

```
cassonic --maintenance {backup|restore|update|mode|setup|--help}
```

| Sub-command | Description |
|-------------|-------------|
| `backup` | Create a backup archive immediately |
| `restore {file}` | Restore from a backup archive |
| `update` | Alias for `--update yes` |
| `mode {production\|development}` | Switch application mode |
| `setup` | Re-run first-time setup |
| `--help` | Show maintenance sub-command help |

### Updates

```
cassonic --update [check|yes|branch {stable|beta|daily}]
```

| Sub-command | Description |
|-------------|-------------|
| `check` | Check for a new version without installing |
| `yes` | Download and install the latest release, then restart |
| `branch stable` | Switch to the stable release channel |
| `branch beta` | Switch to the beta release channel |
| `branch daily` | Switch to the daily build channel |

### Daemon Mode

```
cassonic --daemon
```

Detaches from the terminal and runs in the background. PID is written to `--pid` file if specified.

### Examples

```bash
# Start in development mode on port 8080
cassonic --mode development --port 8080

# Run as a daemon with a custom config directory
cassonic --daemon --config /opt/cassonic/config --data /opt/cassonic/data

# Check for updates
cassonic --update check

# Install as a system service
sudo cassonic --service --install

# Trigger an immediate backup (writes to the configured backup dir)
cassonic --maintenance backup

# Trigger an immediate backup to a specific directory
cassonic --backup /tmp/mybackup --maintenance backup
```

---

## `cassonic-cli` — Companion CLI

`cassonic-cli` connects to a running cassonic server and provides a terminal interface for browsing, playback control, and administration.

### Synopsis

```
cassonic-cli [flags] [command] [args...]
```

### Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--help` | `-h` | Show help |
| `--version` | `-v` | Show version |
| `--server {url}` | | Server URL (default: `http://localhost:4533`) |
| `--token {token}` | | API Bearer token (or set `CASSONIC_TOKEN` env var) |
| `--debug` | | Enable debug output |
| `--color {auto\|yes\|no}` | | Color output control |
| `--format {table\|json\|plain}` | | Output format (default: `table`) |

### Commands

#### Authentication

```bash
# Log in and save token to keyring
cassonic-cli auth login --server http://localhost:4533

# Log out and remove saved token
cassonic-cli auth logout

# Show current authentication status
cassonic-cli auth status
```

#### Library

```bash
# List library paths
cassonic-cli library list

# Trigger a library scan
cassonic-cli library scan

# Watch scan progress
cassonic-cli library scan --watch
```

#### Artists, Albums, Songs

```bash
# List artists (paginated)
cassonic-cli artists

# Search
cassonic-cli search "pink floyd"

# Show an album
cassonic-cli album 42

# Show album songs
cassonic-cli album 42 --songs
```

#### Playlists

```bash
# List playlists
cassonic-cli playlists

# Create a playlist
cassonic-cli playlist create "Road Trip"

# Add songs to a playlist
cassonic-cli playlist add 7 --songs 101 102 103

# Delete a playlist
cassonic-cli playlist delete 7
```

#### Tag Editing

```bash
# Read tags for a song
cassonic-cli tags get 101

# Update a tag
cassonic-cli tags set 101 --title "Comfortably Numb" --artist "Pink Floyd"
```

#### Admin

```bash
# Trigger a backup
cassonic-cli admin backup

# List backups
cassonic-cli admin backups

# Restore from a backup
cassonic-cli admin restore cassonic_backup_2026-05-31_120000.tar.gz

# Show scheduler job status
cassonic-cli admin scheduler

# Run a scheduler job immediately
cassonic-cli admin scheduler run ssl_renewal

# List users
cassonic-cli admin users

# Create a user
cassonic-cli admin users create --username alice --email alice@example.com
```

#### Podcasts

```bash
# List podcast feeds
cassonic-cli podcasts

# Add a podcast feed
cassonic-cli podcasts add "https://example.com/feed.rss"

# List episodes for a feed
cassonic-cli podcasts episodes 3
```

#### Server Info

```bash
# Show server version and health
cassonic-cli server info

# Show server status
cassonic-cli server status
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `CASSONIC_SERVER` | Server URL (overrides `--server`) |
| `CASSONIC_TOKEN` | API Bearer token (overrides `--token`) |
| `CASSONIC_COLOR` | Color mode: `auto`, `yes`, `no` |
| `CASSONIC_FORMAT` | Output format: `table`, `json`, `plain` |
