package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/local/cassonic/src/server/model"
)

// GetTOTPSecret returns the TOTP row for the given user, or nil, nil if none
// exists.
func (s *sqliteAdminStore) GetTOTPSecret(ctx context.Context, userType string, userID int64) (*model.TOTPSecret, error) {
	const q = `
    SELECT id, user_type, user_id, secret, enabled, backup_codes, created_at, last_used
    FROM totp_secrets WHERE user_type = ? AND user_id = ?`

	var t model.TOTPSecret
	var backupCodes sql.NullString
	var createdAt int64
	var lastUsed sql.NullInt64

	err := s.serverDB.QueryRowContext(ctx, q, userType, userID).Scan(
		&t.ID, &t.UserType, &t.UserID, &t.Secret, &t.Enabled, &backupCodes,
		&createdAt, &lastUsed,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get totp secret: %w", err)
	}

	t.BackupCodes = backupCodes.String
	t.CreatedAt = time.Unix(createdAt, 0).UTC()
	if lastUsed.Valid {
		t.LastUsed = time.Unix(lastUsed.Int64, 0).UTC()
	}
	return &t, nil
}

// EnableTOTP creates or replaces the enabled TOTP secret and backup codes for
// a user (AI.md PART 17 "TOTP Two-Factor Authentication" setup).
func (s *sqliteAdminStore) EnableTOTP(ctx context.Context, t *model.TOTPSecret) error {
	const q = `
    INSERT INTO totp_secrets (user_type, user_id, secret, enabled, backup_codes)
    VALUES (?, ?, ?, 1, ?)
    ON CONFLICT(user_type, user_id) DO UPDATE SET
        secret = excluded.secret,
        enabled = 1,
        backup_codes = excluded.backup_codes`

	_, err := s.serverDB.ExecContext(ctx, q, t.UserType, t.UserID, t.Secret, t.BackupCodes)
	if err != nil {
		return fmt.Errorf("enable totp: %w", err)
	}
	return nil
}

// DisableTOTP permanently removes a user's TOTP secret row.
func (s *sqliteAdminStore) DisableTOTP(ctx context.Context, userType string, userID int64) error {
	_, err := s.serverDB.ExecContext(ctx,
		`DELETE FROM totp_secrets WHERE user_type = ? AND user_id = ?`, userType, userID)
	if err != nil {
		return fmt.Errorf("disable totp: %w", err)
	}
	return nil
}

// UpdateTOTPBackupCodes replaces the stored (hashed) backup codes,
// invalidating all previously issued codes.
func (s *sqliteAdminStore) UpdateTOTPBackupCodes(ctx context.Context, userType string, userID int64, backupCodesJSON string) error {
	_, err := s.serverDB.ExecContext(ctx,
		`UPDATE totp_secrets SET backup_codes = ? WHERE user_type = ? AND user_id = ?`,
		backupCodesJSON, userType, userID)
	if err != nil {
		return fmt.Errorf("update totp backup codes: %w", err)
	}
	return nil
}

// TouchTOTPLastUsed stamps the current time as the most recent successful
// TOTP or backup-code verification.
func (s *sqliteAdminStore) TouchTOTPLastUsed(ctx context.Context, userType string, userID int64) error {
	_, err := s.serverDB.ExecContext(ctx,
		`UPDATE totp_secrets SET last_used = strftime('%s', 'now') WHERE user_type = ? AND user_id = ?`,
		userType, userID)
	if err != nil {
		return fmt.Errorf("touch totp last_used: %w", err)
	}
	return nil
}

// CreateAdminMFAChallenge persists a new "password verified, awaiting 2FA"
// login challenge, stored as its SHA-256 hash in the "id" column.
func (s *sqliteAdminStore) CreateAdminMFAChallenge(ctx context.Context, c *model.AdminMFAChallenge) error {
	const q = `
    INSERT INTO admin_mfa_challenges (id, admin_id, ip_address, user_agent, remember, expires_at)
    VALUES (?, ?, ?, ?, ?, ?)`

	_, err := s.serverDB.ExecContext(ctx, q,
		c.TokenHash, c.AdminID, c.IP, c.UserAgent, c.Remember,
		c.ExpiresAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create admin mfa challenge: %w", err)
	}
	return nil
}

// GetAdminMFAChallengeByHash returns the challenge for the given token hash.
func (s *sqliteAdminStore) GetAdminMFAChallengeByHash(ctx context.Context, tokenHash string) (*model.AdminMFAChallenge, error) {
	const q = `
    SELECT id, admin_id, ip_address, user_agent, remember, created_at, expires_at
    FROM admin_mfa_challenges WHERE id = ?`

	var c model.AdminMFAChallenge
	var createdAt, expiresAt sql.NullString

	err := s.serverDB.QueryRowContext(ctx, q, tokenHash).Scan(
		&c.TokenHash, &c.AdminID, &c.IP, &c.UserAgent, &c.Remember,
		&createdAt, &expiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get admin mfa challenge by hash: %w", err)
	}

	if c.CreatedAt, err = parseNullTime(createdAt); err != nil {
		return nil, fmt.Errorf("parse mfa challenge created_at: %w", err)
	}
	if c.ExpiresAt, err = parseNullTime(expiresAt); err != nil {
		return nil, fmt.Errorf("parse mfa challenge expires_at: %w", err)
	}
	return &c, nil
}

// DeleteAdminMFAChallenge removes a single challenge by token hash.
func (s *sqliteAdminStore) DeleteAdminMFAChallenge(ctx context.Context, tokenHash string) error {
	_, err := s.serverDB.ExecContext(ctx, `DELETE FROM admin_mfa_challenges WHERE id = ?`, tokenHash)
	if err != nil {
		return fmt.Errorf("delete admin mfa challenge: %w", err)
	}
	return nil
}

// PurgeExpiredAdminMFAChallenges deletes all challenge rows whose expires_at
// is in the past.
func (s *sqliteAdminStore) PurgeExpiredAdminMFAChallenges(ctx context.Context) error {
	_, err := s.serverDB.ExecContext(ctx,
		`DELETE FROM admin_mfa_challenges WHERE datetime(expires_at) < datetime('now')`)
	if err != nil {
		return fmt.Errorf("purge expired admin mfa challenges: %w", err)
	}
	return nil
}
