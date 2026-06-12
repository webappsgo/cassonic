## Project description

cassonic is a self-hosted music streaming server designed as a full-featured,
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
- Built-in tag editor for all common audio formats, including MusicBrainz IDs
- Multi-server multi-mount Icecast relay streaming (by all tracks, artist, or genre)
- Audio transcoding and format conversion (on-demand, via external binary)
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
- No client-side rendering (SPA) — server-side templates only
- No bundled Tor-incompatible analytics

### Roles & permissions

- **Server Admin**: manages application settings, music libraries, users, Icecast streams, and all data
- **Primary Admin**: first admin account created on first run; cannot be deleted
- **Regular User**: streams music, manages playlists, edits tags on writable files (if granted), sets favorites/ratings

### Data model & sensitivity

- User accounts stored in `users.db` (password-hashed per AI.md PART 11)
- Music library metadata, playlists, ratings, play counts stored in `server.db`
- Audio files remain on disk in operator-configured library paths; cassonic never moves or copies them
- Tag edits write directly to audio files (only when the file is writable by the server process)
- No PII collected beyond what is required for account management

### Trust boundaries & external services

- All input is untrusted until validated server-side
- External transcoding binary: launched with restricted, server-constructed arguments; all paths sanitized before passing
- Icecast servers: operator-configured; credentials stored encrypted at rest; treated as untrusted endpoints
- MusicBrainz lookups: optional, outbound-only, user-initiated or scheduler-triggered; no automatic background phoning home without opt-in
- Tor hidden service: auto-enabled when Tor binary is present (see AI.md PART 32)
- Scrobbling services: outbound-only; credentials stored encrypted at rest; per-service failure never blocks others

### Threat model & abuse cases

**Primary assets:** user data, admin credentials, audio files, application configuration

**Untrusted inputs:** all HTTP request data, audio file tags read from disk, uploaded cover art,
Subsonic/Ampache API parameters, query strings, multipart form data

**Main attacker goals:** credential theft, privilege escalation, unauthorized file access, path traversal
to files outside library paths

**Abuse cases:**
- Brute-force login (Subsonic/Ampache token endpoints included) — mitigated by rate limiting and strong password hashing
- Path traversal via library path or file download parameters — mitigated by path sanitization (AI.md PART 5)
- CSRF on admin and tag-edit actions — mitigated by CSRF tokens (AI.md PART 11)
- XSS via user-supplied tag data rendered in WebUI — mitigated by server-side template escaping (AI.md PART 16)
- Malicious audio file tags causing parser crashes — mitigated by defensive tag parsing with size/encoding limits
- Icecast credential leakage in logs — mitigated by credential masking in all log output

### Security decisions & exceptions

- Subsonic and Ampache APIs use their own authentication schemes (token-based, MD5 challenge) — this is
  intentional for client compatibility; these endpoints are rate-limited and audit-logged
- Admin authentication bypassed in `--debug` mode for local development only
- Tor integration enabled when Tor binary is found (see AI.md PART 32)
- External transcoding binary is a subprocess; all arguments are constructed server-side, never from raw user input

### Subsonic REST API compatibility

All Subsonic REST API endpoints mount at `/rest/`. The Subsonic protocol uses query parameters
for method dispatch (`?v=`, `?c=`, `?u=`, `?t=`, `?s=` or `?p=`). Both XML (default) and JSON
(`?f=json`) response formats are supported for all endpoints.

Target: Subsonic REST API 1.16.1 (current) + full backward compat to 1.1.0.

**Authentication:** Subsonic token auth (`?u=user&t=token&s=salt`) and legacy plaintext/hex
(`?u=user&p=password` or `?u=user&p=enc:hex`) — both required for client compatibility.

**All endpoints (must be fully implemented):**

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

### Ampache API compatibility

Ampache API mounts at `/server/`. Both XML and JSON formats are supported.
Target: Ampache API v5 and v6 simultaneously (detect requested version from `version` param).

| Endpoint | Method | Notes |
|----------|--------|-------|
| `/server/xml.server.php` | GET/POST | Ampache XML API (all actions) |
| `/server/json.server.php` | GET/POST | Ampache JSON API (all actions) |

**Ampache actions (both XML and JSON):**
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
- List/add/update/remove configured music library folders (admin)
- Trigger and check status of library scans (admin)
- List/search artists, albums, songs, genres
- Get artist, album, or song detail
- List albums for an artist; list songs for an artist or album
- List songs and albums by genre

**Streaming & Transcoding:**
- Stream audio for a song (optional transcode params: format, max bitrate)
- Download original audio file
- Get cover art for a song, album, or artist

**Tag Editor:**
- Read all tags for a song
- Write tags to file (requires file to be writable on disk)
- Check whether a file's tags can be edited
- Upload or remove embedded cover art

**Playlists:**
- Create, list, get, update, and delete playlists
- Add, list, remove, or replace songs in a playlist

**User Activity:**
- Record a play (scrobble) for a song
- Star/unstar songs, albums, artists
- Set rating (1–5) for a song
- Save and retrieve play queue for the current user
- Create, list, and delete playback bookmarks
- List what is currently being played across all users

**Icecast Streaming:**
- Add, list, update, and remove Icecast server connections (admin)
- Create, list, update, and delete mount points
- Start and stop streaming to a mount
- Check streaming status (connected, current track, listener count)

