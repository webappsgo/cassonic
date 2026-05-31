package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	cerr "github.com/local/cassonic/src/common/errors"
)

// GetPlayQueue returns the authenticated user's play queue with song details.
func (h *Handler) GetPlayQueue(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	queue, entries, err := h.db.Activity.GetPlayQueue(r.Context(), auth.ID)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("get play queue failed"))
		return
	}
	if queue == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"current":    0,
			"position":   0,
			"songs":      []any{},
			"updated_at": "",
		})
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
		"current":    queue.Current,
		"position":   queue.Position,
		"songs":      songs,
		"changed_by": queue.ChangedBy,
		"updated_at": queue.UpdatedAt.Format(time.RFC3339),
	})
}

// savePlayQueueRequest is the body for PUT /api/v1/play-queues.
type savePlayQueueRequest struct {
	SongIDs  []int64 `json:"song_ids"`
	Current  int64   `json:"current"`
	Position int64   `json:"position"`
}

// SavePlayQueue replaces the authenticated user's play queue.
func (h *Handler) SavePlayQueue(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	var req savePlayQueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}

	queue := &model.PlayQueue{
		UserID:    auth.ID,
		Current:   req.Current,
		Position:  req.Position,
		ChangedBy: r.Header.Get("X-Client-Name"),
	}

	entries := make([]*model.PlayQueueEntry, 0, len(req.SongIDs))
	for i, songID := range req.SongIDs {
		entries = append(entries, &model.PlayQueueEntry{
			SongID:   songID,
			Position: i,
		})
	}

	if err := h.db.Activity.SavePlayQueue(r.Context(), queue, entries); err != nil {
		writeError(w, r, cerr.InternalServerError("save play queue failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// ListBookmarks returns all bookmarks for the authenticated user with song details.
func (h *Handler) ListBookmarks(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	bookmarks, err := h.db.Activity.GetBookmarks(r.Context(), auth.ID)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list bookmarks failed"))
		return
	}

	result := make([]map[string]any, 0, len(bookmarks))
	for _, b := range bookmarks {
		entry := map[string]any{
			"id":         b.ID,
			"item_type":  b.ItemType,
			"item_id":    b.ItemID,
			"position":   b.Position,
			"comment":    b.Comment,
			"updated_at": b.UpdatedAt.Format(time.RFC3339),
		}
		if b.ItemType == "song" {
			song, err := h.db.Music.GetSong(r.Context(), b.ItemID)
			if err == nil && song != nil {
				entry["song"] = song
			}
		}
		result = append(result, entry)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"bookmarks": result,
		"total":     len(result),
	})
}

// createBookmarkRequest is the body for POST /api/v1/bookmarks.
type createBookmarkRequest struct {
	ItemType string `json:"item_type"`
	ItemID   int64  `json:"item_id"`
	Position int64  `json:"position"`
	Comment  string `json:"comment"`
}

// CreateBookmark saves a new playback position bookmark.
func (h *Handler) CreateBookmark(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	var req createBookmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}
	if req.ItemType == "" || req.ItemID <= 0 {
		writeError(w, r, cerr.BadRequest("item_type and item_id are required"))
		return
	}

	b := &model.Bookmark{
		UserID:   auth.ID,
		ItemType: req.ItemType,
		ItemID:   req.ItemID,
		Position: req.Position,
		Comment:  req.Comment,
	}

	if err := h.db.Activity.SetBookmark(r.Context(), b); err != nil {
		writeError(w, r, cerr.InternalServerError("save bookmark failed"))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{})
}

// updateBookmarkRequest is the body for PUT /api/v1/bookmarks/{id}.
type updateBookmarkRequest struct {
	Position int64  `json:"position"`
	Comment  string `json:"comment"`
}

