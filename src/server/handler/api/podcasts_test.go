package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// stubPodcastStore satisfies store.PodcastStore; all methods return errors by default.
type stubPodcastStore struct{}

func (s *stubPodcastStore) CreateChannel(ctx context.Context, ch *model.PodcastChannel) (int64, error) {
	return 0, errors.New("not implemented")
}
func (s *stubPodcastStore) GetChannel(ctx context.Context, id int64) (*model.PodcastChannel, error) {
	return nil, errors.New("not implemented")
}
func (s *stubPodcastStore) ListChannels(ctx context.Context) ([]*model.PodcastChannel, error) {
	return nil, errors.New("not implemented")
}
func (s *stubPodcastStore) UpdateChannel(ctx context.Context, ch *model.PodcastChannel) error {
	return errors.New("not implemented")
}
func (s *stubPodcastStore) DeleteChannel(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}
func (s *stubPodcastStore) GetEpisode(ctx context.Context, id int64) (*model.PodcastEpisode, error) {
	return nil, errors.New("not implemented")
}
func (s *stubPodcastStore) ListEpisodesByChannel(ctx context.Context, channelID int64) ([]*model.PodcastEpisode, error) {
	return nil, errors.New("not implemented")
}
func (s *stubPodcastStore) UpsertEpisode(ctx context.Context, ep *model.PodcastEpisode) (int64, error) {
	return 0, errors.New("not implemented")
}
func (s *stubPodcastStore) UpdateEpisodeStatus(ctx context.Context, id int64, status model.EpisodeStatus, lastErr string) error {
	return errors.New("not implemented")
}
func (s *stubPodcastStore) DeleteEpisode(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

// configPodcastStore embeds stubPodcastStore with configurable return values.
type configPodcastStore struct {
	*stubPodcastStore

	listChannelsResult  []*model.PodcastChannel
	listChannelsErr     error
	createChannelID     int64
	createChannelErr    error
	getChannelResult    *model.PodcastChannel
	getChannelErr       error
	updateChannelErr    error
	deleteChannelErr    error
	getEpisodeResult    *model.PodcastEpisode
	getEpisodeErr       error
	listEpisodesResult  []*model.PodcastEpisode
	listEpisodesErr     error
	updateEpStatusErr   error
	deleteEpisodeErr    error
}

func (s *configPodcastStore) ListChannels(ctx context.Context) ([]*model.PodcastChannel, error) {
	return s.listChannelsResult, s.listChannelsErr
}
func (s *configPodcastStore) CreateChannel(ctx context.Context, ch *model.PodcastChannel) (int64, error) {
	return s.createChannelID, s.createChannelErr
}
func (s *configPodcastStore) GetChannel(ctx context.Context, id int64) (*model.PodcastChannel, error) {
	return s.getChannelResult, s.getChannelErr
}
func (s *configPodcastStore) UpdateChannel(ctx context.Context, ch *model.PodcastChannel) error {
	return s.updateChannelErr
}
func (s *configPodcastStore) DeleteChannel(ctx context.Context, id int64) error {
	return s.deleteChannelErr
}
func (s *configPodcastStore) GetEpisode(ctx context.Context, id int64) (*model.PodcastEpisode, error) {
	return s.getEpisodeResult, s.getEpisodeErr
}
func (s *configPodcastStore) ListEpisodesByChannel(ctx context.Context, channelID int64) ([]*model.PodcastEpisode, error) {
	return s.listEpisodesResult, s.listEpisodesErr
}
func (s *configPodcastStore) UpdateEpisodeStatus(ctx context.Context, id int64, status model.EpisodeStatus, lastErr string) error {
	return s.updateEpStatusErr
}
func (s *configPodcastStore) DeleteEpisode(ctx context.Context, id int64) error {
	return s.deleteEpisodeErr
}

// newPodcastHandler builds a Handler wired to ps for podcast tests.
func newPodcastHandler(ps *configPodcastStore) *Handler {
	return newHealthHandler(&store.DB{
		Music:    &stubMusicStore{},
		Users:    &stubUserStoreForHealth{},
		Podcasts: ps,
	})
}

// podcastAdminUser returns the request with an admin auth context.
func podcastAdminUser(r *http.Request) *http.Request {
	return r.WithContext(mw.WithUser(r.Context(), &mw.AuthUser{
		ID: 1, Username: "admin", IsAdmin: true,
	}))
}

func TestListPodcasts_Success(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		listChannelsResult: []*model.PodcastChannel{
			{ID: 1, Title: "Tech Talk", URL: "https://example.com/feed.rss"},
		},
	}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodGet, "/api/v1/podcasts", nil))
	rec := httptest.NewRecorder()
	h.ListPodcasts(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestListPodcasts_StoreError(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		listChannelsErr:  errors.New("db error"),
	}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodGet, "/api/v1/podcasts", nil))
	rec := httptest.NewRecorder()
	h.ListPodcasts(rec, r)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestCreatePodcast_Success(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		createChannelID:  7,
	}
	h := newPodcastHandler(ps)
	body, _ := json.Marshal(map[string]any{"url": "https://feeds.example.com/podcast.rss"})
	r := podcastAdminUser(httptest.NewRequest(http.MethodPost, "/api/v1/podcasts", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.CreatePodcast(rec, r)
	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201", rec.Code)
	}
}

