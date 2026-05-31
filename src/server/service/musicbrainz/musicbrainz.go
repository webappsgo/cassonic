package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/local/cassonic/src/server/model"
)

const (
	mbBaseURL      = "https://musicbrainz.org/ws/2"
	mbMinScore     = 80
	mbRateInterval = time.Second
)

// Recording holds MusicBrainz recording (track) data.
type Recording struct {
	// ID is the MusicBrainz recording MBID.
	ID string
	Title string
	// ArtistMBID is the MBID of the primary artist credit.
	ArtistMBID string
	// AlbumMBID is the MBID of the first release associated with this recording.
	AlbumMBID string
	// AlbumArtistMBID is the MBID of the release artist credit.
	AlbumArtistMBID string
	// Duration is the recording length in milliseconds.
	Duration int
}

// Release holds MusicBrainz release data.
type Release struct {
	// ID is the release MBID.
	ID string
	Title string
	// ArtistMBID is the MBID of the primary artist credit.
	ArtistMBID string
	Year int
}

// Client queries the MusicBrainz API at 1 request per second as required by MusicBrainz ToS.
type Client struct {
	httpClient *http.Client
	ticker     *time.Ticker
	userAgent  string
}

// NewClient creates a MusicBrainz client identified by the given application version.
func NewClient(version string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		ticker:     time.NewTicker(mbRateInterval),
		userAgent:  fmt.Sprintf("cassonic/%s (https://cassonic.app)", version),
	}
}

// wait blocks until the rate-limit ticker fires.
func (c *Client) wait(ctx context.Context) error {
	select {
	case <-c.ticker.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// get performs a rate-limited GET request against the MusicBrainz API.
func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}

	params.Set("fmt", "json")
	endpoint := mbBaseURL + path + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: read body: %w", err)
	}

	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("musicbrainz: rate limited (503)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz: HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// LookupRecording searches for a recording by title, artist, and optional album.
// Returns the best-match Recording (score >= 80) or nil if no suitable match was found.
func (c *Client) LookupRecording(ctx context.Context, title, artist, album string) (*Recording, error) {
	query := fmt.Sprintf("recording:%s+artist:%s", url.QueryEscape(title), url.QueryEscape(artist))
	if album != "" {
		query += fmt.Sprintf("+release:%s", url.QueryEscape(album))
	}

	params := url.Values{
		"query": {query},
		"limit": {"5"},
	}

	body, err := c.get(ctx, "/recording", params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Recordings []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Score int    `json:"score"`
			// Length is in milliseconds.
			Length       int `json:"length"`
			ArtistCredit []struct {
				Artist struct {
					ID string `json:"id"`
				} `json:"artist"`
			} `json:"artist-credit"`
			Releases []struct {
				ID           string `json:"id"`
				ArtistCredit []struct {
					Artist struct {
						ID string `json:"id"`
					} `json:"artist"`
				} `json:"artist-credit"`
			} `json:"releases"`
		} `json:"recordings"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("musicbrainz: parse recording response: %w", err)
	}

	for _, rec := range result.Recordings {
		if rec.Score < mbMinScore {
			continue
		}

		r := &Recording{
			ID:       rec.ID,
			Title:    rec.Title,
			Duration: rec.Length,
		}

		if len(rec.ArtistCredit) > 0 {
			r.ArtistMBID = rec.ArtistCredit[0].Artist.ID
		}
		if len(rec.Releases) > 0 {
			r.AlbumMBID = rec.Releases[0].ID
			if len(rec.Releases[0].ArtistCredit) > 0 {
				r.AlbumArtistMBID = rec.Releases[0].ArtistCredit[0].Artist.ID
			}
		}

		return r, nil
	}

	return nil, nil
}

// LookupRelease searches for a release by album title and artist.
// Returns the best-match Release (score >= 80) or nil if no suitable match was found.
func (c *Client) LookupRelease(ctx context.Context, album, artist string) (*Release, error) {
	query := fmt.Sprintf("release:%s+artist:%s", url.QueryEscape(album), url.QueryEscape(artist))

	params := url.Values{
		"query": {query},
		"limit": {"5"},
	}

	body, err := c.get(ctx, "/release", params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Releases []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Score int    `json:"score"`
			Date  string `json:"date"`
			ArtistCredit []struct {
				Artist struct {
					ID string `json:"id"`
				} `json:"artist"`
			} `json:"artist-credit"`
		} `json:"releases"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("musicbrainz: parse release response: %w", err)
	}

	for _, rel := range result.Releases {
		if rel.Score < mbMinScore {
			continue
		}

		r := &Release{
			ID:    rel.ID,
			Title: rel.Title,
		}

		if len(rel.ArtistCredit) > 0 {
			r.ArtistMBID = rel.ArtistCredit[0].Artist.ID
		}

		if len(rel.Date) >= 4 {
			year := 0
			fmt.Sscanf(rel.Date[:4], "%d", &year)
			r.Year = year
		}

		return r, nil
	}

	return nil, nil
}

// FillSongMBIDs looks up MusicBrainz IDs for a song and fills any empty MBID fields.
// Non-empty fields are never overwritten regardless of user_edited.
// Returns changed=true if at least one field was updated.
func (c *Client) FillSongMBIDs(ctx context.Context, song *model.Song) (changed bool, err error) {
	if song.MBTrackID != "" && song.MBArtistID != "" && song.MBAlbumID != "" && song.MBAlbumArtistID != "" {
		return false, nil
	}

	rec, err := c.LookupRecording(ctx, song.Title, song.ArtistName, song.AlbumName)
	if err != nil {
		return false, fmt.Errorf("fillSongMBIDs: lookup recording: %w", err)
	}
	if rec == nil {
		return false, nil
	}

	if song.MBTrackID == "" && rec.ID != "" {
		song.MBTrackID = rec.ID
		changed = true
	}
	if song.MBArtistID == "" && rec.ArtistMBID != "" {
		song.MBArtistID = rec.ArtistMBID
		changed = true
	}
	if song.MBAlbumID == "" && rec.AlbumMBID != "" {
		song.MBAlbumID = rec.AlbumMBID
		changed = true
	}
	if song.MBAlbumArtistID == "" && rec.AlbumArtistMBID != "" {
		song.MBAlbumArtistID = rec.AlbumArtistMBID
		changed = true
	}

	return changed, nil
}
