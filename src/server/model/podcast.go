package model

import "time"

// PodcastStatus represents the state of a podcast channel
type PodcastStatus string

const (
	PodcastStatusNew         PodcastStatus = "new"
	PodcastStatusScanning    PodcastStatus = "scanning"
	PodcastStatusCompleted   PodcastStatus = "completed"
	PodcastStatusError       PodcastStatus = "error"
	PodcastStatusDownloading PodcastStatus = "downloading"
)

// EpisodeStatus represents the download/availability state of a podcast episode
type EpisodeStatus string

const (
	EpisodeStatusNew         EpisodeStatus = "new"
	EpisodeStatusDownloading EpisodeStatus = "downloading"
	EpisodeStatusCompleted   EpisodeStatus = "completed"
	EpisodeStatusError       EpisodeStatus = "error"
	EpisodeStatusSkipped     EpisodeStatus = "skipped"
	EpisodeStatusDeleted     EpisodeStatus = "deleted"
)

// PodcastChannel represents a subscribed podcast RSS feed
type PodcastChannel struct {
	ID          int64  `db:"id"`
	URL         string `db:"url"`
	Title       string `db:"title"`
	Description string `db:"description"`
	ImageURL    string `db:"image_url"`
	// OriginalImageURL is the raw URL from the feed before any local caching
	OriginalImageURL string        `db:"original_image_url"`
	Author           string        `db:"author"`
	Language         string        `db:"language"`
	Category         string        `db:"category"`
	Link             string        `db:"link"`
	Status           PodcastStatus `db:"status"`
	EpisodeCount     int           `db:"episode_count"`
	LastCheckedAt    time.Time     `db:"last_checked_at"`
	LastError        string        `db:"last_error"`
	CreatedAt        time.Time     `db:"created_at"`
	UpdatedAt        time.Time     `db:"updated_at"`
}

// PodcastEpisode represents a single episode within a podcast channel
type PodcastEpisode struct {
	ID        int64  `db:"id"`
	ChannelID int64  `db:"channel_id"`
	// GUID is the globally unique identifier from the RSS feed item
	GUID        string `db:"guid"`
	Title       string `db:"title"`
	Description string `db:"description"`
	AudioURL    string `db:"audio_url"`
	// DownloadPath is the local filesystem path when the episode has been downloaded
	DownloadPath string        `db:"download_path"`
	ContentType  string        `db:"content_type"`
	FileSize     int64         `db:"file_size"`
	// Duration is the episode length in seconds
	Duration    int           `db:"duration"`
	PublishedAt time.Time     `db:"published_at"`
	Status      EpisodeStatus `db:"status"`
	// Year is derived from PublishedAt and stored for efficient querying
	Year      int       `db:"year"`
	LastError string    `db:"last_error"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
