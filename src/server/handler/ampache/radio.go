package ampache

import (
	"net/http"
	"strconv"

	"github.com/local/cassonic/src/server/model"
)

// liveStreams returns all internet radio stations.
func (h *Handler) liveStreams(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	list, err := h.db.Users.ListRadioStations(r.Context())
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	result := make([]AmpLiveStream, 0, len(list))
	for _, s := range list {
		result = append(result, radioToAmp(s))
	}
	respond(w, r, isJSON, okResp("live_stream", result))
}

// liveStream returns a single internet radio station by ID.
func (h *Handler) liveStream(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (station ID)"))
		return
	}

	s, err := h.db.Users.GetRadioStation(r.Context(), id)
	if err != nil || s == nil {
		respond(w, r, isJSON, errResp(4704, "Not found"))
		return
	}

	respond(w, r, isJSON, radioToAmp(s))
}

// liveStreamCreate creates a new internet radio station. Admin only.
func (h *Handler) liveStreamCreate(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	name := param(r, "name")
	streamURL := param(r, "url")

	if name == "" || streamURL == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: name and url are required"))
		return
	}

	s := &model.InternetRadioStation{
		Name:        name,
		StreamURL:   streamURL,
		HomepageURL: param(r, "site_url"),
	}

	id, err := h.db.Users.CreateRadioStation(r.Context(), s)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to create station: "+err.Error()))
		return
	}

	s.ID = id
	respond(w, r, isJSON, radioToAmp(s))
}

// liveStreamEdit modifies an existing internet radio station. Admin only.
func (h *Handler) liveStreamEdit(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (station ID)"))
		return
	}

	s, err := h.db.Users.GetRadioStation(r.Context(), id)
	if err != nil || s == nil {
		respond(w, r, isJSON, errResp(4704, "Not found"))
		return
	}

	if v := param(r, "name"); v != "" {
		s.Name = v
	}
	if v := param(r, "url"); v != "" {
		s.StreamURL = v
	}
	if v := param(r, "site_url"); v != "" {
		s.HomepageURL = v
	}

	if err := h.db.Users.UpdateRadioStation(r.Context(), s); err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to update station: "+err.Error()))
		return
	}

	respond(w, r, isJSON, radioToAmp(s))
}

// liveStreamDelete removes an internet radio station. Admin only.
func (h *Handler) liveStreamDelete(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (station ID)"))
		return
	}

	if err := h.db.Users.DeleteRadioStation(r.Context(), id); err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to delete station: "+err.Error()))
		return
	}

	respond(w, r, isJSON, okResp("success", "station deleted"))
}

// radioToAmp converts a model.InternetRadioStation to the Ampache wire type.
func radioToAmp(s *model.InternetRadioStation) AmpLiveStream {
	return AmpLiveStream{
		ID:       strconv.FormatInt(s.ID, 10),
		Name:     s.Name,
		Codec:    "mp3",
		URL:      s.StreamURL,
		SiteURL:  s.HomepageURL,
		IsPublic: 1,
	}
}
