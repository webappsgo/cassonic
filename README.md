# cassonic

A self-hosted server application with a companion CLI.

[![CI](https://github.com/local/cassonic/actions/workflows/ci.yml/badge.svg)](https://github.com/local/cassonic/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE.md)

## Features

- Single static binary — zero runtime dependencies
- Web UI with dark/light/auto theme
- Full admin panel
- REST API + GraphQL
- Tor hidden service (auto-enabled when Tor is installed)
- Built-in scheduler, GeoIP, metrics, email, backup, and update
- i18n: English, Spanish, French, German, Chinese, Arabic, Japanese

## Installation

### Pre-built Binary

```bash
# Linux amd64
curl -Lo cassonic https://github.com/local/cassonic/releases/latest/download/cassonic-linux-amd64
chmod +x cassonic
sudo mv cassonic /usr/local/bin/cassonic
```

### Docker

```bash
# Pull latest image
docker pull ghcr.io/local/cassonic:latest

# Run with Docker Compose
cd docker/
docker compose up -d
```

### Build from Source

```bash
git clone https://github.com/local/cassonic.git
cd cassonic
make local
```

## Usage

```bash
# Start server (first run auto-creates config)
cassonic

# Show help
cassonic --help

# Show version
cassonic --version

# Development mode
cassonic --mode development

# Debug mode
cassonic --debug

# Check status
cassonic --status

# Service management
cassonic --service start
cassonic --service stop
cassonic --service --install
```

## CLI

```bash
# cassonic-cli companion tool
cassonic-cli --help
cassonic-cli --server http://localhost:64580
```

## Configuration

Configuration is auto-generated on first run at:
- Privileged: `/etc/local/cassonic/server.yml`
- User: `~/.config/local/cassonic/server.yml`
- Container: `/config/cassonic/server.yml`

## Development

```bash
# Quick dev build (to temp dir)
make dev

# Run unit tests
make test

# Build all platforms
make build

# Build Docker image
make docker
```

## License

MIT License — see [LICENSE.md](LICENSE.md)
