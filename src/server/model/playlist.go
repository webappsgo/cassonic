package model

import "time"

// Playlist represents a user-created playlist
type Playlist struct {
	ID      int64  `db:"id"`
	UserID  int64  `db:"user_id"`
	Name    string `db:"name"`
	Comment string `db:"comment"`
	IsPublic bool  `db:"is_public"`
	SongCount int  `db:"song_count"`
	// Duration is the total playlist length in seconds
	Duration   int       `db:"duration"`
	CoverArtID int64     `db:"cover_art_id"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// PlaylistEntry is one track in a playlist
type PlaylistEntry struct {
	ID         int64 `db:"id"`
	PlaylistID int64 `db:"playlist_id"`
	SongID     int64 `db:"song_id"`
	// Position is the 0-indexed order of this entry in the playlist
	Position int `db:"position"`
}
