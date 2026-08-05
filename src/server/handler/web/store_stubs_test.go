package web

import (
	"context"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// testMusicStore is a configurable stub implementing store.MusicStore for handler tests.
// Only the fields relevant to a given test are set; all other methods return zero values.
type testMusicStore struct {
	listLibrariesResult []*model.Library
	listLibrariesErr    error
	lastScanResult      *model.ScanStatus
	lastScanErr         error

	listArtistsResult    []*model.Artist
	listArtistsErr       error
	searchArtistsResult  []*model.Artist
	searchArtistsErr     error
	getArtistResult      *model.Artist
	getArtistErr         error
	albumsByArtistResult []*model.Album
	albumsByArtistErr    error

	listAlbumsResult   []*model.Album
	listAlbumsErr      error
	searchAlbumsResult []*model.Album
	searchAlbumsErr    error
	getAlbumResult     *model.Album
	getAlbumErr        error
	newestAlbumsResult []*model.Album
	newestAlbumsErr    error

	songsByAlbumResult []*model.Song
	songsByAlbumErr    error
	searchSongsResult  []*model.Song
	searchSongsErr     error
	getSongResult      *model.Song
	getSongErr         error

	listGenresResult []*model.Genre
	listGenresErr    error
}

func (s *testMusicStore) CreateLibrary(ctx context.Context, lib *model.Library) (int64, error) {
	return 0, nil
}
func (s *testMusicStore) GetLibrary(ctx context.Context, id int64) (*model.Library, error) {
	return nil, nil
}
func (s *testMusicStore) ListLibraries(ctx context.Context) ([]*model.Library, error) {
	return s.listLibrariesResult, s.listLibrariesErr
}
func (s *testMusicStore) UpdateLibrary(ctx context.Context, lib *model.Library) error { return nil }
func (s *testMusicStore) DeleteLibrary(ctx context.Context, id int64) error           { return nil }

func (s *testMusicStore) UpsertArtist(ctx context.Context, a *model.Artist) (int64, error) {
	return 0, nil
}
func (s *testMusicStore) GetArtist(ctx context.Context, id int64) (*model.Artist, error) {
	return s.getArtistResult, s.getArtistErr
}
func (s *testMusicStore) GetArtistByName(ctx context.Context, name string) (*model.Artist, error) {
	return nil, nil
}
func (s *testMusicStore) ListArtists(ctx context.Context, opts store.ListOpts) ([]*model.Artist, error) {
	return s.listArtistsResult, s.listArtistsErr
}
func (s *testMusicStore) SearchArtists(ctx context.Context, query string, opts store.ListOpts) ([]*model.Artist, error) {
	return s.searchArtistsResult, s.searchArtistsErr
}
func (s *testMusicStore) DeleteArtistsWithNoSongs(ctx context.Context) error { return nil }
func (s *testMusicStore) ListAlbumsByArtist(ctx context.Context, artistID int64) ([]*model.Album, error) {
	return s.albumsByArtistResult, s.albumsByArtistErr
}

func (s *testMusicStore) UpsertAlbum(ctx context.Context, a *model.Album) (int64, error) {
	return 0, nil
}
func (s *testMusicStore) GetAlbum(ctx context.Context, id int64) (*model.Album, error) {
	return s.getAlbumResult, s.getAlbumErr
}
func (s *testMusicStore) ListAlbums(ctx context.Context, opts store.ListOpts) ([]*model.Album, error) {
	return s.listAlbumsResult, s.listAlbumsErr
}
func (s *testMusicStore) GetNewestAlbums(ctx context.Context, limit int) ([]*model.Album, error) {
	return s.newestAlbumsResult, s.newestAlbumsErr
}
func (s *testMusicStore) GetRandomAlbums(ctx context.Context, limit int) ([]*model.Album, error) {
	return nil, nil
}
func (s *testMusicStore) SearchAlbums(ctx context.Context, query string, opts store.ListOpts) ([]*model.Album, error) {
	return s.searchAlbumsResult, s.searchAlbumsErr
}
func (s *testMusicStore) DeleteAlbumsWithNoSongs(ctx context.Context) error { return nil }

func (s *testMusicStore) UpsertSong(ctx context.Context, song *model.Song) (int64, error) {
	return 0, nil
}
func (s *testMusicStore) GetSong(ctx context.Context, id int64) (*model.Song, error) {
	return s.getSongResult, s.getSongErr
}
func (s *testMusicStore) GetSongByPath(ctx context.Context, path string) (*model.Song, error) {
	return nil, nil
}
func (s *testMusicStore) ListSongsByAlbum(ctx context.Context, albumID int64) ([]*model.Song, error) {
	return s.songsByAlbumResult, s.songsByAlbumErr
}
func (s *testMusicStore) ListSongsByArtist(ctx context.Context, artistID int64) ([]*model.Song, error) {
	return nil, nil
}
func (s *testMusicStore) ListSongsByGenre(ctx context.Context, genre string, opts store.ListOpts) ([]*model.Song, error) {
	return nil, nil
}
func (s *testMusicStore) GetRandomSongs(ctx context.Context, limit int, genre, fromYear, toYear string) ([]*model.Song, error) {
	return nil, nil
}
func (s *testMusicStore) SearchSongs(ctx context.Context, query string, opts store.ListOpts) ([]*model.Song, error) {
	return s.searchSongsResult, s.searchSongsErr
}
func (s *testMusicStore) MarkSongMissing(ctx context.Context, id int64) error    { return nil }
func (s *testMusicStore) DeleteMissingSongs(ctx context.Context) error           { return nil }
func (s *testMusicStore) IncrementPlayCount(ctx context.Context, id int64) error { return nil }

func (s *testMusicStore) ListGenres(ctx context.Context) ([]*model.Genre, error) {
	return s.listGenresResult, s.listGenresErr
}

func (s *testMusicStore) UpsertCoverArt(ctx context.Context, ca *model.CoverArt) (int64, error) {
	return 0, nil
}
func (s *testMusicStore) GetCoverArt(ctx context.Context, id int64) (*model.CoverArt, error) {
	return nil, nil
}

func (s *testMusicStore) CreateScanStatus(ctx context.Context, sc *model.ScanStatus) (int64, error) {
	return 0, nil
}
func (s *testMusicStore) UpdateScanStatus(ctx context.Context, sc *model.ScanStatus) error {
	return nil
}
func (s *testMusicStore) GetLastScanStatus(ctx context.Context) (*model.ScanStatus, error) {
	return s.lastScanResult, s.lastScanErr
}

// testUserStore is a configurable stub implementing store.UserStore for handler tests.
type testUserStore struct {
	getSessionResult *store.Session
	getSessionErr    error
	deleteSessionErr error

	getUserResult *model.User
	getUserErr    error

	getUserByUsernameResult *model.User
	getUserByUsernameErr    error

	listAPITokensResult []*model.APIToken
	listAPITokensErr    error

	incrementLoginAttemptsErr error
	createSessionErr          error
	resetLoginAttemptsErr     error
	updateLastLoginErr        error
}

func (s *testUserStore) CreateUser(ctx context.Context, u *model.User) (int64, error) {
	return 0, nil
}
func (s *testUserStore) GetUser(ctx context.Context, id int64) (*model.User, error) {
	return s.getUserResult, s.getUserErr
}
func (s *testUserStore) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	return s.getUserByUsernameResult, s.getUserByUsernameErr
}
func (s *testUserStore) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return nil, nil
}
func (s *testUserStore) UpdateUser(ctx context.Context, u *model.User) error { return nil }
func (s *testUserStore) DeleteUser(ctx context.Context, id int64) error      { return nil }
func (s *testUserStore) ListUsers(ctx context.Context) ([]*model.User, error) {
	return nil, nil
}

