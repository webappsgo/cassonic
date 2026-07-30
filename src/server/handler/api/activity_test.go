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
	"time"

	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// stubActivityStore satisfies store.ActivityStore; every method returns an error by default.
type stubActivityStore struct{}

func (s *stubActivityStore) Star(ctx context.Context, userID int64, itemType string, itemID int64) error {
	return errors.New("not implemented")
}

func (s *stubActivityStore) Unstar(ctx context.Context, userID int64, itemType string, itemID int64) error {
	return errors.New("not implemented")
}

func (s *stubActivityStore) GetStarred(ctx context.Context, userID int64) (*store.StarredItems, error) {
	return nil, errors.New("not implemented")
}

func (s *stubActivityStore) IsStarred(ctx context.Context, userID int64, itemType string, itemID int64) (bool, error) {
	return false, errors.New("not implemented")
}

func (s *stubActivityStore) SetRating(ctx context.Context, userID int64, itemType string, itemID int64, rating int) error {
	return errors.New("not implemented")
}

func (s *stubActivityStore) GetRating(ctx context.Context, userID int64, itemType string, itemID int64) (int, error) {
	return 0, errors.New("not implemented")
}

func (s *stubActivityStore) RecordPlay(ctx context.Context, h *model.PlayHistory) error {
	return errors.New("not implemented")
}

func (s *stubActivityStore) GetPlayHistory(ctx context.Context, userID int64, limit int) ([]*model.PlayHistory, error) {
	return nil, errors.New("not implemented")
}

func (s *stubActivityStore) SetBookmark(ctx context.Context, b *model.Bookmark) error {
	return errors.New("not implemented")
}

func (s *stubActivityStore) GetBookmarks(ctx context.Context, userID int64) ([]*model.Bookmark, error) {
	return nil, errors.New("not implemented")
}

func (s *stubActivityStore) DeleteBookmark(ctx context.Context, userID int64, itemType string, itemID int64) error {
	return errors.New("not implemented")
}

func (s *stubActivityStore) SavePlayQueue(ctx context.Context, pq *model.PlayQueue, entries []*model.PlayQueueEntry) error {
	return errors.New("not implemented")
}

func (s *stubActivityStore) GetPlayQueue(ctx context.Context, userID int64) (*model.PlayQueue, []*model.PlayQueueEntry, error) {
	return nil, nil, errors.New("not implemented")
}

// configActivityStore embeds stubActivityStore with per-test overrides.
type configActivityStore struct {
	*stubActivityStore

	starErr       error
	unstarErr     error
	setRatingErr  error
	recordPlayErr error
	setBookmarkErr error
	getBookmarksResult []*model.Bookmark
	getBookmarksErr    error
	deleteBookmarkErr  error
	savePlayQueueErr   error
	getPlayQueueResult *model.PlayQueue
	getPlayQueueEntries []*model.PlayQueueEntry
	getPlayQueueErr    error
}

func (s *configActivityStore) Star(ctx context.Context, userID int64, itemType string, itemID int64) error {
	return s.starErr
}

func (s *configActivityStore) Unstar(ctx context.Context, userID int64, itemType string, itemID int64) error {
	return s.unstarErr
}

func (s *configActivityStore) SetRating(ctx context.Context, userID int64, itemType string, itemID int64, rating int) error {
	return s.setRatingErr
}

func (s *configActivityStore) RecordPlay(ctx context.Context, h *model.PlayHistory) error {
	return s.recordPlayErr
}

func (s *configActivityStore) SetBookmark(ctx context.Context, b *model.Bookmark) error {
	return s.setBookmarkErr
}

func (s *configActivityStore) GetBookmarks(ctx context.Context, userID int64) ([]*model.Bookmark, error) {
	return s.getBookmarksResult, s.getBookmarksErr
}

func (s *configActivityStore) DeleteBookmark(ctx context.Context, userID int64, itemType string, itemID int64) error {
	return s.deleteBookmarkErr
}

func (s *configActivityStore) SavePlayQueue(ctx context.Context, pq *model.PlayQueue, entries []*model.PlayQueueEntry) error {
	return s.savePlayQueueErr
}

func (s *configActivityStore) GetPlayQueue(ctx context.Context, userID int64) (*model.PlayQueue, []*model.PlayQueueEntry, error) {
	return s.getPlayQueueResult, s.getPlayQueueEntries, s.getPlayQueueErr
}

