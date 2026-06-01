# Integrations

## Scrobbling

Cassonic can scrobble (submit listening history) to multiple services simultaneously. Scrobbling is triggered when a track has been played for at least 50% of its duration or 4 minutes, whichever is less — matching the Last.fm specification.

Configure scrobbling per-user in **Admin → Integrations → Scrobbling** or in `server.yml`.

### Last.fm

1. Go to [last.fm/api/account/create](https://www.last.fm/api/account/create) and create an API account.
2. Copy your **API key** and **API secret**.
3. In the admin panel, go to **Integrations → Scrobbling → Last.fm**.
4. Enter the API key and secret, then click **Authorize** — you are redirected to Last.fm to grant access.

```yaml
scrobbling:
  lastfm:
    enabled: true
    api_key: your_api_key
    api_secret: your_api_secret
```

### LibreFM

LibreFM uses the Last.fm-compatible Audioscrobbler protocol. No API key is required.

```yaml
scrobbling:
  librefm:
    enabled: true
```

Users log in with their libre.fm username and password via the admin panel OAuth flow.

### ListenBrainz

1. Log in at [listenbrainz.org](https://listenbrainz.org) and copy your **user token** from the profile page.
2. In the admin panel, go to **Integrations → Scrobbling → ListenBrainz** and paste the token.

```yaml
scrobbling:
  listenbrainz:
    enabled: true
    token: your_listenbrainz_token
```

### Maloja

[Maloja](https://github.com/krateng/maloja) is a self-hosted scrobbling server.

1. Deploy Maloja and copy the API key from its admin panel.
2. Configure cassonic:

```yaml
scrobbling:
  maloja:
    enabled: true
    url: http://maloja.example.com
    api_key: your_maloja_api_key
```

### Funkwhale

[Funkwhale](https://funkwhale.audio) supports the ListenBrainz API for scrobbling.

```yaml
scrobbling:
  funkwhale:
    enabled: true
    url: https://funkwhale.example.com
    token: your_funkwhale_token
```

---

## Icecast Relay

Cassonic can act as a source client for an Icecast or Icecast-compatible (Liquidsoap, Azuracast) server, broadcasting a continuous audio stream from your library.

### Setup

1. Install and configure an [Icecast server](https://icecast.org/).
2. In the admin panel, go to **Integrations → Icecast** and configure:

```yaml
icecast:
  enabled: true
  host: icecast.example.com
  port: 8000
  mount: /cassonic
  source_password: your_source_password
  format: mp3     # mp3 or ogg
  bitrate: 128    # kbps
```

3. Enable and start the relay from the admin panel.

Listeners connect to `http://icecast.example.com:8000/cassonic` with any audio player (VLC, mpv, web browsers).

---

## MusicBrainz

Cassonic automatically queries the [MusicBrainz API](https://musicbrainz.org/) to enrich music metadata. No API key is required for read-only lookups.

### Automatic Enrichment

During a library scan, cassonic attempts MusicBrainz lookups for:

- **Track MBID** — via embedded MusicBrainz ID tags (`MUSICBRAINZ_TRACKID`)
- **Album MBID** — via `MUSICBRAINZ_ALBUMID`
- **Artist MBID** — via `MUSICBRAINZ_ARTISTID`
- **Fuzzy matching** — for untagged files: artist + album + title similarity search

MusicBrainz enrichment fills in missing or incorrect metadata, standardizes artist names, and links releases to canonical identifiers.

### Manual Lookup

In the admin panel, open any song, album, or artist and click **Lookup on MusicBrainz** to trigger an immediate lookup and apply the result.

### Rate Limiting

Cassonic respects the MusicBrainz rate limit (1 request/second) and backs off automatically on 503 responses.

---

## Podcast Clients

Cassonic exposes podcasts as regular Subsonic and Ampache podcast feeds, consumable by any Subsonic client.

For direct RSS access, the cassonic podcast proxy URL is:

```
http://your-server:4040/api/v1/podcasts/{id}/rss
```

Supported podcast client integrations:

- **DSub** — subscribe via Subsonic podcast support
- **Symfonium** — subscribe via Subsonic podcast support
- Any Subsonic client with podcast support

---

## Subsonic Client Compatibility

Cassonic is tested with the following Subsonic clients:

| Client | Platform | Notes |
|--------|----------|-------|
| [Symfonium](https://symfonium.app) | Android | Full feature support |
| [DSub](https://f-droid.org/packages/github.daneren2005.dsub) | Android | Full feature support |
| [Ultrasonic](https://ultrasonic.gitlab.io) | Android | Full feature support |
| [Substreamer](https://apps.apple.com/app/substreamer/id1012991228) | iOS | Full feature support |
| [Tempo](https://apps.apple.com/app/tempo-music-player/id1513511402) | iOS | Full feature support |
| [Airsub](https://github.com/theapeman/Airsub) | macOS | Full feature support |
| [Submariner](https://submarinerapp.com) | macOS | Full feature support |
| [play:Sub](https://apps.apple.com/app/play-sub-subsonic-music-client/id955329386) | iOS/macOS | Full feature support |
| [Clementine](https://www.clementine-player.org) | Desktop | Via Subsonic plugin |

Cassonic implements **Subsonic API version 1.16.1**. Clients that require older API versions work via the `v` parameter negotiation.

---

## Ampache Client Compatibility

| Client | Notes |
|--------|-------|
| [Ample](https://github.com/mitchray/ample) | Web-based |
| [Ampache for Kodi](https://forum.kodi.tv/showthread.php?tid=323216) | Kodi add-on |

---

## Reverse Proxy

### Nginx

```nginx
server {
    listen 443 ssl;
    server_name music.example.com;

    ssl_certificate     /etc/letsencrypt/live/music.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/music.example.com/privkey.pem;

    location / {
        proxy_pass         http://127.0.0.1:4040;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        proxy_buffering    off;
        proxy_read_timeout 300s;
    }
}
```

### Caddy

```caddyfile
music.example.com {
    reverse_proxy 127.0.0.1:4040
}
```

### Traefik

```yaml
labels:
  - "traefik.enable=true"
  - "traefik.http.routers.cassonic.rule=Host(`music.example.com`)"
  - "traefik.http.routers.cassonic.entrypoints=websecure"
  - "traefik.http.routers.cassonic.tls.certresolver=letsencrypt"
  - "traefik.http.services.cassonic.loadbalancer.server.port=4040"
```

### Tor Hidden Service

Cassonic auto-manages its own Tor hidden service. No reverse proxy configuration is needed for Tor — cassonic connects directly to the Tor daemon.

---

## Monitoring and Alerting

### Prometheus

Add cassonic to your Prometheus `scrape_configs`:

```yaml
scrape_configs:
  - job_name: cassonic
    static_configs:
      - targets: ['localhost:4040']
    metrics_path: /metrics
    authorization:
      credentials: your-metrics-token
```

### Key Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `cassonic_http_requests_total` | Counter | HTTP requests by method, path, status |
| `cassonic_library_tracks_total` | Gauge | Total indexed tracks |
| `cassonic_library_scan_duration_seconds` | Histogram | Library scan duration |
| `cassonic_scrobble_total` | Counter | Scrobbles submitted by service |
| `cassonic_scrobble_errors_total` | Counter | Failed scrobble submissions |
| `cassonic_stream_total` | Counter | Audio streams started |
| `cassonic_active_sessions` | Gauge | Current active sessions |

### Grafana Dashboard

Import the cassonic Grafana dashboard (ID: TBD) from [grafana.com/grafana/dashboards](https://grafana.com/grafana/dashboards/) for a pre-built view of library stats, streaming activity, and system health.
