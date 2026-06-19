package store

import (
	"context"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// newTestPlaylistStore creates an in-memory SQLite PlaylistStore with the full server schema applied.
// Each call gets its own isolated DB so tests cannot interfere with each other.
func newTestPlaylistStore(t *testing.T) PlaylistStore {
	t.Helper()
	db, err := openDB(":memory:", serverSchema)
	if err != nil {
		t.Fatalf("openDB :memory:: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &sqlitePlaylistStore{db: db}
}

// samplePlaylist returns a minimal valid model.Playlist for the given user.
func samplePlaylist(userID int64, name string, isPublic bool) *model.Playlist {
	return &model.Playlist{
		UserID:   userID,
		Name:     name,
		Comment:  "test comment",
		IsPublic: isPublic,
	}
}

// TestCreateGetPlaylistRoundtrip verifies that a playlist inserted via CreatePlaylist
// can be retrieved by its ID with all fields intact.
func TestCreateGetPlaylistRoundtrip(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	p := samplePlaylist(1, "My Mix", false)
	p.Comment = "a great mix"
	p.CoverArtID = 42

	id, err := s.CreatePlaylist(ctx, p)
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	if id <= 0 {
		t.Fatalf("CreatePlaylist returned non-positive ID: %d", id)
	}

	got, err := s.GetPlaylist(ctx, id)
	if err != nil {
		t.Fatalf("GetPlaylist: %v", err)
	}
	if got == nil {
		t.Fatal("GetPlaylist returned nil for a just-created playlist")
	}

	if got.ID != id {
		t.Errorf("ID: got %d, want %d", got.ID, id)
	}
	if got.UserID != p.UserID {
		t.Errorf("UserID: got %d, want %d", got.UserID, p.UserID)
	}
	if got.Name != p.Name {
		t.Errorf("Name: got %q, want %q", got.Name, p.Name)
	}
	if got.Comment != p.Comment {
		t.Errorf("Comment: got %q, want %q", got.Comment, p.Comment)
	}
	if got.IsPublic != p.IsPublic {
		t.Errorf("IsPublic: got %v, want %v", got.IsPublic, p.IsPublic)
	}
	if got.CoverArtID != p.CoverArtID {
		t.Errorf("CoverArtID: got %d, want %d", got.CoverArtID, p.CoverArtID)
	}
}

// TestGetPlaylistMissingIDReturnsNil verifies that querying a non-existent playlist ID
// returns (nil, nil) — absence, not an error.
func TestGetPlaylistMissingIDReturnsNil(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	got, err := s.GetPlaylist(ctx, 99999)
	if err != nil {
		t.Fatalf("GetPlaylist (missing): unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("GetPlaylist (missing): got non-nil playlist, want nil")
	}
}

// TestListPlaylistsVisibility verifies the visibility rules: a user sees their own
// playlists (public and private) plus any other user's public playlists, but never
// another user's private playlists.
func TestListPlaylistsVisibility(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	const (
		ownerID int64 = 1
		otherID int64 = 2
	)

	// Owner's private playlist — must appear in owner's list.
	ownerPrivateID, err := s.CreatePlaylist(ctx, samplePlaylist(ownerID, "Owner Private", false))
	if err != nil {
		t.Fatalf("CreatePlaylist (owner private): %v", err)
	}

	// Owner's public playlist — must appear in owner's and other's list.
	ownerPublicID, err := s.CreatePlaylist(ctx, samplePlaylist(ownerID, "Owner Public", true))
	if err != nil {
		t.Fatalf("CreatePlaylist (owner public): %v", err)
	}

	// Other user's public playlist — must appear in owner's list.
	otherPublicID, err := s.CreatePlaylist(ctx, samplePlaylist(otherID, "Other Public", true))
	if err != nil {
		t.Fatalf("CreatePlaylist (other public): %v", err)
	}

	// Other user's private playlist — must NOT appear in owner's list.
	otherPrivateID, err := s.CreatePlaylist(ctx, samplePlaylist(otherID, "Other Private", false))
	if err != nil {
		t.Fatalf("CreatePlaylist (other private): %v", err)
	}

	ownerList, err := s.ListPlaylists(ctx, ownerID)
	if err != nil {
		t.Fatalf("ListPlaylists (owner): %v", err)
	}

	idSet := make(map[int64]bool)
	for _, p := range ownerList {
		idSet[p.ID] = true
	}

	if !idSet[ownerPrivateID] {
		t.Errorf("ListPlaylists (owner): own private playlist %d missing", ownerPrivateID)
	}
	if !idSet[ownerPublicID] {
		t.Errorf("ListPlaylists (owner): own public playlist %d missing", ownerPublicID)
	}
	if !idSet[otherPublicID] {
		t.Errorf("ListPlaylists (owner): other user's public playlist %d missing", otherPublicID)
	}
	if idSet[otherPrivateID] {
		t.Errorf("ListPlaylists (owner): other user's private playlist %d should not appear", otherPrivateID)
	}
}

// TestUpdatePlaylistPersistsChanges verifies that UpdatePlaylist writes name, comment,
// is_public, and cover_art_id back to the database.
func TestUpdatePlaylistPersistsChanges(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	p := samplePlaylist(1, "Before", false)
	id, err := s.CreatePlaylist(ctx, p)
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	updated := &model.Playlist{
		ID:         id,
		UserID:     1,
		Name:       "After",
		Comment:    "updated comment",
		IsPublic:   true,
		CoverArtID: 77,
	}
	if err := s.UpdatePlaylist(ctx, updated); err != nil {
		t.Fatalf("UpdatePlaylist: %v", err)
	}

	got, err := s.GetPlaylist(ctx, id)
	if err != nil {
		t.Fatalf("GetPlaylist after update: %v", err)
	}
	if got == nil {
		t.Fatal("GetPlaylist after update: returned nil")
	}

	if got.Name != "After" {
		t.Errorf("Name: got %q, want %q", got.Name, "After")
	}
	if got.Comment != "updated comment" {
		t.Errorf("Comment: got %q, want %q", got.Comment, "updated comment")
	}
	if !got.IsPublic {
		t.Errorf("IsPublic: got false, want true")
	}
	if got.CoverArtID != 77 {
		t.Errorf("CoverArtID: got %d, want 77", got.CoverArtID)
	}
}

// TestDeletePlaylistRemovesRow verifies that DeletePlaylist removes the playlist;
// a subsequent GetPlaylist returns nil without error.
func TestDeletePlaylistRemovesRow(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	id, err := s.CreatePlaylist(ctx, samplePlaylist(1, "Doomed", false))
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	if err := s.DeletePlaylist(ctx, id); err != nil {
		t.Fatalf("DeletePlaylist: %v", err)
	}

	got, err := s.GetPlaylist(ctx, id)
	if err != nil {
		t.Fatalf("GetPlaylist after delete: unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("GetPlaylist after delete: got non-nil playlist, want nil")
	}
}

// TestGetPlaylistEntriesNewPlaylistEmpty verifies that a freshly created playlist
// has no entries; GetPlaylistEntries returns a nil or empty slice without error.
func TestGetPlaylistEntriesNewPlaylistEmpty(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	id, err := s.CreatePlaylist(ctx, samplePlaylist(1, "Empty", false))
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	entries, err := s.GetPlaylistEntries(ctx, id)
	if err != nil {
		t.Fatalf("GetPlaylistEntries (empty): %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("GetPlaylistEntries (empty): got %d entries, want 0", len(entries))
	}
}

// TestSetPlaylistEntriesRoundtrip verifies that SetPlaylistEntries writes the given
// song IDs and that GetPlaylistEntries returns them in the correct order.
func TestSetPlaylistEntriesRoundtrip(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	id, err := s.CreatePlaylist(ctx, samplePlaylist(1, "Set Test", false))
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	songIDs := []int64{101, 202, 303}
	if err := s.SetPlaylistEntries(ctx, id, songIDs); err != nil {
		t.Fatalf("SetPlaylistEntries: %v", err)
	}

	entries, err := s.GetPlaylistEntries(ctx, id)
	if err != nil {
		t.Fatalf("GetPlaylistEntries: %v", err)
	}
	if len(entries) != len(songIDs) {
		t.Fatalf("GetPlaylistEntries: got %d entries, want %d", len(entries), len(songIDs))
	}

	for i, e := range entries {
		if e.SongID != songIDs[i] {
			t.Errorf("entries[%d].SongID: got %d, want %d", i, e.SongID, songIDs[i])
		}
		if e.Position != i {
			t.Errorf("entries[%d].Position: got %d, want %d", i, e.Position, i)
		}
		if e.PlaylistID != id {
			t.Errorf("entries[%d].PlaylistID: got %d, want %d", i, e.PlaylistID, id)
		}
	}
}

// TestSetPlaylistEntriesReplaces verifies that calling SetPlaylistEntries on a
// non-empty playlist replaces all existing entries rather than appending to them.
func TestSetPlaylistEntriesReplaces(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	id, err := s.CreatePlaylist(ctx, samplePlaylist(1, "Replace Test", false))
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	// Populate with initial entries.
	if err := s.SetPlaylistEntries(ctx, id, []int64{10, 20, 30, 40}); err != nil {
		t.Fatalf("SetPlaylistEntries (initial): %v", err)
	}

	// Replace with a shorter, different list.
	replacement := []int64{99, 88}
	if err := s.SetPlaylistEntries(ctx, id, replacement); err != nil {
		t.Fatalf("SetPlaylistEntries (replace): %v", err)
	}

	entries, err := s.GetPlaylistEntries(ctx, id)
	if err != nil {
		t.Fatalf("GetPlaylistEntries after replace: %v", err)
	}
	if len(entries) != len(replacement) {
		t.Fatalf("after replace: got %d entries, want %d", len(entries), len(replacement))
	}

	for i, e := range entries {
		if e.SongID != replacement[i] {
			t.Errorf("entries[%d].SongID after replace: got %d, want %d", i, e.SongID, replacement[i])
		}
	}
}

// TestAddToPlaylistAppendsAfterExisting verifies that AddToPlaylist adds new entries
// with positions that follow the current maximum, preserving existing entries.
func TestAddToPlaylistAppendsAfterExisting(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	id, err := s.CreatePlaylist(ctx, samplePlaylist(1, "Append Test", false))
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	initial := []int64{1, 2, 3}
	if err := s.SetPlaylistEntries(ctx, id, initial); err != nil {
		t.Fatalf("SetPlaylistEntries: %v", err)
	}

	added := []int64{4, 5}
	if err := s.AddToPlaylist(ctx, id, added); err != nil {
		t.Fatalf("AddToPlaylist: %v", err)
	}

	entries, err := s.GetPlaylistEntries(ctx, id)
	if err != nil {
		t.Fatalf("GetPlaylistEntries: %v", err)
	}

	want := append(initial, added...)
	if len(entries) != len(want) {
		t.Fatalf("GetPlaylistEntries after add: got %d entries, want %d", len(entries), len(want))
	}

	for i, e := range entries {
		if e.SongID != want[i] {
			t.Errorf("entries[%d].SongID: got %d, want %d", i, e.SongID, want[i])
		}
		if e.Position != i {
			t.Errorf("entries[%d].Position: got %d, want %d", i, e.Position, i)
		}
	}
}

// TestAddToPlaylistOnEmpty verifies that AddToPlaylist on a playlist with no existing
// entries inserts from position 0 with consecutive positions.
func TestAddToPlaylistOnEmpty(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	id, err := s.CreatePlaylist(ctx, samplePlaylist(1, "AddEmpty", false))
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	songIDs := []int64{7, 8, 9}
	if err := s.AddToPlaylist(ctx, id, songIDs); err != nil {
		t.Fatalf("AddToPlaylist (empty): %v", err)
	}

	entries, err := s.GetPlaylistEntries(ctx, id)
	if err != nil {
		t.Fatalf("GetPlaylistEntries: %v", err)
	}
	if len(entries) != len(songIDs) {
		t.Fatalf("got %d entries, want %d", len(entries), len(songIDs))
	}

	for i, e := range entries {
		if e.SongID != songIDs[i] {
			t.Errorf("entries[%d].SongID: got %d, want %d", i, e.SongID, songIDs[i])
		}
		if e.Position != i {
			t.Errorf("entries[%d].Position: got %d, want %d", i, e.Position, i)
		}
	}
}

// TestRemoveFromPlaylistCompactsPositions verifies that after removing entries at
// specific indices the remaining entries are renumbered from 0 with no gaps.
func TestRemoveFromPlaylistCompactsPositions(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	id, err := s.CreatePlaylist(ctx, samplePlaylist(1, "Remove Test", false))
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	// Insert songs A=100, B=200, C=300, D=400 at positions 0,1,2,3.
	if err := s.SetPlaylistEntries(ctx, id, []int64{100, 200, 300, 400}); err != nil {
		t.Fatalf("SetPlaylistEntries: %v", err)
	}

	// Remove positions 1 (B=200) and 3 (D=400); expect A=100 at 0, C=300 at 1.
	if err := s.RemoveFromPlaylist(ctx, id, []int{1, 3}); err != nil {
		t.Fatalf("RemoveFromPlaylist: %v", err)
	}

	entries, err := s.GetPlaylistEntries(ctx, id)
	if err != nil {
		t.Fatalf("GetPlaylistEntries after remove: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("after remove: got %d entries, want 2", len(entries))
	}

	if entries[0].SongID != 100 {
		t.Errorf("entries[0].SongID: got %d, want 100", entries[0].SongID)
	}
	if entries[0].Position != 0 {
		t.Errorf("entries[0].Position: got %d, want 0", entries[0].Position)
	}
	if entries[1].SongID != 300 {
		t.Errorf("entries[1].SongID: got %d, want 300", entries[1].SongID)
	}
	if entries[1].Position != 1 {
		t.Errorf("entries[1].Position: got %d, want 1", entries[1].Position)
	}
}

// TestRemoveFromPlaylistSingleEntry verifies removing the only entry in a playlist
// leaves it empty.
func TestRemoveFromPlaylistSingleEntry(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	id, err := s.CreatePlaylist(ctx, samplePlaylist(1, "Single Remove", false))
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	if err := s.SetPlaylistEntries(ctx, id, []int64{55}); err != nil {
		t.Fatalf("SetPlaylistEntries: %v", err)
	}

	if err := s.RemoveFromPlaylist(ctx, id, []int{0}); err != nil {
		t.Fatalf("RemoveFromPlaylist: %v", err)
	}

	entries, err := s.GetPlaylistEntries(ctx, id)
	if err != nil {
		t.Fatalf("GetPlaylistEntries after remove: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("after removing single entry: got %d entries, want 0", len(entries))
	}
}

// TestDuplicatePlaylistNameSameUserAllowed verifies that there is no unique constraint
// on (user_id, name); two playlists with the same name for the same user are permitted.
func TestDuplicatePlaylistNameSameUserAllowed(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	id1, err := s.CreatePlaylist(ctx, samplePlaylist(1, "Duplicated", false))
	if err != nil {
		t.Fatalf("CreatePlaylist (first): %v", err)
	}

	id2, err := s.CreatePlaylist(ctx, samplePlaylist(1, "Duplicated", false))
	if err != nil {
		t.Fatalf("CreatePlaylist (second, same name): %v", err)
	}

	if id1 == id2 {
		t.Errorf("expected distinct IDs for duplicate-named playlists, got %d twice", id1)
	}
}

// TestDeletePlaylistAlsoClearsEntries verifies that deleting a playlist removes its
// entries; GetPlaylistEntries for the deleted ID returns empty without error.
func TestDeletePlaylistAlsoClearsEntries(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	id, err := s.CreatePlaylist(ctx, samplePlaylist(1, "WithEntries", false))
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	if err := s.SetPlaylistEntries(ctx, id, []int64{11, 22, 33}); err != nil {
		t.Fatalf("SetPlaylistEntries: %v", err)
	}

	if err := s.DeletePlaylist(ctx, id); err != nil {
		t.Fatalf("DeletePlaylist: %v", err)
	}

	entries, err := s.GetPlaylistEntries(ctx, id)
	if err != nil {
		t.Fatalf("GetPlaylistEntries after delete: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("GetPlaylistEntries after delete: got %d entries, want 0", len(entries))
	}
}

// TestListPlaylistsEmptyStore verifies that ListPlaylists on a fresh DB returns a
// nil or empty slice without error.
func TestListPlaylistsEmptyStore(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	playlists, err := s.ListPlaylists(ctx, 1)
	if err != nil {
		t.Fatalf("ListPlaylists (empty): %v", err)
	}
	if len(playlists) != 0 {
		t.Errorf("ListPlaylists (empty): got %d playlists, want 0", len(playlists))
	}
}

// TestSetPlaylistEntriesEmptyListClearsAll verifies that calling SetPlaylistEntries
// with an empty slice removes all existing entries.
func TestSetPlaylistEntriesEmptyListClearsAll(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	id, err := s.CreatePlaylist(ctx, samplePlaylist(1, "Clear Test", false))
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	if err := s.SetPlaylistEntries(ctx, id, []int64{1, 2, 3}); err != nil {
		t.Fatalf("SetPlaylistEntries (populate): %v", err)
	}

	if err := s.SetPlaylistEntries(ctx, id, []int64{}); err != nil {
		t.Fatalf("SetPlaylistEntries (clear): %v", err)
	}

	entries, err := s.GetPlaylistEntries(ctx, id)
	if err != nil {
		t.Fatalf("GetPlaylistEntries after clear: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("GetPlaylistEntries after clear: got %d entries, want 0", len(entries))
	}
}

// TestListPlaylistsDoesNotReturnDuplicates verifies that an owner's public playlist
// appears exactly once in their own listing (not duplicated by matching both the
// user_id = ? and is_public = 1 branches of the query).
func TestListPlaylistsDoesNotReturnDuplicates(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	const ownerID int64 = 5

	// A public playlist owned by the querying user hits both predicates.
	if _, err := s.CreatePlaylist(ctx, samplePlaylist(ownerID, "Public Own", true)); err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	playlists, err := s.ListPlaylists(ctx, ownerID)
	if err != nil {
		t.Fatalf("ListPlaylists: %v", err)
	}

	seen := make(map[int64]int)
	for _, p := range playlists {
		seen[p.ID]++
	}
	for id, count := range seen {
		if count > 1 {
			t.Errorf("playlist ID %d appears %d times in ListPlaylists result, want 1", id, count)
		}
	}
}

// TestGetPlaylistEntriesOrderedByPosition verifies that GetPlaylistEntries always
// returns entries sorted by ascending position regardless of insertion order.
func TestGetPlaylistEntriesOrderedByPosition(t *testing.T) {
	s := newTestPlaylistStore(t)
	ctx := context.Background()

	id, err := s.CreatePlaylist(ctx, samplePlaylist(1, "Order Test", false))
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	// SetPlaylistEntries inserts in the order provided; use a deliberately shuffled list
	// to confirm the retrieval order is by position, not insertion row-id.
	songIDs := []int64{300, 100, 200}
	if err := s.SetPlaylistEntries(ctx, id, songIDs); err != nil {
		t.Fatalf("SetPlaylistEntries: %v", err)
	}

	entries, err := s.GetPlaylistEntries(ctx, id)
	if err != nil {
		t.Fatalf("GetPlaylistEntries: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	for i := 1; i < len(entries); i++ {
		if entries[i].Position <= entries[i-1].Position {
			t.Errorf("entries not ascending at index %d: position %d after %d",
				i, entries[i].Position, entries[i-1].Position)
		}
	}

	// Confirm the original insertion order (and thus song identity) is preserved.
	for i, e := range entries {
		if e.SongID != songIDs[i] {
			t.Errorf("entries[%d].SongID: got %d, want %d", i, e.SongID, songIDs[i])
		}
	}
}
