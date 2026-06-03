package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/local/cassonic/src/server/model"
)

// sqliteUserStore implements UserStore against users.db.
type sqliteUserStore struct {
	db *sql.DB
}

// nullableTime converts a potentially zero time.Time to a *string for SQLite storage.
// A zero value produces nil, which maps to a NULL column.
func nullableTime(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

// parseNullTime parses a nullable RFC3339 string column into a time.Time.
// A NULL column yields a zero time.Time.
func parseNullTime(ns sql.NullString) (time.Time, error) {
	if !ns.Valid || ns.String == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, ns.String)
}

// scanUser reads a full users row into a model.User.
func scanUser(row interface {
	Scan(...any) error
}) (*model.User, error) {
	var u model.User
	var lastLogin, lockedUntil, createdAt, updatedAt sql.NullString

	err := row.Scan(
		&u.ID,
		&u.Username,
		&u.Email,
		&u.PasswordHash,
		&u.DisplayName,
		&u.IsAdmin,
		&u.IsEnabled,
		&u.AvatarURL,
		&u.Language,
		&u.Theme,
		&u.MaxBitRate,
		&u.CanDownload,
		&u.CanUpload,
		&u.CanShare,
		&u.CanManageUsers,
		&u.CanComment,
		&u.CanPodcast,
		&u.TOTPSecret,
		&u.TOTPEnabled,
		&lastLogin,
		&u.LoginAttempts,
		&lockedUntil,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if u.LastLoginAt, err = parseNullTime(lastLogin); err != nil {
		return nil, fmt.Errorf("parse last_login_at: %w", err)
	}
	if u.LockedUntil, err = parseNullTime(lockedUntil); err != nil {
		return nil, fmt.Errorf("parse locked_until: %w", err)
	}
	if u.CreatedAt, err = parseNullTime(createdAt); err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	if u.UpdatedAt, err = parseNullTime(updatedAt); err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &u, nil
}

// userSelectCols lists every column fetched by scanUser in the same order.
// The trailing space is required: callers concatenate this directly with "FROM".
const userSelectCols = `
    id, username, email, password_hash, display_name,
    is_admin, is_enabled, avatar_url, language, theme,
    max_bit_rate, can_download, can_upload, can_share, can_manage_users,
    can_comment, can_podcast, totp_secret, totp_enabled,
    last_login_at, login_attempts, locked_until, created_at, updated_at `

// CreateUser inserts a new user and returns the assigned ID.
func (s *sqliteUserStore) CreateUser(ctx context.Context, u *model.User) (int64, error) {
	const q = `
    INSERT INTO users (
        username, email, password_hash, display_name,
        is_admin, is_enabled, avatar_url, language, theme,
        max_bit_rate, can_download, can_upload, can_share, can_manage_users,
        can_comment, can_podcast, totp_secret, totp_enabled
    ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

	res, err := s.db.ExecContext(ctx, q,
		u.Username, u.Email, u.PasswordHash, u.DisplayName,
		u.IsAdmin, u.IsEnabled, u.AvatarURL, u.Language, u.Theme,
		u.MaxBitRate, u.CanDownload, u.CanUpload, u.CanShare, u.CanManageUsers,
		u.CanComment, u.CanPodcast, u.TOTPSecret, u.TOTPEnabled,
	)
	if err != nil {
		return 0, fmt.Errorf("create user: %w", err)
	}
	return res.LastInsertId()
}

// GetUser fetches a user by primary key.
func (s *sqliteUserStore) GetUser(ctx context.Context, id int64) (*model.User, error) {
	q := `SELECT` + userSelectCols + `FROM users WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user %d: %w", id, err)
	}
	return u, nil
}

// GetUserByUsername fetches a user by their unique username.
func (s *sqliteUserStore) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	q := `SELECT` + userSelectCols + `FROM users WHERE username = ?`
	row := s.db.QueryRowContext(ctx, q, username)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return u, nil
}

// GetUserByEmail fetches a user by their unique email address.
func (s *sqliteUserStore) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	q := `SELECT` + userSelectCols + `FROM users WHERE email = ?`
	row := s.db.QueryRowContext(ctx, q, email)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

