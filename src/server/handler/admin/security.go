package admin

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"net/http"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/service/crypto"
	totpsvc "github.com/local/cassonic/src/server/service/totp"
)

// totpUserType identifies the owner of a totp_secrets row. Only "admin" is
// used today; the column exists so a future user-facing 2FA feature (PART
// 34 Multi-User, not yet activated) can share the same table.
const totpUserType = "admin"

// securityData carries the TOTP status/enrollment/backup-codes views for the
// /{admin_username}/profile/security page. Exactly one of the "mode" flags
// is set per render.
type securityData struct {
	Flash      string
	FlashError bool

	// Enabled reports whether TOTP is currently active for this admin.
	Enabled bool

	// Enrolling is true while showing the QR code + manual key + code-entry
	// confirmation form for a not-yet-persisted secret.
	Enrolling bool
	Secret    string
	QRCodePNG string
	URI       string

	// BackupCodes is set exactly once, immediately after a successful
	// enable or regenerate, to display the raw recovery codes. Never
	// re-derivable afterward — only the hashes are stored.
	BackupCodes []string
}

// encryptionKey derives the AES-256-GCM key used to encrypt/decrypt TOTP
// secrets at rest, from server.security.encryption_key (AI.md PART 11 / PART
// 17 "TOTP Two-Factor Authentication").
func (h *Handler) encryptionKey() []byte {
	return crypto.DeriveKey([]byte(h.cfg.Security.EncryptionKey))
}

// Security renders the authenticated admin's TOTP status page.
func (h *Handler) Security(w http.ResponseWriter, r *http.Request) {
	admin := h.currentAdmin(w, r)
	if admin == nil {
		return
	}
	d := securityData{Enabled: admin.TOTPEnabled}
	if flash := r.URL.Query().Get("flash"); flash != "" {
		d.Flash = flash
	}
	if flashErr := r.URL.Query().Get("error"); flashErr != "" {
		d.Flash = flashErr
		d.FlashError = true
	}
	h.render(w, r, "security.html", "Security — Admin", "security", d)
}

// EnrollTOTPStart generates a fresh, unconfirmed TOTP secret and renders the
// QR code + manual key + confirmation form. Nothing is persisted here — the
// secret is threaded through the confirmation form's hidden field, following
// this codebase's stateless-wizard convention (mirrors setup.go), until the
// admin proves possession with a valid 6-digit code.
func (h *Handler) EnrollTOTPStart(w http.ResponseWriter, r *http.Request) {
	admin := h.currentAdmin(w, r)
	if admin == nil {
		return
	}
	if admin.TOTPEnabled {
		h.redirectSecurityError(w, r, "two-factor authentication is already enabled")
		return
	}

	issuer := h.cfg.Server.AppName
	if issuer == "" {
		issuer = "cassonic"
	}
	enrollment, err := totpsvc.Generate(issuer, admin.Username)
	if err != nil {
		h.redirectSecurityError(w, r, "failed to generate TOTP secret")
		return
	}

	h.render(w, r, "security.html", "Security — Admin", "security", securityData{
		Enrolling: true,
		Secret:    enrollment.Secret,
		QRCodePNG: enrollment.QRCodePNGBase64,
		URI:       enrollment.URI,
	})
}

// EnrollTOTPConfirm validates the submitted 6-digit code against the
// not-yet-persisted secret carried in the hidden form field. On success, the
// secret is encrypted and persisted with enabled=1, ten backup codes are
// generated, and the raw codes are shown once.
func (h *Handler) EnrollTOTPConfirm(w http.ResponseWriter, r *http.Request) {
	admin := h.currentAdmin(w, r)
	if admin == nil {
		return
	}
	if err := r.ParseForm(); err != nil {
		h.redirectSecurityError(w, r, "invalid form submission")
		return
	}

	secret := strings.TrimSpace(r.PostForm.Get("secret"))
	code := strings.TrimSpace(r.PostForm.Get("code"))
	if secret == "" || !totpsvc.Validate(secret, code) {
		h.render(w, r, "security.html", "Security — Admin", "security", securityData{
			Enrolling:  true,
			Secret:     secret,
			FlashError: true,
			Flash:      "invalid code — please try again",
		})
		return
	}

	encrypted, err := crypto.Encrypt(h.encryptionKey(), secret)
	if err != nil {
		h.redirectSecurityError(w, r, "failed to encrypt TOTP secret")
		return
	}

	rawCodes, hashedCodes, err := totpsvc.GenerateBackupCodes()
	if err != nil {
		h.redirectSecurityError(w, r, "failed to generate backup codes")
		return
	}
	backupJSON, err := json.Marshal(hashedCodes)
	if err != nil {
		h.redirectSecurityError(w, r, "failed to store backup codes")
		return
	}

	if err := h.db.Admin.EnableTOTP(r.Context(), &model.TOTPSecret{
		UserType:    totpUserType,
		UserID:      admin.ID,
		Secret:      encrypted,
		BackupCodes: string(backupJSON),
	}); err != nil {
		h.redirectSecurityError(w, r, "failed to save TOTP secret")
		return
	}

	admin.TOTPEnabled = true
	if err := h.db.Admin.UpdateAdmin(r.Context(), admin); err != nil {
		h.redirectSecurityError(w, r, "failed to update admin account")
		return
	}

	h.appendSecurityAudit(r, admin.ID, "security.totp_enabled")

	h.render(w, r, "security.html", "Security — Admin", "security", securityData{
		Enabled:     true,
		BackupCodes: rawCodes,
		Flash:       "two-factor authentication enabled — save your backup codes now, they will not be shown again",
	})
}

