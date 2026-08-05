package admin

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// currentAdmin loads the model.Admin record for the authenticated request,
// using the username the requireAdmin middleware already verified. Returns
// nil and writes an error response if the record cannot be loaded (should
// only happen if the admin was deleted mid-session).
func (h *Handler) currentAdmin(w http.ResponseWriter, r *http.Request) *model.Admin {
	a := mw.AdminFromContext(r.Context())
	if a == nil {
		http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
		return nil
	}
	admin, err := h.db.Admin.GetAdmin(r.Context(), a.ID)
	if err != nil || admin == nil {
		http.Error(w, "admin account not found", http.StatusInternalServerError)
		return nil
	}
	return admin
}

// SelfRoot redirects /{admin_username}/ to the profile page (AI.md PART 17
// "Admin Route Hierarchy" — the bare self-account root has no page of its
// own).
func (h *Handler) SelfRoot(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "admin_username")
	http.Redirect(w, r, h.basePath()+"/"+username+"/profile", http.StatusSeeOther)
}

// profileData carries the account-info and password-change form for the
// admin's own profile page.
type profileData struct {
	Username   string
	Email      string
	Role       string
	Source     string
	Flash      string
	FlashError bool
}

// Profile renders the authenticated admin's own account page.
func (h *Handler) Profile(w http.ResponseWriter, r *http.Request) {
	admin := h.currentAdmin(w, r)
	if admin == nil {
		return
	}
	d := profileData{
		Username: admin.Username,
		Email:    admin.Email,
		Role:     admin.Role,
		Source:   admin.Source,
	}
	if flash := r.URL.Query().Get("flash"); flash != "" {
		d.Flash = flash
	}
	if flashErr := r.URL.Query().Get("error"); flashErr != "" {
		d.Flash = flashErr
		d.FlashError = true
	}
	h.render(w, r, "profile.html", "Profile — Admin", "profile", d)
}

// selfPath builds an absolute redirect target under the current admin's
// self-account route, e.g. selfPath(r, "profile", "flash=saved").
func (h *Handler) selfPath(r *http.Request, page, query string) string {
	username := chi.URLParam(r, "admin_username")
	p := h.basePath() + "/" + username + "/" + page
	if query != "" {
		p += "?" + query
	}
	return p
}

// SaveProfile updates the admin's email and, if a new password was
// submitted, verifies the current password and rotates the hash. Externally
// synced admins (Source != "local") cannot change their password here — it
// must be changed at the identity provider (AI.md PART 17 "Server Admin
// Accounts").
func (h *Handler) SaveProfile(w http.ResponseWriter, r *http.Request) {
	admin := h.currentAdmin(w, r)
	if admin == nil {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, h.selfPath(r, "profile", "error="+url.QueryEscape("invalid form submission")), http.StatusSeeOther)
		return
	}

	email := strings.TrimSpace(r.PostForm.Get("email"))
	if email != "" {
		admin.Email = email
	}

	currentPassword := r.PostForm.Get("current_password")
	newPassword := r.PostForm.Get("new_password")
	confirmPassword := r.PostForm.Get("confirm_password")

	if newPassword != "" {
		if admin.Source != "local" {
			h.redirectProfileError(w, r, "password managed externally; cannot be changed here")
			return
		}
		ok, err := store.VerifyPassword(admin.PasswordHash, currentPassword)
		if err != nil || !ok {
			h.redirectProfileError(w, r, "current password is incorrect")
			return
		}
		if newPassword != confirmPassword {
			h.redirectProfileError(w, r, "new password and confirmation do not match")
			return
		}
		hash, err := store.HashPassword(newPassword)
		if err != nil {
			h.redirectProfileError(w, r, "failed to hash new password")
			return
		}
		admin.PasswordHash = hash
	}

	if err := h.db.Admin.UpdateAdmin(r.Context(), admin); err != nil {
		h.redirectProfileError(w, r, "failed to save profile")
		return
	}

	_ = h.db.Admin.AppendAuditEntry(r.Context(), &model.AuditEntry{
		Level:      "info",
		Category:   "admin",
		Action:     "profile_update",
		ActorType:  "admin",
		ActorID:    strconv.FormatInt(admin.ID, 10),
		ActorIP:    r.RemoteAddr,
		TargetType: "admin",
		TargetID:   strconv.FormatInt(admin.ID, 10),
		Success:    true,
	})

	http.Redirect(w, r, h.selfPath(r, "profile", "flash="+url.QueryEscape("profile saved")), http.StatusSeeOther)
}

func (h *Handler) redirectProfileError(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, h.selfPath(r, "profile", "error="+url.QueryEscape(msg)), http.StatusSeeOther)
}

// preferencesData carries the WebUI preference form for the admin's own
// preferences page.
type preferencesData struct {
	*model.AdminPreferences
	Flash      string
	FlashError bool
}

// Preferences renders the authenticated admin's WebUI preferences page.
func (h *Handler) Preferences(w http.ResponseWriter, r *http.Request) {
	admin := h.currentAdmin(w, r)
	if admin == nil {
		return
	}
	prefs, err := h.db.Admin.GetAdminPreferences(r.Context(), admin.ID)
	if err != nil {
		http.Error(w, "failed to load preferences: "+err.Error(), http.StatusInternalServerError)
		return
	}
	d := preferencesData{AdminPreferences: prefs}
	if flash := r.URL.Query().Get("flash"); flash != "" {
		d.Flash = flash
	}
	if flashErr := r.URL.Query().Get("error"); flashErr != "" {
		d.Flash = flashErr
		d.FlashError = true
	}
	h.render(w, r, "preferences.html", "Preferences — Admin", "preferences", d)
}

// SavePreferences persists the submitted preference form. EmailSecurity
// cannot be disabled (AI.md PART 12 admin_preferences — security
// notifications are mandatory), so it is always forced true regardless of
// the submitted value.
func (h *Handler) SavePreferences(w http.ResponseWriter, r *http.Request) {
	admin := h.currentAdmin(w, r)
	if admin == nil {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, h.selfPath(r, "preferences", "error="+url.QueryEscape("invalid form submission")), http.StatusSeeOther)
		return
	}

	prefs, err := h.db.Admin.GetAdminPreferences(r.Context(), admin.ID)
	if err != nil {
		http.Redirect(w, r, h.selfPath(r, "preferences", "error="+url.QueryEscape("failed to load preferences")), http.StatusSeeOther)
		return
	}

	form := r.PostForm
	prefs.Theme = strings.TrimSpace(form.Get("theme"))
	prefs.FontSize = strings.TrimSpace(form.Get("font_size"))
	prefs.ReduceMotion = form.Get("reduce_motion") == "true"
	prefs.DateFormat = strings.TrimSpace(form.Get("date_format"))
	prefs.TimeFormat = strings.TrimSpace(form.Get("time_format"))
	prefs.EmailSecurity = true
	prefs.EmailServer = form.Get("email_server") == "true"
	prefs.EmailBackups = form.Get("email_backups") == "true"
	prefs.EmailUsers = form.Get("email_users") == "true"

	if err := h.db.Admin.UpdateAdminPreferences(r.Context(), prefs); err != nil {
		http.Redirect(w, r, h.selfPath(r, "preferences", "error="+url.QueryEscape("failed to save preferences")), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, h.selfPath(r, "preferences", "flash="+url.QueryEscape("preferences saved")), http.StatusSeeOther)
}
