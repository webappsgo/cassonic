package store

import (
	"context"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
)

// newTestMusicStore creates an in-memory SQLite MusicStore with the full server schema.
func newTestMusicStore(t *testing.T) MusicStore {
	t.Helper()
	db, err := openDB(":memory:", serverSchema)
	if err != nil {
		t.Fatalf("openDB :memory:: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &sqliteMusicStore{db: db}
}

// sampleLibrary returns a minimal Library suitable for insertion.
func sampleLibrary(name, path string) *model.Library {
	return &model.Library{
		Name:    name,
		Path:    path,
		Enabled: true,
	}
}

// sampleArtist returns a minimal Artist suitable for insertion.
func sampleArtist(name string) *model.Artist {
	return &model.Artist{
		Name:     name,
		SortName: name,
	}
}

// sampleAlbum returns a minimal Album suitable for insertion.
func sampleAlbum(title string, artistID int64) *model.Album {
	return &model.Album{
		Title:      title,
		ArtistID:   artistID,
		ArtistName: "Test Artist",
		Year:       2024,
	}
}

// sampleSong returns a minimal Song suitable for insertion.
func sampleSong(path string, libID, artistID, albumID int64) *model.Song {
	return &model.Song{
		LibraryID:   libID,
		Path:        path,
		Title:       "Test Song",
		ArtistID:    artistID,
		ArtistName:  "Test Artist",
		AlbumID:     albumID,
		AlbumName:   "Test Album",
		Duration:    180,
		ContentType: "audio/mpeg",
		FileFormat:  "mp3",
	}
}

// TestCreateGetLibraryRoundtrip verifies that a library inserted via CreateLibrary
// can be retrieved by its ID with fields intact.
func TestCreateGetLibraryRoundtrip(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	lib := sampleLibrary("My Music", "/home/user/music")
	id, err := s.CreateLibrary(ctx, lib)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	if id <= 0 {
		t.Fatalf("CreateLibrary returned non-positive ID: %d", id)
	}

	got, err := s.GetLibrary(ctx, id)
	if err != nil {
		t.Fatalf("GetLibrary: %v", err)
	}
	if got == nil {
		t.Fatal("GetLibrary returned nil for a just-created library")
	}
	if got.Name != lib.Name {
		t.Errorf("Name: got %q, want %q", got.Name, lib.Name)
	}
	if got.Path != lib.Path {
		t.Errorf("Path: got %q, want %q", got.Path, lib.Path)
	}
	if got.Enabled != lib.Enabled {
		t.Errorf("Enabled: got %v, want %v", got.Enabled, lib.Enabled)
	}
}

// TestGetLibraryMissingReturnsNil verifies that GetLibrary for an unknown ID
// returns (nil, nil).
func TestGetLibraryMissingReturnsNil(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	got, err := s.GetLibrary(ctx, 99999)
	if err != nil {
		t.Fatalf("GetLibrary (missing): unexpected error: %v", err)
	}
	if got != nil {
		t.Error("GetLibrary (missing): expected nil, got non-nil")
	}
}

// TestListLibrariesEmpty verifies that ListLibraries on an empty store returns empty slice.
func TestListLibrariesEmpty(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	libs, err := s.ListLibraries(ctx)
	if err != nil {
		t.Fatalf("ListLibraries (empty): %v", err)
	}
	if len(libs) != 0 {
		t.Errorf("ListLibraries (empty): got %d libraries, want 0", len(libs))
	}
}

// TestListLibrariesMultiple verifies that all created libraries are returned.
func TestListLibrariesMultiple(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	paths := []string{"/music/a", "/music/b", "/music/c"}
	for i, p := range paths {
		lib := sampleLibrary("lib", p)
		lib.Name = p
		id, err := s.CreateLibrary(ctx, lib)
		if err != nil {
			t.Fatalf("CreateLibrary[%d]: %v", i, err)
		}
		if id <= 0 {
			t.Fatalf("CreateLibrary[%d]: non-positive ID", i)
		}
	}

	libs, err := s.ListLibraries(ctx)
	if err != nil {
		t.Fatalf("ListLibraries: %v", err)
	}
	if len(libs) != len(paths) {
		t.Errorf("ListLibraries: got %d, want %d", len(libs), len(paths))
	}
}

// TestUpdateLibrary verifies that UpdateLibrary persists changed fields.
func TestUpdateLibrary(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	lib := sampleLibrary("old name", "/music")
	id, err := s.CreateLibrary(ctx, lib)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}

	lib.ID = id
	lib.Name = "new name"
	lib.Enabled = false
	if err := s.UpdateLibrary(ctx, lib); err != nil {
		t.Fatalf("UpdateLibrary: %v", err)
	}

	got, err := s.GetLibrary(ctx, id)
	if err != nil {
		t.Fatalf("GetLibrary after update: %v", err)
	}
	if got.Name != "new name" {
		t.Errorf("Name after update: got %q, want %q", got.Name, "new name")
	}
	if got.Enabled {
		t.Error("Enabled after update: got true, want false")
	}
}

// TestDeleteLibrary verifies that DeleteLibrary removes the library.
func TestDeleteLibrary(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	lib := sampleLibrary("tmp", "/tmp/music")
	id, err := s.CreateLibrary(ctx, lib)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}

	if err := s.DeleteLibrary(ctx, id); err != nil {
		t.Fatalf("DeleteLibrary: %v", err)
	}

	got, err := s.GetLibrary(ctx, id)
	if err != nil {
		t.Fatalf("GetLibrary after delete: %v", err)
	}
	if got != nil {
		t.Error("GetLibrary after delete: expected nil, got non-nil")
	}
}

