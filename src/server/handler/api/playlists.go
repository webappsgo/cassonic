package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	cerr "github.com/local/cassonic/src/common/errors"
)


// ListPlaylists returns all playlists visible to the authenticated user.
func (h *Handler) ListPlaylists(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	playlists, err := h.db.Playlists.ListPlaylists(r.Context(), auth.ID)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list playlists failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"playlists": playlists,
		"total":     len(playlists),
	})
}

// createPlaylistRequest is the body for POST /api/v1/playlists.
type createPlaylistRequest struct {
	Name     string `json:"name"`
	Comment  string `json:"comment"`
	IsPublic bool   `json:"is_public"`
}

// CreatePlaylist creates a new playlist owned by the authenticated user.
func (h *Handler) CreatePlaylist(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	var req createPlaylistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}
	if req.Name == "" {
		writeError(w, r, cerr.BadRequest("name is required"))
		return
	}

	pl := &model.Playlist{
		UserID:   auth.ID,
		Name:     req.Name,
		Comment:  req.Comment,
		IsPublic: req.IsPublic,
	}

	id, err := h.db.Playlists.CreatePlaylist(r.Context(), pl)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("create playlist failed"))
		return
	}
	pl.ID = id

	writeJSON(w, http.StatusCreated, pl)
}

// GetPlaylist returns a single playlist by ID.
func (h *Handler) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid playlist id"))
		return
	}

	pl, err := h.db.Playlists.GetPlaylist(r.Context(), id)
	if err != nil || pl == nil {
		writeError(w, r, cerr.NotFound("playlist not found"))
		return
	}

	if pl.UserID != auth.ID && !pl.IsPublic && !auth.IsAdmin {
		writeError(w, r, cerr.Forbidden("access denied"))
		return
	}

	writeJSON(w, http.StatusOK, pl)
}

// updatePlaylistRequest is the body for PUT /api/v1/playlists/{id}.
type updatePlaylistRequest struct {
	Name     string `json:"name"`
	Comment  string `json:"comment"`
	IsPublic *bool  `json:"is_public"`
}

// UpdatePlaylist updates a playlist owned by the authenticated user.
func (h *Handler) UpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid playlist id"))
		return
	}

	pl, err := h.db.Playlists.GetPlaylist(r.Context(), id)
	if err != nil || pl == nil {
		writeError(w, r, cerr.NotFound("playlist not found"))
		return
	}

	if pl.UserID != auth.ID && !auth.IsAdmin {
		writeError(w, r, cerr.Forbidden("access denied"))
		return
	}

	var req updatePlaylistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}

	if req.Name != "" {
		pl.Name = req.Name
	}
	if req.Comment != "" {
		pl.Comment = req.Comment
	}
	if req.IsPublic != nil {
		pl.IsPublic = *req.IsPublic
	}

	if err := h.db.Playlists.UpdatePlaylist(r.Context(), pl); err != nil {
		writeError(w, r, cerr.InternalServerError("update failed"))
		return
	}

	writeJSON(w, http.StatusOK, pl)
}

// DeletePlaylist removes a playlist owned by the authenticated user.
func (h *Handler) DeletePlaylist(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid playlist id"))
		return
	}

	pl, err := h.db.Playlists.GetPlaylist(r.Context(), id)
	if err != nil || pl == nil {
		writeError(w, r, cerr.NotFound("playlist not found"))
		return
	}

	if pl.UserID != auth.ID && !auth.IsAdmin {
		writeError(w, r, cerr.Forbidden("access denied"))
		return
	}

	if err := h.db.Playlists.DeletePlaylist(r.Context(), id); err != nil {
		writeError(w, r, cerr.InternalServerError("delete failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// GetPlaylistSongs returns the ordered songs in a playlist with full song details.
func (h *Handler) GetPlaylistSongs(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid playlist id"))
		return
	}

	pl, err := h.db.Playlists.GetPlaylist(r.Context(), id)
	if err != nil || pl == nil {
		writeError(w, r, cerr.NotFound("playlist not found"))
		return
	}

	if pl.UserID != auth.ID && !pl.IsPublic && !auth.IsAdmin {
		writeError(w, r, cerr.Forbidden("access denied"))
		return
	}

	entries, err := h.db.Playlists.GetPlaylistEntries(r.Context(), id)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("get entries failed"))
		return
	}

	songs := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		song, err := h.db.Music.GetSong(r.Context(), entry.SongID)
		if err != nil || song == nil {
			continue
		}
		songs = append(songs, map[string]any{
			"position": entry.Position,
			"song":     song,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"songs": songs,
		"total": len(songs),
	})
}

// addPlaylistSongsRequest is the body for POST /api/v1/playlists/{id}/songs.
type addPlaylistSongsRequest struct {
	SongIDs []int64 `json:"song_ids"`
}

// AddPlaylistSongs appends songs to a playlist.
func (h *Handler) AddPlaylistSongs(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid playlist id"))
		return
	}

	pl, err := h.db.Playlists.GetPlaylist(r.Context(), id)
	if err != nil || pl == nil {
		writeError(w, r, cerr.NotFound("playlist not found"))
		return
	}

	if pl.UserID != auth.ID && !auth.IsAdmin {
		writeError(w, r, cerr.Forbidden("access denied"))
		return
	}

	var req addPlaylistSongsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}
	if len(req.SongIDs) == 0 {
		writeError(w, r, cerr.BadRequest("song_ids is required"))
		return
	}

	if err := h.db.Playlists.AddToPlaylist(r.Context(), id, req.SongIDs); err != nil {
		writeError(w, r, cerr.InternalServerError("add songs failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// RemovePlaylistSong removes the first occurrence of a song from a playlist.
func (h *Handler) RemovePlaylistSong(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid playlist id"))
		return
	}

	songIDStr := chi.URLParam(r, "songId")
	songID, err := strconv.ParseInt(songIDStr, 10, 64)
	if err != nil || songID <= 0 {
		writeError(w, r, cerr.BadRequest("invalid song id"))
		return
	}

	pl, err := h.db.Playlists.GetPlaylist(r.Context(), id)
	if err != nil || pl == nil {
		writeError(w, r, cerr.NotFound("playlist not found"))
		return
	}

	if pl.UserID != auth.ID && !auth.IsAdmin {
		writeError(w, r, cerr.Forbidden("access denied"))
		return
	}

	entries, err := h.db.Playlists.GetPlaylistEntries(r.Context(), id)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("get entries failed"))
		return
	}

	removeIdx := -1
	for _, e := range entries {
		if e.SongID == songID {
			removeIdx = e.Position
			break
		}
	}

	if removeIdx < 0 {
		writeError(w, r, cerr.NotFound("song not in playlist"))
		return
	}

	if err := h.db.Playlists.RemoveFromPlaylist(r.Context(), id, []int{removeIdx}); err != nil {
		writeError(w, r, cerr.InternalServerError("remove song failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}
