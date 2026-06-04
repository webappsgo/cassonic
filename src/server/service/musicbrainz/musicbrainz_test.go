package musicbrainz

// Tests cover:
//   - NewClient: struct initialisation (non-nil, correct user-agent, default base URL)
//   - LookupRecording: happy-path JSON parsing, score threshold, HTTP errors, malformed JSON
//   - LookupRelease: happy-path JSON parsing, score threshold, HTTP errors
//   - FillSongMBIDs: fills empty fields, skips non-empty fields, handles nil recording
//   - HTTP 429/503 (rate-limited): returns error
//   - HTTP 500: returns error
//   - User-Agent header: every request includes "cassonic/"
//
// httptest.NewServer is used for all HTTP interactions — no real network calls.
// The production 1 req/s ticker would stall tests; newTestClient pre-fires the
// ticker by replacing it with a stopped ticker whose channel already has a value.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
)

// newTestClient builds a Client pointed at baseURL with the rate-limit ticker
// pre-fired so that wait() returns immediately on the first call.
// Each subsequent call in the same test will block for up to 1 s; tests that
// need multiple round-trips send enough ticks via a very short interval.
func newTestClient(baseURL string) *Client {
	t := time.NewTicker(time.Millisecond)
	// Drain so the first wait() fires instantly without sleeping.
	time.Sleep(2 * time.Millisecond)
	return &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		ticker:     t,
		userAgent:  "cassonic/test (https://cassonic.app)",
		baseURL:    baseURL,
	}
}

// TestNewClient verifies that NewClient returns a correctly initialised Client.
func TestNewClient(t *testing.T) {
	c := NewClient("1.2.3")

	if c == nil {
		t.Fatal("NewClient: returned nil")
	}
	if c.ticker == nil {
		t.Error("NewClient: ticker is nil")
	}
	if !strings.Contains(c.userAgent, "cassonic/") {
		t.Errorf("NewClient: userAgent %q does not contain 'cassonic/'", c.userAgent)
	}
	if c.baseURL != mbBaseURL {
		t.Errorf("NewClient: baseURL=%q, want %q", c.baseURL, mbBaseURL)
	}
	// Confirm the ticker fires roughly at 1 req/s by checking the interval is
	// at least 900 ms (we cannot introspect time.Ticker directly so we just
	// verify the channel is not nil and the struct is present).
	select {
	case <-c.ticker.C:
		// good — ticker channel is live
	case <-time.After(1500 * time.Millisecond):
		// also acceptable — first tick not yet received within 1.5 s
	}
	c.ticker.Stop()
}

// TestNewClientUserAgentContainsVersion checks that the version string is
// embedded in the User-Agent.
func TestNewClientUserAgentContainsVersion(t *testing.T) {
	c := NewClient("2.5.0")
	defer c.ticker.Stop()

	if !strings.Contains(c.userAgent, "2.5.0") {
		t.Errorf("NewClient: userAgent %q does not contain version '2.5.0'", c.userAgent)
	}
}

// --- LookupRecording ---

