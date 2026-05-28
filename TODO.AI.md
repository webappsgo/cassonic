# TODO.AI.md — cassonic implementation tasks

Bootstrap completed: 2026-05-28
IDEA.md updated: 2026-05-28 (full music server spec)

---

## Task Dependency Order

```
config → paths → mode
  └─→ db-schema + models
        └─→ store (users.db + server.db)
              └─→ security-middleware
                    └─→ auth (native + subsonic + ampache)
                          └─→ library-scanner
                                ├─→ tag-editor
                                ├─→ musicbrainz-service
                                │     └─→ tag-editor (lookup UI)
                                ├─→ cover-art
                                ├─→ ffmpeg
                                │     ├─→ audio-streaming
                                │     │     └─→ lastfm-scrobbling
                                │     └─→ icecast-streaming
                                ├─→ podcast-service
                                │     ├─→ subsonic-podcast-api
                                │     └─→ ampache-podcast-api
                                ├─→ upload-service
                                │     ├─→ subsonic-upload
                                │     └─→ native-upload-api
                                ├─→ share-service
                                │     ├─→ subsonic-shares-api
                                │     └─→ native-shares-api
                                ├─→ subsonic-api
                                ├─→ ampache-api
                                ├─→ native-api
                                └─→ webui
                                      ├─→ player-ui
                                      ├─→ tag-editor-ui
                                      ├─→ icecast-ui
                                      ├─→ podcast-ui
                                      ├─→ upload-ui
                                      └─→ shares-ui
server-cli → all of the above
scheduler → ssl + geoip + backup + update + musicbrainz-autolookup
admin-panel → all services
i18n → webui + admin
tor → ssl
client-binary → native-api
tests → everything
```

---

## FOUNDATION (PART 4–9)

### [ ] Config package
Read: AI.md PART 5

Scope: `src/config/config.go`, `src/config/bool.go`
- `config.ParseBool()` supporting 40+ boolean variants (yes/no, true/false, 1/0, on/off, enable/disable, enabled/disabled, active/inactive, allow/deny, accept/reject, affirmative/negative, positive/negative, y/n, t/f, set/unset, checked/unchecked, selected/deselected)
- Runtime hostname/IP/CPU count/memory detection (never hardcoded)
- `server.yml` auto-generation on first run with sane defaults
- Music-specific config sections: `[library]`, `[transcoding]`, `[icecast]`, `[subsonic]`, `[ampache]`, `[tags]`
- Mode detection: `--mode` flag → `MODE` env → default (production)
- Debug detection: `--debug` flag → `DEBUG` env → default (false)
- YAML comments always above settings, never inline

### [ ] OS-specific paths package
Read: AI.md PART 4

Scope: `src/paths/paths.go`
- Container: `/config`, `/data`, `/cache`, `/log`, `/backup`
- Linux privileged: `/etc/local/cassonic`, `/var/lib/local/cassonic`, `/var/cache/local/cassonic`, `/var/log/local/cassonic`
- Linux user: `~/.config/local/cassonic`, `~/.local/share/local/cassonic`, `~/.cache/local/cassonic`
- macOS, Windows, FreeBSD/BSD paths per PART 4 spec
- Music-specific subdirs: `{data_dir}/music/` (library root default), `{data_dir}/covers/` (extracted art cache), `{data_dir}/bin/` (ffmpeg binary), `{data_dir}/thumbs/` (cover art thumbnails)
- Runtime detection of which path set applies (container → privileged → user)

### [ ] Application mode detection
Read: AI.md PART 6

Scope: `src/mode/mode.go`
- TUI / GUI / CLI / daemon / smart-detect mode dispatch
- Mode priority: explicit `--mode` → environment → auto-detect (TTY + DISPLAY/WAYLAND_DISPLAY)
- Headless/daemon fallback when no display server

### [ ] Server binary CLI
Read: AI.md PART 8

Scope: `src/main.go` (replace stub)
- All standard flags: `--help`, `--version`, `--status`, `--mode`, `--config`, `--data`, `--log`, `--pid`, `--address`, `--port`, `--baseurl`, `--debug`, `--daemon`, `--service`, `--maintenance`, `--update`, `--lang`
- Music-specific flags: `--library` (add scan path at startup), `--scan` (trigger scan and exit), `--ffmpeg` (override ffmpeg path)
- Startup sequence: config load → privilege check → dir creation → DB init → schema migration → first-run wizard → library scan (if configured) → server start
- PID file management
- Signal handling: SIGTERM/SIGINT = graceful shutdown (finish active streams), SIGUSR1 = log reopen, SIGUSR2 = state dump (active streams, scan status)
- Banner: version, mode, address, config/data dirs, active library count

### [ ] Error handling and caching
Read: AI.md PART 9

Scope: `src/common/errors/errors.go`, `src/common/cache/cache.go`
- RFC 7807 error body: `{"type","title","status","detail","instance"}`
- In-memory LRU cache (cover art, transcoded chunks, search results)
- Cache size derived from available memory at startup (never hardcoded)
- Cache invalidation on library re-scan

---

## DATA LAYER (PART 10)

### [ ] Music data models
Read: AI.md PART 10, IDEA.md

Scope: `src/server/model/music.go`, `src/server/model/library.go`, `src/server/model/icecast.go`

**Library folder model:**
- `id`, `path`, `name`, `enabled`, `last_scan_at`, `song_count`, `created_at`

**Artist model:**
- `id`, `name`, `sort_name`, `musicbrainz_artist_id`, `biography`, `image_url`, `album_count`, `song_count`, `created_at`, `updated_at`

**Album model:**
- `id`, `title`, `sort_title`, `artist_id`, `album_artist`, `year`, `genre`, `disc_count`, `song_count`, `duration_secs`, `musicbrainz_album_id`, `musicbrainz_release_group_id`, `cover_art_id`, `created_at`, `updated_at`

**Song model:**
- `id`, `path`, `library_id`, `title`, `sort_title`, `artist_id`, `album_id`, `album_artist`, `track_number`, `disc_number`, `year`, `genre`, `composer`, `comment`, `bpm`, `compilation`, `duration_secs`, `bitrate`, `sample_rate`, `channels`, `file_size`, `file_format`, `content_type`, `musicbrainz_track_id`, `musicbrainz_artist_id`, `musicbrainz_album_id`, `musicbrainz_release_group_id`, `lyrics`, `cover_art_id`, `play_count`, `last_played_at`, `user_edited` (bool — non-empty fields protected from overwrite; empty/null fields still eligible for repopulation), `file_mtime`, `created_at`, `updated_at`

**Genre model:**
- `id`, `name`, `song_count`, `album_count`

**Cover art model:**
- `id`, `data` (BLOB), `mime_type`, `width`, `height`, `source` (embedded/directory/url), `created_at`

**Playlist model:**
- `id`, `name`, `comment`, `owner_id`, `public`, `song_count`, `duration_secs`, `created_at`, `updated_at`

**Playlist entry model:**
- `id`, `playlist_id`, `song_id`, `position`

**User activity models:**
- `stars` table: `user_id`, `item_type` (song/album/artist), `item_id`, `created_at`
- `ratings` table: `user_id`, `item_type`, `item_id`, `rating` (1–5)
- `play_history` table: `user_id`, `song_id`, `played_at`, `client_name`
- `bookmarks` table: `user_id`, `song_id`, `position_ms`, `comment`, `created_at`, `updated_at`
- `play_queues` table: `user_id`, `current_index`, `position_ms`, `changed_by`, `changed_at`
- `play_queue_entries` table: `queue_id`, `song_id`, `position`

**Icecast models:**
- `icecast_servers` table: `id`, `name`, `host`, `port`, `admin_user`, `admin_pass_enc`, `source_pass_enc`, `enabled`, `created_at`
- `icecast_mounts` table: `id`, `server_id`, `mount_path`, `display_name`, `description`, `format` (mp3/ogg/opus), `bitrate`, `scope` (all/artist/genre), `scope_id`, `status` (idle/streaming/error), `current_song_id`, `started_at`, `created_at`, `updated_at`

### [ ] Database schema and store layer
Read: AI.md PART 10

Scope: `src/server/store/store.go`, `src/server/store/sqlite.go`, `src/server/store/music_store.go`, `src/server/store/icecast_store.go`

- SQLite: `server.db` (music data + icecast) and `users.db` (auth)
- `CREATE TABLE IF NOT EXISTS` + idempotent `ALTER TABLE` on startup (no migration files)
- Parameterized queries everywhere — never string interpolation
- No `SELECT *` — always name columns
- Indexes: `songs(artist_id)`, `songs(album_id)`, `songs(path)`, `songs(genre)`, `songs(user_edited)`, `albums(artist_id)`, `play_history(user_id, played_at)`, `stars(user_id, item_type, item_id)`
- Connection pool limits per PART 10
- Transactions with `defer tx.Rollback()`
- Music store interface: `SearchArtists`, `SearchAlbums`, `SearchSongs`, `GetSongByPath`, `UpsertSong`, `MarkSongMissing`, `GetCoverArt`, `UpsertCoverArt`

