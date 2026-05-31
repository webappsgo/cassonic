package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/local/cassonic/src/server/model"
)

// PodcastStore manages podcast channels and their episodes.
type PodcastStore interface {
	// CreateChannel inserts a new podcast channel and returns the assigned ID.
	CreateChannel(ctx context.Context, ch *model.PodcastChannel) (int64, error)
	// GetChannel fetches a podcast channel by primary key.
	GetChannel(ctx context.Context, id int64) (*model.PodcastChannel, error)
	// ListChannels returns all podcast channels ordered by title.
	ListChannels(ctx context.Context) ([]*model.PodcastChannel, error)
	// UpdateChannel writes all mutable fields back to the database.
	UpdateChannel(ctx context.Context, ch *model.PodcastChannel) error
	// DeleteChannel permanently removes a channel and all its episodes.
	DeleteChannel(ctx context.Context, id int64) error

	// GetEpisode fetches a podcast episode by primary key.
	GetEpisode(ctx context.Context, id int64) (*model.PodcastEpisode, error)
	// ListEpisodesByChannel returns all episodes for a channel ordered by published_at DESC.
	ListEpisodesByChannel(ctx context.Context, channelID int64) ([]*model.PodcastEpisode, error)
	// UpsertEpisode inserts or updates an episode matched by channel_id+guid.
	UpsertEpisode(ctx context.Context, ep *model.PodcastEpisode) (int64, error)
	// UpdateEpisodeStatus updates the status and optional error for an episode.
	UpdateEpisodeStatus(ctx context.Context, id int64, status model.EpisodeStatus, lastErr string) error
	// DeleteEpisode permanently removes an episode.
	DeleteEpisode(ctx context.Context, id int64) error
}

// podcastSchema is applied to server.db alongside the main serverSchema.
const podcastSchema = `
CREATE TABLE IF NOT EXISTS podcast_channels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    image_url TEXT NOT NULL DEFAULT '',
    original_image_url TEXT NOT NULL DEFAULT '',
    author TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    link TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'new',
    episode_count INTEGER NOT NULL DEFAULT 0,
    last_checked_at DATETIME,
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS podcast_episodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id INTEGER NOT NULL REFERENCES podcast_channels(id) ON DELETE CASCADE,
    guid TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    audio_url TEXT NOT NULL DEFAULT '',
    download_path TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT '',
    file_size INTEGER NOT NULL DEFAULT 0,
    duration INTEGER NOT NULL DEFAULT 0,
    published_at DATETIME,
    status TEXT NOT NULL DEFAULT 'new',
    year INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(channel_id, guid)
);
CREATE INDEX IF NOT EXISTS idx_podcast_episodes_channel_id ON podcast_episodes(channel_id);
`

type sqlitePodcastStore struct {
	db *sql.DB
}

// NewPodcastStore creates a sqlitePodcastStore and ensures the schema is applied.
func NewPodcastStore(db *sql.DB) (PodcastStore, error) {
	if _, err := db.ExecContext(context.Background(), podcastSchema); err != nil {
		return nil, fmt.Errorf("podcast_store: apply schema: %w", err)
	}
	return &sqlitePodcastStore{db: db}, nil
}

