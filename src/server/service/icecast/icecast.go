package icecast

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/service/ffmpeg"
	"github.com/local/cassonic/src/server/store"
)

// Manager manages all active Icecast mount goroutines.
type Manager struct {
	db     *store.DB
	ff     *ffmpeg.Manager
	logger *log.Logger
	mu     sync.RWMutex
	// mounts maps mount ID to its active streaming goroutine handle.
	mounts map[int64]*MountStream
	// cancelAll stops all mount goroutines during shutdown.
	cancelAll context.CancelFunc
}

// MountStream represents a running stream goroutine for one mount.
type MountStream struct {
	Mount *model.IcecastMount
	// cancel stops this specific mount goroutine.
	cancel context.CancelFunc
	// doneCh is closed when the goroutine has fully exited.
	doneCh <-chan struct{}
	// mu protects the current-track fields below.
	mu sync.RWMutex
	// currentSong holds the "Artist - Title" string for ICY metadata.
	currentSong string
	// startedAt records when this stream goroutine was launched.
	startedAt time.Time
}

// MountStatus holds live status information for a mount.
type MountStatus struct {
	Streaming   bool
	CurrentSong string
	StartedAt   time.Time
	UptimeSecs  int
}

// NewManager creates a new Icecast manager.
func NewManager(db *store.DB, ff *ffmpeg.Manager, logger *log.Logger) *Manager {
	return &Manager{
		db:     db,
		ff:     ff,
		logger: logger,
		mounts: make(map[int64]*MountStream),
	}
}

// Start loads all enabled mounts from the database and starts their streaming goroutines.
func (m *Manager) Start(ctx context.Context) error {
	mounts, err := m.db.Icecast.ListMounts(ctx)
	if err != nil {
		return fmt.Errorf("icecast manager: list mounts: %w", err)
	}

	managerCtx, cancel := context.WithCancel(ctx)
	m.cancelAll = cancel

	for _, mount := range mounts {
		if !mount.Enabled {
			continue
		}
		if err := m.StartMount(managerCtx, mount.ID); err != nil {
			m.logger.Printf("icecast: failed to start mount %d (%s): %v", mount.ID, mount.MountPath, err)
		}
	}
	return nil
}

// Stop gracefully stops all streaming goroutines.
func (m *Manager) Stop() {
	if m.cancelAll != nil {
		m.cancelAll()
	}

	m.mu.Lock()
	handles := make([]*MountStream, 0, len(m.mounts))
	for _, h := range m.mounts {
		handles = append(handles, h)
	}
	m.mu.Unlock()

	for _, h := range handles {
		<-h.doneCh
	}
}

// StartMount begins streaming for a specific mount by ID.
// If the mount is already streaming this is a no-op.
func (m *Manager) StartMount(ctx context.Context, mountID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, running := m.mounts[mountID]; running {
		return nil
	}

	mount, err := m.db.Icecast.GetMount(ctx, mountID)
	if err != nil {
		return fmt.Errorf("icecast: get mount %d: %w", mountID, err)
	}
	if mount == nil {
		return fmt.Errorf("icecast: mount %d not found", mountID)
	}

	mountCtx, cancel := context.WithCancel(ctx)
	doneCh := make(chan struct{})

	handle := &MountStream{
		Mount:     mount,
		cancel:    cancel,
		doneCh:    doneCh,
		startedAt: time.Now(),
	}

	m.mounts[mountID] = handle

	go func() {
		defer close(doneCh)
		defer func() {
			m.mu.Lock()
			delete(m.mounts, mountID)
			m.mu.Unlock()
		}()
		m.streamMount(mountCtx, mount, handle)
	}()

	return nil
}

// StopMount stops the streaming goroutine for a specific mount.
func (m *Manager) StopMount(mountID int64) {
	m.mu.Lock()
	handle, ok := m.mounts[mountID]
	m.mu.Unlock()

	if !ok {
		return
	}

	handle.cancel()
	<-handle.doneCh
}

// Status returns the current runtime status for a mount, or nil if not streaming.
func (m *Manager) Status(mountID int64) *MountStatus {
	m.mu.RLock()
	handle, ok := m.mounts[mountID]
	m.mu.RUnlock()

	if !ok {
		return &MountStatus{Streaming: false}
	}

	handle.mu.RLock()
	song := handle.currentSong
	started := handle.startedAt
	handle.mu.RUnlock()

	uptime := int(time.Since(started).Seconds())
	return &MountStatus{
		Streaming:   true,
		CurrentSong: song,
		StartedAt:   started,
		UptimeSecs:  uptime,
	}
}
