package store

import (
	"context"
	"database/sql"
	"time"

	_ "modernc.org/sqlite"

	"github.com/local/cassonic/src/server/model"
)

type sqliteActivityStore struct {
	db *sql.DB
}

func (s *sqliteActivityStore) Star(ctx context.Context, userID int64, itemType string, itemID int64) error {
	const q = `INSERT OR IGNORE INTO stars (user_id, item_type, item_id, starred_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)`
	_, err := s.db.ExecContext(ctx, q, userID, itemType, itemID)
	return err
}

func (s *sqliteActivityStore) Unstar(ctx context.Context, userID int64, itemType string, itemID int64) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM stars WHERE user_id = ? AND item_type = ? AND item_id = ?`,
		userID, itemType, itemID)
	return err
}

func (s *sqliteActivityStore) GetStarred(ctx context.Context, userID int64) (*StarredItems, error) {
	result := &StarredItems{}
	var err error

	result.Songs, err = s.starredSongs(ctx, userID)
	if err != nil {
		return nil, err
	}

	result.Albums, err = s.starredAlbums(ctx, userID)
	if err != nil {
		return nil, err
	}

	result.Artists, err = s.starredArtists(ctx, userID)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *sqliteActivityStore) starredSongs(ctx context.Context, userID int64) ([]*model.Song, error) {
	q := `SELECT ` + songSelectCols + `
		FROM songs
		INNER JOIN stars ON stars.item_id = songs.id AND stars.item_type = 'song'
		WHERE stars.user_id = ?
		ORDER BY stars.starred_at DESC`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSongs(rows)
}

func (s *sqliteActivityStore) starredAlbums(ctx context.Context, userID int64) ([]*model.Album, error) {
	q := `SELECT ` + albumSelectCols + `
		FROM albums
		INNER JOIN stars ON stars.item_id = albums.id AND stars.item_type = 'album'
		WHERE stars.user_id = ?
		ORDER BY stars.starred_at DESC`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []*model.Album
	for rows.Next() {
		a, err := scanAlbum(rows)
		if err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (s *sqliteActivityStore) starredArtists(ctx context.Context, userID int64) ([]*model.Artist, error) {
	q := `SELECT ` + artistSelectCols + `
		FROM artists
		INNER JOIN stars ON stars.item_id = artists.id AND stars.item_type = 'artist'
		WHERE stars.user_id = ?
		ORDER BY stars.starred_at DESC`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artists []*model.Artist
	for rows.Next() {
		a, err := scanArtist(rows)
		if err != nil {
			return nil, err
		}
		artists = append(artists, a)
	}
	return artists, rows.Err()
}

func (s *sqliteActivityStore) IsStarred(ctx context.Context, userID int64, itemType string, itemID int64) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM stars WHERE user_id = ? AND item_type = ? AND item_id = ?`,
		userID, itemType, itemID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *sqliteActivityStore) SetRating(ctx context.Context, userID int64, itemType string, itemID int64, rating int) error {
	const q = `INSERT INTO ratings (user_id, item_type, item_id, rating, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, item_type, item_id) DO UPDATE SET
			rating     = excluded.rating,
			updated_at = CURRENT_TIMESTAMP`
	_, err := s.db.ExecContext(ctx, q, userID, itemType, itemID, rating)
	return err
}

func (s *sqliteActivityStore) GetRating(ctx context.Context, userID int64, itemType string, itemID int64) (int, error) {
	var rating int
	err := s.db.QueryRowContext(ctx,
		`SELECT rating FROM ratings WHERE user_id = ? AND item_type = ? AND item_id = ?`,
		userID, itemType, itemID).Scan(&rating)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return rating, err
}

func (s *sqliteActivityStore) RecordPlay(ctx context.Context, h *model.PlayHistory) error {
	const q = `INSERT INTO play_history (user_id, song_id, played_at, listened_for, client_name, scrobbled)
		VALUES (?, ?, ?, ?, ?, ?)`
	var playedAt string
	if h.PlayedAt.IsZero() {
		playedAt = time.Now().Format(time.RFC3339)
	} else {
		playedAt = h.PlayedAt.Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx, q,
		h.UserID, h.SongID, playedAt, h.ListenedFor, h.ClientName, h.Scrobbled)
	return err
}