// UpdateUser writes all mutable fields back to the database.
func (s *sqliteUserStore) UpdateUser(ctx context.Context, u *model.User) error {
	const q = `
    UPDATE users SET
        username = ?, email = ?, password_hash = ?, display_name = ?,
        is_admin = ?, is_enabled = ?, avatar_url = ?, language = ?, theme = ?,
        max_bit_rate = ?, can_download = ?, can_upload = ?, can_share = ?,
        can_manage_users = ?, can_comment = ?, can_podcast = ?,
        totp_secret = ?, totp_enabled = ?,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = ?`

	_, err := s.db.ExecContext(ctx, q,
		u.Username, u.Email, u.PasswordHash, u.DisplayName,
		u.IsAdmin, u.IsEnabled, u.AvatarURL, u.Language, u.Theme,
		u.MaxBitRate, u.CanDownload, u.CanUpload, u.CanShare,
		u.CanManageUsers, u.CanComment, u.CanPodcast,
		u.TOTPSecret, u.TOTPEnabled,
		u.ID,
	)
	if err != nil {
		return fmt.Errorf("update user %d: %w", u.ID, err)
	}
	return nil
}

// DeleteUser permanently removes a user and all cascade-deleted rows.
func (s *sqliteUserStore) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user %d: %w", id, err)
	}
	return nil
}

// ListUsers returns all users ordered by username.
func (s *sqliteUserStore) ListUsers(ctx context.Context) ([]*model.User, error) {
	q := `SELECT` + userSelectCols + `FROM users ORDER BY username`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// IncrementLoginAttempts adds 1 to the failed login counter for the user.
func (s *sqliteUserStore) IncrementLoginAttempts(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET login_attempts = login_attempts + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("increment login attempts %d: %w", id, err)
	}
	return nil
}

// ResetLoginAttempts clears the failed login counter and removes any lockout.
func (s *sqliteUserStore) ResetLoginAttempts(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET login_attempts = 0, locked_until = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("reset login attempts %d: %w", id, err)
	}
	return nil
}

// SetLockedUntil sets the account lockout expiry.
func (s *sqliteUserStore) SetLockedUntil(ctx context.Context, id int64, until time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET locked_until = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		nullableTime(until), id,
	)
	if err != nil {
		return fmt.Errorf("set locked until %d: %w", id, err)
	}
	return nil
}

// UpdateLastLogin stamps the current time as the most recent successful login.
func (s *sqliteUserStore) UpdateLastLogin(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET last_login_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("update last login %d: %w", id, err)
	}
	return nil
}

