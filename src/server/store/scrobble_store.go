package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/local/cassonic/src/server/model"
)

// ScrobbleStore manages scrobbling services and queue entries.
type ScrobbleStore interface {
	// CreateService inserts a new scrobble service configuration and returns the assigned ID.
	CreateService(ctx context.Context, s *model.ScrobbleService) (int64, error)
	// GetService fetches a scrobble service by primary key.
	GetService(ctx context.Context, id int64) (*model.ScrobbleService, error)
	// ListServices returns all scrobble services for the given user.
	ListServices(ctx context.Context, userID int64) ([]*model.ScrobbleService, error)
	// ListAllEnabledServices returns all enabled scrobble services across all users.
	ListAllEnabledServices(ctx context.Context) ([]*model.ScrobbleService, error)
	// UpdateService writes all mutable fields back to the database.
	UpdateService(ctx context.Context, s *model.ScrobbleService) error
	// DeleteService permanently removes a scrobble service configuration.
	DeleteService(ctx context.Context, id int64) error
	// SetServiceVerified updates the verified flag and last_error for a service.
	SetServiceVerified(ctx context.Context, id int64, verified bool, lastErr string) error

	// EnqueueScrobble adds a new entry to the retry queue.
	EnqueueScrobble(ctx context.Context, q *model.ScrobbleQueueEntry) error
	// ListPendingByService returns up to limit pending queue entries for the given service.
	ListPendingByService(ctx context.Context, serviceID int64, limit int) ([]*model.ScrobbleQueueEntry, error)
	// DeleteQueueEntry removes a single queue entry by ID.
	DeleteQueueEntry(ctx context.Context, id int64) error
	// IncrementAttempts increments the attempt counter and records the last error.
	IncrementAttempts(ctx context.Context, id int64, lastErr string) error
	// PurgeStaleQueue deletes entries older than before or with attempts >= maxAttempts.
	PurgeStaleQueue(ctx context.Context, before time.Time, maxAttempts int) error
}

// scrobbleSchema is applied to server.db alongside the main serverSchema.
const scrobbleSchema = `
CREATE TABLE IF NOT EXISTS scrobble_services (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    service_type TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    base_url TEXT NOT NULL DEFAULT '',
    api_key TEXT NOT NULL DEFAULT '',
    api_secret_enc TEXT NOT NULL DEFAULT '',
    session_key_enc TEXT NOT NULL DEFAULT '',
    token_enc TEXT NOT NULL DEFAULT '',
    username TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    verified INTEGER NOT NULL DEFAULT 0,
    last_verified_at DATETIME,
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_scrobble_services_user_id ON scrobble_services(user_id);

CREATE TABLE IF NOT EXISTS scrobble_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    service_id INTEGER NOT NULL,
    track_data TEXT NOT NULL,
    queued_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at DATETIME,
    last_error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_scrobble_queue_service_id ON scrobble_queue(service_id, attempts);
`

type sqliteScrobbleStore struct {
	db *sql.DB
}

// NewScrobbleStore creates a sqliteScrobbleStore and ensures the schema is applied.
func NewScrobbleStore(db *sql.DB) (ScrobbleStore, error) {
	if _, err := db.ExecContext(context.Background(), scrobbleSchema); err != nil {
		return nil, fmt.Errorf("scrobble_store: apply schema: %w", err)
	}
	return &sqliteScrobbleStore{db: db}, nil
}

const scrobbleServiceCols = `id, user_id, service_type, display_name, base_url, api_key,
	api_secret_enc, session_key_enc, token_enc, username, enabled, verified,
	last_verified_at, last_error, created_at, updated_at`

func scanScrobbleService(row interface {
	Scan(...any) error
}) (*model.ScrobbleService, error) {
	var svc model.ScrobbleService
	var lastVerified, createdAt, updatedAt sql.NullString
	var enabled, verified int
	err := row.Scan(
		&svc.ID,
		&svc.UserID,
		&svc.ServiceType,
		&svc.DisplayName,
		&svc.BaseURL,
		&svc.APIKey,
		&svc.APISecretEnc,
		&svc.SessionKeyEnc,
		&svc.TokenEnc,
		&svc.Username,
		&enabled,
		&verified,
		&lastVerified,
		&svc.LastError,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	svc.Enabled = enabled != 0
	svc.Verified = verified != 0
	if lastVerified.Valid && lastVerified.String != "" {
		svc.LastVerifiedAt, _ = time.Parse(time.RFC3339, lastVerified.String)
	}
	if createdAt.Valid {
		svc.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}
	if updatedAt.Valid {
		svc.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	}
	return &svc, nil
}

func (s *sqliteScrobbleStore) CreateService(ctx context.Context, svc *model.ScrobbleService) (int64, error) {
	const q = `INSERT INTO scrobble_services
		(user_id, service_type, display_name, base_url, api_key, api_secret_enc,
		 session_key_enc, token_enc, username, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, q,
		svc.UserID, svc.ServiceType, svc.DisplayName, svc.BaseURL, svc.APIKey,
		svc.APISecretEnc, svc.SessionKeyEnc, svc.TokenEnc, svc.Username,
		boolToInt(svc.Enabled),
	)
	if err != nil {
		return 0, fmt.Errorf("scrobble_store create service: %w", err)
	}
	return res.LastInsertId()
}

func (s *sqliteScrobbleStore) GetService(ctx context.Context, id int64) (*model.ScrobbleService, error) {
	q := `SELECT ` + scrobbleServiceCols + ` FROM scrobble_services WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	svc, err := scanScrobbleService(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scrobble_store get service: %w", err)
	}
	return svc, nil
}

func (s *sqliteScrobbleStore) ListServices(ctx context.Context, userID int64) ([]*model.ScrobbleService, error) {
	q := `SELECT ` + scrobbleServiceCols + ` FROM scrobble_services WHERE user_id = ? ORDER BY id`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("scrobble_store list services: %w", err)
	}
	defer rows.Close()

	var svcs []*model.ScrobbleService
	for rows.Next() {
		svc, err := scanScrobbleService(rows)
		if err != nil {
			return nil, fmt.Errorf("scrobble_store list services scan: %w", err)
		}
		svcs = append(svcs, svc)
	}
	return svcs, rows.Err()
}

