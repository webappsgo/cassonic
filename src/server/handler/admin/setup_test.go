package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// newSetupToken returns a raw token plus a *model.SetupToken row holding its hash.
func newSetupToken(t *testing.T) (raw string, tok *model.SetupToken) {
	t.Helper()
	raw, hash, err := store.GenerateSetupToken()
	if err != nil {
		t.Fatalf("GenerateSetupToken: %v", err)
	}
	return raw, &model.SetupToken{TokenHash: hash, CreatedAt: time.Now().UTC()}
}

func TestSetupInProgress(t *testing.T) {
	raw, tok := newSetupToken(t)
	db := testDB()
	adminStore := db.Admin.(*testAdminStore)
	adminStore.getSetupTokenResult = tok
	h := newTestHandler(db, testConfig(t.TempDir()))

	t.Run("valid cookie", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(&http.Cookie{Name: setupSessionCookieName, Value: raw})
		if !h.setupInProgress(r) {
			t.Fatal("expected setupInProgress true for valid unconsumed token cookie")
		}
	})

	t.Run("no cookie", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if h.setupInProgress(r) {
			t.Fatal("expected setupInProgress false with no cookie")
		}
	})

	t.Run("wrong cookie value", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(&http.Cookie{Name: setupSessionCookieName, Value: "deadbeef"})
		if h.setupInProgress(r) {
			t.Fatal("expected setupInProgress false for wrong token")
		}
	})

	t.Run("already used token", func(t *testing.T) {
		usedTok := *tok
		usedTok.Used = true
		adminStore.getSetupTokenResult = &usedTok
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(&http.Cookie{Name: setupSessionCookieName, Value: raw})
		if h.setupInProgress(r) {
			t.Fatal("expected setupInProgress false once token is used")
		}
		adminStore.getSetupTokenResult = tok
	})
}

func TestRoot_NoAdmins_ShowsSetupTokenEntry(t *testing.T) {
	db := testDB()
	db.Admin.(*testAdminStore).countAdminsResult = 0
	h := newTestHandler(db, testConfig(t.TempDir()))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.Root(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Setup") {
		t.Fatalf("expected setup token entry page, got: %s", w.Body.String())
	}
}

func TestRoot_AdminsExist_RedirectsToLogin(t *testing.T) {
	db := testDB()
	db.Admin.(*testAdminStore).countAdminsResult = 1
	h := newTestHandler(db, testConfig(t.TempDir()))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.Root(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); !strings.HasPrefix(loc, "/login") {
		t.Fatalf("expected redirect to /login, got %q", loc)
	}
}

func TestRootPost_CorrectToken_SetsCookieAndRedirects(t *testing.T) {
	raw, tok := newSetupToken(t)
	db := testDB()
	adminStore := db.Admin.(*testAdminStore)
	adminStore.getSetupTokenResult = tok
	h := newTestHandler(db, testConfig(t.TempDir()))

	form := url.Values{"token": {raw}}
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.RootPost(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.HasSuffix(w.Header().Get("Location"), "/config/setup") {
		t.Fatalf("expected redirect to /config/setup, got %q", w.Header().Get("Location"))
	}
	var found bool
	for _, c := range w.Result().Cookies() {
		if c.Name == setupSessionCookieName && c.Value == raw {
			found = true
		}
	}
	if !found {
		t.Fatal("expected setup session cookie to be set to the raw token")
	}
}

func TestRootPost_WrongToken_ShowsError(t *testing.T) {
	_, tok := newSetupToken(t)
	db := testDB()
	db.Admin.(*testAdminStore).getSetupTokenResult = tok
	h := newTestHandler(db, testConfig(t.TempDir()))

	form := url.Values{"token": {"0000000000000000000000000000000000"}}
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.RootPost(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-render with error), got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "incorrect setup token") {
		t.Fatalf("expected error message in body, got: %s", w.Body.String())
	}
}

func TestRootPost_TokenAlreadyUsed(t *testing.T) {
	_, tok := newSetupToken(t)
	tok.Used = true
	db := testDB()
	db.Admin.(*testAdminStore).getSetupTokenResult = tok
	h := newTestHandler(db, testConfig(t.TempDir()))

	form := url.Values{"token": {"anything"}}
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.RootPost(w, r)

	if !strings.Contains(w.Body.String(), "already been used") {
		t.Fatalf("expected already-used error, got: %s", w.Body.String())
	}
}

