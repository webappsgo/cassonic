package scrobble

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/local/cassonic/src/server/model"
)

// listenBrainzAdditionalInfo carries the extra fields in a ListenBrainz listen payload.
type listenBrainzAdditionalInfo struct {
	ListeningFrom  string `json:"listening_from"`
	DurationMS     int    `json:"duration_ms,omitempty"`
	TrackNumber    int    `json:"tracknumber,omitempty"`
	RecordingMBID  string `json:"recording_mbid,omitempty"`
}

// listenBrainzTrackMetadata is the track_metadata block in a ListenBrainz listen.
type listenBrainzTrackMetadata struct {
	ArtistName     string                     `json:"artist_name"`
	TrackName      string                     `json:"track_name"`
	ReleaseName    string                     `json:"release_name"`
	AdditionalInfo listenBrainzAdditionalInfo `json:"additional_info"`
}

// listenBrainzPayloadEntry is one listen in the payload array.
type listenBrainzPayloadEntry struct {
	ListenedAt    int64                     `json:"listened_at"`
	TrackMetadata listenBrainzTrackMetadata `json:"track_metadata"`
}

// listenBrainzSubmit is the top-level body for /1/submit-listens.
type listenBrainzSubmit struct {
	ListenType string                     `json:"listen_type"`
	Payload    []listenBrainzPayloadEntry `json:"payload"`
}

// scrobbleListenBrainz sends a single listen via the ListenBrainz API protocol.
func (s *Service) scrobbleListenBrainz(ctx context.Context, svc *model.ScrobbleService, track model.ScrobbleTrackData) error {
	baseURL := svc.BaseURL
	if baseURL == "" {
		baseURL = svc.ServiceType.BaseURL()
	}

	info := listenBrainzAdditionalInfo{
		ListeningFrom: "cassonic",
		DurationMS:    track.Duration * 1000,
	}
	if track.TrackNumber > 0 {
		info.TrackNumber = track.TrackNumber
	}
	if track.MBID != "" {
		info.RecordingMBID = track.MBID
	}

	body := listenBrainzSubmit{
		ListenType: "single",
		Payload: []listenBrainzPayloadEntry{
			{
				ListenedAt: track.Timestamp,
				TrackMetadata: listenBrainzTrackMetadata{
					ArtistName:     track.Artist,
					TrackName:      track.Track,
					ReleaseName:    track.Album,
					AdditionalInfo: info,
				},
			},
		},
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("listenbrainz scrobble: encode body: %w", err)
	}

	endpoint := baseURL + "/1/submit-listens"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("listenbrainz scrobble: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+svc.TokenEnc)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("listenbrainz scrobble: http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("listenbrainz scrobble: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("listenbrainz scrobble: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// verifyListenBrainz validates that the configured token is accepted by the ListenBrainz-compatible service.
// GET {baseURL}/1/validate-token expects {"code": 200, "message": "Token valid.", "user_name": "..."}.
func (s *Service) verifyListenBrainz(ctx context.Context, svc *model.ScrobbleService) error {
	if svc.TokenEnc == "" {
		return fmt.Errorf("listenbrainz verify: no token configured")
	}

	baseURL := svc.BaseURL
	if baseURL == "" {
		baseURL = svc.ServiceType.BaseURL()
	}

	endpoint := baseURL + "/1/validate-token"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("listenbrainz verify: build request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+svc.TokenEnc)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("listenbrainz verify: http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("listenbrainz verify: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("listenbrainz verify: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Code     int    `json:"code"`
		Message  string `json:"message"`
		UserName string `json:"user_name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("listenbrainz verify: parse response: %w", err)
	}

	if result.Code != 200 {
		return fmt.Errorf("listenbrainz verify: token rejected (code %d): %s", result.Code, result.Message)
	}

	return nil
}