func (s *sqliteScrobbleStore) ListAllEnabledServices(ctx context.Context) ([]*model.ScrobbleService, error) {
	q := `SELECT ` + scrobbleServiceCols + ` FROM scrobble_services WHERE enabled = 1 ORDER BY user_id, id`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("scrobble_store list enabled services: %w", err)
	}
	defer rows.Close()

	var svcs []*model.ScrobbleService
	for rows.Next() {
		svc, err := scanScrobbleService(rows)
		if err != nil {
			return nil, fmt.Errorf("scrobble_store list enabled services scan: %w", err)
		}
		svcs = append(svcs, svc)
	}
	return svcs, rows.Err()
}

func (s *sqliteScrobbleStore) UpdateService(ctx context.Context, svc *model.ScrobbleService) error {
	const q = `UPDATE scrobble_services SET
		display_name = ?, base_url = ?, api_key = ?, api_secret_enc = ?,
		session_key_enc = ?, token_enc = ?, username = ?, enabled = ?,
		updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q,
		svc.DisplayName, svc.BaseURL, svc.APIKey, svc.APISecretEnc,
		svc.SessionKeyEnc, svc.TokenEnc, svc.Username, boolToInt(svc.Enabled),
		svc.ID,
	)
	if err != nil {
		return fmt.Errorf("scrobble_store update service: %w", err)
	}
	return nil
}

func (s *sqliteScrobbleStore) DeleteService(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM scrobble_services WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("scrobble_store delete service: %w", err)
	}
	return nil
}

func (s *sqliteScrobbleStore) SetServiceVerified(ctx context.Context, id int64, verified bool, lastErr string) error {
	var verifiedAt any
	if verified {
		verifiedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE scrobble_services SET verified = ?, last_verified_at = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		boolToInt(verified), verifiedAt, lastErr, id,
	)
	if err != nil {
		return fmt.Errorf("scrobble_store set verified: %w", err)
	}
	return nil
}

func (s *sqliteScrobbleStore) EnqueueScrobble(ctx context.Context, q *model.ScrobbleQueueEntry) error {
	data, err := json.Marshal(q.TrackData)
	if err != nil {
		return fmt.Errorf("scrobble_store enqueue: marshal track data: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO scrobble_queue (user_id, service_id, track_data, queued_at) VALUES (?, ?, ?, ?)`,
		q.UserID, q.ServiceID, string(data), time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("scrobble_store enqueue: %w", err)
	}
	return nil
}

func (s *sqliteScrobbleStore) ListPendingByService(ctx context.Context, serviceID int64, limit int) ([]*model.ScrobbleQueueEntry, error) {
	const q = `SELECT id, user_id, service_id, track_data, queued_at, attempts, last_attempt_at, last_error
		FROM scrobble_queue
		WHERE service_id = ? AND attempts < 50 AND queued_at > datetime('now', '-14 days')
		ORDER BY queued_at ASC
		LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, serviceID, limit)
	if err != nil {
		return nil, fmt.Errorf("scrobble_store list pending: %w", err)
	}
	defer rows.Close()

	var entries []*model.ScrobbleQueueEntry
	for rows.Next() {
		var entry model.ScrobbleQueueEntry
		var trackDataStr string
		var queuedAt, lastAttemptAt sql.NullString
		err := rows.Scan(
			&entry.ID, &entry.UserID, &entry.ServiceID, &trackDataStr,
			&queuedAt, &entry.Attempts, &lastAttemptAt, &entry.LastError,
		)
		if err != nil {
			return nil, fmt.Errorf("scrobble_store list pending scan: %w", err)
		}
		if err := json.Unmarshal([]byte(trackDataStr), &entry.TrackData); err != nil {
			return nil, fmt.Errorf("scrobble_store list pending unmarshal: %w", err)
		}
		if queuedAt.Valid {
			entry.QueuedAt, _ = time.Parse(time.RFC3339, queuedAt.String)
		}
		if lastAttemptAt.Valid && lastAttemptAt.String != "" {
			entry.LastAttemptAt, _ = time.Parse(time.RFC3339, lastAttemptAt.String)
		}
		entries = append(entries, &entry)
	}
	return entries, rows.Err()
}

func (s *sqliteScrobbleStore) DeleteQueueEntry(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM scrobble_queue WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("scrobble_store delete queue entry: %w", err)
	}
	return nil
}

func (s *sqliteScrobbleStore) IncrementAttempts(ctx context.Context, id int64, lastErr string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE scrobble_queue SET attempts = attempts + 1, last_attempt_at = ?, last_error = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), lastErr, id,
	)
	if err != nil {
		return fmt.Errorf("scrobble_store increment attempts: %w", err)
	}
	return nil
}

func (s *sqliteScrobbleStore) PurgeStaleQueue(ctx context.Context, before time.Time, maxAttempts int) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM scrobble_queue WHERE attempts >= ? OR queued_at < ?`,
		maxAttempts, before.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("scrobble_store purge stale queue: %w", err)
	}
	return nil
}

// boolToInt converts a bool to 0 or 1 for SQLite storage.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
