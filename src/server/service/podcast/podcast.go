package podcast

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// rssFeed is the top-level RSS wrapper.
type rssChannel struct {
	XMLName     xml.Name     `xml:"channel"`
	Title       string       `xml:"title"`
	Desc        string       `xml:"description"`
	Link        string       `xml:"link"`
	Image       *rssImage    `xml:"image"`
	ITunesImage *itunesImage `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd image"`
	Author      string       `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd author"`
	Lang        string       `xml:"language"`
	Items       []rssItem    `xml:"item"`
}

type rssItem struct {
	Title     string        `xml:"title"`
	GUID      string        `xml:"guid"`
	PubDate   string        `xml:"pubDate"`
	Desc      string        `xml:"description"`
	Enclosure *rssEnclosure `xml:"enclosure"`
	Duration  string        `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd duration"`
}

type rssEnclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length string `xml:"length,attr"`
}

type rssImage struct {
	URL string `xml:"url"`
}

type itunesImage struct {
	URL string `xml:"href,attr"`
}

// rssWrapper wraps the outer <rss> element so we can decode nested <channel>.
type rssWrapper struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

// Service manages podcast subscriptions and episode downloads.
type Service struct {
	db      *store.DB
	dataDir string
	client  *http.Client
	logger  *log.Logger
}

// NewService creates a podcast service.
// dataDir is the base data directory; podcast files are stored in {dataDir}/podcasts.
func NewService(db *store.DB, dataDir string, logger *log.Logger) *Service {
	return &Service{
		db:      db,
		dataDir: dataDir,
		logger:  logger,
		client:  &http.Client{Timeout: 5 * time.Minute},
	}
}

// RefreshAll fetches RSS feeds for all enabled channels and upserts their episodes.
func (s *Service) RefreshAll(ctx context.Context) error {
	channels, err := s.db.Podcasts.ListChannels(ctx)
	if err != nil {
		return fmt.Errorf("podcast refresh all: list channels: %w", err)
	}
	for _, ch := range channels {
		if err := s.RefreshChannel(ctx, ch.ID); err != nil {
			s.logger.Printf("podcast: refresh channel %d (%s): %v", ch.ID, ch.Title, err)
		}
	}
	return nil
}

// RefreshChannel fetches the RSS feed for a single channel and upserts its episodes.
func (s *Service) RefreshChannel(ctx context.Context, channelID int64) error {
	ch, err := s.db.Podcasts.GetChannel(ctx, channelID)
	if err != nil {
		return fmt.Errorf("podcast refresh channel: get: %w", err)
	}
	if ch == nil {
		return fmt.Errorf("podcast refresh channel: channel %d not found", channelID)
	}

	feed, fetchErr := s.fetchRSS(ctx, ch.URL)
	if fetchErr != nil {
		ch.LastError = fetchErr.Error()
		ch.Status = model.PodcastStatusError
		_ = s.db.Podcasts.UpdateChannel(ctx, ch)
		return fetchErr
	}

	imageURL := ""
	if feed.Image != nil && feed.Image.URL != "" {
		imageURL = feed.Image.URL
	} else if feed.ITunesImage != nil && feed.ITunesImage.URL != "" {
		imageURL = feed.ITunesImage.URL
	}

	ch.Title = feed.Title
	ch.Description = feed.Desc
	ch.Author = feed.Author
	ch.Language = feed.Lang
	ch.Link = feed.Link
	if imageURL != "" {
		ch.OriginalImageURL = imageURL
		ch.ImageURL = imageURL
	}
	ch.LastCheckedAt = time.Now().UTC()
	ch.LastError = ""
	ch.Status = model.PodcastStatusCompleted

	if err := s.db.Podcasts.UpdateChannel(ctx, ch); err != nil {
		return fmt.Errorf("podcast refresh channel: update channel: %w", err)
	}

	for _, item := range feed.Items {
		ep := itemToEpisode(channelID, item)
		if _, err := s.db.Podcasts.UpsertEpisode(ctx, ep); err != nil {
			s.logger.Printf("podcast: upsert episode guid=%q: %v", item.GUID, err)
		}
	}

	return nil
}

