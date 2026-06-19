package store

import (
	"context"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
)

// newTestActivityStore returns an in-memory SQLite ActivityStore with the full
// server schema applied. The database is closed automatically via t.Cleanup.
func newTestActivityStore(t *testing.T) ActivityStore {
	t.Helper()
	db, err := openDB(":memory:", serverSchema)
	if err != nil {
		t.Fatalf("openDB :memory:: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &sqliteActivityStore{db: db}
}

// newTestMusicStore returns a sqliteMusicStore backed by the same in-memory
// database as the given ActivityStore. This is used to seed artists, albums,
// and libraries before testing GetStarred joins.
func newTestMusicStoreFromActivity(t *testing.T, a ActivityStore) *sqliteMusicStore {
	t.Helper()
	as, ok := a.(*sqliteActivityStore)
	if !ok {
		t.Fatal("ActivityStore is not *sqliteActivityStore")
	}
	return &sqliteMusicStore{db: as.db}
}

// seedLibrary inserts a minimal library row and returns its ID. Songs require a
// valid library_id foreign key, so a library must exist before inserting songs.
func seedLibrary(t *testing.T, ms *sqliteMusicStore) int64 {
	t.Helper()
	lib := &model.Library{
		Name:    "Test Library",
		Path:    "/tmp/test-lib",
		Enabled: true,
	}
	id, err := ms.CreateLibrary(context.Background(), lib)
	if err != nil {
		t.Fatalf("seedLibrary: %v", err)
	}
	return id
}

// seedArtist inserts a minimal artist and returns its ID.
func seedArtist(t *testing.T, ms *sqliteMusicStore, name string) int64 {
	t.Helper()
	id, err := ms.UpsertArtist(context.Background(), &model.Artist{Name: name})
	if err != nil {
		t.Fatalf("seedArtist(%q): %v", name, err)
	}
	return id
}

// seedAlbum inserts a minimal album and returns its ID.
func seedAlbum(t *testing.T, ms *sqliteMusicStore, title string, artistID int64) int64 {
	t.Helper()
	id, err := ms.UpsertAlbum(context.Background(), &model.Album{
		Title:    title,
		ArtistID: artistID,
	})
	if err != nil {
		t.Fatalf("seedAlbum(%q): %v", title, err)
	}
	return id
}

// seedSong inserts a minimal song and returns its ID.
func seedSong(t *testing.T, ms *sqliteMusicStore, title string, libraryID, artistID, albumID int64) int64 {
	t.Helper()
	id, err := ms.UpsertSong(context.Background(), &model.Song{
		LibraryID: libraryID,
		Path:      "/tmp/test-lib/" + title + ".mp3",
		Title:     title,
		ArtistID:  artistID,
		AlbumID:   albumID,
	})
	if err != nil {
		t.Fatalf("seedSong(%q): %v", title, err)
	}
	return id
}

// TestStarIsStarredSong verifies that starring a song and checking IsStarred
// returns true for the itemType "song".
func TestStarIsStarredSong(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	const userID int64 = 1
	const songID int64 = 42

	if err := s.Star(ctx, userID, "song", songID); err != nil {
		t.Fatalf("Star song: %v", err)
	}

	ok, err := s.IsStarred(ctx, userID, "song", songID)
	if err != nil {
		t.Fatalf("IsStarred song: %v", err)
	}
	if !ok {
		t.Error("IsStarred song: got false, want true after Star")
	}
}

// TestStarIsStarredAlbum verifies the Star/IsStarred roundtrip for itemType "album".
func TestStarIsStarredAlbum(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	const userID int64 = 1
	const albumID int64 = 10

	if err := s.Star(ctx, userID, "album", albumID); err != nil {
		t.Fatalf("Star album: %v", err)
	}

	ok, err := s.IsStarred(ctx, userID, "album", albumID)
	if err != nil {
		t.Fatalf("IsStarred album: %v", err)
	}
	if !ok {
		t.Error("IsStarred album: got false, want true after Star")
	}
}

// TestStarIsStarredArtist verifies the Star/IsStarred roundtrip for itemType "artist".
func TestStarIsStarredArtist(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	const userID int64 = 1
	const artistID int64 = 5

	if err := s.Star(ctx, userID, "artist", artistID); err != nil {
		t.Fatalf("Star artist: %v", err)
	}

	ok, err := s.IsStarred(ctx, userID, "artist", artistID)
	if err != nil {
		t.Fatalf("IsStarred artist: %v", err)
	}
	if !ok {
		t.Error("IsStarred artist: got false, want true after Star")
	}
}

// TestUnstarRemovesStar verifies that Unstar removes the star and IsStarred
// subsequently returns false.
func TestUnstarRemovesStar(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	const userID int64 = 1
	const songID int64 = 7

	if err := s.Star(ctx, userID, "song", songID); err != nil {
		t.Fatalf("Star: %v", err)
	}
	if err := s.Unstar(ctx, userID, "song", songID); err != nil {
		t.Fatalf("Unstar: %v", err)
	}

	ok, err := s.IsStarred(ctx, userID, "song", songID)
	if err != nil {
		t.Fatalf("IsStarred after Unstar: %v", err)
	}
	if ok {
		t.Error("IsStarred after Unstar: got true, want false")
	}
}

// TestUnstarNonExistentNoError verifies that calling Unstar on an item that was
// never starred does not return an error (idempotent operation).
func TestUnstarNonExistentNoError(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	if err := s.Unstar(ctx, 1, "song", 999); err != nil {
		t.Errorf("Unstar (non-existent): expected no error, got %v", err)
	}
}

// TestIsStarredFalseForUnstarredItem verifies that IsStarred returns false for
// an item that has never been starred.
func TestIsStarredFalseForUnstarredItem(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	ok, err := s.IsStarred(ctx, 1, "song", 100)
	if err != nil {
		t.Fatalf("IsStarred (unstarred): %v", err)
	}
	if ok {
		t.Error("IsStarred (unstarred): got true, want false")
	}
}

// TestStarIdempotent verifies that starring the same item twice does not return
// an error and leaves the item starred.
func TestStarIdempotent(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	const userID int64 = 1
	const songID int64 = 3

	if err := s.Star(ctx, userID, "song", songID); err != nil {
		t.Fatalf("Star (first): %v", err)
	}
	if err := s.Star(ctx, userID, "song", songID); err != nil {
		t.Fatalf("Star (second): %v", err)
	}

	ok, err := s.IsStarred(ctx, userID, "song", songID)
	if err != nil {
		t.Fatalf("IsStarred after double Star: %v", err)
	}
	if !ok {
		t.Error("IsStarred after double Star: got false, want true")
	}
}

// TestGetStarredReturnsCorrectTypes verifies that GetStarred correctly separates
// starred items into Songs, Albums, and Artists buckets via the JOIN queries.
func TestGetStarredReturnsCorrectTypes(t *testing.T) {
	s := newTestActivityStore(t)
	ms := newTestMusicStoreFromActivity(t, s)
	ctx := context.Background()

	libID := seedLibrary(t, ms)
	artistID := seedArtist(t, ms, "Test Artist")
	albumID := seedAlbum(t, ms, "Test Album", artistID)
	songID := seedSong(t, ms, "Test Song", libID, artistID, albumID)

	const userID int64 = 1

	if err := s.Star(ctx, userID, "song", songID); err != nil {
		t.Fatalf("Star song: %v", err)
	}
	if err := s.Star(ctx, userID, "album", albumID); err != nil {
		t.Fatalf("Star album: %v", err)
	}
	if err := s.Star(ctx, userID, "artist", artistID); err != nil {
		t.Fatalf("Star artist: %v", err)
	}

	starred, err := s.GetStarred(ctx, userID)
	if err != nil {
		t.Fatalf("GetStarred: %v", err)
	}
	if starred == nil {
		t.Fatal("GetStarred returned nil")
	}

	if len(starred.Songs) != 1 {
		t.Errorf("GetStarred.Songs: got %d, want 1", len(starred.Songs))
	} else if starred.Songs[0].ID != songID {
		t.Errorf("GetStarred.Songs[0].ID: got %d, want %d", starred.Songs[0].ID, songID)
	}

	if len(starred.Albums) != 1 {
		t.Errorf("GetStarred.Albums: got %d, want 1", len(starred.Albums))
	} else if starred.Albums[0].ID != albumID {
		t.Errorf("GetStarred.Albums[0].ID: got %d, want %d", starred.Albums[0].ID, albumID)
	}

	if len(starred.Artists) != 1 {
		t.Errorf("GetStarred.Artists: got %d, want 1", len(starred.Artists))
	} else if starred.Artists[0].ID != artistID {
		t.Errorf("GetStarred.Artists[0].ID: got %d, want %d", starred.Artists[0].ID, artistID)
	}
}

// TestGetStarredEmptyWhenNothingStarred verifies that GetStarred returns an
// empty result (not an error) when the user has not starred anything.
func TestGetStarredEmptyWhenNothingStarred(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	starred, err := s.GetStarred(ctx, 1)
	if err != nil {
		t.Fatalf("GetStarred (empty): %v", err)
	}
	if starred == nil {
		t.Fatal("GetStarred returned nil, want empty *StarredItems")
	}
	if len(starred.Songs) != 0 {
		t.Errorf("GetStarred.Songs: got %d, want 0", len(starred.Songs))
	}
	if len(starred.Albums) != 0 {
		t.Errorf("GetStarred.Albums: got %d, want 0", len(starred.Albums))
	}
	if len(starred.Artists) != 0 {
		t.Errorf("GetStarred.Artists: got %d, want 0", len(starred.Artists))
	}
}

// TestGetStarredIsolatedByUser verifies that stars belonging to one user do not
// appear in GetStarred results for a different user.
func TestGetStarredIsolatedByUser(t *testing.T) {
	s := newTestActivityStore(t)
	ms := newTestMusicStoreFromActivity(t, s)
	ctx := context.Background()

	libID := seedLibrary(t, ms)
	artistID := seedArtist(t, ms, "Shared Artist")
	albumID := seedAlbum(t, ms, "Shared Album", artistID)
	songID := seedSong(t, ms, "Shared Song", libID, artistID, albumID)

	const userA int64 = 1
	const userB int64 = 2

	if err := s.Star(ctx, userA, "song", songID); err != nil {
		t.Fatalf("Star (userA): %v", err)
	}

	starred, err := s.GetStarred(ctx, userB)
	if err != nil {
		t.Fatalf("GetStarred (userB): %v", err)
	}
	if len(starred.Songs) != 0 {
		t.Errorf("GetStarred for userB sees userA song: got %d songs, want 0", len(starred.Songs))
	}
}

// TestSetRatingGetRatingRoundtrip verifies that a rating saved with SetRating
// is returned exactly by GetRating.
func TestSetRatingGetRatingRoundtrip(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	const userID int64 = 1
	const songID int64 = 55
	const rating = 4

	if err := s.SetRating(ctx, userID, "song", songID, rating); err != nil {
		t.Fatalf("SetRating: %v", err)
	}

	got, err := s.GetRating(ctx, userID, "song", songID)
	if err != nil {
		t.Fatalf("GetRating: %v", err)
	}
	if got != rating {
		t.Errorf("GetRating: got %d, want %d", got, rating)
	}
}

// TestGetRatingReturnsZeroForUnratedItem verifies that GetRating returns 0
// without error when no rating exists for the specified user+item.
func TestGetRatingReturnsZeroForUnratedItem(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	got, err := s.GetRating(ctx, 1, "song", 999)
	if err != nil {
		t.Fatalf("GetRating (unrated): %v", err)
	}
	if got != 0 {
		t.Errorf("GetRating (unrated): got %d, want 0", got)
	}
}

// TestSetRatingUpdatesExistingRating verifies that calling SetRating a second
// time replaces the previous value without creating a duplicate row.
func TestSetRatingUpdatesExistingRating(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	const userID int64 = 1
	const songID int64 = 20

	if err := s.SetRating(ctx, userID, "song", songID, 2); err != nil {
		t.Fatalf("SetRating (first): %v", err)
	}
	if err := s.SetRating(ctx, userID, "song", songID, 5); err != nil {
		t.Fatalf("SetRating (update): %v", err)
	}

	got, err := s.GetRating(ctx, userID, "song", songID)
	if err != nil {
		t.Fatalf("GetRating after update: %v", err)
	}
	if got != 5 {
		t.Errorf("GetRating after update: got %d, want 5", got)
	}
}

// TestSetRatingAlbumType verifies SetRating/GetRating works for itemType "album".
func TestSetRatingAlbumType(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	const userID int64 = 1
	const albumID int64 = 8

	if err := s.SetRating(ctx, userID, "album", albumID, 3); err != nil {
		t.Fatalf("SetRating album: %v", err)
	}

	got, err := s.GetRating(ctx, userID, "album", albumID)
	if err != nil {
		t.Fatalf("GetRating album: %v", err)
	}
	if got != 3 {
		t.Errorf("GetRating album: got %d, want 3", got)
	}
}

// TestRecordPlayGetPlayHistoryRoundtrip verifies that a play event written via
// RecordPlay is returned by GetPlayHistory with all fields intact.
func TestRecordPlayGetPlayHistoryRoundtrip(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	playedAt := time.Now().Truncate(time.Second).UTC()
	h := &model.PlayHistory{
		UserID:      1,
		SongID:      100,
		PlayedAt:    playedAt,
		ListenedFor: 180,
		ClientName:  "cassonic-cli",
		Scrobbled:   true,
	}

	if err := s.RecordPlay(ctx, h); err != nil {
		t.Fatalf("RecordPlay: %v", err)
	}

	history, err := s.GetPlayHistory(ctx, 1, 10)
	if err != nil {
		t.Fatalf("GetPlayHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("GetPlayHistory: got %d entries, want 1", len(history))
	}

	got := history[0]
	if got.UserID != h.UserID {
		t.Errorf("UserID: got %d, want %d", got.UserID, h.UserID)
	}
	if got.SongID != h.SongID {
		t.Errorf("SongID: got %d, want %d", got.SongID, h.SongID)
	}
	if got.ListenedFor != h.ListenedFor {
		t.Errorf("ListenedFor: got %d, want %d", got.ListenedFor, h.ListenedFor)
	}
	if got.ClientName != h.ClientName {
		t.Errorf("ClientName: got %q, want %q", got.ClientName, h.ClientName)
	}
	if got.Scrobbled != h.Scrobbled {
		t.Errorf("Scrobbled: got %v, want %v", got.Scrobbled, h.Scrobbled)
	}
}

// TestGetPlayHistoryRespectsLimit verifies that when more play events exist than
// the requested limit, only limit entries are returned.
func TestGetPlayHistoryRespectsLimit(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	const userID int64 = 1

	for i := 0; i < 5; i++ {
		h := &model.PlayHistory{
			UserID:   userID,
			SongID:   int64(i + 1),
			PlayedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := s.RecordPlay(ctx, h); err != nil {
			t.Fatalf("RecordPlay[%d]: %v", i, err)
		}
	}

	history, err := s.GetPlayHistory(ctx, userID, 3)
	if err != nil {
		t.Fatalf("GetPlayHistory: %v", err)
	}
	if len(history) != 3 {
		t.Errorf("GetPlayHistory limit=3: got %d entries, want 3", len(history))
	}
}

// TestGetPlayHistoryNewestFirst verifies that GetPlayHistory returns entries
// ordered by played_at descending (newest first).
func TestGetPlayHistoryNewestFirst(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	const userID int64 = 1
	base := time.Now().Truncate(time.Second).UTC()

	for i := 0; i < 3; i++ {
		h := &model.PlayHistory{
			UserID:   userID,
			SongID:   int64(i + 1),
			PlayedAt: base.Add(time.Duration(i) * time.Second),
		}
		if err := s.RecordPlay(ctx, h); err != nil {
			t.Fatalf("RecordPlay[%d]: %v", i, err)
		}
	}

	history, err := s.GetPlayHistory(ctx, userID, 10)
	if err != nil {
		t.Fatalf("GetPlayHistory: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("GetPlayHistory: got %d, want 3", len(history))
	}

	for i := 1; i < len(history); i++ {
		if history[i-1].PlayedAt.Before(history[i].PlayedAt) {
			t.Errorf("GetPlayHistory not sorted newest-first at index %d", i)
		}
	}
}

// TestGetPlayHistoryEmptyForNewUser verifies that GetPlayHistory returns nil or
// an empty slice (no error) when the user has no play events.
func TestGetPlayHistoryEmptyForNewUser(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	history, err := s.GetPlayHistory(ctx, 99, 10)
	if err != nil {
		t.Fatalf("GetPlayHistory (no events): %v", err)
	}
	if len(history) != 0 {
		t.Errorf("GetPlayHistory (no events): got %d entries, want 0", len(history))
	}
}

// TestSetBookmarkGetBookmarksRoundtrip verifies that a bookmark written via
// SetBookmark appears in GetBookmarks with correct fields.
func TestSetBookmarkGetBookmarksRoundtrip(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	b := &model.Bookmark{
		UserID:   1,
		ItemType: "song",
		ItemID:   77,
		Position: 30000,
		Comment:  "resume here",
	}

	if err := s.SetBookmark(ctx, b); err != nil {
		t.Fatalf("SetBookmark: %v", err)
	}

	bookmarks, err := s.GetBookmarks(ctx, 1)
	if err != nil {
		t.Fatalf("GetBookmarks: %v", err)
	}
	if len(bookmarks) != 1 {
		t.Fatalf("GetBookmarks: got %d, want 1", len(bookmarks))
	}

	got := bookmarks[0]
	if got.UserID != b.UserID {
		t.Errorf("UserID: got %d, want %d", got.UserID, b.UserID)
	}
	if got.ItemType != b.ItemType {
		t.Errorf("ItemType: got %q, want %q", got.ItemType, b.ItemType)
	}
	if got.ItemID != b.ItemID {
		t.Errorf("ItemID: got %d, want %d", got.ItemID, b.ItemID)
	}
	if got.Position != b.Position {
		t.Errorf("Position: got %d, want %d", got.Position, b.Position)
	}
	if got.Comment != b.Comment {
		t.Errorf("Comment: got %q, want %q", got.Comment, b.Comment)
	}
}

// TestSetBookmarkUpsertSemantics verifies that calling SetBookmark a second time
// for the same user+itemType+itemID replaces the bookmark rather than inserting
// a duplicate.
func TestSetBookmarkUpsertSemantics(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	b := &model.Bookmark{
		UserID:   1,
		ItemType: "song",
		ItemID:   50,
		Position: 5000,
		Comment:  "first save",
	}

	if err := s.SetBookmark(ctx, b); err != nil {
		t.Fatalf("SetBookmark (first): %v", err)
	}

	b.Position = 12000
	b.Comment = "second save"

	if err := s.SetBookmark(ctx, b); err != nil {
		t.Fatalf("SetBookmark (second): %v", err)
	}

	bookmarks, err := s.GetBookmarks(ctx, 1)
	if err != nil {
		t.Fatalf("GetBookmarks after upsert: %v", err)
	}
	if len(bookmarks) != 1 {
		t.Fatalf("GetBookmarks after upsert: got %d, want 1 (not a duplicate)", len(bookmarks))
	}
	if bookmarks[0].Position != 12000 {
		t.Errorf("Position after upsert: got %d, want 12000", bookmarks[0].Position)
	}
	if bookmarks[0].Comment != "second save" {
		t.Errorf("Comment after upsert: got %q, want %q", bookmarks[0].Comment, "second save")
	}
}

// TestDeleteBookmarkRemovesEntry verifies that DeleteBookmark removes the target
// bookmark and leaves GetBookmarks returning an empty slice.
func TestDeleteBookmarkRemovesEntry(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	b := &model.Bookmark{
		UserID:   1,
		ItemType: "episode",
		ItemID:   9,
		Position: 60000,
	}

	if err := s.SetBookmark(ctx, b); err != nil {
		t.Fatalf("SetBookmark: %v", err)
	}
	if err := s.DeleteBookmark(ctx, 1, "episode", 9); err != nil {
		t.Fatalf("DeleteBookmark: %v", err)
	}

	bookmarks, err := s.GetBookmarks(ctx, 1)
	if err != nil {
		t.Fatalf("GetBookmarks after delete: %v", err)
	}
	if len(bookmarks) != 0 {
		t.Errorf("GetBookmarks after delete: got %d entries, want 0", len(bookmarks))
	}
}

// TestDeleteBookmarkNonExistentNoError verifies that deleting a bookmark that
// does not exist returns no error (idempotent delete).
func TestDeleteBookmarkNonExistentNoError(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	if err := s.DeleteBookmark(ctx, 1, "song", 999); err != nil {
		t.Errorf("DeleteBookmark (non-existent): expected no error, got %v", err)
	}
}

// TestGetBookmarksEmptyForNewUser verifies that GetBookmarks returns nil or an
// empty slice (no error) when the user has no bookmarks.
func TestGetBookmarksEmptyForNewUser(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	bookmarks, err := s.GetBookmarks(ctx, 99)
	if err != nil {
		t.Fatalf("GetBookmarks (no bookmarks): %v", err)
	}
	if len(bookmarks) != 0 {
		t.Errorf("GetBookmarks (no bookmarks): got %d, want 0", len(bookmarks))
	}
}

// TestSavePlayQueueGetPlayQueueRoundtrip verifies that a play queue and its
// entries written via SavePlayQueue are returned intact by GetPlayQueue.
func TestSavePlayQueueGetPlayQueueRoundtrip(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	pq := &model.PlayQueue{
		UserID:    1,
		Current:   200,
		Position:  45000,
		ChangedBy: "cassonic-cli",
	}
	entries := []*model.PlayQueueEntry{
		{SongID: 200, Position: 0},
		{SongID: 201, Position: 1},
		{SongID: 202, Position: 2},
	}

	if err := s.SavePlayQueue(ctx, pq, entries); err != nil {
		t.Fatalf("SavePlayQueue: %v", err)
	}

	gotPQ, gotEntries, err := s.GetPlayQueue(ctx, 1)
	if err != nil {
		t.Fatalf("GetPlayQueue: %v", err)
	}
	if gotPQ == nil {
		t.Fatal("GetPlayQueue returned nil queue, want non-nil")
	}

	if gotPQ.UserID != pq.UserID {
		t.Errorf("PlayQueue.UserID: got %d, want %d", gotPQ.UserID, pq.UserID)
	}
	if gotPQ.Current != pq.Current {
		t.Errorf("PlayQueue.Current: got %d, want %d", gotPQ.Current, pq.Current)
	}
	if gotPQ.Position != pq.Position {
		t.Errorf("PlayQueue.Position: got %d, want %d", gotPQ.Position, pq.Position)
	}
	if gotPQ.ChangedBy != pq.ChangedBy {
		t.Errorf("PlayQueue.ChangedBy: got %q, want %q", gotPQ.ChangedBy, pq.ChangedBy)
	}

	if len(gotEntries) != len(entries) {
		t.Fatalf("PlayQueueEntries: got %d, want %d", len(gotEntries), len(entries))
	}
	for i, e := range entries {
		if gotEntries[i].SongID != e.SongID {
			t.Errorf("Entry[%d].SongID: got %d, want %d", i, gotEntries[i].SongID, e.SongID)
		}
		if gotEntries[i].Position != e.Position {
			t.Errorf("Entry[%d].Position: got %d, want %d", i, gotEntries[i].Position, e.Position)
		}
	}
}

// TestSavePlayQueueReplacesEntries verifies that calling SavePlayQueue a second
// time with a different entry list replaces the old entries atomically.
func TestSavePlayQueueReplacesEntries(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	pq := &model.PlayQueue{
		UserID:    1,
		Current:   300,
		Position:  0,
		ChangedBy: "web",
	}

	initialEntries := []*model.PlayQueueEntry{
		{SongID: 300, Position: 0},
		{SongID: 301, Position: 1},
	}

	if err := s.SavePlayQueue(ctx, pq, initialEntries); err != nil {
		t.Fatalf("SavePlayQueue (initial): %v", err)
	}

	pq.Current = 400
	pq.Position = 10000
	updatedEntries := []*model.PlayQueueEntry{
		{SongID: 400, Position: 0},
	}

	if err := s.SavePlayQueue(ctx, pq, updatedEntries); err != nil {
		t.Fatalf("SavePlayQueue (update): %v", err)
	}

	gotPQ, gotEntries, err := s.GetPlayQueue(ctx, 1)
	if err != nil {
		t.Fatalf("GetPlayQueue after update: %v", err)
	}
	if gotPQ == nil {
		t.Fatal("GetPlayQueue after update returned nil queue")
	}
	if gotPQ.Current != 400 {
		t.Errorf("PlayQueue.Current after update: got %d, want 400", gotPQ.Current)
	}
	if len(gotEntries) != 1 {
		t.Errorf("PlayQueueEntries after update: got %d, want 1", len(gotEntries))
	} else if gotEntries[0].SongID != 400 {
		t.Errorf("PlayQueueEntries[0].SongID after update: got %d, want 400", gotEntries[0].SongID)
	}
}

// TestSavePlayQueueEmptyEntries verifies that SavePlayQueue with an empty entry
// list is valid and results in GetPlayQueue returning an empty entry slice.
func TestSavePlayQueueEmptyEntries(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	pq := &model.PlayQueue{
		UserID:    1,
		Current:   0,
		Position:  0,
		ChangedBy: "cassonic-cli",
	}

	if err := s.SavePlayQueue(ctx, pq, nil); err != nil {
		t.Fatalf("SavePlayQueue (empty entries): %v", err)
	}

	gotPQ, gotEntries, err := s.GetPlayQueue(ctx, 1)
	if err != nil {
		t.Fatalf("GetPlayQueue (empty entries): %v", err)
	}
	if gotPQ == nil {
		t.Fatal("GetPlayQueue (empty entries): returned nil queue, want non-nil")
	}
	if len(gotEntries) != 0 {
		t.Errorf("PlayQueueEntries (empty): got %d, want 0", len(gotEntries))
	}
}

// TestGetPlayQueueReturnsNilWhenNoQueueExists verifies that GetPlayQueue returns
// nil, nil, nil when no play queue has been saved for the user.
func TestGetPlayQueueReturnsNilWhenNoQueueExists(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	gotPQ, gotEntries, err := s.GetPlayQueue(ctx, 99)
	if err != nil {
		t.Fatalf("GetPlayQueue (no queue): %v", err)
	}
	if gotPQ != nil {
		t.Errorf("GetPlayQueue (no queue): got non-nil PlayQueue, want nil")
	}
	if gotEntries != nil {
		t.Errorf("GetPlayQueue (no queue): got non-nil entries, want nil")
	}
}

// TestPlayQueueIsolatedByUser verifies that two users each get their own
// independent play queue with no cross-contamination.
func TestPlayQueueIsolatedByUser(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	pqA := &model.PlayQueue{UserID: 1, Current: 101, ChangedBy: "clientA"}
	pqB := &model.PlayQueue{UserID: 2, Current: 202, ChangedBy: "clientB"}

	if err := s.SavePlayQueue(ctx, pqA, []*model.PlayQueueEntry{{SongID: 101, Position: 0}}); err != nil {
		t.Fatalf("SavePlayQueue (userA): %v", err)
	}
	if err := s.SavePlayQueue(ctx, pqB, []*model.PlayQueueEntry{{SongID: 202, Position: 0}}); err != nil {
		t.Fatalf("SavePlayQueue (userB): %v", err)
	}

	gotA, entriesA, err := s.GetPlayQueue(ctx, 1)
	if err != nil {
		t.Fatalf("GetPlayQueue (userA): %v", err)
	}
	gotB, entriesB, err := s.GetPlayQueue(ctx, 2)
	if err != nil {
		t.Fatalf("GetPlayQueue (userB): %v", err)
	}

	if gotA == nil || gotA.Current != 101 {
		t.Errorf("GetPlayQueue (userA): Current got %v, want 101", gotA)
	}
	if gotB == nil || gotB.Current != 202 {
		t.Errorf("GetPlayQueue (userB): Current got %v, want 202", gotB)
	}
	if len(entriesA) != 1 || entriesA[0].SongID != 101 {
		t.Errorf("GetPlayQueue (userA) entries: got %v, want [{SongID:101}]", entriesA)
	}
	if len(entriesB) != 1 || entriesB[0].SongID != 202 {
		t.Errorf("GetPlayQueue (userB) entries: got %v, want [{SongID:202}]", entriesB)
	}
}

// TestBookmarkEpisodeType verifies SetBookmark/GetBookmarks works for itemType
// "episode" in addition to the "song" type.
func TestBookmarkEpisodeType(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	b := &model.Bookmark{
		UserID:   1,
		ItemType: "episode",
		ItemID:   15,
		Position: 120000,
		Comment:  "halfway through",
	}

	if err := s.SetBookmark(ctx, b); err != nil {
		t.Fatalf("SetBookmark (episode): %v", err)
	}

	bookmarks, err := s.GetBookmarks(ctx, 1)
	if err != nil {
		t.Fatalf("GetBookmarks: %v", err)
	}
	if len(bookmarks) != 1 {
		t.Fatalf("GetBookmarks: got %d, want 1", len(bookmarks))
	}
	if bookmarks[0].ItemType != "episode" {
		t.Errorf("ItemType: got %q, want %q", bookmarks[0].ItemType, "episode")
	}
	if bookmarks[0].Position != 120000 {
		t.Errorf("Position: got %d, want 120000", bookmarks[0].Position)
	}
}

// TestMultipleBookmarksDifferentItems verifies that a user can hold bookmarks for
// multiple distinct items simultaneously and GetBookmarks returns all of them.
func TestMultipleBookmarksDifferentItems(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	bookmarkData := []struct {
		itemType string
		itemID   int64
		position int64
	}{
		{"song", 1, 10000},
		{"song", 2, 20000},
		{"episode", 3, 30000},
	}

	for _, bd := range bookmarkData {
		if err := s.SetBookmark(ctx, &model.Bookmark{
			UserID:   1,
			ItemType: bd.itemType,
			ItemID:   bd.itemID,
			Position: bd.position,
		}); err != nil {
			t.Fatalf("SetBookmark(%s/%d): %v", bd.itemType, bd.itemID, err)
		}
	}

	bookmarks, err := s.GetBookmarks(ctx, 1)
	if err != nil {
		t.Fatalf("GetBookmarks: %v", err)
	}
	if len(bookmarks) != len(bookmarkData) {
		t.Errorf("GetBookmarks: got %d, want %d", len(bookmarks), len(bookmarkData))
	}
}

// TestRecordPlayZeroPlayedAt verifies that RecordPlay with a zero-value PlayedAt
// stores the current time without error, rather than panicking or corrupting the row.
func TestRecordPlayZeroPlayedAt(t *testing.T) {
	s := newTestActivityStore(t)
	ctx := context.Background()

	h := &model.PlayHistory{
		UserID: 1,
		SongID: 10,
	}

	if err := s.RecordPlay(ctx, h); err != nil {
		t.Fatalf("RecordPlay (zero PlayedAt): %v", err)
	}

	history, err := s.GetPlayHistory(ctx, 1, 1)
	if err != nil {
		t.Fatalf("GetPlayHistory after zero PlayedAt: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("GetPlayHistory after zero PlayedAt: got %d entries, want 1", len(history))
	}
}