// newActivityHandler creates a Handler wired with the given activity store and no other services.
func newActivityHandler(act *configActivityStore) *Handler {
	return newHealthHandler(&store.DB{
		Music:    &stubMusicStore{},
		Users:    &stubUserStoreForHealth{},
		Activity: act,
	})
}

// newActivityHandlerWithMusic creates a Handler wired with both activity and music stores.
func newActivityHandlerWithMusic(act *configActivityStore, ms store.MusicStore) *Handler {
	return newHealthHandler(&store.DB{
		Music:    ms,
		Users:    &stubUserStoreForHealth{},
		Activity: act,
	})
}

// withAuth returns a copy of r with an AuthUser injected into its context.
func withAuth(r *http.Request, userID int64, username string, isAdmin bool) *http.Request {
	u := &mw.AuthUser{ID: userID, Username: username, IsAdmin: isAdmin}
	return r.WithContext(mw.WithUser(r.Context(), u))
}

// decodeEnvelope unmarshals the standard {"ok":true,"data":...} response wrapper.
func decodeEnvelope(t *testing.T, body *bytes.Buffer) (bool, map[string]any) {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal(body.Bytes(), &env); err != nil {
		t.Fatalf("response is not valid JSON: %v — body: %s", err, body.String())
	}
	ok, _ := env["ok"].(bool)
	data, _ := env["data"].(map[string]any)
	return ok, data
}

// ---------- NowPlayingTracker unit tests ----------

func TestNowPlayingTrackerRegisterAndForUser(t *testing.T) {
	tr := NewNowPlayingTracker()
	info := &NowPlayingInfo{UserID: 1, Username: "alice", StartedAt: time.Now()}
	tr.Register(1, info)

	got := tr.ForUser(1)
	if got == nil {
		t.Fatal("ForUser returned nil after Register")
	}
	if got.Username != "alice" {
		t.Errorf("Username: got %q, want alice", got.Username)
	}
}

func TestNowPlayingTrackerForUserNone(t *testing.T) {
	tr := NewNowPlayingTracker()
	if got := tr.ForUser(99); got != nil {
		t.Errorf("ForUser on empty tracker: got %v, want nil", got)
	}
}

func TestNowPlayingTrackerAll(t *testing.T) {
	tr := NewNowPlayingTracker()
	tr.Register(1, &NowPlayingInfo{UserID: 1, Username: "alice", StartedAt: time.Now()})
	tr.Register(2, &NowPlayingInfo{UserID: 2, Username: "bob", StartedAt: time.Now()})

	all := tr.All()
	if len(all) != 2 {
		t.Errorf("All: got %d entries, want 2", len(all))
	}
}

func TestNowPlayingTrackerUnregister(t *testing.T) {
	tr := NewNowPlayingTracker()
	tr.Register(1, &NowPlayingInfo{UserID: 1, Username: "alice", StartedAt: time.Now()})
	tr.Unregister(1)

	if got := tr.ForUser(1); got != nil {
		t.Errorf("ForUser after Unregister: got %v, want nil", got)
	}
}

func TestNowPlayingTrackerReturnsCopy(t *testing.T) {
	tr := NewNowPlayingTracker()
	original := &NowPlayingInfo{UserID: 1, Username: "alice", StartedAt: time.Now()}
	tr.Register(1, original)

	got := tr.ForUser(1)
	got.Username = "mutated"
	if tr.ForUser(1).Username != "alice" {
		t.Error("ForUser returned a reference, not a copy; mutation affected tracker state")
	}
}

// ---------- parsePagination tests ----------

func TestParsePaginationDefaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	limit, offset := parsePagination(r)
	if limit != 50 {
		t.Errorf("default limit: got %d, want 50", limit)
	}
	if offset != 0 {
		t.Errorf("default offset: got %d, want 0", offset)
	}
}

func TestParsePaginationCustomValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=20&offset=10", nil)
	limit, offset := parsePagination(r)
	if limit != 20 {
		t.Errorf("limit: got %d, want 20", limit)
	}
	if offset != 10 {
		t.Errorf("offset: got %d, want 10", offset)
	}
}

func TestParsePaginationMaxCap(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=999", nil)
	limit, _ := parsePagination(r)
	if limit != 500 {
		t.Errorf("limit cap: got %d, want 500", limit)
	}
}

