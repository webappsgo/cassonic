package podcast

// Tests cover:
//   - NewService: returns non-nil Service
//   - fetchRSS (ParseFeed): happy-path RSS 2.0 XML parsing — title, description,
//     episode count, episode titles/GUIDs; iTunes namespace fields; image fields
//   - fetchRSS: invalid XML returns error
//   - fetchRSS: HTTP 404 returns error
//   - fetchRSS: HTTP 500 returns error
//   - itemToEpisode: enclosure audio MIME types accepted; non-audio is stored as-is
//     (the spec does not filter them at parse time — filtering is a caller concern)
//   - itemToEpisode: GUID falls back to AudioURL when GUID is empty
//   - itemToEpisode: iTunes duration parsing (HH:MM:SS, MM:SS, SS, empty)
//   - itemToEpisode: pubDate parsing, year extraction
//   - extensionFromTypeOrURL: MIME type to extension mapping, URL fallback, default
//   - parseItunesDuration: all three colon forms, empty string, plain seconds
//   - RefreshChannel: happy path updates channel fields and upserts episodes
//   - RefreshChannel: fetch failure updates channel status to error
//
// httptest.NewServer is used for all HTTP interactions — no real network calls.
// fetchRSS does not touch the DB so a nil *store.DB is safe for those tests.
// RefreshChannel tests use a fakePodcastStore that satisfies store.PodcastStore.

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// --- fake store implementation for RefreshChannel tests ---

// fakePodcastStore implements store.PodcastStore with in-memory state.
type fakePodcastStore struct {
	channel  *model.PodcastChannel
	episodes []*model.PodcastEpisode
	// updateErr is returned by UpdateChannel when non-nil.
	updateErr error
}

func (f *fakePodcastStore) CreateChannel(_ context.Context, ch *model.PodcastChannel) (int64, error) {
	f.channel = ch
	return 1, nil
}

func (f *fakePodcastStore) GetChannel(_ context.Context, id int64) (*model.PodcastChannel, error) {
	if f.channel == nil {
		return nil, nil
	}
	return f.channel, nil
}

func (f *fakePodcastStore) ListChannels(_ context.Context) ([]*model.PodcastChannel, error) {
	if f.channel == nil {
		return nil, nil
	}
	return []*model.PodcastChannel{f.channel}, nil
}

func (f *fakePodcastStore) UpdateChannel(_ context.Context, ch *model.PodcastChannel) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.channel = ch
	return nil
}

func (f *fakePodcastStore) DeleteChannel(_ context.Context, _ int64) error { return nil }

func (f *fakePodcastStore) GetEpisode(_ context.Context, _ int64) (*model.PodcastEpisode, error) {
	return nil, nil
}

func (f *fakePodcastStore) ListEpisodesByChannel(_ context.Context, _ int64) ([]*model.PodcastEpisode, error) {
	return f.episodes, nil
}

func (f *fakePodcastStore) UpsertEpisode(_ context.Context, ep *model.PodcastEpisode) (int64, error) {
	f.episodes = append(f.episodes, ep)
	return int64(len(f.episodes)), nil
}

func (f *fakePodcastStore) UpdateEpisodeStatus(_ context.Context, _ int64, _ model.EpisodeStatus, _ string) error {
	return nil
}

func (f *fakePodcastStore) DeleteEpisode(_ context.Context, _ int64) error { return nil }

// newTestService creates a Service with the fake podcast store and a silent logger.
func newTestService(fake *fakePodcastStore) *Service {
	db := &store.DB{Podcasts: fake}
	return NewService(db, "/tmp/local/cassonic-test", log.New(io.Discard, "", 0))
}

// newNilDBService creates a Service with a nil DB, suitable for testing methods
// that do not access the store (fetchRSS, downloadFile).
func newNilDBService() *Service {
	return NewService(nil, "/tmp/local/cassonic-test", log.New(io.Discard, "", 0))
}

