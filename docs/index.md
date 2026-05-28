# cassonic

A self-hosted server application with REST API, GraphQL, web UI, and CLI companion.

## Features

- Single static binary — zero runtime dependencies
- Web UI with dark/light/auto theme switching
- Full admin panel
- REST API and GraphQL
- Tor hidden service (auto-enabled when Tor is installed)
- Built-in scheduler, GeoIP, metrics, email, backup, and update
- i18n: English, Spanish, French, German, Chinese, Arabic, Japanese

## Quick Start

```bash
# Download and run
curl -Lo cassonic https://github.com/local/cassonic/releases/latest/download/cassonic-linux-amd64
chmod +x cassonic
./cassonic
```

## Documentation

- [Installation](installation.md) — Installation and first-run setup
- [Configuration](configuration.md) — Configuration reference
- [API Reference](api.md) — REST API documentation
- [CLI Reference](cli.md) — CLI companion reference
- [Admin Panel](admin.md) — Admin panel guide
- [Security](security.md) — Security model and hardening
- [Integrations](integrations.md) — External service integrations
- [Development](development.md) — Contributing and development guide