// TestUpsertGetArtistRoundtrip verifies that an artist upserted via UpsertArtist
// can be retrieved by ID and name.
func TestUpsertGetArtistRoundtrip(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	a := sampleArtist("Pink Floyd")
	id, err := s.UpsertArtist(ctx, a)
	if err != nil {
		t.Fatalf("UpsertArtist: %v", err)
	}
	if id <= 0 {
		t.Fatalf("UpsertArtist returned non-positive ID: %d", id)
	}

	got, err := s.GetArtist(ctx, id)
	if err != nil {
		t.Fatalf("GetArtist: %v", err)
	}
	if got == nil {
		t.Fatal("GetArtist returned nil for a just-upserted artist")
	}
	if got.Name != "Pink Floyd" {
		t.Errorf("Name: got %q, want %q", got.Name, "Pink Floyd")
	}
}

// TestUpsertArtistIsIdempotent verifies that upserting the same artist name
// returns the same ID both times (case-insensitive deduplication).
func TestUpsertArtistIsIdempotent(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	a1 := sampleArtist("Led Zeppelin")
	id1, err := s.UpsertArtist(ctx, a1)
	if err != nil {
		t.Fatalf("UpsertArtist first: %v", err)
	}

	a2 := sampleArtist("Led Zeppelin")
	id2, err := s.UpsertArtist(ctx, a2)
	if err != nil {
		t.Fatalf("UpsertArtist second: %v", err)
	}

	if id1 != id2 {
		t.Errorf("UpsertArtist idempotent: got different IDs %d vs %d", id1, id2)
	}
}

// TestGetArtistByName verifies lookup by artist name.
func TestGetArtistByName(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	a := sampleArtist("The Beatles")
	id, err := s.UpsertArtist(ctx, a)
	if err != nil {
		t.Fatalf("UpsertArtist: %v", err)
	}

	got, err := s.GetArtistByName(ctx, "The Beatles")
	if err != nil {
		t.Fatalf("GetArtistByName: %v", err)
	}
	if got == nil {
		t.Fatal("GetArtistByName returned nil")
	}
	if got.ID != id {
		t.Errorf("ID: got %d, want %d", got.ID, id)
	}
}

// TestGetArtistByNameMissing verifies that GetArtistByName for an unknown name
// returns (nil, nil).
func TestGetArtistByNameMissing(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	got, err := s.GetArtistByName(ctx, "Nobody Famous")
	if err != nil {
		t.Fatalf("GetArtistByName (missing): %v", err)
	}
	if got != nil {
		t.Error("GetArtistByName (missing): expected nil, got non-nil")
	}
}

