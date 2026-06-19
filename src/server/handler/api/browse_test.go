package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// newBrowseStore returns a configMusicStore with a non-nil embedded stub.
func newBrowseStore() *configMusicStore {
	return &configMusicStore{stubMusicStore: &stubMusicStore{}}
}

// --- ListArtists ---

func TestListArtistsDefault(t *testing.T) {
	ms := newBrowseStore()
	ms.listArtistsResult = []*model.Artist{{ID: 1, Name: "Artist A"}}
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/artists", nil)
	rec := httptest.NewRecorder()
	h.ListArtists(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListArtists default: got %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["ok"] != true {
		t.Errorf("ListArtists: ok field missing or false: %v", body)
	}
}

func TestListArtistsWithSearch(t *testing.T) {
	ms := newBrowseStore()
	ms.searchArtistsResult = []*model.Artist{{ID: 2, Name: "Found"}}
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/artists?search=Found", nil)
	rec := httptest.NewRecorder()
	h.ListArtists(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListArtists search: got %d, want 200", rec.Code)
	}
}

func TestListArtistsError(t *testing.T) {
	ms := newBrowseStore()
	ms.listArtistsErr = errors.New("db failure")
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/artists", nil)
	rec := httptest.NewRecorder()
	h.ListArtists(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ListArtists error: got %d, want 500", rec.Code)
	}
}

func TestListArtistsSearchError(t *testing.T) {
	ms := newBrowseStore()
	ms.searchArtistsErr = errors.New("search failure")
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/artists?search=x", nil)
	rec := httptest.NewRecorder()
	h.ListArtists(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ListArtists search error: got %d, want 500", rec.Code)
	}
}

// --- GetArtist ---

func TestGetArtistBadID(t *testing.T) {
	h := newConfigHandler(newBrowseStore())
	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/v1/artists/abc", nil), "id", "abc")
	rec := httptest.NewRecorder()
	h.GetArtist(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("GetArtist bad id: got %d, want 400", rec.Code)
	}
}

func TestGetArtistNotFound(t *testing.T) {
	ms := newBrowseStore()
	ms.getArtistResult = nil
	h := newConfigHandler(ms)

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/v1/artists/99", nil), "id", "99")
	rec := httptest.NewRecorder()
	h.GetArtist(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GetArtist not found: got %d, want 404", rec.Code)
	}
}

func TestGetArtistSuccess(t *testing.T) {
	ms := newBrowseStore()
	ms.getArtistResult = &model.Artist{ID: 1, Name: "Test Artist"}
	ms.albumsByArtistResult = []*model.Album{{ID: 1, Title: "Album A"}}
	h := newConfigHandler(ms)

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/v1/artists/1", nil), "id", "1")
	rec := httptest.NewRecorder()
	h.GetArtist(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GetArtist success: got %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["ok"] != true {
		t.Errorf("GetArtist: ok field missing or false: %v", body)
	}
}

// --- ListAlbums ---

func TestListAlbumsDefault(t *testing.T) {
	ms := newBrowseStore()
	ms.listAlbumsResult = []*model.Album{{ID: 1, Title: "Album A"}}
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/albums", nil)
	rec := httptest.NewRecorder()
	h.ListAlbums(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListAlbums default: got %d, want 200", rec.Code)
	}
}

func TestListAlbumsDefaultError(t *testing.T) {
	ms := newBrowseStore()
	ms.listAlbumsErr = errors.New("db failure")
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/albums", nil)
	rec := httptest.NewRecorder()
	h.ListAlbums(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ListAlbums default error: got %d, want 500", rec.Code)
	}
}

func TestListAlbumsWithSearch(t *testing.T) {
	ms := newBrowseStore()
	ms.searchAlbumsResult = []*model.Album{{ID: 2, Title: "Found Album"}}
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/albums?search=Found", nil)
	rec := httptest.NewRecorder()
	h.ListAlbums(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListAlbums search: got %d, want 200", rec.Code)
	}
}

func TestListAlbumsSearchError(t *testing.T) {
	ms := newBrowseStore()
	ms.searchAlbumsErr = errors.New("search failure")
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/albums?search=x", nil)
	rec := httptest.NewRecorder()
	h.ListAlbums(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ListAlbums search error: got %d, want 500", rec.Code)
	}
}

func TestListAlbumsByArtistID(t *testing.T) {
	ms := newBrowseStore()
	ms.albumsByArtistResult = []*model.Album{{ID: 3, Title: "Artist Album"}}
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/albums?artist_id=5", nil)
	rec := httptest.NewRecorder()
	h.ListAlbums(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListAlbums by artist_id: got %d, want 200", rec.Code)
	}
}

func TestListAlbumsByArtistIDError(t *testing.T) {
	ms := newBrowseStore()
	ms.albumsByArtistErr = errors.New("db failure")
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/albums?artist_id=5", nil)
	rec := httptest.NewRecorder()
	h.ListAlbums(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ListAlbums by artist_id error: got %d, want 500", rec.Code)
	}
}

func TestListAlbumsBadArtistID(t *testing.T) {
	h := newConfigHandler(newBrowseStore())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/albums?artist_id=notanumber", nil)
	rec := httptest.NewRecorder()
	h.ListAlbums(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("ListAlbums bad artist_id: got %d, want 400", rec.Code)
	}
}

// --- GetAlbum ---

func TestGetAlbumBadID(t *testing.T) {
	h := newConfigHandler(newBrowseStore())
	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/v1/albums/xyz", nil), "id", "xyz")
	rec := httptest.NewRecorder()
	h.GetAlbum(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("GetAlbum bad id: got %d, want 400", rec.Code)
	}
}

func TestGetAlbumNotFound(t *testing.T) {
	ms := newBrowseStore()
	ms.getAlbumResult = nil
	h := newConfigHandler(ms)

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/v1/albums/99", nil), "id", "99")
	rec := httptest.NewRecorder()
	h.GetAlbum(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GetAlbum not found: got %d, want 404", rec.Code)
	}
}

func TestGetAlbumSuccess(t *testing.T) {
	ms := newBrowseStore()
	ms.getAlbumResult = &model.Album{ID: 1, Title: "Test Album"}
	ms.songsByAlbumResult = []*model.Song{{ID: 1, Title: "Track 1"}}
	h := newConfigHandler(ms)

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/v1/albums/1", nil), "id", "1")
	rec := httptest.NewRecorder()
	h.GetAlbum(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GetAlbum success: got %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["ok"] != true {
		t.Errorf("GetAlbum: ok field missing or false: %v", body)
	}
}

// --- ListSongs ---

func TestListSongsDefault(t *testing.T) {
	ms := newBrowseStore()
	ms.searchSongsResult = []*model.Song{{ID: 1, Title: "Song A"}}
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	rec := httptest.NewRecorder()
	h.ListSongs(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListSongs default: got %d, want 200", rec.Code)
	}
}

func TestListSongsDefaultError(t *testing.T) {
	ms := newBrowseStore()
	ms.searchSongsErr = errors.New("db failure")
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	rec := httptest.NewRecorder()
	h.ListSongs(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ListSongs default error: got %d, want 500", rec.Code)
	}
}

func TestListSongsWithSearch(t *testing.T) {
	ms := newBrowseStore()
	ms.searchSongsResult = []*model.Song{{ID: 2, Title: "Matching Song"}}
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs?search=Matching", nil)
	rec := httptest.NewRecorder()
	h.ListSongs(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListSongs search: got %d, want 200", rec.Code)
	}
}

func TestListSongsSearchError(t *testing.T) {
	ms := newBrowseStore()
	ms.searchSongsErr = errors.New("search failure")
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs?search=x", nil)
	rec := httptest.NewRecorder()
	h.ListSongs(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ListSongs search error: got %d, want 500", rec.Code)
	}
}

func TestListSongsByAlbumID(t *testing.T) {
	ms := newBrowseStore()
	ms.songsByAlbumResult = []*model.Song{{ID: 3, Title: "Album Track"}}
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs?album_id=2", nil)
	rec := httptest.NewRecorder()
	h.ListSongs(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListSongs by album_id: got %d, want 200", rec.Code)
	}
}

func TestListSongsByAlbumIDError(t *testing.T) {
	ms := newBrowseStore()
	ms.songsByAlbumErr = errors.New("db failure")
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs?album_id=2", nil)
	rec := httptest.NewRecorder()
	h.ListSongs(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ListSongs by album_id error: got %d, want 500", rec.Code)
	}
}

func TestListSongsBadAlbumID(t *testing.T) {
	h := newConfigHandler(newBrowseStore())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs?album_id=notanumber", nil)
	rec := httptest.NewRecorder()
	h.ListSongs(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("ListSongs bad album_id: got %d, want 400", rec.Code)
	}
}

func TestListSongsByArtistID(t *testing.T) {
	ms := newBrowseStore()
	ms.songsByArtistResult = []*model.Song{{ID: 4, Title: "Artist Song"}}
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs?artist_id=3", nil)
	rec := httptest.NewRecorder()
	h.ListSongs(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListSongs by artist_id: got %d, want 200", rec.Code)
	}
}

func TestListSongsByArtistIDError(t *testing.T) {
	ms := newBrowseStore()
	ms.songsByArtistErr = errors.New("db failure")
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs?artist_id=3", nil)
	rec := httptest.NewRecorder()
	h.ListSongs(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ListSongs by artist_id error: got %d, want 500", rec.Code)
	}
}

func TestListSongsBadArtistID(t *testing.T) {
	h := newConfigHandler(newBrowseStore())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs?artist_id=bad", nil)
	rec := httptest.NewRecorder()
	h.ListSongs(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("ListSongs bad artist_id: got %d, want 400", rec.Code)
	}
}

func TestListSongsByGenre(t *testing.T) {
	ms := newBrowseStore()
	ms.songsByGenreResult = []*model.Song{{ID: 5, Title: "Rock Song"}}
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs?genre=Rock", nil)
	rec := httptest.NewRecorder()
	h.ListSongs(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListSongs by genre: got %d, want 200", rec.Code)
	}
}

func TestListSongsByGenreError(t *testing.T) {
	ms := newBrowseStore()
	ms.songsByGenreErr = errors.New("db failure")
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs?genre=Rock", nil)
	rec := httptest.NewRecorder()
	h.ListSongs(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ListSongs by genre error: got %d, want 500", rec.Code)
	}
}

// --- GetSong ---

func TestGetSongBadID(t *testing.T) {
	h := newConfigHandler(newBrowseStore())
	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/v1/songs/bad", nil), "id", "bad")
	rec := httptest.NewRecorder()
	h.GetSong(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("GetSong bad id: got %d, want 400", rec.Code)
	}
}

func TestGetSongNotFound(t *testing.T) {
	ms := newBrowseStore()
	ms.getSongResult = nil
	h := newConfigHandler(ms)

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/v1/songs/99", nil), "id", "99")
	rec := httptest.NewRecorder()
	h.GetSong(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GetSong not found: got %d, want 404", rec.Code)
	}
}

func TestGetSongSuccess(t *testing.T) {
	ms := newBrowseStore()
	ms.getSongResult = &model.Song{ID: 1, Title: "My Song"}
	h := newConfigHandler(ms)

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/v1/songs/1", nil), "id", "1")
	rec := httptest.NewRecorder()
	h.GetSong(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GetSong success: got %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["ok"] != true {
		t.Errorf("GetSong: ok field missing or false: %v", body)
	}
}

// --- ListGenres ---

func TestListGenresSuccess(t *testing.T) {
	ms := newBrowseStore()
	ms.listGenresResult = []*model.Genre{{ID: 1, Name: "Rock", SongCount: 5}}
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/genres", nil)
	rec := httptest.NewRecorder()
	h.ListGenres(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListGenres success: got %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["ok"] != true {
		t.Errorf("ListGenres: ok field missing or false: %v", body)
	}
}

func TestListGenresError(t *testing.T) {
	ms := newBrowseStore()
	ms.listGenresErr = errors.New("db failure")
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/genres", nil)
	rec := httptest.NewRecorder()
	h.ListGenres(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ListGenres error: got %d, want 500", rec.Code)
	}
}

// --- Search ---

func TestSearchMissingQ(t *testing.T) {
	h := newConfigHandler(newBrowseStore())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search", nil)
	rec := httptest.NewRecorder()
	h.Search(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Search missing q: got %d, want 400", rec.Code)
	}
}

func TestSearchSuccess(t *testing.T) {
	ms := newBrowseStore()
	ms.searchArtistsResult = []*model.Artist{{ID: 1, Name: "Artist"}}
	ms.searchAlbumsResult = []*model.Album{{ID: 1, Title: "Album"}}
	ms.searchSongsResult = []*model.Song{{ID: 1, Title: "Song"}}
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	rec := httptest.NewRecorder()
	h.Search(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Search success: got %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["ok"] != true {
		t.Errorf("Search: ok field missing or false: %v", body)
	}
}

func TestSearchWithErrors(t *testing.T) {
	ms := newBrowseStore()
	ms.searchArtistsErr = errors.New("artists failure")
	ms.searchAlbumsErr = errors.New("albums failure")
	ms.searchSongsErr = errors.New("songs failure")
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=fail", nil)
	rec := httptest.NewRecorder()
	h.Search(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Search with errors: got %d, want 200 (errors are tolerated)", rec.Code)
	}
}
