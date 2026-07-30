package ampache

import (
	"context"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// stubMusicStore is a configurable implementation of store.MusicStore.
// All fields default to zero values (no error, empty/nil result) so a handler
// under test only needs to set the fields it actually reads.
type stubMusicStore struct {
	libraries    []*model.Library
	librariesErr error
	library      *model.Library
	libraryErr   error
	createLibID  int64
	createLibErr error
	updateLibErr error
	deleteLibErr error

	artist         *model.Artist
	artistErr      error
	artistByName   *model.Artist
	artists        []*model.Artist
	artistsErr     error
	searchArtists  []*model.Artist
	searchArtErr   error

	album         *model.Album
	albumErr      error
	albums        []*model.Album
	albumsErr     error
	albumsByArtist []*model.Album
	newestAlbums  []*model.Album
	randomAlbums  []*model.Album
	searchAlbums  []*model.Album
	searchAlbErr  error

	song           *model.Song
	songErr        error
	songByPath     *model.Song
	songsByAlbum   []*model.Song
	songsByAlbumErr error
	songsByArtist  []*model.Song
	songsByGenre   []*model.Song
	songsByGenreErr error
	randomSongs    []*model.Song
	randomSongsErr error
	searchSongs    []*model.Song
	searchSongsErr error

	genres    []*model.Genre
	genresErr error

	coverArt    *model.CoverArt
	coverArtErr error

	scanID        int64
	createScanErr error
	lastScan      *model.ScanStatus
	lastScanErr   error
}

func (s *stubMusicStore) CreateLibrary(ctx context.Context, lib *model.Library) (int64, error) {
	return s.createLibID, s.createLibErr
}
func (s *stubMusicStore) GetLibrary(ctx context.Context, id int64) (*model.Library, error) {
	return s.library, s.libraryErr
}
func (s *stubMusicStore) ListLibraries(ctx context.Context) ([]*model.Library, error) {
	return s.libraries, s.librariesErr
}
func (s *stubMusicStore) UpdateLibrary(ctx context.Context, lib *model.Library) error {
	return s.updateLibErr
}
func (s *stubMusicStore) DeleteLibrary(ctx context.Context, id int64) error {
	return s.deleteLibErr
}
func (s *stubMusicStore) UpsertArtist(ctx context.Context, a *model.Artist) (int64, error) {
	return 0, nil
}
func (s *stubMusicStore) GetArtist(ctx context.Context, id int64) (*model.Artist, error) {
	return s.artist, s.artistErr
}
func (s *stubMusicStore) GetArtistByName(ctx context.Context, name string) (*model.Artist, error) {
	return s.artistByName, nil
}
func (s *stubMusicStore) ListArtists(ctx context.Context, opts store.ListOpts) ([]*model.Artist, error) {
	return s.artists, s.artistsErr
}
func (s *stubMusicStore) SearchArtists(ctx context.Context, query string, opts store.ListOpts) ([]*model.Artist, error) {
	return s.searchArtists, s.searchArtErr
}
func (s *stubMusicStore) DeleteArtistsWithNoSongs(ctx context.Context) error { return nil }
func (s *stubMusicStore) UpsertAlbum(ctx context.Context, a *model.Album) (int64, error) {
	return 0, nil
}
func (s *stubMusicStore) GetAlbum(ctx context.Context, id int64) (*model.Album, error) {
	return s.album, s.albumErr
}
func (s *stubMusicStore) ListAlbums(ctx context.Context, opts store.ListOpts) ([]*model.Album, error) {
	return s.albums, s.albumsErr
}
func (s *stubMusicStore) ListAlbumsByArtist(ctx context.Context, artistID int64) ([]*model.Album, error) {
	return s.albumsByArtist, nil
}
func (s *stubMusicStore) GetNewestAlbums(ctx context.Context, limit int) ([]*model.Album, error) {
	return s.newestAlbums, nil
}
func (s *stubMusicStore) GetRandomAlbums(ctx context.Context, limit int) ([]*model.Album, error) {
	return s.randomAlbums, nil
}
func (s *stubMusicStore) SearchAlbums(ctx context.Context, query string, opts store.ListOpts) ([]*model.Album, error) {
	return s.searchAlbums, s.searchAlbErr
}
func (s *stubMusicStore) DeleteAlbumsWithNoSongs(ctx context.Context) error { return nil }
func (s *stubMusicStore) UpsertSong(ctx context.Context, song *model.Song) (int64, error) {
	return 0, nil
}
func (s *stubMusicStore) GetSong(ctx context.Context, id int64) (*model.Song, error) {
	return s.song, s.songErr
}
func (s *stubMusicStore) GetSongByPath(ctx context.Context, path string) (*model.Song, error) {
	return s.songByPath, nil
}
func (s *stubMusicStore) ListSongsByAlbum(ctx context.Context, albumID int64) ([]*model.Song, error) {
	return s.songsByAlbum, s.songsByAlbumErr
}
func (s *stubMusicStore) ListSongsByArtist(ctx context.Context, artistID int64) ([]*model.Song, error) {
	return s.songsByArtist, nil
}
func (s *stubMusicStore) ListSongsByGenre(ctx context.Context, genre string, opts store.ListOpts) ([]*model.Song, error) {
	return s.songsByGenre, s.songsByGenreErr
}
func (s *stubMusicStore) GetRandomSongs(ctx context.Context, limit int, genre, fromYear, toYear string) ([]*model.Song, error) {
	return s.randomSongs, s.randomSongsErr
}
func (s *stubMusicStore) SearchSongs(ctx context.Context, query string, opts store.ListOpts) ([]*model.Song, error) {
	return s.searchSongs, s.searchSongsErr
}
func (s *stubMusicStore) MarkSongMissing(ctx context.Context, id int64) error { return nil }
func (s *stubMusicStore) DeleteMissingSongs(ctx context.Context) error       { return nil }
func (s *stubMusicStore) IncrementPlayCount(ctx context.Context, id int64) error {
	return nil
}
func (s *stubMusicStore) ListGenres(ctx context.Context) ([]*model.Genre, error) {
	return s.genres, s.genresErr
}
func (s *stubMusicStore) UpsertCoverArt(ctx context.Context, ca *model.CoverArt) (int64, error) {
	return 0, nil
}
func (s *stubMusicStore) GetCoverArt(ctx context.Context, id int64) (*model.CoverArt, error) {
	return s.coverArt, s.coverArtErr
}
func (s *stubMusicStore) CreateScanStatus(ctx context.Context, sc *model.ScanStatus) (int64, error) {
	return s.scanID, s.createScanErr
}
func (s *stubMusicStore) UpdateScanStatus(ctx context.Context, sc *model.ScanStatus) error {
	return nil
}
func (s *stubMusicStore) GetLastScanStatus(ctx context.Context) (*model.ScanStatus, error) {
	return s.lastScan, s.lastScanErr
}

// stubUserStore is a configurable implementation of store.UserStore.
type stubUserStore struct {
	user            *model.User
	userErr         error
	userByUsername  *model.User
	userByUsernameErr error
	userByEmail     *model.User
	users           []*model.User
	usersErr        error
	createUserID    int64
	createUserErr   error
	updateUserErr   error
	deleteUserErr   error

	radioStation    *model.InternetRadioStation
	radioStationErr error
	radioStations   []*model.InternetRadioStation
	radioStationsErr error
	createRadioID   int64
	createRadioErr  error
	updateRadioErr  error
	deleteRadioErr  error

	subsonicPassword string
	subsonicOK       bool
	subsonicErr      error
}

func (s *stubUserStore) CreateUser(ctx context.Context, u *model.User) (int64, error) {
	return s.createUserID, s.createUserErr
}
func (s *stubUserStore) GetUser(ctx context.Context, id int64) (*model.User, error) {
	return s.user, s.userErr
}
func (s *stubUserStore) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	return s.userByUsername, s.userByUsernameErr
}
func (s *stubUserStore) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return s.userByEmail, nil
}
func (s *stubUserStore) UpdateUser(ctx context.Context, u *model.User) error {
	return s.updateUserErr
}
func (s *stubUserStore) DeleteUser(ctx context.Context, id int64) error {
	return s.deleteUserErr
}
func (s *stubUserStore) ListUsers(ctx context.Context) ([]*model.User, error) {
	return s.users, s.usersErr
}
func (s *stubUserStore) IncrementLoginAttempts(ctx context.Context, id int64) error { return nil }
func (s *stubUserStore) ResetLoginAttempts(ctx context.Context, id int64) error     { return nil }
func (s *stubUserStore) SetLockedUntil(ctx context.Context, id int64, until time.Time) error {
	return nil
}
func (s *stubUserStore) UpdateLastLogin(ctx context.Context, id int64) error { return nil }
func (s *stubUserStore) CreateAPIToken(ctx context.Context, t *model.APIToken) error {
	return nil
}
func (s *stubUserStore) GetAPITokenByHash(ctx context.Context, hash string) (*model.APIToken, error) {
	return nil, nil
}
func (s *stubUserStore) ListAPITokens(ctx context.Context, userID int64) ([]*model.APIToken, error) {
	return nil, nil
}
func (s *stubUserStore) DeleteAPIToken(ctx context.Context, id int64) error { return nil }
func (s *stubUserStore) UpdateAPITokenLastUsed(ctx context.Context, id int64) error {
	return nil
}
func (s *stubUserStore) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, clientName string) error {
	return nil
}
func (s *stubUserStore) GetSessionByHash(ctx context.Context, tokenHash string) (*store.Session, error) {
	return nil, nil
}
func (s *stubUserStore) DeleteSession(ctx context.Context, tokenHash string) error {
	return nil
}
func (s *stubUserStore) DeleteUserSessions(ctx context.Context, userID int64) error {
	return nil
}
func (s *stubUserStore) PurgeExpiredSessions(ctx context.Context) error { return nil }
func (s *stubUserStore) GetSubsonicPassword(ctx context.Context, username string) (string, bool, error) {
	return s.subsonicPassword, s.subsonicOK, s.subsonicErr
}
func (s *stubUserStore) SetSubsonicPassword(ctx context.Context, username string, encrypted string) error {
	return nil
}
func (s *stubUserStore) CreateRadioStation(ctx context.Context, st *model.InternetRadioStation) (int64, error) {
	return s.createRadioID, s.createRadioErr
}
func (s *stubUserStore) GetRadioStation(ctx context.Context, id int64) (*model.InternetRadioStation, error) {
	return s.radioStation, s.radioStationErr
}
func (s *stubUserStore) ListRadioStations(ctx context.Context) ([]*model.InternetRadioStation, error) {
	return s.radioStations, s.radioStationsErr
}
func (s *stubUserStore) UpdateRadioStation(ctx context.Context, st *model.InternetRadioStation) error {
	return s.updateRadioErr
}
func (s *stubUserStore) DeleteRadioStation(ctx context.Context, id int64) error {
	return s.deleteRadioErr
}

