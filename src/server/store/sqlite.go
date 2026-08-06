package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	// Pure-Go SQLite driver; CGO_ENABLED=0 compatible.
	_ "modernc.org/sqlite"
)

// usersSchema contains all DDL statements executed against users.db on startup.
const usersSchema = `
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    is_admin INTEGER NOT NULL DEFAULT 0,
    is_enabled INTEGER NOT NULL DEFAULT 1,
    avatar_url TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT 'en',
    theme TEXT NOT NULL DEFAULT 'dark',
    max_bit_rate INTEGER NOT NULL DEFAULT 0,
    can_download INTEGER NOT NULL DEFAULT 1,
    can_upload INTEGER NOT NULL DEFAULT 0,
    can_share INTEGER NOT NULL DEFAULT 1,
    can_manage_users INTEGER NOT NULL DEFAULT 0,
    can_comment INTEGER NOT NULL DEFAULT 1,
    can_podcast INTEGER NOT NULL DEFAULT 1,
    totp_secret TEXT NOT NULL DEFAULT '',
    totp_enabled INTEGER NOT NULL DEFAULT 0,
    subsonic_password TEXT NOT NULL DEFAULT '',
    last_login_at DATETIME,
    login_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS api_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    last_used_at DATETIME,
    expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_name TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS internet_radio_stations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    stream_url TEXT NOT NULL,
    homepage_url TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Server Admins (admin WebUI access, AI.md PART 12 "admins" table). A
-- distinct account type from "users" above (which holds Subsonic/Ampache
-- accounts, including the unrelated users.is_admin privileged-user role) —
-- never conflated with it. The Primary Admin is the row with the lowest id,
-- not a stored flag.
CREATE TABLE IF NOT EXISTS admins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT 'admin',
    enabled INTEGER NOT NULL DEFAULT 1,
    api_token_hash TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'local',
    external_id TEXT NOT NULL DEFAULT '',
    groups TEXT NOT NULL DEFAULT '[]',
    last_sync DATETIME,
    last_login DATETIME,
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until DATETIME,
    -- totp_enabled is the "enable 2FA for this admin" preference captured by
    -- setup wizard step 4 (AI.md PART 17 "Security Settings"); a
    -- storage-only placeholder until TOTP enrollment/verification ships in
    -- a future phase.
    totp_enabled INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_admins_external_id ON admins(source, external_id) WHERE external_id != '';

-- Admin WebUI preferences (AI.md PART 12 "admin_preferences" table): one row
-- per admin, appearance/display/email-notification settings.
CREATE TABLE IF NOT EXISTS admin_preferences (
    admin_id INTEGER PRIMARY KEY REFERENCES admins(id) ON DELETE CASCADE,
    theme TEXT NOT NULL DEFAULT 'dark',
    font_size TEXT NOT NULL DEFAULT 'medium',
    reduce_motion INTEGER NOT NULL DEFAULT 0,
    date_format TEXT NOT NULL DEFAULT 'YYYY-MM-DD',
    time_format TEXT NOT NULL DEFAULT '24h',
    email_security INTEGER NOT NULL DEFAULT 1,
    email_server INTEGER NOT NULL DEFAULT 1,
    email_backups INTEGER NOT NULL DEFAULT 1,
    email_users INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

// serverSchema contains all DDL statements executed against server.db on startup.
const serverSchema = `
CREATE TABLE IF NOT EXISTS libraries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    enabled INTEGER NOT NULL DEFAULT 1,
    last_scan_at DATETIME,
    song_count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS artists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    sort_name TEXT NOT NULL DEFAULT '',
    album_count INTEGER NOT NULL DEFAULT 0,
    song_count INTEGER NOT NULL DEFAULT 0,
    cover_art_id INTEGER NOT NULL DEFAULT 0,
    biography TEXT NOT NULL DEFAULT '',
    musicbrainz_id TEXT NOT NULL DEFAULT '',
    user_edited INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_artists_name ON artists(name COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS albums (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    sort_title TEXT NOT NULL DEFAULT '',
    artist_id INTEGER NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    artist_name TEXT NOT NULL DEFAULT '',
    year INTEGER NOT NULL DEFAULT 0,
    genre TEXT NOT NULL DEFAULT '',
    song_count INTEGER NOT NULL DEFAULT 0,
    duration INTEGER NOT NULL DEFAULT 0,
    cover_art_id INTEGER NOT NULL DEFAULT 0,
    musicbrainz_id TEXT NOT NULL DEFAULT '',
    user_edited INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(title, artist_id)
);
CREATE INDEX IF NOT EXISTS idx_albums_artist_id ON albums(artist_id);
CREATE INDEX IF NOT EXISTS idx_albums_year ON albums(year);

CREATE TABLE IF NOT EXISTS songs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    path TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    sort_title TEXT NOT NULL DEFAULT '',
    artist_id INTEGER NOT NULL DEFAULT 0,
    artist_name TEXT NOT NULL DEFAULT '',
    album_artist_id INTEGER NOT NULL DEFAULT 0,
    album_artist_name TEXT NOT NULL DEFAULT '',
    album_id INTEGER NOT NULL DEFAULT 0,
    album_name TEXT NOT NULL DEFAULT '',
    track_number INTEGER NOT NULL DEFAULT 0,
    disc_number INTEGER NOT NULL DEFAULT 0,
    year INTEGER NOT NULL DEFAULT 0,
    genre TEXT NOT NULL DEFAULT '',
    duration INTEGER NOT NULL DEFAULT 0,
    bit_rate INTEGER NOT NULL DEFAULT 0,
    sample_rate INTEGER NOT NULL DEFAULT 0,
    channels INTEGER NOT NULL DEFAULT 0,
    file_size INTEGER NOT NULL DEFAULT 0,
    content_type TEXT NOT NULL DEFAULT '',
    file_format TEXT NOT NULL DEFAULT '',
    cover_art_id INTEGER NOT NULL DEFAULT 0,
    mb_track_id TEXT NOT NULL DEFAULT '',
    mb_album_id TEXT NOT NULL DEFAULT '',
    mb_album_artist_id TEXT NOT NULL DEFAULT '',
    mb_artist_id TEXT NOT NULL DEFAULT '',
    composer TEXT NOT NULL DEFAULT '',
    lyricist TEXT NOT NULL DEFAULT '',
    conductor TEXT NOT NULL DEFAULT '',
    comment TEXT NOT NULL DEFAULT '',
    lyrics TEXT NOT NULL DEFAULT '',
    bpm INTEGER NOT NULL DEFAULT 0,
    replay_gain_track REAL NOT NULL DEFAULT 0,
    replay_gain_album REAL NOT NULL DEFAULT 0,
    file_hash TEXT NOT NULL DEFAULT '',
    last_modified DATETIME,
    user_edited INTEGER NOT NULL DEFAULT 0,
    play_count INTEGER NOT NULL DEFAULT 0,
    last_played_at DATETIME,
    missing INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_songs_artist_id ON songs(artist_id);
CREATE INDEX IF NOT EXISTS idx_songs_album_id ON songs(album_id);
CREATE INDEX IF NOT EXISTS idx_songs_path ON songs(path);
CREATE INDEX IF NOT EXISTS idx_songs_genre ON songs(genre);
CREATE INDEX IF NOT EXISTS idx_songs_user_edited ON songs(user_edited);

CREATE TABLE IF NOT EXISTS cover_art (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    song_id INTEGER NOT NULL DEFAULT 0,
    album_id INTEGER NOT NULL DEFAULT 0,
    data BLOB,
    path TEXT NOT NULL DEFAULT '',
    mime_type TEXT NOT NULL DEFAULT '',
    width INTEGER NOT NULL DEFAULT 0,
    height INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(song_id, album_id)
);

CREATE TABLE IF NOT EXISTS scan_status (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at DATETIME NOT NULL,
    finished_at DATETIME,
    status TEXT NOT NULL DEFAULT 'running',
    scanned_files INTEGER NOT NULL DEFAULT 0,
    added_files INTEGER NOT NULL DEFAULT 0,
    updated_files INTEGER NOT NULL DEFAULT 0,
    deleted_files INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS playlists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    is_public INTEGER NOT NULL DEFAULT 0,
    song_count INTEGER NOT NULL DEFAULT 0,
    duration INTEGER NOT NULL DEFAULT 0,
    cover_art_id INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_playlists_user_id ON playlists(user_id);

CREATE TABLE IF NOT EXISTS playlist_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    song_id INTEGER NOT NULL,
    position INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_playlist_entries_playlist_id ON playlist_entries(playlist_id);

CREATE TABLE IF NOT EXISTS stars (
    user_id INTEGER NOT NULL,
    item_type TEXT NOT NULL,
    item_id INTEGER NOT NULL,
    starred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(user_id, item_type, item_id)
);
CREATE INDEX IF NOT EXISTS idx_stars_user_id ON stars(user_id, item_type, item_id);

CREATE TABLE IF NOT EXISTS ratings (
    user_id INTEGER NOT NULL,
    item_type TEXT NOT NULL,
    item_id INTEGER NOT NULL,
    rating INTEGER NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(user_id, item_type, item_id)
);

CREATE TABLE IF NOT EXISTS play_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    song_id INTEGER NOT NULL,
    played_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    listened_for INTEGER NOT NULL DEFAULT 0,
    client_name TEXT NOT NULL DEFAULT '',
    scrobbled INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_play_history_user_id ON play_history(user_id, played_at);

CREATE TABLE IF NOT EXISTS bookmarks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    item_type TEXT NOT NULL,
    item_id INTEGER NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    comment TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bookmarks_user_item ON bookmarks(user_id, item_type, item_id);

CREATE TABLE IF NOT EXISTS play_queues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    current INTEGER NOT NULL DEFAULT 0,
    position INTEGER NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    changed_by TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS play_queue_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    play_queue_id INTEGER NOT NULL REFERENCES play_queues(id) ON DELETE CASCADE,
    song_id INTEGER NOT NULL,
    position INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS icecast_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL DEFAULT 8000,
    protocol TEXT NOT NULL DEFAULT 'http',
    source_user TEXT NOT NULL DEFAULT 'source',
    source_pass TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS icecast_mounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL REFERENCES icecast_servers(id) ON DELETE CASCADE,
    mount_path TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    scope TEXT NOT NULL DEFAULT 'all',
    artist_id INTEGER NOT NULL DEFAULT 0,
    genre TEXT NOT NULL DEFAULT '',
    format TEXT NOT NULL DEFAULT 'mp3',
    bit_rate INTEGER NOT NULL DEFAULT 128,
    shuffle INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'disconnected',
    current_song TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_icecast_mounts_server_id ON icecast_mounts(server_id);

CREATE TABLE IF NOT EXISTS chat_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    username TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_chat_messages_created_at ON chat_messages(created_at);

-- Admin WebUI login sessions (AI.md PART 12 "admin_sessions" table), kept
-- entirely separate from the regular-user "sessions" table in users.db (own
-- cookie name, own table, own store). admin_id is a logical FK to admins.id
-- in users.db (cross-DB, not enforced).
CREATE TABLE IF NOT EXISTS admin_sessions (
    id TEXT PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    ip_address TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    last_active DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_admin_sessions_admin ON admin_sessions(admin_id);
CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires ON admin_sessions(expires_at);

-- Audit log: admin actions, config changes, security events (AI.md PART 12
-- "audit_log" table, surfaced at /server/{admin_path}/config/logs/audit).
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    level TEXT NOT NULL DEFAULT 'info',
    category TEXT NOT NULL,
    action TEXT NOT NULL,
    actor_type TEXT NOT NULL DEFAULT 'admin',
    actor_id TEXT NOT NULL DEFAULT '',
    actor_ip TEXT NOT NULL DEFAULT '',
    target_type TEXT NOT NULL DEFAULT '',
    target_id TEXT NOT NULL DEFAULT '',
    details TEXT NOT NULL DEFAULT '{}',
    success INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_category ON audit_log(category);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_log(actor_type, actor_id);

-- The one-time first-run setup token (AI.md PART 12 "setup_token" table /
-- PART 17 "Setup Token Rules"). Single row (id always 1); only its SHA-256
-- hash is ever stored. Created once when the server starts with zero admin
-- accounts; used flips to 1 when the setup wizard completes and is never
-- reset — a lost or already-used token requires resetting server.db.
CREATE TABLE IF NOT EXISTS setup_token (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    token_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    used INTEGER NOT NULL DEFAULT 0,
    used_at DATETIME
);
`

// applyPragmas sets WAL mode and foreign key enforcement on a database connection.
func applyPragmas(db *sql.DB) error {
	_, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`)
	return err
}

// openDB opens a SQLite database file, configures it, and applies DDL.
func openDB(path, schema string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	// SQLite with WAL is safe with a single writer connection.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(2)

	if err = applyPragmas(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("pragmas %s: %w", path, err)
	}

	if _, err = db.ExecContext(context.Background(), schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema %s: %w", path, err)
	}

	return db, nil
}

// Open creates dataDir if needed, opens users.db and server.db, applies WAL mode,
// foreign key enforcement, and the full schema, then returns a populated DB.
func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	usersDB, err := openDB(filepath.Join(dataDir, "users.db"), usersSchema)
	if err != nil {
		return nil, err
	}

	// Idempotent migration: add subsonic_password column to existing databases.
	_, _ = usersDB.ExecContext(context.Background(), "ALTER TABLE users ADD COLUMN subsonic_password TEXT NOT NULL DEFAULT ''")

	serverDB, err := openDB(filepath.Join(dataDir, "server.db"), serverSchema)
	if err != nil {
		usersDB.Close()
		return nil, err
	}

	userStore := &sqliteUserStore{db: usersDB}
	adminStore := &sqliteAdminStore{usersDB: usersDB, serverDB: serverDB}
	chatStore := &sqliteChatStore{db: usersDB}
	musicStore := &sqliteMusicStore{db: serverDB}
	activityStore := &sqliteActivityStore{db: serverDB}
	playlistStore := &sqlitePlaylistStore{db: serverDB}
	icecastStore := &sqliteIcecastStore{db: serverDB}

	shareStore, err := NewShareStore(serverDB)
	if err != nil {
		usersDB.Close()
		serverDB.Close()
		return nil, fmt.Errorf("share store: %w", err)
	}

	podcastStore, err := NewPodcastStore(serverDB)
	if err != nil {
		usersDB.Close()
		serverDB.Close()
		return nil, fmt.Errorf("podcast store: %w", err)
	}

	scrobbleStore, err := NewScrobbleStore(serverDB)
	if err != nil {
		usersDB.Close()
		serverDB.Close()
		return nil, fmt.Errorf("scrobble store: %w", err)
	}

	return &DB{
		Users:     userStore,
		Admin:     adminStore,
		Music:     musicStore,
		Activity:  activityStore,
		Playlists: playlistStore,
		Icecast:   icecastStore,
		Chat:      chatStore,
		Shares:    shareStore,
		Podcasts:  podcastStore,
		Scrobble:  scrobbleStore,
	}, nil
}

// Close shuts down both database connections held by db.
func Close(db *DB) error {
	var firstErr error

	// Retrieve the underlying sql.DB from each store implementation.
	if us, ok := db.Users.(*sqliteUserStore); ok {
		if err := us.db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if ms, ok := db.Music.(*sqliteMusicStore); ok {
		if err := ms.db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
