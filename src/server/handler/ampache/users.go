package ampache

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"

	"golang.org/x/crypto/argon2"

	"github.com/local/cassonic/src/server/model"
)

// user returns the user identified by the username parameter.
// Admins may fetch any user; regular users may only fetch themselves.
func (h *Handler) user(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	username := param(r, "username")
	if username == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: username"))
		return
	}

	caller, err := h.db.Users.GetUser(r.Context(), session.UserID)
	if err != nil || caller == nil {
		respond(w, r, isJSON, errResp(4700, "Access denied"))
		return
	}

	if !caller.IsAdmin && caller.Username != username {
		respond(w, r, isJSON, errResp(4742, "Access denied: admin required to view other users"))
		return
	}

	target, err := h.db.Users.GetUserByUsername(r.Context(), username)
	if err != nil || target == nil {
		respond(w, r, isJSON, errResp(4704, "Not found"))
		return
	}

	respond(w, r, isJSON, userToAmp(target))
}

// users returns all users. Admin only.
func (h *Handler) users(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	list, err := h.db.Users.ListUsers(r.Context())
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	result := make([]AmpUser, 0, len(list))
	for _, u := range list {
		result = append(result, userToAmp(u))
	}
	respond(w, r, isJSON, okResp("user", result))
}

// userCreate creates a new user account. Admin only.
func (h *Handler) userCreate(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	username := param(r, "username")
	password := param(r, "password")
	email := param(r, "email")

	if username == "" || password == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: username and password are required"))
		return
	}

	isAdmin := parseIntParam(r, "group", 1) == 0
	disabled := parseIntParam(r, "disable", 0) == 1

	hash, err := hashPasswordArgon2id(password)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to hash password: "+err.Error()))
		return
	}

	u := &model.User{
		Username:     username,
		Email:        email,
		DisplayName:  param(r, "fullname"),
		PasswordHash: hash,
		IsAdmin:      isAdmin,
		IsEnabled:    !disabled,
		CanDownload:  true,
		Theme:        "dark",
		Language:     "en",
	}

	id, err := h.db.Users.CreateUser(r.Context(), u)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to create user: "+err.Error()))
		return
	}

	u.ID = id
	respond(w, r, isJSON, userToAmp(u))
}

// userEdit modifies an existing user account. Admin only.
func (h *Handler) userEdit(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	username := param(r, "username")
	if username == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: username"))
		return
	}

	u, err := h.db.Users.GetUserByUsername(r.Context(), username)
	if err != nil || u == nil {
		respond(w, r, isJSON, errResp(4704, "User not found"))
		return
	}

	if v := param(r, "email"); v != "" {
		u.Email = v
	}
	if v := param(r, "fullname"); v != "" {
		u.DisplayName = v
	}
	if v := parseIntParam(r, "group", -1); v != -1 {
		u.IsAdmin = v == 0
	}
	if v := parseIntParam(r, "disable", -1); v != -1 {
		u.IsEnabled = v == 0
	}

	if err := h.db.Users.UpdateUser(r.Context(), u); err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to update user: "+err.Error()))
		return
	}

	respond(w, r, isJSON, userToAmp(u))
}

// userDelete permanently removes a user account. Admin only.
func (h *Handler) userDelete(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	username := param(r, "username")
	if username == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: username"))
		return
	}

	u, err := h.db.Users.GetUserByUsername(r.Context(), username)
	if err != nil || u == nil {
		respond(w, r, isJSON, errResp(4704, "User not found"))
		return
	}

	if err := h.db.Users.DeleteUser(r.Context(), u.ID); err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to delete user: "+err.Error()))
		return
	}

	respond(w, r, isJSON, okResp("success", "user deleted"))
}

// userPreferences returns preferences for the authenticated user.
func (h *Handler) userPreferences(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	u, err := h.db.Users.GetUser(r.Context(), session.UserID)
	if err != nil || u == nil {
		respond(w, r, isJSON, errResp(4704, "User not found"))
		return
	}

	prefs := buildUserPreferences(u)
	respond(w, r, isJSON, okResp("preference", prefs))
}

// userPreference returns a single preference by name (filter parameter).
func (h *Handler) userPreference(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	filter := param(r, "filter")
	if filter == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (preference name)"))
		return
	}

	u, err := h.db.Users.GetUser(r.Context(), session.UserID)
	if err != nil || u == nil {
		respond(w, r, isJSON, errResp(4704, "User not found"))
		return
	}

	prefs := buildUserPreferences(u)
	for _, p := range prefs {
		if p.Name == filter {
			respond(w, r, isJSON, p)
			return
		}
	}

	respond(w, r, isJSON, errResp(4704, "Preference not found"))
}

