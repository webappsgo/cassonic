package subsonic

import (
	"net/http"
	"strings"
	"time"

	"github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/service"
)

// ping responds with an empty OK response to verify connectivity.
func (h *Handler) ping(w http.ResponseWriter, r *http.Request) {
	respond(w, r, ok(nil))
}

// getLicense returns the server license status, which is always valid.
func (h *Handler) getLicense(w http.ResponseWriter, r *http.Request) {
	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.License = &License{
			Valid:          true,
			Email:          "",
			LicenseExpires: "",
		}
	}))
}

// getScanStatus returns the status of the most recent library scan.
func (h *Handler) getScanStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.db.Music.GetLastScanStatus(r.Context())
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to retrieve scan status."))
		return
	}

	scanning := false
	var count int64
	if status != nil {
		scanning = status.Status == "running"
		count = int64(status.ScannedFiles)
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.ScanStatus = &ScanStatusResp{
			Scanning: scanning,
			Count:    count,
		}
	}))
}

// startScan triggers an incremental library scan in the background and returns the initial status.
func (h *Handler) startScan(w http.ResponseWriter, r *http.Request) {
	go func() {
		_ = h.scanner.Scan(r.Context(), service.ScanModeIncremental)
	}()

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.ScanStatus = &ScanStatusResp{
			Scanning: true,
			Count:    0,
		}
	}))
}

// getUser returns user account details. Regular users may only fetch their own account;
// admins may fetch any account.
func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'username' is missing."))
		return
	}

	if !authUser.IsAdmin && authUser.Username != username {
		respond(w, r, errResp(ErrForbidden, "Permission denied."))
		return
	}

	u, err := h.db.Users.GetUserByUsername(r.Context(), username)
	if err != nil || u == nil {
		respond(w, r, errResp(ErrNotFound, "User not found."))
		return
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.User = modelUserToResp(u)
	}))
}

// getUsers returns all user accounts. Admin only.
func (h *Handler) getUsers(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	if !authUser.IsAdmin {
		respond(w, r, errResp(ErrForbidden, "Permission denied."))
		return
	}

	users, err := h.db.Users.ListUsers(r.Context())
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to list users."))
		return
	}

	userResps := make([]UserResp, 0, len(users))
	for _, u := range users {
		userResps = append(userResps, *modelUserToResp(u))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Users = &UsersResp{User: userResps}
	}))
}

// createUser creates a new user account. Admin only.
func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	if !authUser.IsAdmin {
		respond(w, r, errResp(ErrForbidden, "Permission denied."))
		return
	}

	q := r.URL.Query()
	username := q.Get("username")
	password := q.Get("password")
	email := q.Get("email")

	if username == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'username' is missing."))
		return
	}
	if password == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'password' is missing."))
		return
	}

	hash, err := HashPassword(password)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to hash password."))
		return
	}

	u := &model.User{
		Username:       username,
		Email:          email,
		PasswordHash:   hash,
		IsEnabled:      true,
		IsAdmin:        parseBoolParam(q.Get("adminRole")),
		CanDownload:    parseBoolParam(q.Get("downloadRole")),
		CanUpload:      parseBoolParam(q.Get("uploadRole")),
		CanShare:       parseBoolParam(q.Get("shareRole")),
		CanManageUsers: parseBoolParam(q.Get("settingsRole")),
		CanComment:     parseBoolParam(q.Get("commentRole")),
		CanPodcast:     parseBoolParam(q.Get("podcastRole")),
	}

	_, err = h.db.Users.CreateUser(r.Context(), u)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to create user."))
		return
	}

	respond(w, r, ok(nil))
}

