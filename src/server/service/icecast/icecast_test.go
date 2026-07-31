package icecast

// Tests cover:
//   - NewManager: returns a usable Manager
//   - Status: unknown/never-started mount returns Streaming=false
//   - Status: a manually-registered handle reports Streaming=true with the
//     current song and a non-negative uptime
//   - StartMount: no-op (returns nil, does not replace the handle) when a
//     mount is already tracked as running
//   - StartMount: unknown mount ID returns an error
//   - StopMount: unknown mount ID is a safe no-op (does not block)
//   - Manager.Start + Manager.Stop: end-to-end using a real (temp-file)
//     sqlite-backed store.DB and a nil ffmpeg.Manager, which drives the
//     mount's goroutine down the "ffmpeg not configured" error path
//     deterministically (no real ffmpeg/network needed) and Stop() blocking
//     until the goroutine has fully exited
//
// tempDir and silentLogger (defined here) are shared by all icecast_test
// files in this package.

import (
	"context"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// tempDir creates a temp directory under /tmp/local/cassonic-XXXXXX and
// registers cleanup, matching repo convention.
func tempDir(t *testing.T) string {
	t.Helper()
	base := "/tmp/local"
	if err := os.MkdirAll(base, 0750); err != nil {
		t.Fatalf("tempDir: mkdir %s: %v", base, err)
	}
	dir, err := os.MkdirTemp(base, "cassonic-")
	if err != nil {
		t.Fatalf("tempDir: mkdirtemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// silentLogger returns a logger that discards all output.
func silentLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

// newTestDB opens a fresh sqlite-backed store.DB in a temp directory and
// registers cleanup.
func newTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(tempDir(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close(db) })
	return db
}

// seedServerAndMount inserts a minimal Icecast server and mount and returns
// their IDs.
func seedServerAndMount(t *testing.T, db *store.DB, enabled bool) (serverID, mountID int64) {
	t.Helper()
	ctx := context.Background()

	srv := &model.IcecastServer{
		Name:       "Test Server",
		Host:       "127.0.0.1",
		Port:       8000,
		Protocol:   "http",
		SourceUser: "source",
		SourcePass: "hackme",
		Enabled:    true,
	}
	serverID, err := db.Icecast.CreateServer(ctx, srv)
	if err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	mount := &model.IcecastMount{
		ServerID:  serverID,
		MountPath: "/test",
		Name:      "Test Mount",
		Scope:     model.ScopeAll,
		Format:    model.FormatMP3,
		Enabled:   enabled,
		Status:    model.StatusDisconnected,
	}
	mountID, err = db.Icecast.CreateMount(ctx, mount)
	if err != nil {
		t.Fatalf("CreateMount: %v", err)
	}
	return serverID, mountID
}

// waitForMountStatus polls the mount's status until it reaches want or the
// timeout elapses. Manager.Start launches the streaming goroutine
// asynchronously, so calling Stop immediately after Start races the
// goroutine's own DB work (e.g. buildQueue) against Stop's context
// cancellation: cancelling too early can turn an expected "no songs
// available"/"ffmpeg not configured" terminal error into a generic
// "context canceled" one. Waiting for the intended terminal status first
// makes the subsequent Stop() purely a cleanup step.
func waitForMountStatus(t *testing.T, db *store.DB, mountID int64, want model.MountStatus) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		mount, err := db.Icecast.GetMount(context.Background(), mountID)
		if err != nil {
			t.Fatalf("waitForMountStatus: GetMount: %v", err)
		}
		if mount.Status == want {
			return
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatalf("waitForMountStatus: mount %d did not reach status %q within timeout (last status %q)", mountID, want, mount.Status)
		}
	}
}

// --- NewManager ---

func TestNewManagerReturnsUsableManager(t *testing.T) {
	db := newTestDB(t)
	mgr := NewManager(db, nil, silentLogger(), nil)
	if mgr == nil {
		t.Fatal("NewManager: returned nil")
	}
	if mgr.Status(1) == nil {
		t.Fatal("NewManager: Status on fresh manager returned nil")
	}
}

// --- Status ---

func TestStatusUnknownMountNotStreaming(t *testing.T) {
	db := newTestDB(t)
	mgr := NewManager(db, nil, silentLogger(), nil)

	st := mgr.Status(999)
	if st == nil {
		t.Fatal("Status: returned nil")
	}
	if st.Streaming {
		t.Error("Status: expected Streaming=false for unknown mount ID")
	}
}

func TestStatusRegisteredHandleReportsStreaming(t *testing.T) {
	db := newTestDB(t)
	mgr := NewManager(db, nil, silentLogger(), nil)

	started := time.Now().Add(-5 * time.Second)
	mgr.mu.Lock()
	mgr.mounts[42] = &MountStream{
		currentSong: "Artist - Title",
		startedAt:   started,
		doneCh:      make(chan struct{}),
	}
	mgr.mu.Unlock()

	st := mgr.Status(42)
	if !st.Streaming {
		t.Fatal("Status: expected Streaming=true for registered handle")
	}
	if st.CurrentSong != "Artist - Title" {
		t.Errorf("CurrentSong: got %q, want %q", st.CurrentSong, "Artist - Title")
	}
	if st.UptimeSecs < 0 {
		t.Errorf("UptimeSecs: got %d, want >= 0", st.UptimeSecs)
	}
	if !st.StartedAt.Equal(started) {
		t.Errorf("StartedAt: got %v, want %v", st.StartedAt, started)
	}
}

// --- StartMount ---

func TestStartMountAlreadyRunningIsNoOp(t *testing.T) {
	db := newTestDB(t)
	mgr := NewManager(db, nil, silentLogger(), nil)

	existing := &MountStream{doneCh: make(chan struct{})}
	mgr.mu.Lock()
	mgr.mounts[7] = existing
	mgr.mu.Unlock()

	if err := mgr.StartMount(context.Background(), 7); err != nil {
		t.Fatalf("StartMount: unexpected error for already-running mount: %v", err)
	}

	mgr.mu.RLock()
	got := mgr.mounts[7]
	mgr.mu.RUnlock()
	if got != existing {
		t.Error("StartMount: handle was replaced even though mount was already running")
	}
}

func TestStartMountUnknownIDReturnsError(t *testing.T) {
	db := newTestDB(t)
	mgr := NewManager(db, nil, silentLogger(), nil)

	err := mgr.StartMount(context.Background(), 12345)
	if err == nil {
		t.Fatal("StartMount: expected error for unknown mount ID, got nil")
	}
}

// --- StopMount ---

func TestStopMountUnknownIDIsNoOp(t *testing.T) {
	db := newTestDB(t)
	mgr := NewManager(db, nil, silentLogger(), nil)

	done := make(chan struct{})
	go func() {
		mgr.StopMount(999)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StopMount: blocked on unknown mount ID, expected immediate no-op")
	}
}

// --- Start / Stop end-to-end ---

func TestManagerStartStopWithoutFFmpegSetsErrorStatus(t *testing.T) {
	db := newTestDB(t)
	_, mountID := seedServerAndMount(t, db, true)

	mgr := NewManager(db, nil, silentLogger(), nil)

	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	// Wait for the goroutine to reach its terminal error status before
	// tearing down, so Stop's context cancellation cannot race the
	// "ffmpeg not configured" write (see waitForMountStatus).
	waitForMountStatus(t, db, mountID, model.StatusError)

	// Stop blocks until every mount goroutine has fully exited, so this is
	// deterministic even though streaming itself runs in a goroutine.
	mgr.Stop()

	mount, err := db.Icecast.GetMount(context.Background(), mountID)
	if err != nil {
		t.Fatalf("GetMount: %v", err)
	}
	if mount.Status != model.StatusError {
		t.Errorf("mount.Status: got %q, want %q", mount.Status, model.StatusError)
	}
	if mount.LastError != "ffmpeg not configured" {
		t.Errorf("mount.LastError: got %q, want %q", mount.LastError, "ffmpeg not configured")
	}

	st := mgr.Status(mountID)
	if st.Streaming {
		t.Error("Status after Stop: expected Streaming=false, mount should have been removed")
	}
}

func TestManagerStartSkipsDisabledMounts(t *testing.T) {
	db := newTestDB(t)
	_, mountID := seedServerAndMount(t, db, false)

	mgr := NewManager(db, nil, silentLogger(), nil)
	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}
	mgr.Stop()

	mount, err := db.Icecast.GetMount(context.Background(), mountID)
	if err != nil {
		t.Fatalf("GetMount: %v", err)
	}
	// A disabled mount is never started, so its status is left untouched.
	if mount.Status != model.StatusDisconnected {
		t.Errorf("disabled mount.Status: got %q, want unchanged %q", mount.Status, model.StatusDisconnected)
	}
}
