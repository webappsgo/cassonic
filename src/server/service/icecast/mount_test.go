package icecast

// Tests cover:
//   - shuffleInt64s: length preservation and multiset equality across sizes
//     (0, 1, many), including that it does not panic or lose elements
//   - formatSongTitle: with/without ArtistName
//   - nextBackoff: doubling, cap at maxBackoff, and cap idempotency
//   - sleepWithContext: normal expiry vs. context cancellation (including an
//     already-cancelled context)
//   - buildQueue: dispatch by mount.Scope (ScopeAll/ScopeArtist/ScopeGenre)
//     against a real seeded sqlite store, verifying the correct song IDs are
//     returned for each scope
//   - streamMount: the "no songs available" error path when buildQueue
//     succeeds but returns an empty queue (ScopeArtist with no matching
//     songs), driven end-to-end via Manager.Start/Stop with a non-nil but
//     otherwise-unused *ffmpeg.Manager
//
// transcodeAndSend/streamSongs are not covered directly: exercising them
// requires a real ffmpeg binary and a live Icecast connection, both
// explicitly out of scope per the task constraints.

import (
	"context"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/service/ffmpeg"
	"github.com/local/cassonic/src/server/store"
)

// --- shuffleInt64s ---

func TestShuffleInt64sPreservesElements(t *testing.T) {
	sizes := []int{0, 1, 2, 10, 100}
	for _, n := range sizes {
		ids := make([]int64, n)
		for i := range ids {
			ids[i] = int64(i)
		}
		shuffleInt64s(ids)

		if len(ids) != n {
			t.Fatalf("shuffleInt64s(n=%d): length changed to %d", n, len(ids))
		}
		seen := make(map[int64]bool, n)
		for _, id := range ids {
			seen[id] = true
		}
		if len(seen) != n {
			t.Errorf("shuffleInt64s(n=%d): expected %d distinct elements, got %d", n, n, len(seen))
		}
		for i := 0; i < n; i++ {
			if !seen[int64(i)] {
				t.Errorf("shuffleInt64s(n=%d): missing element %d after shuffle", n, i)
			}
		}
	}
}

// --- formatSongTitle ---

