package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	pquernaotp "github.com/pquerna/otp"
	pquernatotp "github.com/pquerna/otp/totp"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/service/crypto"
	totpsvc "github.com/local/cassonic/src/server/service/totp"
)

// adminHandlerWithEncryptionKey wires a handler with an authenticated admin
// and a 32-byte base64 encryption key set (AI.md PART 11 / PART 17).
func adminHandlerWithEncryptionKey(t *testing.T, admin *model.Admin) (*Handler, *testAdminStore) {
	t.Helper()
	db := testDB()
	adminStore := db.Admin.(*testAdminStore)
	adminStore.getAdminResult = admin
	cfg := testConfig(t.TempDir())
	cfg.Security.EncryptionKey = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=" // 32 raw bytes, base64
	h := newTestHandler(db, cfg)
	return h, adminStore
}

func withAuthedAdminRequest(r *http.Request, id int64, username string) *http.Request {
	r = withAdminUser(r, id, username, "superadmin")
	return withChiParam(r, "admin_username", username)
}

func TestEnrollTOTPStart_ShowsQRCode(t *testing.T) {
	admin := &model.Admin{ID: 1, Username: "boss", Enabled: true}
	h, _ := adminHandlerWithEncryptionKey(t, admin)

	r := httptest.NewRequest(http.MethodPost, "/boss/profile/security/totp/enroll", nil)
	r = withAuthedAdminRequest(r, 1, "boss")
	w := httptest.NewRecorder()
	h.EnrollTOTPStart(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "data:image/png;base64,") {
		t.Fatalf("expected inline QR code image, got: %s", w.Body.String())
	}
}

func TestEnrollTOTPStart_AlreadyEnabled_Errors(t *testing.T) {
	admin := &model.Admin{ID: 1, Username: "boss", Enabled: true, TOTPEnabled: true}
	h, _ := adminHandlerWithEncryptionKey(t, admin)

	r := httptest.NewRequest(http.MethodPost, "/boss/profile/security/totp/enroll", nil)
	r = withAuthedAdminRequest(r, 1, "boss")
	w := httptest.NewRecorder()
	h.EnrollTOTPStart(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Location"), "error=") {
		t.Fatalf("expected error redirect, got %q", w.Header().Get("Location"))
	}
}