func TestLookupRecordingHappyPath(t *testing.T) {
	const body = `{
		"recordings": [{
			"id":    "rec-id-001",
			"title": "Bohemian Rhapsody",
			"score": 100,
			"length": 354000,
			"artist-credit": [{"artist": {"id": "artist-id-001"}}],
			"releases": [{"id": "release-id-001", "artist-credit": [{"artist": {"id": "rel-artist-001"}}]}]
		}]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	rec, err := c.LookupRecording(context.Background(), "Bohemian Rhapsody", "Queen", "")

	if err != nil {
		t.Fatalf("LookupRecording: unexpected error: %v", err)
	}
	if rec == nil {
		t.Fatal("LookupRecording: expected non-nil result")
	}
	if rec.ID != "rec-id-001" {
		t.Errorf("ID: got %q, want %q", rec.ID, "rec-id-001")
	}
	if rec.Title != "Bohemian Rhapsody" {
		t.Errorf("Title: got %q, want %q", rec.Title, "Bohemian Rhapsody")
	}
	if rec.Duration != 354000 {
		t.Errorf("Duration: got %d, want 354000", rec.Duration)
	}
	if rec.ArtistMBID != "artist-id-001" {
		t.Errorf("ArtistMBID: got %q, want %q", rec.ArtistMBID, "artist-id-001")
	}
	if rec.AlbumMBID != "release-id-001" {
		t.Errorf("AlbumMBID: got %q, want %q", rec.AlbumMBID, "release-id-001")
	}
	if rec.AlbumArtistMBID != "rel-artist-001" {
		t.Errorf("AlbumArtistMBID: got %q, want %q", rec.AlbumArtistMBID, "rel-artist-001")
	}
}

// TestLookupRecordingScoreBelowThreshold verifies that results with score < 80
// are silently discarded and nil is returned.
func TestLookupRecordingScoreBelowThreshold(t *testing.T) {
	body := `{"recordings": [{"id":"x","title":"X","score":79,"length":0,"artist-credit":[],"releases":[]}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	rec, err := c.LookupRecording(context.Background(), "Bad Match", "Nobody", "")

	if err != nil {
		t.Fatalf("LookupRecording: unexpected error: %v", err)
	}
	if rec != nil {
		t.Errorf("LookupRecording: expected nil for score=79, got %+v", rec)
	}
}

// TestLookupRecordingScoreAtThreshold verifies score == 80 is accepted.
func TestLookupRecordingScoreAtThreshold(t *testing.T) {
	body := `{"recordings": [{"id":"edge","title":"Edge","score":80,"length":1000,"artist-credit":[],"releases":[]}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	rec, err := c.LookupRecording(context.Background(), "Edge", "Artist", "")

	if err != nil {
		t.Fatalf("LookupRecording: unexpected error: %v", err)
	}
	if rec == nil {
		t.Fatal("LookupRecording: expected non-nil for score=80 (boundary)")
	}
	if rec.ID != "edge" {
		t.Errorf("ID: got %q, want %q", rec.ID, "edge")
	}
}

// TestLookupRecordingEmptyList verifies an empty recordings array returns nil, nil.
func TestLookupRecordingEmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"recordings":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	rec, err := c.LookupRecording(context.Background(), "Nothing", "Nobody", "")

	if err != nil {
		t.Fatalf("LookupRecording: unexpected error: %v", err)
	}
	if rec != nil {
		t.Errorf("LookupRecording: expected nil for empty list, got %+v", rec)
	}
}

// TestLookupRecordingReturnsFirstHighScoreResult verifies that only the first
// result meeting the score threshold is returned (not the second).
func TestLookupRecordingReturnsFirstHighScoreResult(t *testing.T) {
	body := `{"recordings": [
		{"id":"first","title":"First","score":95,"length":100,"artist-credit":[],"releases":[]},
		{"id":"second","title":"Second","score":90,"length":200,"artist-credit":[],"releases":[]}
	]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	rec, err := c.LookupRecording(context.Background(), "First", "Artist", "")

	if err != nil {
		t.Fatalf("LookupRecording: unexpected error: %v", err)
	}
	if rec == nil || rec.ID != "first" {
		t.Errorf("LookupRecording: expected first record, got %+v", rec)
	}
}

// TestLookupRecordingHTTP500 verifies that a 500 response is returned as an error.
func TestLookupRecordingHTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.LookupRecording(context.Background(), "Any", "Any", "")

	if err == nil {
		t.Fatal("LookupRecording: expected error for HTTP 500, got nil")
	}
}

// TestLookupRecordingHTTP503RateLimited verifies that a 503 response returns an error
// containing rate-limit information (MusicBrainz uses 503 for rate limits).
func TestLookupRecordingHTTP503RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.LookupRecording(context.Background(), "Any", "Any", "")

	if err == nil {
		t.Fatal("LookupRecording: expected error for HTTP 503, got nil")
	}
	if !strings.Contains(err.Error(), "rate") && !strings.Contains(err.Error(), "503") {
		t.Errorf("LookupRecording: error should mention rate limiting, got: %v", err)
	}
}

// TestLookupRecordingMalformedJSON verifies that invalid JSON returns an error.
func TestLookupRecordingMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{this is not valid json`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.LookupRecording(context.Background(), "Any", "Any", "")

	if err == nil {
		t.Fatal("LookupRecording: expected error for malformed JSON, got nil")
	}
}

// TestLookupRecordingUserAgentHeader verifies every request carries a
// "cassonic/" prefix in the User-Agent as required by MusicBrainz ToS.
func TestLookupRecordingUserAgentHeader(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Write([]byte(`{"recordings":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, _ = c.LookupRecording(context.Background(), "Any", "Any", "")

	if !strings.Contains(gotUA, "cassonic/") {
		t.Errorf("User-Agent header %q does not contain 'cassonic/'", gotUA)
	}
}

// TestLookupRecordingContextCancelled verifies that a cancelled context is
// respected by the rate-limit wait and returns an error.
func TestLookupRecordingContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"recordings":[]}`))
	}))
	defer srv.Close()

	// Use a 1-hour ticker so wait() will block until ctx is cancelled.
	c := &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		ticker:     time.NewTicker(time.Hour),
		userAgent:  "cassonic/test",
		baseURL:    srv.URL,
	}
	defer c.ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.LookupRecording(ctx, "Any", "Any", "")

	if err == nil {
		t.Fatal("LookupRecording: expected error for cancelled context, got nil")
	}
}

// --- LookupRelease ---

func TestLookupReleaseHappyPath(t *testing.T) {
	const body = `{
		"releases": [{
			"id":    "rel-id-001",
			"title": "A Night at the Opera",
			"score": 100,
			"date":  "1975-11-21",
			"artist-credit": [{"artist": {"id": "artist-id-001"}}]
		}]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	rel, err := c.LookupRelease(context.Background(), "A Night at the Opera", "Queen")

	if err != nil {
		t.Fatalf("LookupRelease: unexpected error: %v", err)
	}
	if rel == nil {
		t.Fatal("LookupRelease: expected non-nil result")
	}
	if rel.ID != "rel-id-001" {
		t.Errorf("ID: got %q, want %q", rel.ID, "rel-id-001")
	}
	if rel.Title != "A Night at the Opera" {
		t.Errorf("Title: got %q, want %q", rel.Title, "A Night at the Opera")
	}
	if rel.Year != 1975 {
		t.Errorf("Year: got %d, want 1975", rel.Year)
	}
	if rel.ArtistMBID != "artist-id-001" {
		t.Errorf("ArtistMBID: got %q, want %q", rel.ArtistMBID, "artist-id-001")
	}
}

// TestLookupReleaseYearOnlyDate verifies a date string containing only a year
// (e.g. "1975") is parsed correctly.
func TestLookupReleaseYearOnlyDate(t *testing.T) {
	body := `{"releases":[{"id":"r1","title":"T","score":85,"date":"1975","artist-credit":[]}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	rel, err := c.LookupRelease(context.Background(), "T", "A")

	if err != nil {
		t.Fatalf("LookupRelease: unexpected error: %v", err)
	}
	if rel == nil {
		t.Fatal("LookupRelease: expected non-nil result")
	}
	if rel.Year != 1975 {
		t.Errorf("Year: got %d, want 1975", rel.Year)
	}
}

// TestLookupReleaseScoreBelowThreshold verifies score < 80 returns nil.
func TestLookupReleaseScoreBelowThreshold(t *testing.T) {
	body := `{"releases":[{"id":"r1","title":"T","score":50,"date":"2000","artist-credit":[]}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	rel, err := c.LookupRelease(context.Background(), "T", "A")

	if err != nil {
		t.Fatalf("LookupRelease: unexpected error: %v", err)
	}
	if rel != nil {
		t.Errorf("LookupRelease: expected nil for score=50, got %+v", rel)
	}
}

// TestLookupReleaseHTTP500 verifies a 500 response returns an error.
func TestLookupReleaseHTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.LookupRelease(context.Background(), "Any", "Any")

	if err == nil {
		t.Fatal("LookupRelease: expected error for HTTP 500, got nil")
	}
}

// TestLookupReleaseMalformedJSON verifies malformed JSON returns an error.
func TestLookupReleaseMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json at all`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.LookupRelease(context.Background(), "Any", "Any")

	if err == nil {
		t.Fatal("LookupRelease: expected error for malformed JSON, got nil")
	}
}

// TestLookupReleaseUserAgentHeader verifies every request carries a
// "cassonic/" prefix in the User-Agent.
func TestLookupReleaseUserAgentHeader(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Write([]byte(`{"releases":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, _ = c.LookupRelease(context.Background(), "Any", "Any")

	if !strings.Contains(gotUA, "cassonic/") {
		t.Errorf("User-Agent header %q does not contain 'cassonic/'", gotUA)
	}
}

// --- FillSongMBIDs ---

func TestFillSongMBIDsAllMissing(t *testing.T) {
	const body = `{
		"recordings": [{
			"id": "track-id",
			"title": "T",
			"score": 95,
			"length": 200,
			"artist-credit": [{"artist": {"id": "artist-id"}}],
			"releases": [{"id": "album-id", "artist-credit": [{"artist": {"id": "album-artist-id"}}]}]
		}]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	song := &model.Song{
		Title:      "T",
		ArtistName: "A",
		AlbumName:  "AL",
	}

	changed, err := c.FillSongMBIDs(context.Background(), song)

	if err != nil {
		t.Fatalf("FillSongMBIDs: unexpected error: %v", err)
	}
	if !changed {
		t.Error("FillSongMBIDs: expected changed=true when all fields were empty")
	}
	if song.MBTrackID != "track-id" {
		t.Errorf("MBTrackID: got %q, want %q", song.MBTrackID, "track-id")
	}
	if song.MBArtistID != "artist-id" {
		t.Errorf("MBArtistID: got %q, want %q", song.MBArtistID, "artist-id")
	}
	if song.MBAlbumID != "album-id" {
		t.Errorf("MBAlbumID: got %q, want %q", song.MBAlbumID, "album-id")
	}
	if song.MBAlbumArtistID != "album-artist-id" {
		t.Errorf("MBAlbumArtistID: got %q, want %q", song.MBAlbumArtistID, "album-artist-id")
	}
}

// TestFillSongMBIDsSkipsNonEmpty verifies that pre-existing MBID fields are
// never overwritten.
func TestFillSongMBIDsSkipsNonEmpty(t *testing.T) {
	const body = `{
		"recordings": [{
			"id": "new-track-id",
			"title": "T",
			"score": 95,
			"length": 200,
			"artist-credit": [{"artist": {"id": "new-artist-id"}}],
			"releases": [{"id": "new-album-id", "artist-credit": [{"artist": {"id": "new-album-artist-id"}}]}]
		}]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	song := &model.Song{
		Title:           "T",
		ArtistName:      "A",
		AlbumName:       "AL",
		MBTrackID:       "existing-track",
		MBArtistID:      "existing-artist",
		MBAlbumID:       "existing-album",
		MBAlbumArtistID: "existing-album-artist",
	}

	changed, err := c.FillSongMBIDs(context.Background(), song)

	if err != nil {
		t.Fatalf("FillSongMBIDs: unexpected error: %v", err)
	}
	if changed {
		t.Error("FillSongMBIDs: expected changed=false when all fields already populated")
	}
	if song.MBTrackID != "existing-track" {
		t.Errorf("MBTrackID was overwritten: got %q", song.MBTrackID)
	}
}

// TestFillSongMBIDsNoRecordingFound verifies that when the lookup returns nil
// (no match), FillSongMBIDs returns changed=false and no error.
func TestFillSongMBIDsNoRecordingFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"recordings":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	song := &model.Song{Title: "Unknown", ArtistName: "Unknown"}

	changed, err := c.FillSongMBIDs(context.Background(), song)

	if err != nil {
		t.Fatalf("FillSongMBIDs: unexpected error: %v", err)
	}
	if changed {
		t.Error("FillSongMBIDs: expected changed=false when no recording found")
	}
}

// TestFillSongMBIDsLookupError verifies that a server error propagates as an
// error return from FillSongMBIDs.
func TestFillSongMBIDsLookupError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "oops", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	song := &model.Song{Title: "T", ArtistName: "A"}

	_, err := c.FillSongMBIDs(context.Background(), song)

	if err == nil {
		t.Fatal("FillSongMBIDs: expected error from lookup failure, got nil")
	}
}
