package icecast

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/service/ffmpeg"
	"github.com/local/cassonic/src/server/store"
)

// maxBackoff is the ceiling for reconnect wait time.
const maxBackoff = 30 * time.Second

// allSongsLimit is the maximum number of songs fetched when streaming the full library.
const allSongsLimit = 100_000

// streamMount is the long-running goroutine for one Icecast mount.
// It reconnects on failure with exponential backoff (1s → 2s → 4s → 8s → 16s → 30s max).
func (m *Manager) streamMount(ctx context.Context, mount *model.IcecastMount, handle *MountStream) {
	if m.ff == nil {
		m.logger.Printf("icecast: mount %d (%s): ffmpeg unavailable, stream disabled", mount.ID, mount.MountPath)
		_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusError, "", "ffmpeg not configured")
		return
	}

	queue, err := m.buildQueue(ctx, mount)
	if err != nil {
		m.logger.Printf("icecast: mount %d: build queue: %v", mount.ID, err)
		_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusError, "", err.Error())
		return
	}
	if len(queue) == 0 {
		m.logger.Printf("icecast: mount %d: no songs in queue", mount.ID)
		_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusError, "", "no songs available")
		return
	}

	queuePos := 0
	backoff := time.Second

	for {
		if ctx.Err() != nil {
			_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusDisconnected, "", "")
			return
		}

		if queuePos >= len(queue) {
			queue, err = m.buildQueue(ctx, mount)
			if err != nil {
				m.logger.Printf("icecast: mount %d: rebuild queue: %v", mount.ID, err)
				_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusError, "", err.Error())
				return
			}
			queuePos = 0
		}

		songID := queue[queuePos]

		song, err := m.db.Music.GetSong(ctx, songID)
		if err != nil || song == nil {
			m.logger.Printf("icecast: mount %d: get song %d: %v", mount.ID, songID, err)
			queuePos++
			continue
		}

		_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusConnecting, "", "")

		server, err := m.db.Icecast.GetServer(ctx, mount.ServerID)
		if err != nil || server == nil {
			errMsg := "server not found"
			if err != nil {
				errMsg = err.Error()
			}
			m.logger.Printf("icecast: mount %d: get server %d: %v", mount.ID, mount.ServerID, err)
			_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusError, "", errMsg)
			if err := sleepWithContext(ctx, backoff); err != nil {
				_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusDisconnected, "", "")
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}

		conn, err := Connect(server, mount)
		if err != nil {
			m.logger.Printf("icecast: mount %d: connect: %v", mount.ID, err)
			_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusError, "", err.Error())
			if err := sleepWithContext(ctx, backoff); err != nil {
				_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusDisconnected, "", "")
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}

		backoff = time.Second

		songTitle := formatSongTitle(song)
		conn.SetMetadata(songTitle)
		handle.mu.Lock()
		handle.currentSong = songTitle
		handle.mu.Unlock()
		_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusConnected, songTitle, "")

		streamErr := m.streamSongs(ctx, mount, handle, conn, queue, &queuePos)

		conn.Close()

		if ctx.Err() != nil {
			_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusDisconnected, "", "")
			return
		}

		if streamErr != nil {
			m.logger.Printf("icecast: mount %d: stream error: %v", mount.ID, streamErr)
			_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusError, "", streamErr.Error())
			if err := sleepWithContext(ctx, backoff); err != nil {
				_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusDisconnected, "", "")
				return
			}
			backoff = nextBackoff(backoff)
		}
	}
}

// streamSongs loops over the queue starting at *pos, transcoding each track and
// piping audio into conn. Returns on connection error or context cancellation.
// On normal track completion (ffmpeg EOF) it advances *pos and continues.
func (m *Manager) streamSongs(
	ctx context.Context,
	mount *model.IcecastMount,
	handle *MountStream,
	conn *IcecastConn,
	queue []int64,
	pos *int,
) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		if *pos >= len(queue) {
			return nil
		}

		songID := queue[*pos]

		song, err := m.db.Music.GetSong(ctx, songID)
		if err != nil || song == nil {
			*pos++
			continue
		}

		songTitle := formatSongTitle(song)
		conn.SetMetadata(songTitle)
		handle.mu.Lock()
		handle.currentSong = songTitle
		handle.mu.Unlock()
		_ = m.db.Icecast.UpdateMountStatus(ctx, mount.ID, model.StatusConnected, songTitle, "")

		transcodeErr := m.transcodeAndSend(ctx, mount, conn, song.Path)
		if transcodeErr != nil {
			return transcodeErr
		}

		*pos++
	}
}

