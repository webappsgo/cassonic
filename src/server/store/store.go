package store

import (
	"context"
	"time"

	"github.com/local/cassonic/src/server/model"
)

// UserStore manages user accounts, sessions, API tokens, and internet radio stations.
type UserStore interface {
	// CreateUser inserts a new user and returns the assigned ID.
	CreateUser(ctx context.Context, u *model.User) (int64, error)
	// GetUser fetches a user by primary key.
	GetUser(ctx context.Context, id int64) (*model.User, error)
	// GetUserByUsername fetches a user by their unique username.
	GetUserByUsername(ctx context.Context, username string) (*model.User, error)
	// GetUserByEmail fetches a user by their unique email address.
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	// UpdateUser writes all mutable fields back to the database.
	UpdateUser(ctx context.Context, u *model.User) error
	// DeleteUser permanently removes a user and all cascade-deleted rows.
	DeleteUser(ctx context.Context, id int64) error
	// ListUsers returns all users ordered by username.
	ListUsers(ctx context.Context) ([]*model.User, error)

	// IncrementLoginAttempts adds 1 to the failed login counter for the user.
	IncrementLoginAttempts(ctx context.Context, id int64) error
	// ResetLoginAttempts clears the failed login counter and any lock.
	ResetLoginAttempts(ctx context.Context, id int64) error
	// SetLockedUntil sets the account lockout expiry.
	SetLockedUntil(ctx context.Context, id int64, until time.Time) error
	// UpdateLastLogin stamps the current time as the most recent successful login.
	UpdateLastLogin(ctx context.Context, id int64) error

	// CreateAPIToken inserts a new API token record.
	CreateAPIToken(ctx context.Context, t *model.APIToken) error
	// GetAPITokenByHash returns the token matching the given SHA-256 hash, or nil
	// if no matching unexpired token exists.
	GetAPITokenByHash(ctx context.Context, hash string) (*model.APIToken, error)
	// ListAPITokens returns all tokens owned by the specified user.
	ListAPITokens(ctx context.Context, userID int64) ([]*model.APIToken, error)
	// DeleteAPIToken removes a single token by ID.
	DeleteAPIToken(ctx context.Context, id int64) error
	// UpdateAPITokenLastUsed stamps the current time on the token.
	UpdateAPITokenLastUsed(ctx context.Context, id int64) error

	// CreateSession persists a new opaque session token (stored as its hash).
	CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, clientName string) error
	// GetSessionByHash returns the session for the given token hash, or nil, nil
	// when no matching session exists.
	GetSessionByHash(ctx context.Context, tokenHash string) (*Session, error)
	// DeleteSession removes a single session by token hash.
	DeleteSession(ctx context.Context, tokenHash string) error
	// DeleteUserSessions removes all sessions belonging to a user.
	DeleteUserSessions(ctx context.Context, userID int64) error
	// PurgeExpiredSessions deletes all rows whose expires_at is in the past.
	PurgeExpiredSessions(ctx context.Context) error

	// GetSubsonicPassword returns the AES-256-GCM encrypted subsonic password for
	// the named user. Returns ("", false, nil) when no subsonic password has been set.
	GetSubsonicPassword(ctx context.Context, username string) (encrypted string, ok bool, err error)
	// SetSubsonicPassword stores an AES-256-GCM encrypted subsonic password for the named user.
	SetSubsonicPassword(ctx context.Context, username string, encrypted string) error

	// CreateRadioStation inserts a new internet radio station.
	CreateRadioStation(ctx context.Context, s *model.InternetRadioStation) (int64, error)
	// GetRadioStation fetches a radio station by primary key.
	GetRadioStation(ctx context.Context, id int64) (*model.InternetRadioStation, error)
	// ListRadioStations returns all radio stations ordered by name.
	ListRadioStations(ctx context.Context) ([]*model.InternetRadioStation, error)
	// UpdateRadioStation writes all mutable fields back to the database.
	UpdateRadioStation(ctx context.Context, s *model.InternetRadioStation) error
	// DeleteRadioStation permanently removes a radio station.
	DeleteRadioStation(ctx context.Context, id int64) error
}

