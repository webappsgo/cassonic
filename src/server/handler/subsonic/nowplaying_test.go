package subsonic

// Tests for NowPlayingTracker in nowplaying.go: Register, Unregister, All, ForUser.

import "testing"

// TestNowPlayingTrackerEmpty verifies a fresh tracker reports no active streams.
func TestNowPlayingTrackerEmpty(t *testing.T) {
	tr := NewNowPlayingTracker()
	if got := tr.All(); len(got) != 0 {
		t.Errorf("All() = %d entries, want 0", len(got))
	}
	if tr.ForUser(1) != nil {
		t.Error("ForUser on empty tracker should return nil")
	}
}

// TestNowPlayingTrackerRegisterAndAll verifies a registered stream shows up in All().
func TestNowPlayingTrackerRegisterAndAll(t *testing.T) {
	tr := NewNowPlayingTracker()
	tr.Register(&NowPlayingEntry{
		UserID:     1,
		Username:   "alice",
		SongID:     10,
		Title:      "Song A",
		PlayerName: "web",
	})

	all := tr.All()
	if len(all) != 1 {
		t.Fatalf("All() = %d entries, want 1", len(all))
	}
	if all[0].Username != "alice" {
		t.Errorf("Username = %q, want alice", all[0].Username)
	}
	if all[0].MinutesAgo != 0 {
		t.Errorf("MinutesAgo = %d, want 0 for just-registered entry", all[0].MinutesAgo)
	}
}

// TestNowPlayingTrackerRegisterReplacesExisting verifies registering a second
// stream for the same user replaces the first, rather than adding a new entry.
func TestNowPlayingTrackerRegisterReplacesExisting(t *testing.T) {
	tr := NewNowPlayingTracker()
	tr.Register(&NowPlayingEntry{UserID: 1, Title: "First"})
	tr.Register(&NowPlayingEntry{UserID: 1, Title: "Second"})

	all := tr.All()
	if len(all) != 1 {
		t.Fatalf("All() = %d entries, want 1 (replace, not add)", len(all))
	}
	if all[0].Title != "Second" {
		t.Errorf("Title = %q, want Second", all[0].Title)
	}
}

// TestNowPlayingTrackerForUser verifies ForUser returns the correct entry for
// a specific user and nil for an unregistered user.
func TestNowPlayingTrackerForUser(t *testing.T) {
	tr := NewNowPlayingTracker()
	tr.Register(&NowPlayingEntry{UserID: 5, Title: "Song X"})

	got := tr.ForUser(5)
	if got == nil {
		t.Fatal("ForUser(5) returned nil, want entry")
	}
	if got.Title != "Song X" {
		t.Errorf("Title = %q, want Song X", got.Title)
	}

	if tr.ForUser(999) != nil {
		t.Error("ForUser(999) should return nil for unregistered user")
	}
}

// TestNowPlayingTrackerUnregister verifies Unregister removes the user's entry.
func TestNowPlayingTrackerUnregister(t *testing.T) {
	tr := NewNowPlayingTracker()
	tr.Register(&NowPlayingEntry{UserID: 1, Title: "Song A"})
	tr.Register(&NowPlayingEntry{UserID: 2, Title: "Song B"})

	tr.Unregister(1)

	all := tr.All()
	if len(all) != 1 {
		t.Fatalf("All() after unregister = %d entries, want 1", len(all))
	}
	if all[0].UserID != 2 {
		t.Errorf("remaining entry UserID = %d, want 2", all[0].UserID)
	}
	if tr.ForUser(1) != nil {
		t.Error("ForUser(1) should be nil after Unregister")
	}
}

// TestNowPlayingTrackerUnregisterNonexistent verifies unregistering a user
// with no active stream does not panic and leaves other entries intact.
func TestNowPlayingTrackerUnregisterNonexistent(t *testing.T) {
	tr := NewNowPlayingTracker()
	tr.Register(&NowPlayingEntry{UserID: 1, Title: "Song A"})

	tr.Unregister(999)

	if got := tr.All(); len(got) != 1 {
		t.Errorf("All() = %d entries, want 1 (unaffected)", len(got))
	}
}

// TestNowPlayingTrackerAllReturnsCopies verifies mutating an entry returned by
// All() does not affect the tracker's internal state.
func TestNowPlayingTrackerAllReturnsCopies(t *testing.T) {
	tr := NewNowPlayingTracker()
	tr.Register(&NowPlayingEntry{UserID: 1, Title: "Original"})

	all := tr.All()
	all[0].Title = "Mutated"

	again := tr.All()
	if again[0].Title != "Original" {
		t.Errorf("internal entry was mutated via returned copy: got %q", again[0].Title)
	}
}
