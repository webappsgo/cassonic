package subsonic

import (
	"sync"
	"time"
)

// NowPlayingTracker tracks active audio streams across all connected clients.
type NowPlayingTracker struct {
	mu      sync.RWMutex
	streams map[string]*NowPlayingEntry
}

// NowPlayingEntry holds metadata for one active stream.
type NowPlayingEntry struct {
	UserID     int64
	Username   string
	SongID     int64
	Title      string
	Artist     string
	Album      string
	StartedAt  time.Time
	PlayerName string
	MinutesAgo int
}

// NewNowPlayingTracker creates an empty NowPlayingTracker.
func NewNowPlayingTracker() *NowPlayingTracker {
	return &NowPlayingTracker{
		streams: make(map[string]*NowPlayingEntry),
	}
}

// Register records or replaces the active stream for the given user.
func (t *NowPlayingTracker) Register(entry *NowPlayingEntry) {
	key := entryKey(entry.UserID)
	entry.StartedAt = time.Now()
	entry.MinutesAgo = 0

	t.mu.Lock()
	t.streams[key] = entry
	t.mu.Unlock()
}

// Unregister removes the active stream for the given user.
func (t *NowPlayingTracker) Unregister(userID int64) {
	t.mu.Lock()
	delete(t.streams, entryKey(userID))
	t.mu.Unlock()
}

// All returns a snapshot of all active streams with refreshed MinutesAgo values.
func (t *NowPlayingTracker) All() []*NowPlayingEntry {
	now := time.Now()

	t.mu.RLock()
	result := make([]*NowPlayingEntry, 0, len(t.streams))
	for _, e := range t.streams {
		copy := *e
		copy.MinutesAgo = int(now.Sub(e.StartedAt).Minutes())
		result = append(result, &copy)
	}
	t.mu.RUnlock()

	return result
}

// ForUser returns the active stream for a specific user, or nil if none exists.
func (t *NowPlayingTracker) ForUser(userID int64) *NowPlayingEntry {
	t.mu.RLock()
	e := t.streams[entryKey(userID)]
	t.mu.RUnlock()

	if e == nil {
		return nil
	}
	copy := *e
	copy.MinutesAgo = int(time.Since(e.StartedAt).Minutes())
	return &copy
}

// entryKey returns the map key for a given user ID.
func entryKey(userID int64) string {
	return "u:" + int64ToStr(userID)
}

// int64ToStr converts an int64 to a decimal string without importing strconv here.
func int64ToStr(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