func (s *testUserStore) IncrementLoginAttempts(ctx context.Context, id int64) error {
	return s.incrementLoginAttemptsErr
}
func (s *testUserStore) ResetLoginAttempts(ctx context.Context, id int64) error {
	return s.resetLoginAttemptsErr
}
func (s *testUserStore) SetLockedUntil(ctx context.Context, id int64, until time.Time) error {
	return nil
}
func (s *testUserStore) UpdateLastLogin(ctx context.Context, id int64) error {
	return s.updateLastLoginErr
}

func (s *testUserStore) CreateAPIToken(ctx context.Context, t *model.APIToken) error { return nil }
func (s *testUserStore) GetAPITokenByHash(ctx context.Context, hash string) (*model.APIToken, error) {
	return nil, nil
}
func (s *testUserStore) ListAPITokens(ctx context.Context, userID int64) ([]*model.APIToken, error) {
	return s.listAPITokensResult, s.listAPITokensErr
}
func (s *testUserStore) DeleteAPIToken(ctx context.Context, id int64) error { return nil }
func (s *testUserStore) UpdateAPITokenLastUsed(ctx context.Context, id int64) error {
	return nil
}

func (s *testUserStore) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, clientName string) error {
	return s.createSessionErr
}
func (s *testUserStore) GetSessionByHash(ctx context.Context, tokenHash string) (*store.Session, error) {
	return s.getSessionResult, s.getSessionErr
}
func (s *testUserStore) DeleteSession(ctx context.Context, tokenHash string) error {
	return s.deleteSessionErr
}
func (s *testUserStore) DeleteUserSessions(ctx context.Context, userID int64) error { return nil }
func (s *testUserStore) PurgeExpiredSessions(ctx context.Context) error             { return nil }

