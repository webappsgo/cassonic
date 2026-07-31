package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
)

// sampleUser returns a minimal valid *model.User for test setup.
func sampleUser(id int64, username string, isAdmin bool) *model.User {
	return &model.User{
		ID:           id,
		Username:     username,
		Email:        username + "@example.com",
		DisplayName:  username,
		IsAdmin:      isAdmin,
		IsEnabled:    true,
		Language:     "en",
		Theme:        "dark",
		PasswordHash: mustHashPassword("testpass"),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
}

func TestListUsers_StoreError(t *testing.T) {
	us := &configUserStore{listUsersErr: errUserStore}
	h := newUserHandler(us)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	w := httptest.NewRecorder()
	h.ListUsers(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestListUsers_Empty(t *testing.T) {
	us := &configUserStore{listUsersResult: []*model.User{}}
	h := newUserHandler(us)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	w := httptest.NewRecorder()
	h.ListUsers(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Total != 0 {
		t.Errorf("expected 0 total, got %d", resp.Data.Total)
	}
}

func TestListUsers_Multiple(t *testing.T) {
	us := &configUserStore{
		listUsersResult: []*model.User{
			sampleUser(1, "alice", false),
			sampleUser(2, "bob", true),
		},
	}
	h := newUserHandler(us)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	w := httptest.NewRecorder()
	h.ListUsers(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Data struct {
			Total int              `json:"total"`
			Users []map[string]any `json:"users"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Total != 2 {
		t.Errorf("expected 2 total, got %d", resp.Data.Total)
	}
}

func TestCreateUser_InvalidJSON(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader("{bad}"))
	w := httptest.NewRecorder()
	h.CreateUser(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateUser_MissingUsernameOrPassword(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	body, _ := json.Marshal(map[string]string{"username": "alice"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateUser(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateUser_StoreError(t *testing.T) {
	us := &configUserStore{createUserErr: errUserStore}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "pass"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateUser(w, r)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestCreateUser_Success(t *testing.T) {
	us := &configUserStore{createUserID: 55}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"username": "newuser", "password": "newpass"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateUser(w, r)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateUser_DefaultsLanguageAndTheme(t *testing.T) {
	us := &configUserStore{createUserID: 1}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"username": "newuser", "password": "pass"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateUser(w, r)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	var resp struct {
		Data struct {
			Language string `json:"language"`
			Theme    string `json:"theme"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Language != "en" {
		t.Errorf("expected language=en, got %q", resp.Data.Language)
	}
	if resp.Data.Theme != "dark" {
		t.Errorf("expected theme=dark, got %q", resp.Data.Theme)
	}
}

func TestGetMe_NotAuthenticated(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	w := httptest.NewRecorder()
	h.GetMe(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetMe_UserNotFound(t *testing.T) {
	us := &configUserStore{getUserResult: nil}
	h := newUserHandler(us)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.GetMe(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetMe_Success(t *testing.T) {
	us := &configUserStore{getUserResult: sampleUser(1, "alice", false)}
	h := newUserHandler(us)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.GetMe(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Data struct {
			Username string `json:"username"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Username != "alice" {
		t.Errorf("expected username=alice, got %q", resp.Data.Username)
	}
}

func TestUpdateMe_NotAuthenticated(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/me", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	h.UpdateMe(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestUpdateMe_UserNotFound(t *testing.T) {
	us := &configUserStore{getUserResult: nil}
	h := newUserHandler(us)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/me", strings.NewReader("{}"))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.UpdateMe(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateMe_InvalidJSON(t *testing.T) {
	us := &configUserStore{getUserResult: sampleUser(1, "alice", false)}
	h := newUserHandler(us)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/me", strings.NewReader("{bad}"))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.UpdateMe(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateMe_UpdateError(t *testing.T) {
	us := &configUserStore{
		getUserResult: sampleUser(1, "alice", false),
		updateUserErr: errUserStore,
	}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"display_name": "Alice"})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/me", bytes.NewReader(body))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.UpdateMe(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestUpdateMe_Success(t *testing.T) {
	us := &configUserStore{getUserResult: sampleUser(1, "alice", false)}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"display_name": "Alice Updated", "language": "fr"})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/me", bytes.NewReader(body))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.UpdateMe(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Data struct {
			DisplayName string `json:"display_name"`
			Language    string `json:"language"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.DisplayName != "Alice Updated" {
		t.Errorf("expected display_name=Alice Updated, got %q", resp.Data.DisplayName)
	}
	if resp.Data.Language != "fr" {
		t.Errorf("expected language=fr, got %q", resp.Data.Language)
	}
}

func TestGetUser_NotAuthenticated(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.GetUser(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetUser_InvalidID(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/abc", nil)
	r = withChiID(r, "id", "abc")
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.GetUser(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetUser_ForbiddenForOtherUser(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/99", nil)
	r = withChiID(r, "id", "99")
	// user ID 1 trying to access user ID 99, not admin
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.GetUser(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestGetUser_AdminCanAccessOthers(t *testing.T) {
	us := &configUserStore{getUserResult: sampleUser(99, "target", false)}
	h := newUserHandler(us)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/99", nil)
	r = withChiID(r, "id", "99")
	r = withAuthUser(r, 1, "admin", true)
	w := httptest.NewRecorder()
	h.GetUser(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetUser_SelfAccess(t *testing.T) {
	us := &configUserStore{getUserResult: sampleUser(5, "bob", false)}
	h := newUserHandler(us)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/5", nil)
	r = withChiID(r, "id", "5")
	r = withAuthUser(r, 5, "bob", false)
	w := httptest.NewRecorder()
	h.GetUser(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	us := &configUserStore{getUserResult: nil}
	h := newUserHandler(us)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/42", nil)
	r = withChiID(r, "id", "42")
	r = withAuthUser(r, 1, "admin", true)
	w := httptest.NewRecorder()
	h.GetUser(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateUser_InvalidID(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/bad", strings.NewReader("{}"))
	r = withChiID(r, "id", "bad")
	w := httptest.NewRecorder()
	h.UpdateUser(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateUser_NotFound(t *testing.T) {
	us := &configUserStore{getUserResult: nil}
	h := newUserHandler(us)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/99", strings.NewReader("{}"))
	r = withChiID(r, "id", "99")
	w := httptest.NewRecorder()
	h.UpdateUser(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateUser_InvalidJSON(t *testing.T) {
	us := &configUserStore{getUserResult: sampleUser(2, "bob", false)}
	h := newUserHandler(us)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/2", strings.NewReader("{bad}"))
	r = withChiID(r, "id", "2")
	w := httptest.NewRecorder()
	h.UpdateUser(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateUser_UpdateError(t *testing.T) {
	us := &configUserStore{
		getUserResult: sampleUser(2, "bob", false),
		updateUserErr: errUserStore,
	}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"display_name": "Bob"})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/2", bytes.NewReader(body))
	r = withChiID(r, "id", "2")
	w := httptest.NewRecorder()
	h.UpdateUser(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestUpdateUser_Success(t *testing.T) {
	us := &configUserStore{getUserResult: sampleUser(2, "bob", false)}
	h := newUserHandler(us)
	isAdmin := true
	body, _ := json.Marshal(map[string]any{"display_name": "Bobby", "is_admin": isAdmin})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/2", bytes.NewReader(body))
	r = withChiID(r, "id", "2")
	w := httptest.NewRecorder()
	h.UpdateUser(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteUser_NotAuthenticated(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/users/2", nil)
	r = withChiID(r, "id", "2")
	w := httptest.NewRecorder()
	h.DeleteUser(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestDeleteUser_InvalidID(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/users/xyz", nil)
	r = withChiID(r, "id", "xyz")
	r = withAuthUser(r, 1, "admin", true)
	w := httptest.NewRecorder()
	h.DeleteUser(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDeleteUser_CannotDeleteSelf(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/users/1", nil)
	r = withChiID(r, "id", "1")
	r = withAuthUser(r, 1, "admin", true)
	w := httptest.NewRecorder()
	h.DeleteUser(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestDeleteUser_DeleteError(t *testing.T) {
	us := &configUserStore{deleteUserErr: errUserStore}
	h := newUserHandler(us)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/users/99", nil)
	r = withChiID(r, "id", "99")
	r = withAuthUser(r, 1, "admin", true)
	w := httptest.NewRecorder()
	h.DeleteUser(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestDeleteUser_Success(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/users/99", nil)
	r = withChiID(r, "id", "99")
	r = withAuthUser(r, 1, "admin", true)
	w := httptest.NewRecorder()
	h.DeleteUser(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSetSubsonicPassword_NotAuthenticated(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/me/subsonic-password", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	h.SetSubsonicPassword(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestSetSubsonicPassword_InvalidJSON(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/me/subsonic-password", strings.NewReader("{bad}"))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.SetSubsonicPassword(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSetSubsonicPassword_EmptyPassword(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	body, _ := json.Marshal(map[string]string{"password": ""})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/me/subsonic-password", bytes.NewReader(body))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.SetSubsonicPassword(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSetSubsonicPassword_NoKey(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	body, _ := json.Marshal(map[string]string{"password": "subsonicpass"})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/me/subsonic-password", bytes.NewReader(body))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.SetSubsonicPassword(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when no subsonic key configured, got %d", w.Code)
	}
}

func TestSetSubsonicPassword_Success(t *testing.T) {
	us := &configUserStore{}
	h := newUserHandler(us)
	// 32-byte AES key
	h.subsonicKey = make([]byte, 32)
	body, _ := json.Marshal(map[string]string{"password": "subsonicpass"})
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/me/subsonic-password", bytes.NewReader(body))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.SetSubsonicPassword(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChangePassword_NotAuthenticated(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users/1/password", strings.NewReader("{}"))
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.ChangePassword(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestChangePassword_InvalidID(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users/bad/password", strings.NewReader("{}"))
	r = withChiID(r, "id", "bad")
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.ChangePassword(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestChangePassword_ForbiddenForOtherUser(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	body, _ := json.Marshal(map[string]string{"new_password": "newpass"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users/99/password", bytes.NewReader(body))
	r = withChiID(r, "id", "99")
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.ChangePassword(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestChangePassword_MissingNewPassword(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	body, _ := json.Marshal(map[string]string{"new_password": ""})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users/1/password", bytes.NewReader(body))
	r = withChiID(r, "id", "1")
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.ChangePassword(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestChangePassword_UserNotFound(t *testing.T) {
	us := &configUserStore{getUserResult: nil}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"new_password": "newpass"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users/1/password", bytes.NewReader(body))
	r = withChiID(r, "id", "1")
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.ChangePassword(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestChangePassword_RegularUserNeedsCurrentPassword(t *testing.T) {
	us := &configUserStore{getUserResult: sampleUser(1, "alice", false)}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"new_password": "newpass", "password": ""})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users/1/password", bytes.NewReader(body))
	r = withChiID(r, "id", "1")
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.ChangePassword(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when current password missing for regular user, got %d", w.Code)
	}
}

func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	user := sampleUser(1, "alice", false)
	us := &configUserStore{getUserResult: user}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"password": "wrongpass", "new_password": "newpass"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users/1/password", bytes.NewReader(body))
	r = withChiID(r, "id", "1")
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.ChangePassword(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong current password, got %d", w.Code)
	}
}

func TestChangePassword_RegularUserSuccess(t *testing.T) {
	user := sampleUser(1, "alice", false)
	us := &configUserStore{getUserResult: user}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"password": "testpass", "new_password": "newpass"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users/1/password", bytes.NewReader(body))
	r = withChiID(r, "id", "1")
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.ChangePassword(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChangePassword_AdminBypassesCurrentPassword(t *testing.T) {
	user := sampleUser(99, "target", false)
	us := &configUserStore{getUserResult: user}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"new_password": "adminset"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users/99/password", bytes.NewReader(body))
	r = withChiID(r, "id", "99")
	r = withAuthUser(r, 1, "admin", true)
	w := httptest.NewRecorder()
	h.ChangePassword(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for admin bypass, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSafeUser_OmitsPasswordHash(t *testing.T) {
	u := sampleUser(1, "alice", false)
	m := safeUser(u)
	if _, ok := m["password_hash"]; ok {
		t.Error("safeUser should not include password_hash")
	}
}

func TestSafeUser_IncludesRequiredFields(t *testing.T) {
	u := sampleUser(3, "carol", true)
	m := safeUser(u)
	required := []string{"id", "username", "email", "is_admin", "is_enabled", "language", "theme"}
	for _, field := range required {
		if _, ok := m[field]; !ok {
			t.Errorf("safeUser missing field %q", field)
		}
	}
}

func TestParsePagination_Defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	limit, offset := parsePagination(r)
	if limit != 50 {
		t.Errorf("expected default limit=50, got %d", limit)
	}
	if offset != 0 {
		t.Errorf("expected default offset=0, got %d", offset)
	}
}

func TestParsePagination_CustomValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=100&offset=25", nil)
	limit, offset := parsePagination(r)
	if limit != 100 {
		t.Errorf("expected limit=100, got %d", limit)
	}
	if offset != 25 {
		t.Errorf("expected offset=25, got %d", offset)
	}
}

func TestParsePagination_MaxCap(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=9999", nil)
	limit, _ := parsePagination(r)
	if limit != 500 {
		t.Errorf("expected max limit=500, got %d", limit)
	}
}

func TestParsePagination_InvalidValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=abc&offset=xyz", nil)
	limit, offset := parsePagination(r)
	if limit != 50 {
		t.Errorf("expected default limit on invalid value, got %d", limit)
	}
	if offset != 0 {
		t.Errorf("expected default offset on invalid value, got %d", offset)
	}
}

func TestParseID_Valid(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiID(r, "id", "42")
	id, err := parseID(r, "id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Errorf("expected id=42, got %d", id)
	}
}

func TestParseID_Zero(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiID(r, "id", "0")
	_, err := parseID(r, "id")
	if err == nil {
		t.Error("expected error for id=0")
	}
}

func TestParseID_Negative(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiID(r, "id", "-1")
	_, err := parseID(r, "id")
	if err == nil {
		t.Error("expected error for id=-1")
	}
}

func TestParseID_NonNumeric(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiID(r, "id", "notanumber")
	_, err := parseID(r, "id")
	if err == nil {
		t.Error("expected error for non-numeric id")
	}
}