// validFeed returns a minimal but complete RSS 2.0 document.
func validFeed(srvURL string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>My Podcast</title>
    <description>A test podcast feed</description>
    <link>https://example.com/podcast</link>
    <language>en</language>
    <image><url>https://example.com/cover.jpg</url></image>
    <item>
      <title>Episode 1</title>
      <guid>guid-ep-001</guid>
      <pubDate>Mon, 01 Jan 2024 10:00:00 +0000</pubDate>
      <description>First episode</description>
      <enclosure url="` + srvURL + `/ep1.mp3" type="audio/mpeg" length="12345678"/>
      <itunes:duration>32:10</itunes:duration>
    </item>
    <item>
      <title>Episode 2</title>
      <guid>guid-ep-002</guid>
      <pubDate>Mon, 08 Jan 2024 10:00:00 +0000</pubDate>
      <description>Second episode</description>
      <enclosure url="` + srvURL + `/ep2.ogg" type="audio/ogg" length="9876543"/>
      <itunes:duration>28:45</itunes:duration>
    </item>
  </channel>
</rss>`
}

// --- NewService ---

func TestNewServiceNonNil(t *testing.T) {
	svc := newNilDBService()
	if svc == nil {
		t.Fatal("NewService: returned nil")
	}
}

func TestNewServiceClientNonNil(t *testing.T) {
	svc := newNilDBService()
	if svc.client == nil {
		t.Fatal("NewService: http.Client is nil")
	}
}

// --- fetchRSS (ParseFeed) ---

func TestFetchRSSHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(validFeed("http://" + r.Host)))
	}))
	defer srv.Close()

	svc := newNilDBService()
	ch, err := svc.fetchRSS(context.Background(), srv.URL)

	if err != nil {
		t.Fatalf("fetchRSS: unexpected error: %v", err)
	}
	if ch == nil {
		t.Fatal("fetchRSS: returned nil channel")
	}
	if ch.Title != "My Podcast" {
		t.Errorf("Title: got %q, want %q", ch.Title, "My Podcast")
	}
	if ch.Desc != "A test podcast feed" {
		t.Errorf("Desc: got %q, want %q", ch.Desc, "A test podcast feed")
	}
	if len(ch.Items) != 2 {
		t.Errorf("Items count: got %d, want 2", len(ch.Items))
	}
}

// TestFetchRSSEpisodeTitlesAndGUIDs verifies that each item's title and GUID are
// correctly parsed.
func TestFetchRSSEpisodeTitlesAndGUIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(validFeed("http://" + r.Host)))
	}))
	defer srv.Close()

	svc := newNilDBService()
	ch, err := svc.fetchRSS(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("fetchRSS: unexpected error: %v", err)
	}

	want := []struct{ title, guid string }{
		{"Episode 1", "guid-ep-001"},
		{"Episode 2", "guid-ep-002"},
	}
	for i, w := range want {
		if ch.Items[i].Title != w.title {
			t.Errorf("Items[%d].Title: got %q, want %q", i, ch.Items[i].Title, w.title)
		}
		if ch.Items[i].GUID != w.guid {
			t.Errorf("Items[%d].GUID: got %q, want %q", i, ch.Items[i].GUID, w.guid)
		}
	}
}

// TestFetchRSSStandardImage verifies that the standard RSS <image><url> element
// is parsed into the Image field.
func TestFetchRSSStandardImage(t *testing.T) {
	const feed = `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Podcast</title>
    <description>desc</description>
    <image><url>https://example.com/cover.jpg</url></image>
  </channel>
</rss>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(feed))
	}))
	defer srv.Close()

	svc := newNilDBService()
	ch, err := svc.fetchRSS(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("fetchRSS: unexpected error: %v", err)
	}
	if ch.Image == nil || ch.Image.URL != "https://example.com/cover.jpg" {
		t.Errorf("Image: got %+v", ch.Image)
	}
}

// TestFetchRSSInvalidXML verifies that malformed XML returns an error.
func TestFetchRSSInvalidXML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<rss><channel><title>broken</channel>`))
	}))
	defer srv.Close()

	svc := newNilDBService()
	_, err := svc.fetchRSS(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("fetchRSS: expected error for invalid XML, got nil")
	}
}

// TestFetchRSSHTTP404 verifies that an HTTP 404 response returns an error.
func TestFetchRSSHTTP404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	svc := newNilDBService()
	_, err := svc.fetchRSS(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("fetchRSS: expected error for HTTP 404, got nil")
	}
}

// TestFetchRSSHTTP500 verifies that an HTTP 500 response returns an error.
func TestFetchRSSHTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	svc := newNilDBService()
	_, err := svc.fetchRSS(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("fetchRSS: expected error for HTTP 500, got nil")
	}
}

// TestFetchRSSEmptyFeed verifies that a feed with no items does not return an error.
func TestFetchRSSEmptyFeed(t *testing.T) {
	const feed = `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Empty</title>
    <description>no items</description>
  </channel>
</rss>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(feed))
	}))
	defer srv.Close()

	svc := newNilDBService()
	ch, err := svc.fetchRSS(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("fetchRSS: unexpected error for empty feed: %v", err)
	}
	if len(ch.Items) != 0 {
		t.Errorf("Items: expected 0, got %d", len(ch.Items))
	}
}