func (s *testUserStore) GetSubsonicPassword(ctx context.Context, username string) (string, bool, error) {
	return "", false, nil
}
func (s *testUserStore) SetSubsonicPassword(ctx context.Context, username string, encrypted string) error {
	return nil
}

func (s *testUserStore) CreateRadioStation(ctx context.Context, st *model.InternetRadioStation) (int64, error) {
	return 0, nil
}
func (s *testUserStore) GetRadioStation(ctx context.Context, id int64) (*model.InternetRadioStation, error) {
	return nil, nil
}
func (s *testUserStore) ListRadioStations(ctx context.Context) ([]*model.InternetRadioStation, error) {
	return nil, nil
}
func (s *testUserStore) UpdateRadioStation(ctx context.Context, st *model.InternetRadioStation) error {
	return nil
}
func (s *testUserStore) DeleteRadioStation(ctx context.Context, id int64) error { return nil }

// testAdminStore is a configurable stub implementing store.AdminStore for handler tests.
// Only GetAdminByUsername and IncrementAdminLoginAttempts/SetAdminLockedUntil are
// exercised by web.LoginPost's tryAdminLogin path.
type testAdminStore struct {
	getAdminByUsernameResult *model.Admin
	getAdminByUsernameErr    error

	getAdminResult *model.Admin
	getAdminErr    error

	createAdminSessionErr error

	incrementAdminLoginAttemptsErr error
	resetAdminLoginAttemptsErr     error
	setAdminLockedUntilErr         error
	updateAdminLastLoginErr        error

	appendAuditEntryErr error

	deleteAdminSessionErr error
}

func (s *testAdminStore) CreateAdmin(ctx context.Context, a *model.Admin) (int64, error) {
	return 0, nil
}
func (s *testAdminStore) GetAdmin(ctx context.Context, id int64) (*model.Admin, error) {
	return s.getAdminResult, s.getAdminErr
}
func (s *testAdminStore) GetAdminByUsername(ctx context.Context, username string) (*model.Admin, error) {
	return s.getAdminByUsernameResult, s.getAdminByUsernameErr
}
func (s *testAdminStore) GetAdminByExternalID(ctx context.Context, source, externalID string) (*model.Admin, error) {
	return nil, nil
}
func (s *testAdminStore) UpdateAdmin(ctx context.Context, a *model.Admin) error { return nil }
func (s *testAdminStore) DeleteAdmin(ctx context.Context, id int64) error      { return nil }
func (s *testAdminStore) ListAdmins(ctx context.Context) ([]*model.Admin, error) {
	return nil, nil
}
func (s *testAdminStore) CountAdmins(ctx context.Context) (int, error) { return 0, nil }
func (s *testAdminStore) IncrementAdminLoginAttempts(ctx context.Context, id int64) error {
	return s.incrementAdminLoginAttemptsErr
}
func (s *testAdminStore) ResetAdminLoginAttempts(ctx context.Context, id int64) error {
	return s.resetAdminLoginAttemptsErr
}
func (s *testAdminStore) SetAdminLockedUntil(ctx context.Context, id int64, until time.Time) error {
	return s.setAdminLockedUntilErr
}
func (s *testAdminStore) UpdateAdminLastLogin(ctx context.Context, id int64) error {
	return s.updateAdminLastLoginErr
}
func (s *testAdminStore) GetAdminPreferences(ctx context.Context, adminID int64) (*model.AdminPreferences, error) {
	return nil, nil
}
func (s *testAdminStore) UpdateAdminPreferences(ctx context.Context, p *model.AdminPreferences) error {
	return nil
}
func (s *testAdminStore) CreateAdminSession(ctx context.Context, sess *model.AdminSession) error {
	return s.createAdminSessionErr
}
func (s *testAdminStore) GetAdminSessionByHash(ctx context.Context, tokenHash string) (*model.AdminSession, error) {
	return nil, nil
}
func (s *testAdminStore) DeleteAdminSession(ctx context.Context, tokenHash string) error {
	return s.deleteAdminSessionErr
}
func (s *testAdminStore) DeleteAdminSessions(ctx context.Context, adminID int64) error { return nil }
func (s *testAdminStore) PurgeExpiredAdminSessions(ctx context.Context) error          { return nil }
func (s *testAdminStore) AppendAuditEntry(ctx context.Context, e *model.AuditEntry) error {
	return s.appendAuditEntryErr
}
func (s *testAdminStore) ListAuditEntries(ctx context.Context, limit int) ([]*model.AuditEntry, error) {
	return nil, nil
}