// AddChannel creates a new channel from an RSS URL.
// Fetches the RSS feed to populate title, description, image, and creates the channel record.
func (s *Service) AddChannel(ctx context.Context, rssURL string) (*model.PodcastChannel, error) {
	if _, err := url.ParseRequestURI(rssURL); err != nil {
		return nil, fmt.Errorf("podcast add channel: invalid URL: %w", err)
	}

	ch := &model.PodcastChannel{
		URL:    rssURL,
		Status: model.PodcastStatusNew,
	}

	id, err := s.db.Podcasts.CreateChannel(ctx, ch)
	if err != nil {
		return nil, fmt.Errorf("podcast add channel: create: %w", err)
	}
	ch.ID = id

	if err := s.RefreshChannel(ctx, id); err != nil {
		s.logger.Printf("podcast: add channel initial refresh failed for %s: %v", rssURL, err)
	}

	updated, err := s.db.Podcasts.GetChannel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("podcast add channel: get after refresh: %w", err)
	}
	if updated != nil {
		return updated, nil
	}
	return ch, nil
}

// DownloadEpisode downloads the audio file for an episode.
// Saves to {dataDir}/podcasts/{channelID}/{episodeID}.{ext}
// Status transitions: new/error → downloading → completed or error.
func (s *Service) DownloadEpisode(ctx context.Context, episodeID int64) error {
	ep, err := s.db.Podcasts.GetEpisode(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("podcast download: get episode: %w", err)
	}
	if ep == nil {
		return fmt.Errorf("podcast download: episode %d not found", episodeID)
	}

	if ep.Status != model.EpisodeStatusNew && ep.Status != model.EpisodeStatusError {
		return fmt.Errorf("podcast download: episode %d status is %q, must be new or error", episodeID, ep.Status)
	}

	if err := s.db.Podcasts.UpdateEpisodeStatus(ctx, episodeID, model.EpisodeStatusDownloading, ""); err != nil {
		return fmt.Errorf("podcast download: set downloading status: %w", err)
	}

	downloadDir := filepath.Join(s.dataDir, "podcasts", strconv.FormatInt(ep.ChannelID, 10))
	if err := os.MkdirAll(downloadDir, 0750); err != nil {
		_ = s.db.Podcasts.UpdateEpisodeStatus(ctx, episodeID, model.EpisodeStatusError, err.Error())
		return fmt.Errorf("podcast download: create dir: %w", err)
	}

	ext := extensionFromTypeOrURL(ep.ContentType, ep.AudioURL)
	finalPath := filepath.Join(downloadDir, strconv.FormatInt(episodeID, 10)+ext)
	tmpPath := finalPath + ".tmp"

	fileSize, downloadErr := s.downloadFile(ctx, ep.AudioURL, tmpPath)
	if downloadErr != nil {
		os.Remove(tmpPath)
		_ = s.db.Podcasts.UpdateEpisodeStatus(ctx, episodeID, model.EpisodeStatusError, downloadErr.Error())
		return fmt.Errorf("podcast download: download: %w", downloadErr)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		_ = s.db.Podcasts.UpdateEpisodeStatus(ctx, episodeID, model.EpisodeStatusError, err.Error())
		return fmt.Errorf("podcast download: rename: %w", err)
	}

	ep.DownloadPath = finalPath
	ep.FileSize = fileSize
	ep.Status = model.EpisodeStatusCompleted
	ep.LastError = ""

	if _, err := s.db.Podcasts.UpsertEpisode(ctx, ep); err != nil {
		return fmt.Errorf("podcast download: update episode: %w", err)
	}

	return nil
}

// DeleteEpisodeFile deletes the downloaded audio file for an episode and clears its path.
func (s *Service) DeleteEpisodeFile(ctx context.Context, episodeID int64) error {
	ep, err := s.db.Podcasts.GetEpisode(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("podcast delete file: get episode: %w", err)
	}
	if ep == nil {
		return fmt.Errorf("podcast delete file: episode %d not found", episodeID)
	}

	if ep.DownloadPath != "" {
		if err := os.Remove(ep.DownloadPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("podcast delete file: remove: %w", err)
		}
	}

	ep.DownloadPath = ""
	ep.FileSize = 0
	ep.Status = model.EpisodeStatusDeleted

	if _, err := s.db.Podcasts.UpsertEpisode(ctx, ep); err != nil {
		return fmt.Errorf("podcast delete file: update episode: %w", err)
	}

	return nil
}

