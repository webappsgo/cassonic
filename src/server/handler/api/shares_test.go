package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// configShareStore is a configurable stub for ShareStore.
type configShareStore struct {
	listResult []*model.Share
	listErr    error

	createResult int64
	createErr    error

	getResult *model.Share
	getErr    error

	getByTokenResult *model.Share
	getByTokenErr    error

	updateErr    error
	deleteErr    error
	incrementErr error
}

func (s *configShareStore) CreateShare(ctx context.Context, sh *model.Share) (int64, error) {
	return s.createResult, s.createErr
}

func (s *configShareStore) GetShare(ctx context.Context, id int64) (*model.Share, error) {
	return s.getResult, s.getErr
}

func (s *configShareStore) GetShareByToken(ctx context.Context, token string) (*model.Share, error) {
	return s.getByTokenResult, s.getByTokenErr
}

func (s *configShareStore) ListSharesByUser(ctx context.Context, userID int64) ([]*model.Share, error) {
	return s.listResult, s.listErr
}

func (s *configShareStore) UpdateShare(ctx context.Context, sh *model.Share) error {
	return s.updateErr
}

func (s *configShareStore) DeleteShare(ctx context.Context, id int64) error {
	return s.deleteErr
}

func (s *configShareStore) IncrementViewCount(ctx context.Context, id int64) error {
	return s.incrementErr
}

// newShareHandler builds a Handler backed by the given ShareStore.
func newShareHandler(ss store.ShareStore) *Handler {
	return newHealthHandler(&store.DB{
		Music:  &stubMusicStore{},
		Users:  &stubUserStoreForHealth{},
		Shares: ss,
	})
}

// ------------------------------------------------------------------
// ListShares
// ------------------------------------------------------------------