---

## SECURITY & AUTH (PART 11)

### [ ] Security middleware
Read: AI.md PART 11

Scope: `src/server/middleware/middleware.go`, `src/server/middleware/auth.go`

- Middleware order: Allowlist → Blocklist → RateLimit → GeoIP → Auth
- Rate limiting: separate limiters for native API, Subsonic API, Ampache API, login endpoints
- Security headers (production only): CSP, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin`, HSTS (HTTPS only)
- `X-Request-ID` generation and propagation on all responses
- IP allowlist/blocklist per PART 11

### [ ] Authentication — three schemes
Read: AI.md PART 11, IDEA.md

Scope: `src/server/middleware/auth_native.go`, `src/server/middleware/auth_subsonic.go`, `src/server/middleware/auth_ampache.go`

**Native API auth:**
- Bearer token (opaque session token or JWT)
- Session tokens stored in `users.db`, Argon2id hashed
- API tokens: SHA-256 stored, never plaintext

**Subsonic auth (client compat — required):**
- Token auth: `?u=user&t=md5(password+salt)&s=salt` — preferred
- Legacy plaintext: `?u=user&p=password` — required for older clients
- Legacy hex-encoded: `?u=user&p=enc:hexstring`
- All three rate-limited independently via username + IP
- Failed auth response: Subsonic XML/JSON error format (code 40/41)
- Constant-time comparison to prevent timing attacks

**Ampache auth (client compat — required):**
- Handshake: SHA256(`timestamp` + SHA256(`password`)) or MD5 legacy
- Session token returned from `action=handshake`; stored server-side with TTL
- `action=goodbye` invalidates session
- Both v5 and v6 handshake formats

---

## LIBRARY & SCANNING

### [ ] Library scanner
Read: IDEA.md (Library Scanning section)

Scope: `src/server/service/scanner.go`, `src/server/service/scanner_walk.go`

- Recursive walk of all configured library folders
- Supported extensions: `.mp3`, `.flac`, `.ogg`, `.opus`, `.m4a`, `.aac`, `.wav`, `.aiff`, `.wma`, `.ape`
- Scan modes: `full` (re-read all tags regardless of mtime) and `incremental` (skip if mtime unchanged)
- For each file: read tags → upsert artist → upsert album → upsert song
- `user_edited = true` songs: per-field rule — if field is non-empty in DB → skip (user set it); if field is empty/null in DB → update from scan (user cleared it, wants it refilled)
- Missing files: set `available = false` in DB; clean up on next full scan after 2 missed full scans
- Progress tracking: `scan_status` table with `files_total`, `files_done`, `started_at`, `finished_at`, `status` (running/done/error)
- Concurrent walk with bounded goroutine pool (size = `min(CPU count, 8)`)
- Scan triggered by: admin UI, `POST /api/v1/libraries/{id}/scan`, `--scan` flag, first run (if `scan_on_start` config enabled)

### [ ] Tag reading and parsing
Read: IDEA.md (Tag Editor section)

Scope: `src/server/service/tags/reader.go`, `src/server/service/tags/formats.go`

Library: `github.com/nicholasgasior/gsfmt` or `go.senan.xyz/taglib` (evaluate: must support all 7 formats with pure Go or cgo-free binding)

Alternative: use `github.com/dhowden/tag` for reading + per-format writers for writing

- Read support: MP3 (ID3v2.3, ID3v2.4, ID3v1 fallback), FLAC (Vorbis Comments), OGG Vorbis, Opus, M4A/AAC (iTunes atoms), WAV (ID3 chunk), AIFF (ID3 chunk)
- Normalize all format-specific field names to the internal Song model
- Extract embedded cover art as raw bytes + MIME type
- Parse MusicBrainz IDs from tag frames: `TXXX:MusicBrainz Track Id`, `TXXX:MusicBrainz Artist Id`, `TXXX:MusicBrainz Album Id`, `TXXX:MusicBrainz Release Group Id`
- Handle malformed tags defensively: size limits, encoding guards, never crash on bad data
- Duration calculation: from header if available, ffprobe fallback

### [ ] Tag writing
Read: IDEA.md (Tag Editor section)

Scope: `src/server/service/tags/writer.go`, `src/server/service/tags/writer_mp3.go`, `src/server/service/tags/writer_flac.go`, `src/server/service/tags/writer_m4a.go`, `src/server/service/tags/writer_ogg.go`

- Write check: `os.Access(path, W_OK)` before any write attempt; return `ErrNotWritable` if fails
- MP3: always write ID3v2.4; preserve non-music frames; never downgrade to ID3v2.3 on rewrite
- FLAC: Vorbis Comment block; preserve other metadata blocks
- OGG/Opus: Vorbis Comment in first packet
- M4A: iTunes atom writing; preserve non-music atoms
- WAV/AIFF: ID3 chunk
- All writes: atomic (write to `.tmp` then `os.Rename`)
- After successful write: set `user_edited = true` in DB; record which specific fields were written; update `file_mtime`
- Write rule applied field-by-field: only fields present in the PATCH request body are written and marked; unmentioned fields retain their current `user_edited` state
- MusicBrainz IDs: written to appropriate frames/atoms per format

### [ ] Cover art service
Read: IDEA.md

Scope: `src/server/service/cover_art.go`

- Source priority: embedded tag → `cover.jpg` → `folder.jpg` → `album.jpg` → `front.jpg` in same directory
- Extracted art stored as BLOB in `server.db` (cover_art table), keyed by `cover_art_id`
- Thumbnails: resize to 300×300 and 64×64 via `golang.org/x/image` (no cgo, no ImageMagick)
- Thumbnail cache: `{data_dir}/thumbs/{id}_{size}.webp`
- `GET /rest/getCoverArt.view?id=&size=` — Subsonic cover art endpoint
- `GET /api/v1/songs/{id}/cover-art?size=` — native endpoint
- `POST /api/v1/songs/{id}/cover-art` — upload and embed new art (writes to file, updates DB)
- Cover art served with far-future cache headers (`ETag` = cover_art_id)

---

## FFMPEG INTEGRATION

### [ ] ffmpeg binary management
Read: IDEA.md (ffmpeg Integration section)

Scope: `src/server/service/ffmpeg/ffmpeg.go`, `src/server/service/ffmpeg/download.go`

- Detect ffmpeg at: configured `ffmpeg_path` → `{data_dir}/bin/ffmpeg` → system `$PATH`
- Auto-download on first need if not found:
  - URL: `https://github.com/binmgr/ffmpeg/releases/latest/download/ffmpeg-{os}-{arch}.tar.gz`
  - Platforms: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64
  - Verify SHA256 from release checksums file before extracting
  - Extract to `{data_dir}/bin/ffmpeg`, chmod 0755
- Version check: run `ffmpeg -version` and cache result
- All subprocess invocations: always include `-nostdin`, `-loglevel error`
- All path arguments: sanitized and validated against allowed library paths before passing
- Never construct ffmpeg args from raw user input

### [ ] Audio transcoding service
Read: IDEA.md (Transcoding & Streaming section)

Scope: `src/server/service/ffmpeg/transcode.go`

- Transcode any format → MP3 (libmp3lame), OGG (libvorbis), Opus (libopus), AAC, FLAC
- Bitrate options: 32, 48, 64, 96, 128, 160, 192, 256, 320 kbps
- `TranscodeStream(ctx, inputPath, format, bitrate) (io.ReadCloser, error)` — returns pipe from ffmpeg stdout
- `ConvertFile(ctx, inputPath, outputPath, format, bitrate) error` — for download conversion
- `ExtractCoverArt(ctx, inputPath) ([]byte, string, error)` — extract cover art frame
- `ProbeFile(ctx, inputPath) (FileInfo, error)` — duration, bitrate, format, sample rate, channels
- ffmpeg stdout piped directly to HTTP response for streaming (no temp files)
- Context cancellation: ffmpeg process killed when client disconnects

---

## AUDIO STREAMING (NATIVE)

### [ ] HTTP audio streaming handler
Read: IDEA.md (Transcoding & Streaming section)

Scope: `src/server/handler/stream.go`

- `GET /api/v1/songs/{id}/stream` params: `format` (mp3/ogg/opus/flac/aac/original), `maxBitRate` (int kbps)
- If format = original and no bitrate limit: stream file directly with `io.Copy` (no ffmpeg)
- If format ≠ original or bitrate limit set: transcode via ffmpeg pipe
- HTTP Range requests: supported for direct file streaming (no range for transcoded streams)
- Response headers: `Content-Type`, `Content-Length` (direct only), `Accept-Ranges: bytes` (direct only), `X-Content-Duration`, `Cache-Control: no-store` (transcoded)
- `GET /api/v1/songs/{id}/download` — always original file, with `Content-Disposition: attachment`
- Scrobble: record play to `play_history` after 50% of duration played or 4 minutes
- Active streams tracked in memory for `GET /api/v1/now-playing`

---

## SUBSONIC API COMPATIBILITY LAYER

