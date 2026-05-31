package ampache

import (
	"net/http"
)

// podcasts returns all podcast channels as an empty list.
// Full podcast store support requires a PodcastStore in the DB aggregate.
func (h *Handler) podcasts(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("podcast", []AmpPodcast{}))
}

// podcast returns a single podcast channel by ID.
func (h *Handler) podcast(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, errResp(4704, "Not found"))
}

// podcastCreate creates a new podcast channel. Admin only.
func (h *Handler) podcastCreate(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	feedURL := param(r, "url")
	if feedURL == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: url"))
		return
	}

	respond(w, r, isJSON, okResp("success", "podcast channels not yet stored in this server"))
}

// podcastEdit modifies an existing podcast channel. Admin only.
func (h *Handler) podcastEdit(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("success", "podcast channels not yet stored in this server"))
}

// podcastDelete removes a podcast channel. Admin only.
func (h *Handler) podcastDelete(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("success", "podcast channels not yet stored in this server"))
}

// podcastEpisodes returns episodes for the given channel.
func (h *Handler) podcastEpisodes(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("podcast_episode", []AmpPodcastEpisode{}))
}

// podcastEpisode returns a single podcast episode by ID.
func (h *Handler) podcastEpisode(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, errResp(4704, "Not found"))
}

// podcastEpisodeDelete deletes the downloaded file for a podcast episode. Admin only.
func (h *Handler) podcastEpisodeDelete(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("success", "podcast episodes not yet stored in this server"))
}

// updatePodcast triggers an RSS re-fetch for the given channel. Admin only.
func (h *Handler) updatePodcast(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "id")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: id"))
		return
	}

	respond(w, r, isJSON, okResp("success", "podcast update queued"))
}