// scanAPIToken reads a full api_tokens row into a model.APIToken.
func scanAPIToken(row interface {
	Scan(...any) error
}) (*model.APIToken, error) {
	var t model.APIToken
	var lastUsed, expiresAt, createdAt sql.NullString

	err := row.Scan(
		&t.ID,
		&t.UserID,
		&t.TokenHash,
		&t.Name,
		&lastUsed,
		&expiresAt,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	if t.LastUsedAt, err = parseNullTime(lastUsed); err != nil {
		return nil, fmt.Errorf("parse last_used_at: %w", err)
	}
	if t.ExpiresAt, err = parseNullTime(expiresAt); err != nil {
		return nil, fmt.Errorf("parse expires_at: %w", err)
	}
	if t.CreatedAt, err = parseNullTime(createdAt); err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	return &t, nil
}

// CreateAPIToken inserts a new API token record.
func (s *sqliteUserStore) CreateAPIToken(ctx context.Context, t *model.APIToken) error {
	const q = `
    INSERT INTO api_tokens (user_id, token_hash, name, expires_at)
    VALUES (?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, q,
		t.UserID, t.TokenHash, t.Name, nullableTime(t.ExpiresAt),
	)
	if err != nil {
		return fmt.Errorf("create api token: %w", err)
	}
	return nil
}

// GetAPITokenByHash returns the token matching the given SHA-256 hash.
// Returns nil, nil when no matching unexpired token is found.
func (s *sqliteUserStore) GetAPITokenByHash(ctx context.Context, hash string) (*model.APIToken, error) {
	const q = `
    SELECT id, user_id, token_hash, name, last_used_at, expires_at, created_at
    FROM api_tokens
    WHERE token_hash = ?
      AND (expires_at IS NULL OR expires_at > datetime('now'))`

	row := s.db.QueryRowContext(ctx, q, hash)
	t, err := scanAPIToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get api token by hash: %w", err)
	}
	return t, nil
}

// ListAPITokens returns all tokens owned by the specified user.
func (s *sqliteUserStore) ListAPITokens(ctx context.Context, userID int64) ([]*model.APIToken, error) {
	const q = `
    SELECT id, user_id, token_hash, name, last_used_at, expires_at, created_at
    FROM api_tokens
    WHERE user_id = ?
    ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*model.APIToken
	for rows.Next() {
		t, err := scanAPIToken(rows)
		if err != nil {
			return nil, fmt.Errorf("scan api token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// DeleteAPIToken removes a single token by ID.
func (s *sqliteUserStore) DeleteAPIToken(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete api token %d: %w", id, err)
	}
	return nil
}

// UpdateAPITokenLastUsed stamps the current time on the token.
func (s *sqliteUserStore) UpdateAPITokenLastUsed(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE api_tokens SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("update api token last used %d: %w", id, err)
	}
	return nil
}

// CreateSession persists a new opaque session token stored as its SHA-256 hash.
func (s *sqliteUserStore) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, clientName string) error {
	const q = `
    INSERT INTO sessions (token_hash, user_id, client_name, expires_at)
    VALUES (?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, q,
		tokenHash, userID, clientName, expiresAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// GetSessionByHash returns the session for the given token hash.
// Returns nil, nil when no matching session exists.
func (s *sqliteUserStore) GetSessionByHash(ctx context.Context, tokenHash string) (*Session, error) {
	const q = `
    SELECT token_hash, user_id, client_name, created_at, expires_at
    FROM sessions
    WHERE token_hash = ?`

	row := s.db.QueryRowContext(ctx, q, tokenHash)

	var sess Session
	var createdAt, expiresAt sql.NullString

	err := row.Scan(&sess.TokenHash, &sess.UserID, &sess.ClientName, &createdAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session by hash: %w", err)
	}

	if sess.CreatedAt, err = parseNullTime(createdAt); err != nil {
		return nil, fmt.Errorf("parse session created_at: %w", err)
	}
	if sess.ExpiresAt, err = parseNullTime(expiresAt); err != nil {
		return nil, fmt.Errorf("parse session expires_at: %w", err)
	}

	return &sess, nil
}

// DeleteSession removes a single session by token hash.
func (s *sqliteUserStore) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteUserSessions removes all sessions belonging to a user.
func (s *sqliteUserStore) DeleteUserSessions(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete user sessions %d: %w", userID, err)
	}
	return nil
}

// PurgeExpiredSessions deletes all session rows whose expires_at is in the past.
func (s *sqliteUserStore) PurgeExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at < datetime('now')`,
	)
	if err != nil {
		return fmt.Errorf("purge expired sessions: %w", err)
	}
	return nil
}

// GetSubsonicPassword returns the encrypted subsonic password for the named user.
// Returns ("", false, nil) when no subsonic password has been set.
func (s *sqliteUserStore) GetSubsonicPassword(ctx context.Context, username string) (string, bool, error) {
	var enc string
	err := s.db.QueryRowContext(ctx,
		"SELECT subsonic_password FROM users WHERE username = ? AND is_enabled = 1",
		username).Scan(&enc)
	if errors.Is(err, sql.ErrNoRows) || enc == "" {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get subsonic password: %w", err)
	}
	return enc, true, nil
}

// SetSubsonicPassword stores an encrypted subsonic password for the named user.
func (s *sqliteUserStore) SetSubsonicPassword(ctx context.Context, username string, encrypted string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET subsonic_password = ?, updated_at = CURRENT_TIMESTAMP WHERE username = ?",
		encrypted, username)
	if err != nil {
		return fmt.Errorf("set subsonic password: %w", err)
	}
	return nil
}