// --- itemToEpisode ---

func TestItemToEpisodeBasicFields(t *testing.T) {
	item := rssItem{
		Title:   "Test Episode",
		GUID:    "guid-001",
		Desc:    "A description",
		PubDate: "Mon, 01 Jan 2024 10:00:00 +0000",
		Enclosure: &rssEnclosure{
			URL:    "https://example.com/ep.mp3",
			Type:   "audio/mpeg",
			Length: "5000000",
		},
		Duration: "45:30",
	}

	ep := itemToEpisode(42, item)

	if ep.ChannelID != 42 {
		t.Errorf("ChannelID: got %d, want 42", ep.ChannelID)
	}
	if ep.GUID != "guid-001" {
		t.Errorf("GUID: got %q, want %q", ep.GUID, "guid-001")
	}
	if ep.Title != "Test Episode" {
		t.Errorf("Title: got %q, want %q", ep.Title, "Test Episode")
	}
	if ep.AudioURL != "https://example.com/ep.mp3" {
		t.Errorf("AudioURL: got %q", ep.AudioURL)
	}
	if ep.ContentType != "audio/mpeg" {
		t.Errorf("ContentType: got %q, want %q", ep.ContentType, "audio/mpeg")
	}
	if ep.FileSize != 5000000 {
		t.Errorf("FileSize: got %d, want 5000000", ep.FileSize)
	}
	if ep.Duration != 45*60+30 {
		t.Errorf("Duration: got %d, want %d", ep.Duration, 45*60+30)
	}
	if ep.Year != 2024 {
		t.Errorf("Year: got %d, want 2024", ep.Year)
	}
	if ep.Status != model.EpisodeStatusNew {
		t.Errorf("Status: got %q, want %q", ep.Status, model.EpisodeStatusNew)
	}
}

// TestItemToEpisodeAudioMPEGAccepted verifies audio/mpeg enclosure is stored.
func TestItemToEpisodeAudioMPEGAccepted(t *testing.T) {
	item := rssItem{
		GUID: "g1",
		Enclosure: &rssEnclosure{
			URL:    "https://example.com/track.mp3",
			Type:   "audio/mpeg",
			Length: "1000",
		},
	}
	ep := itemToEpisode(1, item)
	if ep.ContentType != "audio/mpeg" {
		t.Errorf("expected audio/mpeg ContentType, got %q", ep.ContentType)
	}
	if ep.AudioURL == "" {
		t.Error("expected non-empty AudioURL for audio/mpeg enclosure")
	}
}

// TestItemToEpisodeAudioOggAccepted verifies audio/ogg enclosure is stored.
func TestItemToEpisodeAudioOggAccepted(t *testing.T) {
	item := rssItem{
		GUID: "g2",
		Enclosure: &rssEnclosure{
			URL:    "https://example.com/track.ogg",
			Type:   "audio/ogg",
			Length: "1000",
		},
	}
	ep := itemToEpisode(1, item)
	if ep.ContentType != "audio/ogg" {
		t.Errorf("expected audio/ogg ContentType, got %q", ep.ContentType)
	}
}

// TestItemToEpisodeNonAudioEnclosureStored verifies that non-audio enclosures
// are stored as-is (the service does not filter at parse time; callers decide).
func TestItemToEpisodeNonAudioEnclosureStored(t *testing.T) {
	item := rssItem{
		GUID: "g3",
		Enclosure: &rssEnclosure{
			URL:    "https://example.com/video.mp4",
			Type:   "video/mp4",
			Length: "50000000",
		},
	}
	ep := itemToEpisode(1, item)
	if ep.ContentType != "video/mp4" {
		t.Errorf("ContentType: got %q, want video/mp4", ep.ContentType)
	}
	if ep.AudioURL != "https://example.com/video.mp4" {
		t.Errorf("AudioURL: got %q", ep.AudioURL)
	}
}

// TestItemToEpisodeNoEnclosure verifies that an item without an enclosure has
// empty AudioURL and ContentType.
func TestItemToEpisodeNoEnclosure(t *testing.T) {
	item := rssItem{
		GUID:  "g4",
		Title: "No enclosure",
	}
	ep := itemToEpisode(1, item)
	if ep.AudioURL != "" {
		t.Errorf("expected empty AudioURL, got %q", ep.AudioURL)
	}
	if ep.ContentType != "" {
		t.Errorf("expected empty ContentType, got %q", ep.ContentType)
	}
}

