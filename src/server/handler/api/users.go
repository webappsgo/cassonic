package api

import (
	"encoding/json"
	"net/http"
	"time"

	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/service/crypto"
	cerr "github.com/local/cassonic/src/common/errors"
)

// safeUser converts a model.User to a response map, omitting sensitive fields.
func safeUser(u *model.User) map[string]any {
	return map[string]any{
		"id":               u.ID,
		"username":         u.Username,
		"email":            u.Email,
		"display_name":     u.DisplayName,
		"is_admin":         u.IsAdmin,
		"is_enabled":       u.IsEnabled,
		"avatar_url":       u.AvatarURL,
		"language":         u.Language,
		"theme":            u.Theme,
		"max_bit_rate":     u.MaxBitRate,
		"can_download":     u.CanDownload,
		"can_upload":       u.CanUpload,
		"can_share":        u.CanShare,
		"can_manage_users": u.CanManageUsers,
		"can_comment":      u.CanComment,
		"can_podcast":      u.CanPodcast,
		"totp_enabled":     u.TOTPEnabled,
		"last_login_at":    u.LastLoginAt.Format(time.RFC3339),
		"created_at":       u.CreatedAt.Format(time.RFC3339),
		"updated_at":       u.UpdatedAt.Format(time.RFC3339),
	}
}

// ListUsers returns all users; admin only.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.db.Users.ListUsers(r.Context())
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list users failed"))
		return
	}

	result := make([]map[string]any, 0, len(users))
	for _, u := range users {
		result = append(result, safeUser(u))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users": result,
		"total": len(result),
	})
}

// createUserRequest is the body for POST /api/v1/users.
type createUserRequest struct {
	Username       string `json:"username"`
	Email          string `json:"email"`
	Password       string `json:"password"`
	DisplayName    string `json:"display_name"`
	IsAdmin        bool   `json:"is_admin"`
	IsEnabled      bool   `json:"is_enabled"`
	CanDownload    bool   `json:"can_download"`
	CanUpload      bool   `json:"can_upload"`
	CanShare       bool   `json:"can_share"`
	CanManageUsers bool   `json:"can_manage_users"`
	CanComment     bool   `json:"can_comment"`
	CanPodcast     bool   `json:"can_podcast"`
	Language       string `json:"language"`
	Theme          string `json:"theme"`
	MaxBitRate     int    `json:"max_bit_rate"`
}

// CreateUser creates a new user; admin only.
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, r, cerr.BadRequest("username and password are required"))
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("password hashing failed"))
		return
	}

	if req.Language == "" {
		req.Language = "en"
	}
	if req.Theme == "" {
		req.Theme = "dark"
	}

	u := &model.User{
		Username:       req.Username,
		Email:          req.Email,
		PasswordHash:   hash,
		DisplayName:    req.DisplayName,
		IsAdmin:        req.IsAdmin,
		IsEnabled:      req.IsEnabled,
		CanDownload:    req.CanDownload,
		CanUpload:      req.CanUpload,
		CanShare:       req.CanShare,
		CanManageUsers: req.CanManageUsers,
		CanComment:     req.CanComment,
		CanPodcast:     req.CanPodcast,
		Language:       req.Language,
		Theme:          req.Theme,
		MaxBitRate:     req.MaxBitRate,
	}

	id, err := h.db.Users.CreateUser(r.Context(), u)
	if err != nil {
		writeError(w, r, cerr.Conflict("user creation failed: "+err.Error()))
		return
	}
	u.ID = id

	writeJSON(w, http.StatusCreated, safeUser(u))
}

// GetMe returns the authenticated user's profile.
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	user, err := h.db.Users.GetUser(r.Context(), auth.ID)
	if err != nil || user == nil {
		writeError(w, r, cerr.NotFound("user not found"))
		return
	}

	writeJSON(w, http.StatusOK, safeUser(user))
}

// updateMeRequest is the body for PUT /api/v1/users/me.
type updateMeRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Language    string `json:"language"`
	Theme       string `json:"theme"`
}

// UpdateMe updates the authenticated user's own profile fields.
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	user, err := h.db.Users.GetUser(r.Context(), auth.ID)
	if err != nil || user == nil {
		writeError(w, r, cerr.NotFound("user not found"))
		return
	}

	var req updateMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}

	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Language != "" {
		user.Language = req.Language
	}
	if req.Theme != "" {
		user.Theme = req.Theme
	}

	if err := h.db.Users.UpdateUser(r.Context(), user); err != nil {
		writeError(w, r, cerr.InternalServerError("update failed"))
		return
	}

	writeJSON(w, http.StatusOK, safeUser(user))
}

// GetUser returns a user by ID; admin or the user themselves.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid user id"))
		return
	}

	if !auth.IsAdmin && auth.ID != id {
		writeError(w, r, cerr.Forbidden("access denied"))
		return
	}

	user, err := h.db.Users.GetUser(r.Context(), id)
	if err != nil || user == nil {
		writeError(w, r, cerr.NotFound("user not found"))
		return
	}

	writeJSON(w, http.StatusOK, safeUser(user))
}