// scanRadioStation reads a full internet_radio_stations row.
func scanRadioStation(row interface {
	Scan(...any) error
}) (*model.InternetRadioStation, error) {
	var s model.InternetRadioStation
	var createdAt, updatedAt sql.NullString

	err := row.Scan(
		&s.ID,
		&s.Name,
		&s.StreamURL,
		&s.HomepageURL,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if s.CreatedAt, err = parseNullTime(createdAt); err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	if s.UpdatedAt, err = parseNullTime(updatedAt); err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &s, nil
}

// CreateRadioStation inserts a new internet radio station.
func (s *sqliteUserStore) CreateRadioStation(ctx context.Context, st *model.InternetRadioStation) (int64, error) {
	const q = `
    INSERT INTO internet_radio_stations (name, stream_url, homepage_url)
    VALUES (?, ?, ?)`

	res, err := s.db.ExecContext(ctx, q, st.Name, st.StreamURL, st.HomepageURL)
	if err != nil {
		return 0, fmt.Errorf("create radio station: %w", err)
	}
	return res.LastInsertId()
}

// GetRadioStation fetches a radio station by primary key.
func (s *sqliteUserStore) GetRadioStation(ctx context.Context, id int64) (*model.InternetRadioStation, error) {
	const q = `
    SELECT id, name, stream_url, homepage_url, created_at, updated_at
    FROM internet_radio_stations
    WHERE id = ?`

	row := s.db.QueryRowContext(ctx, q, id)
	st, err := scanRadioStation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get radio station %d: %w", id, err)
	}
	return st, nil
}

// ListRadioStations returns all radio stations ordered by name.
func (s *sqliteUserStore) ListRadioStations(ctx context.Context) ([]*model.InternetRadioStation, error) {
	const q = `
    SELECT id, name, stream_url, homepage_url, created_at, updated_at
    FROM internet_radio_stations
    ORDER BY name`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list radio stations: %w", err)
	}
	defer rows.Close()

	var stations []*model.InternetRadioStation
	for rows.Next() {
		st, err := scanRadioStation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan radio station: %w", err)
		}
		stations = append(stations, st)
	}
	return stations, rows.Err()
}

// UpdateRadioStation writes all mutable fields back to the database.
func (s *sqliteUserStore) UpdateRadioStation(ctx context.Context, st *model.InternetRadioStation) error {
	const q = `
    UPDATE internet_radio_stations
    SET name = ?, stream_url = ?, homepage_url = ?, updated_at = CURRENT_TIMESTAMP
    WHERE id = ?`

	_, err := s.db.ExecContext(ctx, q, st.Name, st.StreamURL, st.HomepageURL, st.ID)
	if err != nil {
		return fmt.Errorf("update radio station %d: %w", st.ID, err)
	}
	return nil
}

// DeleteRadioStation permanently removes a radio station.
func (s *sqliteUserStore) DeleteRadioStation(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM internet_radio_stations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete radio station %d: %w", id, err)
	}
	return nil
}

// sqliteChatStore implements ChatStore against users.db.
type sqliteChatStore struct {
	db *sql.DB
}

// AddMessage appends a chat message to the store.
func (s *sqliteChatStore) AddMessage(ctx context.Context, msg *model.ChatMessage) error {
	const q = `
    INSERT INTO chat_messages (user_id, username, message)
    VALUES (?, ?, ?)`

	_, err := s.db.ExecContext(ctx, q, msg.UserID, msg.Username, msg.Message)
	if err != nil {
		return fmt.Errorf("add chat message: %w", err)
	}
	return nil
}

// GetMessages returns all messages posted at or after since, ordered by created_at ASC.
func (s *sqliteChatStore) GetMessages(ctx context.Context, since time.Time) ([]*model.ChatMessage, error) {
	const q = `
    SELECT id, user_id, username, message, created_at
    FROM chat_messages
    WHERE created_at >= ?
    ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, q, since.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("get chat messages: %w", err)
	}
	defer rows.Close()

	var msgs []*model.ChatMessage
	for rows.Next() {
		var m model.ChatMessage
		var createdAt sql.NullString

		if err := rows.Scan(&m.ID, &m.UserID, &m.Username, &m.Message, &createdAt); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}

		if m.CreatedAt, err = parseNullTime(createdAt); err != nil {
			return nil, fmt.Errorf("parse chat message created_at: %w", err)
		}

		msgs = append(msgs, &m)
	}
	return msgs, rows.Err()
}

// PurgeOldMessages deletes all messages created strictly before before.
func (s *sqliteChatStore) PurgeOldMessages(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM chat_messages WHERE created_at < ?`,
		before.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("purge old chat messages: %w", err)
	}
	return nil
}