// TestListArtists verifies that ListArtists returns all artists.
func TestListArtists(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	names := []string{"Artist A", "Artist B", "Artist C"}
	for _, n := range names {
		if _, err := s.UpsertArtist(ctx, sampleArtist(n)); err != nil {
			t.Fatalf("UpsertArtist(%q): %v", n, err)
		}
	}

	artists, err := s.ListArtists(ctx, ListOpts{})
	if err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	if len(artists) != len(names) {
		t.Errorf("ListArtists: got %d, want %d", len(artists), len(names))
	}
}

// TestSearchArtists verifies that SearchArtists filters by name substring.
func TestSearchArtists(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	for _, n := range []string{"Rolling Stones", "Stone Temple Pilots", "Bob Dylan"} {
		if _, err := s.UpsertArtist(ctx, sampleArtist(n)); err != nil {
			t.Fatalf("UpsertArtist(%q): %v", n, err)
		}
	}

	results, err := s.SearchArtists(ctx, "stone", ListOpts{})
	if err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("SearchArtists(%q): got %d results, want 2", "stone", len(results))
	}
}

// TestUpsertGetAlbumRoundtrip verifies Album upsert+get.
func TestUpsertGetAlbumRoundtrip(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	artistID, err := s.UpsertArtist(ctx, sampleArtist("Radiohead"))
	if err != nil {
		t.Fatalf("UpsertArtist: %v", err)
	}

	al := sampleAlbum("OK Computer", artistID)
	id, err := s.UpsertAlbum(ctx, al)
	if err != nil {
		t.Fatalf("UpsertAlbum: %v", err)
	}
	if id <= 0 {
		t.Fatalf("UpsertAlbum returned non-positive ID: %d", id)
	}

	got, err := s.GetAlbum(ctx, id)
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}
	if got == nil {
		t.Fatal("GetAlbum returned nil for a just-upserted album")
	}
	if got.Title != "OK Computer" {
		t.Errorf("Title: got %q, want %q", got.Title, "OK Computer")
	}
	if got.ArtistID != artistID {
		t.Errorf("ArtistID: got %d, want %d", got.ArtistID, artistID)
	}
}

// TestUpsertAlbumIsIdempotent verifies that upserting the same album title+artist
// returns the same ID.
func TestUpsertAlbumIsIdempotent(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	artistID, err := s.UpsertArtist(ctx, sampleArtist("Nirvana"))
	if err != nil {
		t.Fatalf("UpsertArtist: %v", err)
	}

	id1, err := s.UpsertAlbum(ctx, sampleAlbum("Nevermind", artistID))
	if err != nil {
		t.Fatalf("UpsertAlbum first: %v", err)
	}

	id2, err := s.UpsertAlbum(ctx, sampleAlbum("Nevermind", artistID))
	if err != nil {
		t.Fatalf("UpsertAlbum second: %v", err)
	}

	if id1 != id2 {
		t.Errorf("UpsertAlbum idempotent: got different IDs %d vs %d", id1, id2)
	}
}

// TestGetAlbumMissingReturnsNil verifies GetAlbum for unknown ID returns (nil, nil).
func TestGetAlbumMissingReturnsNil(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	got, err := s.GetAlbum(ctx, 99999)
	if err != nil {
		t.Fatalf("GetAlbum (missing): %v", err)
	}
	if got != nil {
		t.Error("GetAlbum (missing): expected nil, got non-nil")
	}
}

// TestListAlbumsByArtist verifies that albums are filtered by artist.
func TestListAlbumsByArtist(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	artistID1, _ := s.UpsertArtist(ctx, sampleArtist("David Bowie"))
	artistID2, _ := s.UpsertArtist(ctx, sampleArtist("Iggy Pop"))

	for _, title := range []string{"Ziggy Stardust", "Aladdin Sane", "Heroes"} {
		if _, err := s.UpsertAlbum(ctx, sampleAlbum(title, artistID1)); err != nil {
			t.Fatalf("UpsertAlbum(%q): %v", title, err)
		}
	}
	if _, err := s.UpsertAlbum(ctx, sampleAlbum("The Idiot", artistID2)); err != nil {
		t.Fatalf("UpsertAlbum (Iggy): %v", err)
	}

	albums, err := s.ListAlbumsByArtist(ctx, artistID1)
	if err != nil {
		t.Fatalf("ListAlbumsByArtist: %v", err)
	}
	if len(albums) != 3 {
		t.Errorf("ListAlbumsByArtist: got %d, want 3", len(albums))
	}

	for _, al := range albums {
		if al.ArtistID != artistID1 {
			t.Errorf("album %q has ArtistID %d, want %d", al.Title, al.ArtistID, artistID1)
		}
	}
}