func scanChannel(row interface {
	Scan(...any) error
}) (*model.PodcastChannel, error) {
	var ch model.PodcastChannel
	var lastChecked, createdAt, updatedAt sql.NullString
	err := row.Scan(
		&ch.ID,
		&ch.URL,
		&ch.Title,
		&ch.Description,
		&ch.ImageURL,
		&ch.OriginalImageURL,
		&ch.Author,
		&ch.Language,
		&ch.Category,
		&ch.Link,
		&ch.Status,
		&ch.EpisodeCount,
		&lastChecked,
		&ch.LastError,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lastChecked.Valid && lastChecked.String != "" {
		ch.LastCheckedAt, _ = time.Parse(time.RFC3339, lastChecked.String)
	}
	if createdAt.Valid {
		ch.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}
	if updatedAt.Valid {
		ch.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	}
	return &ch, nil
}

func scanEpisode(row interface {
	Scan(...any) error
}) (*model.PodcastEpisode, error) {
	var ep model.PodcastEpisode
	var publishedAt, createdAt, updatedAt sql.NullString
	err := row.Scan(
		&ep.ID,
		&ep.ChannelID,
		&ep.GUID,
		&ep.Title,
		&ep.Description,
		&ep.AudioURL,
		&ep.DownloadPath,
		&ep.ContentType,
		&ep.FileSize,
		&ep.Duration,
		&publishedAt,
		&ep.Status,
		&ep.Year,
		&ep.LastError,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if publishedAt.Valid && publishedAt.String != "" {
		ep.PublishedAt, _ = time.Parse(time.RFC3339, publishedAt.String)
	}
	if createdAt.Valid {
		ep.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}
	if updatedAt.Valid {
		ep.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	}
	return &ep, nil
}

const channelSelectCols = `id, url, title, description, image_url, original_image_url, author,
	language, category, link, status, episode_count, last_checked_at, last_error, created_at, updated_at`

const episodeSelectCols = `id, channel_id, guid, title, description, audio_url, download_path,
	content_type, file_size, duration, published_at, status, year, last_error, created_at, updated_at`

func (s *sqlitePodcastStore) CreateChannel(ctx context.Context, ch *model.PodcastChannel) (int64, error) {
	const q = `INSERT INTO podcast_channels
		(url, title, description, image_url, original_image_url, author, language, category, link, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, q,
		ch.URL, ch.Title, ch.Description, ch.ImageURL, ch.OriginalImageURL,
		ch.Author, ch.Language, ch.Category, ch.Link, ch.Status,
	)
	if err != nil {
		return 0, fmt.Errorf("podcast_store create channel: %w", err)
	}
	return res.LastInsertId()
}

func (s *sqlitePodcastStore) GetChannel(ctx context.Context, id int64) (*model.PodcastChannel, error) {
	q := `SELECT ` + channelSelectCols + ` FROM podcast_channels WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	ch, err := scanChannel(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("podcast_store get channel: %w", err)
	}
	return ch, nil
}

func (s *sqlitePodcastStore) ListChannels(ctx context.Context) ([]*model.PodcastChannel, error) {
	q := `SELECT ` + channelSelectCols + ` FROM podcast_channels ORDER BY title`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("podcast_store list channels: %w", err)
	}
	defer rows.Close()

	var channels []*model.PodcastChannel
	for rows.Next() {
		ch, err := scanChannel(rows)
		if err != nil {
			return nil, fmt.Errorf("podcast_store list channels scan: %w", err)
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

func (s *sqlitePodcastStore) UpdateChannel(ctx context.Context, ch *model.PodcastChannel) error {
	const q = `UPDATE podcast_channels SET
		title = ?, description = ?, image_url = ?, original_image_url = ?,
		author = ?, language = ?, category = ?, link = ?, status = ?,
		last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q,
		ch.Title, ch.Description, ch.ImageURL, ch.OriginalImageURL,
		ch.Author, ch.Language, ch.Category, ch.Link, ch.Status,
		ch.LastError, ch.ID,
	)
	if err != nil {
		return fmt.Errorf("podcast_store update channel: %w", err)
	}
	return nil
}

func (s *sqlitePodcastStore) DeleteChannel(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM podcast_channels WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("podcast_store delete channel: %w", err)
	}
	return nil
}

func (s *sqlitePodcastStore) GetEpisode(ctx context.Context, id int64) (*model.PodcastEpisode, error) {
	q := `SELECT ` + episodeSelectCols + ` FROM podcast_episodes WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	ep, err := scanEpisode(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("podcast_store get episode: %w", err)
	}
	return ep, nil
}

func (s *sqlitePodcastStore) ListEpisodesByChannel(ctx context.Context, channelID int64) ([]*model.PodcastEpisode, error) {
	q := `SELECT ` + episodeSelectCols + ` FROM podcast_episodes WHERE channel_id = ? ORDER BY published_at DESC`
	rows, err := s.db.QueryContext(ctx, q, channelID)
	if err != nil {
		return nil, fmt.Errorf("podcast_store list episodes: %w", err)
	}
	defer rows.Close()

	var episodes []*model.PodcastEpisode
	for rows.Next() {
		ep, err := scanEpisode(rows)
		if err != nil {
			return nil, fmt.Errorf("podcast_store list episodes scan: %w", err)
		}
		episodes = append(episodes, ep)
	}
	return episodes, rows.Err()
}

func (s *sqlitePodcastStore) UpsertEpisode(ctx context.Context, ep *model.PodcastEpisode) (int64, error) {
	const q = `INSERT INTO podcast_episodes
		(channel_id, guid, title, description, audio_url, content_type, duration, published_at, status, year)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(channel_id, guid) DO UPDATE SET
		title = excluded.title,
		description = excluded.description,
		audio_url = excluded.audio_url,
		content_type = excluded.content_type,
		duration = excluded.duration,
		published_at = excluded.published_at,
		year = excluded.year,
		updated_at = CURRENT_TIMESTAMP`

	var publishedAt any
	if !ep.PublishedAt.IsZero() {
		publishedAt = ep.PublishedAt.UTC().Format(time.RFC3339)
	}

	res, err := s.db.ExecContext(ctx, q,
		ep.ChannelID, ep.GUID, ep.Title, ep.Description, ep.AudioURL,
		ep.ContentType, ep.Duration, publishedAt, ep.Status, ep.Year,
	)
	if err != nil {
		return 0, fmt.Errorf("podcast_store upsert episode: %w", err)
	}
	return res.LastInsertId()
}

func (s *sqlitePodcastStore) UpdateEpisodeStatus(ctx context.Context, id int64, status model.EpisodeStatus, lastErr string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE podcast_episodes SET status = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, lastErr, id)
	if err != nil {
		return fmt.Errorf("podcast_store update episode status: %w", err)
	}
	return nil
}

func (s *sqlitePodcastStore) DeleteEpisode(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM podcast_episodes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("podcast_store delete episode: %w", err)
	}
	return nil
}
