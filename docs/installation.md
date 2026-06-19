# Installation

## Pre-built Binary

Download the latest binary for your platform from the [releases page](https://github.com/local/cassonic/releases).

=== "Linux amd64"

    ```bash
    curl -Lo cassonic https://github.com/local/cassonic/releases/latest/download/cassonic-linux-amd64
    curl -Lo cassonic-cli https://github.com/local/cassonic/releases/latest/download/cassonic-cli-linux-amd64
    chmod +x cassonic cassonic-cli
    sudo mv cassonic cassonic-cli /usr/local/bin/
    ```

=== "Linux arm64"

    ```bash
    curl -Lo cassonic https://github.com/local/cassonic/releases/latest/download/cassonic-linux-arm64
    curl -Lo cassonic-cli https://github.com/local/cassonic/releases/latest/download/cassonic-cli-linux-arm64
    chmod +x cassonic cassonic-cli
    sudo mv cassonic cassonic-cli /usr/local/bin/
    ```

=== "macOS amd64"

    ```bash
    curl -Lo cassonic https://github.com/local/cassonic/releases/latest/download/cassonic-darwin-amd64
    curl -Lo cassonic-cli https://github.com/local/cassonic/releases/latest/download/cassonic-cli-darwin-amd64
    chmod +x cassonic cassonic-cli
    sudo mv cassonic cassonic-cli /usr/local/bin/
    ```

=== "macOS arm64 (Apple Silicon)"

    ```bash
    curl -Lo cassonic https://github.com/local/cassonic/releases/latest/download/cassonic-darwin-arm64
    curl -Lo cassonic-cli https://github.com/local/cassonic/releases/latest/download/cassonic-cli-darwin-arm64
    chmod +x cassonic cassonic-cli
    sudo mv cassonic cassonic-cli /usr/local/bin/
    ```

=== "Windows amd64"

    Download `cassonic-windows-amd64.exe` and `cassonic-cli-windows-amd64.exe` from the [releases page](https://github.com/local/cassonic/releases), rename them to `cassonic.exe` and `cassonic-cli.exe`, and add their directory to your `PATH`.

## First Run

```bash
cassonic
```

On first run, cassonic:

1. Auto-creates the configuration directory (`~/.config/local/cassonic/` for a user install, `/etc/local/cassonic/` for root).
2. Generates `server.yml` with safe defaults.
3. Prints a one-time **setup token** in the startup banner.
4. Listens on `http://0.0.0.0:4533` by default.

Open `http://localhost:4533` and use the setup token to create your primary admin account. The setup token is invalidated after the first admin is created.

## Install as a System Service

Cassonic detects your init system (systemd, OpenRC, runit, s6, launchd, Windows Service) and installs itself automatically.

```bash
# Install and enable as a system service (requires root/sudo)
sudo cassonic --service --install

# Check status
cassonic --status

# Manage the service
cassonic --service start
cassonic --service stop
cassonic --service restart
```

When installed as root, cassonic creates a dedicated `cassonic` system user and drops privileges after binding to the configured port.

### User Service (systemd)

```bash
# Install as a user-level systemd service (no root required)
cassonic --service --install
```

User services bind to ports above 1024 only. Default port is `4533`.

### Uninstall

!!! warning "Destructive operation"
    Uninstalling deletes all data, configuration, and the `cassonic` system user. This cannot be undone.

```bash
sudo cassonic --service --uninstall
```

Cassonic prompts for confirmation before deleting any data.

## Docker

```bash
docker pull ghcr.io/local/cassonic:latest
```

### Docker Compose

```yaml
services:
  cassonic:
    image: ghcr.io/local/cassonic:latest
    restart: unless-stopped
    ports:
      - "4533:80"
    volumes:
      - ./volumes/config:/config:z
      - ./volumes/data:/data:z
      - /path/to/music:/music:ro
    environment:
      MODE: production
      TZ: America/New_York
```

Save as `docker-compose.yml`, then:

```bash
docker compose up -d
docker compose logs -f cassonic
```

The setup token appears in `docker compose logs cassonic` on the first run.

### Container Paths

| Path | Purpose |
|------|---------|
| `/config/cassonic/server.yml` | Configuration file |
| `/data/cassonic/` | Application data |
| `/data/db/sqlite/server.db` | Main database |
| `/data/log/cassonic/` | Log files |
| `/music` | Mount your music library here (any path works) |

## Build from Source

Requires Docker. The Go toolchain runs inside the project's build container — no local Go installation needed.

```bash
git clone https://github.com/local/cassonic.git
cd cassonic

# Quick dev build to a temp directory
make dev

# Production build to binaries/
make local

# Build all 8 platforms (linux/darwin/windows × amd64/arm64)
make build
```

## Verify the Installation

```bash
cassonic --version
cassonic --status
```

```bash
curl http://localhost:4533/health
# {"status":"ok","version":"1.2.3"}
```