func TestEnrollTOTPConfirm_ValidCode_EnablesAndShowsBackupCodes(t *testing.T) {
	admin := &model.Admin{ID: 1, Username: "boss", Enabled: true}
	h, adminStore := adminHandlerWithEncryptionKey(t, admin)

	enrollment, err := totpsvc.Generate("cassonic", "boss")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	code, err := currentTOTPCode(enrollment.Secret)
	if err != nil {
		t.Fatalf("currentTOTPCode: %v", err)
	}

	form := url.Values{"secret": {enrollment.Secret}, "code": {code}}
	r := httptest.NewRequest(http.MethodPost, "/boss/profile/security/totp/confirm", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = withAuthedAdminRequest(r, 1, "boss")
	w := httptest.NewRecorder()
	h.EnrollTOTPConfirm(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Backup Codes") {
		t.Fatalf("expected backup codes shown once, got: %s", w.Body.String())
	}
	if adminStore.lastCreatedAdmin != nil {
		t.Fatalf("EnableTOTP should not create a new admin row")
	}
}

func TestEnrollTOTPConfirm_InvalidCode_ReRendersEnrolling(t *testing.T) {
	admin := &model.Admin{ID: 1, Username: "boss", Enabled: true}
	h, _ := adminHandlerWithEncryptionKey(t, admin)

	enrollment, err := totpsvc.Generate("cassonic", "boss")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	form := url.Values{"secret": {enrollment.Secret}, "code": {"000000"}}
	r := httptest.NewRequest(http.MethodPost, "/boss/profile/security/totp/confirm", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = withAuthedAdminRequest(r, 1, "boss")
	w := httptest.NewRecorder()
	h.EnrollTOTPConfirm(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-render), got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid code") {
		t.Fatalf("expected invalid code message, got: %s", w.Body.String())
	}
}

func TestDisableTOTP_ValidCode_Disables(t *testing.T) {
	admin := &model.Admin{ID: 1, Username: "boss", Enabled: true, TOTPEnabled: true}
	h, adminStore := adminHandlerWithEncryptionKey(t, admin)

	secret, err := totpsvc.Generate("cassonic", "boss")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	encrypted := encryptForTest(t, h, secret.Secret)
	adminStore.getTOTPSecretResult = &model.TOTPSecret{
		UserType: totpUserType, UserID: 1, Secret: encrypted, Enabled: true,
	}

	code, err := currentTOTPCode(secret.Secret)
	if err != nil {
		t.Fatalf("currentTOTPCode: %v", err)
	}
	form := url.Values{"code": {code}}
	r := httptest.NewRequest(http.MethodPost, "/boss/profile/security/totp/disable", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = withAuthedAdminRequest(r, 1, "boss")
	w := httptest.NewRecorder()
	h.DisableTOTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Location"), "flash=") {
		t.Fatalf("expected success flash redirect, got %q", w.Header().Get("Location"))
	}
}

func TestDisableTOTP_InvalidCode_Errors(t *testing.T) {
	admin := &model.Admin{ID: 1, Username: "boss", Enabled: true, TOTPEnabled: true}
	h, adminStore := adminHandlerWithEncryptionKey(t, admin)

	secret, err := totpsvc.Generate("cassonic", "boss")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	encrypted := encryptForTest(t, h, secret.Secret)
	adminStore.getTOTPSecretResult = &model.TOTPSecret{
		UserType: totpUserType, UserID: 1, Secret: encrypted, Enabled: true,
	}

	form := url.Values{"code": {"000000"}}
	r := httptest.NewRequest(http.MethodPost, "/boss/profile/security/totp/disable", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = withAuthedAdminRequest(r, 1, "boss")
	w := httptest.NewRecorder()
	h.DisableTOTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Location"), "error=") {
		t.Fatalf("expected error redirect, got %q", w.Header().Get("Location"))
	}
}

func TestDisableTOTP_ValidBackupCode_Disables(t *testing.T) {
	admin := &model.Admin{ID: 1, Username: "boss", Enabled: true, TOTPEnabled: true}
	h, adminStore := adminHandlerWithEncryptionKey(t, admin)

	secret, err := totpsvc.Generate("cassonic", "boss")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	rawCodes, hashedCodes, err := totpsvc.GenerateBackupCodes()
	if err != nil {
		t.Fatalf("GenerateBackupCodes: %v", err)
	}
	backupJSON := marshalForTest(t, hashedCodes)
	encrypted := encryptForTest(t, h, secret.Secret)
	adminStore.getTOTPSecretResult = &model.TOTPSecret{
		UserType: totpUserType, UserID: 1, Secret: encrypted, Enabled: true, BackupCodes: backupJSON,
	}

	form := url.Values{"code": {rawCodes[0]}}
	r := httptest.NewRequest(http.MethodPost, "/boss/profile/security/totp/disable", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = withAuthedAdminRequest(r, 1, "boss")
	w := httptest.NewRecorder()
	h.DisableTOTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Location"), "flash=") {
		t.Fatalf("expected success flash redirect, got %q", w.Header().Get("Location"))
	}
	if adminStore.lastBackupCodesJSON == "" {
		t.Fatalf("expected consumed backup code to update stored codes")
	}
}

func TestRegenerateBackupCodes_ValidCode_ShowsNewCodes(t *testing.T) {
	admin := &model.Admin{ID: 1, Username: "boss", Enabled: true, TOTPEnabled: true}
	h, adminStore := adminHandlerWithEncryptionKey(t, admin)

	secret, err := totpsvc.Generate("cassonic", "boss")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	encrypted := encryptForTest(t, h, secret.Secret)
	adminStore.getTOTPSecretResult = &model.TOTPSecret{
		UserType: totpUserType, UserID: 1, Secret: encrypted, Enabled: true,
	}

	code, err := currentTOTPCode(secret.Secret)
	if err != nil {
		t.Fatalf("currentTOTPCode: %v", err)
	}
	form := url.Values{"code": {code}}
	r := httptest.NewRequest(http.MethodPost, "/boss/profile/security/totp/backup-codes/regenerate", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = withAuthedAdminRequest(r, 1, "boss")
	w := httptest.NewRecorder()
	h.RegenerateBackupCodes(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Backup Codes") {
		t.Fatalf("expected new backup codes shown, got: %s", w.Body.String())
	}
	if adminStore.lastBackupCodesJSON == "" {
		t.Fatalf("expected backup codes to be replaced")
	}
}

// currentTOTPCode returns the currently valid 6-digit code for secret,
// mirroring what an authenticator app would show.
func currentTOTPCode(secret string) (string, error) {
	return pquernatotp.GenerateCodeCustom(secret, time.Now(), pquernatotp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    pquernaotp.DigitsSix,
		Algorithm: pquernaotp.AlgorithmSHA1,
	})
}

// encryptForTest encrypts a plaintext TOTP secret with the handler's
// configured encryption key, mirroring encryptionKey() in security.go.
func encryptForTest(t *testing.T, h *Handler, plaintext string) string {
	t.Helper()
	encrypted, err := crypto.Encrypt(h.encryptionKey(), plaintext)
	if err != nil {
		t.Fatalf("crypto.Encrypt: %v", err)
	}
	return encrypted
}

// marshalForTest JSON-marshals v, failing the test on error.
func marshalForTest(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(b)
}