### [ ] Subsonic API — core infrastructure
Read: IDEA.md (Subsonic REST API section)

Scope: `src/server/handler/subsonic/subsonic.go`, `src/server/handler/subsonic/response.go`, `src/server/handler/subsonic/auth.go`

- All endpoints mount at `/rest/`
- Method dispatch via URL path suffix (`.view`)
- Response format: XML (default) or JSON (`?f=json`)
- XML root element `<subsonic-response xmlns="http://subsonic.org/restapi" status="ok" version="1.16.1">`
- JSON wrapper: `{"subsonic-response":{"status":"ok","version":"1.16.1",...}}`
- Error responses: `<error code="N" message="..."/>` with correct Subsonic error codes (0=generic, 10=missing param, 20=wrong version, 30=not auth, 40=wrong creds, 41=token auth required, 50=not permitted, 70=not found)
- Auth middleware: token auth + legacy plaintext + hex-encoded (all three)
- Version negotiation: accept any client version ≥ 1.1.0; always respond as 1.16.1
- `?callback=` JSONP support (legacy client compat)

### [ ] Subsonic API — system endpoints
Read: IDEA.md

Scope: `src/server/handler/subsonic/system.go`
- `ping.view` — returns empty OK response
- `getLicense.view` — always valid, perpetual, email from config
- `getScanStatus.view` — current scan status from DB
- `startScan.view` — trigger incremental scan (admin only)
- `getUser.view` / `getUsers.view` (admin)
- `createUser.view` / `updateUser.view` / `deleteUser.view` (admin)
- `changePassword.view`

### [ ] Subsonic API — browsing endpoints
Read: IDEA.md

Scope: `src/server/handler/subsonic/browse.go`
- `getMusicFolders.view` — configured library folders
- `getIndexes.view` — artists by first letter, with `ifModifiedSince` support
- `getMusicDirectory.view` — directory listing (folder-based browsing)
- `getGenres.view` — all genres with song/album counts
- `getArtists.view` — ID3-based artist list (alphabetical index)
- `getArtist.view` — artist detail with albums
- `getAlbum.view` — album detail with songs
- `getSong.view` — single song detail
- `getAlbumList.view` / `getAlbumList2.view` — recent/newest/highest/frequent/random/starred/byYear/byGenre
- `getRandomSongs.view` — random songs with filters
- `getSongsByGenre.view`
- `getStarred.view` / `getStarred2.view`
- `getNowPlaying.view`
- `getVideos.view` / `getVideoInfo.view` — return empty (audio-only server)
- `getArtistInfo.view` / `getArtistInfo2.view` — biography, similar artists
- `getAlbumInfo.view` / `getAlbumInfo2.view`
- `getSimilarSongs.view` / `getSimilarSongs2.view`
- `getTopSongs.view`

### [ ] Subsonic API — streaming endpoints
Read: IDEA.md

Scope: `src/server/handler/subsonic/stream.go`
- `stream.view` — stream with optional `maxBitRate`, `format`, `estimateContentLength`, `converted`
- `download.view` — download original file
- `hls.view` — HLS playlist (m3u8) for adaptive streaming
- `getCoverArt.view` — cover art by ID, optional `size` param
- `getLyrics.view` — embedded lyrics from tags
- `getAvatar.view` — user avatar (return default if not set)
- `getCaptions.view` — not applicable, return error 70

### [ ] Subsonic API — playlist and user interaction endpoints
Read: IDEA.md

Scope: `src/server/handler/subsonic/playlists.go`, `src/server/handler/subsonic/interaction.go`
- `getPlaylists.view` / `getPlaylist.view` / `createPlaylist.view` / `updatePlaylist.view` / `deletePlaylist.view`
- `search.view` (legacy) / `search2.view` / `search3.view` — full-text search artists/albums/songs
- `star.view` / `unstar.view` — star songs/albums/artists
- `setRating.view` — rate songs/albums
- `scrobble.view` — record play with `submission` flag
- `getShares.view` / `createShare.view` / `updateShare.view` / `deleteShare.view`
- `getBookmarks.view` / `createBookmark.view` / `deleteBookmark.view`
- `getPlayQueue.view` / `savePlayQueue.view`
- `getChatMessages.view` / `addChatMessage.view` — in-memory or DB chat

### [ ] Subsonic API — podcast and internet radio endpoints
Read: IDEA.md

Scope: `src/server/handler/subsonic/podcast.go`, `src/server/handler/subsonic/radio.go`
- `getPodcasts.view` — list channels with optional episode list (`includeEpisodes` param)
- `getNewestPodcasts.view` — most recently published episodes across all channels
- `refreshPodcasts.view` — trigger RSS re-fetch for all channels (admin)
- `createPodcastChannel.view` — add channel by RSS URL (admin)
- `deletePodcastChannel.view` — remove channel + episodes (admin)
- `deletePodcastEpisode.view` — delete downloaded episode file
- `downloadPodcastEpisode.view` — queue episode for download
- `getInternetRadioStations.view` / `createInternetRadioStation.view` / `updateInternetRadioStation.view` / `deleteInternetRadioStation.view`
- `jukeboxControl.view` — return error 0 (not supported, jukebox = physical device)
- All responses use fully populated `<podcast>` / `<podcastEpisode>` elements with correct status values (skipped/downloading/completed/error)

---

## AMPACHE API COMPATIBILITY LAYER

### [ ] Ampache API — core infrastructure
Read: IDEA.md (Ampache API section)

Scope: `src/server/handler/ampache/ampache.go`, `src/server/handler/ampache/response.go`, `src/server/handler/ampache/auth.go`

- Endpoints: `/server/xml.server.php` (XML) and `/server/json.server.php` (JSON)
- Action dispatch via `?action=` query param
- Response format: XML `<root>` or JSON object depending on endpoint
- Both Ampache v5 and v6 response shapes — detect requested version from `?version=` in handshake
- Error format: `<error errorCode="N">message</error>` (XML) or `{"error":{"errorCode":N,"errorMessage":"..."}}`  (JSON)
- Auth: SHA256 and MD5 legacy handshake; sessions stored in `users.db` with TTL
- Constant-time comparison for auth tokens

### [ ] Ampache API — handshake and session actions
Read: IDEA.md

Scope: `src/server/handler/ampache/session.go`
- `handshake` — SHA256 or MD5 auth; return session token, server version, catalog counts
- `goodbye` — invalidate session
- `ping` — extend session TTL, return server status
- `check_parameter` — validate a parameter value

### [ ] Ampache API — catalog and browsing actions
Read: IDEA.md

Scope: `src/server/handler/ampache/catalog.go`, `src/server/handler/ampache/browse.go`
- `get_indexes` — artist/album/song/playlist index
- `advanced_search` — rule-based search (type, operator, value triples)
- `artists` / `artist` / `artist_albums` / `artist_songs`
- `albums` / `album` / `album_songs`
- `songs` / `song` / `song_delete` (admin)
- `genres` / `genre` / `genre_songs` / `genre_albums` / `genre_artists`
- `labels` / `label` / `label_artists` — map genres to labels
- `catalogs` / `catalog` / `catalog_songs` / `catalog_albums` / `catalog_artists` / `catalog_action` / `catalog_file`
- `system_update` — trigger library re-scan (admin)
- `stats` — server statistics (song count, album count, artist count, play count)

### [ ] Ampache API — streaming and artwork actions
Read: IDEA.md

Scope: `src/server/handler/ampache/stream.go`
- `stream` — stream audio with optional `format`, `bitrate`, `offset`
- `download` — download file
- `get_art` — cover art by `id` and `type` (song/album/artist)
- `update_art` — fetch and update art from URL (admin)
- `update_artist_info` — fetch biography from external source (admin)
- `upload` — upload a new audio file to a library folder (if enabled)

### [ ] Ampache API — playlist and interaction actions
Read: IDEA.md

Scope: `src/server/handler/ampache/playlists.go`, `src/server/handler/ampache/interaction.go`
- `playlists` / `playlist` / `playlist_songs` / `playlist_create` / `playlist_edit` / `playlist_delete`
- `playlist_add_song` / `playlist_remove_song` / `playlist_generate`
- `searches` / `search_songs`
- `flag` — star/unstar item
- `rate` — rate item 1–5
- `record_play` — scrobble
- `scrobble` — scrobble with metadata
- `now_playing` — currently playing tracks
- `get_similar` — similar songs/artists (based on genre/tags)
- `shares` / `share` / `share_create` / `share_edit` / `share_delete`
- `bookmarks` / `bookmark_create` / `bookmark_edit` / `bookmark_delete` / `get_bookmark`
- `deleted_songs` / `deleted_video` / `deleted_podcast_episodes`

### [ ] Ampache API — user management actions
Read: IDEA.md

Scope: `src/server/handler/ampache/users.go`
- `user` / `users` / `user_create` / `user_edit` / `user_delete` (admin)
- `user_preferences` / `user_preference`
- `system_preferences` / `system_preference` (admin)
- `preference_create` / `preference_edit` / `preference_delete` (admin)
- `toggle_follow` / `last_shouts` / `timeline` / `friends_timeline`

