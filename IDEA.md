## Project description

cassonic is a self-hosted music streaming server written in Go, designed as a full-featured,
drop-in replacement for Airsonic, Subsonic, Libresonic, Ampache, and kPlaylist. It exposes
complete compatibility APIs so every existing client (DSub, Ultrasonic, Symfonium, Clementine,
Rhythmbox, Amarok, etc.) works without reconfiguration.

Beyond compatibility, cassonic adds a built-in tag editor (ID3v2-first, all formats, MusicBrainz
ID support), multi-server multi-mount Icecast relay streaming, and an elegant mobile-first WebUI
that makes cassonic pleasant to use directly without any third-party client.

## Project variables

project_name:     cassonic
project_org:      local
internal_name:    cassonic
app_name:         cassonic
official_site:    cassonic.local.us
maintainer_name:  CasjaysDev
maintainer_email: casjay@yahoo.com

## Business logic

### Product scope & non-goals

**Product scope:** cassonic is a self-hosted music streaming server with:
- Full Subsonic REST API compatibility (all versions through 1.16.1, plus legacy 1.x endpoints)
- Full Ampache API compatibility (v5 and v6, XML and JSON)
- Own native REST API following AI.md PART 14 conventions (versioned, noun-based, RFC 7807 errors)
- Built-in tag editor for all common audio formats (ID3v2-preferred), including MusicBrainz IDs
- Multi-server multi-mount Icecast relay streaming (by all tracks, artist, or genre)
- Transcoding and format conversion via ffmpeg static binary (from binmgr/ffmpeg)
- Mobile-first WebUI that is also a full-featured music player in the browser
- CLI companion binary (`cassonic-cli`) for library management, scanning, and admin ops

**Also included (not deferred):**
- Podcast support: RSS fetch, episode download, playback (Subsonic + Ampache + native API)
- MusicBrainz auto-lookup: opt-in nightly background job; never overwrites non-empty user-edited fields; empty/cleared fields are repopulated
- Multi-service scrobbling: Last.fm, ListenBrainz, Libre.fm, GNU FM, Maloja, and any custom Last.fm-compat or ListenBrainz-compat server; fan-out to all configured+enabled+verified services simultaneously; per-service queue + retry
- Audio file upload: admin-configurable, per-user permission, path-sanitized
- Public share links: songs/albums/playlists, optional expiry + password, Tor .onion URL included

**Replaces (feature/API parity, no data migration required):**
- Airsonic / Subsonic / Libresonic (full API + client compat)
- Ampache (full API compat, XML + JSON, v5 + v6)
- kPlaylist (feature parity: playlists, library browsing, streaming, user management)

**Non-goals:**
- No paid tiers or feature gating — all features free
- No telemetry without explicit opt-in
- No data migration tools from Ampache/kPlaylist databases
- No client-side rendering (SPA) — server-side Go templates only
- No bundled Tor-incompatible analytics

### Roles & permissions

- **Server Admin**: manages application settings, music libraries, users, Icecast streams, and all data
- **Primary Admin**: first admin account created on first run; cannot be deleted
- **Regular User**: streams music, manages playlists, edits tags on writable files (if granted), sets favorites/ratings

### Data model & sensitivity

- User accounts stored in `users.db` (Argon2id password hashing)
- Music library metadata, playlists, ratings, play counts stored in `server.db`
- Audio files remain on disk in operator-configured library paths; cassonic never moves or copies them
- Tag edits write directly to audio files (only when the file is writable by the server process)
- No PII collected beyond what is required for account management

### Trust boundaries & external services

- All input is untrusted until validated server-side
- ffmpeg subprocess: launched with restricted arguments; all paths sanitized before passing
- Icecast servers: operator-configured; credentials stored encrypted at rest; treated as untrusted endpoints
- MusicBrainz lookups: optional, outbound-only, user-initiated; no automatic background phoning home
- Tor hidden service: auto-enabled when Tor binary is present (see PART 32)