// transcodeAndSend starts an ffmpeg transcode subprocess and copies its output
// into the Icecast connection. Returns nil on clean track completion, or an error
// if the connection failed (which triggers reconnection in the caller).
func (m *Manager) transcodeAndSend(
	ctx context.Context,
	mount *model.IcecastMount,
	conn *IcecastConn,
	inputPath string,
) error {
	opts := ffmpeg.TranscodeOpts{
		InputPath: inputPath,
		Format:    string(mount.Format),
		BitRate:   mount.BitRate,
	}

	result, err := m.ff.Transcode(ctx, opts)
	if err != nil {
		return fmt.Errorf("transcode start %q: %w", inputPath, err)
	}
	defer result.Close()

	buf := make([]byte, 4096)
	for {
		nr, readErr := result.Read(buf)
		if nr > 0 {
			if _, writeErr := conn.Write(buf[:nr]); writeErr != nil {
				return fmt.Errorf("write to icecast: %w", writeErr)
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			return nil
		}
	}
}

// buildQueue constructs and optionally shuffles the song ID list for a mount.
func (m *Manager) buildQueue(ctx context.Context, mount *model.IcecastMount) ([]int64, error) {
	switch mount.Scope {
	case model.ScopeArtist:
		return m.getSongIDsByArtist(ctx, mount.ArtistID)
	case model.ScopeGenre:
		return m.getSongIDsByGenre(ctx, mount.Genre)
	default:
		return m.getAllSongIDs(ctx)
	}
}

// getAllSongIDs returns IDs for all songs in the library, shuffled.
func (m *Manager) getAllSongIDs(ctx context.Context) ([]int64, error) {
	songs, err := m.db.Music.GetRandomSongs(ctx, allSongsLimit, "", "", "")
	if err != nil {
		return nil, fmt.Errorf("get all songs: %w", err)
	}
	ids := make([]int64, len(songs))
	for i, s := range songs {
		ids[i] = s.ID
	}
	shuffleInt64s(ids)
	return ids, nil
}

// getSongIDsByArtist returns IDs for all songs by a specific artist, shuffled.
func (m *Manager) getSongIDsByArtist(ctx context.Context, artistID int64) ([]int64, error) {
	songs, err := m.db.Music.ListSongsByArtist(ctx, artistID)
	if err != nil {
		return nil, fmt.Errorf("list songs by artist %d: %w", artistID, err)
	}
	ids := make([]int64, len(songs))
	for i, s := range songs {
		ids[i] = s.ID
	}
	shuffleInt64s(ids)
	return ids, nil
}

// getSongIDsByGenre returns IDs for all songs in a specific genre, shuffled.
func (m *Manager) getSongIDsByGenre(ctx context.Context, genre string) ([]int64, error) {
	songs, err := m.db.Music.ListSongsByGenre(ctx, genre, store.ListOpts{Limit: allSongsLimit})
	if err != nil {
		return nil, fmt.Errorf("list songs by genre %q: %w", genre, err)
	}
	ids := make([]int64, len(songs))
	for i, s := range songs {
		ids[i] = s.ID
	}
	shuffleInt64s(ids)
	return ids, nil
}

// shuffleInt64s performs an in-place Fisher-Yates shuffle using math/rand.
// Song ordering is not security-sensitive; crypto/rand is not needed here.
func shuffleInt64s(ids []int64) {
	for i := len(ids) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		ids[i], ids[j] = ids[j], ids[i]
	}
}

// formatSongTitle returns "Artist - Title" for use as ICY stream metadata.
func formatSongTitle(song *model.Song) string {
	if song.ArtistName != "" {
		return song.ArtistName + " - " + song.Title
	}
	return song.Title
}

// nextBackoff doubles the backoff duration up to maxBackoff.
func nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > maxBackoff {
		return maxBackoff
	}
	return next
}

// sleepWithContext waits for d to elapse or for ctx to be cancelled.
// Returns ctx.Err() if cancelled, nil otherwise.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
