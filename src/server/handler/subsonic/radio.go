package subsonic

import (
	"net/http"

	"github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
)

// getInternetRadioStations returns all configured internet radio stations.
func (h *Handler) getInternetRadioStations(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	stations, err := h.db.Users.ListRadioStations(r.Context())
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to list radio stations."))
		return
	}

	stationResps := make([]InternetRadioStation, 0, len(stations))
	for _, s := range stations {
		stationResps = append(stationResps, modelRadioToResp(s))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.RadioStations = &InternetRadioStations{InternetRadioStation: stationResps}
	}))
}

// createInternetRadioStation adds a new internet radio station. Admin only.
func (h *Handler) createInternetRadioStation(w http.ResponseWriter, r *http.Request) {
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
	name := q.Get("name")
	streamURL := q.Get("streamUrl")

	if name == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'name' is missing."))
		return
	}
	if streamURL == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'streamUrl' is missing."))
		return
	}

	station := &model.InternetRadioStation{
		Name:        name,
		StreamURL:   streamURL,
		HomepageURL: q.Get("homepageUrl"),
	}

	_, err := h.db.Users.CreateRadioStation(r.Context(), station)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to create radio station."))
		return
	}

	respond(w, r, ok(nil))
}

// updateInternetRadioStation updates an existing radio station's fields. Admin only.
func (h *Handler) updateInternetRadioStation(w http.ResponseWriter, r *http.Request) {
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
	id := q.Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	dbID, err := decodeRadioID(id)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Radio station not found."))
		return
	}

	ctx := r.Context()
	station, err := h.db.Users.GetRadioStation(ctx, dbID)
	if err != nil || station == nil {
		respond(w, r, errResp(ErrNotFound, "Radio station not found."))
		return
	}

	if v := q.Get("name"); v != "" {
		station.Name = v
	}
	if v := q.Get("streamUrl"); v != "" {
		station.StreamURL = v
	}
	if v := q.Get("homepageUrl"); v != "" {
		station.HomepageURL = v
	}

	if err := h.db.Users.UpdateRadioStation(ctx, station); err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to update radio station."))
		return
	}

	respond(w, r, ok(nil))
}

// deleteInternetRadioStation permanently removes a radio station. Admin only.
func (h *Handler) deleteInternetRadioStation(w http.ResponseWriter, r *http.Request) {
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

	dbID, err := decodeRadioID(id)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Radio station not found."))
		return
	}

	if err := h.db.Users.DeleteRadioStation(r.Context(), dbID); err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to delete radio station."))
		return
	}

	respond(w, r, ok(nil))
}

// jukeboxControl always returns an error; jukebox mode is not supported.
func (h *Handler) jukeboxControl(w http.ResponseWriter, r *http.Request) {
	respond(w, r, errResp(ErrGeneric, "Jukebox is not supported."))
}

// modelRadioToResp converts a model.InternetRadioStation to a Subsonic response element.
func modelRadioToResp(s *model.InternetRadioStation) InternetRadioStation {
	return InternetRadioStation{
		ID:          encodeRadioID(s.ID),
		Name:        s.Name,
		StreamURL:   s.StreamURL,
		HomepageURL: s.HomepageURL,
	}
}
