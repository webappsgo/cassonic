package ampache

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// TestLiveStreams covers listing internet radio stations and the store-error path.
func TestLiveStreams(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.radioStations = []*model.InternetRadioStation{{ID: 1, Name: "R", StreamURL: "http://x"}}
		rec := httptest.NewRecorder()
		h.liveStreams(rec, newRequest("live_streams", map[string]string{"auth": token}), true)
		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		list, ok := got["live_stream"].([]any)
		if !ok || len(list) != 1 {
			t.Fatalf("expected 1 station, got %v", got)
		}
	})

	t.Run("store error", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.radioStationsErr = errors.New("x")
		rec := httptest.NewRecorder()
		h.liveStreams(rec, newRequest("live_streams", map[string]string{"auth": token}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4710 {
			t.Fatalf("expected 4710, got %+v", got)
		}
	})
}

// TestLiveStream covers the single-station lookup: missing filter, not found, success.
func TestLiveStream(t *testing.T) {
	h, ts, token := newAuthedHandler(1)

	rec := httptest.NewRecorder()
	h.liveStream(rec, newRequest("live_stream", map[string]string{"auth": token}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	rec2 := httptest.NewRecorder()
	h.liveStream(rec2, newRequest("live_stream", map[string]string{"auth": token, "filter": "1"}), true)
	var notFound xmlErrorJSON
	decodeJSON(t, rec2.Body.Bytes(), &notFound)
	if notFound.ErrorCode != 4704 {
		t.Fatalf("expected 4704, got %+v", notFound)
	}

	ts.users.radioStation = &model.InternetRadioStation{ID: 1, Name: "R", StreamURL: "http://x"}
	rec3 := httptest.NewRecorder()
	h.liveStream(rec3, newRequest("live_stream", map[string]string{"auth": token, "filter": "1"}), true)
	var got AmpLiveStream
	decodeJSON(t, rec3.Body.Bytes(), &got)
	if got.ID != "1" || got.Name != "R" || got.URL != "http://x" || got.IsPublic != 1 {
		t.Fatalf("unexpected station: %+v", got)
	}
}

// TestLiveStreamCreate covers admin-only creation, missing params, and store error.
func TestLiveStreamCreate(t *testing.T) {
	t.Run("non-admin denied", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, IsAdmin: false}
		rec := httptest.NewRecorder()
		h.liveStreamCreate(rec, newRequest("live_stream_create", map[string]string{"auth": token, "name": "R", "url": "http://x"}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4742 {
			t.Fatalf("expected 4742, got %+v", got)
		}
	})

	t.Run("missing params", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, IsAdmin: true}
		rec := httptest.NewRecorder()
		h.liveStreamCreate(rec, newRequest("live_stream_create", map[string]string{"auth": token}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4705 {
			t.Fatalf("expected 4705, got %+v", got)
		}
	})

	t.Run("store error", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, IsAdmin: true}
		ts.users.createRadioErr = errors.New("x")
		rec := httptest.NewRecorder()
		h.liveStreamCreate(rec, newRequest("live_stream_create", map[string]string{"auth": token, "name": "R", "url": "http://x"}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4710 {
			t.Fatalf("expected 4710, got %+v", got)
		}
	})

	t.Run("success", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, IsAdmin: true}
		ts.users.createRadioID = 42
		rec := httptest.NewRecorder()
		h.liveStreamCreate(rec, newRequest("live_stream_create", map[string]string{
			"auth": token, "name": "R", "url": "http://x", "site_url": "http://home",
		}), true)
		var got AmpLiveStream
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ID != "42" || got.SiteURL != "http://home" {
			t.Fatalf("unexpected created station: %+v", got)
		}
	})
}

// TestLiveStreamEdit covers admin-only editing: missing filter, not found,
// partial field update (only non-empty params overwrite), and store error.
func TestLiveStreamEdit(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.users.user = &model.User{ID: 1, IsAdmin: true}

	rec := httptest.NewRecorder()
	h.liveStreamEdit(rec, newRequest("live_stream_edit", map[string]string{"auth": token}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	rec2 := httptest.NewRecorder()
	h.liveStreamEdit(rec2, newRequest("live_stream_edit", map[string]string{"auth": token, "filter": "1"}), true)
	var notFound xmlErrorJSON
	decodeJSON(t, rec2.Body.Bytes(), &notFound)
	if notFound.ErrorCode != 4704 {
		t.Fatalf("expected 4704, got %+v", notFound)
	}

	ts.users.radioStation = &model.InternetRadioStation{ID: 1, Name: "Old", StreamURL: "http://old"}
	rec3 := httptest.NewRecorder()
	h.liveStreamEdit(rec3, newRequest("live_stream_edit", map[string]string{"auth": token, "filter": "1", "name": "New"}), true)
	var got AmpLiveStream
	decodeJSON(t, rec3.Body.Bytes(), &got)
	if got.Name != "New" || got.URL != "http://old" {
		t.Fatalf("expected partial update to preserve unset fields, got %+v", got)
	}

	ts.users.updateRadioErr = errors.New("x")
	rec4 := httptest.NewRecorder()
	h.liveStreamEdit(rec4, newRequest("live_stream_edit", map[string]string{"auth": token, "filter": "1"}), true)
	var errGot xmlErrorJSON
	decodeJSON(t, rec4.Body.Bytes(), &errGot)
	if errGot.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", errGot)
	}
}

// TestLiveStreamDelete covers admin-only deletion: missing filter, success, and store error.
func TestLiveStreamDelete(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.users.user = &model.User{ID: 1, IsAdmin: true}

	rec := httptest.NewRecorder()
	h.liveStreamDelete(rec, newRequest("live_stream_delete", map[string]string{"auth": token}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	rec2 := httptest.NewRecorder()
	h.liveStreamDelete(rec2, newRequest("live_stream_delete", map[string]string{"auth": token, "filter": "1"}), true)
	var got map[string]any
	decodeJSON(t, rec2.Body.Bytes(), &got)
	if got["success"] == nil {
		t.Fatalf("expected success key, got %v", got)
	}

	ts.users.deleteRadioErr = errors.New("x")
	rec3 := httptest.NewRecorder()
	h.liveStreamDelete(rec3, newRequest("live_stream_delete", map[string]string{"auth": token, "filter": "1"}), true)
	var errGot xmlErrorJSON
	decodeJSON(t, rec3.Body.Bytes(), &errGot)
	if errGot.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", errGot)
	}
}
