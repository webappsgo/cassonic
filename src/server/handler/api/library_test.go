package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// newLibStore returns a configMusicStore pre-configured for library tests.
func newLibStore() *configMusicStore {
	return &configMusicStore{stubMusicStore: &stubMusicStore{}}
}

// --- ListLibraries ---

func TestListLibrariesSuccess(t *testing.T) {
	ms := newLibStore()
	ms.listLibsResult = []*model.Library{
		{ID: 1, Name: "Music", Path: "/music", Enabled: true},
	}
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/libraries", nil)
	rec := httptest.NewRecorder()
	h.ListLibraries(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListLibraries success: got %d, want 200", rec.Code)
	}
}

func TestListLibrariesEmpty(t *testing.T) {
	ms := newLibStore()
	ms.listLibsResult = []*model.Library{}
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/libraries", nil)
	rec := httptest.NewRecorder()
	h.ListLibraries(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListLibraries empty: got %d, want 200", rec.Code)
	}
}

func TestListLibrariesError(t *testing.T) {
	ms := newLibStore()
	ms.listLibsErr = errors.New("db failure")
	h := newConfigHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/libraries", nil)
	rec := httptest.NewRecorder()
	h.ListLibraries(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ListLibraries error: got %d, want 500", rec.Code)
	}
}

// --- GetLibrary ---

func TestGetLibraryBadID(t *testing.T) {
	h := newConfigHandler(newLibStore())
	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/v1/libraries/abc", nil), "id", "abc")
	rec := httptest.NewRecorder()
	h.GetLibrary(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("GetLibrary bad id: got %d, want 400", rec.Code)
	}
}

func TestGetLibraryNotFound(t *testing.T) {
	ms := newLibStore()
	ms.getLibResult = nil
	h := newConfigHandler(ms)

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/v1/libraries/99", nil), "id", "99")
	rec := httptest.NewRecorder()
	h.GetLibrary(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GetLibrary not found: got %d, want 404", rec.Code)
	}
}

func TestGetLibrarySuccess(t *testing.T) {
	ms := newLibStore()
	ms.getLibResult = &model.Library{ID: 1, Name: "Music", Path: "/music", Enabled: true}
	h := newConfigHandler(ms)

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/v1/libraries/1", nil), "id", "1")
	rec := httptest.NewRecorder()
	h.GetLibrary(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GetLibrary success: got %d, want 200", rec.Code)
	}
}

// --- DeleteLibrary ---

func TestDeleteLibraryBadID(t *testing.T) {
	h := newConfigHandler(newLibStore())
	req := withChiID(httptest.NewRequest(http.MethodDelete, "/api/v1/libraries/bad", nil), "id", "bad")
	rec := httptest.NewRecorder()
	h.DeleteLibrary(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("DeleteLibrary bad id: got %d, want 400", rec.Code)
	}
}

func TestDeleteLibraryError(t *testing.T) {
	ms := newLibStore()
	ms.deleteLibErr = errors.New("delete failed")
	h := newConfigHandler(ms)

	req := withChiID(httptest.NewRequest(http.MethodDelete, "/api/v1/libraries/1", nil), "id", "1")
	rec := httptest.NewRecorder()
	h.DeleteLibrary(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("DeleteLibrary error: got %d, want 500", rec.Code)
	}
}

func TestDeleteLibrarySuccess(t *testing.T) {
	ms := newLibStore()
	ms.deleteLibErr = nil
	h := newConfigHandler(ms)

	req := withChiID(httptest.NewRequest(http.MethodDelete, "/api/v1/libraries/1", nil), "id", "1")
	rec := httptest.NewRecorder()
	h.DeleteLibrary(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("DeleteLibrary success: got %d, want 200", rec.Code)
	}
}

// --- CreateLibrary ---

func TestCreateLibraryInvalidJSON(t *testing.T) {
	h := newConfigHandler(newLibStore())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/libraries", strings.NewReader("not-json"))
	rec := httptest.NewRecorder()
	h.CreateLibrary(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("CreateLibrary invalid JSON: got %d, want 400", rec.Code)
	}
}

func TestCreateLibraryMissingName(t *testing.T) {
	h := newConfigHandler(newLibStore())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/libraries",
		strings.NewReader(`{"path":"/music"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateLibrary(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("CreateLibrary missing name: got %d, want 400", rec.Code)
	}
}

func TestCreateLibraryMissingPath(t *testing.T) {
	h := newConfigHandler(newLibStore())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/libraries",
		strings.NewReader(`{"name":"Music"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateLibrary(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("CreateLibrary missing path: got %d, want 400", rec.Code)
	}
}

func TestCreateLibraryPathNotExists(t *testing.T) {
	h := newConfigHandler(newLibStore())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/libraries",
		strings.NewReader(`{"name":"Music","path":"/nonexistent/cassonic/test/path"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateLibrary(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("CreateLibrary path not exists: got %d, want 400", rec.Code)
	}
}

func TestCreateLibrarySuccess(t *testing.T) {
	ms := newLibStore()
	ms.createLibResult = 42
	h := newConfigHandler(ms)

	body := strings.NewReader(`{"name":"Music","path":"/tmp"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/libraries", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateLibrary(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("CreateLibrary success: got %d, want 201", rec.Code)
	}
}

func TestCreateLibraryStoreError(t *testing.T) {
	ms := newLibStore()
	ms.createLibErr = errors.New("duplicate")
	h := newConfigHandler(ms)

	body := strings.NewReader(`{"name":"Music","path":"/tmp"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/libraries", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateLibrary(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("CreateLibrary store error: got %d, want 409", rec.Code)
	}
}

// --- UpdateLibrary ---

func TestUpdateLibraryBadID(t *testing.T) {
	h := newConfigHandler(newLibStore())
	req := withChiID(httptest.NewRequest(http.MethodPut, "/api/v1/libraries/bad",
		strings.NewReader(`{}`)), "id", "bad")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.UpdateLibrary(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("UpdateLibrary bad id: got %d, want 400", rec.Code)
	}
}

func TestUpdateLibraryNotFound(t *testing.T) {
	ms := newLibStore()
	ms.getLibResult = nil
	h := newConfigHandler(ms)

	req := withChiID(httptest.NewRequest(http.MethodPut, "/api/v1/libraries/1",
		strings.NewReader(`{"name":"New"}`)), "id", "1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.UpdateLibrary(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("UpdateLibrary not found: got %d, want 404", rec.Code)
	}
}

func TestUpdateLibrarySuccess(t *testing.T) {
	ms := newLibStore()
	ms.getLibResult = &model.Library{ID: 1, Name: "Old", Path: "/old", Enabled: true}
	ms.updateLibErr = nil
	h := newConfigHandler(ms)

	body := strings.NewReader(`{"name":"New Name"}`)
	req := withChiID(httptest.NewRequest(http.MethodPut, "/api/v1/libraries/1", body), "id", "1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.UpdateLibrary(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("UpdateLibrary success: got %d, want 200", rec.Code)
	}
}

func TestUpdateLibraryStoreError(t *testing.T) {
	ms := newLibStore()
	ms.getLibResult = &model.Library{ID: 1, Name: "Old", Path: "/old", Enabled: true}
	ms.updateLibErr = errors.New("update failed")
	h := newConfigHandler(ms)

	body := strings.NewReader(`{"name":"New Name"}`)
	req := withChiID(httptest.NewRequest(http.MethodPut, "/api/v1/libraries/1", body), "id", "1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.UpdateLibrary(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("UpdateLibrary store error: got %d, want 500", rec.Code)
	}
}