// Session represents an active login session stored in the database.
type Session struct {
	TokenHash  string
	UserID     int64
	ClientName string
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// IsExpired returns true if the session has passed its expiry time.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// MusicStore manages the music library including artists, albums, songs, and scan state.
type MusicStore interface {
	// CreateLibrary inserts a new library root and returns the assigned ID.
	CreateLibrary(ctx context.Context, lib *model.Library) (int64, error)
	// GetLibrary fetches a library by primary key.
	GetLibrary(ctx context.Context, id int64) (*model.Library, error)
	// ListLibraries returns all library roots ordered by name.
	ListLibraries(ctx context.Context) ([]*model.Library, error)
	// UpdateLibrary writes all mutable fields back to the database.
	UpdateLibrary(ctx context.Context, lib *model.Library) error
	// DeleteLibrary permanently removes a library and all cascade-deleted rows.
	DeleteLibrary(ctx context.Context, id int64) error

	// UpsertArtist inserts or updates an artist matched by name (case-insensitive).
	// Returns the artist ID whether it was created or already existed.
	UpsertArtist(ctx context.Context, a *model.Artist) (int64, error)
	// GetArtist fetches an artist by primary key.
	GetArtist(ctx context.Context, id int64) (*model.Artist, error)
	// GetArtistByName fetches an artist by exact name (case-insensitive).
	GetArtistByName(ctx context.Context, name string) (*model.Artist, error)
	// ListArtists returns a paginated, optionally sorted list of all artists.
	ListArtists(ctx context.Context, opts ListOpts) ([]*model.Artist, error)
	// SearchArtists returns artists whose name contains query (case-insensitive).
	SearchArtists(ctx context.Context, query string, opts ListOpts) ([]*model.Artist, error)
	// DeleteArtistsWithNoSongs removes all artist rows that have no associated songs.
	DeleteArtistsWithNoSongs(ctx context.Context) error

	// UpsertAlbum inserts or updates an album matched by title+artist_id.
	// Returns the album ID whether it was created or already existed.
	UpsertAlbum(ctx context.Context, a *model.Album) (int64, error)
	// GetAlbum fetches an album by primary key.
	GetAlbum(ctx context.Context, id int64) (*model.Album, error)
	// ListAlbums returns a paginated, optionally sorted list of all albums.
	ListAlbums(ctx context.Context, opts ListOpts) ([]*model.Album, error)
	// ListAlbumsByArtist returns all albums for the given artist, ordered by year.
	ListAlbumsByArtist(ctx context.Context, artistID int64) ([]*model.Album, error)
	// GetNewestAlbums returns up to limit albums ordered by created_at DESC.
	GetNewestAlbums(ctx context.Context, limit int) ([]*model.Album, error)
	// GetRandomAlbums returns up to limit albums in random order.
	GetRandomAlbums(ctx context.Context, limit int) ([]*model.Album, error)
	// SearchAlbums returns albums whose title contains query (case-insensitive).
	SearchAlbums(ctx context.Context, query string, opts ListOpts) ([]*model.Album, error)
	// DeleteAlbumsWithNoSongs removes all album rows that have no associated songs.
	DeleteAlbumsWithNoSongs(ctx context.Context) error

	// UpsertSong inserts or updates a song matched by path.
	// Returns the song ID whether it was created or already existed.
	UpsertSong(ctx context.Context, s *model.Song) (int64, error)
	// GetSong fetches a song by primary key.
	GetSong(ctx context.Context, id int64) (*model.Song, error)
	// GetSongByPath fetches a song by its absolute filesystem path.
	GetSongByPath(ctx context.Context, path string) (*model.Song, error)
	// ListSongsByAlbum returns all songs for the given album, ordered by disc then track.
	ListSongsByAlbum(ctx context.Context, albumID int64) ([]*model.Song, error)
	// ListSongsByArtist returns all songs attributed to the given artist.
	ListSongsByArtist(ctx context.Context, artistID int64) ([]*model.Song, error)
	// ListSongsByGenre returns a paginated list of songs in the given genre.
	ListSongsByGenre(ctx context.Context, genre string, opts ListOpts) ([]*model.Song, error)
	// GetRandomSongs returns up to limit songs filtered by optional genre and year range.
	// Empty string values for genre, fromYear, toYear disable those filters.
	GetRandomSongs(ctx context.Context, limit int, genre, fromYear, toYear string) ([]*model.Song, error)
	// SearchSongs returns songs whose title contains query (case-insensitive).
	SearchSongs(ctx context.Context, query string, opts ListOpts) ([]*model.Song, error)
	// MarkSongMissing sets the missing flag on a song whose file no longer exists.
	MarkSongMissing(ctx context.Context, id int64) error
	// DeleteMissingSongs removes all songs whose missing flag is set.
	DeleteMissingSongs(ctx context.Context) error
	// IncrementPlayCount adds 1 to the play_count for a song and stamps last_played_at.
	IncrementPlayCount(ctx context.Context, id int64) error

	// ListGenres returns all genres with their song and album counts.
	ListGenres(ctx context.Context) ([]*model.Genre, error)

	// UpsertCoverArt inserts or updates a cover art record.
	UpsertCoverArt(ctx context.Context, ca *model.CoverArt) (int64, error)
	// GetCoverArt fetches a cover art record by primary key.
	GetCoverArt(ctx context.Context, id int64) (*model.CoverArt, error)

	// CreateScanStatus inserts a new scan status record.
	CreateScanStatus(ctx context.Context, s *model.ScanStatus) (int64, error)
	// UpdateScanStatus writes updated scan progress back to the database.
	UpdateScanStatus(ctx context.Context, s *model.ScanStatus) error
	// GetLastScanStatus returns the most recently started scan, or nil if none exist.
	GetLastScanStatus(ctx context.Context) (*model.ScanStatus, error)
}

// ListOpts carries pagination and sorting parameters for list queries.
type ListOpts struct {
	// Offset is the number of rows to skip before returning results.
	Offset int
	// Limit caps the number of rows returned; 0 uses the implementation default (500).
	Limit int
	// SortBy names the column to order results by; empty string uses the default order.
	SortBy string
	// Desc reverses the sort order when true.
	Desc bool
}

// ActivityStore manages per-user activity: stars, ratings, play history, bookmarks,
// and the cross-client play queue.
type ActivityStore interface {
	// Star marks an item as starred for a user.
	// itemType is one of "song", "album", or "artist".
	Star(ctx context.Context, userID int64, itemType string, itemID int64) error
	// Unstar removes a star from an item for a user.
	Unstar(ctx context.Context, userID int64, itemType string, itemID int64) error
	// GetStarred returns all starred songs, albums, and artists for a user.
	GetStarred(ctx context.Context, userID int64) (*StarredItems, error)
	// IsStarred reports whether the user has starred the given item.
	IsStarred(ctx context.Context, userID int64, itemType string, itemID int64) (bool, error)

	// SetRating creates or replaces the user's rating for an item.
	// rating must be in the range 1–5.
	SetRating(ctx context.Context, userID int64, itemType string, itemID int64, rating int) error
	// GetRating returns the user's rating for an item, or 0 if not rated.
	GetRating(ctx context.Context, userID int64, itemType string, itemID int64) (int, error)

	// RecordPlay appends a play event to the user's history.
	RecordPlay(ctx context.Context, h *model.PlayHistory) error
	// GetPlayHistory returns up to limit recent play events for a user, newest first.
	GetPlayHistory(ctx context.Context, userID int64, limit int) ([]*model.PlayHistory, error)

	// SetBookmark creates or replaces the playback bookmark for a user+item.
	SetBookmark(ctx context.Context, b *model.Bookmark) error
	// GetBookmarks returns all bookmarks for a user.
	GetBookmarks(ctx context.Context, userID int64) ([]*model.Bookmark, error)
	// DeleteBookmark removes the bookmark for a specific user+item combination.
	DeleteBookmark(ctx context.Context, userID int64, itemType string, itemID int64) error

	// SavePlayQueue replaces the user's play queue atomically.
	SavePlayQueue(ctx context.Context, pq *model.PlayQueue, entries []*model.PlayQueueEntry) error
	// GetPlayQueue returns the user's current play queue and its entries.
	// Returns nil, nil, nil when no queue exists.
	GetPlayQueue(ctx context.Context, userID int64) (*model.PlayQueue, []*model.PlayQueueEntry, error)
}

// StarredItems groups all starred items returned for a single user.
type StarredItems struct {
	Songs   []*model.Song
	Albums  []*model.Album
	Artists []*model.Artist
}

// PlaylistStore manages user-created playlists and their entries.
type PlaylistStore interface {
	// CreatePlaylist inserts a new playlist and returns the assigned ID.
	CreatePlaylist(ctx context.Context, p *model.Playlist) (int64, error)
	// GetPlaylist fetches a playlist by primary key.
	GetPlaylist(ctx context.Context, id int64) (*model.Playlist, error)
	// ListPlaylists returns all playlists visible to the given user:
	// playlists owned by the user plus all public playlists.
	ListPlaylists(ctx context.Context, userID int64) ([]*model.Playlist, error)
	// UpdatePlaylist writes all mutable fields back to the database.
	UpdatePlaylist(ctx context.Context, p *model.Playlist) error
	// DeletePlaylist permanently removes a playlist and all its entries.
	DeletePlaylist(ctx context.Context, id int64) error
	// GetPlaylistEntries returns the ordered entries for a playlist.
	GetPlaylistEntries(ctx context.Context, playlistID int64) ([]*model.PlaylistEntry, error)
	// SetPlaylistEntries atomically replaces all entries for a playlist with the
	// provided ordered slice of song IDs.
	SetPlaylistEntries(ctx context.Context, playlistID int64, songIDs []int64) error
	// AddToPlaylist appends the given song IDs after the last existing entry.
	AddToPlaylist(ctx context.Context, playlistID int64, songIDs []int64) error
	// RemoveFromPlaylist removes entries at the specified 0-indexed positions
	// and compacts the remaining positions.
	RemoveFromPlaylist(ctx context.Context, playlistID int64, indices []int) error
}

// IcecastStore manages Icecast server and mount point configuration.
type IcecastStore interface {
	// CreateServer inserts a new Icecast server record.
	CreateServer(ctx context.Context, s *model.IcecastServer) (int64, error)
	// GetServer fetches an Icecast server by primary key.
	GetServer(ctx context.Context, id int64) (*model.IcecastServer, error)
	// ListServers returns all Icecast server records ordered by name.
	ListServers(ctx context.Context) ([]*model.IcecastServer, error)
	// UpdateServer writes all mutable fields back to the database.
	UpdateServer(ctx context.Context, s *model.IcecastServer) error
	// DeleteServer permanently removes a server and all its mount points.
	DeleteServer(ctx context.Context, id int64) error

	// CreateMount inserts a new Icecast mount point.
	CreateMount(ctx context.Context, m *model.IcecastMount) (int64, error)
	// GetMount fetches a mount point by primary key.
	GetMount(ctx context.Context, id int64) (*model.IcecastMount, error)
	// ListMounts returns all mount points ordered by server_id, then mount_path.
	ListMounts(ctx context.Context) ([]*model.IcecastMount, error)
	// ListMountsByServer returns all mount points for a specific server.
	ListMountsByServer(ctx context.Context, serverID int64) ([]*model.IcecastMount, error)
	// UpdateMount writes all mutable configuration fields back to the database.
	UpdateMount(ctx context.Context, m *model.IcecastMount) error
	// UpdateMountStatus updates only the runtime status fields for a mount.
	UpdateMountStatus(ctx context.Context, id int64, status model.MountStatus, currentSong, lastErr string) error
	// DeleteMount permanently removes a mount point.
	DeleteMount(ctx context.Context, id int64) error
}

// ChatStore manages Subsonic-compatible chat messages.
type ChatStore interface {
	// AddMessage appends a chat message to the store.
	AddMessage(ctx context.Context, msg *model.ChatMessage) error
	// GetMessages returns all messages posted at or after since, ordered by created_at ASC.
	GetMessages(ctx context.Context, since time.Time) ([]*model.ChatMessage, error)
	// PurgeOldMessages deletes all messages created strictly before before.
	PurgeOldMessages(ctx context.Context, before time.Time) error
}

// DB is the aggregate store that holds all domain-specific store implementations.
type DB struct {
	Users     UserStore
	Music     MusicStore
	Activity  ActivityStore
	Playlists PlaylistStore
	Icecast   IcecastStore
	Chat      ChatStore
	Shares    ShareStore
	Podcasts  PodcastStore
	Scrobble  ScrobbleStore
}