### [ ] Ampache API — podcast and live stream actions
Read: IDEA.md

Scope: `src/server/handler/ampache/podcast.go`, `src/server/handler/ampache/radio.go`
- `podcasts` / `podcast` — list/get channels; backed by same podcast service as Subsonic layer
- `podcast_create` / `podcast_edit` / `podcast_delete` (admin)
- `podcast_episodes` / `podcast_episode` / `podcast_episode_delete`
- `update_podcast` — trigger RSS re-fetch for one channel
- `live_streams` / `live_stream` / `live_stream_create` / `live_stream_edit` / `live_stream_delete`
- Both v5 and v6 response shapes for all podcast objects

---

## ICECAST STREAMING

### [ ] Icecast streaming service
Read: IDEA.md (Icecast Streaming section)

Scope: `src/server/service/icecast/icecast.go`, `src/server/service/icecast/mount.go`, `src/server/service/icecast/source.go`

**Connection protocol:** Raw Icecast source protocol (HTTP PUT to `http://source:pass@host:port/mount`) — no external libshout dependency.

**Mount manager:**
- One goroutine per active mount (`StreamingMount` struct with `context.Context` for lifecycle)
- Goroutine started on `POST /api/v1/icecast-mounts/{id}/start`, stopped on `.../stop`
- Goroutines survive server-side reconnect: on connection drop, retry with exponential backoff (max 30s)
- Goroutines persist across config reloads; only restart if mount config changed

**Track selection by scope:**
- `all`: shuffle entire library (Fisher-Yates on song ID list), loop when exhausted
- `artist`: all songs by `scope_id` artist, shuffled, loop
- `genre`: all songs in `scope_id` genre, shuffled, loop
- Queue rebuilt on library re-scan (mid-stream track finishes normally, then new queue used)

**Resume behavior:**
- On reconnect mid-stream: resume current track from beginning (not from disconnection offset, not restart from track 1)
- Track position state (`current_song_id`, `byte_offset`, `started_at`) held in mount goroutine memory and persisted to DB on track change
- ICY metadata: send `StreamTitle=Artist - Title` in-band every `icy-metaint` bytes (default 8192)

**ffmpeg pipeline per mount:**
- `ffmpeg -i {input_path} -vn -acodec {codec} -b:a {bitrate}k -f {format} pipe:1`
- stdout piped to Icecast source connection
- On track end (ffmpeg exits 0): advance to next track, start new ffmpeg process
- On ffmpeg error: log, skip to next track

**Credential handling:**
- Passwords stored AES-256-GCM encrypted in `server.db` (key derived from server secret)
- Never appear in logs (credential masking)
- Never sent to client in API responses

### [ ] Icecast API handlers
Read: IDEA.md (Native cassonic REST API — Icecast section)

Scope: `src/server/handler/icecast.go`
- Full CRUD for servers and mounts per IDEA.md route table
- `POST .../start` / `POST .../stop` — start/stop mount goroutine
- `GET .../status` — `{streaming: bool, current_song: {...}, listener_count: N, uptime_secs: N}`
- Admin-only for server/mount create/update/delete; authenticated users can view status

---

## NATIVE REST API

### [ ] Native API — library and browsing handlers
Read: IDEA.md (Native cassonic REST API section), AI.md PART 14

Scope: `src/server/handler/api/library.go`, `src/server/handler/api/browse.go`
- All routes per IDEA.md route table: `/api/v1/libraries`, `/api/v1/artists`, `/api/v1/albums`, `/api/v1/songs`, `/api/v1/genres`, `/api/v1/search`
- Pagination: `?limit=` (default 50, max 500) + `?offset=` on all list endpoints
- Sorting: `?sort=` + `?order=asc|desc` on all list endpoints
- Search: full-text across artist name, album title, song title, genre
- Response shape: `{"ok":true,"data":{...}}` with RFC 7807 errors

### [ ] Native API — tag editor handlers
Read: IDEA.md (Tag Editor section)

Scope: `src/server/handler/api/tags.go`
- `GET /api/v1/songs/{id}/tags` — return all tag fields + `writable` bool
- `PATCH /api/v1/songs/{id}/tags` — partial update; only write fields present in request body
- `GET /api/v1/songs/{id}/tags/writable` — `{"writable": true/false}`
- `POST /api/v1/songs/{id}/cover-art` — multipart upload, embed in file, update DB
- `DELETE /api/v1/songs/{id}/cover-art` — remove embedded art, update DB
- After write: set `user_edited = true`, update DB, return updated tags; empty values in request body clear the field (repopulation eligible on next scan/lookup)
- Return `409 Conflict` (RFC 7807) if file not writable

### [ ] Native API — playlist and user activity handlers
Read: IDEA.md

Scope: `src/server/handler/api/playlists.go`, `src/server/handler/api/activity.go`
- All playlist CRUD per IDEA.md route table
- `POST /api/v1/songs/{id}/scrobbles`
- `POST /api/v1/songs/{id}/stars` / `DELETE .../stars`
- `PATCH /api/v1/songs/{id}/rating` — `{"rating": 1-5}`
- `GET/PUT /api/v1/play-queues` — get/replace current user play queue
- `GET/POST/DELETE /api/v1/bookmarks` / `/api/v1/bookmarks/{id}`
- `GET /api/v1/now-playing` — list all active streams with user (admin sees all, user sees own)

### [ ] Native API — user and auth handlers
Read: AI.md PART 11, IDEA.md

Scope: `src/server/handler/api/auth.go`, `src/server/handler/api/users.go`
- `POST /api/v1/auth/login` → session token (Argon2id verify, return opaque token)
- `POST /api/v1/auth/logout` → invalidate token
- `POST /api/v1/auth/tokens` → create API token (SHA-256 stored)
- `DELETE /api/v1/auth/tokens/{id}` → revoke
- Full user CRUD per IDEA.md route table

---

## WEB FRONTEND (PART 16)

### [ ] Base template layout and theme system
Read: AI.md PART 16, IDEA.md (WebUI Design Requirements)

Scope: `src/server/template/layout/`, `src/server/static/css/`, `src/server/static/js/`

- Base layout: `<head>` with theme CSS variables, viewport meta, `<main>`, `<footer>` (persistent player bar)
- CSS custom properties (no hardcoded colors): `--surface-bg`, `--surface-card`, `--text-primary`, `--text-secondary`, `--accent`, `--border`, `--player-bg`, `--player-text`
- Dark theme (default), light theme, auto (prefers-color-scheme)
- Theme toggle stored in `localStorage` + cookie fallback
- Mobile-first breakpoints: 320px base, 768px tablet, 1024px desktop
- Touch targets ≥ 44×44px everywhere
- Fonts: system font stack (no external font loading)

### [ ] Persistent bottom player bar
Read: IDEA.md (WebUI Design Requirements)

Scope: `src/server/template/partials/player_bar.html`, `src/server/static/js/player.js`

- Always visible at bottom of page on all routes
- Shows: cover art thumbnail, song title, artist, album, progress bar, play/pause, prev/next, volume, shuffle, repeat
- Expands to full-screen player on tap/click
- HTML5 `<audio>` element with JS enhancement
- SSE endpoint `GET /api/v1/events/now-playing` for real-time "now playing" updates (EventSource)
- Keyboard shortcuts: Space=play/pause, Left/Right arrow=seek ±10s, Shift+Left/Right=prev/next track
- Play queue: JS-managed array of song IDs; fetches stream URL on play
- Graceful degradation: without JS, player links open individual song pages

### [ ] WebUI — library browsing pages
Read: IDEA.md (WebUI Routes section)

Scope: `src/server/template/pages/`

- `GET /` — home dashboard: recently played, recently added, now playing, random picks
- `GET /library` — top-level library browser
- `GET /artists` — paginated artist grid with cover art
- `GET /artists/{id}` — artist detail: image, bio, album grid
- `GET /albums` — paginated album grid
- `GET /albums/{id}` — album detail: cover art, tracklist table with play buttons
- `GET /songs/{id}` — song detail: full tag display, play button, "edit tags" link (if writable)
- `GET /genres` — genre tiles with song/album counts
- `GET /genres/{id}` — genre browser with album/song list
- `GET /playlists` — user's playlists + public playlists
- `GET /playlists/{id}` — playlist detail with drag-reorder (JS) / reorder buttons (no-JS)
- `GET /search` — search results page (GET `?q=` param, server-side rendered)
- `GET /player` — full-screen expanded player with queue view
- `GET /login` — login form

### [ ] WebUI — tag editor page
Read: IDEA.md (Tag Editor section)

Scope: `src/server/template/pages/tag_editor.html`