### Threat model & abuse cases

**Primary assets:** user data, admin credentials, audio files, application configuration

**Untrusted inputs:** all HTTP request data, audio file tags read from disk, uploaded cover art,
Subsonic/Ampache API parameters, query strings, multipart form data

**Main attacker goals:** credential theft, privilege escalation, unauthorized file access, path traversal
to files outside library paths

**Abuse cases:**
- Brute-force login (Subsonic/Ampache token endpoints included) — mitigated by rate limiting and Argon2id
- Path traversal via library path or file download parameters — mitigated by path sanitization (PART 5)
- CSRF on admin and tag-edit actions — mitigated by CSRF tokens (PART 11)
- XSS via user-supplied tag data rendered in WebUI — mitigated by server-side template escaping (PART 16)
- Malicious audio file tags causing parser crashes — mitigated by defensive tag parsing with size/encoding limits
- Icecast credential leakage in logs — mitigated by credential masking in all log output

### Security decisions & exceptions

- Subsonic and Ampache APIs use their own authentication schemes (token-based, MD5 challenge) — this is
  intentional for client compatibility; these endpoints are rate-limited and audit-logged
- Admin authentication bypassed in `--debug` mode for local development only
- Tor integration enabled when Tor binary is found (see PART 32)
- ffmpeg is a subprocess (not a library); all arguments are constructed server-side, never from raw user input

### Go package directory naming exceptions

The project-rules.md convention requires singular directory names to match Go package names.
The following directories use stdlib-idiomatic names that are already singular concepts in Go and
are approved exceptions to the explicit-plural rule:

- `src/paths/` — matches Go convention for path-utility packages (cf. stdlib `path/`)
- `src/server/metrics/` — matches Prometheus convention; `metric` would be non-standard
- `src/common/errors/` — matches Go convention (cf. stdlib `errors`)
- `src/server/service/tags/` — domain package for audio tag reading/writing; `tag` is ambiguous

---

## Routes & API Endpoints

### Subsonic REST API Compatibility Layer

All Subsonic REST API endpoints mount at `/rest/`. The Subsonic protocol uses query parameters
for method dispatch (`?v=`, `?c=`, `?u=`, `?t=`, `?s=` or `?p=`). Both XML (default) and JSON
(`?f=json`) response formats are supported for all endpoints.

Target: Subsonic REST API 1.16.1 (current) + full backward compat to 1.1.0.

**Authentication:** Subsonic token auth (`?u=user&t=token&s=salt`) and legacy plaintext/hex
(`?u=user&p=password` or `?u=user&p=enc:hex`) — both required for client compatibility.

**Core endpoints (must all be implemented):**

