package podcast

// Tests in this file cover the remaining podcast.Service methods not exercised
// by podcast_test.go:
//   - AddChannel: invalid URL rejected; happy path creates + refreshes + returns
//     the updated channel; refresh failure still returns the created channel
//   - DownloadEpisode: episode not found, wrong status, happy path (writes a
//     temp file, renames it into place, updates episode fields), HTTP failure
//   - DeleteEpisodeFile: episode not found, happy path removes the file and
//     clears DownloadPath, missing file on disk is not an error, no DownloadPath set
//   - downloadFile: happy path byte count, HTTP failure, malformed URL
//   - RefreshAll: tolerates per-channel failures, propagates ListChannels errors

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// newTestServiceWithDataDir mirrors newTestService but allows the caller to
// supply a real temp directory so downloaded files can be verified on disk.
func newTestServiceWithDataDir(fake *fakePodcastStore, dataDir string) *Service {
	db := &store.DB{Podcasts: fake}
	return NewService(db, dataDir, log.New(io.Discard, "", 0))
}

func TestAddChannelInvalidURL(t *testing.T) {
	fake := &fakePodcastStore{}
	svc := newTestService(fake)

	_, err := svc.AddChannel(context.Background(), "not a url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestAddChannelSuccess(t *testing.T) {
	var feedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(validFeed(feedURL)))
	}))
	defer srv.Close()
	feedURL = srv.URL

	fake := &fakePodcastStore{}
	svc := newTestService(fake)

	ch, err := svc.AddChannel(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("AddChannel: unexpected error: %v", err)
	}
	if ch == nil {
		t.Fatal("AddChannel: expected non-nil channel")
	}
	if fake.channel == nil {
		t.Error("expected channel to be created in store")
	}
}

func TestAddChannelRefreshFailureStillReturnsChannel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	fake := &fakePodcastStore{}
	svc := newTestService(fake)

	ch, err := svc.AddChannel(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("AddChannel: unexpected error: %v", err)
	}
	if ch == nil {
		t.Fatal("AddChannel: expected non-nil channel even on refresh failure")
	}
}

func TestDownloadEpisodeNotFound(t *testing.T) {
	fake := &fakePodcastStore{}
	svc := newTestService(fake)

	err := svc.DownloadEpisode(context.Background(), 42)
	if err == nil {
		t.Fatal("expected error for missing episode")
	}
}

func TestDownloadEpisodeWrongStatus(t *testing.T) {
	fake := &fakePodcastStore{singleEpisode: &model.PodcastEpisode{
		ID:     1,
		Status: model.EpisodeStatusCompleted,
	}}
	svc := newTestService(fake)

	err := svc.DownloadEpisode(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error for episode not in new/error status")
	}
}

func TestDownloadEpisodeSuccess(t *testing.T) {
	const body = "fake audio bytes"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	fake := &fakePodcastStore{singleEpisode: &model.PodcastEpisode{
		ID:        7,
		ChannelID: 3,
		AudioURL:  srv.URL,
		Status:    model.EpisodeStatusNew,
	}}
	svc := newTestServiceWithDataDir(fake, dataDir)

	if err := svc.DownloadEpisode(context.Background(), 7); err != nil {
		t.Fatalf("DownloadEpisode: unexpected error: %v", err)
	}

	if fake.singleEpisode.Status != model.EpisodeStatusCompleted {
		t.Errorf("status: got %q, want %q", fake.singleEpisode.Status, model.EpisodeStatusCompleted)
	}
	if fake.singleEpisode.FileSize != int64(len(body)) {
		t.Errorf("file size: got %d, want %d", fake.singleEpisode.FileSize, len(body))
	}
	if _, err := os.Stat(fake.singleEpisode.DownloadPath); err != nil {
		t.Errorf("downloaded file not found on disk: %v", err)
	}
}

func TestDownloadEpisodeHTTPFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	fake := &fakePodcastStore{singleEpisode: &model.PodcastEpisode{
		ID:        8,
		ChannelID: 3,
		AudioURL:  srv.URL,
		Status:    model.EpisodeStatusNew,
	}}
	svc := newTestServiceWithDataDir(fake, dataDir)

	err := svc.DownloadEpisode(context.Background(), 8)
	if err == nil {
		t.Fatal("expected error for HTTP failure")
	}
	if fake.singleEpisode.Status != model.EpisodeStatusError {
		t.Errorf("status: got %q, want %q", fake.singleEpisode.Status, model.EpisodeStatusError)
	}
}

func TestDeleteEpisodeFileNotFound(t *testing.T) {
	fake := &fakePodcastStore{}
	svc := newTestService(fake)

	err := svc.DeleteEpisodeFile(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error for missing episode")
	}
}

func TestDeleteEpisodeFileSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "episode.mp3")
	if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	fake := &fakePodcastStore{singleEpisode: &model.PodcastEpisode{
		ID:           9,
		DownloadPath: path,
		FileSize:     4,
		Status:       model.EpisodeStatusCompleted,
	}}
	svc := newTestService(fake)

	if err := svc.DeleteEpisodeFile(context.Background(), 9); err != nil {
		t.Fatalf("DeleteEpisodeFile: unexpected error: %v", err)
	}
	if fake.singleEpisode.DownloadPath != "" {
		t.Error("expected DownloadPath to be cleared")
	}
	if fake.singleEpisode.Status != model.EpisodeStatusDeleted {
		t.Errorf("status: got %q, want %q", fake.singleEpisode.Status, model.EpisodeStatusDeleted)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be removed from disk")
	}
}

func TestDeleteEpisodeFileMissingOnDiskIsNotError(t *testing.T) {
	fake := &fakePodcastStore{singleEpisode: &model.PodcastEpisode{
		ID:           10,
		DownloadPath: "/nonexistent/path/episode.mp3",
		Status:       model.EpisodeStatusCompleted,
	}}
	svc := newTestService(fake)

	if err := svc.DeleteEpisodeFile(context.Background(), 10); err != nil {
		t.Fatalf("DeleteEpisodeFile: unexpected error for already-missing file: %v", err)
	}
}

func TestDeleteEpisodeFileNoDownloadPath(t *testing.T) {
	fake := &fakePodcastStore{singleEpisode: &model.PodcastEpisode{
		ID:     11,
		Status: model.EpisodeStatusNew,
	}}
	svc := newTestService(fake)

	if err := svc.DeleteEpisodeFile(context.Background(), 11); err != nil {
		t.Fatalf("DeleteEpisodeFile: unexpected error when DownloadPath is empty: %v", err)
	}
	if fake.singleEpisode.Status != model.EpisodeStatusDeleted {
		t.Errorf("status: got %q, want %q", fake.singleEpisode.Status, model.EpisodeStatusDeleted)
	}
}

func TestDownloadFileSuccess(t *testing.T) {
	const body = "hello world"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	svc := newNilDBService()
	dest := filepath.Join(t.TempDir(), "out.bin")

	n, err := svc.downloadFile(context.Background(), srv.URL, dest)
	if err != nil {
		t.Fatalf("downloadFile: unexpected error: %v", err)
	}
	if n != int64(len(body)) {
		t.Errorf("bytes written: got %d, want %d", n, len(body))
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(data) != body {
		t.Errorf("content: got %q, want %q", data, body)
	}
}

func TestDownloadFileHTTPFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	svc := newNilDBService()
	dest := filepath.Join(t.TempDir(), "out.bin")

	_, err := svc.downloadFile(context.Background(), srv.URL, dest)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestDownloadFileInvalidURL(t *testing.T) {
	svc := newNilDBService()
	dest := filepath.Join(t.TempDir(), "out.bin")

	_, err := svc.downloadFile(context.Background(), "://bad-url", dest)
	if err == nil {
		t.Fatal("expected error for malformed URL")
	}
}

// multiChannelStore extends fakePodcastStore to report a fixed set of channels
// for RefreshAll tests, independent of the single-channel field used elsewhere.
type multiChannelStore struct {
	*fakePodcastStore
	channels []*model.PodcastChannel
	listErr  error
}

func (f *multiChannelStore) ListChannels(_ context.Context) ([]*model.PodcastChannel, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.channels, nil
}

func (f *multiChannelStore) GetChannel(_ context.Context, id int64) (*model.PodcastChannel, error) {
	for _, c := range f.channels {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, nil
}

func TestRefreshAllToleratesPerChannelFailure(t *testing.T) {
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer badSrv.Close()

	fake := &multiChannelStore{
		fakePodcastStore: &fakePodcastStore{},
		channels: []*model.PodcastChannel{
			{ID: 1, Title: "bad", URL: badSrv.URL},
		},
	}
	db := &store.DB{Podcasts: fake}
	svc := NewService(db, t.TempDir(), log.New(io.Discard, "", 0))

	if err := svc.RefreshAll(context.Background()); err != nil {
		t.Fatalf("RefreshAll: unexpected error: %v", err)
	}
}

func TestRefreshAllListChannelsError(t *testing.T) {
	fake := &multiChannelStore{
		fakePodcastStore: &fakePodcastStore{},
		listErr:          errors.New("db down"),
	}
	db := &store.DB{Podcasts: fake}
	svc := NewService(db, t.TempDir(), log.New(io.Discard, "", 0))

	if err := svc.RefreshAll(context.Background()); err == nil {
		t.Fatal("expected error when ListChannels fails")
	}
}
