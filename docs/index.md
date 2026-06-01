# Cassonic

Cassonic is a self-hosted music streaming server with a Subsonic-compatible API, Ampache-compatible API, and a modern progressive web app UI. It ships as a single static binary with zero runtime dependencies.

## Features

- **Subsonic/Airsonic compatible** — works with all Subsonic clients (DSub, Ultrasonic, Symfonium, Tempo, Substreamer, and more)
- **Ampache compatible** — works with Ampache clients and plugins
- **Music library scanner** — watches directories for changes, reads ID3v2/FLAC/Ogg/Opus/AAC/M4A tags
- **Tag editor** — edit track, album, and artist metadata from the web UI or CLI
- **MusicBrainz auto-lookup** — automatically enriches metadata from MusicBrainz
- **Scrobbling** — Last.fm, LibreFM, ListenBrainz, Maloja, Funkwhale
- **Icecast relay** — broadcast your library through an Icecast-compatible endpoint
- **Podcasts** — add RSS feeds and stream/download episodes
- **File upload** — upload music files directly through the admin panel or API
- **Public share links** — share albums, playlists, or individual tracks via time-limited URLs
- **Progressive web app** — installable on mobile and desktop, works offline
- **Admin panel** — full server management at `/server/admin/`
- **REST API** — versioned JSON API at `/api/v1/`
- **Prometheus metrics** — at `/metrics` (configurable, internal-only)
- **Tor hidden service** — auto-enabled when the `tor` binary is found
- **Built-in scheduler** — SSL renewal, GeoIP updates, library scans, backups, log rotation
- **GeoIP country filtering** — block or allow-list countries without external services
- **Automatic Let's Encrypt TLS** — zero-config HTTPS with auto-renewal
- **i18n** — English, Spanish, French, German, Chinese, Arabic, Japanese
- **WCAG 2.1 AA accessibility** — full keyboard navigation, screen-reader support
- Single static binary, `CGO_ENABLED=0`, zero runtime dependencies

## Quick Start

```bash
# Download the binary
curl -Lo cassonic https://github.com/local/cassonic/releases/latest/download/cassonic-linux-amd64
chmod +x cassonic

# Run — auto-creates config, shows setup token in the banner
./cassonic
```

Open `http://localhost:4040` and use the setup token from the banner to create your admin account.

## Documentation

| Guide | Description |
|-------|-------------|
| [Installation](installation.md) | Binary, Docker, and service install |
| [Configuration](configuration.md) | All `server.yml` options |
| [API Reference](api.md) | REST, Subsonic, and Ampache APIs |
| [CLI Reference](cli.md) | `cassonic-cli` commands and flags |
| [Admin Panel](admin.md) | Admin panel walkthrough |
| [Security](security.md) | Auth, TLS, GeoIP, blocklists, Tor |
| [Integrations](integrations.md) | Scrobbling, Icecast, MusicBrainz, podcasts |
| [Development](development.md) | Building from source, contributing |
