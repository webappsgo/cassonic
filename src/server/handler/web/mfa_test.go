package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	pquernaotp "github.com/pquerna/otp"
	pquernatotp "github.com/pquerna/otp/totp"

	"github.com/local/cassonic/src/config"
	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/service/crypto"
	totpsvc "github.com/local/cassonic/src/server/service/totp"
	"github.com/local/cassonic/src/server/store"
)

// newMFATestHandlerFrom wires a web Handler with a 32-byte base64 encryption
// key set (AI.md PART 11 / PART 17), required to decrypt stored TOTP
// secrets.
func newMFATestHandlerFrom(db *store.DB) *Handler {
	cfg := config.Defaults()
	cfg.Security.EncryptionKey = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="
	return NewHandlerWithConfig(db, cfg, "test-version")
}

func currentTOTPCode(t *testing.T, secret string) string {
	t.Helper()
	code, err := pquernatotp.GenerateCodeCustom(secret, time.Now(), pquernatotp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    pquernaotp.DigitsSix,
		Algorithm: pquernaotp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("GenerateCodeCustom: %v", err)
	}
	return code
}

func TestLoginPost_AdminWithTOTP_BeginsChallenge(t *testing.T) {
	db := testDB()
	hash := mustHashAdminPassword(t, "correct")
	db.Admin.(*testAdminStore).getAdminByUsernameResult = &model.Admin{
		ID: 1, Username: "root", Enabled: true, PasswordHash: hash, TOTPEnabled: true,
	}
	h := newTestHandler(db)
	form := url.Values{"username": {"root"}, "password": {"correct"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.LoginPost(w, r)

	if w.Header().Get("Location") != "/login/mfa" {
		t.Fatalf("expected redirect to /login/mfa, got %q", w.Header().Get("Location"))
	}
	var mfaCookie, adminSessionCookie bool
	for _, c := range w.Result().Cookies() {
		if c.Name == AdminMFACookieName && c.Value != "" {
			mfaCookie = true
		}
		if c.Name == mw.AdminSessionCookieName && c.Value != "" {
			adminSessionCookie = true
		}
	}
	if !mfaCookie {
		t.Fatal("expected admin MFA challenge cookie to be set")
	}
	if adminSessionCookie {
		t.Fatal("expected no real admin session cookie until 2FA is completed")
	}
	if db.Admin.(*testAdminStore).lastCreatedMFAChallenge == nil {
		t.Fatal("expected an MFA challenge row to be created")
	}
}

func TestMFAChallenge_NoPendingChallenge_RedirectsToLogin(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/login/mfa", nil)
	w := httptest.NewRecorder()
	h.MFAChallenge(w, r)

	if w.Header().Get("Location") != "/login" {
		t.Fatalf("expected redirect to /login, got %q", w.Header().Get("Location"))
	}
}

func TestMFAChallenge_PendingChallenge_Renders(t *testing.T) {
	db := testDB()
	db.Admin.(*testAdminStore).getAdminMFAChallengeResult = &model.AdminMFAChallenge{
		AdminID: 1, ExpiresAt: time.Now().Add(time.Minute),
	}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/login/mfa", nil)
	r.AddCookie(&http.Cookie{Name: AdminMFACookieName, Value: "raw-token"})
	w := httptest.NewRecorder()
	h.MFAChallenge(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMFAChallengePost_ValidCode_CompletesLogin(t *testing.T) {
	db := testDB()
	adminStore := db.Admin.(*testAdminStore)
	adminStore.getAdminMFAChallengeResult = &model.AdminMFAChallenge{
		AdminID: 1, ExpiresAt: time.Now().Add(time.Minute),
	}
	adminStore.getAdminResult = &model.Admin{ID: 1, Username: "root", Enabled: true, TOTPEnabled: true}

	h := newMFATestHandlerFrom(db)

	secret, err := totpsvc.Generate("cassonic", "root")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	encrypted, err := crypto.Encrypt(crypto.DeriveKey([]byte(h.cfg.Security.EncryptionKey)), secret.Secret)
	if err != nil {
		t.Fatalf("crypto.Encrypt: %v", err)
	}
	adminStore.getTOTPSecretResult = &model.TOTPSecret{UserType: "admin", UserID: 1, Secret: encrypted, Enabled: true}

	code := currentTOTPCode(t, secret.Secret)
	form := url.Values{"code": {code}}
	r := httptest.NewRequest(http.MethodPost, "/login/mfa", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: AdminMFACookieName, Value: "raw-token"})
	w := httptest.NewRecorder()
	h.MFAChallengePost(w, r)

	wantLoc := "/server/" + h.cfg.AdminPath()
	if w.Header().Get("Location") != wantLoc {
		t.Fatalf("expected redirect to %q, got %q", wantLoc, w.Header().Get("Location"))
	}
	var adminSessionCookie bool
	for _, c := range w.Result().Cookies() {
		if c.Name == mw.AdminSessionCookieName && c.Value != "" {
			adminSessionCookie = true
		}
	}
	if !adminSessionCookie {
		t.Fatal("expected real admin session cookie after successful 2FA")
	}
}

func TestMFAChallengePost_InvalidCode_Rejected(t *testing.T) {
	db := testDB()
	adminStore := db.Admin.(*testAdminStore)
	adminStore.getAdminMFAChallengeResult = &model.AdminMFAChallenge{
		AdminID: 1, ExpiresAt: time.Now().Add(time.Minute),
	}
	adminStore.getAdminResult = &model.Admin{ID: 1, Username: "root", Enabled: true, TOTPEnabled: true}

	h := newMFATestHandlerFrom(db)

	secret, err := totpsvc.Generate("cassonic", "root")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	encrypted, err := crypto.Encrypt(crypto.DeriveKey([]byte(h.cfg.Security.EncryptionKey)), secret.Secret)
	if err != nil {
		t.Fatalf("crypto.Encrypt: %v", err)
	}
	adminStore.getTOTPSecretResult = &model.TOTPSecret{UserType: "admin", UserID: 1, Secret: encrypted, Enabled: true}

	form := url.Values{"code": {"000000"}}
	r := httptest.NewRequest(http.MethodPost, "/login/mfa", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: AdminMFACookieName, Value: "raw-token"})
	w := httptest.NewRecorder()
	h.MFAChallengePost(w, r)

	if !strings.Contains(w.Header().Get("Location"), "invalid+code") {
		t.Fatalf("expected invalid code redirect, got %q", w.Header().Get("Location"))
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == mw.AdminSessionCookieName && c.Value != "" {
			t.Fatal("expected no admin session cookie on invalid 2FA code")
		}
	}
}