// stubActivityStore is a configurable implementation of store.ActivityStore.
type stubActivityStore struct {
	starErr    error
	unstarErr  error
	starred    *store.StarredItems
	starredErr error
	isStarred  bool
	isStarredErr error

	setRatingErr error
	rating       int
	ratingErr    error

	recordPlayErr error
	playHistory   []*model.PlayHistory
	playHistoryErr error

	setBookmarkErr error
	bookmarks      []*model.Bookmark
	bookmarksErr   error
	deleteBookmarkErr error
}

func (s *stubActivityStore) Star(ctx context.Context, userID int64, itemType string, itemID int64) error {
	return s.starErr
}
func (s *stubActivityStore) Unstar(ctx context.Context, userID int64, itemType string, itemID int64) error {
	return s.unstarErr
}
func (s *stubActivityStore) GetStarred(ctx context.Context, userID int64) (*store.StarredItems, error) {
	return s.starred, s.starredErr
}
func (s *stubActivityStore) IsStarred(ctx context.Context, userID int64, itemType string, itemID int64) (bool, error) {
	return s.isStarred, s.isStarredErr
}
func (s *stubActivityStore) SetRating(ctx context.Context, userID int64, itemType string, itemID int64, rating int) error {
	return s.setRatingErr
}
func (s *stubActivityStore) GetRating(ctx context.Context, userID int64, itemType string, itemID int64) (int, error) {
	return s.rating, s.ratingErr
}
func (s *stubActivityStore) RecordPlay(ctx context.Context, h *model.PlayHistory) error {
	return s.recordPlayErr
}
func (s *stubActivityStore) GetPlayHistory(ctx context.Context, userID int64, limit int) ([]*model.PlayHistory, error) {
	return s.playHistory, s.playHistoryErr
}
func (s *stubActivityStore) SetBookmark(ctx context.Context, b *model.Bookmark) error {
	return s.setBookmarkErr
}
func (s *stubActivityStore) GetBookmarks(ctx context.Context, userID int64) ([]*model.Bookmark, error) {
	return s.bookmarks, s.bookmarksErr
}
func (s *stubActivityStore) DeleteBookmark(ctx context.Context, userID int64, itemType string, itemID int64) error {
	return s.deleteBookmarkErr
}
func (s *stubActivityStore) SavePlayQueue(ctx context.Context, pq *model.PlayQueue, entries []*model.PlayQueueEntry) error {
	return nil
}
func (s *stubActivityStore) GetPlayQueue(ctx context.Context, userID int64) (*model.PlayQueue, []*model.PlayQueueEntry, error) {
	return nil, nil, nil
}