- `GET /tags/{id}` — tag editor for a song
- Displays current tag values in editable form fields
- All tag fields: title, artist, album artist, album, track, disc, year, genre, composer, comment, BPM, compilation checkbox
- MusicBrainz section: 4 ID fields with "Look up" button (AJAX to MusicBrainz API, JS-enhanced; shows lookup result for user to confirm)
- Cover art: preview thumbnail, upload new art button, remove art button
- Writable indicator: green lock icon if writable, red lock with message if not
- Submit → `PATCH /api/v1/songs/{id}/tags` → success/error flash message
- No-JS fallback: standard form POST with redirect

### [ ] WebUI — Icecast stream manager page
Read: IDEA.md (Icecast Streaming section)

Scope: `src/server/template/pages/icecast.html`

- `GET /icecast` — lists all mount points with live status (streaming/idle/error)
- Per-mount: start/stop toggle button, current song playing, scope badge (all/artist/genre)
- "Add mount" form: server select, mount path, format, bitrate, scope selector
  - Scope = artist: shows searchable artist dropdown
  - Scope = genre: shows genre dropdown
- Server management section (admin only): add/edit/delete Icecast servers

### [ ] WebUI — settings and user pages
Read: AI.md PART 16

Scope: `src/server/template/pages/settings.html`
- `GET /settings` — user settings: display name, password change, language preference, theme preference, API token management, Subsonic credentials view, scrobbling section (all services: add/edit/delete/toggle/verify/queue status)

---

## ADMIN PANEL (PART 17)

### [ ] Admin panel core
Read: AI.md PART 17

Scope: `src/server/handler/admin.go`, `src/admin/`

- Setup wizard (first-run): create primary admin account, set library path, configure SMTP (optional)
- Admin panel at `/server/{admin_path}/` (configurable, default: `admin`)
- MFA: TOTP (RFC 6238) for admin accounts
- Dashboard: system stats, active streams, scan status, recent errors, disk usage by library
- Config management: edit `server.yml` via web form (key settings only, not raw YAML)

### [ ] Admin panel — library management
Read: AI.md PART 17, IDEA.md

Scope: `src/server/template/pages/admin/libraries.html`
- `GET /server/{admin_path}/libraries` — list libraries with song counts, last scan time
- Add/remove library folders
- Trigger full or incremental scan per library
- Live scan progress via SSE (`GET /api/v1/libraries/{id}/scan` streaming)
- Missing file cleanup control

### [ ] Admin panel — user management
Read: AI.md PART 17

Scope: `src/server/template/pages/admin/users.html`
- Full user CRUD from admin panel
- Per-user: Subsonic access flag, Ampache access flag, download flag, upload flag, tag edit flag
- Roles: admin / regular user
- Password reset (send email or show one-time link if no email configured)

### [ ] Admin panel — Icecast management
Read: IDEA.md

Scope: `src/server/template/pages/admin/icecast.html`
- Server add/edit/delete with connection test button
- Mount point full management
- Live status for all active mounts

---

## SERVER INFRASTRUCTURE (PARTS 12–15)

### [ ] Server startup and HTTP routing
Read: AI.md PART 12

Scope: `src/server/server.go`
- HTTP server with graceful shutdown (drain active streams before exit, max 30s)
- Route registration: native API, Subsonic `/rest/`, Ampache `/server/`, WebUI, admin, swagger, graphql, metrics, static files
- Context propagation: X-Request-ID, language, auth user
- TLS listener when SSL configured

### [ ] Health and version endpoints
Read: AI.md PART 13

Scope: `src/server/handler/health.go`
- `GET /server/healthz` — HTML page with system health (DB, ffmpeg, scan status, active streams)
- `GET /api/v1/server/healthz` — JSON health
- `GET /api/healthz` — JSON alias
- `GET /api/v1/server/version` — version + build info + subsonic version + ampache version

### [ ] SSL/TLS and Let's Encrypt
Read: AI.md PART 15

Scope: `src/ssl/ssl.go`
- TLS 1.2+ minimum; TLS 1.3 preferred
- Let's Encrypt auto-cert (ACME HTTP-01 and DNS-01)
- Cert paths: `{config_dir}/ssl/letsencrypt/` and `ssl/local/`
- Auto-renewal via scheduler (daily 03:00, 7 days before expiry)

---

## PLATFORM SERVICES (PARTS 18–25)

### [ ] Email and notifications
Read: AI.md PART 18

Scope: `src/server/service/email.go`, `src/server/template/email/`
- SMTP auto-detection on first run
- All 16 required email templates embedded in binary
- SMTP disabled = all email features hidden

### [ ] Built-in scheduler
Read: AI.md PART 19

Scope: `src/scheduler/scheduler.go`
- All 12 required built-in tasks plus music-specific additions:
  - `library_scan_daily` — incremental scan, configurable time (default 04:00)
  - `cover_art_refresh` — weekly, re-extract missing cover art
  - `ffmpeg_version_check` — weekly, log if newer static build available
  - `podcast_refresh` — every 4 hours, fetch all enabled channels
  - `musicbrainz_autolookup` — nightly 02:00, fills empty/null MusicBrainz ID fields (opt-in); skips non-empty fields on `user_edited` songs; repopulates empty fields even on `user_edited` songs
  - `scrobble_retry` — every 30 minutes, drain `scrobble_queue` for all enabled+verified services across all users; batch per service (up to 50 for Last.fm-compat, up to 1000 for ListenBrainz-compat); drop after 14 days or 50 attempts
- Persistent state in `server.db`

### [ ] GeoIP support
Read: AI.md PART 20

Scope: `src/server/service/geoip.go`
- `github.com/oschwald/maxminddb-golang` (not geoip2-golang)
- sapics/ip-location-db MMDB databases
- Weekly update via scheduler

### [ ] Prometheus metrics
Read: AI.md PART 21

Scope: `src/server/handler/metrics.go`
- Standard prefix: `cassonic_`
- Music-specific metrics: `cassonic_songs_total`, `cassonic_active_streams`, `cassonic_icecast_mounts_active`, `cassonic_library_scan_duration_seconds`, `cassonic_transcoding_duration_seconds`

### [ ] Backup and restore
Read: AI.md PART 22

Scope: `src/server/service/backup.go`
- Contents: `server.yml` + `server.db` + `users.db` + custom templates/themes
- Does NOT back up audio files (operator-managed)
- AES-256-GCM encryption (optional)

### [ ] Self-update command
Read: AI.md PART 23

Scope: `src/service/update.go`
- GitHub Releases API; SHA256 verify; atomic replace

### [ ] Privilege escalation and service install
Read: AI.md PART 24, PART 25

Scope: `src/service/service.go`
- systemd, OpenRC, launchd, Windows Service
- Create `cassonic` system user; drop privileges after port binding

---

## DOCUMENTATION & TOOLING (PARTS 26–33)

### [ ] Swagger and GraphQL endpoints
Read: AI.md PART 14

Scope: `src/swagger/swagger.go`, `src/graphql/graphql.go`
- `GET /swagger/` — Swagger UI with cassonic theme; all native API routes documented
- `GET /graphql/` — GraphQL playground; schema covers library browsing and user activity
- Subsonic and Ampache APIs documented in separate swagger files (compatibility layer reference)

### [ ] i18n — 7 locale files
Read: AI.md PART 31

Scope: `src/common/i18n/locales/{en,es,zh,fr,ar,de,ja}.json`
- All user-facing strings use translation keys
- Music-specific keys: player controls, tag field labels, genre names, error messages
- RTL support for Arabic
- Build-time key coverage check

### [ ] Tor hidden service
Read: AI.md PART 32

Scope: `src/ssl/tor.go`
- `github.com/cretz/bine`; auto-enable when `tor` binary found
- HiddenServiceVersion 3

### [ ] cassonic-cli binary
Read: AI.md PART 33, IDEA.md

Scope: `src/client/main.go` (replace stub)
- Commands mirroring native API: `library scan`, `library list`, `song tags get {id}`, `song tags set {id} --title --artist ...`, `playlist create`, `icecast start {mount-id}`, `icecast stop {mount-id}`, `icecast status`, `podcast add {url}`, `podcast list`, `podcast refresh {id}`, `episode download {id}`, `upload {file}`, `share create song {id}`, `share create album {id}`, `share list`, `share delete {id}`
- `--server URL` flag; config stored in `{config_dir}/cli.yml`
- JSON output for scripts; pretty-printed table for terminals
- `--format json|table|csv` flag

### [ ] MkDocs documentation updates
Read: AI.md PART 30

Scope: `docs/`
- `docs/api/subsonic.md` — Subsonic API compatibility notes, version support matrix
- `docs/api/ampache.md` — Ampache API compatibility notes
- `docs/api/native.md` — native REST API reference
- `docs/guides/tag-editor.md` — tag editing guide
- `docs/guides/icecast.md` — Icecast setup guide
- `docs/guides/clients.md` — compatible client setup guide
- `docs/guides/transcoding.md` — ffmpeg + transcoding config
- `docs/guides/podcasts.md` — adding podcast channels, episode management
- `docs/guides/musicbrainz.md` — auto-lookup config, manual tag lookup
- `docs/guides/scrobbling.md` — all supported services (Last.fm, ListenBrainz, Libre.fm, GNU FM, Maloja, custom), auth flows, enabled/verified flags, retry queue, fan-out behavior
- `docs/guides/upload.md` — enabling uploads, permissions, library routing
- `docs/guides/share-links.md` — creating shares, passwords, expiry, public URLs

