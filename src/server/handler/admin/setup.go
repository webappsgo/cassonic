package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/local/cassonic/src/config"
	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// setupSessionCookieName holds the still-unconsumed raw setup token after a
// successful token-entry (AI.md PART 17 "Setup Flow" steps 3-4). Its hash is
// re-validated against setup_token.token_hash on every /config/setup request,
// so no separate wizard-progress session store is needed — the token itself
// doubles as the setup session credential until it is consumed in step 6.
const setupSessionCookieName = "cassonic_setup"

// setupTokenViewData carries the token-entry page's error state.
type setupTokenViewData struct {
	Error string
}

// wizardViewData carries the setup wizard's current step and every value
// collected so far, threaded through hidden form fields (AI.md PART 17
// "Setup Wizard Steps"). The wizard is intentionally stateless server-side:
// no wizard-progress table exists, so every step's form re-submits all
// prior steps' values. Raw secrets (password, API token) are hashed
// immediately in the step that collects them and never carried forward in
// plaintext past their own step's render.
type wizardViewData struct {
	Step  int
	Error string

	// Step 1 (display + carry-forward).
	Username     string
	PasswordHash string

	// Step 2 (APITokenRaw is shown once, on step 2's own render, then
	// discarded — only its hash is carried forward).
	APITokenRaw  string
	APITokenHash string

	// Step 3.
	AppName  string
	Domain   string
	Mode     string
	Timezone string

	// Step 4.
	BackupEncryptionPassword string
	Enable2FA                bool

	// Step 5.
	HTTPSReviewed bool
}

