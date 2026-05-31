package model

import "time"

// Share represents a public share link for a song, album, or playlist
type Share struct {
	ID     int64  `db:"id"`
	UserID int64  `db:"user_id"`
	// Token is the random URL-safe token embedded in the public share URL
	Token    string `db:"token"`
	// ItemType is one of "song", "album", or "playlist"
	ItemType    string `db:"item_type"`
	ItemID      int64  `db:"item_id"`
	Description string `db:"description"`
	// PasswordHash is the SHA-256 hash of the access password; empty means no password required
	PasswordHash string    `db:"password_hash"`
	ViewCount    int       `db:"view_count"`
	// ExpiresAt is the share expiry time; zero value means the share never expires
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// IsExpired returns true if the share has passed its expiry time
func (s *Share) IsExpired() bool {
	if s.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(s.ExpiresAt)
}

// HasPassword returns true if the share requires a password to access
func (s *Share) HasPassword() bool {
	return s.PasswordHash != ""
}
