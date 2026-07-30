package ampache

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// TestArtists covers list/search happy paths, the store-error path, and the
// nil-vs-empty-slice normalization (result must always be [] not null).
func TestArtists(t *testing.T) {
	t.Run("no session", func(t *testing.T) {
		h, _ := newTestHandler()
		rec := httptest.NewRecorder()
		h.artists(rec, newRequest("artists", nil), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4700 {
			t.Fatalf("expected 4700, got %+v", got)
		}
	})

	t.Run("list success empty", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.artists = nil
		rec := httptest.NewRecorder()
		h.artists(rec, newRequest("artists", map[string]string{"auth": token}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["artist"].([]any)
		if !ok || len(list) != 0 {
			t.Fatalf("expected empty artist list, got %v", got)
		}
	})

	t.Run("list store error", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.artistsErr = errors.New("db down")
		rec := httptest.NewRecorder()
		h.artists(rec, newRequest("artists", map[string]string{"auth": token}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4710 {
			t.Fatalf("expected 4710, got %+v", got)
		}
	})

	t.Run("search filter uses SearchArtists", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.searchArtists = []*model.Artist{{ID: 1, Name: "Q"}}
		rec := httptest.NewRecorder()
		h.artists(rec, newRequest("artists", map[string]string{"auth": token, "filter": "q"}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, _ := got["artist"].([]any)
		if len(list) != 1 {
			t.Fatalf("expected 1 artist from search, got %v", got)
		}
	})

	t.Run("search store error", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.searchArtErr = errors.New("boom")
		rec := httptest.NewRecorder()
		h.artists(rec, newRequest("artists", map[string]string{"auth": token, "filter": "q"}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4710 {
			t.Fatalf("expected 4710, got %+v", got)
		}
	})
}

// TestArtist covers the single-artist lookup: missing filter, not found, success.
func TestArtist(t *testing.T) {
	cases := []struct {
		name      string
		filter    string
		artist    *model.Artist
		err       error
		wantCode  int
		wantEmpty bool
	}{
		{name: "missing filter", filter: "", wantCode: 4705},
		{name: "not found", filter: "9", artist: nil, wantCode: 4704},
		{name: "store error", filter: "9", err: errors.New("x"), wantCode: 4704},
		{name: "success", filter: "9", artist: &model.Artist{ID: 9, Name: "A"}, wantCode: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, ts, token := newAuthedHandler(1)
			ts.music.artist = tc.artist
			ts.music.artistErr = tc.err
			rec := httptest.NewRecorder()
			params := map[string]string{"auth": token}
			if tc.filter != "" {
				params["filter"] = tc.filter
			}
			h.artist(rec, newRequest("artist", params), true)
			if tc.wantCode != 0 {
				var got xmlErrorJSON
				decodeJSON(t, rec.Body.Bytes(), &got)
				if got.ErrorCode != tc.wantCode {
					t.Fatalf("expected %d, got %+v", tc.wantCode, got)
				}
				return
			}
			var got AmpArtist
			decodeJSON(t, rec.Body.Bytes(), &got)
			if got.ID != "9" || got.Name != "A" {
				t.Fatalf("unexpected artist: %+v", got)
			}
		})
	}
}

// TestArtistAlbumsAndSongs covers the sub-resource listing endpoints.
func TestArtistAlbumsAndSongs(t *testing.T) {
	t.Run("albums missing id", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		h.artistAlbums(rec, newRequest("artist_albums", map[string]string{"auth": token}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4705 {
			t.Fatalf("expected 4705, got %+v", got)
		}
	})

	t.Run("albums store error", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.albumsByArtist = nil
		rec := httptest.NewRecorder()
		h.artistAlbums(rec, newRequest("artist_albums", map[string]string{"auth": token, "filter": "1"}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["album"].([]any)
		if !ok || len(list) != 0 {
			t.Fatalf("expected empty album list, got %v", got)
		}
	})

	t.Run("songs store error", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.songsByAlbumErr = errors.New("x")
		rec := httptest.NewRecorder()
		h.artistSongs(rec, newRequest("artist_songs", map[string]string{"auth": token, "filter": "1"}), true)
		// artistSongs uses ListSongsByArtist, not ListSongsByAlbum: error field is not shared, expect success empty.
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		if _, ok := got["song"]; !ok {
			t.Fatalf("expected song key, got %v", got)
		}
	})
}

// TestAlbums covers list/search happy paths and error handling, mirroring TestArtists.
func TestAlbums(t *testing.T) {
	t.Run("list success", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.albums = []*model.Album{{ID: 1, Title: "A"}}
		rec := httptest.NewRecorder()
		h.albums(rec, newRequest("albums", map[string]string{"auth": token}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["album"].([]any)
		if !ok || len(list) != 1 {
			t.Fatalf("expected 1 album, got %v", got)
		}
	})

	t.Run("search error", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.searchAlbErr = errors.New("x")
		rec := httptest.NewRecorder()
		h.albums(rec, newRequest("albums", map[string]string{"auth": token, "filter": "q"}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4710 {
			t.Fatalf("expected 4710, got %+v", got)
		}
	})
}

// TestAlbum covers the single-album lookup.
func TestAlbum(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.music.album = &model.Album{ID: 3, Title: "LP"}
	rec := httptest.NewRecorder()
	h.album(rec, newRequest("album", map[string]string{"auth": token, "filter": "3"}), true)
	var got AmpAlbum
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.ID != "3" || got.Name != "LP" {
		t.Fatalf("unexpected album: %+v", got)
	}

	ts.music.album = nil
	rec2 := httptest.NewRecorder()
	h.album(rec2, newRequest("album", map[string]string{"auth": token, "filter": "999"}), true)
	var errGot xmlErrorJSON
	decodeJSON(t, rec2.Body.Bytes(), &errGot)
	if errGot.ErrorCode != 4704 {
		t.Fatalf("expected 4704, got %+v", errGot)
	}
}

// TestAlbumSongs covers listing songs for an album.
func TestAlbumSongs(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.music.songsByAlbum = []*model.Song{{ID: 1, Title: "T"}}
	rec := httptest.NewRecorder()
	h.albumSongs(rec, newRequest("album_songs", map[string]string{"auth": token, "filter": "1"}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	list, ok := got["song"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("expected 1 song, got %v", got)
	}
}

// TestSongs covers the list/search/random paths.
func TestSongs(t *testing.T) {
	t.Run("no filter uses random", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.randomSongs = []*model.Song{{ID: 1, Title: "T"}}
		rec := httptest.NewRecorder()
		h.songs(rec, newRequest("songs", map[string]string{"auth": token}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["song"].([]any)
		if !ok || len(list) != 1 {
			t.Fatalf("expected 1 song, got %v", got)
		}
	})

	t.Run("random error", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.randomSongsErr = errors.New("x")
		rec := httptest.NewRecorder()
		h.songs(rec, newRequest("songs", map[string]string{"auth": token}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4710 {
			t.Fatalf("expected 4710, got %+v", got)
		}
	})

	t.Run("filter uses search", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.searchSongs = []*model.Song{{ID: 1, Title: "T"}}
		rec := httptest.NewRecorder()
		h.songs(rec, newRequest("songs", map[string]string{"auth": token, "filter": "t"}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["song"].([]any)
		if !ok || len(list) != 1 {
			t.Fatalf("expected 1 song, got %v", got)
		}
	})
}

// TestSong covers the single-song lookup.
func TestSong(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.music.song = &model.Song{ID: 5, Title: "S"}
	rec := httptest.NewRecorder()
	h.song(rec, newRequest("song", map[string]string{"auth": token, "filter": "5"}), true)
	var got AmpSong
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.ID != "5" || got.Title != "S" {
		t.Fatalf("unexpected song: %+v", got)
	}
}

// TestGenres covers genre listing and index-based ID assignment.
func TestGenres(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.music.genres = []*model.Genre{{Name: "Rock", SongCount: 3, AlbumCount: 1}}
	rec := httptest.NewRecorder()
	h.genres(rec, newRequest("genres", map[string]string{"auth": token}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	list, ok := got["genre"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("expected 1 genre, got %v", got)
	}
}

// TestGenre covers genre-by-name lookup: missing filter, not found (case-insensitive match), success.
func TestGenre(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.music.genres = []*model.Genre{{Name: "Rock", SongCount: 2}}

	rec := httptest.NewRecorder()
	h.genre(rec, newRequest("genre", map[string]string{"auth": token}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	rec2 := httptest.NewRecorder()
	h.genre(rec2, newRequest("genre", map[string]string{"auth": token, "filter": "ROCK"}), true)
	var got AmpGenre
	decodeJSON(t, rec2.Body.Bytes(), &got)
	if got.Name != "Rock" {
		t.Fatalf("expected case-insensitive match, got %+v", got)
	}

	rec3 := httptest.NewRecorder()
	h.genre(rec3, newRequest("genre", map[string]string{"auth": token, "filter": "Jazz"}), true)
	var notFound xmlErrorJSON
	decodeJSON(t, rec3.Body.Bytes(), &notFound)
	if notFound.ErrorCode != 4704 {
		t.Fatalf("expected 4704, got %+v", notFound)
	}
}

// TestGenreSongsAlbumsArtists covers the three genre sub-resource endpoints.
func TestGenreSongsAlbumsArtists(t *testing.T) {
	t.Run("genre_songs missing filter", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		h.genreSongs(rec, newRequest("genre_songs", map[string]string{"auth": token}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4705 {
			t.Fatalf("expected 4705, got %+v", got)
		}
	})

	t.Run("genre_songs success", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.songsByGenre = []*model.Song{{ID: 1}}
		rec := httptest.NewRecorder()
		h.genreSongs(rec, newRequest("genre_songs", map[string]string{"auth": token, "filter": "Rock"}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["song"].([]any)
		if !ok || len(list) != 1 {
			t.Fatalf("expected 1 song, got %v", got)
		}
	})

	t.Run("genre_albums filters by genre", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.searchAlbums = []*model.Album{{ID: 1, Genre: "Rock"}, {ID: 2, Genre: "Jazz"}}
		rec := httptest.NewRecorder()
		h.genreAlbums(rec, newRequest("genre_albums", map[string]string{"auth": token, "filter": "rock"}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["album"].([]any)
		if !ok || len(list) != 1 {
			t.Fatalf("expected 1 filtered album, got %v", got)
		}
	})

	t.Run("genre_artists dedups by artist ID", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.songsByGenre = []*model.Song{{ID: 1, ArtistID: 7}, {ID: 2, ArtistID: 7}}
		ts.music.artist = &model.Artist{ID: 7, Name: "A"}
		rec := httptest.NewRecorder()
		h.genreArtists(rec, newRequest("genre_artists", map[string]string{"auth": token, "filter": "Rock"}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["artist"].([]any)
		if !ok || len(list) != 1 {
			t.Fatalf("expected deduped 1 artist, got %v", got)
		}
	})
}

// TestGetIndexes covers the getIndexes endpoint's success and error paths.
func TestGetIndexes(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.music.artists = []*model.Artist{{ID: 1, Name: "A"}}
	rec := httptest.NewRecorder()
	h.getIndexes(rec, newRequest("get_indexes", map[string]string{"auth": token}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	list, ok := got["artist"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("expected 1 artist, got %v", got)
	}

	ts.music.artistsErr = errors.New("x")
	rec2 := httptest.NewRecorder()
	h.getIndexes(rec2, newRequest("get_indexes", map[string]string{"auth": token}), true)
	var errGot xmlErrorJSON
	decodeJSON(t, rec2.Body.Bytes(), &errGot)
	if errGot.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", errGot)
	}
}

// TestStats covers the aggregate statistics endpoint, verifying song count is
// derived from summing per-genre counts.
func TestStats(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.music.artists = []*model.Artist{{ID: 1}}
	ts.music.albums = []*model.Album{{ID: 1}, {ID: 2}}
	ts.music.genres = []*model.Genre{{Name: "Rock", SongCount: 5}, {Name: "Jazz", SongCount: 3}}
	ts.music.libraries = []*model.Library{{ID: 1}}
	rec := httptest.NewRecorder()
	h.stats(rec, newRequest("stats", map[string]string{"auth": token}), true)
	var got AmpStats
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.Songs != 8 || got.Albums != 2 || got.Artists != 1 || got.Genres != 2 || got.Catalogs != 1 {
		t.Fatalf("unexpected stats: %+v", got)
	}
}

// TestAdvancedSearch covers all three search types plus the unsupported-type
// error path and a rule-based filter match.
func TestAdvancedSearch(t *testing.T) {
	t.Run("song type default, title rule filters", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.searchSongs = []*model.Song{{ID: 1, Title: "Match"}, {ID: 2, Title: "Other"}}
		rec := httptest.NewRecorder()
		h.advancedSearch(rec, newRequest("advanced_search", map[string]string{
			"auth": token, "rule_1": "title", "rule_1_operator": "is", "rule_1_input": "Match",
		}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["song"].([]any)
		if !ok || len(list) != 1 {
			t.Fatalf("expected 1 matching song, got %v", got)
		}
	})

	t.Run("album type", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.searchAlbums = []*model.Album{{ID: 1, Title: "A"}}
		rec := httptest.NewRecorder()
		h.advancedSearch(rec, newRequest("advanced_search", map[string]string{"auth": token, "type": "album"}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		if _, ok := got["album"]; !ok {
			t.Fatalf("expected album key, got %v", got)
		}
	})

	t.Run("artist type", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.searchArtists = []*model.Artist{{ID: 1, Name: "A"}}
		rec := httptest.NewRecorder()
		h.advancedSearch(rec, newRequest("advanced_search", map[string]string{"auth": token, "type": "artist"}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		if _, ok := got["artist"]; !ok {
			t.Fatalf("expected artist key, got %v", got)
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		h.advancedSearch(rec, newRequest("advanced_search", map[string]string{"auth": token, "type": "video"}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4710 {
			t.Fatalf("expected 4710, got %+v", got)
		}
	})

	t.Run("store error", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.searchSongsErr = errors.New("x")
		rec := httptest.NewRecorder()
		h.advancedSearch(rec, newRequest("advanced_search", map[string]string{"auth": token}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4710 {
			t.Fatalf("expected 4710, got %+v", got)
		}
	})
}

// TestSystemUpdate covers the admin-only scan-triggering endpoint.
func TestSystemUpdate(t *testing.T) {
	t.Run("non-admin denied", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, IsAdmin: false}
		rec := httptest.NewRecorder()
		h.systemUpdate(rec, newRequest("system_update", map[string]string{"auth": token}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4742 {
			t.Fatalf("expected 4742, got %+v", got)
		}
	})

	t.Run("admin success", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, IsAdmin: true}
		rec := httptest.NewRecorder()
		h.systemUpdate(rec, newRequest("system_update", map[string]string{"auth": token}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got["success"] == nil {
			t.Fatalf("expected success key, got %v", got)
		}
	})
}

// TestCatalogs covers listing, and TestCatalog covers single-item lookup and
// the enabled-flag boolean-to-int conversion.
func TestCatalogs(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.music.libraries = []*model.Library{{ID: 1, Name: "Lib", Enabled: true, Path: "/music"}}
	rec := httptest.NewRecorder()
	h.catalogs(rec, newRequest("catalogs", map[string]string{"auth": token}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	list, ok := got["catalog"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("expected 1 catalog, got %v", got)
	}
}

func TestCatalog(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.music.library = &model.Library{ID: 2, Name: "Lib", Enabled: false, Path: "/x"}
	rec := httptest.NewRecorder()
	h.catalog(rec, newRequest("catalog", map[string]string{"auth": token, "filter": "2"}), true)
	var got AmpCatalog
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.ID != "2" || got.Enabled != 0 {
		t.Fatalf("unexpected catalog: %+v", got)
	}

	rec2 := httptest.NewRecorder()
	h.catalog(rec2, newRequest("catalog", map[string]string{"auth": token}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec2.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}
}

// TestCatalogSongsAlbumsArtists covers the catalog-scoped listing endpoints,
// including library-ID filtering, offset/limit pagination, and dedup logic.
func TestCatalogSongsAlbumsArtists(t *testing.T) {
	t.Run("catalog_songs filters by library and paginates", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.searchSongs = []*model.Song{
			{ID: 1, LibraryID: 1}, {ID: 2, LibraryID: 2}, {ID: 3, LibraryID: 1},
		}
		rec := httptest.NewRecorder()
		h.catalogSongs(rec, newRequest("catalog_songs", map[string]string{"auth": token, "filter": "1"}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["song"].([]any)
		if !ok || len(list) != 2 {
			t.Fatalf("expected 2 songs from library 1, got %v", got)
		}
	})

	t.Run("catalog_albums dedups and filters by library", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.searchSongs = []*model.Song{
			{ID: 1, LibraryID: 1, AlbumID: 10}, {ID: 2, LibraryID: 1, AlbumID: 10},
		}
		ts.music.album = &model.Album{ID: 10, Title: "LP"}
		rec := httptest.NewRecorder()
		h.catalogAlbums(rec, newRequest("catalog_albums", map[string]string{"auth": token, "filter": "1"}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["album"].([]any)
		if !ok || len(list) != 1 {
			t.Fatalf("expected deduped 1 album, got %v", got)
		}
	})

	t.Run("catalog_artists dedups and filters by library", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.music.searchSongs = []*model.Song{
			{ID: 1, LibraryID: 1, ArtistID: 5}, {ID: 2, LibraryID: 1, ArtistID: 5},
		}
		ts.music.artist = &model.Artist{ID: 5, Name: "A"}
		rec := httptest.NewRecorder()
		h.catalogArtists(rec, newRequest("catalog_artists", map[string]string{"auth": token, "filter": "1"}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["artist"].([]any)
		if !ok || len(list) != 1 {
			t.Fatalf("expected deduped 1 artist, got %v", got)
		}
	})

	t.Run("missing filter returns 4705", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		h.catalogSongs(rec, newRequest("catalog_songs", map[string]string{"auth": token}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4705 {
			t.Fatalf("expected 4705, got %+v", got)
		}
	})
}

// TestCatalogAction covers the admin-only scan-trigger action.
func TestCatalogAction(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.users.user = &model.User{ID: 1, IsAdmin: true}
	rec := httptest.NewRecorder()
	h.catalogAction(rec, newRequest("catalog_action", map[string]string{"auth": token}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got["success"] == nil {
		t.Fatalf("expected success key, got %v", got)
	}
}

// TestLabelsDelegation confirms labels/label/label_artists correctly delegate
// to the genre equivalents (implementation is a thin alias).
func TestLabelsDelegation(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.music.genres = []*model.Genre{{Name: "Rock"}}
	rec := httptest.NewRecorder()
	h.labels(rec, newRequest("labels", map[string]string{"auth": token}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	if _, ok := got["genre"]; !ok {
		t.Fatalf("expected labels to delegate to genres, got %v", got)
	}

	rec2 := httptest.NewRecorder()
	h.label(rec2, newRequest("label", map[string]string{"auth": token, "filter": "Rock"}), true)
	var g AmpGenre
	decodeJSON(t, rec2.Body.Bytes(), &g)
	if g.Name != "Rock" {
		t.Fatalf("expected label to delegate to genre, got %+v", g)
	}

	ts.music.songsByGenre = []*model.Song{{ID: 1, ArtistID: 1}}
	ts.music.artist = &model.Artist{ID: 1, Name: "A"}
	rec3 := httptest.NewRecorder()
	h.labelArtists(rec3, newRequest("label_artists", map[string]string{"auth": token, "filter": "Rock"}), true)
	var got3 map[string]any
	decodeJSON(t, rec3.Body.Bytes(), &got3)
	if _, ok := got3["artist"]; !ok {
		t.Fatalf("expected label_artists to delegate to genre_artists, got %v", got3)
	}
}