// TestSearchAlbums verifies that SearchAlbums filters by title substring.
func TestSearchAlbums(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	artistID, _ := s.UpsertArtist(ctx, sampleArtist("Various"))

	for _, title := range []string{"Dark Side of the Moon", "The Wall", "Animals"} {
		if _, err := s.UpsertAlbum(ctx, sampleAlbum(title, artistID)); err != nil {
			t.Fatalf("UpsertAlbum(%q): %v", title, err)
		}
	}

	results, err := s.SearchAlbums(ctx, "the", ListOpts{})
	if err != nil {
		t.Fatalf("SearchAlbums: %v", err)
	}
	if len(results) < 1 {
		t.Errorf("SearchAlbums(%q): expected at least 1 result, got %d", "the", len(results))
	}
}

// TestUpsertGetSongRoundtrip verifies Song upsert+get.
func TestUpsertGetSongRoundtrip(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	libID, err := s.CreateLibrary(ctx, sampleLibrary("lib", "/music"))
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	artistID, _ := s.UpsertArtist(ctx, sampleArtist("Queen"))
	albumID, _ := s.UpsertAlbum(ctx, sampleAlbum("Bohemian Rhapsody EP", artistID))

	song := sampleSong("/music/queen/bohemian.mp3", libID, artistID, albumID)
	song.Title = "Bohemian Rhapsody"
	id, err := s.UpsertSong(ctx, song)
	if err != nil {
		t.Fatalf("UpsertSong: %v", err)
	}
	if id <= 0 {
		t.Fatalf("UpsertSong returned non-positive ID: %d", id)
	}

	got, err := s.GetSong(ctx, id)
	if err != nil {
		t.Fatalf("GetSong: %v", err)
	}
	if got == nil {
		t.Fatal("GetSong returned nil for a just-upserted song")
	}
	if got.Title != "Bohemian Rhapsody" {
		t.Errorf("Title: got %q, want %q", got.Title, "Bohemian Rhapsody")
	}
	if got.LibraryID != libID {
		t.Errorf("LibraryID: got %d, want %d", got.LibraryID, libID)
	}
}

// TestGetSongByPath verifies lookup by filesystem path.
func TestGetSongByPath(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	libID, _ := s.CreateLibrary(ctx, sampleLibrary("lib", "/music"))
	artistID, _ := s.UpsertArtist(ctx, sampleArtist("AC/DC"))
	albumID, _ := s.UpsertAlbum(ctx, sampleAlbum("Back in Black", artistID))

	const path = "/music/acdc/back_in_black.mp3"
	song := sampleSong(path, libID, artistID, albumID)
	id, err := s.UpsertSong(ctx, song)
	if err != nil {
		t.Fatalf("UpsertSong: %v", err)
	}

	got, err := s.GetSongByPath(ctx, path)
	if err != nil {
		t.Fatalf("GetSongByPath: %v", err)
	}
	if got == nil {
		t.Fatal("GetSongByPath returned nil")
	}
	if got.ID != id {
		t.Errorf("ID: got %d, want %d", got.ID, id)
	}
}

// TestGetSongByPathMissing verifies that GetSongByPath for an unknown path
// returns (nil, nil).
func TestGetSongByPathMissing(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	got, err := s.GetSongByPath(ctx, "/does/not/exist.mp3")
	if err != nil {
		t.Fatalf("GetSongByPath (missing): %v", err)
	}
	if got != nil {
		t.Error("GetSongByPath (missing): expected nil, got non-nil")
	}
}

