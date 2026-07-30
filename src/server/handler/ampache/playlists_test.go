package ampache

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// TestPlaylists covers listing and the store-error path.
func TestPlaylists(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.playlists.playlists = []*model.Playlist{{ID: 1, Name: "P", UserID: 1}}
	rec := httptest.NewRecorder()
	h.playlists(rec, newRequest("playlists", map[string]string{"auth": token}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	list, ok := got["playlist"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("expected 1 playlist, got %v", got)
	}

	ts.playlists.playlistsErr = errors.New("x")
	rec2 := httptest.NewRecorder()
	h.playlists(rec2, newRequest("playlists", map[string]string{"auth": token}), true)
	var errGot xmlErrorJSON
	decodeJSON(t, rec2.Body.Bytes(), &errGot)
	if errGot.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", errGot)
	}
}

// TestPlaylist covers the visibility/access-control matrix: missing filter,
// not found, private-owned-by-other-non-admin denied, private-owned-by-self
// allowed, public allowed, and admin-override allowed.
func TestPlaylist(t *testing.T) {
	t.Run("missing filter", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		h.playlist(rec, newRequest("playlist", map[string]string{"auth": token}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4705 {
			t.Fatalf("expected 4705, got %+v", got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		h.playlist(rec, newRequest("playlist", map[string]string{"auth": token, "filter": "1"}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4704 {
			t.Fatalf("expected 4704, got %+v", got)
		}
	})

	t.Run("private owned by other, non-admin denied", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.playlists.playlist = &model.Playlist{ID: 1, UserID: 2, IsPublic: false}
		ts.users.user = &model.User{ID: 1, IsAdmin: false}
		rec := httptest.NewRecorder()
		h.playlist(rec, newRequest("playlist", map[string]string{"auth": token, "filter": "1"}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4742 {
			t.Fatalf("expected 4742, got %+v", got)
		}
	})

	t.Run("private owned by other, admin allowed", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.playlists.playlist = &model.Playlist{ID: 1, UserID: 2, IsPublic: false}
		ts.users.user = &model.User{ID: 1, IsAdmin: true}
		rec := httptest.NewRecorder()
		h.playlist(rec, newRequest("playlist", map[string]string{"auth": token, "filter": "1"}), true)
		var got AmpPlaylist
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ID != "1" {
			t.Fatalf("unexpected playlist: %+v", got)
		}
	})

	t.Run("public allowed for anyone", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.playlists.playlist = &model.Playlist{ID: 1, UserID: 2, IsPublic: true}
		rec := httptest.NewRecorder()
		h.playlist(rec, newRequest("playlist", map[string]string{"auth": token, "filter": "1"}), true)
		var got AmpPlaylist
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ID != "1" {
			t.Fatalf("unexpected playlist: %+v", got)
		}
	})

	t.Run("owner always allowed", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.playlists.playlist = &model.Playlist{ID: 1, UserID: 1, IsPublic: false}
		rec := httptest.NewRecorder()
		h.playlist(rec, newRequest("playlist", map[string]string{"auth": token, "filter": "1"}), true)
		var got AmpPlaylist
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ID != "1" {
			t.Fatalf("unexpected playlist: %+v", got)
		}
	})
}

// TestPlaylistSongs covers the entry-listing endpoint, including that songs
// which fail to resolve are silently skipped.
func TestPlaylistSongs(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.playlists.playlist = &model.Playlist{ID: 1, UserID: 1, IsPublic: true}
	ts.playlists.entries = []*model.PlaylistEntry{{SongID: 5}, {SongID: 6}}
	ts.music.song = &model.Song{ID: 5, Title: "T"}
	rec := httptest.NewRecorder()
	h.playlistSongs(rec, newRequest("playlist_songs", map[string]string{"auth": token, "filter": "1"}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	list, ok := got["song"].([]any)
	// stubMusicStore.GetSong always returns the same configured song regardless
	// of ID, so both entries resolve — this exercises the "song found" path.
	if !ok || len(list) != 2 {
		t.Fatalf("expected 2 songs, got %v", got)
	}
}

// TestPlaylistCreate covers missing name and success, including the
// public/private type mapping.
func TestPlaylistCreate(t *testing.T) {
	h, _, token := newAuthedHandler(1)
	rec := httptest.NewRecorder()
	h.playlistCreate(rec, newRequest("playlist_create", map[string]string{"auth": token}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	rec2 := httptest.NewRecorder()
	h.playlistCreate(rec2, newRequest("playlist_create", map[string]string{"auth": token, "name": "New", "type": "public"}), true)
	var got AmpPlaylist
	decodeJSON(t, rec2.Body.Bytes(), &got)
	if got.Type != "public" {
		t.Fatalf("expected public type, got %+v", got)
	}
}

// TestPlaylistEdit covers not-found, access denial, item-list parsing
// (including malformed entries being skipped), and store error.
func TestPlaylistEdit(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.playlists.playlist = &model.Playlist{ID: 1, UserID: 1}
	rec := httptest.NewRecorder()
	h.playlistEdit(rec, newRequest("playlist_edit", map[string]string{
		"auth": token, "filter": "1", "name": "Renamed", "items": "1,bogus,2",
	}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got["success"] == nil {
		t.Fatalf("expected success key, got %v", got)
	}

	ts.playlists.setEntriesErr = errors.New("x")
	rec2 := httptest.NewRecorder()
	h.playlistEdit(rec2, newRequest("playlist_edit", map[string]string{
		"auth": token, "filter": "1", "items": "1",
	}), true)
	var errGot xmlErrorJSON
	decodeJSON(t, rec2.Body.Bytes(), &errGot)
	if errGot.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", errGot)
	}
}

// TestPlaylistDelete covers not-found, non-owner-denied, and success.
func TestPlaylistDelete(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.playlists.playlist = &model.Playlist{ID: 1, UserID: 2}
	ts.users.user = &model.User{ID: 1, IsAdmin: false}
	rec := httptest.NewRecorder()
	h.playlistDelete(rec, newRequest("playlist_delete", map[string]string{"auth": token, "filter": "1"}), true)
	var denied xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &denied)
	if denied.ErrorCode != 4742 {
		t.Fatalf("expected 4742, got %+v", denied)
	}

	ts.playlists.playlist = &model.Playlist{ID: 1, UserID: 1}
	rec2 := httptest.NewRecorder()
	h.playlistDelete(rec2, newRequest("playlist_delete", map[string]string{"auth": token, "filter": "1"}), true)
	var got map[string]any
	decodeJSON(t, rec2.Body.Bytes(), &got)
	if got["success"] == nil {
		t.Fatalf("expected success key, got %v", got)
	}
}

// TestPlaylistAddSong covers missing params, not found, the duplicate-check
// short-circuit, and success.
func TestPlaylistAddSong(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.playlists.playlist = &model.Playlist{ID: 1, UserID: 1}

	rec := httptest.NewRecorder()
	h.playlistAddSong(rec, newRequest("playlist_add_song", map[string]string{"auth": token, "filter": "1"}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	ts.playlists.entries = []*model.PlaylistEntry{{SongID: 9}}
	rec2 := httptest.NewRecorder()
	h.playlistAddSong(rec2, newRequest("playlist_add_song", map[string]string{
		"auth": token, "filter": "1", "song_id": "9", "check": "1",
	}), true)
	var got map[string]any
	decodeJSON(t, rec2.Body.Bytes(), &got)
	if got["success"] != "song already in playlist" {
		t.Fatalf("expected dup short-circuit, got %v", got)
	}

	rec3 := httptest.NewRecorder()
	h.playlistAddSong(rec3, newRequest("playlist_add_song", map[string]string{
		"auth": token, "filter": "1", "song_id": "42",
	}), true)
	var got3 map[string]any
	decodeJSON(t, rec3.Body.Bytes(), &got3)
	if got3["success"] != "song added to playlist" {
		t.Fatalf("expected add success, got %v", got3)
	}
}

// TestPlaylistRemoveSong covers missing track param and the 1-to-0-index conversion.
func TestPlaylistRemoveSong(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.playlists.playlist = &model.Playlist{ID: 1, UserID: 1}

	rec := httptest.NewRecorder()
	h.playlistRemoveSong(rec, newRequest("playlist_remove_song", map[string]string{"auth": token, "filter": "1"}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	rec2 := httptest.NewRecorder()
	h.playlistRemoveSong(rec2, newRequest("playlist_remove_song", map[string]string{"auth": token, "filter": "1", "track": "1"}), true)
	var got map[string]any
	decodeJSON(t, rec2.Body.Bytes(), &got)
	if got["success"] == nil {
		t.Fatalf("expected success key, got %v", got)
	}
}

// TestPlaylistGenerate covers the mode dispatch and error path.
func TestPlaylistGenerate(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.music.randomSongs = []*model.Song{{ID: 1}}
	rec := httptest.NewRecorder()
	h.playlistGenerate(rec, newRequest("playlist_generate", map[string]string{"auth": token, "mode": "recent"}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	list, ok := got["song"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("expected 1 song, got %v", got)
	}

	ts.music.randomSongsErr = errors.New("x")
	rec2 := httptest.NewRecorder()
	h.playlistGenerate(rec2, newRequest("playlist_generate", map[string]string{"auth": token}), true)
	var errGot xmlErrorJSON
	decodeJSON(t, rec2.Body.Bytes(), &errGot)
	if errGot.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", errGot)
	}
}

// TestFlag covers missing params, star (flag=1), unstar (flag=0), and store error.
func TestFlag(t *testing.T) {
	h, ts, token := newAuthedHandler(1)

	rec := httptest.NewRecorder()
	h.flag(rec, newRequest("flag", map[string]string{"auth": token}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	rec2 := httptest.NewRecorder()
	h.flag(rec2, newRequest("flag", map[string]string{"auth": token, "type": "song", "id": "1", "flag": "1"}), true)
	var got map[string]any
	decodeJSON(t, rec2.Body.Bytes(), &got)
	if got["success"] == nil {
		t.Fatalf("expected success key, got %v", got)
	}

	rec3 := httptest.NewRecorder()
	h.flag(rec3, newRequest("flag", map[string]string{"auth": token, "type": "song", "id": "1", "flag": "0"}), true)
	var got3 map[string]any
	decodeJSON(t, rec3.Body.Bytes(), &got3)
	if got3["success"] == nil {
		t.Fatalf("expected success key, got %v", got3)
	}

	ts.activity.starErr = errors.New("x")
	rec4 := httptest.NewRecorder()
	h.flag(rec4, newRequest("flag", map[string]string{"auth": token, "type": "song", "id": "1", "flag": "1"}), true)
	var errGot xmlErrorJSON
	decodeJSON(t, rec4.Body.Bytes(), &errGot)
	if errGot.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", errGot)
	}
}

// TestRate covers missing params, out-of-range rating, and success.
func TestRate(t *testing.T) {
	h, _, token := newAuthedHandler(1)

	rec := httptest.NewRecorder()
	h.rate(rec, newRequest("rate", map[string]string{"auth": token}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	rec2 := httptest.NewRecorder()
	h.rate(rec2, newRequest("rate", map[string]string{"auth": token, "type": "song", "id": "1", "rating": "6"}), true)
	var badRange xmlErrorJSON
	decodeJSON(t, rec2.Body.Bytes(), &badRange)
	if badRange.ErrorCode != 4710 {
		t.Fatalf("expected 4710 for out-of-range rating, got %+v", badRange)
	}

	rec3 := httptest.NewRecorder()
	h.rate(rec3, newRequest("rate", map[string]string{"auth": token, "type": "song", "id": "1", "rating": "3"}), true)
	var got map[string]any
	decodeJSON(t, rec3.Body.Bytes(), &got)
	if got["success"] == nil {
		t.Fatalf("expected success key, got %v", got)
	}
}

// TestRecordPlay covers missing id, custom date parsing, and store error.
func TestRecordPlay(t *testing.T) {
	h, ts, token := newAuthedHandler(1)

	rec := httptest.NewRecorder()
	h.recordPlay(rec, newRequest("record_play", map[string]string{"auth": token}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	rec2 := httptest.NewRecorder()
	h.recordPlay(rec2, newRequest("record_play", map[string]string{"auth": token, "id": "1", "date": "1700000000"}), true)
	var got map[string]any
	decodeJSON(t, rec2.Body.Bytes(), &got)
	if got["success"] == nil {
		t.Fatalf("expected success key, got %v", got)
	}

	ts.activity.recordPlayErr = errors.New("x")
	rec3 := httptest.NewRecorder()
	h.recordPlay(rec3, newRequest("record_play", map[string]string{"auth": token, "id": "1"}), true)
	var errGot xmlErrorJSON
	decodeJSON(t, rec3.Body.Bytes(), &errGot)
	if errGot.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", errGot)
	}
}

// TestScrobble covers the title+artist match logic: match found records a
// play, no match still returns success (best-effort, never errors to caller).
func TestScrobble(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.music.searchSongs = []*model.Song{{ID: 1, Title: "Song", ArtistName: "Artist"}}
	rec := httptest.NewRecorder()
	h.scrobble(rec, newRequest("scrobble", map[string]string{"auth": token, "song": "Song", "artist": "Artist"}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got["success"] != "scrobble recorded" {
		t.Fatalf("unexpected scrobble response: %v", got)
	}

	rec2 := httptest.NewRecorder()
	h.scrobble(rec2, newRequest("scrobble", map[string]string{"auth": token, "song": "NoMatch", "artist": "Nobody"}), true)
	var got2 map[string]any
	decodeJSON(t, rec2.Body.Bytes(), &got2)
	if got2["success"] != "scrobble recorded" {
		t.Fatalf("unexpected no-match scrobble response: %v", got2)
	}
}

// TestNowPlaying covers the always-empty stub response.
func TestNowPlaying(t *testing.T) {
	h, _, token := newAuthedHandler(1)
	rec := httptest.NewRecorder()
	h.nowPlaying(rec, newRequest("now_playing", map[string]string{"auth": token}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	list, ok := got["now_playing"].([]any)
	if !ok || len(list) != 0 {
		t.Fatalf("expected empty now_playing, got %v", got)
	}
}

// TestGetSimilar covers missing id, song-type genre lookup, artist-type genre
// lookup, self-exclusion from results, and store error.
func TestGetSimilar(t *testing.T) {
	h, ts, token := newAuthedHandler(1)

	rec := httptest.NewRecorder()
	h.getSimilar(rec, newRequest("get_similar", map[string]string{"auth": token}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	t.Run("song not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.getSimilar(rec, newRequest("get_similar", map[string]string{"auth": token, "id": "1"}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4704 {
			t.Fatalf("expected 4704, got %+v", got)
		}
	})

	t.Run("song type excludes self from results", func(t *testing.T) {
		ts.music.song = &model.Song{ID: 1, Genre: "Rock"}
		ts.music.randomSongs = []*model.Song{{ID: 1}, {ID: 2}}
		rec := httptest.NewRecorder()
		h.getSimilar(rec, newRequest("get_similar", map[string]string{"auth": token, "id": "1"}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["song"].([]any)
		if !ok || len(list) != 1 {
			t.Fatalf("expected self-excluded 1 song, got %v", got)
		}
	})

	t.Run("artist type", func(t *testing.T) {
		ts.music.songsByArtist = []*model.Song{{ID: 5, Genre: "Jazz"}}
		ts.music.randomSongs = []*model.Song{{ID: 9}}
		rec := httptest.NewRecorder()
		h.getSimilar(rec, newRequest("get_similar", map[string]string{"auth": token, "id": "1", "type": "artist"}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		if _, ok := got["song"]; !ok {
			t.Fatalf("expected song key, got %v", got)
		}
	})

	t.Run("store error", func(t *testing.T) {
		ts.music.song = &model.Song{ID: 1}
		ts.music.randomSongsErr = errors.New("x")
		rec := httptest.NewRecorder()
		h.getSimilar(rec, newRequest("get_similar", map[string]string{"auth": token, "id": "1"}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4710 {
			t.Fatalf("expected 4710, got %+v", got)
		}
	})
}

// TestShares covers listing, single lookup access control, create, edit, delete.
func TestShares(t *testing.T) {
	t.Run("shares list", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, Username: "alice"}
		ts.shares.shares = []*model.Share{{ID: 1, UserID: 1, Token: "tok"}}
		rec := httptest.NewRecorder()
		h.shares(rec, newRequest("shares", map[string]string{"auth": token}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["share"].([]any)
		if !ok || len(list) != 1 {
			t.Fatalf("expected 1 share, got %v", got)
		}
	})

	t.Run("share not found", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		h.share(rec, newRequest("share", map[string]string{"auth": token, "filter": "1"}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4704 {
			t.Fatalf("expected 4704, got %+v", got)
		}
	})

	t.Run("share access denied for non-owner non-admin", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.shares.share = &model.Share{ID: 1, UserID: 2, Token: "tok"}
		ts.users.user = &model.User{ID: 1, IsAdmin: false}
		rec := httptest.NewRecorder()
		h.share(rec, newRequest("share", map[string]string{"auth": token, "filter": "1"}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4742 {
			t.Fatalf("expected 4742, got %+v", got)
		}
	})

	t.Run("share_create missing params", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		h.shareCreate(rec, newRequest("share_create", map[string]string{"auth": token}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4705 {
			t.Fatalf("expected 4705, got %+v", got)
		}
	})

	t.Run("share_create success", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, Username: "alice"}
		rec := httptest.NewRecorder()
		h.shareCreate(rec, newRequest("share_create", map[string]string{"auth": token, "type": "song", "id": "5"}), true)
		var got AmpShare
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ObjectID != "5" || got.ObjectType != "song" {
			t.Fatalf("unexpected created share: %+v", got)
		}
	})

	t.Run("share_edit and share_delete require ownership", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.shares.share = &model.Share{ID: 1, UserID: 1, Token: "tok"}
		ts.users.user = &model.User{ID: 1, IsAdmin: false}
		rec := httptest.NewRecorder()
		h.shareEdit(rec, newRequest("share_edit", map[string]string{"auth": token, "filter": "1", "description": "d"}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got["success"] == nil {
			t.Fatalf("expected success key, got %v", got)
		}

		rec2 := httptest.NewRecorder()
		h.shareDelete(rec2, newRequest("share_delete", map[string]string{"auth": token, "filter": "1"}), true)
		var got2 map[string]any
		decodeJSON(t, rec2.Body.Bytes(), &got2)
		if got2["success"] == nil {
			t.Fatalf("expected success key, got %v", got2)
		}
	})
}

// TestBookmarks covers list, create/edit (edit delegates to create), delete,
// and get_bookmark find/not-found.
func TestBookmarks(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.activity.bookmarks = []*model.Bookmark{{ID: 1, ItemType: "song", ItemID: 5}}
		rec := httptest.NewRecorder()
		h.bookmarks(rec, newRequest("bookmarks", map[string]string{"auth": token}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["bookmark"].([]any)
		if !ok || len(list) != 1 {
			t.Fatalf("expected 1 bookmark, got %v", got)
		}
	})

	t.Run("create missing filter", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		h.bookmarkCreate(rec, newRequest("bookmark_create", map[string]string{"auth": token}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4705 {
			t.Fatalf("expected 4705, got %+v", got)
		}
	})

	t.Run("create success and edit delegates", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		h.bookmarkCreate(rec, newRequest("bookmark_create", map[string]string{"auth": token, "filter": "5", "position": "30"}), true)
		var got AmpBookmark
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ObjectID != "5" || got.Position != 30 {
			t.Fatalf("unexpected bookmark: %+v", got)
		}

		rec2 := httptest.NewRecorder()
		h.bookmarkEdit(rec2, newRequest("bookmark_edit", map[string]string{"auth": token, "filter": "5", "position": "60"}), true)
		var got2 AmpBookmark
		decodeJSON(t, rec2.Body.Bytes(), &got2)
		if got2.Position != 60 {
			t.Fatalf("expected edit to delegate to create, got %+v", got2)
		}
	})

	t.Run("delete missing filter and success", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		h.bookmarkDelete(rec, newRequest("bookmark_delete", map[string]string{"auth": token}), true)
		var missing xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &missing)
		if missing.ErrorCode != 4705 {
			t.Fatalf("expected 4705, got %+v", missing)
		}

		rec2 := httptest.NewRecorder()
		h.bookmarkDelete(rec2, newRequest("bookmark_delete", map[string]string{"auth": token, "filter": "5"}), true)
		var got map[string]any
		decodeJSON(t, rec2.Body.Bytes(), &got)
		if got["success"] == nil {
			t.Fatalf("expected success key, got %v", got)
		}
	})

	t.Run("get_bookmark found and not found", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.activity.bookmarks = []*model.Bookmark{{ID: 1, ItemType: "song", ItemID: 5}}
		rec := httptest.NewRecorder()
		h.getBookmark(rec, newRequest("get_bookmark", map[string]string{"auth": token, "filter": "5"}), true)
		var got AmpBookmark
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ObjectID != "5" {
			t.Fatalf("unexpected bookmark: %+v", got)
		}

		rec2 := httptest.NewRecorder()
		h.getBookmark(rec2, newRequest("get_bookmark", map[string]string{"auth": token, "filter": "999"}), true)
		var notFound xmlErrorJSON
		decodeJSON(t, rec2.Body.Bytes(), &notFound)
		if notFound.ErrorCode != 4704 {
			t.Fatalf("expected 4704, got %+v", notFound)
		}
	})
}

// TestDeletedStubEndpoints covers the always-empty "deleted" list endpoints.
func TestDeletedStubEndpoints(t *testing.T) {
	h, _, token := newAuthedHandler(1)

	rec := httptest.NewRecorder()
	h.deletedSongs(rec, newRequest("deleted_songs", map[string]string{"auth": token}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	if list, ok := got["song"].([]any); !ok || len(list) != 0 {
		t.Fatalf("expected empty song list, got %v", got)
	}

	rec2 := httptest.NewRecorder()
	h.deletedVideo(rec2, newRequest("deleted_video", map[string]string{"auth": token}), true)
	var got2 map[string]any
	decodeJSON(t, rec2.Body.Bytes(), &got2)
	if list, ok := got2["video"].([]any); !ok || len(list) != 0 {
		t.Fatalf("expected empty video list, got %v", got2)
	}

	rec3 := httptest.NewRecorder()
	h.deletedPodcastEpisodes(rec3, newRequest("deleted_podcast_episodes", map[string]string{"auth": token}), true)
	var got3 map[string]any
	decodeJSON(t, rec3.Body.Bytes(), &got3)
	if list, ok := got3["podcast_episode"].([]any); !ok || len(list) != 0 {
		t.Fatalf("expected empty podcast_episode list, got %v", got3)
	}
}