---

## PODCASTS

### [ ] Podcast data models and store
Read: IDEA.md

Scope: `src/server/model/podcast.go`, `src/server/store/podcast_store.go`

**PodcastChannel model:**
- `id`, `url` (RSS feed URL), `title`, `description`, `image_url`, `author`, `category`, `language`, `episode_count`, `last_fetch_at`, `fetch_status` (ok/error), `fetch_error`, `created_at`, `updated_at`

**PodcastEpisode model:**
- `id`, `channel_id`, `guid`, `title`, `description`, `publish_date`, `duration_secs`, `content_url` (remote audio URL), `local_path` (downloaded file, nullable), `file_size`, `content_type`, `download_status` (skipped/queued/downloading/completed/error), `download_error`, `cover_art_id`, `play_count`, `created_at`, `updated_at`

Store interface: `GetChannels`, `GetChannel`, `UpsertChannel`, `DeleteChannel`, `GetEpisodes`, `GetEpisode`, `UpsertEpisode`, `DeleteEpisode`, `GetNewestEpisodes(limit)`

### [ ] Podcast RSS service
Read: IDEA.md

Scope: `src/server/service/podcast.go`, `src/server/service/podcast_rss.go`, `src/server/service/podcast_download.go`

**RSS fetch:**
- Parse RSS 2.0 and Atom feeds with `<itunes:*>` extension tags
- Extract: title, description, image, author, category, language, episode list
- Per-episode: GUID (dedup key), title, description, pubDate, duration (`<itunes:duration>`), enclosure URL + type + length
- Upsert channels and episodes using GUID as identity key; never delete episodes already downloaded
- HTTP fetch with timeout (30s); follow redirects (max 5); respect `ETag`/`Last-Modified` for conditional fetch
- Store fetch errors in channel record; do not crash on malformed feed

**Episode download:**
- Download goroutine per episode; bounded pool (max 3 concurrent downloads)
- Stream to `{data_dir}/podcasts/{channel_id}/{episode_id}.{ext}`
- Progress tracked in DB (`download_status = downloading`)
- Verify content-length after download; mark `completed` or `error`
- Deleted episode file → set `local_path = null`, `download_status = skipped`

**Scheduler task:** `podcast_refresh` — fetch all channels, configurable interval (default: every 4 hours); defined in scheduler task list

### [ ] Podcast API handlers (native)
Read: IDEA.md

Scope: `src/server/handler/api/podcasts.go`

Add to native API route table:

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/v1/podcasts` | List channels |
| POST | `/api/v1/podcasts` | Add channel by RSS URL (admin) |
| GET | `/api/v1/podcasts/{id}` | Channel detail |
| PATCH | `/api/v1/podcasts/{id}` | Update channel metadata (admin) |
| DELETE | `/api/v1/podcasts/{id}` | Delete channel + episodes (admin) |
| POST | `/api/v1/podcasts/{id}/refresh` | Trigger RSS re-fetch |
| GET | `/api/v1/podcasts/{id}/episodes` | List episodes for channel |
| GET | `/api/v1/podcast-episodes` | Newest episodes across all channels |
| GET | `/api/v1/podcast-episodes/{id}` | Episode detail |
| POST | `/api/v1/podcast-episodes/{id}/download` | Queue download |
| DELETE | `/api/v1/podcast-episodes/{id}/download` | Delete downloaded file |
| GET | `/api/v1/podcast-episodes/{id}/stream` | Stream episode (local if downloaded, proxy remote otherwise) |

### [ ] WebUI — podcast pages
Read: IDEA.md

Scope: `src/server/template/pages/podcasts.html`, `src/server/template/pages/podcast_detail.html`

Add to WebUI route table:
- `GET /podcasts` — channel list with cover art, episode count, last updated
- `GET /podcasts/{id}` — channel detail: description, episode list with download/play/delete controls
- `GET /podcasts/{id}/episodes/{episodeId}` — episode detail with player
- Download status shown as progress indicator (JS SSE poll or page refresh)
- "Add podcast" form (admin only): RSS URL input
- No-JS: all actions work via standard form POSTs

---

## MUSICBRAINZ

### [ ] MusicBrainz service
Read: IDEA.md

Scope: `src/server/service/musicbrainz/musicbrainz.go`, `src/server/service/musicbrainz/lookup.go`

**API client:**
- Base URL: `https://musicbrainz.org/ws/2/`
- JSON format (`fmt=json`)
- User-Agent header required by MusicBrainz policy: `cassonic/{version} ( {official_site} )`
- Rate limit: 1 request/second enforced by token bucket in client (never burst)
- Retry on 503 with `Retry-After` header respected
- All calls have 10s timeout

**Lookup types:**
- `LookupRecording(mbid string) (*Recording, error)` — by MusicBrainz track ID; returns title, artist credits, album, duration
- `LookupArtist(mbid string) (*Artist, error)` — by MusicBrainz artist ID; returns name, biography, begin/end dates
- `LookupRelease(mbid string) (*Release, error)` — by MusicBrainz album ID; returns title, date, label, cover art archive URL
- `SearchRecording(title, artist, album string) ([]Recording, error)` — fuzzy search, returns top 10 candidates
- `SearchArtist(name string) ([]Artist, error)` — fuzzy search

**Auto-lookup scheduler task** (`musicbrainz_autolookup`):
- Runs only when `musicbrainz_autolookup: true` in config (opt-in, default false)
- Scheduled: nightly at 02:00
- Query: `SELECT id FROM songs WHERE (musicbrainz_track_id = '' OR musicbrainz_artist_id = '' OR musicbrainz_album_id = '' OR musicbrainz_release_group_id = '') LIMIT 100`
- For each song: call `SearchRecording(title, artist, album)` → if single high-confidence match (score ≥ 90): populate only the empty ID fields
- Per-field write rule: if `user_edited = true` AND field is non-empty → skip that field; if field is empty/null → write it (user cleared it, wants it filled)
- `user_edited = false` songs: all matching fields updated unconditionally
- Log: count of songs updated; individual failures as warnings (never fatal)

**Manual lookup API** (user-initiated from tag editor):

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/v1/songs/{id}/musicbrainz/search` | Search MusicBrainz for matching recordings; returns candidates |
| POST | `/api/v1/songs/{id}/musicbrainz/apply` | Apply selected candidate's IDs to song tags |

- `apply` sets `user_edited = true` for the MusicBrainz fields — treated as user change, never auto-overwritten

---

## SCROBBLING

### [ ] Scrobble service data model
Read: IDEA.md

Scope: `src/server/model/scrobble.go`, `src/server/store/scrobble_store.go`

**`scrobble_services` table** (per user, one row per configured service):
- `id`, `user_id`, `service_type` (enum: `lastfm` / `listenbrainz` / `librefm` / `gnufm` / `maloja` / `custom_lastfm` / `custom_listenbrainz`), `display_name` (user-set label, e.g. "My Maloja"), `base_url` (nullable — required for gnufm/maloja/custom; blank for lastfm/librefm/listenbrainz which have fixed URLs), `api_key` (nullable), `api_secret_enc` (nullable, AES-256-GCM encrypted), `session_key_enc` (nullable, encrypted — Last.fm-compat auth result), `token_enc` (nullable, encrypted — ListenBrainz-compat Bearer token), `username` (nullable), `enabled` (bool, default true), `verified` (bool — set true only after a successful connection test), `last_verified_at` (nullable), `last_error` (nullable, last failure message), `created_at`, `updated_at`

**`scrobble_queue` table** (per service, per user):
- `id`, `user_id`, `service_id`, `track_data` (JSON blob: title, artist, album, duration, timestamp, mbid), `queued_at`, `attempts` (int), `last_attempt_at` (nullable), `last_error` (nullable)
- Index: `scrobble_queue(service_id, queued_at)`

**Fixed base URLs (built-in, not stored):**
- `lastfm` → `https://ws.audioscrobbler.com/2.0/`
- `librefm` → `https://libre.fm/2.0/`
- `listenbrainz` → `https://api.listenbrainz.org/1/`
- `gnufm`, `maloja`, `custom_lastfm`, `custom_listenbrainz` → operator-supplied `base_url`

### [ ] Scrobble backend interface and fan-out manager
Read: IDEA.md

Scope: `src/server/service/scrobble/scrobble.go`, `src/server/service/scrobble/manager.go`

**Backend interface** (implemented by each protocol):
```go
type Backend interface {
    Protocol()     string  // "lastfm-compat" or "listenbrainz-compat"
    Verify(ctx context.Context, cfg ServiceConfig) error
    NowPlaying(ctx context.Context, cfg ServiceConfig, track TrackInfo) error
    Scrobble(ctx context.Context, cfg ServiceConfig, s ScrobbleInfo) error
    ScrobbleBatch(ctx context.Context, cfg ServiceConfig, ss []ScrobbleInfo) error
}
```

