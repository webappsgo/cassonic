package admin

import (
	"context"
	"errors"
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

func (s *testMusicStore) CreateLibrary(ctx context.Context, l *model.Library) (int64, error) { return 0, nil }
func (s *testMusicStore) GetLibrary(ctx context.Context, id int64) (*model.Library, error)   { return nil, nil }
func (s *testMusicStore) ListLibraries(ctx context.Context) ([]*model.Library, error) {
	return s.listLibrariesResult, s.listLibrariesErr
}
func (s *testMusicStore) UpdateLibrary(ctx context.Context, l *model.Library) error { return nil }
func (s *testMusicStore) DeleteLibrary(ctx context.Context, id int64) error         { return nil }
func (s *testMusicStore) UpsertArtist(ctx context.Context, a *model.Artist) (int64, error) {
	return 0, nil
}
func (s *testMusicStore) GetArtist(ctx context.Context, id int64) (*model.Artist, error) { return nil, nil }
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
func (s *testMusicStore) GetAlbum(ctx context.Context, id int64) (*model.Album, error) { return nil, nil }
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
func (s *testMusicStore) MarkSongMissing(ctx context.Context, id int64) error { return nil }
func (s *testMusicStore) DeleteMissingSongs(ctx context.Context) error        { return nil }
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
func (s *testMusicStore) UpdateScanStatus(ctx context.Context, st *model.ScanStatus) error { return nil }
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
func (s *testUserStore) UpdateUser(ctx context.Context, u *model.User) error  { return nil }
func (s *testUserStore) DeleteUser(ctx context.Context, id int64) error      { return nil }
func (s *testUserStore) ListUsers(ctx context.Context) ([]*model.User, error) { return nil, nil }
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

// testDB builds a *store.DB with stub Music and User stores; other stores are unused by admin.go.
func testDB() *store.DB {
	return &store.DB{
		Music: &testMusicStore{},
		Users: &testUserStore{},
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
	return New(db, cfg, "test-version", nil)
}