// TestListSongsByAlbum verifies that ListSongsByAlbum filters by album.
func TestListSongsByAlbum(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	libID, _ := s.CreateLibrary(ctx, sampleLibrary("lib", "/music"))
	artistID, _ := s.UpsertArtist(ctx, sampleArtist("Eagles"))
	albumID, _ := s.UpsertAlbum(ctx, sampleAlbum("Hotel California", artistID))

	songPaths := []string{
		"/music/eagles/hotel_california.mp3",
		"/music/eagles/new_kid.mp3",
		"/music/eagles/life_in_fast_lane.mp3",
	}
	for _, p := range songPaths {
		song := sampleSong(p, libID, artistID, albumID)
		song.AlbumID = albumID
		if _, err := s.UpsertSong(ctx, song); err != nil {
			t.Fatalf("UpsertSong(%q): %v", p, err)
		}
	}

	songs, err := s.ListSongsByAlbum(ctx, albumID)
	if err != nil {
		t.Fatalf("ListSongsByAlbum: %v", err)
	}
	if len(songs) != len(songPaths) {
		t.Errorf("ListSongsByAlbum: got %d, want %d", len(songs), len(songPaths))
	}
}

// TestSearchSongs verifies that SearchSongs filters by title substring.
func TestSearchSongs(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	libID, _ := s.CreateLibrary(ctx, sampleLibrary("lib", "/music"))
	artistID, _ := s.UpsertArtist(ctx, sampleArtist("Various"))
	albumID, _ := s.UpsertAlbum(ctx, sampleAlbum("Hits", artistID))

	songs := []string{"Stairway to Heaven", "Highway to Hell", "Boulevard of Broken Dreams"}
	for i, title := range songs {
		song := sampleSong("/music/"+title+".mp3", libID, artistID, albumID)
		song.Title = title
		if _, err := s.UpsertSong(ctx, song); err != nil {
			t.Fatalf("UpsertSong[%d]: %v", i, err)
		}
	}

	results, err := s.SearchSongs(ctx, "heaven", ListOpts{})
	if err != nil {
		t.Fatalf("SearchSongs: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("SearchSongs(%q): got %d, want 1", "heaven", len(results))
	}
	if results[0].Title != "Stairway to Heaven" {
		t.Errorf("SearchSongs: got %q, want %q", results[0].Title, "Stairway to Heaven")
	}
}

// TestIncrementPlayCount verifies IncrementPlayCount does not error for an existing song.
func TestIncrementPlayCount(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	libID, _ := s.CreateLibrary(ctx, sampleLibrary("lib", "/music"))
	artistID, _ := s.UpsertArtist(ctx, sampleArtist("Test"))
	albumID, _ := s.UpsertAlbum(ctx, sampleAlbum("Test Album", artistID))

	song := sampleSong("/music/test.mp3", libID, artistID, albumID)
	id, err := s.UpsertSong(ctx, song)
	if err != nil {
		t.Fatalf("UpsertSong: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := s.IncrementPlayCount(ctx, id); err != nil {
			t.Fatalf("IncrementPlayCount (pass %d): %v", i+1, err)
		}
	}
}

// TestMarkSongMissingAndDelete verifies MarkSongMissing + DeleteMissingSongs.
func TestMarkSongMissingAndDelete(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	libID, _ := s.CreateLibrary(ctx, sampleLibrary("lib", "/music"))
	artistID, _ := s.UpsertArtist(ctx, sampleArtist("Test"))
	albumID, _ := s.UpsertAlbum(ctx, sampleAlbum("Test Album", artistID))

	id, err := s.UpsertSong(ctx, sampleSong("/music/track.mp3", libID, artistID, albumID))
	if err != nil {
		t.Fatalf("UpsertSong: %v", err)
	}

	if err := s.MarkSongMissing(ctx, id); err != nil {
		t.Fatalf("MarkSongMissing: %v", err)
	}

	if err := s.DeleteMissingSongs(ctx); err != nil {
		t.Fatalf("DeleteMissingSongs: %v", err)
	}

	got, err := s.GetSong(ctx, id)
	if err != nil {
		t.Fatalf("GetSong after delete: %v", err)
	}
	if got != nil {
		t.Error("GetSong after DeleteMissingSongs: expected nil, got non-nil")
	}
}

// TestListGenresEmpty verifies ListGenres on an empty library returns empty slice.
func TestListGenresEmpty(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	genres, err := s.ListGenres(ctx)
	if err != nil {
		t.Fatalf("ListGenres (empty): %v", err)
	}
	if len(genres) != 0 {
		t.Errorf("ListGenres (empty): got %d, want 0", len(genres))
	}
}

// TestListGenres verifies ListGenres returns genres present in songs.
func TestListGenres(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	libID, _ := s.CreateLibrary(ctx, sampleLibrary("lib", "/music"))
	artistID, _ := s.UpsertArtist(ctx, sampleArtist("Test"))
	albumID, _ := s.UpsertAlbum(ctx, sampleAlbum("Test Album", artistID))

	for i, genre := range []string{"Rock", "Jazz", "Rock"} {
		song := sampleSong("/music/genre"+string(rune('0'+i))+".mp3", libID, artistID, albumID)
		song.Genre = genre
		if _, err := s.UpsertSong(ctx, song); err != nil {
			t.Fatalf("UpsertSong[%d]: %v", i, err)
		}
	}

	genres, err := s.ListGenres(ctx)
	if err != nil {
		t.Fatalf("ListGenres: %v", err)
	}
	if len(genres) < 1 {
		t.Errorf("ListGenres: expected at least 1 genre, got %d", len(genres))
	}
}

// TestGetRandomSongsEmpty verifies GetRandomSongs on an empty library returns empty slice.
func TestGetRandomSongsEmpty(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	songs, err := s.GetRandomSongs(ctx, 10, "", "", "")
	if err != nil {
		t.Fatalf("GetRandomSongs (empty): %v", err)
	}
	if len(songs) != 0 {
		t.Errorf("GetRandomSongs (empty): got %d, want 0", len(songs))
	}
}

// TestGetRandomAlbumsEmpty verifies GetRandomAlbums on an empty library returns empty slice.
func TestGetRandomAlbumsEmpty(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	albums, err := s.GetRandomAlbums(ctx, 10)
	if err != nil {
		t.Fatalf("GetRandomAlbums (empty): %v", err)
	}
	if len(albums) != 0 {
		t.Errorf("GetRandomAlbums (empty): got %d, want 0", len(albums))
	}
}

// TestGetNewestAlbumsEmpty verifies GetNewestAlbums on an empty library returns empty slice.
func TestGetNewestAlbumsEmpty(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	albums, err := s.GetNewestAlbums(ctx, 10)
	if err != nil {
		t.Fatalf("GetNewestAlbums (empty): %v", err)
	}
	if len(albums) != 0 {
		t.Errorf("GetNewestAlbums (empty): got %d, want 0", len(albums))
	}
}

// TestCreateScanStatusRoundtrip verifies scan status insert + get.
func TestCreateScanStatusRoundtrip(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	ss := &model.ScanStatus{
		StartedAt: time.Now().Truncate(time.Second),
		Status:    "running",
	}
	id, err := s.CreateScanStatus(ctx, ss)
	if err != nil {
		t.Fatalf("CreateScanStatus: %v", err)
	}
	if id <= 0 {
		t.Fatalf("CreateScanStatus returned non-positive ID: %d", id)
	}

	got, err := s.GetLastScanStatus(ctx)
	if err != nil {
		t.Fatalf("GetLastScanStatus: %v", err)
	}
	if got == nil {
		t.Fatal("GetLastScanStatus returned nil")
	}
	if got.Status != "running" {
		t.Errorf("Status: got %q, want running", got.Status)
	}
}

// TestGetLastScanStatusNil verifies GetLastScanStatus on an empty store returns nil.
func TestGetLastScanStatusNil(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	got, err := s.GetLastScanStatus(ctx)
	if err != nil {
		t.Fatalf("GetLastScanStatus (empty): %v", err)
	}
	if got != nil {
		t.Error("GetLastScanStatus (empty): expected nil, got non-nil")
	}
}

// TestUpdateScanStatus verifies UpdateScanStatus persists changed fields.
func TestUpdateScanStatus(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	ss := &model.ScanStatus{
		StartedAt: time.Now().Truncate(time.Second),
		Status:    "running",
	}
	id, err := s.CreateScanStatus(ctx, ss)
	if err != nil {
		t.Fatalf("CreateScanStatus: %v", err)
	}

	ss.ID = id
	ss.Status = "completed"
	ss.ScannedFiles = 100
	ss.AddedFiles = 50
	if err := s.UpdateScanStatus(ctx, ss); err != nil {
		t.Fatalf("UpdateScanStatus: %v", err)
	}

	got, err := s.GetLastScanStatus(ctx)
	if err != nil {
		t.Fatalf("GetLastScanStatus after update: %v", err)
	}
	if got.Status != "completed" {
		t.Errorf("Status after update: got %q, want completed", got.Status)
	}
	if got.ScannedFiles != 100 {
		t.Errorf("ScannedFiles after update: got %d, want 100", got.ScannedFiles)
	}
}

// TestDeleteArtistsWithNoSongs verifies that artists without songs are pruned.
func TestDeleteArtistsWithNoSongs(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	// This artist will have no songs.
	artistID, err := s.UpsertArtist(ctx, sampleArtist("Orphaned Artist"))
	if err != nil {
		t.Fatalf("UpsertArtist: %v", err)
	}

	if err := s.DeleteArtistsWithNoSongs(ctx); err != nil {
		t.Fatalf("DeleteArtistsWithNoSongs: %v", err)
	}

	got, err := s.GetArtist(ctx, artistID)
	if err != nil {
		t.Fatalf("GetArtist after prune: %v", err)
	}
	if got != nil {
		t.Error("Artist with no songs should have been pruned")
	}
}

// TestDeleteAlbumsWithNoSongs verifies that albums without songs are pruned.
func TestDeleteAlbumsWithNoSongs(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	artistID, _ := s.UpsertArtist(ctx, sampleArtist("Some Artist"))
	albumID, err := s.UpsertAlbum(ctx, sampleAlbum("Orphaned Album", artistID))
	if err != nil {
		t.Fatalf("UpsertAlbum: %v", err)
	}

	if err := s.DeleteAlbumsWithNoSongs(ctx); err != nil {
		t.Fatalf("DeleteAlbumsWithNoSongs: %v", err)
	}

	got, err := s.GetAlbum(ctx, albumID)
	if err != nil {
		t.Fatalf("GetAlbum after prune: %v", err)
	}
	if got != nil {
		t.Error("Album with no songs should have been pruned")
	}
}

// TestUpsertGetCoverArtRoundtrip verifies CoverArt upsert+get.
func TestUpsertGetCoverArtRoundtrip(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	ca := &model.CoverArt{
		MimeType: "image/jpeg",
		Width:    300,
		Height:   300,
		Data:     []byte{0xFF, 0xD8, 0xFF},
	}
	id, err := s.UpsertCoverArt(ctx, ca)
	if err != nil {
		t.Fatalf("UpsertCoverArt: %v", err)
	}
	if id <= 0 {
		t.Fatalf("UpsertCoverArt returned non-positive ID: %d", id)
	}

	got, err := s.GetCoverArt(ctx, id)
	if err != nil {
		t.Fatalf("GetCoverArt: %v", err)
	}
	if got == nil {
		t.Fatal("GetCoverArt returned nil")
	}
	if got.MimeType != "image/jpeg" {
		t.Errorf("MimeType: got %q, want image/jpeg", got.MimeType)
	}
}

// TestListAlbums verifies that ListAlbums returns all albums.
func TestListAlbums(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	artistID, _ := s.UpsertArtist(ctx, sampleArtist("Multi Album Artist"))

	for _, title := range []string{"Album One", "Album Two", "Album Three"} {
		if _, err := s.UpsertAlbum(ctx, sampleAlbum(title, artistID)); err != nil {
			t.Fatalf("UpsertAlbum(%q): %v", title, err)
		}
	}

	albums, err := s.ListAlbums(ctx, ListOpts{})
	if err != nil {
		t.Fatalf("ListAlbums: %v", err)
	}
	if len(albums) != 3 {
		t.Errorf("ListAlbums: got %d, want 3", len(albums))
	}
}

// TestGetSongMissingReturnsNil verifies that GetSong for an unknown ID
// returns (nil, nil).
func TestGetSongMissingReturnsNil(t *testing.T) {
	s := newTestMusicStore(t)
	ctx := context.Background()

	got, err := s.GetSong(ctx, 99999)
	if err != nil {
		t.Fatalf("GetSong (missing): %v", err)
	}
	if got != nil {
		t.Error("GetSong (missing): expected nil, got non-nil")
	}
}
