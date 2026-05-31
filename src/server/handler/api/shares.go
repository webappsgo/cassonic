package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	cerr "github.com/local/cassonic/src/common/errors"
)

// generateShareToken creates a URL-safe random token.
func generateShareToken() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashSharePassword returns the SHA-256 hex hash of a share password.
func hashSharePassword(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

// ListShares returns all share links owned by the authenticated user.
func (h *Handler) ListShares(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	shares, err := h.db.Shares.ListSharesByUser(r.Context(), auth.ID)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list shares failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"shares": shares,
		"total":  len(shares),
	})
}

// createShareRequest is the body for POST /api/v1/shares.
type createShareRequest struct {
	ItemType    string `json:"item_type"`
	ItemID      int64  `json:"item_id"`
	Description string `json:"description"`
	Password    string `json:"password"`
	ExpiresAt   string `json:"expires_at"`
}

// CreateShare creates a new share link.
func (h *Handler) CreateShare(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	var req createShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}
	if req.ItemType == "" || req.ItemID <= 0 {
		writeError(w, r, cerr.BadRequest("item_type and item_id are required"))
		return
	}

	token, err := generateShareToken()
	if err != nil {
		writeError(w, r, cerr.InternalServerError("token generation failed"))
		return
	}

	var expiresAt time.Time
	if req.ExpiresAt != "" {
		expiresAt, err = time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			writeError(w, r, cerr.BadRequest("invalid expires_at format; use RFC3339"))
			return
		}
	}

	var passwordHash string
	if req.Password != "" {
		passwordHash = hashSharePassword(req.Password)
	}

	share := &model.Share{
		UserID:       auth.ID,
		Token:        token,
		ItemType:     req.ItemType,
		ItemID:       req.ItemID,
		Description:  req.Description,
		PasswordHash: passwordHash,
		ExpiresAt:    expiresAt,
	}

	id, err := h.db.Shares.CreateShare(r.Context(), share)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("create share failed"))
		return
	}
	share.ID = id

	writeJSON(w, http.StatusCreated, share)
}

// GetShare returns a share by its public token (no auth required).
func (h *Handler) GetShare(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		writeError(w, r, cerr.BadRequest("token is required"))
		return
	}

	share, err := h.db.Shares.GetShareByToken(r.Context(), token)
	if err != nil || share == nil {
		writeError(w, r, cerr.NotFound("share not found"))
		return
	}

	if share.IsExpired() {
		writeError(w, r, cerr.NotFound("share has expired"))
		return
	}

	if share.HasPassword() {
		password := r.URL.Query().Get("password")
		if password == "" {
			writeError(w, r, cerr.Unauthorized("password required"))
			return
		}
		submitted := hashSharePassword(password)
		if submitted != share.PasswordHash {
			writeError(w, r, cerr.Unauthorized("incorrect password"))
			return
		}
	}

	_ = h.db.Shares.IncrementViewCount(r.Context(), share.ID)

	responseShare := map[string]any{
		"id":          share.ID,
		"token":       share.Token,
		"item_type":   share.ItemType,
		"item_id":     share.ItemID,
		"description": share.Description,
		"view_count":  share.ViewCount + 1,
		"created_at":  share.CreatedAt.Format(time.RFC3339),
	}
	if !share.ExpiresAt.IsZero() {
		responseShare["expires_at"] = share.ExpiresAt.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, responseShare)
}

// updateShareRequest is the body for PUT /api/v1/shares/{id}.
type updateShareRequest struct {
	Description string `json:"description"`
	ExpiresAt   string `json:"expires_at"`
}

// UpdateShare updates a share's description or expiry.
func (h *Handler) UpdateShare(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid share id"))
		return
	}

	share, err := h.db.Shares.GetShare(r.Context(), id)
	if err != nil || share == nil {
		writeError(w, r, cerr.NotFound("share not found"))
		return
	}

	if share.UserID != auth.ID && !auth.IsAdmin {
		writeError(w, r, cerr.Forbidden("access denied"))
		return
	}

	var req updateShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}

	if req.Description != "" {
		share.Description = req.Description
	}
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			writeError(w, r, cerr.BadRequest("invalid expires_at format; use RFC3339"))
			return
		}
		share.ExpiresAt = t
	}

	if err := h.db.Shares.UpdateShare(r.Context(), share); err != nil {
		writeError(w, r, cerr.InternalServerError("update share failed"))
		return
	}

	writeJSON(w, http.StatusOK, share)
}

// DeleteShare removes a share link.
func (h *Handler) DeleteShare(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid share id"))
		return
	}

	share, err := h.db.Shares.GetShare(r.Context(), id)
	if err != nil || share == nil {
		writeError(w, r, cerr.NotFound("share not found"))
		return
	}

	if share.UserID != auth.ID && !auth.IsAdmin {
		writeError(w, r, cerr.Forbidden("access denied"))
		return
	}

	if err := h.db.Shares.DeleteShare(r.Context(), id); err != nil {
		writeError(w, r, cerr.InternalServerError("delete share failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}
