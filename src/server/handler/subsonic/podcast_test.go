package subsonic

// Tests for podcast.go handler methods: getPodcasts, getNewestPodcasts,
// refreshPodcasts, createPodcastChannel, deletePodcastChannel,
// deletePodcastEpisode, downloadPodcastEpisode, and podcastChannelToResp.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// ---- getPodcasts -------------------------------------------------------------

func TestGetPodcastsUnauthenticated(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getPodcasts?f=json", nil)

	h.getPodcasts(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getPodcasts unauthenticated: got %+v", resp.Error)
	}
}

func TestGetPodcastsEmpty(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getPodcasts?f=json", false)

	h.getPodcasts(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("getPodcasts: status %q, want ok", resp.Status)
	}
	if resp.Podcasts == nil {
		t.Fatal("getPodcasts: Podcasts is nil")
	}
	if len(resp.Podcasts.Channel) != 0 {
		t.Errorf("getPodcasts: got %d channels, want 0 (no podcast store)", len(resp.Podcasts.Channel))
	}
}

func TestGetPodcastsIncludeEpisodesFlag(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getPodcasts?f=json&includeEpisodes=true", false)

	h.getPodcasts(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("getPodcasts includeEpisodes: status %q, want ok", resp.Status)
	}
}

// ---- getNewestPodcasts -------------------------------------------------------

func TestGetNewestPodcastsUnauthenticated(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getNewestPodcasts?f=json", nil)

	h.getNewestPodcasts(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getNewestPodcasts unauthenticated: got %+v", resp.Error)
	}
}

func TestGetNewestPodcastsEmpty(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getNewestPodcasts?f=json&count=5", false)

	h.getNewestPodcasts(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.NewestPodcasts == nil {
		t.Fatal("getNewestPodcasts: NewestPodcasts is nil")
	}
	if len(resp.NewestPodcasts.Episode) != 0 {
		t.Errorf("getNewestPodcasts: got %d episodes, want 0", len(resp.NewestPodcasts.Episode))
	}
}

// ---- refreshPodcasts (admin only) -------------------------------------------

func TestRefreshPodcastsUnauthenticated(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/refreshPodcasts?f=json", nil)

	h.refreshPodcasts(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("refreshPodcasts unauthenticated: got %+v", resp.Error)
	}
}

func TestRefreshPodcastsForbiddenNonAdmin(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/refreshPodcasts?f=json", false)

	h.refreshPodcasts(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrForbidden {
		t.Errorf("refreshPodcasts non-admin: got %+v, want code %d", resp.Error, ErrForbidden)
	}
}

func TestRefreshPodcastsAdminSucceeds(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/refreshPodcasts?f=json", true)

	h.refreshPodcasts(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("refreshPodcasts admin: status %q, want ok", resp.Status)
	}
}

// ---- createPodcastChannel (admin only) --------------------------------------

func TestCreatePodcastChannelForbiddenNonAdmin(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/createPodcastChannel?f=json&url=http://example.com/feed.xml", false)

	h.createPodcastChannel(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrForbidden {
		t.Errorf("createPodcastChannel non-admin: got %+v, want code %d", resp.Error, ErrForbidden)
	}
}

func TestCreatePodcastChannelMissingURL(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/createPodcastChannel?f=json", true)

	h.createPodcastChannel(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("createPodcastChannel missing url: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestCreatePodcastChannelSucceeds(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/createPodcastChannel?f=json&url=http://example.com/feed.xml", true)

	h.createPodcastChannel(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("createPodcastChannel: status %q, want ok", resp.Status)
	}
}

// ---- deletePodcastChannel (admin only) --------------------------------------

func TestDeletePodcastChannelForbiddenNonAdmin(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deletePodcastChannel?f=json&id="+encodePodcastID(1), false)

	h.deletePodcastChannel(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrForbidden {
		t.Errorf("deletePodcastChannel non-admin: got %+v, want code %d", resp.Error, ErrForbidden)
	}
}

func TestDeletePodcastChannelMissingID(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deletePodcastChannel?f=json", true)

	h.deletePodcastChannel(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("deletePodcastChannel missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestDeletePodcastChannelSucceeds(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deletePodcastChannel?f=json&id="+encodePodcastID(1), true)

	h.deletePodcastChannel(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("deletePodcastChannel: status %q, want ok", resp.Status)
	}
}

// ---- deletePodcastEpisode (admin only) --------------------------------------

func TestDeletePodcastEpisodeForbiddenNonAdmin(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deletePodcastEpisode?f=json&id=1", false)

	h.deletePodcastEpisode(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrForbidden {
		t.Errorf("deletePodcastEpisode non-admin: got %+v, want code %d", resp.Error, ErrForbidden)
	}
}

func TestDeletePodcastEpisodeMissingID(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deletePodcastEpisode?f=json", true)

	h.deletePodcastEpisode(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("deletePodcastEpisode missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

// ---- downloadPodcastEpisode (admin only) ------------------------------------

func TestDownloadPodcastEpisodeUnauthenticated(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/downloadPodcastEpisode?f=json&id=1", nil)

	h.downloadPodcastEpisode(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("downloadPodcastEpisode unauthenticated: got %+v", resp.Error)
	}
}

func TestDownloadPodcastEpisodeMissingID(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/downloadPodcastEpisode?f=json", true)

	h.downloadPodcastEpisode(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("downloadPodcastEpisode missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestDownloadPodcastEpisodeSucceeds(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/downloadPodcastEpisode?f=json&id=1", true)

	h.downloadPodcastEpisode(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("downloadPodcastEpisode: status %q, want ok", resp.Status)
	}
}

// ---- podcastChannelToResp ------------------------------------------------------

func TestPodcastChannelToResp(t *testing.T) {
	ch := &model.PodcastChannel{
		ID:               7,
		URL:              "http://example.com/feed.xml",
		Title:            "My Podcast",
		Description:      "A podcast",
		OriginalImageURL: "http://example.com/image.png",
		Status:           model.PodcastStatusCompleted,
		LastError:        "",
	}
	resp := podcastChannelToResp(ch)
	if resp.ID != encodePodcastID(7) {
		t.Errorf("podcastChannelToResp: ID = %q, want %q", resp.ID, encodePodcastID(7))
	}
	if resp.Status != "completed" {
		t.Errorf("podcastChannelToResp: Status = %q, want completed", resp.Status)
	}
	if resp.Title != "My Podcast" {
		t.Errorf("podcastChannelToResp: Title = %q, want My Podcast", resp.Title)
	}
}

func TestPodcastChannelToRespErrorStatus(t *testing.T) {
	ch := &model.PodcastChannel{
		ID:        1,
		Status:    model.PodcastStatusError,
		LastError: "feed unreachable",
	}
	resp := podcastChannelToResp(ch)
	if resp.Status != "error" {
		t.Errorf("podcastChannelToResp: Status = %q, want error", resp.Status)
	}
	if resp.ErrorMessage != "feed unreachable" {
		t.Errorf("podcastChannelToResp: ErrorMessage = %q, want feed unreachable", resp.ErrorMessage)
	}
}