| Endpoint | Method |
|----------|--------|
| `/rest/ping.view` | GET/POST |
| `/rest/getLicense.view` | GET/POST |
| `/rest/getMusicFolders.view` | GET/POST |
| `/rest/getIndexes.view` | GET/POST |
| `/rest/getMusicDirectory.view` | GET/POST |
| `/rest/getGenres.view` | GET/POST |
| `/rest/getArtists.view` | GET/POST |
| `/rest/getArtist.view` | GET/POST |
| `/rest/getAlbum.view` | GET/POST |
| `/rest/getSong.view` | GET/POST |
| `/rest/getVideos.view` | GET/POST |
| `/rest/getVideoInfo.view` | GET/POST |
| `/rest/getArtistInfo.view` | GET/POST |
| `/rest/getArtistInfo2.view` | GET/POST |
| `/rest/getAlbumInfo.view` | GET/POST |
| `/rest/getAlbumInfo2.view` | GET/POST |
| `/rest/getSimilarSongs.view` | GET/POST |
| `/rest/getSimilarSongs2.view` | GET/POST |
| `/rest/getTopSongs.view` | GET/POST |
| `/rest/getAlbumList.view` | GET/POST |
| `/rest/getAlbumList2.view` | GET/POST |
| `/rest/getRandomSongs.view` | GET/POST |
| `/rest/getSongsByGenre.view` | GET/POST |
| `/rest/getNowPlaying.view` | GET/POST |
| `/rest/getStarred.view` | GET/POST |
| `/rest/getStarred2.view` | GET/POST |
| `/rest/search.view` | GET/POST |
| `/rest/search2.view` | GET/POST |
| `/rest/search3.view` | GET/POST |
| `/rest/getPlaylists.view` | GET/POST |
| `/rest/getPlaylist.view` | GET/POST |
| `/rest/createPlaylist.view` | GET/POST |
| `/rest/updatePlaylist.view` | GET/POST |
| `/rest/deletePlaylist.view` | GET/POST |
| `/rest/stream.view` | GET/POST |
| `/rest/download.view` | GET/POST |
| `/rest/hls.view` | GET/POST |
| `/rest/getCaptions.view` | GET/POST |
| `/rest/getCoverArt.view` | GET/POST |
| `/rest/getLyrics.view` | GET/POST |
| `/rest/getAvatar.view` | GET/POST |
| `/rest/star.view` | GET/POST |
| `/rest/unstar.view` | GET/POST |
| `/rest/setRating.view` | GET/POST |
| `/rest/scrobble.view` | GET/POST |
| `/rest/getShares.view` | GET/POST |
| `/rest/createShare.view` | GET/POST |
| `/rest/updateShare.view` | GET/POST |
| `/rest/deleteShare.view` | GET/POST |
| `/rest/getPodcasts.view` | GET/POST |
| `/rest/getNewestPodcasts.view` | GET/POST |
| `/rest/refreshPodcasts.view` | GET/POST |
| `/rest/createPodcastChannel.view` | GET/POST |
| `/rest/deletePodcastChannel.view` | GET/POST |
| `/rest/deletePodcastEpisode.view` | GET/POST |
| `/rest/downloadPodcastEpisode.view` | GET/POST |
| `/rest/jukeboxControl.view` | GET/POST |
| `/rest/getInternetRadioStations.view` | GET/POST |
| `/rest/createInternetRadioStation.view` | GET/POST |
| `/rest/updateInternetRadioStation.view` | GET/POST |
| `/rest/deleteInternetRadioStation.view` | GET/POST |
| `/rest/getChatMessages.view` | GET/POST |
| `/rest/addChatMessage.view` | GET/POST |
| `/rest/getUser.view` | GET/POST |
| `/rest/getUsers.view` | GET/POST |
| `/rest/createUser.view` | GET/POST |
| `/rest/updateUser.view` | GET/POST |
| `/rest/deleteUser.view` | GET/POST |
| `/rest/changePassword.view` | GET/POST |
| `/rest/getBookmarks.view` | GET/POST |
| `/rest/createBookmark.view` | GET/POST |
| `/rest/deleteBookmark.view` | GET/POST |
| `/rest/getPlayQueue.view` | GET/POST |
| `/rest/savePlayQueue.view` | GET/POST |
| `/rest/getScanStatus.view` | GET/POST |
| `/rest/startScan.view` | GET/POST |

### Ampache API Compatibility Layer

Ampache API mounts at `/server/`. Both XML and JSON formats are supported.
Target: Ampache API v5 and v6 simultaneously (detect requested version from `version` param).

| Endpoint | Method | Notes |
|----------|--------|-------|
| `/server/xml.server.php` | GET/POST | Ampache XML API (all actions) |
| `/server/json.server.php` | GET/POST | Ampache JSON API (all actions) |