// updateUser updates fields on an existing user account. Admin only.
func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	if !authUser.IsAdmin {
		respond(w, r, errResp(ErrForbidden, "Permission denied."))
		return
	}

	q := r.URL.Query()
	username := q.Get("username")
	if username == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'username' is missing."))
		return
	}

	u, err := h.db.Users.GetUserByUsername(r.Context(), username)
	if err != nil || u == nil {
		respond(w, r, errResp(ErrNotFound, "User not found."))
		return
	}

	if v := q.Get("email"); v != "" {
		u.Email = v
	}
	if v := q.Get("adminRole"); v != "" {
		u.IsAdmin = parseBoolParam(v)
	}
	if v := q.Get("downloadRole"); v != "" {
		u.CanDownload = parseBoolParam(v)
	}
	if v := q.Get("uploadRole"); v != "" {
		u.CanUpload = parseBoolParam(v)
	}
	if v := q.Get("shareRole"); v != "" {
		u.CanShare = parseBoolParam(v)
	}
	if v := q.Get("settingsRole"); v != "" {
		u.CanManageUsers = parseBoolParam(v)
	}
	if v := q.Get("commentRole"); v != "" {
		u.CanComment = parseBoolParam(v)
	}
	if v := q.Get("podcastRole"); v != "" {
		u.CanPodcast = parseBoolParam(v)
	}

	u.UpdatedAt = time.Now()

	if err := h.db.Users.UpdateUser(r.Context(), u); err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to update user."))
		return
	}

	respond(w, r, ok(nil))
}

// deleteUser permanently removes a user account. Admin only.
func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	if !authUser.IsAdmin {
		respond(w, r, errResp(ErrForbidden, "Permission denied."))
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'username' is missing."))
		return
	}

	u, err := h.db.Users.GetUserByUsername(r.Context(), username)
	if err != nil || u == nil {
		respond(w, r, errResp(ErrNotFound, "User not found."))
		return
	}

	if err := h.db.Users.DeleteUser(r.Context(), u.ID); err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to delete user."))
		return
	}

	respond(w, r, ok(nil))
}

// changePassword updates the password for a user account.
// Regular users may only change their own password; admins may change any password.
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	q := r.URL.Query()
	username := q.Get("username")
	password := q.Get("password")

	if username == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'username' is missing."))
		return
	}
	if password == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'password' is missing."))
		return
	}

	if !authUser.IsAdmin && authUser.Username != username {
		respond(w, r, errResp(ErrForbidden, "Permission denied."))
		return
	}

	u, err := h.db.Users.GetUserByUsername(r.Context(), username)
	if err != nil || u == nil {
		respond(w, r, errResp(ErrNotFound, "User not found."))
		return
	}

	hash, err := HashPassword(password)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to hash password."))
		return
	}

	u.PasswordHash = hash
	u.UpdatedAt = time.Now()

	if err := h.db.Users.UpdateUser(r.Context(), u); err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to update password."))
		return
	}

	respond(w, r, ok(nil))
}

// modelUserToResp converts a model.User to a Subsonic UserResp.
func modelUserToResp(u *model.User) *UserResp {
	return &UserResp{
		Username:          u.Username,
		Email:             u.Email,
		ScrobblingEnabled: true,
		MaxBitRate:        u.MaxBitRate,
		AdminRole:         u.IsAdmin,
		SettingsRole:      u.CanManageUsers || u.IsAdmin,
		DownloadRole:      u.CanDownload,
		UploadRole:        u.CanUpload,
		PlaylistRole:      true,
		CoverArtRole:      true,
		CommentRole:       u.CanComment,
		PodcastRole:       u.CanPodcast,
		StreamRole:        true,
		JukeboxRole:       false,
		ShareRole:         u.CanShare,
		VideoConversionRole: false,
	}
}

// getOpenSubsonicExtensions returns the list of OpenSubsonic extensions supported by this server.
// See https://opensubsonic.netlify.app/ for the extension registry.
func (h *Handler) getOpenSubsonicExtensions(w http.ResponseWriter, r *http.Request) {
	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.OpenSubsonicExtensions = &OpenSubsonicExtensions{
			Extension: []OpenSubsonicExtension{
				{Name: "formPost", Versions: []int{1}},
				{Name: "transcodeOffset", Versions: []int{1}},
				{Name: "songLyrics", Versions: []int{1}},
			},
		}
	}))
}

// parseBoolParam converts common truthy string values to bool.
func parseBoolParam(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "1", "on", "enable", "enabled":
		return true
	}
	return false
}