**`ScrobbleManager`** (per user, lazily instantiated):
- Holds all `enabled = true` AND `verified = true` service configs for the user
- `FanOut(ctx, userID, event ScrobbleEvent)` — calls all backends concurrently via goroutines
- Each backend call independent: one failure does not block others; errors logged and queued
- On backend error: append to `scrobble_queue` for that service; continue fan-out to remaining services
- `NowPlayingFanOut` — same pattern, errors logged only (not queued; now-playing is best-effort)
- Config cache: reload from DB when `updated_at` changes; checked on each fan-out call

**Scrobble rules (universal, applied before fan-out):**
- Scrobble threshold: ≥ 50% of track duration played, OR ≥ 4 minutes played (whichever comes first)
- Now-playing update: sent immediately when streaming starts (all enabled services)
- Duplicate guard: one scrobble per (user, song, timestamp window of 30s); dedup in manager before fan-out

**Trigger points (all route to `ScrobbleManager.FanOut`):**
- Subsonic `scrobble.view?submission=true` → scrobble
- Subsonic `scrobble.view?submission=false` → now-playing only
- Native API `POST /api/v1/songs/{id}/scrobbles` → scrobble
- Ampache `record_play` / `scrobble` actions → scrobble
- Internal play tracker (50% threshold, measured in stream handler) → scrobble

**Retry scheduler task** (`scrobble_retry`, every 30 minutes):
- For each service with queued entries: call `ScrobbleBatch` (up to 50 per call for Last.fm-compat; up to 1000 for ListenBrainz-compat)
- On success: delete queued entries
- On failure: increment `attempts`, update `last_error`
- Drop policy: delete after 14 days OR `attempts ≥ 50` (whichever first); log drop with song + service info

### [ ] Last.fm-compatible protocol backend
Read: IDEA.md

Scope: `src/server/service/scrobble/lastfm_compat.go`

Covers: **Last.fm**, **Libre.fm**, **GNU FM**, **Maloja** (via `/api/audioscrobbler/2.0/`), **custom Last.fm-compat** servers.

**Auth flow (Last.fm / Libre.fm):**
- Step 1: redirect user to `{base_url}?method=auth.getToken&api_key=...` (or `https://www.last.fm/api/auth/?api_key=...&cb=...`)
- Step 2: callback with `?token=` → call `auth.getSession` → receive `session.key`
- Store `session_key_enc` in `scrobble_services`

**Auth flow (GNU FM / Maloja / custom):**
- Username + password → `auth.getMobileSession` (POST, HTTPS only) → receive `session.key`
- Store `session_key_enc`; never store plaintext password after auth

**API calls (all POST to `base_url`, HMAC-MD5 signed):**
- `track.updateNowPlaying` — `artist`, `track`, `album`, `duration`, `mbid`
- `track.scrobble` — same fields + `timestamp` (Unix); batch up to 50 tracks per call using indexed params (`artist[0]`, `artist[1]`, …)
- All params sorted alphabetically before HMAC-MD5 signing; `api_sig` appended last

**`Verify`:** call `user.getInfo` with session key; success = `verified = true`

### [ ] ListenBrainz-compatible protocol backend
Read: IDEA.md

Scope: `src/server/service/scrobble/listenbrainz_compat.go`

Covers: **ListenBrainz** (`https://api.listenbrainz.org/1/`), **Maloja** (via `/api/listenbrainz/1/`), **custom ListenBrainz-compat** servers.

**Auth:** Bearer token in `Authorization: Token {token}` header; token stored as `token_enc`

**API calls (POST `{base_url}/submit-listens`, JSON body):**

Now-playing:
```json
{"listen_type": "playing_now", "payload": [{"track_metadata": {"artist_name": "...", "track_name": "...", "release_name": "...", "additional_info": {"duration_ms": N, "music_service": "cassonic", "recording_mbid": "..."}}}]}
```

Single scrobble (`listen_type: "single"`): same shape + `listened_at` (Unix timestamp) in payload root.

Batch (`listen_type: "import"`): payload array up to 1000 entries.

**`Verify`:** `GET {base_url}/validate-token` with `Authorization: Token {token}`; parse `{"code": 200, "message": "Token valid.", "user_name": "..."}` — store `username` on success

### [ ] Scrobble service API handlers
Read: IDEA.md

Scope: `src/server/handler/api/scrobble.go`

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/v1/users/{id}/scrobble-services` | List all configured scrobble services for user |
| POST | `/api/v1/users/{id}/scrobble-services` | Add a service (any type) |
| GET | `/api/v1/users/{id}/scrobble-services/{serviceId}` | Get service detail |
| PATCH | `/api/v1/users/{id}/scrobble-services/{serviceId}` | Update config or toggle `enabled` |
| DELETE | `/api/v1/users/{id}/scrobble-services/{serviceId}` | Remove service |
| POST | `/api/v1/users/{id}/scrobble-services/{serviceId}/verify` | Test connection; sets `verified` flag |
| GET | `/api/v1/users/{id}/scrobble-services/{serviceId}/auth` | Last.fm-compat: returns auth redirect URL |
| GET | `/api/v1/users/{id}/scrobble-services/{serviceId}/callback` | Last.fm-compat: OAuth callback, completes auth |
| GET | `/api/v1/users/{id}/scrobble-services/{serviceId}/queue` | View pending retry queue for this service |
| DELETE | `/api/v1/users/{id}/scrobble-services/{serviceId}/queue` | Clear retry queue for this service |

**`POST /api/v1/users/{id}/scrobble-services` request body:**
```json
{
  "service_type": "lastfm|listenbrainz|librefm|gnufm|maloja|custom_lastfm|custom_listenbrainz",
  "display_name": "My Last.fm",
  "base_url": "https://...",
  "api_key": "...",
  "api_secret": "...",
  "token": "...",
  "username": "...",
  "password": "...",
  "enabled": true
}
```
- `password` accepted on create only for GNU FM / Maloja / custom_lastfm auth; immediately used to call `auth.getMobileSession`, then discarded — never stored
- `api_secret` accepted on create/update; stored encrypted immediately; not returned in GET responses (masked as `"api_secret": "xxxxx"`)
- `token` accepted on create/update; stored encrypted; masked in responses

### [ ] WebUI — scrobbling settings
Read: IDEA.md

Scope: `src/server/template/pages/settings_scrobble.html` (section within `/settings`)

- "Scrobbling" section on settings page lists all configured services as cards
- Each card: service logo/icon, display name, type badge, status (connected ✓ / disconnected ✗ / not verified), enabled toggle, "Test connection" button, edit button, delete button
- "Add service" button opens a form:
  - Service type dropdown (Last.fm, ListenBrainz, Libre.fm, GNU FM, Maloja, Custom Last.fm-compat, Custom ListenBrainz-compat)
  - Fields shown/hidden based on type: base URL (self-hosted only), API key+secret (Last.fm-compat), token (ListenBrainz-compat), username+password (GNU FM/Maloja mobile session)
  - On save → auto-runs verify → shows result inline
- Last.fm / Libre.fm OAuth: "Connect" button redirects to provider auth page; callback completes setup
- Queue badge: shows count of queued scrobbles per service; "Clear queue" button
- No-JS: forms work without JS; toggle via checkbox form POST

---

## UPLOAD

### [ ] Upload service
Read: IDEA.md

Scope: `src/server/service/upload.go`

**Config:** `upload_enabled: true/false` (default false), `upload_library_id` (which library folder receives uploads), `upload_max_size_mb` (default 50), `upload_allowed_roles` (default: admin; can add `user`)

**Upload flow:**
1. Validate file extension against allowed audio formats
2. Validate MIME type from file header (not just extension)
3. Validate file size against `upload_max_size_mb`
4. Write to temp file in `{data_dir}/uploads/tmp/`
5. Run tag reader on temp file — extract metadata
6. Move to `{upload_library_path}/{artist}/{album}/{filename}` (sanitized, no path traversal)
7. Insert song into DB with `library_id = upload_library_id`
8. Run cover art extraction on new file
9. Return song detail JSON

**Security:** all path components (artist, album, filename) sanitized: strip `..`, `/`, null bytes; max 255 chars per component; fall back to `Unknown Artist` / `Unknown Album` if tags empty

### [ ] Upload API handlers (native + Subsonic + Ampache)
Read: IDEA.md

Scope: `src/server/handler/api/upload.go`

**Native API:**

| Method | Route | Description |
|--------|-------|-------------|
| POST | `/api/v1/uploads` | Upload audio file (multipart/form-data, field `file`) |
| GET | `/api/v1/uploads/config` | Returns upload config (enabled, max size, allowed formats) |

**Subsonic:** `upload` is not part of the official Subsonic spec — expose via `/rest/upload.view` as a cassonic extension; return appropriate error if upload is disabled

**Ampache:** `upload` action on `/server/xml.server.php` and `/server/json.server.php` — back it with the same upload service; respects `upload_enabled` config

**Permissions:** user must have `upload` flag set in their account (per-user permission, admin-controlled)

### [ ] WebUI — upload page
Read: IDEA.md

Scope: `src/server/template/pages/upload.html`

- `GET /upload` — upload page (only rendered if `upload_enabled = true` and user has upload permission)
- Drag-and-drop zone + file picker (JS-enhanced); plain file input fallback
- Client-side format validation before submit (JS); server always re-validates
- Upload progress bar (JS `XMLHttpRequest` with progress event)
- After upload: show song detail card with tags and "Edit tags" link
- Multiple file upload: process sequentially, show per-file status

---

## SHARE LINKS

### [ ] Share links data model and service
Read: IDEA.md

Scope: `src/server/model/share.go`, `src/server/service/share.go`, `src/server/store/share_store.go`

**Share model:**
- `id`, `token` (URL-safe random 24-byte base64url, unique), `owner_id`, `item_type` (song/album/playlist), `item_id`, `description`, `expires_at` (nullable), `password_hash` (nullable, Argon2id), `view_count`, `last_viewed_at`, `created_at`, `updated_at`

**Share service:**
- `CreateShare(ownerID, itemType, itemID, opts)` — generate token, store
- `GetShareByToken(token)` — lookup; check expiry; increment `view_count`
- `ValidatePassword(share, password)` — constant-time Argon2id verify; return `ErrWrongPassword` on fail
- Token generation: `crypto/rand` 24 bytes → base64url (no padding); collision check against DB

**Public share URL:** `https://{host}/share/{token}`
- If password set: render password prompt page first; on correct password set short-lived cookie and redirect to share view
- Shared song: embeds player, shows metadata, download button (if `allow_download = true` on share)
- Shared album: shows tracklist, play-all button, per-track play
- Shared playlist: same as album view
- No auth required to view public share (by design — token is the credential)
- Tor .onion URL also generated and shown if Tor is active

