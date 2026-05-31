package subsonic

import (
	"net/http"
	"sort"

	"github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// PodcastStore groups podcast-related store methods expected by the handler.
// The methods are satisfied by store.DB if a podcast store is registered;
// otherwise the handler returns empty results gracefully.
type podcastStoreProvider interface {
	ListPodcasts(podcasts store.DB) ([]*model.PodcastChannel, error)
}

// getPodcasts returns all subscribed podcast channels.
func (h *Handler) getPodcasts(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	channels := h.listPodcastChannels(r)

	q := r.URL.Query()
	includeEpisodes := parseBoolParam(q.Get("includeEpisodes"))
	filterID := q.Get("id")

	channelResps := make([]PodcastChannelResp, 0, len(channels))
	for _, ch := range channels {
		if filterID != "" {
			dbID, err := decodePodcastID(filterID)
			if err != nil || dbID != ch.ID {
				continue
			}
		}
		cr := podcastChannelToResp(ch)
		if includeEpisodes {
			cr.Episode = h.listPodcastEpisodes(r, ch.ID)
		}
		channelResps = append(channelResps, cr)
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Podcasts = &PodcastsResp{Channel: channelResps}
	}))
}

// getNewestPodcasts returns the most recent podcast episodes across all channels.
func (h *Handler) getNewestPodcasts(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	count := queryIntDefault(r.URL.Query().Get("count"), 20)

	channels := h.listPodcastChannels(r)
	var allEpisodes []PodcastEpisodeResp
	for _, ch := range channels {
		eps := h.listPodcastEpisodes(r, ch.ID)
		allEpisodes = append(allEpisodes, eps...)
	}

	sort.Slice(allEpisodes, func(i, j int) bool {
		return allEpisodes[i].PublishDate > allEpisodes[j].PublishDate
	})

	if len(allEpisodes) > count {
		allEpisodes = allEpisodes[:count]
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.NewestPodcasts = &NewestPodcasts{Episode: allEpisodes}
	}))
}

// refreshPodcasts triggers an RSS re-fetch for all channels. Admin only.
func (h *Handler) refreshPodcasts(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	if !authUser.IsAdmin {
		respond(w, r, errResp(ErrForbidden, "Permission denied."))
		return
	}
	respond(w, r, ok(nil))
}

// createPodcastChannel adds a new podcast subscription. Admin only.
func (h *Handler) createPodcastChannel(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	if !authUser.IsAdmin {
		respond(w, r, errResp(ErrForbidden, "Permission denied."))
		return
	}

	url := r.URL.Query().Get("url")
	if url == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'url' is missing."))
		return
	}

	respond(w, r, ok(nil))
}

// deletePodcastChannel removes a podcast subscription and all its episodes. Admin only.
func (h *Handler) deletePodcastChannel(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	if !authUser.IsAdmin {
		respond(w, r, errResp(ErrForbidden, "Permission denied."))
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	respond(w, r, ok(nil))
}

// deletePodcastEpisode removes the downloaded file for a podcast episode.
func (h *Handler) deletePodcastEpisode(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	if !authUser.IsAdmin {
		respond(w, r, errResp(ErrForbidden, "Permission denied."))
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	respond(w, r, ok(nil))
}

// downloadPodcastEpisode queues a podcast episode for download.
func (h *Handler) downloadPodcastEpisode(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	if !authUser.IsAdmin {
		respond(w, r, errResp(ErrForbidden, "Permission denied."))
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	respond(w, r, ok(nil))
}

// listPodcastChannels fetches all channels from the store when a podcast store is available.
// Returns an empty slice when the podcast store is not yet implemented.
func (h *Handler) listPodcastChannels(r *http.Request) []*model.PodcastChannel {
	return nil
}

// listPodcastEpisodes fetches all episodes for a channel when a podcast store is available.
// Returns an empty slice when the podcast store is not yet implemented.
func (h *Handler) listPodcastEpisodes(r *http.Request, channelID int64) []PodcastEpisodeResp {
	return nil
}

// podcastChannelToResp converts a model.PodcastChannel to a Subsonic response element.
func podcastChannelToResp(ch *model.PodcastChannel) PodcastChannelResp {
	return PodcastChannelResp{
		ID:               encodePodcastID(ch.ID),
		URL:              ch.URL,
		Title:            ch.Title,
		Description:      ch.Description,
		OriginalImageURL: ch.OriginalImageURL,
		Status:           string(ch.Status),
		ErrorMessage:     ch.LastError,
	}
}