**Ampache actions to implement (both XML and JSON):**
handshake, goodbye, ping, check_parameter, get_indexes, get_bookmark, advanced_search,
artists, artist, artist_albums, artist_songs, albums, album, album_songs, songs, song,
song_delete, genre_songs, genre_albums, genre_artists, genres, genre, labels, label,
label_artists, live_streams, live_stream, live_stream_create, live_stream_edit,
live_stream_delete, playlists, playlist, playlist_songs, playlist_create, playlist_edit,
playlist_delete, playlist_add_song, playlist_remove_song, playlist_generate,
searches, search_songs, user, users, user_create, user_edit, user_delete,
user_preferences, user_preference, system_update, system_preferences, system_preference,
preference_create, preference_edit, preference_delete, toggle_follow, last_shouts,
timeline, friends_timeline, catalog_action, catalog_file, catalogs, catalog, catalog_songs,
catalog_albums, catalog_artists, flag, rate, record_play, scrobble, now_playing, stats,
podcasts, podcast, podcast_create, podcast_edit, podcast_delete, podcast_episodes,
podcast_episode, podcast_episode_delete, update_podcast, stream, download, get_art,
update_art, update_artist_info, upload, get_similar, shares, share, share_create, share_edit,
share_delete, bookmarks, bookmark_create, bookmark_edit, bookmark_delete, deleted_songs,
deleted_video, deleted_podcast_episodes

### Native cassonic REST API

Follows AI.md PART 14 exactly: versioned, plural nouns, lowercase, hyphens, no trailing slash.
Base: `/api/v1/`

**Library & Browsing:**

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/v1/libraries` | List configured music library folders |
| POST | `/api/v1/libraries` | Add a library folder (admin) |
| PATCH | `/api/v1/libraries/{id}` | Update library folder (admin) |
| DELETE | `/api/v1/libraries/{id}` | Remove library folder (admin) |
| POST | `/api/v1/libraries/{id}/scan` | Trigger library scan (admin) |
| GET | `/api/v1/libraries/{id}/scan` | Get scan status |
| GET | `/api/v1/artists` | List/search artists |
| GET | `/api/v1/artists/{id}` | Get artist detail |
| GET | `/api/v1/artists/{id}/albums` | List albums for artist |
| GET | `/api/v1/artists/{id}/songs` | List songs for artist |
| GET | `/api/v1/albums` | List/search albums |
| GET | `/api/v1/albums/{id}` | Get album detail |
| GET | `/api/v1/albums/{id}/songs` | List songs for album |
| GET | `/api/v1/songs` | List/search songs |
| GET | `/api/v1/songs/{id}` | Get song detail |
| GET | `/api/v1/genres` | List genres |
| GET | `/api/v1/genres/{id}/songs` | Songs by genre |
| GET | `/api/v1/genres/{id}/albums` | Albums by genre |
| GET | `/api/v1/genres/{id}/artists` | Artists by genre |
| GET | `/api/v1/search` | Full-text search (artists, albums, songs) |

**Streaming & Transcoding:**

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/v1/songs/{id}/stream` | Stream audio (optional transcode params: `format`, `maxBitRate`) |
| GET | `/api/v1/songs/{id}/download` | Download original file |
| GET | `/api/v1/songs/{id}/cover-art` | Get cover art image |
| GET | `/api/v1/albums/{id}/cover-art` | Get album cover art |
| GET | `/api/v1/artists/{id}/cover-art` | Get artist image |

**Tag Editor:**

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/v1/songs/{id}/tags` | Read all tags for a song |
| PATCH | `/api/v1/songs/{id}/tags` | Write tags to file (file must be writable) |
| GET | `/api/v1/songs/{id}/tags/writable` | Check if file is writable (can tags be edited?) |
| POST | `/api/v1/songs/{id}/cover-art` | Upload/replace embedded cover art |
| DELETE | `/api/v1/songs/{id}/cover-art` | Remove embedded cover art |

**Playlists:**

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/v1/playlists` | List playlists |
| POST | `/api/v1/playlists` | Create playlist |
| GET | `/api/v1/playlists/{id}` | Get playlist detail |
| PATCH | `/api/v1/playlists/{id}` | Update playlist metadata |
| DELETE | `/api/v1/playlists/{id}` | Delete playlist |
| GET | `/api/v1/playlists/{id}/songs` | List songs in playlist |
| POST | `/api/v1/playlists/{id}/songs` | Add songs to playlist |
| DELETE | `/api/v1/playlists/{id}/songs/{songId}` | Remove song from playlist |
| PUT | `/api/v1/playlists/{id}/songs` | Replace playlist songs |