func TestCreatePodcast_MissingURL(t *testing.T) {
	ps := &configPodcastStore{stubPodcastStore: &stubPodcastStore{}}
	h := newPodcastHandler(ps)
	body, _ := json.Marshal(map[string]any{})
	r := podcastAdminUser(httptest.NewRequest(http.MethodPost, "/api/v1/podcasts", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.CreatePodcast(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestCreatePodcast_InvalidJSON(t *testing.T) {
	ps := &configPodcastStore{stubPodcastStore: &stubPodcastStore{}}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodPost, "/api/v1/podcasts", bytes.NewReader([]byte("not-json"))))
	rec := httptest.NewRecorder()
	h.CreatePodcast(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestCreatePodcast_StoreError(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		createChannelErr: errors.New("duplicate url"),
	}
	h := newPodcastHandler(ps)
	body, _ := json.Marshal(map[string]any{"url": "https://example.com/feed.rss"})
	r := podcastAdminUser(httptest.NewRequest(http.MethodPost, "/api/v1/podcasts", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.CreatePodcast(rec, r)
	if rec.Code != http.StatusConflict {
		t.Errorf("status: got %d, want 409", rec.Code)
	}
}

func TestGetPodcast_Success(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		getChannelResult: &model.PodcastChannel{ID: 1, Title: "Tech Talk"},
	}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodGet, "/api/v1/podcasts/1", nil))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.GetPodcast(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestGetPodcast_NotFound(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		getChannelResult: nil,
	}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodGet, "/api/v1/podcasts/99", nil))
	r = withChiID(r, "id", "99")
	rec := httptest.NewRecorder()
	h.GetPodcast(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestGetPodcast_BadID(t *testing.T) {
	ps := &configPodcastStore{stubPodcastStore: &stubPodcastStore{}}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodGet, "/api/v1/podcasts/abc", nil))
	r = withChiID(r, "id", "abc")
	rec := httptest.NewRecorder()
	h.GetPodcast(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestUpdatePodcast_Success(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		getChannelResult: &model.PodcastChannel{ID: 1, Title: "Old Title"},
	}
	h := newPodcastHandler(ps)
	body, _ := json.Marshal(map[string]any{"title": "New Title"})
	r := podcastAdminUser(httptest.NewRequest(http.MethodPut, "/api/v1/podcasts/1", bytes.NewReader(body)))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.UpdatePodcast(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestUpdatePodcast_NotFound(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		getChannelResult: nil,
	}
	h := newPodcastHandler(ps)
	body, _ := json.Marshal(map[string]any{"title": "New Title"})
	r := podcastAdminUser(httptest.NewRequest(http.MethodPut, "/api/v1/podcasts/1", bytes.NewReader(body)))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.UpdatePodcast(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestDeletePodcast_Success(t *testing.T) {
	ps := &configPodcastStore{stubPodcastStore: &stubPodcastStore{}}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodDelete, "/api/v1/podcasts/1", nil))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.DeletePodcast(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestDeletePodcast_StoreError(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		deleteChannelErr: errors.New("db error"),
	}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodDelete, "/api/v1/podcasts/1", nil))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.DeletePodcast(rec, r)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestListPodcastEpisodes_Success(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		getChannelResult: &model.PodcastChannel{ID: 1, Title: "Tech Talk"},
		listEpisodesResult: []*model.PodcastEpisode{
			{ID: 1, ChannelID: 1, Title: "Episode 1"},
		},
	}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodGet, "/api/v1/podcasts/1/episodes", nil))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.ListPodcastEpisodes(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestListPodcastEpisodes_ChannelNotFound(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		getChannelResult: nil,
	}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodGet, "/api/v1/podcasts/99/episodes", nil))
	r = withChiID(r, "id", "99")
	rec := httptest.NewRecorder()
	h.ListPodcastEpisodes(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestGetPodcastEpisode_Success(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		getEpisodeResult: &model.PodcastEpisode{ID: 5, ChannelID: 1, Title: "Deep Dive"},
	}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodGet, "/api/v1/podcasts/episodes/5", nil))
	r = withChiID(r, "id", "5")
	rec := httptest.NewRecorder()
	h.GetPodcastEpisode(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestGetPodcastEpisode_NotFound(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		getEpisodeResult: nil,
	}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodGet, "/api/v1/podcasts/episodes/99", nil))
	r = withChiID(r, "id", "99")
	rec := httptest.NewRecorder()
	h.GetPodcastEpisode(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestDownloadPodcastEpisode_Success(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		getEpisodeResult: &model.PodcastEpisode{ID: 5, ChannelID: 1},
	}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodPost, "/api/v1/podcasts/episodes/5/download", nil))
	r = withChiID(r, "id", "5")
	rec := httptest.NewRecorder()
	h.DownloadPodcastEpisode(rec, r)
	if rec.Code != http.StatusAccepted {
		t.Errorf("status: got %d, want 202", rec.Code)
	}
}

