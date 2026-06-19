package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// configMusicStore wraps stubMusicStore with configurable return values for handler tests.
// Embedding *stubMusicStore provides default error stubs for methods not explicitly overridden.
type configMusicStore struct {
	*stubMusicStore

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

	songsByAlbumResult  []*model.Song
	songsByAlbumErr     error
	songsByArtistResult []*model.Song
	songsByArtistErr    error
	songsByGenreResult  []*model.Song
	songsByGenreErr     error
	searchSongsResult   []*model.Song
	searchSongsErr      error
	getSongResult       *model.Song
	getSongErr          error

	listGenresResult []*model.Genre
	listGenresErr    error

	listLibsResult  []*model.Library
	listLibsErr     error
	createLibResult int64
	createLibErr    error
	getLibResult    *model.Library
	getLibErr       error
	updateLibErr    error
	deleteLibErr    error
	createScanID    int64
	createScanErr   error
}

func (s *configMusicStore) ListArtists(ctx context.Context, opts store.ListOpts) ([]*model.Artist, error) {
	return s.listArtistsResult, s.listArtistsErr
}

func (s *configMusicStore) SearchArtists(ctx context.Context, query string, opts store.ListOpts) ([]*model.Artist, error) {
	return s.searchArtistsResult, s.searchArtistsErr
}

func (s *configMusicStore) GetArtist(ctx context.Context, id int64) (*model.Artist, error) {
	return s.getArtistResult, s.getArtistErr
}

func (s *configMusicStore) ListAlbumsByArtist(ctx context.Context, artistID int64) ([]*model.Album, error) {
	return s.albumsByArtistResult, s.albumsByArtistErr
}

func (s *configMusicStore) ListAlbums(ctx context.Context, opts store.ListOpts) ([]*model.Album, error) {
	return s.listAlbumsResult, s.listAlbumsErr
}

func (s *configMusicStore) SearchAlbums(ctx context.Context, query string, opts store.ListOpts) ([]*model.Album, error) {
	return s.searchAlbumsResult, s.searchAlbumsErr
}

func (s *configMusicStore) GetAlbum(ctx context.Context, id int64) (*model.Album, error) {
	return s.getAlbumResult, s.getAlbumErr
}

func (s *configMusicStore) ListSongsByAlbum(ctx context.Context, albumID int64) ([]*model.Song, error) {
	return s.songsByAlbumResult, s.songsByAlbumErr
}

func (s *configMusicStore) ListSongsByArtist(ctx context.Context, artistID int64) ([]*model.Song, error) {
	return s.songsByArtistResult, s.songsByArtistErr
}

func (s *configMusicStore) ListSongsByGenre(ctx context.Context, genre string, opts store.ListOpts) ([]*model.Song, error) {
	return s.songsByGenreResult, s.songsByGenreErr
}

func (s *configMusicStore) SearchSongs(ctx context.Context, query string, opts store.ListOpts) ([]*model.Song, error) {
	return s.searchSongsResult, s.searchSongsErr
}

func (s *configMusicStore) GetSong(ctx context.Context, id int64) (*model.Song, error) {
	return s.getSongResult, s.getSongErr
}

func (s *configMusicStore) ListGenres(ctx context.Context) ([]*model.Genre, error) {
	return s.listGenresResult, s.listGenresErr
}

func (s *configMusicStore) ListLibraries(ctx context.Context) ([]*model.Library, error) {
	return s.listLibsResult, s.listLibsErr
}

func (s *configMusicStore) CreateLibrary(ctx context.Context, lib *model.Library) (int64, error) {
	return s.createLibResult, s.createLibErr
}

func (s *configMusicStore) GetLibrary(ctx context.Context, id int64) (*model.Library, error) {
	return s.getLibResult, s.getLibErr
}

func (s *configMusicStore) UpdateLibrary(ctx context.Context, lib *model.Library) error {
	return s.updateLibErr
}

func (s *configMusicStore) DeleteLibrary(ctx context.Context, id int64) error {
	return s.deleteLibErr
}

func (s *configMusicStore) CreateScanStatus(ctx context.Context, sc *model.ScanStatus) (int64, error) {
	return s.createScanID, s.createScanErr
}

// newConfigHandler returns a Handler backed by the given configMusicStore.
func newConfigHandler(ms *configMusicStore) *Handler {
	return newHealthHandler(&store.DB{
		Music: ms,
		Users: &stubUserStoreForHealth{},
	})
}

// withChiID injects a chi route URL parameter into the request context.
func withChiID(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