// stubPlaylistStore is a configurable implementation of store.PlaylistStore.
type stubPlaylistStore struct {
	createID    int64
	createErr   error
	playlist    *model.Playlist
	playlistErr error
	playlists   []*model.Playlist
	playlistsErr error
	updateErr   error
	deleteErr   error
	entries     []*model.PlaylistEntry
	entriesErr  error
	setEntriesErr error
	addErr      error
	removeErr   error
}

func (s *stubPlaylistStore) CreatePlaylist(ctx context.Context, p *model.Playlist) (int64, error) {
	return s.createID, s.createErr
}
func (s *stubPlaylistStore) GetPlaylist(ctx context.Context, id int64) (*model.Playlist, error) {
	return s.playlist, s.playlistErr
}
func (s *stubPlaylistStore) ListPlaylists(ctx context.Context, userID int64) ([]*model.Playlist, error) {
	return s.playlists, s.playlistsErr
}
func (s *stubPlaylistStore) UpdatePlaylist(ctx context.Context, p *model.Playlist) error {
	return s.updateErr
}
func (s *stubPlaylistStore) DeletePlaylist(ctx context.Context, id int64) error {
	return s.deleteErr
}
func (s *stubPlaylistStore) GetPlaylistEntries(ctx context.Context, playlistID int64) ([]*model.PlaylistEntry, error) {
	return s.entries, s.entriesErr
}
func (s *stubPlaylistStore) SetPlaylistEntries(ctx context.Context, playlistID int64, songIDs []int64) error {
	return s.setEntriesErr
}
func (s *stubPlaylistStore) AddToPlaylist(ctx context.Context, playlistID int64, songIDs []int64) error {
	return s.addErr
}
func (s *stubPlaylistStore) RemoveFromPlaylist(ctx context.Context, playlistID int64, indices []int) error {
	return s.removeErr
}

// stubShareStore is a configurable implementation of store.ShareStore.
type stubShareStore struct {
	createID     int64
	createErr    error
	share        *model.Share
	shareErr     error
	shareByToken *model.Share
	shares       []*model.Share
	sharesErr    error
	updateErr    error
	deleteErr    error
	incrementErr error
}

func (s *stubShareStore) CreateShare(ctx context.Context, sh *model.Share) (int64, error) {
	return s.createID, s.createErr
}
func (s *stubShareStore) GetShare(ctx context.Context, id int64) (*model.Share, error) {
	return s.share, s.shareErr
}
func (s *stubShareStore) GetShareByToken(ctx context.Context, token string) (*model.Share, error) {
	return s.shareByToken, nil
}
func (s *stubShareStore) ListSharesByUser(ctx context.Context, userID int64) ([]*model.Share, error) {
	return s.shares, s.sharesErr
}
func (s *stubShareStore) UpdateShare(ctx context.Context, sh *model.Share) error {
	return s.updateErr
}
func (s *stubShareStore) DeleteShare(ctx context.Context, id int64) error {
	return s.deleteErr
}
func (s *stubShareStore) IncrementViewCount(ctx context.Context, id int64) error {
	return s.incrementErr
}
