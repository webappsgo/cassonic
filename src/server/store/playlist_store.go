package store

import (
	"context"
	"database/sql"
	"sort"
	"time"

	_ "modernc.org/sqlite"

	"github.com/local/cassonic/src/server/model"
)

type sqlitePlaylistStore struct {
	db *sql.DB
}

// scanPlaylist reads a playlists row into a model.Playlist.
func scanPlaylist(row interface {
	Scan(...any) error
}) (*model.Playlist, error) {
	var p model.Playlist
	var createdAt, updatedAt string
	err := row.Scan(
		&p.ID,
		&p.UserID,
		&p.Name,
		&p.Comment,
		&p.IsPublic,
		&p.SongCount,
		&p.Duration,
		&p.CoverArtID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &p, nil
}

// playlistSelectCols is the standard SELECT column list for playlists.
const playlistSelectCols = `id, user_id, name, comment, is_public, song_count, duration, cover_art_id, created_at, updated_at`

func (s *sqlitePlaylistStore) CreatePlaylist(ctx context.Context, p *model.Playlist) (int64, error) {
	const q = `INSERT INTO playlists (user_id, name, comment, is_public, song_count, duration, cover_art_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
	res, err := s.db.ExecContext(ctx, q,
		p.UserID, p.Name, p.Comment, p.IsPublic, p.SongCount, p.Duration, p.CoverArtID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *sqlitePlaylistStore) GetPlaylist(ctx context.Context, id int64) (*model.Playlist, error) {
	q := `SELECT ` + playlistSelectCols + ` FROM playlists WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	p, err := scanPlaylist(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (s *sqlitePlaylistStore) ListPlaylists(ctx context.Context, userID int64) ([]*model.Playlist, error) {
	q := `SELECT ` + playlistSelectCols + `
		FROM playlists
		WHERE user_id = ? OR is_public = 1
		ORDER BY name`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playlists []*model.Playlist
	for rows.Next() {
		p, err := scanPlaylist(rows)
		if err != nil {
			return nil, err
		}
		playlists = append(playlists, p)
	}
	return playlists, rows.Err()
}

func (s *sqlitePlaylistStore) UpdatePlaylist(ctx context.Context, p *model.Playlist) error {
	const q = `UPDATE playlists SET
		name = ?, comment = ?, is_public = ?, cover_art_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, p.Name, p.Comment, p.IsPublic, p.CoverArtID, p.ID)
	return err
}

func (s *sqlitePlaylistStore) DeletePlaylist(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM playlist_entries WHERE playlist_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM playlists WHERE id = ?`, id); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *sqlitePlaylistStore) GetPlaylistEntries(ctx context.Context, playlistID int64) ([]*model.PlaylistEntry, error) {
	const q = `SELECT id, playlist_id, song_id, position
		FROM playlist_entries WHERE playlist_id = ? ORDER BY position`
	rows, err := s.db.QueryContext(ctx, q, playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*model.PlaylistEntry
	for rows.Next() {
		e := &model.PlaylistEntry{}
		if err := rows.Scan(&e.ID, &e.PlaylistID, &e.SongID, &e.Position); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *sqlitePlaylistStore) SetPlaylistEntries(ctx context.Context, playlistID int64, songIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM playlist_entries WHERE playlist_id = ?`, playlistID); err != nil {
		return err
	}

	for i, songID := range songIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO playlist_entries (playlist_id, song_id, position) VALUES (?, ?, ?)`,
			playlistID, songID, i,
		); err != nil {
			return err
		}
	}

	if err := syncPlaylistCounts(ctx, tx, playlistID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *sqlitePlaylistStore) AddToPlaylist(ctx context.Context, playlistID int64, songIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var maxPos int
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), -1) FROM playlist_entries WHERE playlist_id = ?`,
		playlistID).Scan(&maxPos)
	if err != nil {
		return err
	}

	for _, songID := range songIDs {
		maxPos++
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO playlist_entries (playlist_id, song_id, position) VALUES (?, ?, ?)`,
			playlistID, songID, maxPos,
		); err != nil {
			return err
		}
	}

	if err := syncPlaylistCounts(ctx, tx, playlistID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *sqlitePlaylistStore) RemoveFromPlaylist(ctx context.Context, playlistID int64, indices []int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Sort indices descending so earlier positions are not shifted by later deletions
	sorted := make([]int, len(indices))
	copy(sorted, indices)
	sort.Sort(sort.Reverse(sort.IntSlice(sorted)))

	for _, idx := range sorted {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM playlist_entries WHERE playlist_id = ? AND position = ?`,
			playlistID, idx,
		); err != nil {
			return err
		}
	}

	// Renumber remaining entries sequentially from 0
	if _, err := tx.ExecContext(ctx, `
		WITH ranked AS (
			SELECT id, ROW_NUMBER() OVER (ORDER BY position) - 1 AS new_pos
			FROM playlist_entries
			WHERE playlist_id = ?
		)
		UPDATE playlist_entries SET position = ranked.new_pos
		FROM ranked
		WHERE playlist_entries.id = ranked.id`,
		playlistID,
	); err != nil {
		return err
	}

	if err := syncPlaylistCounts(ctx, tx, playlistID); err != nil {
		return err
	}

	return tx.Commit()
}

// syncPlaylistCounts recalculates and writes song_count and duration for a playlist within a transaction.
func syncPlaylistCounts(ctx context.Context, tx *sql.Tx, playlistID int64) error {
	const q = `UPDATE playlists SET
		song_count = (SELECT COUNT(*) FROM playlist_entries WHERE playlist_id = ?),
		duration   = (
			SELECT COALESCE(SUM(s.duration), 0)
			FROM playlist_entries pe
			JOIN songs s ON pe.song_id = s.id
			WHERE pe.playlist_id = ?
		),
		updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`
	_, err := tx.ExecContext(ctx, q, playlistID, playlistID, playlistID)
	return err
}