// UpdateBookmark updates the position or comment of an existing bookmark.
func (h *Handler) UpdateBookmark(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid bookmark id"))
		return
	}

	bookmarks, err := h.db.Activity.GetBookmarks(r.Context(), auth.ID)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("get bookmark failed"))
		return
	}

	var target *model.Bookmark
	for _, b := range bookmarks {
		if b.ID == id {
			target = b
			break
		}
	}
	if target == nil {
		writeError(w, r, cerr.NotFound("bookmark not found"))
		return
	}

	var req updateBookmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}

	target.Position = req.Position
	if req.Comment != "" {
		target.Comment = req.Comment
	}

	if err := h.db.Activity.SetBookmark(r.Context(), target); err != nil {
		writeError(w, r, cerr.InternalServerError("update bookmark failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// DeleteBookmark removes a bookmark by ID.
func (h *Handler) DeleteBookmark(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid bookmark id"))
		return
	}

	bookmarks, err := h.db.Activity.GetBookmarks(r.Context(), auth.ID)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("get bookmark failed"))
		return
	}

	var target *model.Bookmark
	for _, b := range bookmarks {
		if b.ID == id {
			target = b
			break
		}
	}
	if target == nil {
		writeError(w, r, cerr.NotFound("bookmark not found"))
		return
	}

	if err := h.db.Activity.DeleteBookmark(r.Context(), auth.ID, target.ItemType, target.ItemID); err != nil {
		writeError(w, r, cerr.InternalServerError("delete bookmark failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// NowPlaying returns the currently active stream(s).
// Admins see all streams; regular users see only their own.
func (h *Handler) NowPlaying(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	var streams []*NowPlayingInfo
	if auth.IsAdmin {
		streams = h.nowPlaying.All()
	} else {
		info := h.nowPlaying.ForUser(auth.ID)
		if info != nil {
			streams = []*NowPlayingInfo{info}
		}
	}

	result := make([]map[string]any, 0, len(streams))
	for _, s := range streams {
		entry := map[string]any{
			"user_id":     s.UserID,
			"username":    s.Username,
			"started_at":  s.StartedAt.Format(time.RFC3339),
			"player_name": s.PlayerName,
			"minutes_ago": int(time.Since(s.StartedAt).Minutes()),
		}
		if s.Song != nil {
			entry["song"] = s.Song
		}
		result = append(result, entry)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"now_playing": result,
		"total":       len(result),
	})
}

// NowPlayingSSE streams now-playing updates as Server-Sent Events.
func (h *Handler) NowPlayingSSE(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, cerr.InternalServerError("streaming not supported"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			var streams []*NowPlayingInfo
			if auth.IsAdmin {
				streams = h.nowPlaying.All()
			} else {
				info := h.nowPlaying.ForUser(auth.ID)
				if info != nil {
					streams = []*NowPlayingInfo{info}
				}
			}

			data, err := json.Marshal(streams)
			if err != nil {
				continue
			}

			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// StarSong stars a song for the authenticated user.
func (h *Handler) StarSong(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	songID, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid song id"))
		return
	}

	if err := h.db.Activity.Star(r.Context(), auth.ID, "song", songID); err != nil {
		writeError(w, r, cerr.InternalServerError("star failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// UnstarSong removes a star from a song for the authenticated user.
func (h *Handler) UnstarSong(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	songID, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid song id"))
		return
	}

	if err := h.db.Activity.Unstar(r.Context(), auth.ID, "song", songID); err != nil {
		writeError(w, r, cerr.InternalServerError("unstar failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// rateSongRequest is the body for PATCH /api/v1/songs/{id}/rating.
type rateSongRequest struct {
	Rating int `json:"rating"`
}

// RateSong sets the authenticated user's rating for a song.
func (h *Handler) RateSong(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	songID, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid song id"))
		return
	}

	var req rateSongRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}
	if req.Rating < 1 || req.Rating > 5 {
		writeError(w, r, cerr.BadRequest("rating must be between 1 and 5"))
		return
	}

	if err := h.db.Activity.SetRating(r.Context(), auth.ID, "song", songID, req.Rating); err != nil {
		writeError(w, r, cerr.InternalServerError("set rating failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// scrobbleRequest is the body for POST /api/v1/songs/{id}/scrobbles.
type scrobbleRequest struct {
	Timestamp  int64 `json:"timestamp"`
	Submission bool  `json:"submission"`
}

// Scrobble records a play event for a song.
func (h *Handler) Scrobble(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	songID, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid song id"))
		return
	}

	var req scrobbleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}

	playedAt := time.Now()
	if req.Timestamp > 0 {
		playedAt = time.Unix(req.Timestamp, 0)
	}

	history := &model.PlayHistory{
		UserID:     auth.ID,
		SongID:     songID,
		PlayedAt:   playedAt,
		Scrobbled:  req.Submission,
		ClientName: r.Header.Get("X-Client-Name"),
	}

	if err := h.db.Activity.RecordPlay(r.Context(), history); err != nil {
		writeError(w, r, cerr.InternalServerError("record play failed"))
		return
	}

	if req.Submission {
		_ = h.db.Music.IncrementPlayCount(r.Context(), songID)
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// unused prevents the import of strconv from being removed.
var _ = strconv.Itoa
