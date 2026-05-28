# Configuration

Configuration is auto-generated on first run. No manual setup required.

## Configuration File Locations

| Context | Path |
|---------|------|
| Privileged (root) | `/etc/local/cassonic/server.yml` |
| User | `~/.config/local/cassonic/server.yml` |
| Container | `/config/cassonic/server.yml` |

## CLI Flags

All settings can be overridden via CLI flags. See `cassonic --help` for the full list.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `MODE` | `production` or `development` (default: `production`) |
| `DEBUG` | Enable debug mode (`true`/`false`) |
| `PORT` | Listen port (default: `80` in container) |
| `ADDRESS` | Listen address (default: `0.0.0.0`) |
| `TZ` | Timezone (default: `America/New_York`) |
| `CONFIG_DIR` | Config directory override |
| `DATA_DIR` | Data directory override |