func TestFormatSongTitle(t *testing.T) {
	tests := []struct {
		name string
		song *model.Song
		want string
	}{
		{
			name: "with artist",
			song: &model.Song{ArtistName: "Artist", Title: "Title"},
			want: "Artist - Title",
		},
		{
			name: "without artist",
			song: &model.Song{ArtistName: "", Title: "Title"},
			want: "Title",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSongTitle(tt.song)
			if got != tt.want {
				t.Errorf("formatSongTitle: got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- nextBackoff ---

func TestNextBackoff(t *testing.T) {
	tests := []struct {
		name    string
		current time.Duration
		want    time.Duration
	}{
		{"doubles below cap", time.Second, 2 * time.Second},
		{"doubles again", 4 * time.Second, 8 * time.Second},
		{"caps at maxBackoff", 20 * time.Second, maxBackoff},
		{"stays at cap once past it", maxBackoff, maxBackoff},
		{"zero doubles to zero", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextBackoff(tt.current)
			if got != tt.want {
				t.Errorf("nextBackoff(%v): got %v, want %v", tt.current, got, tt.want)
			}
		})
	}
}

// --- sleepWithContext ---

func TestSleepWithContextExpiresNormally(t *testing.T) {
	err := sleepWithContext(context.Background(), 10*time.Millisecond)
	if err != nil {
		t.Errorf("sleepWithContext: unexpected error: %v", err)
	}
}

func TestSleepWithContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := sleepWithContext(ctx, time.Second)
	if err == nil {
		t.Fatal("sleepWithContext: expected error for already-cancelled context, got nil")
	}
}

func TestSleepWithContextCancelledMidway(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := sleepWithContext(ctx, time.Second)
	if err == nil {
		t.Fatal("sleepWithContext: expected error when context cancelled before deadline, got nil")
	}
}

// --- buildQueue ---

func seedSong(t *testing.T, db *store.DB, libraryID, artistID int64, artistName, genre, path string) int64 {
	t.Helper()
	song := &model.Song{
		LibraryID:  libraryID,
		Path:       path,
		Title:      "Song " + path,
		ArtistID:   artistID,
		ArtistName: artistName,
		Genre:      genre,
	}
	id, err := db.Music.UpsertSong(context.Background(), song)
	if err != nil {
		t.Fatalf("UpsertSong(%s): %v", path, err)
	}
	return id
}

func TestBuildQueueScopeAll(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	libID, err := db.Music.CreateLibrary(ctx, &model.Library{Name: "lib", Path: "/music"})
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	artistID, err := db.Music.UpsertArtist(ctx, &model.Artist{Name: "Artist A"})
	if err != nil {
		t.Fatalf("UpsertArtist: %v", err)
	}

	want := map[int64]bool{
		seedSong(t, db, libID, artistID, "Artist A", "Rock", "/music/a.mp3"): true,
		seedSong(t, db, libID, artistID, "Artist A", "Jazz", "/music/b.mp3"): true,
	}

	mgr := NewManager(db, nil, silentLogger(), nil)
	mount := &model.IcecastMount{Scope: model.ScopeAll}

	queue, err := mgr.buildQueue(ctx, mount)
	if err != nil {
		t.Fatalf("buildQueue: %v", err)
	}
	if len(queue) != len(want) {
		t.Fatalf("buildQueue: got %d IDs, want %d", len(queue), len(want))
	}
	for _, id := range queue {
		if !want[id] {
			t.Errorf("buildQueue: unexpected song ID %d", id)
		}
	}
}

func TestBuildQueueScopeArtist(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	libID, err := db.Music.CreateLibrary(ctx, &model.Library{Name: "lib", Path: "/music"})
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	artistA, err := db.Music.UpsertArtist(ctx, &model.Artist{Name: "Artist A"})
	if err != nil {
		t.Fatalf("UpsertArtist A: %v", err)
	}
	artistB, err := db.Music.UpsertArtist(ctx, &model.Artist{Name: "Artist B"})
	if err != nil {
		t.Fatalf("UpsertArtist B: %v", err)
	}

	wantID := seedSong(t, db, libID, artistA, "Artist A", "Rock", "/music/a.mp3")
	seedSong(t, db, libID, artistB, "Artist B", "Rock", "/music/b.mp3")

	mgr := NewManager(db, nil, silentLogger(), nil)
	mount := &model.IcecastMount{Scope: model.ScopeArtist, ArtistID: artistA}

	queue, err := mgr.buildQueue(ctx, mount)
	if err != nil {
		t.Fatalf("buildQueue: %v", err)
	}
	if len(queue) != 1 || queue[0] != wantID {
		t.Errorf("buildQueue(ScopeArtist): got %v, want [%d]", queue, wantID)
	}
}

func TestBuildQueueScopeGenre(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	libID, err := db.Music.CreateLibrary(ctx, &model.Library{Name: "lib", Path: "/music"})
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	artistID, err := db.Music.UpsertArtist(ctx, &model.Artist{Name: "Artist A"})
	if err != nil {
		t.Fatalf("UpsertArtist: %v", err)
	}

	wantID := seedSong(t, db, libID, artistID, "Artist A", "Jazz", "/music/a.mp3")
	seedSong(t, db, libID, artistID, "Artist A", "Rock", "/music/b.mp3")

	mgr := NewManager(db, nil, silentLogger(), nil)
	mount := &model.IcecastMount{Scope: model.ScopeGenre, Genre: "Jazz"}

	queue, err := mgr.buildQueue(ctx, mount)
	if err != nil {
		t.Fatalf("buildQueue: %v", err)
	}
	if len(queue) != 1 || queue[0] != wantID {
		t.Errorf("buildQueue(ScopeGenre): got %v, want [%d]", queue, wantID)
	}
}

func TestBuildQueueScopeArtistNoSongsIsEmptyNotError(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	mgr := NewManager(db, nil, silentLogger(), nil)
	mount := &model.IcecastMount{Scope: model.ScopeArtist, ArtistID: 999}

	queue, err := mgr.buildQueue(ctx, mount)
	if err != nil {
		t.Fatalf("buildQueue: unexpected error: %v", err)
	}
	if len(queue) != 0 {
		t.Errorf("buildQueue: got %d IDs for nonexistent artist, want 0", len(queue))
	}
}

// --- streamMount: empty-queue error path, driven end-to-end ---

func TestManagerStartEmptyQueueSetsErrorStatus(t *testing.T) {
	db := newTestDB(t)
	_, mountID := seedServerAndMount(t, db, true)

	// Restrict the mount to a scope with no matching songs so buildQueue
	// succeeds but returns an empty slice, exercising the "no songs
	// available" branch without needing a real ffmpeg binary or network.
	ctx := context.Background()
	mount, err := db.Icecast.GetMount(ctx, mountID)
	if err != nil {
		t.Fatalf("GetMount: %v", err)
	}
	mount.Scope = model.ScopeArtist
	mount.ArtistID = 999
	if err := db.Icecast.UpdateMount(ctx, mount); err != nil {
		t.Fatalf("UpdateMount: %v", err)
	}

	// A non-nil ffmpeg.Manager is required to get past the "ffmpeg
	// unavailable" short-circuit and reach buildQueue; the empty-queue
	// path returns before ffmpeg is ever invoked, so a zero-value Manager
	// is sufficient here.
	mgr := NewManager(db, &ffmpeg.Manager{}, silentLogger(), nil)
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	// Wait for the goroutine to reach its terminal error status before
	// tearing down, so Stop's context cancellation cannot race buildQueue's
	// DB call and turn "no songs available" into "context canceled".
	waitForMountStatus(t, db, mountID, model.StatusError)

	mgr.Stop()

	got, err := db.Icecast.GetMount(ctx, mountID)
	if err != nil {
		t.Fatalf("GetMount: %v", err)
	}
	if got.Status != model.StatusError {
		t.Errorf("mount.Status: got %q, want %q", got.Status, model.StatusError)
	}
	if got.LastError != "no songs available" {
		t.Errorf("mount.LastError: got %q, want %q", got.LastError, "no songs available")
	}
}
