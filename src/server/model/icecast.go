package model

import "time"

// StreamScope controls what content is streamed to an Icecast mount
type StreamScope string

const (
	// ScopeAll streams the entire library
	ScopeAll StreamScope = "all"
	// ScopeArtist streams only tracks by a specific artist
	ScopeArtist StreamScope = "artist"
	// ScopeGenre streams only tracks belonging to a specific genre
	ScopeGenre StreamScope = "genre"
)

// MountStatus represents the current state of an Icecast mount connection
type MountStatus string

const (
	StatusDisconnected MountStatus = "disconnected"
	StatusConnecting   MountStatus = "connecting"
	StatusConnected    MountStatus = "connected"
	StatusError        MountStatus = "error"
)

// StreamFormat represents the audio encoding format for Icecast output
type StreamFormat string

const (
	FormatMP3  StreamFormat = "mp3"
	FormatOGG  StreamFormat = "ogg"
	FormatAAC  StreamFormat = "aac"
	FormatOpus StreamFormat = "opus"
)

// IcecastServer represents a remote Icecast server connection configuration
type IcecastServer struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
	Host string `db:"host"`
	// Port is the Icecast HTTP port; typically 8000
	Port int `db:"port"`
	// Protocol is "http" or "https"
	Protocol   string `db:"protocol"`
	SourceUser string `db:"source_user"`
	// SourcePass is the Icecast source password stored encrypted
	SourcePass string    `db:"source_pass"`
	Enabled    bool      `db:"enabled"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// IcecastMount represents a configured streaming mount point on an Icecast server
type IcecastMount struct {
	ID       int64  `db:"id"`
	ServerID int64  `db:"server_id"`
	// MountPath is the Icecast mount path, e.g. "/cassonic"
	MountPath   string      `db:"mount_path"`
	Name        string      `db:"name"`
	Description string      `db:"description"`
	Scope       StreamScope `db:"scope"`
	// ArtistID identifies the artist to stream when Scope is ScopeArtist
	ArtistID int64 `db:"artist_id"`
	// Genre identifies the genre to stream when Scope is ScopeGenre
	Genre  string       `db:"genre"`
	Format StreamFormat `db:"format"`
	// BitRate is the target encoding bit rate in kbps
	BitRate int  `db:"bit_rate"`
	Shuffle bool `db:"shuffle"`
	Enabled bool `db:"enabled"`
	Status  MountStatus `db:"status"`
	// CurrentSong is the ICY metadata display string for the currently playing track
	CurrentSong string    `db:"current_song"`
	LastError   string    `db:"last_error"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}
