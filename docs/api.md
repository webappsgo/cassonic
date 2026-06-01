# API Reference

Cassonic exposes three API surfaces: the native REST API, the Subsonic-compatible API, and the Ampache-compatible API. All three can be used simultaneously by different clients.

## Native REST API

Base path: `/api/v1/`

Interactive documentation (Swagger UI): [`/swagger/`](http://localhost:4040/swagger/)

OpenAPI spec (machine-readable): `/api/v1/openapi.json`

### Authentication

All protected endpoints require a Bearer token in the `Authorization` header.

```bash
# Obtain a token
curl -X POST http://localhost:4040/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "yourpassword"}'

# Response: {"ok": true, "data": {"token": "eyJ..."}}

# Use the token
curl http://localhost:4040/api/v1/artists \
  -H "Authorization: Bearer eyJ..."
```

Tokens are stored as SHA-256 hashes and never returned after creation. Use the admin panel or `/api/v1/auth/tokens` to manage tokens.

### Response Format

All responses use a consistent envelope:

```json
{
  "ok": true,
  "data": { ... }
}
```

Errors use [RFC 7807](https://www.rfc-editor.org/rfc/rfc7807) Problem Details:

```json
{
  "ok": false,
  "type": "https://cassonic.local/errors/not-found",
  "title": "Not Found",
  "status": 404,
  "detail": "Artist with id=42 does not exist"
}
```

### Public Endpoints (no auth required)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | JSON health check |
| `GET` | `/api/v1/health` | JSON health check |
| `GET` | `/version` | Server version |
| `GET` | `/api/v1/version` | Server version |
| `GET` | `/api/v1/autodiscover` | Client auto-configuration hints |
| `POST` | `/api/v1/auth/login` | Authenticate, returns Bearer token |

### Library Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/artists` | Paginated list of artists |
| `GET` | `/api/v1/artists/{id}` | Artist by ID |
| `GET` | `/api/v1/albums` | Paginated list of albums |
| `GET` | `/api/v1/albums/{id}` | Album by ID |
| `GET` | `/api/v1/songs` | Paginated list of songs |
| `GET` | `/api/v1/songs/{id}` | Song by ID |
| `GET` | `/api/v1/songs/{id}/stream` | Stream audio (range requests supported) |
| `GET` | `/api/v1/genres` | List all genres |
| `GET` | `/api/v1/search` | Search across artists, albums, songs (`?q=`) |

### Playlist Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/playlists` | List playlists |
| `POST` | `/api/v1/playlists` | Create playlist |
| `GET` | `/api/v1/playlists/{id}` | Get playlist |
| `PUT` | `/api/v1/playlists/{id}` | Update playlist |
| `DELETE` | `/api/v1/playlists/{id}` | Delete playlist |
| `POST` | `/api/v1/playlists/{id}/songs` | Add songs to playlist |
| `DELETE` | `/api/v1/playlists/{id}/songs/{songId}` | Remove song from playlist |

### Tag Editing

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/songs/{id}/tags` | Read ID3/Vorbis tags |
| `PATCH` | `/api/v1/songs/{id}/tags` | Update tags (writes to file on disk) |

### Library Management

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/libraries` | List configured library paths |
| `POST` | `/api/v1/libraries/{id}/scan` | Trigger a library scan |
| `GET` | `/api/v1/libraries/{id}/scan/status` | Current scan progress |

### Admin Endpoints (admin token required)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/admin/backup` | Trigger a backup |
| `GET` | `/api/v1/admin/backups` | List available backups |
| `POST` | `/api/v1/admin/restore` | Restore from a backup |
| `GET` | `/api/v1/admin/scheduler` | Scheduler job status |
| `POST` | `/api/v1/admin/scheduler/{job}/run` | Trigger a scheduler job |
| `GET` | `/api/v1/admin/users` | List users |
| `POST` | `/api/v1/admin/users` | Create user |
| `DELETE` | `/api/v1/admin/users/{id}` | Delete user |

### Podcast Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/podcasts` | List podcast feeds |
| `POST` | `/api/v1/podcasts` | Add a feed by RSS URL |
| `DELETE` | `/api/v1/podcasts/{id}` | Remove a feed |
| `GET` | `/api/v1/podcasts/{id}/episodes` | List episodes |
| `GET` | `/api/v1/podcasts/{id}/episodes/{epId}/stream` | Stream episode |

### Pagination

List endpoints accept `limit` (default `50`, max `500`) and `offset` query parameters.

```bash
curl "http://localhost:4040/api/v1/albums?limit=20&offset=40" \
  -H "Authorization: Bearer $TOKEN"
```

### Request ID

Every response includes an `X-Request-ID` header for tracing. Pass `X-Request-ID` in a request to propagate your own ID.

---

## Subsonic API

Cassonic implements the Subsonic REST API (version 1.16.1), compatible with Airsonic, Navidrome, and all Subsonic clients.

Base path: `/rest/`

### Supported Clients

Symfonium, DSub, Ultrasonic, Substreamer, Tempo, Airsub, Submariner, play:Sub, and any other Subsonic-compatible client.

### Authentication

Subsonic clients authenticate with a username, a password or token, and a salt. The client configuration in your Subsonic app:

| Field | Value |
|-------|-------|
| Server URL | `http://your-server:4040` |
| Username | Your cassonic username |
| Password | Your cassonic password |
| API version | `1.16.1` |

Subsonic token authentication: clients send `t=md5(password+salt)&s=salt`. Cassonic validates these tokens against the stored Argon2id password hash.

### Subsonic Endpoints Implemented

| Category | Endpoints |
|----------|-----------|
| System | `ping`, `getLicense` |
| Browsing | `getMusicFolders`, `getIndexes`, `getMusicDirectory`, `getArtists`, `getArtist`, `getAlbum`, `getSong`, `getGenres`, `getSongsByGenre` |
| Playlists | `getPlaylists`, `getPlaylist`, `createPlaylist`, `updatePlaylist`, `deletePlaylist` |
| Media | `stream`, `download`, `getCoverArt`, `getLyrics`, `getAvatar` |
| Searching | `search2`, `search3` |
| Bookmarks | `getBookmarks`, `createBookmark`, `deleteBookmark`, `getPlayQueue`, `savePlayQueue` |
| Scrobbling | `scrobble`, `star`, `unstar`, `setRating` |
| Podcasts | `getPodcasts`, `getNewestPodcasts`, `downloadPodcastEpisode` |
| User management | `getUser`, `getUsers` (admin only), `createUser` (admin only) |

---

## Ampache API

Cassonic implements the Ampache API (version 5), used by Ampache clients and plugins.

Base path: `/server/xml.server.php` (XML) and `/server/json.server.php` (JSON)

### Authentication

```bash
# Step 1: hash your password
PASSPHRASE=$(echo -n "yourpassword" | sha256sum | awk '{print $1}')
TIMESTAMP=$(date +%s)
AUTH_KEY=$(echo -n "${TIMESTAMP}${PASSPHRASE}" | sha256sum | awk '{print $1}')

# Step 2: authenticate
curl "http://localhost:4040/server/json.server.php?action=handshake&auth=${AUTH_KEY}&timestamp=${TIMESTAMP}&version=500000&user=admin"
```

### Supported Ampache Clients

Ample, Ampache for Kodi, and any other Ampache-compatible client.

---

## GraphQL

A GraphQL endpoint is available at `/graphql/` for client discovery. Full query execution will be added in a future release; use `/api/v1` REST endpoints in the meantime.

---

## Autodiscover

Clients can discover all API endpoints automatically:

```bash
curl http://localhost:4040/api/v1/autodiscover
```

```json
{
  "server": "cassonic",
  "version": "1.2.3",
  "api_version": "v1",
  "base_url": "/",
  "features": ["subsonic", "ampache", "icecast", "podcasts", "scrobbling", "tor"],
  "subsonic_url": "/rest",
  "ampache_url": "/server",
  "api_url": "/api/v1",
  "docs_url": "/swagger/",
  "metrics_url": "/metrics"
}
```
