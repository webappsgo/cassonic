package api

import (
	"encoding/json"
	"net/http"

	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	cerr "github.com/local/cassonic/src/common/errors"
)

// ListPodcasts returns all subscribed podcast channels.
func (h *Handler) ListPodcasts(w http.ResponseWriter, r *http.Request) {
	channels, err := h.db.Podcasts.ListChannels(r.Context())
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list podcasts failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"podcasts": channels,
		"total":    len(channels),
	})
}

// createPodcastRequest is the body for POST /api/v1/podcasts.
type createPodcastRequest struct {
	URL string `json:"url"`
}

// CreatePodcast subscribes to a new podcast RSS feed; admin only.
func (h *Handler) CreatePodcast(w http.ResponseWriter, r *http.Request) {
	var req createPodcastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}
	if req.URL == "" {
		writeError(w, r, cerr.BadRequest("url is required"))
		return
	}

	ch := &model.PodcastChannel{
		URL:    req.URL,
		Status: model.PodcastStatusNew,
	}

	id, err := h.db.Podcasts.CreateChannel(r.Context(), ch)
	if err != nil {
		writeError(w, r, cerr.Conflict("create podcast failed: "+err.Error()))
		return
	}
	ch.ID = id

	writeJSON(w, http.StatusCreated, ch)
}

// GetPodcast returns a single podcast channel by ID.
func (h *Handler) GetPodcast(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid podcast id"))
		return
	}

	ch, err := h.db.Podcasts.GetChannel(r.Context(), id)
	if err != nil || ch == nil {
		writeError(w, r, cerr.NotFound("podcast not found"))
		return
	}

	writeJSON(w, http.StatusOK, ch)
}

// updatePodcastRequest is the body for PUT /api/v1/podcasts/{id}.
type updatePodcastRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// UpdatePodcast updates a podcast channel's metadata; admin only.
func (h *Handler) UpdatePodcast(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid podcast id"))
		return
	}

	ch, err := h.db.Podcasts.GetChannel(r.Context(), id)
	if err != nil || ch == nil {
		writeError(w, r, cerr.NotFound("podcast not found"))
		return
	}

	var req updatePodcastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}

	if req.Title != "" {
		ch.Title = req.Title
	}
	if req.Description != "" {
		ch.Description = req.Description
	}

	if err := h.db.Podcasts.UpdateChannel(r.Context(), ch); err != nil {
		writeError(w, r, cerr.InternalServerError("update podcast failed"))
		return
	}

	writeJSON(w, http.StatusOK, ch)
}

// DeletePodcast removes a podcast channel and all its episodes; admin only.
func (h *Handler) DeletePodcast(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid podcast id"))
		return
	}

	if err := h.db.Podcasts.DeleteChannel(r.Context(), id); err != nil {
		writeError(w, r, cerr.InternalServerError("delete podcast failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// ListPodcastEpisodes returns all episodes for a podcast channel.
func (h *Handler) ListPodcastEpisodes(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid podcast id"))
		return
	}

	ch, err := h.db.Podcasts.GetChannel(r.Context(), id)
	if err != nil || ch == nil {
		writeError(w, r, cerr.NotFound("podcast not found"))
		return
	}

	episodes, err := h.db.Podcasts.ListEpisodesByChannel(r.Context(), id)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list episodes failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"episodes": episodes,
		"total":    len(episodes),
	})
}

// GetPodcastEpisode returns a single podcast episode by ID.
func (h *Handler) GetPodcastEpisode(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid episode id"))
		return
	}

	ep, err := h.db.Podcasts.GetEpisode(r.Context(), id)
	if err != nil || ep == nil {
		writeError(w, r, cerr.NotFound("episode not found"))
		return
	}

	writeJSON(w, http.StatusOK, ep)
}

// DownloadPodcastEpisode triggers a download of the episode audio; admin only.
func (h *Handler) DownloadPodcastEpisode(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid episode id"))
		return
	}

	ep, err := h.db.Podcasts.GetEpisode(r.Context(), id)
	if err != nil || ep == nil {
		writeError(w, r, cerr.NotFound("episode not found"))
		return
	}

	if err := h.db.Podcasts.UpdateEpisodeStatus(r.Context(), id, model.EpisodeStatusDownloading, ""); err != nil {
		writeError(w, r, cerr.InternalServerError("update episode status failed"))
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{"episode_id": ep.ID, "status": "downloading"})
}

// DeletePodcastEpisode removes a podcast episode; admin only.
func (h *Handler) DeletePodcastEpisode(w http.ResponseWriter, r *http.Request) {
	_ = mw.UserFromContext(r.Context())

	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid episode id"))
		return
	}

	if err := h.db.Podcasts.DeleteEpisode(r.Context(), id); err != nil {
		writeError(w, r, cerr.InternalServerError("delete episode failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}