**Users & Auth:**
- Login and logout (session tokens)
- Create and revoke API tokens
- Create, list, get, update, and delete users (admin)

**Standard infrastructure routes (from AI.md PART 13):**
- Health check endpoint (HTML and JSON variants)
- Help page with real endpoints and examples
- Version endpoint
- Admin panel WebUI and API
- Swagger UI
- GraphQL playground

### WebUI screens

Server-side rendered, mobile-first. All pages work without JavaScript.

- Home / Now playing dashboard
- Library browser (artists → albums → songs)
- Artists list and artist detail with albums
- Albums list and album detail with tracklist
- Song detail
- Genres list and genre browser
- Playlists list and playlist detail
- Search results
- Full-screen player
- Tag editor for a song
- Icecast stream manager
- User settings
- Login
- Admin panel: dashboard, user management, library management, Icecast management, log viewer, server settings

### Tag editor

**Supported formats:** MP3, FLAC, OGG Vorbis, Opus, M4A/AAC, WAV, AIFF

**Fields exposed:** title, artist, album artist, album, track number, disc number, year, genre,
composer, comment, BPM, compilation flag, MusicBrainz track ID, MusicBrainz artist ID,
MusicBrainz album ID, MusicBrainz release group ID, embedded lyrics, embedded cover art

**Business rules:**
- Tag write is only attempted if the file is writable by the server process; returns a clear error if not
- User edits are tracked with a `user_edited` flag in the database
- Rule: if `user_edited = true` and a field is non-empty → never overwrite during re-scan or auto-lookup
- Rule: if `user_edited = true` and a field is empty/null → repopulate (user cleared it, signaling "fill this in")
- MusicBrainz IDs: set manually by the user or looked up via MusicBrainz API (outbound, user-initiated or opt-in scheduler job)
- Cover art: read from embedded tags first, then fall back to `cover.jpg`/`folder.jpg` in the same directory

### Icecast streaming

- An **Icecast Server** is a configured remote Icecast instance (host, port, credentials). Multiple servers supported.
- A **Mount Point** belongs to one server and defines:
  - Mount path (e.g. `/music.mp3`)
  - Output format: MP3, OGG, or Opus (transcoded from source format as needed)
  - Bitrate: user-configurable
  - Source scope: `all` (entire library shuffled), `artist` (single artist), `genre` (single genre)
  - Stream metadata: name, description, URL, genre label
  - ICY metadata updates: track title sent on track change
- **Resume behavior**: when the stream advances to the next track, it does not restart the current track. If the stream reconnects mid-track, it resumes from the beginning of the current track.
- Credentials stored encrypted at rest; never logged in plaintext.

### Library scanning

- Recursive walk of configured library folders
- Supported extensions: `.mp3`, `.flac`, `.ogg`, `.opus`, `.m4a`, `.aac`, `.wav`, `.aiff`, `.wma`, `.ape`
- Scan modes: full (re-read all tags) and incremental (skip unchanged files by modification time)
- Missing files: marked as unavailable in database (not deleted); cleaned up on next full scan
- Cover art: extracted from embedded tags or directory file; stored in database
- Scan triggered by: admin UI, Subsonic scan endpoint, native API scan endpoint, or on startup if configured

### Scrobbling

- Two protocol families cover all supported services:
  - **Last.fm-compatible**: Last.fm, Libre.fm, GNU FM, Maloja, any custom compatible server
  - **ListenBrainz-compatible**: ListenBrainz, Maloja, any custom compatible server
- Each user configures zero or more services; each service has: type, display name, credentials, `enabled` toggle, `verified` flag
- `verified` is set only after a successful connection test — unverified services are excluded from fan-out even if `enabled = true`
- Fan-out: all `enabled = true` AND `verified = true` services called concurrently on every scrobble event; one failure never blocks others
- Scrobble threshold: ≥ 50% of duration played OR ≥ 4 minutes (whichever comes first)
- Now-playing: sent to all services immediately on stream start; failures logged only (not queued)
- Retry queue: failed scrobbles queued per service; retried on a schedule; dropped after 14 days or 50 attempts
- Credentials stored encrypted at rest; never returned in API responses

### Transcoding

- On-the-fly transcoding: output piped directly to HTTP response (no temp files)
- `maxBitRate` respected (both Subsonic param and native API query param)
- Format requested by client respected if supported; fallback to MP3
- HTTP range requests supported for downloaded files
- Bitrate limiting supported for mobile clients

### Subsonic client compatibility targets

Must work without configuration changes with: DSub (Android), Ultrasonic (Android),
Symfonium (Android), Clementine (desktop), Rhythmbox (desktop), Amarok (desktop),
Sublime Music (desktop), Audinaut, SubFire, iSub (iOS).

### Ampache client compatibility targets

Must work with Ampache-compatible clients: Ario, Amarok (Ampache plugin), Clementine
(Ampache source), Rhythmbox (Ampache plugin).

### Authentication compatibility

- Subsonic: token auth (`?t=&s=`) and legacy plaintext (`?p=`) — both required for client compatibility
- Ampache: handshake-based session tokens (SHA256 and MD5 legacy) — both required for client compatibility
- Native API: Bearer token (session or API token)
- All three auth schemes rate-limited independently
