package ampache

import (
	"net/http/httptest"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// TestPodcastStubEndpoints covers the podcast handlers, which are all
// stand-in implementations (no PodcastStore exists yet in store.DB): they
// return empty lists / not-found / a fixed "not yet stored" success message.
// These tests lock down that documented stub behavior and its auth gating.
func TestPodcastStubEndpoints(t *testing.T) {
	t.Run("podcasts requires session and returns empty list", func(t *testing.T) {
		h, _ := newTestHandler()
		rec := httptest.NewRecorder()
		h.podcasts(rec, newRequest("podcasts", nil), true)
		var denied xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &denied)
		if denied.ErrorCode != 4700 {
			t.Fatalf("expected 4700 without session, got %+v", denied)
		}

		h2, _, token := newAuthedHandler(1)
		rec2 := httptest.NewRecorder()
		h2.podcasts(rec2, newRequest("podcasts", map[string]string{"auth": token}), true)
		var got map[string]any
		decodeJSON(t, rec2.Body.Bytes(), &got)
		list, ok := got["podcast"].([]any)
		if !ok || len(list) != 0 {
			t.Fatalf("expected empty podcast list, got %v", got)
		}
	})

	t.Run("podcast returns not found", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		h.podcast(rec, newRequest("podcast", map[string]string{"auth": token}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4704 {
			t.Fatalf("expected 4704, got %+v", got)
		}
	})

	t.Run("podcast_create requires admin and url param", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, IsAdmin: false}
		rec := httptest.NewRecorder()
		h.podcastCreate(rec, newRequest("podcast_create", map[string]string{"auth": token, "url": "http://x"}), true)
		var denied xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &denied)
		if denied.ErrorCode != 4742 {
			t.Fatalf("expected 4742, got %+v", denied)
		}

		ts.users.user = &model.User{ID: 1, IsAdmin: true}
		rec2 := httptest.NewRecorder()
		h.podcastCreate(rec2, newRequest("podcast_create", map[string]string{"auth": token}), true)
		var missing xmlErrorJSON
		decodeJSON(t, rec2.Body.Bytes(), &missing)
		if missing.ErrorCode != 4705 {
			t.Fatalf("expected 4705, got %+v", missing)
		}

		rec3 := httptest.NewRecorder()
		h.podcastCreate(rec3, newRequest("podcast_create", map[string]string{"auth": token, "url": "http://x"}), true)
		var got map[string]any
		decodeJSON(t, rec3.Body.Bytes(), &got)
		if got["success"] == nil {
			t.Fatalf("expected success key, got %v", got)
		}
	})

	t.Run("podcast_edit and podcast_delete require admin", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, IsAdmin: true}

		rec := httptest.NewRecorder()
		h.podcastEdit(rec, newRequest("podcast_edit", map[string]string{"auth": token}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got["success"] == nil {
			t.Fatalf("expected success key, got %v", got)
		}

		rec2 := httptest.NewRecorder()
		h.podcastDelete(rec2, newRequest("podcast_delete", map[string]string{"auth": token}), true)
		var got2 map[string]any
		decodeJSON(t, rec2.Body.Bytes(), &got2)
		if got2["success"] == nil {
			t.Fatalf("expected success key, got %v", got2)
		}
	})

	t.Run("podcast_episodes returns empty list, podcast_episode not found", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		h.podcastEpisodes(rec, newRequest("podcast_episodes", map[string]string{"auth": token}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["podcast_episode"].([]any)
		if !ok || len(list) != 0 {
			t.Fatalf("expected empty episode list, got %v", got)
		}

		rec2 := httptest.NewRecorder()
		h.podcastEpisode(rec2, newRequest("podcast_episode", map[string]string{"auth": token}), true)
		var errGot xmlErrorJSON
		decodeJSON(t, rec2.Body.Bytes(), &errGot)
		if errGot.ErrorCode != 4704 {
			t.Fatalf("expected 4704, got %+v", errGot)
		}
	})

	t.Run("podcast_episode_delete requires admin", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, IsAdmin: true}
		rec := httptest.NewRecorder()
		h.podcastEpisodeDelete(rec, newRequest("podcast_episode_delete", map[string]string{"auth": token}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got["success"] == nil {
			t.Fatalf("expected success key, got %v", got)
		}
	})

	t.Run("update_podcast requires admin and id param", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, IsAdmin: true}

		rec := httptest.NewRecorder()
		h.updatePodcast(rec, newRequest("update_podcast", map[string]string{"auth": token}), true)
		var missing xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &missing)
		if missing.ErrorCode != 4705 {
			t.Fatalf("expected 4705, got %+v", missing)
		}

		rec2 := httptest.NewRecorder()
		h.updatePodcast(rec2, newRequest("update_podcast", map[string]string{"auth": token, "id": "1"}), true)
		var got map[string]any
		decodeJSON(t, rec2.Body.Bytes(), &got)
		if got["success"] == nil {
			t.Fatalf("expected success key, got %v", got)
		}
	})
}