func TestDownloadPodcastEpisode_NotFound(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		getEpisodeResult: nil,
	}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodPost, "/api/v1/podcasts/episodes/99/download", nil))
	r = withChiID(r, "id", "99")
	rec := httptest.NewRecorder()
	h.DownloadPodcastEpisode(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestDownloadPodcastEpisode_StatusUpdateError(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore:  &stubPodcastStore{},
		getEpisodeResult:  &model.PodcastEpisode{ID: 5, ChannelID: 1},
		updateEpStatusErr: errors.New("db error"),
	}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodPost, "/api/v1/podcasts/episodes/5/download", nil))
	r = withChiID(r, "id", "5")
	rec := httptest.NewRecorder()
	h.DownloadPodcastEpisode(rec, r)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestDeletePodcastEpisode_Success(t *testing.T) {
	ps := &configPodcastStore{stubPodcastStore: &stubPodcastStore{}}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodDelete, "/api/v1/podcasts/episodes/5", nil))
	r = withChiID(r, "id", "5")
	rec := httptest.NewRecorder()
	h.DeletePodcastEpisode(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestDeletePodcastEpisode_StoreError(t *testing.T) {
	ps := &configPodcastStore{
		stubPodcastStore: &stubPodcastStore{},
		deleteEpisodeErr: errors.New("db error"),
	}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodDelete, "/api/v1/podcasts/episodes/5", nil))
	r = withChiID(r, "id", "5")
	rec := httptest.NewRecorder()
	h.DeletePodcastEpisode(rec, r)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestDeletePodcastEpisode_BadID(t *testing.T) {
	ps := &configPodcastStore{stubPodcastStore: &stubPodcastStore{}}
	h := newPodcastHandler(ps)
	r := podcastAdminUser(httptest.NewRequest(http.MethodDelete, "/api/v1/podcasts/episodes/abc", nil))
	r = withChiID(r, "id", "abc")
	rec := httptest.NewRecorder()
	h.DeletePodcastEpisode(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}
