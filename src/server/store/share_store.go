package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/local/cassonic/src/server/model"
)

// ShareStore manages share links.
type ShareStore interface {
	// CreateShare inserts a new share and returns the assigned ID.
	CreateShare(ctx context.Context, s *model.Share) (int64, error)
	// GetShare fetches a share by primary key.
	GetShare(ctx context.Context, id int64) (*model.Share, error)
	// GetShareByToken fetches a share by its URL token.
	GetShareByToken(ctx context.Context, token string) (*model.Share, error)
	// ListSharesByUser returns all shares owned by the specified user.
	ListSharesByUser(ctx context.Context, userID int64) ([]*model.Share, error)
	// UpdateShare writes all mutable fields back to the database.
	UpdateShare(ctx context.Context, s *model.Share) error
	// DeleteShare permanently removes a share.
	DeleteShare(ctx context.Context, id int64) error
	// IncrementViewCount adds 1 to the view counter for a share.
	IncrementViewCount(ctx context.Context, id int64) error
}

// shareSchema is applied to server.db alongside the main serverSchema.
// Using CREATE TABLE IF NOT EXISTS ensures it is safe to run repeatedly.
const shareSchema = `
CREATE TABLE IF NOT EXISTS shares (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token TEXT NOT NULL UNIQUE,
    item_type TEXT NOT NULL,
    item_id INTEGER NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL DEFAULT '',
    view_count INTEGER NOT NULL DEFAULT 0,
    expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_shares_user_id ON shares(user_id);
CREATE INDEX IF NOT EXISTS idx_shares_token ON shares(token);
`

type sqliteShareStore struct {
	db *sql.DB
}

// NewShareStore creates a sqliteShareStore and ensures the schema is applied.
func NewShareStore(db *sql.DB) (ShareStore, error) {
	if _, err := db.ExecContext(context.Background(), shareSchema); err != nil {
		return nil, fmt.Errorf("share_store: apply schema: %w", err)
	}
	return &sqliteShareStore{db: db}, nil
}

func scanShare(row interface {
	Scan(...any) error
}) (*model.Share, error) {
	var s model.Share
	var expiresAt, createdAt, updatedAt sql.NullString
	err := row.Scan(
		&s.ID,
		&s.UserID,
		&s.Token,
		&s.ItemType,
		&s.ItemID,
		&s.Description,
		&s.PasswordHash,
		&s.ViewCount,
		&expiresAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if expiresAt.Valid && expiresAt.String != "" {
		s.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt.String)
	}
	if createdAt.Valid {
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}
	if updatedAt.Valid {
		s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	}
	return &s, nil
}

func (s *sqliteShareStore) CreateShare(ctx context.Context, sh *model.Share) (int64, error) {
	const q = `INSERT INTO shares
		(user_id, token, item_type, item_id, description, password_hash, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`

	var expiresAt any
	if !sh.ExpiresAt.IsZero() {
		expiresAt = sh.ExpiresAt.UTC().Format(time.RFC3339)
	}

	res, err := s.db.ExecContext(ctx, q,
		sh.UserID,
		sh.Token,
		sh.ItemType,
		sh.ItemID,
		sh.Description,
		sh.PasswordHash,
		expiresAt,
	)
	if err != nil {
		return 0, fmt.Errorf("share_store create: %w", err)
	}
	return res.LastInsertId()
}

func (s *sqliteShareStore) GetShare(ctx context.Context, id int64) (*model.Share, error) {
	const q = `SELECT id, user_id, token, item_type, item_id, description,
		password_hash, view_count, expires_at, created_at, updated_at
		FROM shares WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	sh, err := scanShare(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("share_store get: %w", err)
	}
	return sh, nil
}

func (s *sqliteShareStore) GetShareByToken(ctx context.Context, token string) (*model.Share, error) {
	const q = `SELECT id, user_id, token, item_type, item_id, description,
		password_hash, view_count, expires_at, created_at, updated_at
		FROM shares WHERE token = ?`
	row := s.db.QueryRowContext(ctx, q, token)
	sh, err := scanShare(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("share_store get by token: %w", err)
	}
	return sh, nil
}

func (s *sqliteShareStore) ListSharesByUser(ctx context.Context, userID int64) ([]*model.Share, error) {
	const q = `SELECT id, user_id, token, item_type, item_id, description,
		password_hash, view_count, expires_at, created_at, updated_at
		FROM shares WHERE user_id = ? ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("share_store list: %w", err)
	}
	defer rows.Close()

	var shares []*model.Share
	for rows.Next() {
		sh, err := scanShare(rows)
		if err != nil {
			return nil, fmt.Errorf("share_store list scan: %w", err)
		}
		shares = append(shares, sh)
	}
	return shares, rows.Err()
}

func (s *sqliteShareStore) UpdateShare(ctx context.Context, sh *model.Share) error {
	const q = `UPDATE shares SET description = ?, password_hash = ?, expires_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`

	var expiresAt any
	if !sh.ExpiresAt.IsZero() {
		expiresAt = sh.ExpiresAt.UTC().Format(time.RFC3339)
	}

	_, err := s.db.ExecContext(ctx, q, sh.Description, sh.PasswordHash, expiresAt, sh.ID)
	if err != nil {
		return fmt.Errorf("share_store update: %w", err)
	}
	return nil
}

func (s *sqliteShareStore) DeleteShare(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM shares WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("share_store delete: %w", err)
	}
	return nil
}

func (s *sqliteShareStore) IncrementViewCount(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE shares SET view_count = view_count + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("share_store increment view count: %w", err)
	}
	return nil
}
