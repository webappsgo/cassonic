package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// configPlaylistStore is a configurable stub for PlaylistStore.
type configPlaylistStore struct {
	listResult []*model.Playlist
	listErr    error

	createResult int64
	createErr    error

	getResult *model.Playlist
	getErr    error

	updateErr error
	deleteErr error

	entriesResult []*model.PlaylistEntry
	entriesErr    error

	setEntriesErr error
	addErr        error
	removeErr     error
}

func (s *configPlaylistStore) CreatePlaylist(ctx context.Context, p *model.Playlist) (int64, error) {
	return s.createResult, s.createErr
}

func (s *configPlaylistStore) GetPlaylist(ctx context.Context, id int64) (*model.Playlist, error) {
	return s.getResult, s.getErr
}

func (s *configPlaylistStore) ListPlaylists(ctx context.Context, userID int64) ([]*model.Playlist, error) {
	return s.listResult, s.listErr
}

func (s *configPlaylistStore) UpdatePlaylist(ctx context.Context, p *model.Playlist) error {
	return s.updateErr
}

func (s *configPlaylistStore) DeletePlaylist(ctx context.Context, id int64) error {
	return s.deleteErr
}

func (s *configPlaylistStore) GetPlaylistEntries(ctx context.Context, playlistID int64) ([]*model.PlaylistEntry, error) {
	return s.entriesResult, s.entriesErr
}

func (s *configPlaylistStore) SetPlaylistEntries(ctx context.Context, playlistID int64, songIDs []int64) error {
	return s.setEntriesErr
}

func (s *configPlaylistStore) AddToPlaylist(ctx context.Context, playlistID int64, songIDs []int64) error {
	return s.addErr
}

func (s *configPlaylistStore) RemoveFromPlaylist(ctx context.Context, playlistID int64, indices []int) error {
	return s.removeErr
}

// newPlaylistHandler builds a Handler with the given playlist and music stores.
func newPlaylistHandler(ps store.PlaylistStore, ms store.MusicStore) *Handler {
	if ms == nil {
		ms = &stubMusicStore{}
	}
	return newHealthHandler(&store.DB{
		Music:     ms,
		Users:     &stubUserStoreForHealth{},
		Playlists: ps,
	})
}

// jsonBody serialises v to JSON and returns the result as a *bytes.Buffer.
func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("jsonBody marshal: %v", err)
	}
	return bytes.NewBuffer(b)
}