func TestRootPost_SetupAlreadyComplete_RedirectsToLogin(t *testing.T) {
	db := testDB()
	db.Admin.(*testAdminStore).countAdminsResult = 1
	h := newTestHandler(db, testConfig(t.TempDir()))

	form := url.Values{"token": {"anything"}}
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.RootPost(w, r)

	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login" {
		t.Fatalf("expected 303 to /login, got %d %q", w.Code, w.Header().Get("Location"))
	}
}

func TestSetupWizard_NoSession_RedirectsToRoot(t *testing.T) {
	db := testDB()
	h := newTestHandler(db, testConfig(t.TempDir()))

	r := httptest.NewRequest(http.MethodGet, "/config/setup", nil)
	w := httptest.NewRecorder()
	h.SetupWizard(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
}

func TestSetupWizard_WithSession_ShowsStep1(t *testing.T) {
	raw, tok := newSetupToken(t)
	db := testDB()
	db.Admin.(*testAdminStore).getSetupTokenResult = tok
	h := newTestHandler(db, testConfig(t.TempDir()))

	r := httptest.NewRequest(http.MethodGet, "/config/setup", nil)
	r.AddCookie(&http.Cookie{Name: setupSessionCookieName, Value: raw})
	w := httptest.NewRecorder()
	h.SetupWizard(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "administrator") {
		t.Fatalf("expected default username 'administrator' in step 1, got: %s", w.Body.String())
	}
}

// postSetupStep is a helper posting one wizard step with an active setup session.
func postSetupStep(t *testing.T, h *Handler, sessionCookie string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/config/setup", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: setupSessionCookieName, Value: sessionCookie})
	w := httptest.NewRecorder()
	h.SetupWizardPost(w, r)
	return w
}

func TestSetupWizardPost_Step1_GeneratesTokenAndAdvances(t *testing.T) {
	raw, tok := newSetupToken(t)
	db := testDB()
	db.Admin.(*testAdminStore).getSetupTokenResult = tok
	h := newTestHandler(db, testConfig(t.TempDir()))

	form := url.Values{"step": {"1"}, "username": {"boss"}, "password": {""}, "password_confirm": {""}}
	w := postSetupStep(t, h, raw, form)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "adm_") {
		t.Fatalf("expected generated API token shown on step 2, got: %s", body)
	}
}

func TestSetupWizardPost_Step1_PasswordMismatch(t *testing.T) {
	raw, tok := newSetupToken(t)
	db := testDB()
	db.Admin.(*testAdminStore).getSetupTokenResult = tok
	h := newTestHandler(db, testConfig(t.TempDir()))

	form := url.Values{"step": {"1"}, "username": {"boss"}, "password": {"aaa"}, "password_confirm": {"bbb"}}
	w := postSetupStep(t, h, raw, form)

	if !strings.Contains(w.Body.String(), "passwords do not match") {
		t.Fatalf("expected mismatch error, got: %s", w.Body.String())
	}
}