// updateUserRequest is the body for PUT /api/v1/users/{id}.
type updateUserRequest struct {
	DisplayName    string `json:"display_name"`
	Email          string `json:"email"`
	IsAdmin        *bool  `json:"is_admin"`
	IsEnabled      *bool  `json:"is_enabled"`
	CanDownload    *bool  `json:"can_download"`
	CanUpload      *bool  `json:"can_upload"`
	CanShare       *bool  `json:"can_share"`
	CanManageUsers *bool  `json:"can_manage_users"`
	CanComment     *bool  `json:"can_comment"`
	CanPodcast     *bool  `json:"can_podcast"`
	Language       string `json:"language"`
	Theme          string `json:"theme"`
	MaxBitRate     *int   `json:"max_bit_rate"`
}

// UpdateUser updates any user's fields; admin only.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid user id"))
		return
	}

	user, err := h.db.Users.GetUser(r.Context(), id)
	if err != nil || user == nil {
		writeError(w, r, cerr.NotFound("user not found"))
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}

	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Language != "" {
		user.Language = req.Language
	}
	if req.Theme != "" {
		user.Theme = req.Theme
	}
	if req.IsAdmin != nil {
		user.IsAdmin = *req.IsAdmin
	}
	if req.IsEnabled != nil {
		user.IsEnabled = *req.IsEnabled
	}
	if req.CanDownload != nil {
		user.CanDownload = *req.CanDownload
	}
	if req.CanUpload != nil {
		user.CanUpload = *req.CanUpload
	}
	if req.CanShare != nil {
		user.CanShare = *req.CanShare
	}
	if req.CanManageUsers != nil {
		user.CanManageUsers = *req.CanManageUsers
	}
	if req.CanComment != nil {
		user.CanComment = *req.CanComment
	}
	if req.CanPodcast != nil {
		user.CanPodcast = *req.CanPodcast
	}
	if req.MaxBitRate != nil {
		user.MaxBitRate = *req.MaxBitRate
	}

	if err := h.db.Users.UpdateUser(r.Context(), user); err != nil {
		writeError(w, r, cerr.InternalServerError("update failed"))
		return
	}

	writeJSON(w, http.StatusOK, safeUser(user))
}

// DeleteUser permanently removes a user; admin only; cannot delete own account.
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid user id"))
		return
	}

	if auth.ID == id {
		writeError(w, r, cerr.Forbidden("cannot delete own account"))
		return
	}

	if err := h.db.Users.DeleteUser(r.Context(), id); err != nil {
		writeError(w, r, cerr.InternalServerError("delete failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// setSubsonicPasswordRequest is the body for PUT /api/v1/users/me/subsonic-password.
type setSubsonicPasswordRequest struct {
	Password string `json:"password"`
}

// SetSubsonicPassword stores an AES-256-GCM encrypted copy of the caller's
// subsonic password. Subsonic and Ampache clients require the plaintext password
// for their token/handshake auth schemes, so it must be recoverable.
// The encrypted value is stored in the subsonic_password column.
func (h *Handler) SetSubsonicPassword(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	var req setSubsonicPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}
	if req.Password == "" {
		writeError(w, r, cerr.BadRequest("password is required"))
		return
	}

	if len(h.subsonicKey) == 0 {
		writeError(w, r, cerr.InternalServerError("subsonic key not configured"))
		return
	}

	encrypted, err := crypto.Encrypt(h.subsonicKey, req.Password)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("encryption failed"))
		return
	}

	if err := h.db.Users.SetSubsonicPassword(r.Context(), auth.Username, encrypted); err != nil {
		writeError(w, r, cerr.InternalServerError("failed to store subsonic password"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// changePasswordRequest is the body for POST /api/v1/users/{id}/password.
type changePasswordRequest struct {
	Password    string `json:"password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword updates a user's password.
// Regular users must provide their current password; admins can skip it.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid user id"))
		return
	}

	if !auth.IsAdmin && auth.ID != id {
		writeError(w, r, cerr.Forbidden("access denied"))
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}
	if req.NewPassword == "" {
		writeError(w, r, cerr.BadRequest("new_password is required"))
		return
	}

	user, err := h.db.Users.GetUser(r.Context(), id)
	if err != nil || user == nil {
		writeError(w, r, cerr.NotFound("user not found"))
		return
	}

	if !auth.IsAdmin {
		if req.Password == "" {
			writeError(w, r, cerr.BadRequest("current password is required"))
			return
		}
		ok, err := verifyPassword(req.Password, user.PasswordHash)
		if err != nil || !ok {
			writeError(w, r, cerr.Unauthorized("current password is incorrect"))
			return
		}
	}

	hash, err := hashPassword(req.NewPassword)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("password hashing failed"))
		return
	}
	user.PasswordHash = hash

	if err := h.db.Users.UpdateUser(r.Context(), user); err != nil {
		writeError(w, r, cerr.InternalServerError("password update failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}