func TestListShares_Success(t *testing.T) {
	shares := []*model.Share{
		{ID: 1, UserID: 1, Token: "abc", ItemType: "song", ItemID: 10},
		{ID: 2, UserID: 1, Token: "def", ItemType: "album", ItemID: 5},
	}
	h := newShareHandler(&configShareStore{listResult: shares})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/shares", nil)
	r = withAuth(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.ListShares(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	_, data := decodeEnvelope(t, w.Body)
	if total, _ := data["total"].(float64); total != 2 {
		t.Errorf("expected total 2, got %v", total)
	}
}

func TestListShares_NoAuth(t *testing.T) {
	h := newShareHandler(&configShareStore{})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/shares", nil)
	w := httptest.NewRecorder()
	h.ListShares(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestListShares_DBError(t *testing.T) {
	h := newShareHandler(&configShareStore{listErr: errors.New("db down")})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/shares", nil)
	r = withAuth(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.ListShares(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestListShares_EmptyList(t *testing.T) {
	h := newShareHandler(&configShareStore{listResult: nil})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/shares", nil)
	r = withAuth(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.ListShares(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ------------------------------------------------------------------
// CreateShare
// ------------------------------------------------------------------

func TestCreateShare_Success(t *testing.T) {
	h := newShareHandler(&configShareStore{createResult: 7})
	body := jsonBody(t, map[string]any{"item_type": "song", "item_id": 10})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/shares", body)
	r = withAuth(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreateShare(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestCreateShare_WithPassword(t *testing.T) {
	h := newShareHandler(&configShareStore{createResult: 8})
	body := jsonBody(t, map[string]any{
		"item_type": "album",
		"item_id":   5,
		"password":  "s3cret",
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/shares", body)
	r = withAuth(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreateShare(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestCreateShare_WithExpiresAt(t *testing.T) {
	h := newShareHandler(&configShareStore{createResult: 9})
	expires := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	body := jsonBody(t, map[string]any{
		"item_type":  "song",
		"item_id":    1,
		"expires_at": expires,
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/shares", body)
	r = withAuth(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreateShare(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestCreateShare_NoAuth(t *testing.T) {
	h := newShareHandler(&configShareStore{})
	body := jsonBody(t, map[string]any{"item_type": "song", "item_id": 1})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/shares", body)
	w := httptest.NewRecorder()
	h.CreateShare(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreateShare_BadJSON(t *testing.T) {
	h := newShareHandler(&configShareStore{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/shares", strings.NewReader("{bad"))
	r = withAuth(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreateShare(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateShare_MissingItemType(t *testing.T) {
	h := newShareHandler(&configShareStore{})
	body := jsonBody(t, map[string]any{"item_id": 1})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/shares", body)
	r = withAuth(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreateShare(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateShare_ZeroItemID(t *testing.T) {
	h := newShareHandler(&configShareStore{})
	body := jsonBody(t, map[string]any{"item_type": "song", "item_id": 0})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/shares", body)
	r = withAuth(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreateShare(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateShare_InvalidExpiresAt(t *testing.T) {
	h := newShareHandler(&configShareStore{})
	body := jsonBody(t, map[string]any{
		"item_type":  "song",
		"item_id":    1,
		"expires_at": "not-a-date",
	})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/shares", body)
	r = withAuth(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreateShare(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateShare_DBError(t *testing.T) {
	h := newShareHandler(&configShareStore{createErr: errors.New("db error")})
	body := jsonBody(t, map[string]any{"item_type": "song", "item_id": 1})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/shares", body)
	r = withAuth(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreateShare(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ------------------------------------------------------------------
// GetShare (public — no auth needed)
// ------------------------------------------------------------------

func TestGetShare_Success(t *testing.T) {
	share := &model.Share{
		ID: 1, UserID: 1, Token: "tok123",
		ItemType: "song", ItemID: 5,
	}
	h := newShareHandler(&configShareStore{getByTokenResult: share})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/shares/tok123", nil)
	r = withChiID(r, "token", "tok123")
	w := httptest.NewRecorder()
	h.GetShare(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetShare_NotFound(t *testing.T) {
	h := newShareHandler(&configShareStore{getByTokenResult: nil})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/shares/missing", nil)
	r = withChiID(r, "token", "missing")
	w := httptest.NewRecorder()
	h.GetShare(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetShare_Expired(t *testing.T) {
	share := &model.Share{
		ID: 1, Token: "tok",
		ItemType:  "song",
		ItemID:    5,
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	h := newShareHandler(&configShareStore{getByTokenResult: share})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/shares/tok", nil)
	r = withChiID(r, "token", "tok")
	w := httptest.NewRecorder()
	h.GetShare(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for expired share, got %d", w.Code)
	}
}

func TestGetShare_PasswordRequired(t *testing.T) {
	share := &model.Share{
		ID: 1, Token: "tok",
		ItemType:     "song",
		ItemID:       5,
		PasswordHash: "somehash",
	}
	h := newShareHandler(&configShareStore{getByTokenResult: share})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/shares/tok", nil)
	r = withChiID(r, "token", "tok")
	w := httptest.NewRecorder()
	h.GetShare(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when password required but not given, got %d", w.Code)
	}
}

func TestGetShare_IncorrectPassword(t *testing.T) {
	pw := hashSharePassword("correct")
	share := &model.Share{
		ID: 1, Token: "tok",
		ItemType:     "song",
		ItemID:       5,
		PasswordHash: pw,
	}
	h := newShareHandler(&configShareStore{getByTokenResult: share})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/shares/tok?password=wrong", nil)
	r = withChiID(r, "token", "tok")
	w := httptest.NewRecorder()
	h.GetShare(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d", w.Code)
	}
}

func TestGetShare_CorrectPassword(t *testing.T) {
	pw := hashSharePassword("correct")
	share := &model.Share{
		ID: 1, Token: "tok",
		ItemType:     "song",
		ItemID:       5,
		PasswordHash: pw,
	}
	h := newShareHandler(&configShareStore{getByTokenResult: share})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/shares/tok?password=correct", nil)
	r = withChiID(r, "token", "tok")
	w := httptest.NewRecorder()
	h.GetShare(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with correct password, got %d", w.Code)
	}
}

func TestGetShare_IncrementsViewCount(t *testing.T) {
	share := &model.Share{
		ID: 1, Token: "tok",
		ItemType:  "song",
		ItemID:    5,
		ViewCount: 4,
	}
	h := newShareHandler(&configShareStore{getByTokenResult: share})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/shares/tok", nil)
	r = withChiID(r, "token", "tok")
	w := httptest.NewRecorder()
	h.GetShare(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	_, data := decodeEnvelope(t, w.Body)
	if vc, _ := data["view_count"].(float64); vc != 5 {
		t.Errorf("expected view_count 5, got %v", vc)
	}
}

func TestGetShare_WithFutureExpiry(t *testing.T) {
	share := &model.Share{
		ID: 1, Token: "tok",
		ItemType:  "song",
		ItemID:    5,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	h := newShareHandler(&configShareStore{getByTokenResult: share})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/shares/tok", nil)
	r = withChiID(r, "token", "tok")
	w := httptest.NewRecorder()
	h.GetShare(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for non-expired share, got %d", w.Code)
	}
}

// ------------------------------------------------------------------
// UpdateShare
// ------------------------------------------------------------------

func TestUpdateShare_Success(t *testing.T) {
	share := &model.Share{ID: 1, UserID: 1, Token: "tok"}
	h := newShareHandler(&configShareStore{getResult: share})
	body := jsonBody(t, map[string]any{"description": "Updated"})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/shares/1", body)
	r = withAuth(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.UpdateShare(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestUpdateShare_NoAuth(t *testing.T) {
	h := newShareHandler(&configShareStore{})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/shares/1", jsonBody(t, map[string]any{}))
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.UpdateShare(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestUpdateShare_BadID(t *testing.T) {
	h := newShareHandler(&configShareStore{})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/shares/bad", jsonBody(t, map[string]any{}))
	r = withAuth(r, 1, "alice", false)
	r = withChiID(r, "id", "bad")
	w := httptest.NewRecorder()
	h.UpdateShare(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateShare_NotFound(t *testing.T) {
	h := newShareHandler(&configShareStore{getResult: nil})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/shares/1", jsonBody(t, map[string]any{}))
	r = withAuth(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.UpdateShare(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateShare_Forbidden(t *testing.T) {
	share := &model.Share{ID: 1, UserID: 2}
	h := newShareHandler(&configShareStore{getResult: share})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/shares/1", jsonBody(t, map[string]any{"description": "x"}))
	r = withAuth(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.UpdateShare(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestUpdateShare_AdminOverride(t *testing.T) {
	share := &model.Share{ID: 1, UserID: 2}
	h := newShareHandler(&configShareStore{getResult: share})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/shares/1", jsonBody(t, map[string]any{"description": "admin edit"}))
	r = withAuth(r, 99, "admin", true)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.UpdateShare(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin, got %d", w.Code)
	}
}

func TestUpdateShare_InvalidExpiresAt(t *testing.T) {
	share := &model.Share{ID: 1, UserID: 1}
	h := newShareHandler(&configShareStore{getResult: share})
	body := jsonBody(t, map[string]any{"expires_at": "not-a-date"})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/shares/1", body)
	r = withAuth(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.UpdateShare(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateShare_ValidExpiresAt(t *testing.T) {
	share := &model.Share{ID: 1, UserID: 1}
	h := newShareHandler(&configShareStore{getResult: share})
	expires := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	body := jsonBody(t, map[string]any{"expires_at": expires})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/shares/1", body)
	r = withAuth(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.UpdateShare(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestUpdateShare_DBError(t *testing.T) {
	share := &model.Share{ID: 1, UserID: 1}
	h := newShareHandler(&configShareStore{getResult: share, updateErr: errors.New("db error")})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/shares/1", jsonBody(t, map[string]any{"description": "x"}))
	r = withAuth(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.UpdateShare(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ------------------------------------------------------------------
// DeleteShare
// ------------------------------------------------------------------

func TestDeleteShare_Success(t *testing.T) {
	share := &model.Share{ID: 1, UserID: 1}
	h := newShareHandler(&configShareStore{getResult: share})
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/1", nil)
	r = withAuth(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.DeleteShare(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDeleteShare_NoAuth(t *testing.T) {
	h := newShareHandler(&configShareStore{})
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/1", nil)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.DeleteShare(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestDeleteShare_BadID(t *testing.T) {
	h := newShareHandler(&configShareStore{})
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/bad", nil)
	r = withAuth(r, 1, "alice", false)
	r = withChiID(r, "id", "bad")
	w := httptest.NewRecorder()
	h.DeleteShare(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeleteShare_NotFound(t *testing.T) {
	h := newShareHandler(&configShareStore{getResult: nil})
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/1", nil)
	r = withAuth(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.DeleteShare(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteShare_Forbidden(t *testing.T) {
	share := &model.Share{ID: 1, UserID: 2}
	h := newShareHandler(&configShareStore{getResult: share})
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/1", nil)
	r = withAuth(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.DeleteShare(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestDeleteShare_AdminCanDelete(t *testing.T) {
	share := &model.Share{ID: 1, UserID: 2}
	h := newShareHandler(&configShareStore{getResult: share})
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/1", nil)
	r = withAuth(r, 99, "admin", true)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.DeleteShare(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin delete, got %d", w.Code)
	}
}

func TestDeleteShare_DBError(t *testing.T) {
	share := &model.Share{ID: 1, UserID: 1}
	h := newShareHandler(&configShareStore{getResult: share, deleteErr: errors.New("db error")})
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/1", nil)
	r = withAuth(r, 1, "alice", false)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.DeleteShare(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