func TestParsePaginationInvalidIgnored(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=abc&offset=-1", nil)
	limit, offset := parsePagination(r)
	if limit != 50 {
		t.Errorf("invalid limit: got %d, want default 50", limit)
	}
	if offset != 0 {
		t.Errorf("negative offset: got %d, want 0", offset)
	}
}

// ---------- GetPlayQueue tests ----------

func TestGetPlayQueueNoAuth(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/play-queues", nil)
	rec := httptest.NewRecorder()
	h.GetPlayQueue(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestGetPlayQueueStoreError(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		getPlayQueueErr:   errors.New("db error"),
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/play-queues", nil)
	req = withAuth(req, 1, "alice", false)
	rec := httptest.NewRecorder()
	h.GetPlayQueue(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestGetPlayQueueEmpty(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore:   &stubActivityStore{},
		getPlayQueueResult:  nil,
		getPlayQueueEntries: nil,
		getPlayQueueErr:     nil,
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/play-queues", nil)
	req = withAuth(req, 1, "alice", false)
	rec := httptest.NewRecorder()
	h.GetPlayQueue(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ok, data := decodeEnvelope(t, rec.Body)
	if !ok {
		t.Error("ok field should be true")
	}
	songs, _ := data["songs"].([]any)
	if songs == nil {
		t.Error("songs field should be empty array, not nil")
	}
}

func TestGetPlayQueueWithEntries(t *testing.T) {
	queue := &model.PlayQueue{
		ID:        1,
		UserID:    1,
		Current:   42,
		Position:  5000,
		ChangedBy: "web",
		UpdatedAt: time.Now(),
	}
	entries := []*model.PlayQueueEntry{
		{ID: 1, PlayQueueID: 1, SongID: 42, Position: 0},
	}
	song := &model.Song{ID: 42, Title: "Test Song"}
	ms := &configMusicStore{
		stubMusicStore: &stubMusicStore{},
		getSongResult:  song,
	}
	act := &configActivityStore{
		stubActivityStore:   &stubActivityStore{},
		getPlayQueueResult:  queue,
		getPlayQueueEntries: entries,
		getPlayQueueErr:     nil,
	}
	h := newActivityHandlerWithMusic(act, ms)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/play-queues", nil)
	req = withAuth(req, 1, "alice", false)
	rec := httptest.NewRecorder()
	h.GetPlayQueue(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ok, data := decodeEnvelope(t, rec.Body)
	if !ok {
		t.Error("ok should be true")
	}
	if cur, _ := data["current"].(float64); int(cur) != 42 {
		t.Errorf("current: got %v, want 42", data["current"])
	}
}

// ---------- SavePlayQueue tests ----------

func TestSavePlayQueueNoAuth(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	body := bytes.NewBufferString(`{"song_ids":[1,2],"current":1,"position":0}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/play-queues", body)
	rec := httptest.NewRecorder()
	h.SavePlayQueue(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestSavePlayQueueBadJSON(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/play-queues", strings.NewReader("not json"))
	req = withAuth(req, 1, "alice", false)
	rec := httptest.NewRecorder()
	h.SavePlayQueue(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestSavePlayQueueStoreError(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		savePlayQueueErr:  errors.New("db error"),
	}
	h := newActivityHandler(act)
	body := bytes.NewBufferString(`{"song_ids":[1,2],"current":1,"position":0}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/play-queues", body)
	req = withAuth(req, 1, "alice", false)
	rec := httptest.NewRecorder()
	h.SavePlayQueue(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestSavePlayQueueSuccess(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		savePlayQueueErr:  nil,
	}
	h := newActivityHandler(act)
	body := bytes.NewBufferString(`{"song_ids":[1,2,3],"current":1,"position":5000}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/play-queues", body)
	req = withAuth(req, 1, "alice", false)
	req.Header.Set("X-Client-Name", "testclient")
	rec := httptest.NewRecorder()
	h.SavePlayQueue(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

// ---------- ListBookmarks tests ----------

func TestListBookmarksNoAuth(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bookmarks", nil)
	rec := httptest.NewRecorder()
	h.ListBookmarks(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestListBookmarksStoreError(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		getBookmarksErr:   errors.New("db error"),
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bookmarks", nil)
	req = withAuth(req, 1, "alice", false)
	rec := httptest.NewRecorder()
	h.ListBookmarks(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestListBookmarksEmpty(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore:  &stubActivityStore{},
		getBookmarksResult: []*model.Bookmark{},
		getBookmarksErr:    nil,
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bookmarks", nil)
	req = withAuth(req, 1, "alice", false)
	rec := httptest.NewRecorder()
	h.ListBookmarks(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ok, data := decodeEnvelope(t, rec.Body)
	if !ok {
		t.Error("ok should be true")
	}
	if total, _ := data["total"].(float64); total != 0 {
		t.Errorf("total: got %v, want 0", data["total"])
	}
}

func TestListBookmarksSongDetails(t *testing.T) {
	bookmarks := []*model.Bookmark{
		{ID: 1, UserID: 1, ItemType: "song", ItemID: 42, Position: 1234, UpdatedAt: time.Now()},
	}
	song := &model.Song{ID: 42, Title: "Track A"}
	ms := &configMusicStore{
		stubMusicStore: &stubMusicStore{},
		getSongResult:  song,
	}
	act := &configActivityStore{
		stubActivityStore:  &stubActivityStore{},
		getBookmarksResult: bookmarks,
		getBookmarksErr:    nil,
	}
	h := newActivityHandlerWithMusic(act, ms)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bookmarks", nil)
	req = withAuth(req, 1, "alice", false)
	rec := httptest.NewRecorder()
	h.ListBookmarks(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ok, data := decodeEnvelope(t, rec.Body)
	if !ok {
		t.Error("ok should be true")
	}
	bms, _ := data["bookmarks"].([]any)
	if len(bms) != 1 {
		t.Errorf("bookmarks count: got %d, want 1", len(bms))
	}
}

// ---------- CreateBookmark tests ----------

func TestCreateBookmarkNoAuth(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	body := bytes.NewBufferString(`{"item_type":"song","item_id":1,"position":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bookmarks", body)
	rec := httptest.NewRecorder()
	h.CreateBookmark(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestCreateBookmarkBadJSON(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bookmarks", strings.NewReader("bad"))
	req = withAuth(req, 1, "alice", false)
	rec := httptest.NewRecorder()
	h.CreateBookmark(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestCreateBookmarkMissingFields(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	body := bytes.NewBufferString(`{"item_type":"","item_id":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bookmarks", body)
	req = withAuth(req, 1, "alice", false)
	rec := httptest.NewRecorder()
	h.CreateBookmark(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestCreateBookmarkStoreError(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore:  &stubActivityStore{},
		setBookmarkErr:     errors.New("db error"),
	}
	h := newActivityHandler(act)
	body := bytes.NewBufferString(`{"item_type":"song","item_id":5,"position":1000}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bookmarks", body)
	req = withAuth(req, 1, "alice", false)
	rec := httptest.NewRecorder()
	h.CreateBookmark(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestCreateBookmarkSuccess(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		setBookmarkErr:    nil,
	}
	h := newActivityHandler(act)
	body := bytes.NewBufferString(`{"item_type":"song","item_id":5,"position":1000,"comment":"resume here"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bookmarks", body)
	req = withAuth(req, 1, "alice", false)
	rec := httptest.NewRecorder()
	h.CreateBookmark(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201", rec.Code)
	}
}

// ---------- UpdateBookmark tests ----------

func TestUpdateBookmarkNoAuth(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/bookmarks/1", strings.NewReader(`{}`))
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.UpdateBookmark(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestUpdateBookmarkInvalidID(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/bookmarks/abc", strings.NewReader(`{}`))
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "abc")
	rec := httptest.NewRecorder()
	h.UpdateBookmark(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestUpdateBookmarkGetError(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		getBookmarksErr:   errors.New("db error"),
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/bookmarks/1", strings.NewReader(`{}`))
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.UpdateBookmark(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestUpdateBookmarkNotFound(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore:  &stubActivityStore{},
		getBookmarksResult: []*model.Bookmark{},
		getBookmarksErr:    nil,
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/bookmarks/99", strings.NewReader(`{}`))
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "99")
	rec := httptest.NewRecorder()
	h.UpdateBookmark(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestUpdateBookmarkBadJSON(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore:  &stubActivityStore{},
		getBookmarksResult: []*model.Bookmark{{ID: 1, UserID: 1, ItemType: "song", ItemID: 5}},
		getBookmarksErr:    nil,
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/bookmarks/1", strings.NewReader("bad"))
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.UpdateBookmark(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestUpdateBookmarkStoreError(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore:  &stubActivityStore{},
		getBookmarksResult: []*model.Bookmark{{ID: 1, UserID: 1, ItemType: "song", ItemID: 5}},
		getBookmarksErr:    nil,
		setBookmarkErr:     errors.New("db error"),
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/bookmarks/1", bytes.NewBufferString(`{"position":2000}`))
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.UpdateBookmark(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestUpdateBookmarkSuccess(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore:  &stubActivityStore{},
		getBookmarksResult: []*model.Bookmark{{ID: 1, UserID: 1, ItemType: "song", ItemID: 5}},
		getBookmarksErr:    nil,
		setBookmarkErr:     nil,
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/bookmarks/1", bytes.NewBufferString(`{"position":2000,"comment":"updated"}`))
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.UpdateBookmark(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

// ---------- DeleteBookmark tests ----------

func TestDeleteBookmarkNoAuth(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bookmarks/1", nil)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.DeleteBookmark(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestDeleteBookmarkInvalidID(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bookmarks/bad", nil)
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "bad")
	rec := httptest.NewRecorder()
	h.DeleteBookmark(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestDeleteBookmarkGetError(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		getBookmarksErr:   errors.New("db error"),
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bookmarks/1", nil)
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.DeleteBookmark(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestDeleteBookmarkNotFound(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore:  &stubActivityStore{},
		getBookmarksResult: []*model.Bookmark{},
		getBookmarksErr:    nil,
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bookmarks/99", nil)
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "99")
	rec := httptest.NewRecorder()
	h.DeleteBookmark(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestDeleteBookmarkStoreError(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore:  &stubActivityStore{},
		getBookmarksResult: []*model.Bookmark{{ID: 1, UserID: 1, ItemType: "song", ItemID: 5}},
		getBookmarksErr:    nil,
		deleteBookmarkErr:  errors.New("db error"),
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bookmarks/1", nil)
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.DeleteBookmark(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestDeleteBookmarkSuccess(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore:  &stubActivityStore{},
		getBookmarksResult: []*model.Bookmark{{ID: 1, UserID: 1, ItemType: "song", ItemID: 5}},
		getBookmarksErr:    nil,
		deleteBookmarkErr:  nil,
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bookmarks/1", nil)
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.DeleteBookmark(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

// ---------- NowPlaying tests ----------

func TestNowPlayingNoAuth(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/now-playing", nil)
	rec := httptest.NewRecorder()
	h.NowPlaying(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestNowPlayingUserSeesOwn(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	h.nowPlaying.Register(1, &NowPlayingInfo{UserID: 1, Username: "alice", StartedAt: time.Now()})
	h.nowPlaying.Register(2, &NowPlayingInfo{UserID: 2, Username: "bob", StartedAt: time.Now()})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/now-playing", nil)
	req = withAuth(req, 1, "alice", false)
	rec := httptest.NewRecorder()
	h.NowPlaying(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ok, data := decodeEnvelope(t, rec.Body)
	if !ok {
		t.Error("ok should be true")
	}
	streams, _ := data["now_playing"].([]any)
	if len(streams) != 1 {
		t.Errorf("non-admin should see only own stream; got %d streams", len(streams))
	}
}

func TestNowPlayingAdminSeesAll(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	h.nowPlaying.Register(1, &NowPlayingInfo{UserID: 1, Username: "alice", StartedAt: time.Now()})
	h.nowPlaying.Register(2, &NowPlayingInfo{UserID: 2, Username: "bob", StartedAt: time.Now()})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/now-playing", nil)
	req = withAuth(req, 99, "admin", true)
	rec := httptest.NewRecorder()
	h.NowPlaying(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ok, data := decodeEnvelope(t, rec.Body)
	if !ok {
		t.Error("ok should be true")
	}
	streams, _ := data["now_playing"].([]any)
	if len(streams) != 2 {
		t.Errorf("admin should see all streams; got %d", len(streams))
	}
}

func TestNowPlayingNoStreams(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/now-playing", nil)
	req = withAuth(req, 1, "alice", false)
	rec := httptest.NewRecorder()
	h.NowPlaying(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ok, data := decodeEnvelope(t, rec.Body)
	if !ok {
		t.Error("ok should be true")
	}
	if total, _ := data["total"].(float64); total != 0 {
		t.Errorf("total: got %v, want 0", data["total"])
	}
}

// ---------- StarSong tests ----------

func TestStarSongNoAuth(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/songs/1/stars", nil)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.StarSong(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestStarSongInvalidID(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/songs/bad/stars", nil)
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "bad")
	rec := httptest.NewRecorder()
	h.StarSong(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestStarSongStoreError(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		starErr:           errors.New("db error"),
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/songs/1/stars", nil)
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.StarSong(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestStarSongSuccess(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		starErr:           nil,
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/songs/1/stars", nil)
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.StarSong(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

// ---------- UnstarSong tests ----------

func TestUnstarSongNoAuth(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/songs/1/stars", nil)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.UnstarSong(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestUnstarSongInvalidID(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/songs/bad/stars", nil)
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "bad")
	rec := httptest.NewRecorder()
	h.UnstarSong(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestUnstarSongStoreError(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		unstarErr:         errors.New("db error"),
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/songs/1/stars", nil)
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.UnstarSong(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestUnstarSongSuccess(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		unstarErr:         nil,
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/songs/1/stars", nil)
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.UnstarSong(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

// ---------- RateSong tests ----------

func TestRateSongNoAuth(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/songs/1/rating", bytes.NewBufferString(`{"rating":3}`))
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.RateSong(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestRateSongInvalidID(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/songs/bad/rating", bytes.NewBufferString(`{"rating":3}`))
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "bad")
	rec := httptest.NewRecorder()
	h.RateSong(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestRateSongBadJSON(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/songs/1/rating", strings.NewReader("bad"))
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.RateSong(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestRateSongOutOfRange(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	for _, rating := range []int{0, 6} {
		body, _ := json.Marshal(map[string]int{"rating": rating})
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/songs/1/rating", bytes.NewBuffer(body))
		req = withAuth(req, 1, "alice", false)
		req = withChiID(req, "id", "1")
		rec := httptest.NewRecorder()
		h.RateSong(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("rating %d: status got %d, want 400", rating, rec.Code)
		}
	}
}

func TestRateSongStoreError(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		setRatingErr:      errors.New("db error"),
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/songs/1/rating", bytes.NewBufferString(`{"rating":3}`))
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.RateSong(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestRateSongSuccess(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		setRatingErr:      nil,
	}
	h := newActivityHandler(act)
	for _, rating := range []int{1, 3, 5} {
		body, _ := json.Marshal(map[string]int{"rating": rating})
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/songs/1/rating", bytes.NewBuffer(body))
		req = withAuth(req, 1, "alice", false)
		req = withChiID(req, "id", "1")
		rec := httptest.NewRecorder()
		h.RateSong(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("rating %d: status got %d, want 200", rating, rec.Code)
		}
	}
}

// ---------- Scrobble tests ----------

func TestScrobbleNoAuth(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/songs/1/scrobbles", bytes.NewBufferString(`{}`))
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.Scrobble(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestScrobbleInvalidID(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/songs/bad/scrobbles", bytes.NewBufferString(`{}`))
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "bad")
	rec := httptest.NewRecorder()
	h.Scrobble(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestScrobbleBadJSON(t *testing.T) {
	h := newActivityHandler(&configActivityStore{stubActivityStore: &stubActivityStore{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/songs/1/scrobbles", strings.NewReader("bad"))
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.Scrobble(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestScrobbleRecordError(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		recordPlayErr:     errors.New("db error"),
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/songs/1/scrobbles", bytes.NewBufferString(`{"submission":false}`))
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.Scrobble(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestScrobbleSuccessNowPlaying(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		recordPlayErr:     nil,
	}
	h := newActivityHandler(act)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/songs/1/scrobbles", bytes.NewBufferString(`{"submission":false}`))
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	rec := httptest.NewRecorder()
	h.Scrobble(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestScrobbleSuccessSubmission(t *testing.T) {
	act := &configActivityStore{
		stubActivityStore: &stubActivityStore{},
		recordPlayErr:     nil,
	}
	ms := &configMusicStore{stubMusicStore: &stubMusicStore{}}
	h := newActivityHandlerWithMusic(act, ms)
	body := bytes.NewBufferString(`{"submission":true,"timestamp":1700000000}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/songs/1/scrobbles", body)
	req = withAuth(req, 1, "alice", false)
	req = withChiID(req, "id", "1")
	req.Header.Set("X-Client-Name", "testapp")
	rec := httptest.NewRecorder()
	h.Scrobble(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}
