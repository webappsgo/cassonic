package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/local/cassonic/src/server/model"
)

// sqliteAdminStore implements AdminStore. Admins and their preferences live
// in users.db; admin sessions and the audit log live in server.db (AI.md
// PART 12) — so this store, uniquely, spans both database files.
type sqliteAdminStore struct {
	usersDB  *sql.DB
	serverDB *sql.DB
}

// adminSelectCols lists every column fetched by scanAdmin in the same order.
const adminSelectCols = `
    id, username, password_hash, email, role, enabled, api_token_hash,
    source, external_id, groups, last_sync, last_login, failed_attempts,
    locked_until, totp_enabled, created_at, updated_at `

// scanAdmin reads a full admins row into a model.Admin.
func scanAdmin(row interface {
	Scan(...any) error
}) (*model.Admin, error) {
	var a model.Admin
	var lastSync, lastLogin, lockedUntil, createdAt, updatedAt sql.NullString

	err := row.Scan(
		&a.ID, &a.Username, &a.PasswordHash, &a.Email, &a.Role, &a.Enabled,
		&a.APITokenHash, &a.Source, &a.ExternalID, &a.Groups,
		&lastSync, &lastLogin, &a.FailedAttempts, &lockedUntil,
		&a.TOTPEnabled, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if a.LastSync, err = parseNullTime(lastSync); err != nil {
		return nil, fmt.Errorf("parse last_sync: %w", err)
	}
	if a.LastLogin, err = parseNullTime(lastLogin); err != nil {
		return nil, fmt.Errorf("parse last_login: %w", err)
	}
	if a.LockedUntil, err = parseNullTime(lockedUntil); err != nil {
		return nil, fmt.Errorf("parse locked_until: %w", err)
	}
	if a.CreatedAt, err = parseNullTime(createdAt); err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	if a.UpdatedAt, err = parseNullTime(updatedAt); err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &a, nil
}

// CreateAdmin inserts a new admin and a matching default preferences row,
// wrapped in a transaction so the two never go out of sync.
func (s *sqliteAdminStore) CreateAdmin(ctx context.Context, a *model.Admin) (int64, error) {
	tx, err := s.usersDB.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("create admin: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const q = `
    INSERT INTO admins (
        username, password_hash, email, role, enabled, api_token_hash,
        source, external_id, groups, totp_enabled
    ) VALUES (?,?,?,?,?,?,?,?,?,?)`

	res, err := tx.ExecContext(ctx, q,
		a.Username, a.PasswordHash, a.Email, a.Role, a.Enabled, a.APITokenHash,
		a.Source, a.ExternalID, a.Groups, a.TOTPEnabled,
	)
	if err != nil {
		return 0, fmt.Errorf("create admin: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create admin: last insert id: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO admin_preferences (admin_id) VALUES (?)`, id,
	); err != nil {
		return 0, fmt.Errorf("create admin preferences: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("create admin: commit: %w", err)
	}
	return id, nil
}

// GetAdmin fetches an admin by primary key.
func (s *sqliteAdminStore) GetAdmin(ctx context.Context, id int64) (*model.Admin, error) {
	q := `SELECT` + adminSelectCols + `FROM admins WHERE id = ?`
	a, err := scanAdmin(s.usersDB.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get admin %d: %w", id, err)
	}
	return a, nil
}

// GetAdminByUsername fetches an admin by their unique username.
func (s *sqliteAdminStore) GetAdminByUsername(ctx context.Context, username string) (*model.Admin, error) {
	q := `SELECT` + adminSelectCols + `FROM admins WHERE username = ?`
	a, err := scanAdmin(s.usersDB.QueryRowContext(ctx, q, username))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get admin by username: %w", err)
	}
	return a, nil
}

// GetAdminByExternalID fetches an externally synced admin by (source, external_id).
func (s *sqliteAdminStore) GetAdminByExternalID(ctx context.Context, source, externalID string) (*model.Admin, error) {
	q := `SELECT` + adminSelectCols + `FROM admins WHERE source = ? AND external_id = ?`
	a, err := scanAdmin(s.usersDB.QueryRowContext(ctx, q, source, externalID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get admin by external id: %w", err)
	}
	return a, nil
}

// UpdateAdmin writes all mutable fields back to the database.
func (s *sqliteAdminStore) UpdateAdmin(ctx context.Context, a *model.Admin) error {
	const q = `
    UPDATE admins SET
        username = ?, password_hash = ?, email = ?, role = ?, enabled = ?,
        api_token_hash = ?, source = ?, external_id = ?, groups = ?,
        totp_enabled = ?, updated_at = CURRENT_TIMESTAMP
    WHERE id = ?`

	_, err := s.usersDB.ExecContext(ctx, q,
		a.Username, a.PasswordHash, a.Email, a.Role, a.Enabled, a.APITokenHash,
		a.Source, a.ExternalID, a.Groups, a.TOTPEnabled,
		a.ID,
	)
	if err != nil {
		return fmt.Errorf("update admin %d: %w", a.ID, err)
	}
	return nil
}

// DeleteAdmin permanently removes an admin and its preferences row (cascade).
func (s *sqliteAdminStore) DeleteAdmin(ctx context.Context, id int64) error {
	_, err := s.usersDB.ExecContext(ctx, `DELETE FROM admins WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete admin %d: %w", id, err)
	}
	return nil
}

// ListAdmins returns all admins ordered by id (lowest id is the Primary Admin).
func (s *sqliteAdminStore) ListAdmins(ctx context.Context) ([]*model.Admin, error) {
	q := `SELECT` + adminSelectCols + `FROM admins ORDER BY id`
	rows, err := s.usersDB.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list admins: %w", err)
	}
	defer rows.Close()

	var admins []*model.Admin
	for rows.Next() {
		a, err := scanAdmin(rows)
		if err != nil {
			return nil, fmt.Errorf("scan admin: %w", err)
		}
		admins = append(admins, a)
	}
	return admins, rows.Err()
}

// CountAdmins returns the total number of admin accounts.
func (s *sqliteAdminStore) CountAdmins(ctx context.Context) (int, error) {
	var n int
	if err := s.usersDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM admins`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return n, nil
}

// IncrementAdminLoginAttempts adds 1 to the failed login counter.
func (s *sqliteAdminStore) IncrementAdminLoginAttempts(ctx context.Context, id int64) error {
	_, err := s.usersDB.ExecContext(ctx,
		`UPDATE admins SET failed_attempts = failed_attempts + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("increment admin login attempts %d: %w", id, err)
	}
	return nil
}

// ResetAdminLoginAttempts clears the failed login counter and any lockout.
func (s *sqliteAdminStore) ResetAdminLoginAttempts(ctx context.Context, id int64) error {
	_, err := s.usersDB.ExecContext(ctx,
		`UPDATE admins SET failed_attempts = 0, locked_until = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("reset admin login attempts %d: %w", id, err)
	}
	return nil
}

// SetAdminLockedUntil sets the account lockout expiry.
func (s *sqliteAdminStore) SetAdminLockedUntil(ctx context.Context, id int64, until time.Time) error {
	_, err := s.usersDB.ExecContext(ctx,
		`UPDATE admins SET locked_until = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		nullableTime(until), id,
	)
	if err != nil {
		return fmt.Errorf("set admin locked until %d: %w", id, err)
	}
	return nil
}

// UpdateAdminLastLogin stamps the current time as the most recent successful login.
func (s *sqliteAdminStore) UpdateAdminLastLogin(ctx context.Context, id int64) error {
	_, err := s.usersDB.ExecContext(ctx,
		`UPDATE admins SET last_login = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("update admin last login %d: %w", id, err)
	}
	return nil
}

// GetAdminPreferences returns the admin's WebUI preferences.
func (s *sqliteAdminStore) GetAdminPreferences(ctx context.Context, adminID int64) (*model.AdminPreferences, error) {
	const q = `
    SELECT admin_id, theme, font_size, reduce_motion, date_format, time_format,
        email_security, email_server, email_backups, email_users,
        created_at, updated_at
    FROM admin_preferences WHERE admin_id = ?`

	var p model.AdminPreferences
	var createdAt, updatedAt sql.NullString
	err := s.usersDB.QueryRowContext(ctx, q, adminID).Scan(
		&p.AdminID, &p.Theme, &p.FontSize, &p.ReduceMotion, &p.DateFormat, &p.TimeFormat,
		&p.EmailSecurity, &p.EmailServer, &p.EmailBackups, &p.EmailUsers,
		&createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get admin preferences %d: %w", adminID, err)
	}
	if p.CreatedAt, err = parseNullTime(createdAt); err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	if p.UpdatedAt, err = parseNullTime(updatedAt); err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}
	return &p, nil
}

// UpdateAdminPreferences writes all mutable preference fields back.
func (s *sqliteAdminStore) UpdateAdminPreferences(ctx context.Context, p *model.AdminPreferences) error {
	const q = `
    UPDATE admin_preferences SET
        theme = ?, font_size = ?, reduce_motion = ?, date_format = ?, time_format = ?,
        email_security = ?, email_server = ?, email_backups = ?, email_users = ?,
        updated_at = CURRENT_TIMESTAMP
    WHERE admin_id = ?`

	_, err := s.usersDB.ExecContext(ctx, q,
		p.Theme, p.FontSize, p.ReduceMotion, p.DateFormat, p.TimeFormat,
		p.EmailSecurity, p.EmailServer, p.EmailBackups, p.EmailUsers,
		p.AdminID,
	)
	if err != nil {
		return fmt.Errorf("update admin preferences %d: %w", p.AdminID, err)
	}
	return nil
}

// CreateAdminSession persists a new opaque admin session token, stored as its
// SHA-256 hash in the "id" column.
func (s *sqliteAdminStore) CreateAdminSession(ctx context.Context, sess *model.AdminSession) error {
	const q = `
    INSERT INTO admin_sessions (id, admin_id, ip_address, user_agent, expires_at)
    VALUES (?, ?, ?, ?, ?)`

	_, err := s.serverDB.ExecContext(ctx, q,
		sess.TokenHash, sess.AdminID, sess.IP, sess.UserAgent,
		sess.ExpiresAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create admin session: %w", err)
	}
	return nil
}

// GetAdminSessionByHash returns the session for the given token hash.
func (s *sqliteAdminStore) GetAdminSessionByHash(ctx context.Context, tokenHash string) (*model.AdminSession, error) {
	const q = `
    SELECT id, admin_id, ip_address, user_agent, created_at, expires_at, last_active
    FROM admin_sessions WHERE id = ?`

	var sess model.AdminSession
	var createdAt, expiresAt, lastActive sql.NullString

	err := s.serverDB.QueryRowContext(ctx, q, tokenHash).Scan(
		&sess.TokenHash, &sess.AdminID, &sess.IP, &sess.UserAgent,
		&createdAt, &expiresAt, &lastActive,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get admin session by hash: %w", err)
	}

	if sess.CreatedAt, err = parseNullTime(createdAt); err != nil {
		return nil, fmt.Errorf("parse session created_at: %w", err)
	}
	if sess.ExpiresAt, err = parseNullTime(expiresAt); err != nil {
		return nil, fmt.Errorf("parse session expires_at: %w", err)
	}
	if sess.LastActive, err = parseNullTime(lastActive); err != nil {
		return nil, fmt.Errorf("parse session last_active: %w", err)
	}
	return &sess, nil
}

// DeleteAdminSession removes a single session by token hash.
func (s *sqliteAdminStore) DeleteAdminSession(ctx context.Context, tokenHash string) error {
	_, err := s.serverDB.ExecContext(ctx, `DELETE FROM admin_sessions WHERE id = ?`, tokenHash)
	if err != nil {
		return fmt.Errorf("delete admin session: %w", err)
	}
	return nil
}

// DeleteAdminSessions removes all sessions belonging to an admin.
func (s *sqliteAdminStore) DeleteAdminSessions(ctx context.Context, adminID int64) error {
	_, err := s.serverDB.ExecContext(ctx, `DELETE FROM admin_sessions WHERE admin_id = ?`, adminID)
	if err != nil {
		return fmt.Errorf("delete admin sessions %d: %w", adminID, err)
	}
	return nil
}

// PurgeExpiredAdminSessions deletes all session rows whose expires_at is in the past.
func (s *sqliteAdminStore) PurgeExpiredAdminSessions(ctx context.Context) error {
	// expires_at is wrapped in datetime() because rows can be written either
	// as RFC3339 ("...T...Z", from CreateAdminSession/the sqlite driver's
	// CURRENT_TIMESTAMP conversion) or SQLite's own "YYYY-MM-DD HH:MM:SS"
	// format; datetime() normalizes both sides before comparing.
	_, err := s.serverDB.ExecContext(ctx, `DELETE FROM admin_sessions WHERE datetime(expires_at) < datetime('now')`)
	if err != nil {
		return fmt.Errorf("purge expired admin sessions: %w", err)
	}
	return nil
}

// AppendAuditEntry records a single admin action or security event.
func (s *sqliteAdminStore) AppendAuditEntry(ctx context.Context, e *model.AuditEntry) error {
	const q = `
    INSERT INTO audit_log (
        level, category, action, actor_type, actor_id, actor_ip,
        target_type, target_id, details, success
    ) VALUES (?,?,?,?,?,?,?,?,?,?)`

	_, err := s.serverDB.ExecContext(ctx, q,
		e.Level, e.Category, e.Action, e.ActorType, e.ActorID, e.ActorIP,
		e.TargetType, e.TargetID, e.Detail, e.Success,
	)
	if err != nil {
		return fmt.Errorf("append audit entry: %w", err)
	}
	return nil
}

// ListAuditEntries returns the most recent audit entries, newest first.
func (s *sqliteAdminStore) ListAuditEntries(ctx context.Context, limit int) ([]*model.AuditEntry, error) {
	const q = `
    SELECT id, timestamp, level, category, action, actor_type, actor_id,
        actor_ip, target_type, target_id, details, success
    FROM audit_log ORDER BY timestamp DESC LIMIT ?`

	rows, err := s.serverDB.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit entries: %w", err)
	}
	defer rows.Close()

	var entries []*model.AuditEntry
	for rows.Next() {
		var e model.AuditEntry
		var timestamp sql.NullString
		if err := rows.Scan(
			&e.ID, &timestamp, &e.Level, &e.Category, &e.Action, &e.ActorType,
			&e.ActorID, &e.ActorIP, &e.TargetType, &e.TargetID, &e.Detail, &e.Success,
		); err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		if e.CreatedAt, err = parseNullTime(timestamp); err != nil {
			return nil, fmt.Errorf("parse timestamp: %w", err)
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

// GenerateSetupToken creates the one-time, 128-bit (32 hex character) setup
// token (AI.md PART 17 "Setup Token Rules") and its SHA-256 hash for
// storage. The raw token is shown to the operator exactly once in the
// startup console banner and is never persisted in plaintext.
func GenerateSetupToken() (raw, hash string, err error) {
	b := make([]byte, 16)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("setup token: rand: %w", err)
	}
	raw = hex.EncodeToString(b)
	hash = HashSetupToken(raw)
	return raw, hash, nil
}

// HashSetupToken returns the SHA-256 hash of a raw setup token, for
// comparing an operator-submitted token against the stored hash.
func HashSetupToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// CreateSetupToken persists the one-time first-run setup token's hash.
func (s *sqliteAdminStore) CreateSetupToken(ctx context.Context, tokenHash string) error {
	const q = `INSERT INTO setup_token (id, token_hash) VALUES (1, ?)`
	if _, err := s.serverDB.ExecContext(ctx, q, tokenHash); err != nil {
		return fmt.Errorf("create setup token: %w", err)
	}
	return nil
}

// GetSetupToken returns the current setup token row, or nil, nil if one has
// never been generated.
func (s *sqliteAdminStore) GetSetupToken(ctx context.Context) (*model.SetupToken, error) {
	const q = `SELECT token_hash, created_at, used, used_at FROM setup_token WHERE id = 1`

	var t model.SetupToken
	var createdAt, usedAt sql.NullString

	err := s.serverDB.QueryRowContext(ctx, q).Scan(&t.TokenHash, &createdAt, &t.Used, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get setup token: %w", err)
	}
	if t.CreatedAt, err = parseNullTime(createdAt); err != nil {
		return nil, fmt.Errorf("parse setup token created_at: %w", err)
	}
	if t.UsedAt, err = parseNullTime(usedAt); err != nil {
		return nil, fmt.Errorf("parse setup token used_at: %w", err)
	}
	return &t, nil
}

// ConsumeSetupToken marks the setup token used (single-use), permanently
// invalidating it for any future setup attempt.
func (s *sqliteAdminStore) ConsumeSetupToken(ctx context.Context) error {
	const q = `UPDATE setup_token SET used = 1, used_at = CURRENT_TIMESTAMP WHERE id = 1`

	res, err := s.serverDB.ExecContext(ctx, q)
	if err != nil {
		return fmt.Errorf("consume setup token: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("consume setup token: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("consume setup token: no setup token has been generated")
	}
	return nil
}
