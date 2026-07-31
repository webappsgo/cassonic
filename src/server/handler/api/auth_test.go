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

func TestHashPassword_ReturnsArgon2idPrefix(t *testing.T) {
	h, err := hashPassword("secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$") {
		t.Errorf("expected argon2id prefix, got %q", h)
	}
}

func TestHashPassword_DifferentSaltsEachCall(t *testing.T) {
	h1, _ := hashPassword("password")
	h2, _ := hashPassword("password")
	if h1 == h2 {
		t.Error("expected different hashes due to random salts")
	}
}

func TestVerifyPassword_CorrectPassword(t *testing.T) {
	hash, _ := hashPassword("correct")
	ok, err := verifyPassword("correct", hash)
	if err != nil {
		t.Fatalf("verifyPassword error: %v", err)
	}
	if !ok {
		t.Error("expected correct password to verify")
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	hash, _ := hashPassword("correct")
	ok, err := verifyPassword("wrong", hash)
	if err != nil {
		t.Fatalf("verifyPassword error: %v", err)
	}
	if ok {
		t.Error("expected wrong password to not verify")
	}
}

func TestVerifyPassword_BadHashFormat(t *testing.T) {
	_, err := verifyPassword("pass", "notahash")
	if err == nil {
		t.Error("expected error for malformed hash")
	}
}

func TestGenerateToken_UniqueValues(t *testing.T) {
	r1, h1, err1 := generateToken()
	r2, h2, err2 := generateToken()
	if err1 != nil || err2 != nil {
		t.Fatalf("generateToken errors: %v, %v", err1, err2)
	}
	if r1 == r2 {
		t.Error("expected unique raw tokens")
	}
	if h1 == h2 {
		t.Error("expected unique token hashes")
	}
}

func TestGenerateToken_HashLength(t *testing.T) {
	_, hash, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken: %v", err)
	}
	// SHA-256 hex = 64 characters
	if len(hash) != 64 {
		t.Errorf("expected 64-char hash, got %d", len(hash))
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("{bad}"))
	w := httptest.NewRecorder()
	h.Login(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	body, _ := json.Marshal(map[string]string{"username": "alice"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Login(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestLogin_StoreError(t *testing.T) {
	us := &configUserStore{getUserByUsernameErr: errUserStore}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "pass"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Login(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	us := &configUserStore{getUserByUsernameResult: nil}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"username": "nobody", "password": "pass"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Login(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestLogin_DisabledAccount(t *testing.T) {
	us := &configUserStore{
		getUserByUsernameResult: &model.User{
			ID:           1,
			Username:     "alice",
			IsEnabled:    false,
			PasswordHash: mustHashPassword("pass"),
		},
	}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "pass"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Login(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for disabled account, got %d", w.Code)
	}
}

func TestLogin_LockedAccount(t *testing.T) {
	us := &configUserStore{
		getUserByUsernameResult: &model.User{
			ID:           1,
			Username:     "alice",
			IsEnabled:    true,
			LockedUntil:  time.Now().Add(10 * time.Minute),
			PasswordHash: mustHashPassword("pass"),
		},
	}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "pass"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Login(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for locked account, got %d", w.Code)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	hash := mustHashPassword("correct")
	us := &configUserStore{
		getUserByUsernameResult: &model.User{
			ID:           1,
			Username:     "alice",
			IsEnabled:    true,
			PasswordHash: hash,
		},
	}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "wrong"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Login(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password, got %d", w.Code)
	}
}

func TestLogin_SessionCreationError(t *testing.T) {
	hash := mustHashPassword("pass")
	us := &configUserStore{
		getUserByUsernameResult: &model.User{
			ID:           1,
			Username:     "alice",
			IsEnabled:    true,
			PasswordHash: hash,
		},
		createSessionErr: errUserStore,
	}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "pass"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Login(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on session creation error, got %d", w.Code)
	}
}

func TestLogin_Success(t *testing.T) {
	hash := mustHashPassword("pass")
	us := &configUserStore{
		getUserByUsernameResult: &model.User{
			ID:           42,
			Username:     "alice",
			IsEnabled:    true,
			PasswordHash: hash,
		},
	}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "pass"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Login(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK {
		t.Error("expected ok:true")
	}
	if resp.Data.Token == "" {
		t.Error("expected non-empty token in response")
	}
}

func TestLogin_ClientNameHeader(t *testing.T) {
	hash := mustHashPassword("pass")
	us := &configUserStore{
		getUserByUsernameResult: &model.User{
			ID:           1,
			Username:     "bob",
			IsEnabled:    true,
			PasswordHash: hash,
		},
	}
	h := newUserHandler(us)
	body, _ := json.Marshal(map[string]string{"username": "bob", "password": "pass"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	r.Header.Set("X-Client-Name", "my-app")
	w := httptest.NewRecorder()
	h.Login(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestLogout_MissingBearer(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	h.Logout(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestLogout_WrongScheme(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	h.Logout(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestLogout_DeleteSessionError(t *testing.T) {
	us := &configUserStore{deleteSessionErr: errUserStore}
	h := newUserHandler(us)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	r.Header.Set("Authorization", "Bearer sometoken")
	w := httptest.NewRecorder()
	h.Logout(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestLogout_Success(t *testing.T) {
	h := newUserHandler(&configUserStore{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	r.Header.Set("Authorization", "Bearer somerawtoken")
	w := httptest.NewRecorder()
	h.Logout(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCreateToken_InvalidJSON(t *testing.T) {
	h := newUserHandlerWithAuth(&configUserStore{}, false)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader("{bad}"))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreateToken(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateToken_MissingName(t *testing.T) {
	h := newUserHandlerWithAuth(&configUserStore{}, false)
	body, _ := json.Marshal(map[string]string{"name": ""})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", bytes.NewReader(body))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreateToken(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateToken_InvalidExpiresAt(t *testing.T) {
	h := newUserHandlerWithAuth(&configUserStore{}, false)
	body, _ := json.Marshal(map[string]string{"name": "mytoken", "expires_at": "not-a-date"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", bytes.NewReader(body))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreateToken(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateToken_StoreError(t *testing.T) {
	us := &configUserStore{createAPITokenErr: errUserStore}
	h := newUserHandlerWithAuth(us, false)
	body, _ := json.Marshal(map[string]string{"name": "mytoken"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", bytes.NewReader(body))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreateToken(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestCreateToken_Success(t *testing.T) {
	us := &configUserStore{
		listAPITokensResult: []*model.APIToken{
			{ID: 99, UserID: 1, TokenHash: "", Name: "mytoken"},
		},
	}
	h := newUserHandlerWithAuth(us, false)
	body, _ := json.Marshal(map[string]string{"name": "mytoken"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", bytes.NewReader(body))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreateToken(w, r)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Token string `json:"token"`
			Name  string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Data.Token == "" {
		t.Error("expected non-empty token in response")
	}
}

func TestCreateToken_WithExpiry(t *testing.T) {
	us := &configUserStore{}
	h := newUserHandlerWithAuth(us, false)
	expiry := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	body, _ := json.Marshal(map[string]string{"name": "tok", "expires_at": expiry})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", bytes.NewReader(body))
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.CreateToken(w, r)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestDeleteToken_InvalidID(t *testing.T) {
	h := newUserHandlerWithAuth(&configUserStore{}, false)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/bad", nil)
	r = withChiID(r, "id", "bad")
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.DeleteToken(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDeleteToken_ListError(t *testing.T) {
	us := &configUserStore{listAPITokensErr: errUserStore}
	h := newUserHandlerWithAuth(us, false)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/5", nil)
	r = withChiID(r, "id", "5")
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.DeleteToken(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestDeleteToken_NotFound(t *testing.T) {
	us := &configUserStore{
		listAPITokensResult: []*model.APIToken{{ID: 10, UserID: 1}},
	}
	h := newUserHandlerWithAuth(us, false)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/99", nil)
	r = withChiID(r, "id", "99")
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.DeleteToken(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteToken_DeleteError(t *testing.T) {
	us := &configUserStore{
		listAPITokensResult: []*model.APIToken{{ID: 5, UserID: 1}},
		deleteAPITokenErr:   errUserStore,
	}
	h := newUserHandlerWithAuth(us, false)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/5", nil)
	r = withChiID(r, "id", "5")
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.DeleteToken(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestDeleteToken_Success(t *testing.T) {
	us := &configUserStore{
		listAPITokensResult: []*model.APIToken{{ID: 7, UserID: 1}},
	}
	h := newUserHandlerWithAuth(us, false)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/7", nil)
	r = withChiID(r, "id", "7")
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.DeleteToken(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
