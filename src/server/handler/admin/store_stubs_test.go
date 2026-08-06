package admin

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/local/cassonic/src/config"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// errStore is a sentinel error used in admin handler store-failure tests.
var errStore = errors.New("store error")

// testMusicStore is a configurable stub implementing store.MusicStore for admin handler tests.
// Only ListLibraries is exercised by admin.go; all other methods return zero values.
type testMusicStore struct {
	listLibrariesResult []*model.Library
	listLibrariesErr    error
}

func (s *testMusicStore) CreateLibrary(ctx context.Context, l *model.Library) (int64, error) {
	return 0, nil
}
func (s *testMusicStore) GetLibrary(ctx context.Context, id int64) (*model.Library, error) {
	return nil, nil
}
func (s *testMusicStore) ListLibraries(ctx context.Context) ([]*model.Library, error) {
	return s.listLibrariesResult, s.listLibrariesErr
}
func (s *testMusicStore) UpdateLibrary(ctx context.Context, l *model.Library) error { return nil }
func (s *testMusicStore) DeleteLibrary(ctx context.Context, id int64) error         { return nil }
func (s *testMusicStore) UpsertArtist(ctx context.Context, a *model.Artist) (int64, error) {
	return 0, nil
}
func (s *testMusicStore) GetArtist(ctx context.Context, id int64) (*model.Artist, error) {
	return nil, nil
}
func (s *testMusicStore) GetArtistByName(ctx context.Context, name string) (*model.Artist, error) {
	return nil, nil
}
func (s *testMusicStore) ListArtists(ctx context.Context, opts store.ListOpts) ([]*model.Artist, error) {
	return nil, nil
}
func (s *testMusicStore) SearchArtists(ctx context.Context, q string, opts store.ListOpts) ([]*model.Artist, error) {
	return nil, nil
}
func (s *testMusicStore) DeleteArtistsWithNoSongs(ctx context.Context) error { return nil }
func (s *testMusicStore) UpsertAlbum(ctx context.Context, a *model.Album) (int64, error) {
	return 0, nil
}
func (s *testMusicStore) GetAlbum(ctx context.Context, id int64) (*model.Album, error) {
	return nil, nil
}
func (s *testMusicStore) ListAlbums(ctx context.Context, opts store.ListOpts) ([]*model.Album, error) {
	return nil, nil
}
func (s *testMusicStore) ListAlbumsByArtist(ctx context.Context, artistID int64) ([]*model.Album, error) {
	return nil, nil
}
func (s *testMusicStore) GetNewestAlbums(ctx context.Context, limit int) ([]*model.Album, error) {
	return nil, nil
}
func (s *testMusicStore) GetRandomAlbums(ctx context.Context, limit int) ([]*model.Album, error) {
	return nil, nil
}
func (s *testMusicStore) SearchAlbums(ctx context.Context, q string, opts store.ListOpts) ([]*model.Album, error) {
	return nil, nil
}
func (s *testMusicStore) DeleteAlbumsWithNoSongs(ctx context.Context) error { return nil }
func (s *testMusicStore) UpsertSong(ctx context.Context, song *model.Song) (int64, error) {
	return 0, nil
}
func (s *testMusicStore) GetSong(ctx context.Context, id int64) (*model.Song, error) { return nil, nil }
func (s *testMusicStore) GetSongByPath(ctx context.Context, path string) (*model.Song, error) {
	return nil, nil
}
func (s *testMusicStore) ListSongsByAlbum(ctx context.Context, albumID int64) ([]*model.Song, error) {
	return nil, nil
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
func (s *testMusicStore) SearchSongs(ctx context.Context, q string, opts store.ListOpts) ([]*model.Song, error) {
	return nil, nil
}
func (s *testMusicStore) MarkSongMissing(ctx context.Context, id int64) error    { return nil }
func (s *testMusicStore) DeleteMissingSongs(ctx context.Context) error           { return nil }
func (s *testMusicStore) IncrementPlayCount(ctx context.Context, id int64) error { return nil }
func (s *testMusicStore) ListGenres(ctx context.Context) ([]*model.Genre, error) { return nil, nil }
func (s *testMusicStore) UpsertCoverArt(ctx context.Context, c *model.CoverArt) (int64, error) {
	return 0, nil
}
func (s *testMusicStore) GetCoverArt(ctx context.Context, id int64) (*model.CoverArt, error) {
	return nil, nil
}
func (s *testMusicStore) CreateScanStatus(ctx context.Context, st *model.ScanStatus) (int64, error) {
	return 0, nil
}
func (s *testMusicStore) UpdateScanStatus(ctx context.Context, st *model.ScanStatus) error {
	return nil
}
func (s *testMusicStore) GetLastScanStatus(ctx context.Context) (*model.ScanStatus, error) {
	return nil, nil
}

// testUserStore is a configurable stub implementing store.UserStore for admin handler tests.
// Only GetSessionByHash and GetUser are exercised by requireAdmin.
type testUserStore struct {
	getSessionByHashResult *store.Session
	getSessionByHashErr    error

	getUserResult *model.User
	getUserErr    error
}

func (s *testUserStore) CreateUser(ctx context.Context, u *model.User) (int64, error) { return 0, nil }
func (s *testUserStore) GetUser(ctx context.Context, id int64) (*model.User, error) {
	return s.getUserResult, s.getUserErr
}
func (s *testUserStore) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	return nil, nil
}
func (s *testUserStore) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return nil, nil
}
func (s *testUserStore) UpdateUser(ctx context.Context, u *model.User) error        { return nil }
func (s *testUserStore) DeleteUser(ctx context.Context, id int64) error             { return nil }
func (s *testUserStore) ListUsers(ctx context.Context) ([]*model.User, error)       { return nil, nil }
func (s *testUserStore) IncrementLoginAttempts(ctx context.Context, id int64) error { return nil }
func (s *testUserStore) ResetLoginAttempts(ctx context.Context, id int64) error     { return nil }
func (s *testUserStore) SetLockedUntil(ctx context.Context, id int64, until time.Time) error {
	return nil
}
func (s *testUserStore) UpdateLastLogin(ctx context.Context, id int64) error         { return nil }
func (s *testUserStore) CreateAPIToken(ctx context.Context, t *model.APIToken) error { return nil }
func (s *testUserStore) GetAPITokenByHash(ctx context.Context, hash string) (*model.APIToken, error) {
	return nil, nil
}
func (s *testUserStore) ListAPITokens(ctx context.Context, userID int64) ([]*model.APIToken, error) {
	return nil, nil
}
func (s *testUserStore) DeleteAPIToken(ctx context.Context, id int64) error         { return nil }
func (s *testUserStore) UpdateAPITokenLastUsed(ctx context.Context, id int64) error { return nil }
func (s *testUserStore) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, clientName string) error {
	return nil
}
func (s *testUserStore) GetSessionByHash(ctx context.Context, tokenHash string) (*store.Session, error) {
	return s.getSessionByHashResult, s.getSessionByHashErr
}
func (s *testUserStore) DeleteSession(ctx context.Context, tokenHash string) error  { return nil }
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