func (s *sqliteActivityStore) GetPlayHistory(ctx context.Context, userID int64, limit int) ([]*model.PlayHistory, error) {
	const q = `SELECT id, user_id, song_id, played_at, listened_for, client_name, scrobbled
		FROM play_history WHERE user_id = ? ORDER BY played_at DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []*model.PlayHistory
	for rows.Next() {
		h := &model.PlayHistory{}
		var playedAt string
		if err := rows.Scan(
			&h.ID, &h.UserID, &h.SongID, &playedAt,
			&h.ListenedFor, &h.ClientName, &h.Scrobbled,
		); err != nil {
			return nil, err
		}
		h.PlayedAt, _ = time.Parse(time.RFC3339, playedAt)
		history = append(history, h)
	}
	return history, rows.Err()
}

func (s *sqliteActivityStore) SetBookmark(ctx context.Context, b *model.Bookmark) error {
	const q = `INSERT OR REPLACE INTO bookmarks (user_id, item_type, item_id, position, comment, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`
	_, err := s.db.ExecContext(ctx, q, b.UserID, b.ItemType, b.ItemID, b.Position, b.Comment)
	return err
}

func (s *sqliteActivityStore) GetBookmarks(ctx context.Context, userID int64) ([]*model.Bookmark, error) {
	const q = `SELECT id, user_id, item_type, item_id, position, comment, updated_at
		FROM bookmarks WHERE user_id = ? ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookmarks []*model.Bookmark
	for rows.Next() {
		bm := &model.Bookmark{}
		var updatedAt string
		if err := rows.Scan(
			&bm.ID, &bm.UserID, &bm.ItemType, &bm.ItemID,
			&bm.Position, &bm.Comment, &updatedAt,
		); err != nil {
			return nil, err
		}
		bm.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		bookmarks = append(bookmarks, bm)
	}
	return bookmarks, rows.Err()
}

func (s *sqliteActivityStore) DeleteBookmark(ctx context.Context, userID int64, itemType string, itemID int64) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM bookmarks WHERE user_id = ? AND item_type = ? AND item_id = ?`,
		userID, itemType, itemID)
	return err
}

func (s *sqliteActivityStore) SavePlayQueue(ctx context.Context, pq *model.PlayQueue, entries []*model.PlayQueueEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const upsertQ = `INSERT INTO play_queues (user_id, current, position, updated_at, changed_by)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			current    = excluded.current,
			position   = excluded.position,
			updated_at = CURRENT_TIMESTAMP,
			changed_by = excluded.changed_by`
	res, err := tx.ExecContext(ctx, upsertQ, pq.UserID, pq.Current, pq.Position, pq.ChangedBy)
	if err != nil {
		return err
	}

	queueID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	if queueID == 0 {
		err = tx.QueryRowContext(ctx,
			`SELECT id FROM play_queues WHERE user_id = ?`, pq.UserID).Scan(&queueID)
		if err != nil {
			return err
		}
	}

	if _, err = tx.ExecContext(ctx,
		`DELETE FROM play_queue_entries WHERE play_queue_id = ?`, queueID); err != nil {
		return err
	}

	for _, entry := range entries {
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO play_queue_entries (play_queue_id, song_id, position) VALUES (?, ?, ?)`,
			queueID, entry.SongID, entry.Position,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *sqliteActivityStore) GetPlayQueue(ctx context.Context, userID int64) (*model.PlayQueue, []*model.PlayQueueEntry, error) {
	const q = `SELECT id, user_id, current, position, updated_at, changed_by
		FROM play_queues WHERE user_id = ?`
	row := s.db.QueryRowContext(ctx, q, userID)

	pq := &model.PlayQueue{}
	var updatedAt string
	err := row.Scan(&pq.ID, &pq.UserID, &pq.Current, &pq.Position, &updatedAt, &pq.ChangedBy)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	pq.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	const entryQ = `SELECT id, play_queue_id, song_id, position
		FROM play_queue_entries WHERE play_queue_id = ? ORDER BY position`
	rows, err := s.db.QueryContext(ctx, entryQ, pq.ID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var entries []*model.PlayQueueEntry
	for rows.Next() {
		e := &model.PlayQueueEntry{}
		if err := rows.Scan(&e.ID, &e.PlayQueueID, &e.SongID, &e.Position); err != nil {
			return nil, nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return pq, entries, nil
}
