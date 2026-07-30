package subsonic

// Tests for browse.go handler methods: getIndexes, getMusicDirectory,
// getGenres, getArtists, getArtist, getAlbum, getSong, getAlbumList,
// getAlbumList2, getRandomSongs, getSongsByGenre, getStarred, getStarred2,
// getNowPlaying, getVideos, getVideoInfo, getArtistInfo(2), getAlbumInfo(2),
// getSimilarSongs(2), getTopSongs, and the artistIndexLetter/stripIgnoredArticle
// helpers.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// ---- getIndexes ------------------------------------------------------------

func TestGetIndexesUnauthenticated(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getIndexes?f=json", nil)

	h.getIndexes(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getIndexes unauthenticated: got %+v, want code %d", resp.Error, ErrNotAuthenticated)
	}
}

func TestGetIndexesGroupsByLetter(t *testing.T) {
	artists := []*model.Artist{
		{ID: 1, Name: "The Beatles"},
		{ID: 2, Name: "ABBA"},
		{ID: 3, Name: "1990s Band"},
	}
	music := &stubArtistsMusicStore{artists: artists}
	h := newTestHandlerFull(nil, nil, nil)
	h.db.Music = music

	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getIndexes?f=json", false)

	h.getIndexes(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Indexes == nil {
		t.Fatal("getIndexes: Indexes is nil")
	}
	if len(resp.Indexes.Index) != 3 {
		t.Fatalf("getIndexes: got %d index groups, want 3 (A, B, #)", len(resp.Indexes.Index))
	}
}

func TestGetIndexesStoreError(t *testing.T) {
	music := &stubArtistsMusicStore{artistsErr: errFake("db down")}
	h := newTestHandlerFull(nil, nil, nil)
	h.db.Music = music
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getIndexes?f=json", false)

	h.getIndexes(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("getIndexes store error: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

// ---- getMusicDirectory ------------------------------------------------------

func TestGetMusicDirectoryMissingID(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getMusicDirectory?f=json", false)

	h.getMusicDirectory(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("getMusicDirectory missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestGetMusicDirectoryMalformedID(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getMusicDirectory?f=json&id=bad-id-xyz", false)

	h.getMusicDirectory(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("getMusicDirectory malformed id: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestGetMusicDirectoryArtistNotFound(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getMusicDirectory?f=json&id="+encodeArtistID(1), false)

	h.getMusicDirectory(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("getMusicDirectory artist not found: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

// ---- getGenres ---------------------------------------------------------------

func TestGetGenresEmpty(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getGenres?f=json", false)

	h.getGenres(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Genres == nil {
		t.Fatal("getGenres: Genres is nil")
	}
	if len(resp.Genres.Genre) != 0 {
		t.Errorf("getGenres: got %d genres, want 0", len(resp.Genres.Genre))
	}
}

// ---- getArtist / getAlbum / getSong: missing param + not found -------------

func TestGetArtistMissingID(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getArtist?f=json", false)

	h.getArtist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("getArtist missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestGetArtistNotFound(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getArtist?f=json&id="+encodeArtistID(99), false)

	h.getArtist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("getArtist not found: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestGetAlbumMissingID(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getAlbum?f=json", false)

	h.getAlbum(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("getAlbum missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestGetAlbumNotFound(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getAlbum?f=json&id="+encodeAlbumID(99), false)

	h.getAlbum(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("getAlbum not found: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestGetSongMissingID(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getSong?f=json", false)

	h.getSong(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("getSong missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestGetSongNotFound(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getSong?f=json&id="+encodeSongID(99), false)

	h.getSong(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("getSong not found: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

// ---- getAlbumList / getAlbumList2 -------------------------------------------

func TestGetAlbumListDefaultsToNewest(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getAlbumList?f=json", false)

	h.getAlbumList(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("getAlbumList: status %q, want ok", resp.Status)
	}
	if resp.AlbumList == nil {
		t.Fatal("getAlbumList: AlbumList is nil")
	}
}

func TestGetAlbumList2Unauthenticated(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getAlbumList2?f=json", nil)

	h.getAlbumList2(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getAlbumList2 unauthenticated: got %+v", resp.Error)
	}
}

// ---- getRandomSongs / getSongsByGenre ---------------------------------------

func TestGetRandomSongsEmpty(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getRandomSongs?f=json", false)

	h.getRandomSongs(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.RandomSongs == nil {
		t.Fatal("getRandomSongs: RandomSongs is nil")
	}
	if len(resp.RandomSongs.Song) != 0 {
		t.Errorf("getRandomSongs: got %d songs, want 0", len(resp.RandomSongs.Song))
	}
}

func TestGetSongsByGenreMissingGenre(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getSongsByGenre?f=json", false)

	h.getSongsByGenre(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("getSongsByGenre missing genre: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestGetSongsByGenreSuccess(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getSongsByGenre?f=json&genre=Rock", false)

	h.getSongsByGenre(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("getSongsByGenre: status %q, want ok", resp.Status)
	}
	if resp.SongsByGenre == nil {
		t.Fatal("getSongsByGenre: SongsByGenre is nil")
	}
}

// ---- getStarred / getStarred2 -----------------------------------------------

func TestGetStarredUnauthenticated(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getStarred?f=json", nil)

	h.getStarred(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getStarred unauthenticated: got %+v", resp.Error)
	}
}

func TestGetStarredWithItems(t *testing.T) {
	starred := &store.StarredItems{
		Songs:   []*model.Song{{ID: 1, Title: "Song A"}},
		Albums:  []*model.Album{{ID: 2, Title: "Album B"}},
		Artists: []*model.Artist{{ID: 3, Name: "Artist C"}},
	}
	h := newTestHandlerFull(nil, nil, &stubActivityStore{starred: starred})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getStarred?f=json", false)

	h.getStarred(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Starred == nil {
		t.Fatal("getStarred: Starred is nil")
	}
	if len(resp.Starred.Song) != 1 || len(resp.Starred.Album) != 1 || len(resp.Starred.Artist) != 1 {
		t.Errorf("getStarred: got song=%d album=%d artist=%d, want 1/1/1",
			len(resp.Starred.Song), len(resp.Starred.Album), len(resp.Starred.Artist))
	}
}

func TestGetStarredStoreError(t *testing.T) {
	h := newTestHandlerFull(nil, nil, &stubActivityStore{starredErr: errFake("db down")})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getStarred?f=json", false)

	h.getStarred(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("getStarred store error: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

func TestGetStarred2WithItems(t *testing.T) {
	starred := &store.StarredItems{
		Songs:   []*model.Song{{ID: 1, Title: "Song A"}},
		Albums:  []*model.Album{{ID: 2, Title: "Album B"}},
		Artists: []*model.Artist{{ID: 3, Name: "Artist C"}},
	}
	h := newTestHandlerFull(nil, nil, &stubActivityStore{starred: starred})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getStarred2?f=json", false)

	h.getStarred2(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Starred2 == nil {
		t.Fatal("getStarred2: Starred2 is nil")
	}
	if len(resp.Starred2.Song) != 1 || len(resp.Starred2.Album) != 1 || len(resp.Starred2.Artist) != 1 {
		t.Errorf("getStarred2: got song=%d album=%d artist=%d, want 1/1/1",
			len(resp.Starred2.Song), len(resp.Starred2.Album), len(resp.Starred2.Artist))
	}
}

// ---- getNowPlaying / getVideos / getVideoInfo -------------------------------

func TestGetNowPlayingEmpty(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getNowPlaying?f=json", false)

	h.getNowPlaying(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.NowPlaying == nil {
		t.Fatal("getNowPlaying: NowPlaying is nil")
	}
	if len(resp.NowPlaying.Entry) != 0 {
		t.Errorf("getNowPlaying: got %d entries, want 0", len(resp.NowPlaying.Entry))
	}
}

func TestGetNowPlayingWithActiveStream(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	h.nowPlaying.Register(&NowPlayingEntry{
		UserID: 1, Username: "alice", SongID: 5, Title: "Song A", PlayerName: "web",
	})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getNowPlaying?f=json", false)

	h.getNowPlaying(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if len(resp.NowPlaying.Entry) != 1 {
		t.Fatalf("getNowPlaying: got %d entries, want 1", len(resp.NowPlaying.Entry))
	}
	if resp.NowPlaying.Entry[0].Username != "alice" {
		t.Errorf("getNowPlaying: Username = %q, want alice", resp.NowPlaying.Entry[0].Username)
	}
}

func TestGetVideosAlwaysEmpty(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getVideos?f=json", false)

	h.getVideos(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Videos == nil {
		t.Fatal("getVideos: Videos is nil")
	}
	if len(resp.Videos.Video) != 0 {
		t.Errorf("getVideos: got %d videos, want 0", len(resp.Videos.Video))
	}
}

func TestGetVideoInfoNotFound(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getVideoInfo?f=json", false)

	h.getVideoInfo(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("getVideoInfo: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

// ---- getArtistInfo / getArtistInfo2 / getAlbumInfo / getAlbumInfo2 ---------

func TestGetArtistInfoMissingID(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getArtistInfo?f=json", false)

	h.getArtistInfo(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("getArtistInfo missing id: got %+v", resp.Error)
	}
}

func TestGetArtistInfoNotFound(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getArtistInfo?f=json&id="+encodeArtistID(1), false)

	h.getArtistInfo(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("getArtistInfo not found: got %+v", resp.Error)
	}
}

func TestGetArtistInfo2NotFound(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getArtistInfo2?f=json&id="+encodeArtistID(1), false)

	h.getArtistInfo2(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("getArtistInfo2 not found: got %+v", resp.Error)
	}
}

func TestGetAlbumInfoMissingID(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getAlbumInfo?f=json", false)

	h.getAlbumInfo(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("getAlbumInfo missing id: got %+v", resp.Error)
	}
}

func TestGetAlbumInfo2DelegatesToGetAlbumInfo(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getAlbumInfo2?f=json", false)

	h.getAlbumInfo2(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("getAlbumInfo2 missing id: got %+v, want it to delegate to getAlbumInfo", resp.Error)
	}
}

// ---- getSimilarSongs / getSimilarSongs2 -------------------------------------

func TestGetSimilarSongsMissingID(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getSimilarSongs?f=json", false)

	h.getSimilarSongs(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("getSimilarSongs missing id: got %+v", resp.Error)
	}
}

func TestGetSimilarSongsSongNotFound(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getSimilarSongs?f=json&id="+encodeSongID(1), false)

	h.getSimilarSongs(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("getSimilarSongs song not found: got %+v", resp.Error)
	}
}

func TestGetSimilarSongs2ArtistNotFound(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getSimilarSongs2?f=json&id="+encodeArtistID(1), false)

	h.getSimilarSongs2(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("getSimilarSongs2 artist not found: got %+v", resp.Error)
	}
}

// ---- getTopSongs -------------------------------------------------------------

func TestGetTopSongsMissingArtist(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getTopSongs?f=json", false)

	h.getTopSongs(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("getTopSongs missing artist: got %+v", resp.Error)
	}
}

func TestGetTopSongsArtistNotFoundReturnsEmpty(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getTopSongs?f=json&artist=Unknown", false)

	h.getTopSongs(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("getTopSongs unknown artist: status %q, want ok (empty result)", resp.Status)
	}
	if resp.TopSongs == nil {
		t.Fatal("getTopSongs: TopSongs is nil")
	}
	if len(resp.TopSongs.Song) != 0 {
		t.Errorf("getTopSongs: got %d songs, want 0", len(resp.TopSongs.Song))
	}
}

// ---- artistIndexLetter / stripIgnoredArticle --------------------------------

func TestArtistIndexLetter(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"The Beatles", "B"},
		{"ABBA", "A"},
		{"1990s Band", "#"},
		{"", "#"},
		{"élan", "É"},
		{"An Apple", "A"},
		{"A Band", "B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := artistIndexLetter(tt.name); got != tt.want {
				t.Errorf("artistIndexLetter(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestStripIgnoredArticle(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"The Beatles", "Beatles"},
		{"A Band", "Band"},
		{"An Apple", "Apple"},
		{"Metallica", "Metallica"},
		{"THE Who", "Who"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripIgnoredArticle(tt.name); got != tt.want {
				t.Errorf("stripIgnoredArticle(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// ---- queryIntDefault / isoTime helpers ---------------------------------------

func TestQueryIntDefault(t *testing.T) {
	if got := queryIntDefault("", 5); got != 5 {
		t.Errorf("queryIntDefault(\"\", 5) = %d, want 5", got)
	}
	if got := queryIntDefault("42", 5); got != 42 {
		t.Errorf("queryIntDefault(\"42\", 5) = %d, want 42", got)
	}
	if got := queryIntDefault("notanumber", 5); got != 5 {
		t.Errorf("queryIntDefault(\"notanumber\", 5) = %d, want 5 (fallback)", got)
	}
}

// ---- Child/AlbumID3/ArtistID3 conversion helpers -----------------------------

func TestSongToChildSuffixFromPath(t *testing.T) {
	s := &model.Song{ID: 1, AlbumID: 2, ArtistID: 3, Path: "/music/song.flac", Title: "Song"}
	c := songToChild(s)
	if c.Suffix != "flac" {
		t.Errorf("songToChild: Suffix = %q, want flac", c.Suffix)
	}
	if c.ID != encodeSongID(1) {
		t.Errorf("songToChild: ID = %q, want %q", c.ID, encodeSongID(1))
	}
	if c.IsDir {
		t.Error("songToChild: IsDir should be false")
	}
}

func TestAlbumToChildIsDir(t *testing.T) {
	al := &model.Album{ID: 1, ArtistID: 2, Title: "Album"}
	c := albumToChild(al)
	if !c.IsDir {
		t.Error("albumToChild: IsDir should be true")
	}
	if c.ID != encodeAlbumID(1) {
		t.Errorf("albumToChild: ID = %q, want %q", c.ID, encodeAlbumID(1))
	}
}

func TestArtistToChildIsDir(t *testing.T) {
	a := &model.Artist{ID: 1, Name: "Artist"}
	c := artistToChild(a)
	if !c.IsDir {
		t.Error("artistToChild: IsDir should be true")
	}
	if c.Title != "Artist" {
		t.Errorf("artistToChild: Title = %q, want Artist", c.Title)
	}
}

// errFake is a minimal error type used across subsonic package tests.
type errFake string

func (e errFake) Error() string { return string(e) }

// ---- stub for artist listing used by getIndexes/getArtists tests -----------

// stubArtistsMusicStore extends stubMusicStore to control ListArtists output
// independently, since stubMusicStore.ListArtists always returns (nil, nil).
type stubArtistsMusicStore struct {
	stubMusicStore
	artists    []*model.Artist
	artistsErr error
}

func (s *stubArtistsMusicStore) ListArtists(_ context.Context, _ store.ListOpts) ([]*model.Artist, error) {
	return s.artists, s.artistsErr
}