// verifyTOTPOrBackupCode reports whether code is either a currently valid
// TOTP code, or an unused backup code — and, if it matched a backup code,
// consumes it (removes it from the stored hash list) so it cannot be reused.
func (h *Handler) verifyTOTPOrBackupCode(r *http.Request, admin *model.Admin, code string) (bool, error) {
	secretRow, err := h.db.Admin.GetTOTPSecret(r.Context(), totpUserType, admin.ID)
	if err != nil || secretRow == nil || !secretRow.Enabled {
		return false, err
	}

	decrypted, err := crypto.Decrypt(h.encryptionKey(), secretRow.Secret)
	if err == nil && totpsvc.Validate(decrypted, code) {
		_ = h.db.Admin.TouchTOTPLastUsed(r.Context(), totpUserType, admin.ID)
		return true, nil
	}

	var hashes []string
	if secretRow.BackupCodes != "" {
		if err := json.Unmarshal([]byte(secretRow.BackupCodes), &hashes); err != nil {
			return false, nil
		}
	}
	target := totpsvc.HashBackupCode(strings.TrimSpace(code))
	for i, hash := range hashes {
		if hash == target {
			remaining := append(hashes[:i], hashes[i+1:]...)
			remainingJSON, err := json.Marshal(remaining)
			if err != nil {
				return false, err
			}
			if err := h.db.Admin.UpdateTOTPBackupCodes(r.Context(), totpUserType, admin.ID, string(remainingJSON)); err != nil {
				return false, err
			}
			_ = h.db.Admin.TouchTOTPLastUsed(r.Context(), totpUserType, admin.ID)
			return true, nil
		}
	}
	return false, nil
}

// DisableTOTP requires a valid current TOTP code or unused backup code (AI.md
// PART 17 "Disable" — "Requires current TOTP code or recovery key").
func (h *Handler) DisableTOTP(w http.ResponseWriter, r *http.Request) {
	admin := h.currentAdmin(w, r)
	if admin == nil {
		return
	}
	if err := r.ParseForm(); err != nil {
		h.redirectSecurityError(w, r, "invalid form submission")
		return
	}

	code := strings.TrimSpace(r.PostForm.Get("code"))
	ok, err := h.verifyTOTPOrBackupCode(r, admin, code)
	if err != nil {
		h.redirectSecurityError(w, r, "failed to verify code")
		return
	}
	if !ok {
		h.redirectSecurityError(w, r, "invalid code — two-factor authentication was not disabled")
		return
	}

	if err := h.db.Admin.DisableTOTP(r.Context(), totpUserType, admin.ID); err != nil {
		h.redirectSecurityError(w, r, "failed to disable two-factor authentication")
		return
	}
	admin.TOTPEnabled = false
	if err := h.db.Admin.UpdateAdmin(r.Context(), admin); err != nil {
		h.redirectSecurityError(w, r, "failed to update admin account")
		return
	}

	h.appendSecurityAudit(r, admin.ID, "security.totp_disabled")

	http.Redirect(w, r, h.selfPath(r, "profile/security", "flash="+url.QueryEscape("two-factor authentication disabled")), http.StatusSeeOther)
}

// RegenerateBackupCodes requires a valid current TOTP code (AI.md PART 17
// "Regenerate" — invalidates old backup codes) and shows the new codes once.
func (h *Handler) RegenerateBackupCodes(w http.ResponseWriter, r *http.Request) {
	admin := h.currentAdmin(w, r)
	if admin == nil {
		return
	}
	if err := r.ParseForm(); err != nil {
		h.redirectSecurityError(w, r, "invalid form submission")
		return
	}

	secretRow, err := h.db.Admin.GetTOTPSecret(r.Context(), totpUserType, admin.ID)
	if err != nil || secretRow == nil || !secretRow.Enabled {
		h.redirectSecurityError(w, r, "two-factor authentication is not enabled")
		return
	}
	decrypted, err := crypto.Decrypt(h.encryptionKey(), secretRow.Secret)
	code := strings.TrimSpace(r.PostForm.Get("code"))
	if err != nil || !totpsvc.Validate(decrypted, code) {
		h.redirectSecurityError(w, r, "invalid code — backup codes were not regenerated")
		return
	}

	rawCodes, hashedCodes, err := totpsvc.GenerateBackupCodes()
	if err != nil {
		h.redirectSecurityError(w, r, "failed to generate backup codes")
		return
	}
	backupJSON, err := json.Marshal(hashedCodes)
	if err != nil {
		h.redirectSecurityError(w, r, "failed to store backup codes")
		return
	}
	if err := h.db.Admin.UpdateTOTPBackupCodes(r.Context(), totpUserType, admin.ID, string(backupJSON)); err != nil {
		h.redirectSecurityError(w, r, "failed to store backup codes")
		return
	}

	h.appendSecurityAudit(r, admin.ID, "security.totp_backup_codes_regenerated")

	h.render(w, r, "security.html", "Security — Admin", "security", securityData{
		Enabled:     true,
		BackupCodes: rawCodes,
		Flash:       "backup codes regenerated — save them now, they will not be shown again",
	})
}

// appendSecurityAudit records a TOTP security event for the given admin.
func (h *Handler) appendSecurityAudit(r *http.Request, adminID int64, action string) {
	_ = h.db.Admin.AppendAuditEntry(r.Context(), &model.AuditEntry{
		Level:      "security",
		Category:   "auth",
		Action:     action,
		ActorType:  "admin",
		ActorID:    strconv.FormatInt(adminID, 10),
		ActorIP:    r.RemoteAddr,
		TargetType: "admin",
		TargetID:   strconv.FormatInt(adminID, 10),
		Success:    true,
	})
}

func (h *Handler) redirectSecurityError(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, h.selfPath(r, "profile/security", "error="+url.QueryEscape(msg)), http.StatusSeeOther)
}