// testAdminStore is a configurable stub implementing store.AdminStore for
// admin handler tests. Only GetAdminSessionByHash and GetAdmin are
// exercised by requireAdmin.
type testAdminStore struct {
	getAdminSessionByHashResult *model.AdminSession
	getAdminSessionByHashErr    error

	getAdminResult *model.Admin
	getAdminErr    error

	updateAdminErr error

	getAdminPreferencesResult *model.AdminPreferences
	getAdminPreferencesErr    error
	updateAdminPreferencesErr error
	lastUpdatedPreferences    *model.AdminPreferences

	countAdminsResult int
	countAdminsErr    error

	createAdminResult int64
	createAdminErr    error
	lastCreatedAdmin  *model.Admin

	createAdminSessionErr error
	lastCreatedSession    *model.AdminSession

	getSetupTokenResult  *model.SetupToken
	getSetupTokenErr     error
	createSetupTokenErr  error
	consumeSetupTokenErr error
}

func (s *testAdminStore) CreateAdmin(ctx context.Context, a *model.Admin) (int64, error) {
	s.lastCreatedAdmin = a
	return s.createAdminResult, s.createAdminErr
}
func (s *testAdminStore) GetAdmin(ctx context.Context, id int64) (*model.Admin, error) {
	return s.getAdminResult, s.getAdminErr
}
func (s *testAdminStore) GetAdminByUsername(ctx context.Context, username string) (*model.Admin, error) {
	return nil, nil
}
func (s *testAdminStore) GetAdminByExternalID(ctx context.Context, source, externalID string) (*model.Admin, error) {
	return nil, nil
}
func (s *testAdminStore) UpdateAdmin(ctx context.Context, a *model.Admin) error {
	return s.updateAdminErr
}
func (s *testAdminStore) DeleteAdmin(ctx context.Context, id int64) error { return nil }
func (s *testAdminStore) ListAdmins(ctx context.Context) ([]*model.Admin, error) {
	return nil, nil
}
func (s *testAdminStore) CountAdmins(ctx context.Context) (int, error) {
	return s.countAdminsResult, s.countAdminsErr
}
func (s *testAdminStore) IncrementAdminLoginAttempts(ctx context.Context, id int64) error {
	return nil
}
func (s *testAdminStore) ResetAdminLoginAttempts(ctx context.Context, id int64) error { return nil }
func (s *testAdminStore) SetAdminLockedUntil(ctx context.Context, id int64, until time.Time) error {
	return nil
}
func (s *testAdminStore) UpdateAdminLastLogin(ctx context.Context, id int64) error { return nil }
func (s *testAdminStore) GetAdminPreferences(ctx context.Context, adminID int64) (*model.AdminPreferences, error) {
	if s.getAdminPreferencesResult != nil || s.getAdminPreferencesErr != nil {
		return s.getAdminPreferencesResult, s.getAdminPreferencesErr
	}
	return &model.AdminPreferences{AdminID: adminID, Theme: "auto", FontSize: "medium", DateFormat: "YYYY-MM-DD", TimeFormat: "24h", EmailSecurity: true}, nil
}
func (s *testAdminStore) UpdateAdminPreferences(ctx context.Context, p *model.AdminPreferences) error {
	s.lastUpdatedPreferences = p
	return s.updateAdminPreferencesErr
}
func (s *testAdminStore) CreateAdminSession(ctx context.Context, sess *model.AdminSession) error {
	s.lastCreatedSession = sess
	return s.createAdminSessionErr
}
func (s *testAdminStore) GetAdminSessionByHash(ctx context.Context, tokenHash string) (*model.AdminSession, error) {
	return s.getAdminSessionByHashResult, s.getAdminSessionByHashErr
}
func (s *testAdminStore) DeleteAdminSession(ctx context.Context, tokenHash string) error { return nil }
func (s *testAdminStore) DeleteAdminSessions(ctx context.Context, adminID int64) error   { return nil }
func (s *testAdminStore) PurgeExpiredAdminSessions(ctx context.Context) error            { return nil }
func (s *testAdminStore) AppendAuditEntry(ctx context.Context, e *model.AuditEntry) error {
	return nil
}
func (s *testAdminStore) ListAuditEntries(ctx context.Context, limit int) ([]*model.AuditEntry, error) {
	return nil, nil
}
func (s *testAdminStore) CreateSetupToken(ctx context.Context, tokenHash string) error {
	return s.createSetupTokenErr
}
func (s *testAdminStore) GetSetupToken(ctx context.Context) (*model.SetupToken, error) {
	return s.getSetupTokenResult, s.getSetupTokenErr
}
func (s *testAdminStore) ConsumeSetupToken(ctx context.Context) error {
	return s.consumeSetupTokenErr
}

// testDB builds a *store.DB with stub Music, User, and Admin stores; other stores are unused by admin.go.
func testDB() *store.DB {
	return &store.DB{
		Music: &testMusicStore{},
		Users: &testUserStore{},
		Admin: &testAdminStore{},
	}
}

// testConfig returns a config.Config with Paths pointed at a temp directory.
func testConfig(dir string) *config.Config {
	cfg := config.Defaults()
	cfg.Paths.Log = dir
	cfg.Paths.Data = dir
	return cfg
}

// newTestHandler builds an admin Handler with real templates, the given db and cfg, and no scheduler.
func newTestHandler(db *store.DB, cfg *config.Config) *Handler {
	return New(db, cfg, filepath.Join(cfg.Paths.Data, "server.yml"), "test-version", nil)
}