// systemPreferences returns server-level preferences. Admin only.
func (h *Handler) systemPreferences(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	prefs := []AmpPreference{
		{ID: "1", Name: "api_enable_5", Value: "1", Description: "Enable Ampache v5 API", Level: 100, Type: "boolean", Category: "system"},
		{ID: "2", Name: "api_enable_6", Value: "1", Description: "Enable Ampache v6 API", Level: 100, Type: "boolean", Category: "system"},
	}
	respond(w, r, isJSON, okResp("preference", prefs))
}

// systemPreference returns a single server-level preference. Admin only.
func (h *Handler) systemPreference(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, errResp(4704, "Preference not found"))
}

// preferenceCreate creates a new server-level preference. Admin only.
// Cassonic uses static preferences defined in server.yml; dynamic preference
// creation via the API is not supported. Returns error 4710 per Ampache spec.
func (h *Handler) preferenceCreate(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, errResp(4710, "Preferences are read-only on this server"))
}

// preferenceEdit modifies an existing server-level preference. Admin only.
// Cassonic uses static preferences defined in server.yml; dynamic preference
// editing via the API is not supported. Returns error 4710 per Ampache spec.
func (h *Handler) preferenceEdit(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, errResp(4710, "Preferences are read-only on this server"))
}

// preferenceDelete removes a server-level preference. Admin only.
// Cassonic uses static preferences defined in server.yml; dynamic preference
// deletion via the API is not supported. Returns error 4710 per Ampache spec.
func (h *Handler) preferenceDelete(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, errResp(4710, "Preferences are read-only on this server"))
}

// toggleFollow acknowledges a follow/unfollow request. Cassonic does not
// implement social networking features; the request is accepted but has no effect.
func (h *Handler) toggleFollow(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("success", "follow toggled"))
}

// lastShouts returns the shout history. Cassonic does not implement social
// shout features; an empty list is returned per Ampache protocol.
func (h *Handler) lastShouts(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("shout", []map[string]any{}))
}

// timeline returns user activity events. Cassonic does not implement social
// timeline features; an empty list is returned per Ampache protocol.
func (h *Handler) timeline(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("activity", []map[string]any{}))
}

// friendsTimeline returns activity events for followed users. Cassonic does not
// implement social follow features; an empty list is returned per Ampache protocol.
func (h *Handler) friendsTimeline(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("activity", []map[string]any{}))
}

// userToAmp converts a model.User to the Ampache wire type.
func userToAmp(u *model.User) AmpUser {
	access := 25
	if u.IsAdmin {
		access = 100
	}
	disabled := 0
	if !u.IsEnabled {
		disabled = 1
	}
	return AmpUser{
		ID:          itoa(u.ID),
		Username:    u.Username,
		Email:       u.Email,
		Access:      access,
		FullName:    u.DisplayName,
		CanDownload: boolInt(u.CanDownload),
		CanUpload:   boolInt(u.CanUpload),
		Disabled:    disabled,
	}
}

// buildUserPreferences maps user model fields to Ampache preference objects.
func buildUserPreferences(u *model.User) []AmpPreference {
	return []AmpPreference{
		{ID: "1", Name: "play_type", Value: "web_player", Description: "Play type", Level: 25, Type: "special", Category: "interface"},
		{ID: "2", Name: "interface_theme", Value: u.Theme, Description: "Interface theme", Level: 25, Type: "string", Category: "interface"},
		{ID: "3", Name: "site_title", Value: "cassonic", Description: "Website title", Level: 25, Type: "string", Category: "interface"},
		{ID: "4", Name: "bitrate_default", Value: itoa(int64(u.MaxBitRate)), Description: "Default bitrate", Level: 25, Type: "integer", Category: "streaming"},
		{ID: "5", Name: "lang", Value: u.Language, Description: "Language", Level: 25, Type: "string", Category: "interface"},
	}
}

// hashPasswordArgon2id produces an Argon2id hash of the given password.
// Format: $argon2id$v=19$m=65536,t=3,p=4$<base64salt>$<base64hash>
func hashPasswordArgon2id(password string) (string, error) {
	const (
		memory      = 65536
		iterations  = 3
		parallelism = 4
		keyLen      = 32
		saltLen     = 16
	)
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2id: generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLen)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memory, iterations, parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}