**User Activity:**

| Method | Route | Description |
|--------|-------|-------------|
| POST | `/api/v1/songs/{id}/scrobbles` | Record play (scrobble) |
| POST | `/api/v1/songs/{id}/stars` | Star a song |
| DELETE | `/api/v1/songs/{id}/stars` | Unstar a song |
| PATCH | `/api/v1/songs/{id}/rating` | Set rating (1–5) |
| GET | `/api/v1/play-queues` | Get saved play queue for current user |
| PUT | `/api/v1/play-queues` | Save play queue |
| GET | `/api/v1/bookmarks` | List bookmarks |
| POST | `/api/v1/bookmarks` | Create/update bookmark (song + position) |
| DELETE | `/api/v1/bookmarks/{id}` | Delete bookmark |
| GET | `/api/v1/now-playing` | What is currently being played (all users) |

**Icecast Streaming:**

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/v1/icecast-servers` | List configured Icecast servers |
| POST | `/api/v1/icecast-servers` | Add Icecast server (admin) |
| PATCH | `/api/v1/icecast-servers/{id}` | Update Icecast server (admin) |
| DELETE | `/api/v1/icecast-servers/{id}` | Delete Icecast server (admin) |
| GET | `/api/v1/icecast-mounts` | List all configured mount points |
| POST | `/api/v1/icecast-mounts` | Create a mount point |
| PATCH | `/api/v1/icecast-mounts/{id}` | Update mount point |
| DELETE | `/api/v1/icecast-mounts/{id}` | Delete mount point |
| POST | `/api/v1/icecast-mounts/{id}/start` | Start streaming to mount |
| POST | `/api/v1/icecast-mounts/{id}/stop` | Stop streaming to mount |
| GET | `/api/v1/icecast-mounts/{id}/status` | Stream status (connected, track, listeners) |

**Users & Auth (Native API):**

| Method | Route | Description |
|--------|-------|-------------|
| POST | `/api/v1/auth/login` | Login, get session token |
| POST | `/api/v1/auth/logout` | Invalidate session token |
| POST | `/api/v1/auth/tokens` | Create API token |
| DELETE | `/api/v1/auth/tokens/{id}` | Revoke API token |
| GET | `/api/v1/users` | List users (admin) |
| POST | `/api/v1/users` | Create user (admin) |
| GET | `/api/v1/users/{id}` | Get user detail |
| PATCH | `/api/v1/users/{id}` | Update user |
| DELETE | `/api/v1/users/{id}` | Delete user (admin) |

**Standard infrastructure routes (from AI.md):**

| Route | Description |
|-------|-------------|
| `GET /server/healthz` | HTML health page |
| `GET /api/v1/server/healthz` | JSON health |
| `GET /api/healthz` | JSON health (alias) |
| `GET /server/help` | HTML help page (real endpoints + curl examples) |
| `GET /api/v1/server/version` | Version JSON |
| `GET /api/v1/server/{admin_path}/*` | Admin panel API |
| `GET /server/{admin_path}/*` | Admin panel WebUI |
| `GET /swagger/` | Swagger UI |
| `GET /graphql/` | GraphQL playground |

### WebUI Routes (frontend, server-side Go templates)

| Route | Page |
|-------|------|
| `GET /` | Home / Now playing dashboard |
| `GET /library` | Library browser (artists → albums → songs) |
| `GET /artists` | Artists list |
| `GET /artists/{id}` | Artist detail + albums |
| `GET /albums` | Albums list |
| `GET /albums/{id}` | Album detail + tracklist |
| `GET /songs/{id}` | Song detail |
| `GET /genres` | Genres list |
| `GET /genres/{id}` | Genre browser |
| `GET /playlists` | Playlists list |
| `GET /playlists/{id}` | Playlist detail |
| `GET /search` | Search results |
| `GET /player` | Full-screen player |
| `GET /tags/{id}` | Tag editor for a song |
| `GET /icecast` | Icecast stream manager |
| `GET /settings` | User settings |
| `GET /login` | Login page |
| `GET /server/{admin_path}/` | Admin dashboard |
| `GET /server/{admin_path}/users` | User management |
| `GET /server/{admin_path}/libraries` | Library management |
| `GET /server/{admin_path}/icecast` | Icecast server/mount management |
| `GET /server/{admin_path}/logs` | Log viewer |
| `GET /server/{admin_path}/settings` | Server settings |

---

## Feature Specifications

### Tag Editor

- Supported formats: MP3 (ID3v2.3 and ID3v2.4, preferred), FLAC (Vorbis Comments),
  OGG Vorbis, Opus, M4A/AAC (iTunes atoms), WAV (ID3 chunk), AIFF (ID3 chunk)
- When reading: prefer ID3v2 over ID3v1; expose all standard fields
- When writing: always write ID3v2.4 for MP3; preserve existing format for other types
- Fields: title, artist, album artist, album, track number, disc number, year, genre,
  composer, comment, BPM, compilation flag, MusicBrainz track ID, MusicBrainz artist ID,
  MusicBrainz album ID, MusicBrainz release group ID, lyrics (embedded), cover art (embedded)
- MusicBrainz IDs: user can set manually or look up via MusicBrainz API (outbound, user-initiated)
- User edits persist and protect non-empty fields from being overwritten by re-scan or auto-lookup;
  a `user_edited` flag in the DB marks the song as user-touched. Rule: if `user_edited = true`
  and a field is non-empty → never overwrite. If `user_edited = true` and a field is empty/null →
  repopulate (user cleared it intentionally, signaling "fill this in again")
- Tag write is only attempted if `os.Access(path, os.W_OK)` passes; returns clear error if not writable
- Cover art: read from embedded tags first, then fall back to `cover.jpg`/`folder.jpg` in directory

### Icecast Streaming

- An **Icecast Server** is a configured remote Icecast instance (host, port, admin user/pass,
  source password). Multiple servers supported.
- A **Mount Point** belongs to one server and defines:
  - Mount path (e.g. `/music.mp3`)
  - Format: MP3, OGG, Opus (transcoded via ffmpeg if source format differs)
  - Bitrate: user-configurable
  - Source scope: `all` (entire library shuffled), `artist` (single artist ID), `genre` (single genre)
  - Stream metadata: name, description, URL, genre label
  - ICY metadata updates: send `StreamTitle` on track change
- **Resume behavior**: when the streaming goroutine advances to the next track, it does NOT
  restart the current track. If the stream reconnects mid-track, it resumes from the beginning
  of the current track (not the previous one). Track position state is held in memory.
- Streaming goroutine: one per active mount; ffmpeg subprocess pipes audio to libshout or raw
  TCP Icecast source protocol; goroutine survives server-side reconnects automatically.
- Credentials stored encrypted in `server.db`; never logged in plaintext.

### ffmpeg Integration

- Binary source: `https://github.com/binmgr/ffmpeg/releases/latest` static builds
- Platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
- Auto-download on first use if not found at configured path (default: `{data_dir}/bin/ffmpeg`)
- Operator can override with system ffmpeg via config (`ffmpeg_path`)
- All ffmpeg invocations: constructed server-side, never from raw user input, always with `-nostdin`
- Supported transcode operations:
  - Stream transcoding (any format → MP3/OGG/Opus at configurable bitrate)
  - Format conversion for download (any → MP3/FLAC/OGG/Opus/AAC)
  - Cover art extraction (`-an -vcodec copy`)
  - Cover art embedding

### Library Scanning

- Recursive directory walk of configured library folders
- Supported extensions: `.mp3`, `.flac`, `.ogg`, `.opus`, `.m4a`, `.aac`, `.wav`, `.aiff`, `.wma`, `.ape`
- Tag reading via Go tag library (no ffmpeg dependency for scanning)
- Scan modes: full (re-read all tags) and incremental (mtime-based, skip unchanged files)
- Missing files: marked as unavailable in DB (not deleted); cleaned up on next full scan
- Cover art: extracted from tags or directory file; stored as blob in `server.db`
- Scan triggered by: admin UI, `/rest/startScan.view`, `/api/v1/libraries/{id}/scan`, startup (if configured)

### WebUI Design Requirements

- Mobile-first: all layouts designed for 320px width first, then enhanced for tablet/desktop
- Theme: dark by default; light and auto (system preference) supported; no hardcoded colors
- Design language: clean, minimal, professional — no cluttered toolbars; large touch targets (≥44px)
- Persistent bottom player bar on all pages (mobile-style); expands to full-screen player
- WebSocket or SSE for real-time "now playing" updates without page refresh
- Album art displayed prominently throughout
- Keyboard shortcuts for player controls (space=play/pause, arrows=seek/skip)
- No JavaScript required for browsing and basic playback (progressive enhancement)
- Audio playback in browser via HTML5 `<audio>` element; JS enhances with continuous play queue
- Waveform/progress scrubbing via JS (graceful degradation if JS absent)

### Scrobbling

- Two protocol backends cover all supported services:
  - **Last.fm-compatible** (HMAC-MD5 signed POST): Last.fm, Libre.fm, GNU FM, Maloja, any custom server
  - **ListenBrainz-compatible** (Bearer token, JSON): ListenBrainz, Maloja, any custom server
- Each user configures zero or more services; each service has: type, display name, credentials (encrypted at rest), `enabled` toggle, `verified` flag
- `verified` is set only after a successful connection test — a service that has never been verified or last failed verification is not included in fan-out even if `enabled = true`
- Fan-out: all `enabled = true` AND `verified = true` services called concurrently on every scrobble event; one failure never blocks others
- Scrobble threshold: ≥ 50% of duration played OR ≥ 4 minutes (whichever first)
- Now-playing: sent to all services immediately on stream start; failures logged only (not queued)
- Retry queue: failed scrobbles queued per service in `scrobble_queue`; retried every 30 minutes in batches (50 for Last.fm-compat, 1000 for ListenBrainz-compat); dropped after 14 days or 50 attempts
- Credentials: `api_secret`, `session_key`, `token` stored AES-256-GCM encrypted; never returned in API responses (masked as `xxxxx`); passwords used only during mobile-session auth and immediately discarded

### Transcoding & Streaming

- On-the-fly transcoding: ffmpeg subprocess, output piped directly to HTTP response
- `maxBitRate` Subsonic param and native API `maxBitRate` query param both respected
- Format requested by client respected if supported; fallback to MP3
- HTTP range requests supported for seeking in downloaded files
- Bitrate limiting for mobile clients

---

## Compatibility Notes

### Subsonic Client Compatibility Targets

Must work without configuration changes with: DSub (Android), Ultrasonic (Android),
Symfonium (Android), Clementine (desktop), Rhythmbox (desktop), Amarok (desktop),
Sublime Music (desktop), Audinaut, SubFire, iSub (iOS).

### Ampache Client Compatibility Targets

Must work with Ampache-compatible clients: Ario, Amarok (Ampache plugin), Clementine
(Ampache source), Rhythmbox (Ampache plugin).

### Authentication Compatibility

- Subsonic: token auth (`?t=&s=`) and legacy plaintext (`?p=`) — both required
- Ampache: handshake-based session tokens (SHA256, MD5 legacy) — both required
- Native API: Bearer token (JWT or opaque session token)
- All three auth schemes rate-limited independently