### [ ] Share API handlers (native + Subsonic)
Read: IDEA.md

Scope: `src/server/handler/api/shares.go`, addition to `src/server/handler/subsonic/`

**Native API:**

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/v1/shares` | List own shares |
| POST | `/api/v1/shares` | Create share |
| GET | `/api/v1/shares/{id}` | Get share detail |
| PATCH | `/api/v1/shares/{id}` | Update (description, expiry, password) |
| DELETE | `/api/v1/shares/{id}` | Delete share |
| GET | `/share/{token}` | Public share view (WebUI, no auth) |
| GET | `/api/v1/share/{token}` | Public share metadata (no auth, returns item info) |

**Subsonic shares** (already stub-listed — now backed by share service):
- `getShares.view` — list all shares owned by authenticated user
- `createShare.view` — `?id=&description=&expires=` (epoch ms)
- `updateShare.view` — update description/expiry
- `deleteShare.view`

**Request body for `POST /api/v1/shares`:**
```json
{
  "item_type": "song|album|playlist",
  "item_id": "string",
  "description": "optional string",
  "expires_at": "optional ISO 8601",
  "password": "optional string",
  "allow_download": true
}
```

### [ ] WebUI — share pages
Read: IDEA.md

Scope: `src/server/template/pages/share_view.html`, `src/server/template/pages/share_password.html`

- `GET /share/{token}` — public share page (no login required)
  - Song share: cover art, title/artist/album, embedded HTML5 player, optional download link
  - Album/playlist share: cover art, tracklist, play-all, per-track play
  - Expired share: show clear "This share has expired" message
- `GET /share/{token}` (password-protected): render password prompt first; POST password → validate → set `share_session` cookie → redirect to same URL
- Share management integrated into `/settings` page: list own shares, create new share, delete, copy link button (JS) / display URL (no-JS)

---

## TESTING (PART 29)

### [ ] Unit tests — foundation packages
Read: AI.md PART 29

Scope: `src/config/*_test.go`, `src/paths/*_test.go`, `src/mode/*_test.go`, `src/common/i18n/*_test.go`
- `config.ParseBool()` — all 40+ variants, both true and false sides
- Path detection for all OS contexts (container/privileged/user)
- Mode detection table-driven tests
- i18n key coverage check

### [ ] Unit tests — tag reading and writing
Read: AI.md PART 29

Scope: `src/server/service/tags/*_test.go`
- Fixture files for each format in `tests/fixtures/audio/`
- Read tests: verify all fields parsed correctly per format
- Write tests: write tags, read back, verify round-trip
- `user_edited` flag tests: non-empty field not overwritten; empty field on user_edited song IS repopulated; scan updates user_edited=false songs normally
- Writable check tests
- Malformed tag handling (no panic on bad input)

### [ ] Unit tests — Subsonic API
Read: AI.md PART 29

Scope: `src/server/handler/subsonic/*_test.go`
- Table-driven tests for each endpoint
- Both XML and JSON response format assertions
- Auth: token, legacy plaintext, hex-encoded — all three verified
- Error code correctness (missing param, wrong creds, not found)

### [ ] Unit tests — Ampache API
Read: AI.md PART 29

Scope: `src/server/handler/ampache/*_test.go`
- Handshake (SHA256 + MD5 legacy), session TTL, goodbye
- Both v5 and v6 response shape verification
- Action dispatch for all implemented actions

### [ ] Unit tests — Icecast streaming
Read: AI.md PART 29

Scope: `src/server/service/icecast/*_test.go`
- Mount goroutine lifecycle (start, stop, reconnect)
- Track selection by scope (all/artist/genre) with shuffle
- Resume behavior (current track restarts, not previous)
- Credential masking in logs

### [ ] Integration tests
Read: AI.md PART 29

Scope: `tests/`
- `tests/subsonic_test.go` — spin up server, run through Subsonic API with real DB
- `tests/ampache_test.go` — spin up server, run through Ampache handshake + browse
- `tests/streaming_test.go` — transcode + stream with real ffmpeg binary
- `tests/tag_editor_test.go` — real audio files, write tags, verify on-disk changes
- `tests/podcast_test.go` — RSS fetch + episode download + stream
- `tests/upload_test.go` — upload audio file, verify in DB, verify path sanitization
- `tests/share_test.go` — create share, access public URL, password protection, expiry
- `tests/scrobble_test.go` — mock all five service endpoints; verify fan-out hits all enabled+verified services; one failure does not block others; queued entries retried; drop policy enforced
- `tests/musicbrainz_test.go` — mock MusicBrainz API, verify rate limiting, verify non-empty user_edited fields not overwritten, verify empty user_edited fields ARE repopulated
- Coverage gate: 60% minimum enforced in CI

### [ ] Unit tests — podcast service
Read: AI.md PART 29

Scope: `src/server/service/*_test.go`
- RSS parse: valid feed, malformed feed (no crash), Atom feed, feed with `<itunes:*>` tags
- GUID dedup: same GUID on re-fetch does not create duplicate episode
- Download queue: bounded concurrency, error handling, status transitions
- Conditional fetch: `ETag` / `Last-Modified` round-trip

### [ ] Unit tests — MusicBrainz service
Read: AI.md PART 29

Scope: `src/server/service/musicbrainz/*_test.go`
- Rate limiter: verify 1 req/s enforced (mock clock)
- Auto-lookup: per-field rule — `user_edited = true` + non-empty field → skipped; `user_edited = true` + empty field → repopulated (table-driven, both cases)
- Score threshold: candidates below 90 not applied
- Manual apply: sets `user_edited = true` on MusicBrainz fields

### [ ] Unit tests — scrobbling
Read: AI.md PART 29

Scope: `src/server/service/scrobble/*_test.go`
- Fan-out: all enabled+verified services called concurrently; disabled or unverified services skipped
- Fan-out isolation: one backend error does not cancel others (table-driven with injected mock failures)
- Scrobble threshold: 50% and 4-minute ceiling (mock clock)
- Now-playing: sent on stream start; failures do not queue (best-effort)
- Duplicate guard: second scrobble within 30s window dropped
- Last.fm-compat: HMAC-MD5 signature correctness, batch param indexing (`artist[0]`, `artist[1]`)
- ListenBrainz-compat: JSON payload shape, Bearer token header, batch `listen_type: import`
- Queue drain: batch sizes respected per protocol; success clears entries; failure increments attempts
- Drop policy: entries dropped at 14 days or 50 attempts (table-driven)
- Verify: `verified` flag set on success; `last_error` set on failure; `enabled` toggle respected

### [ ] Unit tests — upload service
Read: AI.md PART 29

Scope: `src/server/service/*_test.go`
- Path sanitization: `..` stripped, `/` normalized, null bytes rejected
- MIME type validation from file header (not extension)
- Size limit enforcement
- Fallback to `Unknown Artist`/`Unknown Album` when tags empty

### [ ] Unit tests — share service
Read: AI.md PART 29

Scope: `src/server/service/*_test.go`
- Token uniqueness: collision check
- Expiry: expired share returns `ErrExpired`
- Password: correct password passes; wrong password constant-time fail
- View count incremented on each access