// testPlaylistStore is a configurable stub implementing store.PlaylistStore for handler tests.
type testPlaylistStore struct {
	listPlaylistsResult []*model.Playlist
	listPlaylistsErr    error
	getPlaylistResult   *model.Playlist
	getPlaylistErr      error
	getEntriesResult    []*model.PlaylistEntry
	getEntriesErr       error
}

func (s *testPlaylistStore) CreatePlaylist(ctx context.Context, p *model.Playlist) (int64, error) {
	return 0, nil
}
func (s *testPlaylistStore) GetPlaylist(ctx context.Context, id int64) (*model.Playlist, error) {
	return s.getPlaylistResult, s.getPlaylistErr
}
func (s *testPlaylistStore) ListPlaylists(ctx context.Context, userID int64) ([]*model.Playlist, error) {
	return s.listPlaylistsResult, s.listPlaylistsErr
}
func (s *testPlaylistStore) UpdatePlaylist(ctx context.Context, p *model.Playlist) error {
	return nil
}
func (s *testPlaylistStore) DeletePlaylist(ctx context.Context, id int64) error { return nil }
func (s *testPlaylistStore) GetPlaylistEntries(ctx context.Context, playlistID int64) ([]*model.PlaylistEntry, error) {
	return s.getEntriesResult, s.getEntriesErr
}
func (s *testPlaylistStore) SetPlaylistEntries(ctx context.Context, playlistID int64, songIDs []int64) error {
	return nil
}
func (s *testPlaylistStore) AddToPlaylist(ctx context.Context, playlistID int64, songIDs []int64) error {
	return nil
}
func (s *testPlaylistStore) RemoveFromPlaylist(ctx context.Context, playlistID int64, indices []int) error {
	return nil
}

// testIcecastStore is a configurable stub implementing store.IcecastStore for handler tests.
type testIcecastStore struct {
	listServersResult []*model.IcecastServer
	listServersErr    error
	mountsByServer    map[int64][]*model.IcecastMount
	mountsByServerErr error
}

func (s *testIcecastStore) CreateServer(ctx context.Context, sv *model.IcecastServer) (int64, error) {
	return 0, nil
}
func (s *testIcecastStore) GetServer(ctx context.Context, id int64) (*model.IcecastServer, error) {
	return nil, nil
}
func (s *testIcecastStore) ListServers(ctx context.Context) ([]*model.IcecastServer, error) {
	return s.listServersResult, s.listServersErr
}
func (s *testIcecastStore) UpdateServer(ctx context.Context, sv *model.IcecastServer) error {
	return nil
}
func (s *testIcecastStore) DeleteServer(ctx context.Context, id int64) error { return nil }

