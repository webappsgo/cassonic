package model

import "time"

// ServiceType identifies a scrobbling backend
type ServiceType string

const (
	ServiceLastFM       ServiceType = "lastfm"
	ServiceLibreFM      ServiceType = "librefm"
	ServiceGnuFM        ServiceType = "gnufm"
	ServiceListenBrainz ServiceType = "listenbrainz"
	ServiceMaloja       ServiceType = "maloja"
	// ServiceCustomLastFM is a user-supplied Last.fm-protocol-compatible server
	ServiceCustomLastFM ServiceType = "custom_lastfm"
	// ServiceCustomListenBrainz is a user-supplied ListenBrainz-protocol-compatible server
	ServiceCustomListenBrainz ServiceType = "custom_listenbrainz"
)

// Protocol returns the wire protocol used by this service type.
// "lastfm" denotes the Last.fm HMAC-MD5 authentication protocol.
// "listenbrainz" denotes the ListenBrainz Bearer token + JSON protocol.
func (s ServiceType) Protocol() string {
	switch s {
	case ServiceListenBrainz, ServiceMaloja, ServiceCustomListenBrainz:
		return "listenbrainz"
	default:
		return "lastfm"
	}
}

// BaseURL returns the canonical API base URL for well-known service types.
// Custom service types return "" and the caller must use the stored base_url field.
func (s ServiceType) BaseURL() string {
	switch s {
	case ServiceLastFM:
		return "https://ws.audioscrobbler.com/2.0/"
	case ServiceLibreFM:
		return "https://libre.fm/2.0/"
	case ServiceGnuFM:
		return "https://gnufm.libresource.org/2.0/"
	case ServiceListenBrainz:
		return "https://api.listenbrainz.org"
	default:
		return ""
	}
}

// ScrobbleService represents a configured scrobbling endpoint for a user
type ScrobbleService struct {
	ID          int64       `db:"id"`
	UserID      int64       `db:"user_id"`
	ServiceType ServiceType `db:"service_type"`
	DisplayName string      `db:"display_name"`
	// BaseURL is the API endpoint; for well-known services use ServiceType.BaseURL().
	// For custom_lastfm and custom_listenbrainz this is the user-supplied URL.
	BaseURL string `db:"base_url"`
	APIKey  string `db:"api_key"`
	// APISecretEnc holds the AES-256-GCM encrypted API secret
	APISecretEnc string `db:"api_secret_enc"`
	// SessionKeyEnc holds the AES-256-GCM encrypted session key
	SessionKeyEnc string `db:"session_key_enc"`
	// TokenEnc holds the AES-256-GCM encrypted authentication token
	TokenEnc       string      `db:"token_enc"`
	Username       string      `db:"username"`
	Enabled        bool        `db:"enabled"`
	Verified       bool        `db:"verified"`
	LastVerifiedAt time.Time   `db:"last_verified_at"`
	LastError      string      `db:"last_error"`
	CreatedAt      time.Time   `db:"created_at"`
	UpdatedAt      time.Time   `db:"updated_at"`
}

// ScrobbleTrackData holds the track metadata sent to a scrobbling service
type ScrobbleTrackData struct {
	Artist      string `json:"artist"`
	Track       string `json:"track"`
	Album       string `json:"album"`
	AlbumArtist string `json:"album_artist,omitempty"`
	// Duration is the track length in seconds
	Duration    int    `json:"duration"`
	TrackNumber int    `json:"track_number,omitempty"`
	// MBID is the MusicBrainz recording ID for the track
	MBID      string `json:"mbid,omitempty"`
	// Timestamp is the Unix epoch second when the track started playing
	Timestamp int64  `json:"timestamp"`
}

// ScrobbleQueueEntry is a pending scrobble awaiting delivery to a service
type ScrobbleQueueEntry struct {
	ID        int64             `db:"id"`
	UserID    int64             `db:"user_id"`
	ServiceID int64             `db:"service_id"`
	// TrackData is serialized as JSON in the database column
	TrackData     ScrobbleTrackData `db:"track_data"`
	QueuedAt      time.Time         `db:"queued_at"`
	Attempts      int               `db:"attempts"`
	LastAttemptAt time.Time         `db:"last_attempt_at"`
	LastError     string            `db:"last_error"`
}
