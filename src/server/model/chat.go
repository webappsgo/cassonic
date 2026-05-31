package model

import "time"

// ChatMessage represents a Subsonic-compatible chat message posted by a user
type ChatMessage struct {
	ID     int64  `db:"id"`
	UserID int64  `db:"user_id"`
	// Username is denormalized from the user row to avoid joins on message retrieval
	Username  string    `db:"username"`
	Message   string    `db:"message"`
	CreatedAt time.Time `db:"created_at"`
}