// withChiParams injects multiple chi URL parameters into the request context.
// pairs is alternating key, value strings.
func withChiParams(r *http.Request, pairs ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(pairs); i += 2 {
		rctx.URLParams.Add(pairs[i], pairs[i+1])
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// ------------------------------------------------------------------
// ListPlaylists
// ------------------------------------------------------------------

func TestListPlaylists_Success(t *testing.T) {
	playlists := []*model.Playlist{
		{ID: 1, UserID: 1, Name: "Chill"},
		{ID: 2, UserID: 1, Name: "Workout"},
	}
	h := newPlaylistHandler(&configPlaylistStore{listResult: playlists}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists", nil)
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.ListPlaylists(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	_, data := decodeEnvelope(t, w.Body)
	if total, _ := data["total"].(float64); total != 2 {
		t.Errorf("expected total 2, got %v", data["total"])
	}
}

func TestListPlaylists_NoAuth(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists", nil)
	w := httptest.NewRecorder()
	h.ListPlaylists(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestListPlaylists_DBError(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{listErr: errors.New("db down")}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists", nil)
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.ListPlaylists(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestListPlaylists_EmptyList(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{listResult: nil}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists", nil)
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.ListPlaylists(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ------------------------------------------------------------------
// CreatePlaylist
// ------------------------------------------------------------------

func TestCreatePlaylist_Success(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{createResult: 42}, nil)
	body := jsonBody(t, map[string]any{"name": "Road Trip", "is_public": true})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/playlists", body)
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreatePlaylist(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestCreatePlaylist_NoAuth(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{createResult: 1}, nil)
	body := jsonBody(t, map[string]any{"name": "X"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/playlists", body)
	w := httptest.NewRecorder()
	h.CreatePlaylist(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreatePlaylist_BadJSON(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/playlists", strings.NewReader("{bad"))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreatePlaylist(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreatePlaylist_MissingName(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	body := jsonBody(t, map[string]any{"comment": "no name"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/playlists", body)
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreatePlaylist(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreatePlaylist_DBError(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{createErr: errors.New("db error")}, nil)
	body := jsonBody(t, map[string]any{"name": "Fail"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/playlists", body)
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreatePlaylist(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ------------------------------------------------------------------
// GetPlaylist
// ------------------------------------------------------------------

func TestGetPlaylist_Success(t *testing.T) {
	pl := &model.Playlist{ID: 5, UserID: 1, Name: "Mine"}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists/5", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "5")
	w := httptest.NewRecorder()
	h.GetPlaylist(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetPlaylist_NoAuth(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists/5", nil)
	r = withChiID(r, "id", "5")
	w := httptest.NewRecorder()
	h.GetPlaylist(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGetPlaylist_BadID(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists/abc", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "abc")
	w := httptest.NewRecorder()
	h.GetPlaylist(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetPlaylist_NotFound(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{getResult: nil}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists/99", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "99")
	w := httptest.NewRecorder()
	h.GetPlaylist(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetPlaylist_ForbiddenOtherUserPrivate(t *testing.T) {
	pl := &model.Playlist{ID: 5, UserID: 2, Name: "Theirs", IsPublic: false}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists/5", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "5")
	w := httptest.NewRecorder()
	h.GetPlaylist(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestGetPlaylist_AllowOtherUserPublic(t *testing.T) {
	pl := &model.Playlist{ID: 5, UserID: 2, Name: "Public", IsPublic: true}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists/5", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "5")
	w := httptest.NewRecorder()
	h.GetPlaylist(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetPlaylist_AdminCanSeePrivate(t *testing.T) {
	pl := &model.Playlist{ID: 5, UserID: 2, Name: "Hidden", IsPublic: false}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists/5", nil)
	r = withAuthUser(r, 99, "admin", true)
	r = withChiID(r, "id", "5")
	w := httptest.NewRecorder()
	h.GetPlaylist(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ------------------------------------------------------------------
// UpdatePlaylist
// ------------------------------------------------------------------

func TestUpdatePlaylist_Success(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 1, Name: "Old"}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl}, nil)
	body := jsonBody(t, map[string]any{"name": "New"})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/playlists/1", body)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.UpdatePlaylist(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestUpdatePlaylist_NoAuth(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/playlists/1", jsonBody(t, map[string]any{"name": "X"}))
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.UpdatePlaylist(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestUpdatePlaylist_BadID(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/playlists/x", jsonBody(t, map[string]any{"name": "X"}))
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "x")
	w := httptest.NewRecorder()
	h.UpdatePlaylist(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdatePlaylist_NotFound(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{getResult: nil}, nil)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/playlists/1", jsonBody(t, map[string]any{"name": "X"}))
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.UpdatePlaylist(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdatePlaylist_Forbidden(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 2, Name: "Theirs"}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl}, nil)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/playlists/1", jsonBody(t, map[string]any{"name": "X"}))
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.UpdatePlaylist(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestUpdatePlaylist_DBError(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 1, Name: "Mine"}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl, updateErr: errors.New("db error")}, nil)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/playlists/1", jsonBody(t, map[string]any{"name": "X"}))
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.UpdatePlaylist(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ------------------------------------------------------------------
// DeletePlaylist
// ------------------------------------------------------------------

func TestDeletePlaylist_Success(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 1, Name: "Mine"}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl}, nil)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/playlists/1", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.DeletePlaylist(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDeletePlaylist_NoAuth(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/playlists/1", nil)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.DeletePlaylist(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestDeletePlaylist_BadID(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/playlists/bad", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "bad")
	w := httptest.NewRecorder()
	h.DeletePlaylist(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeletePlaylist_NotFound(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{getResult: nil}, nil)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/playlists/1", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.DeletePlaylist(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeletePlaylist_Forbidden(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 2, Name: "Theirs"}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl}, nil)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/playlists/1", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.DeletePlaylist(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestDeletePlaylist_DBError(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 1, Name: "Mine"}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl, deleteErr: errors.New("db error")}, nil)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/playlists/1", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.DeletePlaylist(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ------------------------------------------------------------------
// GetPlaylistSongs
// ------------------------------------------------------------------

func TestGetPlaylistSongs_Success(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 1}
	entries := []*model.PlaylistEntry{
		{ID: 1, PlaylistID: 1, SongID: 10, Position: 0},
	}
	song := &model.Song{ID: 10, Title: "Track One"}
	ms := &configMusicStore{stubMusicStore: &stubMusicStore{}, getSongResult: song}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl, entriesResult: entries}, ms)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists/1/songs", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.GetPlaylistSongs(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetPlaylistSongs_NoAuth(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists/1/songs", nil)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.GetPlaylistSongs(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGetPlaylistSongs_BadID(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists/bad/songs", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "bad")
	w := httptest.NewRecorder()
	h.GetPlaylistSongs(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetPlaylistSongs_NotFound(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{getResult: nil}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists/1/songs", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.GetPlaylistSongs(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetPlaylistSongs_Forbidden(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 2, IsPublic: false}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists/1/songs", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.GetPlaylistSongs(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestGetPlaylistSongs_EntriesError(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 1}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl, entriesErr: errors.New("db down")}, nil)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists/1/songs", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.GetPlaylistSongs(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGetPlaylistSongs_SongSkippedOnMiss(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 1}
	entries := []*model.PlaylistEntry{
		{ID: 1, PlaylistID: 1, SongID: 999, Position: 0},
	}
	ms := &configMusicStore{stubMusicStore: &stubMusicStore{}, getSongResult: nil}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl, entriesResult: entries}, ms)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/playlists/1/songs", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.GetPlaylistSongs(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	_, data := decodeEnvelope(t, w.Body)
	if total, _ := data["total"].(float64); total != 0 {
		t.Errorf("expected 0 songs (miss), got %v", total)
	}
}

// ------------------------------------------------------------------
// AddPlaylistSongs
// ------------------------------------------------------------------

func TestAddPlaylistSongs_Success(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 1}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl}, nil)
	body := jsonBody(t, map[string]any{"song_ids": []int64{10, 20}})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/playlists/1/songs", body)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.AddPlaylistSongs(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAddPlaylistSongs_NoAuth(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/playlists/1/songs", jsonBody(t, map[string]any{"song_ids": []int64{1}}))
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.AddPlaylistSongs(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAddPlaylistSongs_BadID(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/playlists/x/songs", jsonBody(t, map[string]any{"song_ids": []int64{1}}))
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "x")
	w := httptest.NewRecorder()
	h.AddPlaylistSongs(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAddPlaylistSongs_NotFound(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{getResult: nil}, nil)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/playlists/1/songs", jsonBody(t, map[string]any{"song_ids": []int64{1}}))
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.AddPlaylistSongs(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAddPlaylistSongs_Forbidden(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 2}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl}, nil)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/playlists/1/songs", jsonBody(t, map[string]any{"song_ids": []int64{1}}))
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.AddPlaylistSongs(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestAddPlaylistSongs_EmptySongIDs(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 1}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl}, nil)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/playlists/1/songs", jsonBody(t, map[string]any{"song_ids": []int64{}}))
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.AddPlaylistSongs(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAddPlaylistSongs_DBError(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 1}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl, addErr: errors.New("db error")}, nil)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/playlists/1/songs", jsonBody(t, map[string]any{"song_ids": []int64{1}}))
	r = withAuthUser(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.AddPlaylistSongs(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ------------------------------------------------------------------
// RemovePlaylistSong
// ------------------------------------------------------------------

func TestRemovePlaylistSong_Success(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 1}
	entries := []*model.PlaylistEntry{
		{ID: 1, PlaylistID: 1, SongID: 10, Position: 0},
	}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl, entriesResult: entries}, nil)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/playlists/1/songs/10", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiParams(r, "id", "1", "songId", "10")
	w := httptest.NewRecorder()
	h.RemovePlaylistSong(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRemovePlaylistSong_NoAuth(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/playlists/1/songs/10", nil)
	r = withChiParams(r, "id", "1", "songId", "10")
	w := httptest.NewRecorder()
	h.RemovePlaylistSong(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRemovePlaylistSong_BadPlaylistID(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/playlists/bad/songs/10", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiParams(r, "id", "bad", "songId", "10")
	w := httptest.NewRecorder()
	h.RemovePlaylistSong(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRemovePlaylistSong_BadSongID(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{}, nil)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/playlists/1/songs/bad", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiParams(r, "id", "1", "songId", "bad")
	w := httptest.NewRecorder()
	h.RemovePlaylistSong(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRemovePlaylistSong_NotFound(t *testing.T) {
	h := newPlaylistHandler(&configPlaylistStore{getResult: nil}, nil)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/playlists/1/songs/10", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiParams(r, "id", "1", "songId", "10")
	w := httptest.NewRecorder()
	h.RemovePlaylistSong(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRemovePlaylistSong_Forbidden(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 2}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl}, nil)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/playlists/1/songs/10", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiParams(r, "id", "1", "songId", "10")
	w := httptest.NewRecorder()
	h.RemovePlaylistSong(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRemovePlaylistSong_SongNotInPlaylist(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 1}
	entries := []*model.PlaylistEntry{
		{ID: 1, PlaylistID: 1, SongID: 99, Position: 0},
	}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl, entriesResult: entries}, nil)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/playlists/1/songs/10", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiParams(r, "id", "1", "songId", "10")
	w := httptest.NewRecorder()
	h.RemovePlaylistSong(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRemovePlaylistSong_DBError(t *testing.T) {
	pl := &model.Playlist{ID: 1, UserID: 1}
	entries := []*model.PlaylistEntry{
		{ID: 1, PlaylistID: 1, SongID: 10, Position: 0},
	}
	h := newPlaylistHandler(&configPlaylistStore{getResult: pl, entriesResult: entries, removeErr: errors.New("db error")}, nil)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/playlists/1/songs/10", nil)
	r = withAuthUser(r, 1, "alice", false)
	r = withChiParams(r, "id", "1", "songId", "10")
	w := httptest.NewRecorder()
	h.RemovePlaylistSong(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
