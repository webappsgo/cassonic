package model

import "time"

// User represents a cassonic user account
type User struct {
	ID           int64  `db:"id"`
	Username     string `db:"username"`
	Email        string `db:"email"`
	PasswordHash string `db:"password_hash"`
	DisplayName  string `db:"display_name"`
	IsAdmin      bool   `db:"is_admin"`
	IsEnabled    bool   `db:"is_enabled"`
	AvatarURL    string `db:"avatar_url"`
	// Language stores the user's preferred locale code
	Language string `db:"language"`
	// Theme stores the user's preferred color theme: "dark", "light", or "auto"
	Theme string `db:"theme"`
	// MaxBitRate caps streaming quality in kbps; 0 means unlimited
	MaxBitRate int `db:"max_bit_rate"`
	// Subsonic-compatible permission flags
	CanDownload    bool `db:"can_download"`
	CanUpload      bool `db:"can_upload"`
	CanShare       bool `db:"can_share"`
	CanManageUsers bool `db:"can_manage_users"`
	CanComment     bool `db:"can_comment"`
	CanPodcast     bool `db:"can_podcast"`
	// TOTPSecret holds the base32-encoded TOTP secret for MFA
	TOTPSecret  string `db:"totp_secret"`
	TOTPEnabled bool   `db:"totp_enabled"`
	// LastLoginAt is the timestamp of the most recent successful login
	LastLoginAt time.Time `db:"last_login_at"`
	// LoginAttempts tracks consecutive failed login attempts
	LoginAttempts int `db:"login_attempts"`
	// LockedUntil is the time after which the account becomes accessible again
	LockedUntil time.Time `db:"locked_until"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// IsLocked returns true if the account is currently locked out
func (u *User) IsLocked() bool {
	if u.LockedUntil.IsZero() {
		return false
	}
	return time.Now().Before(u.LockedUntil)
}

// APIToken represents a long-lived API token for a user
type APIToken struct {
	ID     int64  `db:"id"`
	UserID int64  `db:"user_id"`
	// TokenHash is the SHA-256 hash of the raw token; the raw value is never stored
	TokenHash  string    `db:"token_hash"`
	Name       string    `db:"name"`
	LastUsedAt time.Time `db:"last_used_at"`
	// ExpiresAt is the token expiry; zero value means the token never expires
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
}

// IsExpired returns true if the token has passed its expiry time
func (t *APIToken) IsExpired() bool {
	if t.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(t.ExpiresAt)
}

// ScanStatus tracks a library scan operation
type ScanStatus struct {
	ID          int64     `db:"id"`
	StartedAt   time.Time `db:"started_at"`
	FinishedAt  time.Time `db:"finished_at"`
	// Status is one of "running", "completed", or "failed"
	Status       string `db:"status"`
	ScannedFiles int    `db:"scanned_files"`
	AddedFiles   int    `db:"added_files"`
	UpdatedFiles int    `db:"updated_files"`
	DeletedFiles int    `db:"deleted_files"`
	ErrorCount   int    `db:"error_count"`
	LastError    string `db:"last_error"`
}

// InternetRadioStation represents a manually-added internet radio stream
type InternetRadioStation struct {
	ID          int64     `db:"id"`
	Name        string    `db:"name"`
	StreamURL   string    `db:"stream_url"`
	HomepageURL string    `db:"homepage_url"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}