func (s *testIcecastStore) CreateMount(ctx context.Context, m *model.IcecastMount) (int64, error) {
	return 0, nil
}
func (s *testIcecastStore) GetMount(ctx context.Context, id int64) (*model.IcecastMount, error) {
	return nil, nil
}
func (s *testIcecastStore) ListMounts(ctx context.Context) ([]*model.IcecastMount, error) {
	return nil, nil
}
func (s *testIcecastStore) ListMountsByServer(ctx context.Context, serverID int64) ([]*model.IcecastMount, error) {
	if s.mountsByServerErr != nil {
		return nil, s.mountsByServerErr
	}
	return s.mountsByServer[serverID], nil
}
func (s *testIcecastStore) UpdateMount(ctx context.Context, m *model.IcecastMount) error {
	return nil
}
func (s *testIcecastStore) UpdateMountStatus(ctx context.Context, id int64, status model.MountStatus, currentSong, lastErr string) error {
	return nil
}
func (s *testIcecastStore) DeleteMount(ctx context.Context, id int64) error { return nil }

// testShareStore is a configurable stub implementing store.ShareStore for handler tests.
type testShareStore struct {
	getShareByTokenResult *model.Share
	getShareByTokenErr    error
	incrementViewCountErr error
}

func (s *testShareStore) CreateShare(ctx context.Context, sh *model.Share) (int64, error) {
	return 0, nil
}
func (s *testShareStore) GetShare(ctx context.Context, id int64) (*model.Share, error) {
	return nil, nil
}
func (s *testShareStore) GetShareByToken(ctx context.Context, token string) (*model.Share, error) {
	return s.getShareByTokenResult, s.getShareByTokenErr
}
func (s *testShareStore) ListSharesByUser(ctx context.Context, userID int64) ([]*model.Share, error) {
	return nil, nil
}
func (s *testShareStore) UpdateShare(ctx context.Context, sh *model.Share) error { return nil }
func (s *testShareStore) DeleteShare(ctx context.Context, id int64) error        { return nil }
func (s *testShareStore) IncrementViewCount(ctx context.Context, id int64) error {
	return s.incrementViewCountErr
}

// testActivityStore is a configurable stub implementing store.ActivityStore for handler tests.
type testActivityStore struct {
	getPlayHistoryResult []*model.PlayHistory
	getPlayHistoryErr    error
}

func (s *testActivityStore) Star(ctx context.Context, userID int64, itemType string, itemID int64) error {
	return nil
}
func (s *testActivityStore) Unstar(ctx context.Context, userID int64, itemType string, itemID int64) error {
	return nil
}
func (s *testActivityStore) GetStarred(ctx context.Context, userID int64) (*store.StarredItems, error) {
	return nil, nil
}
func (s *testActivityStore) IsStarred(ctx context.Context, userID int64, itemType string, itemID int64) (bool, error) {
	return false, nil
}
func (s *testActivityStore) SetRating(ctx context.Context, userID int64, itemType string, itemID int64, rating int) error {
	return nil
}
func (s *testActivityStore) GetRating(ctx context.Context, userID int64, itemType string, itemID int64) (int, error) {
	return 0, nil
}
func (s *testActivityStore) RecordPlay(ctx context.Context, h *model.PlayHistory) error {
	return nil
}
func (s *testActivityStore) GetPlayHistory(ctx context.Context, userID int64, limit int) ([]*model.PlayHistory, error) {
	return s.getPlayHistoryResult, s.getPlayHistoryErr
}
func (s *testActivityStore) SetBookmark(ctx context.Context, b *model.Bookmark) error { return nil }
func (s *testActivityStore) GetBookmarks(ctx context.Context, userID int64) ([]*model.Bookmark, error) {
	return nil, nil
}
func (s *testActivityStore) DeleteBookmark(ctx context.Context, userID int64, itemType string, itemID int64) error {
	return nil
}
func (s *testActivityStore) SavePlayQueue(ctx context.Context, pq *model.PlayQueue, entries []*model.PlayQueueEntry) error {
	return nil
}
func (s *testActivityStore) GetPlayQueue(ctx context.Context, userID int64) (*model.PlayQueue, []*model.PlayQueueEntry, error) {
	return nil, nil, nil
}

// testDB builds a *store.DB with all stubs defaulted to zero values, allowing callers
// to override only the specific store needed for a given test.
func testDB() *store.DB {
	return &store.DB{
		Music:     &testMusicStore{},
		Users:     &testUserStore{},
		Admin:     &testAdminStore{},
		Activity:  &testActivityStore{},
		Playlists: &testPlaylistStore{},
		Icecast:   &testIcecastStore{},
		Shares:    &testShareStore{},
	}
}

// newTestHandler builds a web Handler with real templates and the given db.
func newTestHandler(db *store.DB) *Handler {
	return NewHandler(db)
}
