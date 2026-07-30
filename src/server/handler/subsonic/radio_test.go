package subsonic

// Tests for radio.go handler methods: getInternetRadioStations,
// createInternetRadioStation, updateInternetRadioStation,
// deleteInternetRadioStation, jukeboxControl, and modelRadioToResp.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// ---- getInternetRadioStations ------------------------------------------------

func TestGetInternetRadioStationsUnauthenticated(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getInternetRadioStations?f=json", nil)

	h.getInternetRadioStations(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getInternetRadioStations unauthenticated: got %+v", resp.Error)
	}
}

func TestGetInternetRadioStationsSuccess(t *testing.T) {
	users := &stubUserStore{
		radioStations: []*model.InternetRadioStation{
			{ID: 1, Name: "Station A", StreamURL: "http://a.example.com"},
			{ID: 2, Name: "Station B", StreamURL: "http://b.example.com"},
		},
	}
	h := newTestHandlerFull(nil, users, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getInternetRadioStations?f=json", false)

	h.getInternetRadioStations(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.RadioStations == nil {
		t.Fatal("getInternetRadioStations: RadioStations is nil")
	}
	if len(resp.RadioStations.InternetRadioStation) != 2 {
		t.Fatalf("getInternetRadioStations: got %d stations, want 2",
			len(resp.RadioStations.InternetRadioStation))
	}
	if resp.RadioStations.InternetRadioStation[0].Name != "Station A" {
		t.Errorf("getInternetRadioStations: Name = %q, want Station A",
			resp.RadioStations.InternetRadioStation[0].Name)
	}
}

func TestGetInternetRadioStationsStoreError(t *testing.T) {
	users := &stubUserStore{radioListErr: errFake("db down")}
	h := newTestHandlerFull(nil, users, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getInternetRadioStations?f=json", false)

	h.getInternetRadioStations(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("getInternetRadioStations store error: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

// ---- createInternetRadioStation (admin only) --------------------------------

func TestCreateInternetRadioStationForbiddenNonAdmin(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet,
		"/rest/createInternetRadioStation?f=json&name=X&streamUrl=http://x.example.com", false)

	h.createInternetRadioStation(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrForbidden {
		t.Errorf("createInternetRadioStation non-admin: got %+v, want code %d", resp.Error, ErrForbidden)
	}
}

func TestCreateInternetRadioStationMissingName(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet,
		"/rest/createInternetRadioStation?f=json&streamUrl=http://x.example.com", true)

	h.createInternetRadioStation(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("createInternetRadioStation missing name: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestCreateInternetRadioStationMissingStreamURL(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/createInternetRadioStation?f=json&name=X", true)

	h.createInternetRadioStation(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("createInternetRadioStation missing streamUrl: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestCreateInternetRadioStationSuccess(t *testing.T) {
	users := &stubUserStore{}
	h := newTestHandlerFull(nil, users, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet,
		"/rest/createInternetRadioStation?f=json&name=MyStation&streamUrl=http://x.example.com&homepageUrl=http://x.example.com/home",
		true)

	h.createInternetRadioStation(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("createInternetRadioStation: status %q, want ok", resp.Status)
	}
	if users.createdRadio == nil {
		t.Fatal("createInternetRadioStation: no station was created")
	}
	if users.createdRadio.Name != "MyStation" {
		t.Errorf("createInternetRadioStation: Name = %q, want MyStation", users.createdRadio.Name)
	}
	if users.createdRadio.HomepageURL != "http://x.example.com/home" {
		t.Errorf("createInternetRadioStation: HomepageURL = %q, want http://x.example.com/home",
			users.createdRadio.HomepageURL)
	}
}

func TestCreateInternetRadioStationStoreError(t *testing.T) {
	users := &stubUserStore{radioCreateErr: errFake("db down")}
	h := newTestHandlerFull(nil, users, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet,
		"/rest/createInternetRadioStation?f=json&name=X&streamUrl=http://x.example.com", true)

	h.createInternetRadioStation(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("createInternetRadioStation store error: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

// ---- updateInternetRadioStation (admin only) --------------------------------

func TestUpdateInternetRadioStationMissingID(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/updateInternetRadioStation?f=json", true)

	h.updateInternetRadioStation(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("updateInternetRadioStation missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestUpdateInternetRadioStationNotFound(t *testing.T) {
	users := &stubUserStore{radioGetErr: errFake("not found")}
	h := newTestHandlerFull(nil, users, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet,
		"/rest/updateInternetRadioStation?f=json&id="+encodeRadioID(1)+"&name=NewName", true)

	h.updateInternetRadioStation(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("updateInternetRadioStation not found: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestUpdateInternetRadioStationSuccess(t *testing.T) {
	users := &stubUserStore{
		radioStation: &model.InternetRadioStation{ID: 1, Name: "Old", StreamURL: "http://old.example.com"},
	}
	h := newTestHandlerFull(nil, users, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet,
		"/rest/updateInternetRadioStation?f=json&id="+encodeRadioID(1)+"&name=New&streamUrl=http://new.example.com",
		true)

	h.updateInternetRadioStation(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("updateInternetRadioStation: status %q, want ok", resp.Status)
	}
	if users.updatedRadio == nil {
		t.Fatal("updateInternetRadioStation: no station was updated")
	}
	if users.updatedRadio.Name != "New" {
		t.Errorf("updateInternetRadioStation: Name = %q, want New", users.updatedRadio.Name)
	}
	if users.updatedRadio.StreamURL != "http://new.example.com" {
		t.Errorf("updateInternetRadioStation: StreamURL = %q, want http://new.example.com",
			users.updatedRadio.StreamURL)
	}
}

// ---- deleteInternetRadioStation (admin only) --------------------------------

func TestDeleteInternetRadioStationForbiddenNonAdmin(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deleteInternetRadioStation?f=json&id="+encodeRadioID(1), false)

	h.deleteInternetRadioStation(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrForbidden {
		t.Errorf("deleteInternetRadioStation non-admin: got %+v, want code %d", resp.Error, ErrForbidden)
	}
}

func TestDeleteInternetRadioStationMalformedID(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deleteInternetRadioStation?f=json&id=notanid", true)

	h.deleteInternetRadioStation(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("deleteInternetRadioStation malformed id: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestDeleteInternetRadioStationSuccess(t *testing.T) {
	users := &stubUserStore{}
	h := newTestHandlerFull(nil, users, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deleteInternetRadioStation?f=json&id="+encodeRadioID(9), true)

	h.deleteInternetRadioStation(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("deleteInternetRadioStation: status %q, want ok", resp.Status)
	}
	if users.deletedRadioID != 9 {
		t.Errorf("deleteInternetRadioStation: deletedRadioID = %d, want 9", users.deletedRadioID)
	}
}

func TestDeleteInternetRadioStationStoreError(t *testing.T) {
	users := &stubUserStore{radioDeleteErr: errFake("db down")}
	h := newTestHandlerFull(nil, users, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deleteInternetRadioStation?f=json&id="+encodeRadioID(1), true)

	h.deleteInternetRadioStation(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("deleteInternetRadioStation store error: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

// ---- jukeboxControl -----------------------------------------------------------

func TestJukeboxControlAlwaysUnsupported(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/jukeboxControl?f=json", false)

	h.jukeboxControl(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("jukeboxControl: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

// ---- modelRadioToResp ----------------------------------------------------------

func TestModelRadioToResp(t *testing.T) {
	s := &model.InternetRadioStation{
		ID:          5,
		Name:        "Station",
		StreamURL:   "http://stream.example.com",
		HomepageURL: "http://home.example.com",
	}
	resp := modelRadioToResp(s)
	if resp.ID != encodeRadioID(5) {
		t.Errorf("modelRadioToResp: ID = %q, want %q", resp.ID, encodeRadioID(5))
	}
	if resp.Name != "Station" {
		t.Errorf("modelRadioToResp: Name = %q, want Station", resp.Name)
	}
	if resp.StreamURL != "http://stream.example.com" {
		t.Errorf("modelRadioToResp: StreamURL = %q, want http://stream.example.com", resp.StreamURL)
	}
}
