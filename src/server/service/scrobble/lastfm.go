package scrobble

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/local/cassonic/src/server/model"
)

// lastfmSign computes the Last.fm API signature for the given params map.
// Signature = MD5(sorted key+value pairs excluding "format" and "callback", then appended api_secret).
func lastfmSign(params map[string]string, apiSecret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "format" || k == "callback" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString(params[k])
	}
	sb.WriteString(apiSecret)

	sum := md5.Sum([]byte(sb.String()))
	return fmt.Sprintf("%x", sum)
}

// lastfmAuth obtains a Last.fm session key using mobile session authentication.
// passphrase = MD5(username + MD5(password)) as required by auth.getMobileSession.
func (s *Service) lastfmAuth(ctx context.Context, svc *model.ScrobbleService, password string) (string, error) {
	passHash := fmt.Sprintf("%x", md5.Sum([]byte(password)))
	authToken := fmt.Sprintf("%x", md5.Sum([]byte(svc.Username+passHash)))

	params := map[string]string{
		"method":    "auth.getMobileSession",
		"api_key":   svc.APIKey,
		"username":  svc.Username,
		"authToken": authToken,
		"format":    "json",
	}

	resp, err := s.lastfmCall(ctx, svc.BaseURL, svc.APIKey, svc.APISecretEnc, "", params)
	if err != nil {
		return "", err
	}

	session, ok := resp["session"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("lastfm auth: missing session in response")
	}
	key, ok := session["key"].(string)
	if !ok || key == "" {
		return "", fmt.Errorf("lastfm auth: missing session key")
	}
	return key, nil
}

// lastfmCall makes a signed Last.fm API call and returns the parsed JSON response.
func (s *Service) lastfmCall(ctx context.Context, baseURL, apiKey, apiSecret, sessionKey string, params map[string]string) (map[string]any, error) {
	params["api_key"] = apiKey
	if sessionKey != "" {
		params["sk"] = sessionKey
	}
	params["format"] = "json"

	params["api_sig"] = lastfmSign(params, apiSecret)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("lastfm call: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lastfm call: http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lastfm call: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lastfm call: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("lastfm call: parse response: %w", err)
	}

	if errObj, ok := result["error"]; ok {
		msg, _ := result["message"].(string)
		return nil, fmt.Errorf("lastfm call: API error %v: %s", errObj, msg)
	}

	return result, nil
}

// scrobbleLastFM sends a single track scrobble using the Last.fm API protocol.
func (s *Service) scrobbleLastFM(ctx context.Context, svc *model.ScrobbleService, track model.ScrobbleTrackData) error {
	baseURL := svc.BaseURL
	if baseURL == "" {
		baseURL = svc.ServiceType.BaseURL()
	}

	params := map[string]string{
		"method":         "track.scrobble",
		"track[0]":       track.Track,
		"artist[0]":      track.Artist,
		"album[0]":       track.Album,
		"timestamp[0]":   fmt.Sprintf("%d", track.Timestamp),
		"duration[0]":    fmt.Sprintf("%d", track.Duration),
	}
	if track.MBID != "" {
		params["mbid[0]"] = track.MBID
	}

	_, err := s.lastfmCall(ctx, baseURL, svc.APIKey, svc.APISecretEnc, svc.SessionKeyEnc, params)
	if err != nil {
		return fmt.Errorf("scrobble lastfm (%s): %w", svc.ServiceType, err)
	}
	return nil
}

// verifyLastFM checks that the stored session key is accepted by the Last.fm-compatible service.
func (s *Service) verifyLastFM(ctx context.Context, svc *model.ScrobbleService) error {
	if svc.SessionKeyEnc == "" {
		return fmt.Errorf("lastfm verify: no session key configured")
	}

	baseURL := svc.BaseURL
	if baseURL == "" {
		baseURL = svc.ServiceType.BaseURL()
	}

	params := map[string]string{
		"method": "user.getInfo",
		"user":   svc.Username,
	}

	_, err := s.lastfmCall(ctx, baseURL, svc.APIKey, svc.APISecretEnc, svc.SessionKeyEnc, params)
	if err != nil {
		return fmt.Errorf("lastfm verify (%s): %w", svc.ServiceType, err)
	}
	return nil
}