// setupInProgress reports whether the request carries a valid, unconsumed
// setup-session cookie (i.e. the operator already entered the correct setup
// token at "/" and has not yet completed the wizard).
func (h *Handler) setupInProgress(r *http.Request) bool {
	cookie, err := r.Cookie(setupSessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	tok, err := h.db.Admin.GetSetupToken(r.Context())
	if err != nil || tok == nil || tok.Used {
		return false
	}
	hash := store.HashSetupToken(cookie.Value)
	return subtle.ConstantTimeCompare([]byte(hash), []byte(tok.TokenHash)) == 1
}

// Root serves the admin panel entry point (AI.md PART 17 "Setup Flow" step
// 2). An authenticated admin sees the dashboard; before setup is complete,
// unauthenticated visitors see the one-time setup token entry form; after
// setup is complete, unauthenticated visitors are redirected to /login.
func (h *Handler) Root(w http.ResponseWriter, r *http.Request) {
	if a := mw.AdminFromContext(r.Context()); a != nil {
		h.Dashboard(w, r)
		return
	}
	if admin := h.sessionAdmin(r); admin != nil {
		ctx := mw.WithAdmin(r.Context(), admin)
		h.Dashboard(w, r.WithContext(ctx))
		return
	}

	count, err := h.db.Admin.CountAdmins(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if count > 0 {
		http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
		return
	}
	h.renderSetupTokenEntry(w, r, "")
}

func (h *Handler) renderSetupTokenEntry(w http.ResponseWriter, r *http.Request, errMsg string) {
	h.render(w, r, "setup_token.html", "Setup — cassonic", "", setupTokenViewData{Error: errMsg})
}

// RootPost handles the one-time setup token submission (AI.md PART 17
// "Setup Flow" step 3). On success it sets the setup-session cookie and
// redirects to the wizard; the raw token is never logged.
func (h *Handler) RootPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderSetupTokenEntry(w, r, "invalid form submission")
		return
	}

	count, err := h.db.Admin.CountAdmins(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if count > 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	submitted := strings.TrimSpace(r.PostForm.Get("token"))
	tok, err := h.db.Admin.GetSetupToken(r.Context())
	if err != nil || tok == nil || tok.Used {
		h.renderSetupTokenEntry(w, r, "setup token is invalid or has already been used")
		return
	}

	submittedHash := store.HashSetupToken(submitted)
	if subtle.ConstantTimeCompare([]byte(submittedHash), []byte(tok.TokenHash)) != 1 {
		h.renderSetupTokenEntry(w, r, "incorrect setup token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     setupSessionCookieName,
		Value:    submitted,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, h.basePath()+"/config/setup", http.StatusSeeOther)
}

// SetupWizard renders step 1 of the setup wizard (GET requests only ever
// show step 1 — later steps are only ever reached by a POST from the
// previous step, keeping the stateless hidden-field chain intact).
func (h *Handler) SetupWizard(w http.ResponseWriter, r *http.Request) {
	if !h.setupInProgress(r) {
		http.Redirect(w, r, h.basePath()+"/", http.StatusSeeOther)
		return
	}
	h.renderWizardStep(w, r, wizardViewData{Step: 1, Username: "administrator"})
}

func (h *Handler) renderWizardStep(w http.ResponseWriter, r *http.Request, data wizardViewData) {
	h.render(w, r, "setup_wizard.html", "Setup — cassonic", "", data)
}

// SetupWizardPost processes one step of the setup wizard and renders the
// next (AI.md PART 17 "Setup Wizard Steps"). Every field collected by prior
// steps is threaded through as hidden form fields since no server-side
// wizard-progress state exists.
func (h *Handler) SetupWizardPost(w http.ResponseWriter, r *http.Request) {
	if !h.setupInProgress(r) {
		http.Redirect(w, r, h.basePath()+"/", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderWizardStep(w, r, wizardViewData{Step: 1, Error: "invalid form submission", Username: "administrator"})
		return
	}

	form := r.PostForm
	step, _ := strconv.Atoi(form.Get("step"))

	// carried collects every value threaded through hidden fields so far,
	// regardless of which step we are about to process.
	carried := wizardViewData{
		Username:                 form.Get("username"),
		PasswordHash:             form.Get("password_hash"),
		APITokenHash:             form.Get("api_token_hash"),
		AppName:                  form.Get("app_name"),
		Domain:                   form.Get("domain"),
		Mode:                     form.Get("mode"),
		Timezone:                 form.Get("timezone"),
		BackupEncryptionPassword: form.Get("backup_encryption_password"),
		Enable2FA:                form.Get("enable_2fa") == "on",
		HTTPSReviewed:            form.Get("https_reviewed") == "on",
	}

	switch step {
	case 1:
		h.wizardStep1(w, r, form, carried)
	case 2:
		carried.Step = 3
		carried.AppName = h.cfg.Server.AppName
		carried.Domain = strings.Join(h.cfg.Server.Domain, ",")
		carried.Mode = h.cfg.Server.Mode
		carried.Timezone = h.cfg.Server.Timezone
		h.renderWizardStep(w, r, carried)
	case 3:
		h.wizardStep3(w, r, form, carried)
	case 4:
		h.wizardStep4(w, r, form, carried)
	case 5:
		h.wizardStep5(w, r, form, carried)
	case 6:
		h.completeSetup(w, r, carried)
	default:
		h.renderWizardStep(w, r, wizardViewData{Step: 1, Error: "invalid step", Username: "administrator"})
	}
}

// wizardStep1 validates the admin account fields (Step 1: Create Admin
// Account) and advances to Step 2. The username blocklist note in AI.md PART
// 17 ("username blocklist does NOT apply to admin") is a no-op in this
// codebase — no regular-username blocklist exists to exempt from.
func (h *Handler) wizardStep1(w http.ResponseWriter, r *http.Request, form map[string][]string, carried wizardViewData) {
	username := strings.TrimSpace(oneOf(form, "username"))
	if username == "" {
		username = "administrator"
	}
	password := oneOf(form, "password")
	confirm := oneOf(form, "password_confirm")

	if password == "" {
		raw, err := randomPassword(20)
		if err != nil {
			h.renderWizardStep(w, r, wizardViewData{Step: 1, Error: "server error generating password", Username: username})
			return
		}
		password = raw
	} else if password != confirm {
		h.renderWizardStep(w, r, wizardViewData{Step: 1, Error: "passwords do not match", Username: username})
		return
	}

	hash, err := store.HashPassword(password)
	if err != nil {
		h.renderWizardStep(w, r, wizardViewData{Step: 1, Error: "server error hashing password", Username: username})
		return
	}

	rawToken, tokenHash, err := generateAdminAPIToken()
	if err != nil {
		h.renderWizardStep(w, r, wizardViewData{Step: 1, Error: "server error generating API token", Username: username})
		return
	}

	next := carried
	next.Step = 2
	next.Username = username
	next.PasswordHash = hash
	next.APITokenRaw = rawToken
	next.APITokenHash = tokenHash
	h.renderWizardStep(w, r, next)
}

func oneOf(form map[string][]string, key string) string {
	if v, ok := form[key]; ok && len(v) > 0 {
		return strings.TrimSpace(v[0])
	}
	return ""
}

// wizardStep3 collects Server Configuration (Step 3) and advances to Step 4.
func (h *Handler) wizardStep3(w http.ResponseWriter, r *http.Request, form map[string][]string, carried wizardViewData) {
	next := carried
	next.Step = 4
	next.AppName = strings.TrimSpace(oneOf(form, "app_name"))
	if next.AppName == "" {
		next.AppName = "cassonic"
	}
	next.Domain = strings.TrimSpace(oneOf(form, "domain"))
	next.Mode = oneOf(form, "mode")
	if next.Mode != "production" && next.Mode != "development" {
		next.Mode = "production"
	}
	next.Timezone = strings.TrimSpace(oneOf(form, "timezone"))
	h.renderWizardStep(w, r, next)
}

// wizardStep4 collects Security Settings (Step 4) and advances to Step 5.
func (h *Handler) wizardStep4(w http.ResponseWriter, r *http.Request, form map[string][]string, carried wizardViewData) {
	next := carried
	next.Step = 5
	next.BackupEncryptionPassword = oneOf(form, "backup_encryption_password")
	next.Enable2FA = oneOf(form, "enable_2fa") == "on"
	h.renderWizardStep(w, r, next)
}

// wizardStep5 collects Optional Services (Step 5 — HTTPS/certificate review
// only; the Multi-User toggle from AI.md PART 17 is intentionally absent
// because PART 34 is not activated in this project, per
// .claude/rules/optional-rules.md) and advances to Step 6.
func (h *Handler) wizardStep5(w http.ResponseWriter, r *http.Request, form map[string][]string, carried wizardViewData) {
	next := carried
	next.Step = 6
	next.HTTPSReviewed = oneOf(form, "https_reviewed") == "on"
	h.renderWizardStep(w, r, next)
}

// completeSetup finalizes the wizard (Step 6: Complete): creates the admin
// account, persists server.yml, marks the setup token consumed, logs the new
// admin in, and redirects to the admin panel root (AI.md PART 17 "Setup
// Wizard Steps" step 6).
func (h *Handler) completeSetup(w http.ResponseWriter, r *http.Request, data wizardViewData) {
	ctx := r.Context()

	if data.Username == "" || data.PasswordHash == "" {
		h.renderWizardStep(w, r, wizardViewData{Step: 1, Error: "setup session expired, please start over", Username: "administrator"})
		return
	}

	admin := &model.Admin{
		Username:     data.Username,
		PasswordHash: data.PasswordHash,
		Role:         "superadmin",
		Enabled:      true,
		APITokenHash: data.APITokenHash,
		Source:       "local",
		TOTPEnabled:  data.Enable2FA,
	}
	adminID, err := h.db.Admin.CreateAdmin(ctx, admin)
	if err != nil {
		h.renderWizardStep(w, r, wizardViewData{Step: 1, Error: "failed to create admin account: " + err.Error(), Username: data.Username})
		return
	}

	next := *h.cfg
	next.Server.AppName = data.AppName
	if data.Domain != "" {
		next.Server.Domain = splitCSV(data.Domain)
	}
	next.Server.Mode = data.Mode
	next.Server.Timezone = data.Timezone
	next.Backup.EncryptionPassword = data.BackupEncryptionPassword

	if err := next.Validate(); err != nil {
		h.renderWizardStep(w, r, wizardViewData{Step: 3, Error: "invalid configuration: " + err.Error(),
			Username: data.Username, PasswordHash: data.PasswordHash, APITokenHash: data.APITokenHash,
			AppName: data.AppName, Domain: data.Domain, Mode: data.Mode, Timezone: data.Timezone})
		return
	}
	if err := config.Save(&next, h.cfgPath); err != nil {
		h.renderWizardStep(w, r, wizardViewData{Step: 3, Error: "failed to write server.yml: " + err.Error(),
			Username: data.Username, PasswordHash: data.PasswordHash, APITokenHash: data.APITokenHash,
			AppName: data.AppName, Domain: data.Domain, Mode: data.Mode, Timezone: data.Timezone})
		return
	}
	*h.cfg = next

	_ = h.db.Admin.ConsumeSetupToken(ctx)

	raw, sessTokenHash, err := generateAdminSessionToken()
	if err == nil {
		sess := &model.AdminSession{
			TokenHash: sessTokenHash,
			AdminID:   adminID,
			IP:        r.RemoteAddr,
			UserAgent: r.UserAgent(),
			ExpiresAt: time.Now().UTC().Add(time.Duration(h.cfg.Auth.SessionDuration) * time.Hour),
		}
		if err := h.db.Admin.CreateAdminSession(ctx, sess); err == nil {
			http.SetCookie(w, &http.Cookie{
				Name:     mw.AdminSessionCookieName,
				Value:    raw,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
			})
			mw.ResetCSRFCookie(w, r.TLS != nil)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     setupSessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	_ = h.db.Admin.AppendAuditEntry(ctx, &model.AuditEntry{
		Level:      "info",
		Category:   "auth",
		Action:     "setup_complete",
		ActorType:  "admin",
		ActorID:    strconv.FormatInt(adminID, 10),
		ActorIP:    r.RemoteAddr,
		TargetType: "admin",
		TargetID:   strconv.FormatInt(adminID, 10),
		Success:    true,
	})

	http.Redirect(w, r, h.basePath()+"/", http.StatusSeeOther)
}

// generateAdminAPIToken creates the admin's API token (AI.md PART 17 "Setup
// Wizard Steps" Step 2), prefixed "adm_" per the reserved prefix documented
// on model.Admin.APITokenHash. raw is shown to the operator exactly once;
// only hash is ever persisted.
func generateAdminAPIToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("api token: rand: %w", err)
	}
	raw = "adm_" + hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return raw, hash, nil
}

// generateAdminSessionToken mirrors web.go's generateSessionToken: 32 random
// bytes hex-encoded as the raw cookie value, SHA-256 hex as the stored hash.
func generateAdminSessionToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("session token: rand: %w", err)
	}
	raw = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return raw, hash, nil
}

// randomPassword generates a random alphanumeric password of length n for
// the "Password: Random (generated)" default in Step 1.
func randomPassword(n int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	b := make([]byte, n)
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("password: rand: %w", err)
	}
	for i, v := range raw {
		b[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(b), nil
}