func TestSetupWizardPost_FullFlow_CompletesSetup(t *testing.T) {
	raw, tok := newSetupToken(t)
	db := testDB()
	adminStore := db.Admin.(*testAdminStore)
	adminStore.getSetupTokenResult = tok
	adminStore.createAdminResult = 1
	cfg := testConfig(t.TempDir())
	h := newTestHandler(db, cfg)

	// Step 1.
	w := postSetupStep(t, h, raw, url.Values{
		"step": {"1"}, "username": {"boss"}, "password": {"correcthorsebatterystaple"}, "password_confirm": {"correcthorsebatterystaple"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("step1: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	passwordHash := extractHiddenValue(t, w.Body.String(), "password_hash")
	apiTokenHash := extractHiddenValue(t, w.Body.String(), "api_token_hash")

	// Step 2 (acknowledge API token).
	w = postSetupStep(t, h, raw, url.Values{
		"step": {"2"}, "username": {"boss"}, "password_hash": {passwordHash}, "api_token_hash": {apiTokenHash},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("step2: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Step 3 (server configuration).
	w = postSetupStep(t, h, raw, url.Values{
		"step": {"3"}, "username": {"boss"}, "password_hash": {passwordHash}, "api_token_hash": {apiTokenHash},
		"app_name": {"MyServer"}, "domain": {"music.example.com"}, "mode": {"production"}, "timezone": {"UTC"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("step3: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Step 4 (security settings).
	w = postSetupStep(t, h, raw, url.Values{
		"step": {"4"}, "username": {"boss"}, "password_hash": {passwordHash}, "api_token_hash": {apiTokenHash},
		"app_name": {"MyServer"}, "domain": {"music.example.com"}, "mode": {"production"}, "timezone": {"UTC"},
		"backup_encryption_password": {"backupsecret"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("step4: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Step 5 (optional services).
	w = postSetupStep(t, h, raw, url.Values{
		"step": {"5"}, "username": {"boss"}, "password_hash": {passwordHash}, "api_token_hash": {apiTokenHash},
		"app_name": {"MyServer"}, "domain": {"music.example.com"}, "mode": {"production"}, "timezone": {"UTC"},
		"backup_encryption_password": {"backupsecret"}, "https_reviewed": {"on"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("step5: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Step 6 (complete).
	w = postSetupStep(t, h, raw, url.Values{
		"step": {"6"}, "username": {"boss"}, "password_hash": {passwordHash}, "api_token_hash": {apiTokenHash},
		"app_name": {"MyServer"}, "domain": {"music.example.com"}, "mode": {"production"}, "timezone": {"UTC"},
		"backup_encryption_password": {"backupsecret"}, "https_reviewed": {"on"},
	})
	if w.Code != http.StatusSeeOther {
		t.Fatalf("step6: expected 303, got %d: %s", w.Code, w.Body.String())
	}

	if adminStore.lastCreatedAdmin == nil {
		t.Fatal("expected CreateAdmin to be called")
	}
	if adminStore.lastCreatedAdmin.Username != "boss" {
		t.Fatalf("expected username 'boss', got %q", adminStore.lastCreatedAdmin.Username)
	}
	if adminStore.lastCreatedAdmin.Role != "superadmin" {
		t.Fatalf("expected role 'superadmin', got %q", adminStore.lastCreatedAdmin.Role)
	}
	if adminStore.lastCreatedSession == nil {
		t.Fatal("expected an admin session to be created (auto-login)")
	}
	if cfg.Server.AppName != "MyServer" {
		t.Fatalf("expected server.yml to be updated with app_name, got %q", cfg.Server.AppName)
	}
	if cfg.Backup.EncryptionPassword != "backupsecret" {
		t.Fatalf("expected backup encryption password to be saved, got %q", cfg.Backup.EncryptionPassword)
	}

	var setupCookieCleared bool
	for _, c := range w.Result().Cookies() {
		if c.Name == setupSessionCookieName && c.MaxAge < 0 {
			setupCookieCleared = true
		}
	}
	if !setupCookieCleared {
		t.Fatal("expected the setup-session cookie to be cleared on completion")
	}
}

// extractHiddenValue pulls the value attribute of a hidden input with the given name
// out of rendered HTML, for chaining wizard steps in tests.
func extractHiddenValue(t *testing.T, body, name string) string {
	t.Helper()
	marker := `name="` + name + `" value="`
	idx := strings.Index(body, marker)
	if idx == -1 {
		t.Fatalf("hidden field %q not found in body: %s", name, body)
	}
	rest := body[idx+len(marker):]
	end := strings.Index(rest, `"`)
	if end == -1 {
		t.Fatalf("hidden field %q value not terminated", name)
	}
	return rest[:end]
}

func TestCompleteSetup_MissingSession_RestartsWizard(t *testing.T) {
	raw, tok := newSetupToken(t)
	db := testDB()
	db.Admin.(*testAdminStore).getSetupTokenResult = tok
	h := newTestHandler(db, testConfig(t.TempDir()))

	w := postSetupStep(t, h, raw, url.Values{"step": {"6"}})
	if !strings.Contains(w.Body.String(), "setup session expired") {
		t.Fatalf("expected expired-session error, got: %s", w.Body.String())
	}
}