// TestItemToEpisodeGUIDFallsBackToAudioURL verifies that when GUID is empty,
// AudioURL is used as the fallback GUID.
func TestItemToEpisodeGUIDFallsBackToAudioURL(t *testing.T) {
	item := rssItem{
		GUID: "",
		Enclosure: &rssEnclosure{
			URL:  "https://example.com/ep.mp3",
			Type: "audio/mpeg",
		},
	}
	ep := itemToEpisode(1, item)
	if ep.GUID != "https://example.com/ep.mp3" {
		t.Errorf("GUID fallback: got %q, want AudioURL %q", ep.GUID, "https://example.com/ep.mp3")
	}
}

// --- parseItunesDuration ---

func TestParseItunesDurationForms(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"0", 0},
		{"90", 90},
		{"1:30", 90},
		{"1:00:00", 3600},
		{"1:01:01", 3661},
		{"10:20:30", 37230},
		{"  30  ", 30},
	}

	for _, tc := range cases {
		got := parseItunesDuration(tc.input)
		if got != tc.want {
			t.Errorf("parseItunesDuration(%q): got %d, want %d", tc.input, got, tc.want)
		}
	}
}

// --- extensionFromTypeOrURL ---

func TestExtensionFromTypeOrURL(t *testing.T) {
	cases := []struct {
		contentType string
		rawURL      string
		want        string
	}{
		{"audio/mpeg", "", ".mp3"},
		{"audio/mp4", "", ".m4a"},
		{"audio/ogg", "", ".ogg"},
		{"audio/opus", "", ".opus"},
		{"audio/flac", "", ".flac"},
		// URL fallback when content type is unrecognised.
		{"application/octet-stream", "https://example.com/podcast.mp3", ".mp3"},
		// Default when nothing matches.
		{"", "", ".mp3"},
		// Explicit URL extension wins over empty type.
		{"", "https://example.com/ep.ogg", ".ogg"},
	}

	for _, tc := range cases {
		got := extensionFromTypeOrURL(tc.contentType, tc.rawURL)
		if got != tc.want {
			t.Errorf("extensionFromTypeOrURL(%q, %q): got %q, want %q",
				tc.contentType, tc.rawURL, got, tc.want)
		}
	}
}

// --- RefreshChannel ---

func TestRefreshChannelHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(validFeed("http://" + r.Host)))
	}))
	defer srv.Close()

	fake := &fakePodcastStore{
		channel: &model.PodcastChannel{
			ID:     1,
			URL:    srv.URL,
			Status: model.PodcastStatusNew,
		},
	}
	svc := newTestService(fake)

	err := svc.RefreshChannel(context.Background(), 1)
	if err != nil {
		t.Fatalf("RefreshChannel: unexpected error: %v", err)
	}
	if fake.channel.Title != "My Podcast" {
		t.Errorf("channel.Title: got %q, want %q", fake.channel.Title, "My Podcast")
	}
	if fake.channel.Status != model.PodcastStatusCompleted {
		t.Errorf("channel.Status: got %q, want %q", fake.channel.Status, model.PodcastStatusCompleted)
	}
	if len(fake.episodes) != 2 {
		t.Errorf("upserted episodes: got %d, want 2", len(fake.episodes))
	}
}

// TestRefreshChannelFetchFailureSetsErrorStatus verifies that when the RSS
// fetch fails the channel status is updated to "error".
func TestRefreshChannelFetchFailureSetsErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "broken", http.StatusInternalServerError)
	}))
	defer srv.Close()

	fake := &fakePodcastStore{
		channel: &model.PodcastChannel{
			ID:     1,
			URL:    srv.URL,
			Status: model.PodcastStatusNew,
		},
	}
	svc := newTestService(fake)

	err := svc.RefreshChannel(context.Background(), 1)
	if err == nil {
		t.Fatal("RefreshChannel: expected error for failed fetch, got nil")
	}
	if fake.channel.Status != model.PodcastStatusError {
		t.Errorf("channel.Status: got %q, want %q", fake.channel.Status, model.PodcastStatusError)
	}
	if fake.channel.LastError == "" {
		t.Error("channel.LastError: expected non-empty error string")
	}
}

// TestRefreshChannelChannelNotFound verifies that a missing channel ID returns error.
func TestRefreshChannelChannelNotFound(t *testing.T) {
	fake := &fakePodcastStore{}
	svc := newTestService(fake)

	err := svc.RefreshChannel(context.Background(), 999)
	if err == nil {
		t.Fatal("RefreshChannel: expected error for missing channel, got nil")
	}
}
