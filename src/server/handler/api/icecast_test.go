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

// stubIcecastStore satisfies store.IcecastStore; all methods return errors by default.
type stubIcecastStore struct{}

func (s *stubIcecastStore) CreateServer(ctx context.Context, sv *model.IcecastServer) (int64, error) {
	return 0, errors.New("not implemented")
}
func (s *stubIcecastStore) GetServer(ctx context.Context, id int64) (*model.IcecastServer, error) {
	return nil, errors.New("not implemented")
}
func (s *stubIcecastStore) ListServers(ctx context.Context) ([]*model.IcecastServer, error) {
	return nil, errors.New("not implemented")
}
func (s *stubIcecastStore) UpdateServer(ctx context.Context, sv *model.IcecastServer) error {
	return errors.New("not implemented")
}
func (s *stubIcecastStore) DeleteServer(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}
func (s *stubIcecastStore) CreateMount(ctx context.Context, m *model.IcecastMount) (int64, error) {
	return 0, errors.New("not implemented")
}
func (s *stubIcecastStore) GetMount(ctx context.Context, id int64) (*model.IcecastMount, error) {
	return nil, errors.New("not implemented")
}
func (s *stubIcecastStore) ListMounts(ctx context.Context) ([]*model.IcecastMount, error) {
	return nil, errors.New("not implemented")
}
func (s *stubIcecastStore) ListMountsByServer(ctx context.Context, serverID int64) ([]*model.IcecastMount, error) {
	return nil, errors.New("not implemented")
}
func (s *stubIcecastStore) UpdateMount(ctx context.Context, m *model.IcecastMount) error {
	return errors.New("not implemented")
}
func (s *stubIcecastStore) UpdateMountStatus(ctx context.Context, id int64, status model.MountStatus, currentSong, lastErr string) error {
	return errors.New("not implemented")
}
func (s *stubIcecastStore) DeleteMount(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

// configIcecastStore embeds stubIcecastStore with configurable return values.
type configIcecastStore struct {
	*stubIcecastStore

	listServersResult []*model.IcecastServer
	listServersErr    error
	createServerID    int64
	createServerErr   error
	getServerResult   *model.IcecastServer
	getServerErr      error
	updateServerErr   error
	deleteServerErr   error

	listMountsResult []*model.IcecastMount
	listMountsErr    error
	createMountID    int64
	createMountErr   error
	getMountResult   *model.IcecastMount
	getMountErr      error
	updateMountErr   error
	deleteMountErr   error
}

func (s *configIcecastStore) ListServers(ctx context.Context) ([]*model.IcecastServer, error) {
	return s.listServersResult, s.listServersErr
}
func (s *configIcecastStore) CreateServer(ctx context.Context, sv *model.IcecastServer) (int64, error) {
	return s.createServerID, s.createServerErr
}
func (s *configIcecastStore) GetServer(ctx context.Context, id int64) (*model.IcecastServer, error) {
	return s.getServerResult, s.getServerErr
}
func (s *configIcecastStore) UpdateServer(ctx context.Context, sv *model.IcecastServer) error {
	return s.updateServerErr
}
func (s *configIcecastStore) DeleteServer(ctx context.Context, id int64) error {
	return s.deleteServerErr
}
func (s *configIcecastStore) ListMounts(ctx context.Context) ([]*model.IcecastMount, error) {
	return s.listMountsResult, s.listMountsErr
}
func (s *configIcecastStore) CreateMount(ctx context.Context, m *model.IcecastMount) (int64, error) {
	return s.createMountID, s.createMountErr
}
func (s *configIcecastStore) GetMount(ctx context.Context, id int64) (*model.IcecastMount, error) {
	return s.getMountResult, s.getMountErr
}
func (s *configIcecastStore) UpdateMount(ctx context.Context, m *model.IcecastMount) error {
	return s.updateMountErr
}
func (s *configIcecastStore) DeleteMount(ctx context.Context, id int64) error {
	return s.deleteMountErr
}

// newIcecastHandler builds a Handler wired to cs for icecast tests.
func newIcecastHandler(cs *configIcecastStore) *Handler {
	return newHealthHandler(&store.DB{
		Music:   &stubMusicStore{},
		Users:   &stubUserStoreForHealth{},
		Icecast: cs,
	})
}

// adminUser returns a request context carrying an admin AuthUser.
func adminUser(r *http.Request) *http.Request {
	return r.WithContext(mw.WithUser(r.Context(), &mw.AuthUser{
		ID: 1, Username: "admin", IsAdmin: true,
	}))
}

func TestListIcecastServers_Success(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		listServersResult: []*model.IcecastServer{
			{ID: 1, Name: "Main", Host: "radio.example.com", Port: 8000},
		},
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/icecast/servers", nil))
	rec := httptest.NewRecorder()
	h.ListIcecastServers(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestListIcecastServers_StoreError(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		listServersErr:   errors.New("db error"),
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/icecast/servers", nil))
	rec := httptest.NewRecorder()
	h.ListIcecastServers(rec, r)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestCreateIcecastServer_Success(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		createServerID:   42,
	}
	h := newIcecastHandler(cs)
	body, _ := json.Marshal(map[string]any{
		"name": "Test", "host": "icecast.local", "port": 8000,
	})
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/icecast/servers", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.CreateIcecastServer(rec, r)
	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201", rec.Code)
	}
}

func TestCreateIcecastServer_MissingFields(t *testing.T) {
	cs := &configIcecastStore{stubIcecastStore: &stubIcecastStore{}}
	h := newIcecastHandler(cs)
	body, _ := json.Marshal(map[string]any{"port": 8000})
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/icecast/servers", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.CreateIcecastServer(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestCreateIcecastServer_InvalidJSON(t *testing.T) {
	cs := &configIcecastStore{stubIcecastStore: &stubIcecastStore{}}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/icecast/servers", bytes.NewReader([]byte("not-json"))))
	rec := httptest.NewRecorder()
	h.CreateIcecastServer(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestCreateIcecastServer_StoreError(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		createServerErr:  errors.New("db error"),
	}
	h := newIcecastHandler(cs)
	body, _ := json.Marshal(map[string]any{"name": "Test", "host": "h"})
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/icecast/servers", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.CreateIcecastServer(rec, r)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestGetIcecastServer_Success(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		getServerResult:  &model.IcecastServer{ID: 1, Name: "Main"},
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/icecast/servers/1", nil))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.GetIcecastServer(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestGetIcecastServer_NotFound(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		getServerResult:  nil,
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/icecast/servers/99", nil))
	r = withChiID(r, "id", "99")
	rec := httptest.NewRecorder()
	h.GetIcecastServer(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestGetIcecastServer_BadID(t *testing.T) {
	cs := &configIcecastStore{stubIcecastStore: &stubIcecastStore{}}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/icecast/servers/abc", nil))
	r = withChiID(r, "id", "abc")
	rec := httptest.NewRecorder()
	h.GetIcecastServer(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestUpdateIcecastServer_Success(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		getServerResult:  &model.IcecastServer{ID: 1, Name: "Old", Host: "old.host"},
	}
	h := newIcecastHandler(cs)
	body, _ := json.Marshal(map[string]any{"name": "New"})
	r := adminUser(httptest.NewRequest(http.MethodPut, "/api/v1/icecast/servers/1", bytes.NewReader(body)))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.UpdateIcecastServer(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestUpdateIcecastServer_NotFound(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		getServerResult:  nil,
	}
	h := newIcecastHandler(cs)
	body, _ := json.Marshal(map[string]any{"name": "New"})
	r := adminUser(httptest.NewRequest(http.MethodPut, "/api/v1/icecast/servers/1", bytes.NewReader(body)))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.UpdateIcecastServer(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestDeleteIcecastServer_Success(t *testing.T) {
	cs := &configIcecastStore{stubIcecastStore: &stubIcecastStore{}}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodDelete, "/api/v1/icecast/servers/1", nil))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.DeleteIcecastServer(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestDeleteIcecastServer_StoreError(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		deleteServerErr:  errors.New("db error"),
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodDelete, "/api/v1/icecast/servers/1", nil))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.DeleteIcecastServer(rec, r)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestListIcecastMounts_Success(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		listMountsResult: []*model.IcecastMount{
			{ID: 1, ServerID: 1, MountPath: "/stream"},
		},
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/icecast/mounts", nil))
	rec := httptest.NewRecorder()
	h.ListIcecastMounts(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestListIcecastMounts_StoreError(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		listMountsErr:    errors.New("db error"),
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/icecast/mounts", nil))
	rec := httptest.NewRecorder()
	h.ListIcecastMounts(rec, r)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestCreateIcecastMount_Success(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		createMountID:    10,
	}
	h := newIcecastHandler(cs)
	body, _ := json.Marshal(map[string]any{
		"server_id": 1, "mount_path": "/stream",
	})
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/icecast/mounts", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.CreateIcecastMount(rec, r)
	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201", rec.Code)
	}
}

func TestCreateIcecastMount_MissingFields(t *testing.T) {
	cs := &configIcecastStore{stubIcecastStore: &stubIcecastStore{}}
	h := newIcecastHandler(cs)
	body, _ := json.Marshal(map[string]any{"server_id": 0})
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/icecast/mounts", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.CreateIcecastMount(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestGetIcecastMount_Success(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		getMountResult:   &model.IcecastMount{ID: 1, ServerID: 1, MountPath: "/stream"},
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/icecast/mounts/1", nil))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.GetIcecastMount(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestGetIcecastMount_NotFound(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		getMountResult:   nil,
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/icecast/mounts/99", nil))
	r = withChiID(r, "id", "99")
	rec := httptest.NewRecorder()
	h.GetIcecastMount(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestUpdateIcecastMount_Success(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		getMountResult:   &model.IcecastMount{ID: 1, ServerID: 1, MountPath: "/stream"},
	}
	h := newIcecastHandler(cs)
	enabled := true
	body, _ := json.Marshal(map[string]any{"name": "New Name", "enabled": enabled})
	r := adminUser(httptest.NewRequest(http.MethodPut, "/api/v1/icecast/mounts/1", bytes.NewReader(body)))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.UpdateIcecastMount(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestDeleteIcecastMount_Success(t *testing.T) {
	cs := &configIcecastStore{stubIcecastStore: &stubIcecastStore{}}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodDelete, "/api/v1/icecast/mounts/1", nil))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.DeleteIcecastMount(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestStartIcecastMount_Success(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		getMountResult:   &model.IcecastMount{ID: 1, ServerID: 1, MountPath: "/stream"},
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/icecast/mounts/1/start", nil))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.StartIcecastMount(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestStartIcecastMount_NotFound(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		getMountResult:   nil,
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/icecast/mounts/99/start", nil))
	r = withChiID(r, "id", "99")
	rec := httptest.NewRecorder()
	h.StartIcecastMount(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestStopIcecastMount_Success(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		getMountResult:   &model.IcecastMount{ID: 1, ServerID: 1, Enabled: true},
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/icecast/mounts/1/stop", nil))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.StopIcecastMount(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestStopIcecastMount_NotFound(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		getMountResult:   nil,
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodPost, "/api/v1/icecast/mounts/99/stop", nil))
	r = withChiID(r, "id", "99")
	rec := httptest.NewRecorder()
	h.StopIcecastMount(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestIcecastMountStatus_Success(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		getMountResult: &model.IcecastMount{
			ID:          1,
			Status:      model.StatusConnected,
			CurrentSong: "Artist - Song",
			Enabled:     true,
		},
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/icecast/mounts/1/status", nil))
	r = withChiID(r, "id", "1")
	rec := httptest.NewRecorder()
	h.IcecastMountStatus(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestIcecastMountStatus_NotFound(t *testing.T) {
	cs := &configIcecastStore{
		stubIcecastStore: &stubIcecastStore{},
		getMountResult:   nil,
	}
	h := newIcecastHandler(cs)
	r := adminUser(httptest.NewRequest(http.MethodGet, "/api/v1/icecast/mounts/99/status", nil))
	r = withChiID(r, "id", "99")
	rec := httptest.NewRecorder()
	h.IcecastMountStatus(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}
