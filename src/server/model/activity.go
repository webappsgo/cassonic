package model

import "time"

// Star represents a user's starred (favorited) item
type Star struct {
	ID int64 `db:"id"`
	UserID int64 `db:"user_id"`
	// ItemType is one of "song", "album", or "artist"
	ItemType  string    `db:"item_type"`
	ItemID    int64     `db:"item_id"`
	StarredAt time.Time `db:"starred_at"`
}

// Rating represents a user's 1–5 star rating for an item
type Rating struct {
	ID int64 `db:"id"`
	UserID int64 `db:"user_id"`
	// ItemType is one of "song" or "album"
	ItemType string `db:"item_type"`
	ItemID   int64  `db:"item_id"`
	// Rating is an integer in the range 1–5 inclusive
	Rating    int       `db:"rating"`
	UpdatedAt time.Time `db:"updated_at"`
}

// PlayHistory records each play event for a user
type PlayHistory struct {
	ID       int64     `db:"id"`
	UserID   int64     `db:"user_id"`
	SongID   int64     `db:"song_id"`
	PlayedAt time.Time `db:"played_at"`
	// ListenedFor is the number of seconds the user actually listened;
	// used to determine whether the play qualifies for scrobbling
	ListenedFor int    `db:"listened_for"`
	ClientName  string `db:"client_name"`
	Scrobbled   bool   `db:"scrobbled"`
}

// Bookmark saves a playback position so the user can resume later
type Bookmark struct {
	ID int64 `db:"id"`
	UserID int64 `db:"user_id"`
	// ItemType is one of "song" or "episode"
	ItemType string `db:"item_type"`
	ItemID   int64  `db:"item_id"`
	// Position is the saved playback offset in milliseconds
	Position  int64     `db:"position"`
	Comment   string    `db:"comment"`
	UpdatedAt time.Time `db:"updated_at"`
}

// PlayQueue represents the current play queue for a user across clients
type PlayQueue struct {
	ID     int64 `db:"id"`
	UserID int64 `db:"user_id"`
	// Current is the song ID of the track currently playing
	Current int64 `db:"current"`
	// Position is the playback offset into the current song in milliseconds
	Position  int64     `db:"position"`
	UpdatedAt time.Time `db:"updated_at"`
	// ChangedBy identifies the client that last modified the queue
	ChangedBy string `db:"changed_by"`
}

// PlayQueueEntry is one track in a play queue
type PlayQueueEntry struct {
	ID          int64 `db:"id"`
	PlayQueueID int64 `db:"play_queue_id"`
	SongID      int64 `db:"song_id"`
	// Position is the 0-indexed order of this entry in the queue
	Position int `db:"position"`
}