// fetchRSS downloads and parses an RSS feed from the given URL.
func (s *Service) fetchRSS(ctx context.Context, rssURL string) (*rssChannel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rssURL, nil)
	if err != nil {
		return nil, fmt.Errorf("podcast fetch rss: build request: %w", err)
	}
	req.Header.Set("User-Agent", "cassonic/1.0 (https://cassonic.app)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("podcast fetch rss: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("podcast fetch rss: HTTP %d", resp.StatusCode)
	}

	var wrapper rssWrapper
	dec := xml.NewDecoder(resp.Body)
	dec.Strict = false
	if err := dec.Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("podcast fetch rss: parse XML: %w", err)
	}

	return &wrapper.Channel, nil
}

// downloadFile downloads a URL to destPath and returns the number of bytes written.
func (s *Service) downloadFile(ctx context.Context, rawURL, destPath string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, fmt.Errorf("download file: build request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download file: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download file: HTTP %d", resp.StatusCode)
	}

	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return 0, fmt.Errorf("download file: create: %w", err)
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return n, fmt.Errorf("download file: copy: %w", err)
	}
	return n, nil
}

// itemToEpisode converts an rssItem to a model.PodcastEpisode.
func itemToEpisode(channelID int64, item rssItem) *model.PodcastEpisode {
	ep := &model.PodcastEpisode{
		ChannelID:   channelID,
		GUID:        item.GUID,
		Title:       item.Title,
		Description: item.Desc,
		Duration:    parseItunesDuration(item.Duration),
		Status:      model.EpisodeStatusNew,
	}

	if item.Enclosure != nil {
		ep.AudioURL = item.Enclosure.URL
		ep.ContentType = item.Enclosure.Type
		if sz, err := strconv.ParseInt(item.Enclosure.Length, 10, 64); err == nil {
			ep.FileSize = sz
		}
	}

	if item.PubDate != "" {
		for _, layout := range []string{
			time.RFC1123Z,
			time.RFC1123,
			"Mon, 02 Jan 2006 15:04:05 -0700",
			"Mon, 02 Jan 2006 15:04:05 MST",
			"2006-01-02T15:04:05Z07:00",
		} {
			if t, err := time.Parse(layout, item.PubDate); err == nil {
				ep.PublishedAt = t.UTC()
				ep.Year = t.Year()
				break
			}
		}
	}

	if ep.GUID == "" {
		ep.GUID = ep.AudioURL
	}

	return ep
}

// extensionFromTypeOrURL returns a file extension (including dot) derived from
// the MIME content type, falling back to the URL path extension.
func extensionFromTypeOrURL(contentType, rawURL string) string {
	switch {
	case strings.Contains(contentType, "mpeg"):
		return ".mp3"
	case strings.Contains(contentType, "mp4"):
		return ".m4a"
	case strings.Contains(contentType, "ogg"):
		return ".ogg"
	case strings.Contains(contentType, "opus"):
		return ".opus"
	case strings.Contains(contentType, "flac"):
		return ".flac"
	}

	if rawURL != "" {
		if u, err := url.Parse(rawURL); err == nil {
			ext := filepath.Ext(u.Path)
			if ext != "" {
				return ext
			}
		}
	}

	return ".mp3"
}

// parseItunesDuration parses an iTunes duration string in "HH:MM:SS", "MM:SS", or "SS" format.
// Returns the total duration in seconds.
func parseItunesDuration(s string) int {
	if s == "" {
		return 0
	}

	parts := strings.Split(strings.TrimSpace(s), ":")
	switch len(parts) {
	case 1:
		sec, _ := strconv.Atoi(parts[0])
		return sec
	case 2:
		min, _ := strconv.Atoi(parts[0])
		sec, _ := strconv.Atoi(parts[1])
		return min*60 + sec
	case 3:
		hr, _ := strconv.Atoi(parts[0])
		min, _ := strconv.Atoi(parts[1])
		sec, _ := strconv.Atoi(parts[2])
		return hr*3600 + min*60 + sec
	}

	return 0
}
