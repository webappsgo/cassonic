package subsonic

// Tests for system endpoints (ping, getLicense), response helpers (ok, errResp,
// respond), and getMusicFolders.
//
// Strategy: all handler tests call handler methods directly (bypassing the chi
// router and any auth middleware) and inject an AuthUser into the request context
// via middleware.WithUser when the handler requires authentication.  This keeps
// tests fast, deterministic, and free of real database dependencies by using
// minimal stub implementations of MusicStore and UserStore.

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// ---- minimal stubs --------------------------------------------------------

// stubMusicStore implements store.MusicStore. All methods return empty results
// or nil errors unless overridden via the corresponding exported field.
type stubMusicStore struct {
	libraries    []*model.Library
	libraryErr   error
	scanStatus   *model.ScanStatus
	scanErr      error
}

func (s *stubMusicStore) CreateLibrary(_ context.Context, _ *model.Library) (int64, error) {
	return 0, errors.New("not implemented")
}
func (s *stubMusicStore) GetLibrary(_ context.Context, _ int64) (*model.Library, error) {
	return nil, errors.New("not implemented")
}
func (s *stubMusicStore) ListLibraries(_ context.Context) ([]*model.Library, error) {
	return s.libraries, s.libraryErr
}
func (s *stubMusicStore) UpdateLibrary(_ context.Context, _ *model.Library) error {
	return errors.New("not implemented")
}
func (s *stubMusicStore) DeleteLibrary(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}
func (s *stubMusicStore) UpsertArtist(_ context.Context, _ *model.Artist) (int64, error) {
	return 0, errors.New("not implemented")
}
func (s *stubMusicStore) GetArtist(_ context.Context, _ int64) (*model.Artist, error) {
	return nil, errors.New("not implemented")
}
func (s *stubMusicStore) GetArtistByName(_ context.Context, _ string) (*model.Artist, error) {
	return nil, errors.New("not implemented")
}
func (s *stubMusicStore) ListArtists(_ context.Context, _ store.ListOpts) ([]*model.Artist, error) {
	return nil, nil
}
func (s *stubMusicStore) SearchArtists(_ context.Context, _ string, _ store.ListOpts) ([]*model.Artist, error) {
	return nil, nil
}
func (s *stubMusicStore) DeleteArtistsWithNoSongs(_ context.Context) error {
	return errors.New("not implemented")
}
func (s *stubMusicStore) UpsertAlbum(_ context.Context, _ *model.Album) (int64, error) {
	return 0, errors.New("not implemented")
}
func (s *stubMusicStore) GetAlbum(_ context.Context, _ int64) (*model.Album, error) {
	return nil, errors.New("not implemented")
}
func (s *stubMusicStore) ListAlbums(_ context.Context, _ store.ListOpts) ([]*model.Album, error) {
	return nil, nil
}
func (s *stubMusicStore) ListAlbumsByArtist(_ context.Context, _ int64) ([]*model.Album, error) {
	return nil, nil
}
func (s *stubMusicStore) GetNewestAlbums(_ context.Context, _ int) ([]*model.Album, error) {
	return nil, nil
}
func (s *stubMusicStore) GetRandomAlbums(_ context.Context, _ int) ([]*model.Album, error) {
	return nil, nil
}
func (s *stubMusicStore) SearchAlbums(_ context.Context, _ string, _ store.ListOpts) ([]*model.Album, error) {
	return nil, nil
}
func (s *stubMusicStore) DeleteAlbumsWithNoSongs(_ context.Context) error {
	return errors.New("not implemented")
}
func (s *stubMusicStore) UpsertSong(_ context.Context, _ *model.Song) (int64, error) {
	return 0, errors.New("not implemented")
}
func (s *stubMusicStore) GetSong(_ context.Context, _ int64) (*model.Song, error) {
	return nil, errors.New("not implemented")
}
func (s *stubMusicStore) GetSongByPath(_ context.Context, _ string) (*model.Song, error) {
	return nil, errors.New("not implemented")
}
func (s *stubMusicStore) ListSongsByAlbum(_ context.Context, _ int64) ([]*model.Song, error) {
	return nil, nil
}
func (s *stubMusicStore) ListSongsByArtist(_ context.Context, _ int64) ([]*model.Song, error) {
	return nil, nil
}
func (s *stubMusicStore) ListSongsByGenre(_ context.Context, _ string, _ store.ListOpts) ([]*model.Song, error) {
	return nil, nil
}
func (s *stubMusicStore) GetRandomSongs(_ context.Context, _ int, _, _, _ string) ([]*model.Song, error) {
	return nil, nil
}
func (s *stubMusicStore) SearchSongs(_ context.Context, _ string, _ store.ListOpts) ([]*model.Song, error) {
	return nil, nil
}
func (s *stubMusicStore) MarkSongMissing(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}
func (s *stubMusicStore) DeleteMissingSongs(_ context.Context) error {
	return errors.New("not implemented")
}
func (s *stubMusicStore) IncrementPlayCount(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}
func (s *stubMusicStore) ListGenres(_ context.Context) ([]*model.Genre, error) {
	return nil, nil
}
func (s *stubMusicStore) UpsertCoverArt(_ context.Context, _ *model.CoverArt) (int64, error) {
	return 0, errors.New("not implemented")
}
func (s *stubMusicStore) GetCoverArt(_ context.Context, _ int64) (*model.CoverArt, error) {
	return nil, errors.New("not implemented")
}
func (s *stubMusicStore) CreateScanStatus(_ context.Context, _ *model.ScanStatus) (int64, error) {
	return 0, errors.New("not implemented")
}
func (s *stubMusicStore) UpdateScanStatus(_ context.Context, _ *model.ScanStatus) error {
	return errors.New("not implemented")
}
func (s *stubMusicStore) GetLastScanStatus(_ context.Context) (*model.ScanStatus, error) {
	return s.scanStatus, s.scanErr
}

// stubUserStore satisfies store.UserStore with no-op implementations.
type stubUserStore struct{}

func (s *stubUserStore) CreateUser(_ context.Context, _ *model.User) (int64, error) {
	return 0, errors.New("not implemented")
}
func (s *stubUserStore) GetUser(_ context.Context, _ int64) (*model.User, error) {
	return nil, errors.New("not found")
}
func (s *stubUserStore) GetUserByUsername(_ context.Context, _ string) (*model.User, error) {
	return nil, errors.New("not found")
}
func (s *stubUserStore) GetUserByEmail(_ context.Context, _ string) (*model.User, error) {
	return nil, errors.New("not found")
}
func (s *stubUserStore) UpdateUser(_ context.Context, _ *model.User) error {
	return errors.New("not implemented")
}
func (s *stubUserStore) DeleteUser(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}
func (s *stubUserStore) ListUsers(_ context.Context) ([]*model.User, error) {
	return nil, nil
}
func (s *stubUserStore) IncrementLoginAttempts(_ context.Context, _ int64) error { return nil }
func (s *stubUserStore) ResetLoginAttempts(_ context.Context, _ int64) error     { return nil }
func (s *stubUserStore) SetLockedUntil(_ context.Context, _ int64, _ time.Time) error {
	return nil
}
func (s *stubUserStore) UpdateLastLogin(_ context.Context, _ int64) error { return nil }
func (s *stubUserStore) CreateAPIToken(_ context.Context, _ *model.APIToken) error {
	return errors.New("not implemented")
}
func (s *stubUserStore) GetAPITokenByHash(_ context.Context, _ string) (*model.APIToken, error) {
	return nil, errors.New("not found")
}
func (s *stubUserStore) ListAPITokens(_ context.Context, _ int64) ([]*model.APIToken, error) {
	return nil, nil
}
func (s *stubUserStore) DeleteAPIToken(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}
func (s *stubUserStore) UpdateAPITokenLastUsed(_ context.Context, _ int64) error { return nil }
func (s *stubUserStore) CreateSession(_ context.Context, _ int64, _ string, _ time.Time, _ string) error {
	return errors.New("not implemented")
}
func (s *stubUserStore) GetSessionByHash(_ context.Context, _ string) (*store.Session, error) {
	return nil, errors.New("not found")
}
func (s *stubUserStore) DeleteSession(_ context.Context, _ string) error {
	return errors.New("not implemented")
}
func (s *stubUserStore) DeleteUserSessions(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}
func (s *stubUserStore) PurgeExpiredSessions(_ context.Context) error { return nil }
func (s *stubUserStore) GetSubsonicPassword(_ context.Context, _ string) (string, bool, error) {
	return "", false, nil
}
func (s *stubUserStore) SetSubsonicPassword(_ context.Context, _, _ string) error {
	return errors.New("not implemented")
}
func (s *stubUserStore) CreateRadioStation(_ context.Context, _ *model.InternetRadioStation) (int64, error) {
	return 0, errors.New("not implemented")
}
func (s *stubUserStore) GetRadioStation(_ context.Context, _ int64) (*model.InternetRadioStation, error) {
	return nil, errors.New("not found")
}
func (s *stubUserStore) ListRadioStations(_ context.Context) ([]*model.InternetRadioStation, error) {
	return nil, nil
}
func (s *stubUserStore) UpdateRadioStation(_ context.Context, _ *model.InternetRadioStation) error {
	return errors.New("not implemented")
}
func (s *stubUserStore) DeleteRadioStation(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}

// ---- helpers ---------------------------------------------------------------

// newTestHandler builds a Handler backed entirely by stub stores.
func newTestHandler(music *stubMusicStore) *Handler {
	db := &store.DB{
		Users: &stubUserStore{},
		Music: music,
	}
	return &Handler{
		db:         db,
		nowPlaying: NewNowPlayingTracker(),
		subsPass:   func(_ context.Context, _ string) (string, bool) { return "", false },
	}
}

// authedRequest creates an httptest request and injects an AuthUser into its
// context, simulating a successfully authenticated Subsonic session.
func authedRequest(method, target string, admin bool) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	u := &middleware.AuthUser{
		ID:       1,
		Username: "testuser",
		IsAdmin:  admin,
		Scheme:   middleware.SchemeSubsonic,
	}
	r = r.WithContext(middleware.WithUser(r.Context(), u))
	return r
}

// parseXMLResponse decodes the response body as a SubsonicResponse.
func parseXMLResponse(t *testing.T, body string) *SubsonicResponse {
	t.Helper()
	var resp SubsonicResponse
	if err := xml.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("xml.Unmarshal: %v\nbody: %s", err, body)
	}
	return &resp
}

// parseJSONResponse decodes the response body as the jsonWrapper envelope and
// returns the inner SubsonicResponse.
func parseJSONResponse(t *testing.T, body string) *SubsonicResponse {
	t.Helper()
	var w jsonWrapper
	if err := json.Unmarshal([]byte(body), &w); err != nil {
		t.Fatalf("json.Unmarshal: %v\nbody: %s", err, body)
	}
	if w.SubsonicResponse == nil {
		t.Fatalf("json response missing subsonic-response key; body: %s", body)
	}
	return w.SubsonicResponse
}

// ---- response helper unit tests -------------------------------------------

// TestOkBuildsCorrectResponse verifies ok() sets status="ok" and the right version.
func TestOkBuildsCorrectResponse(t *testing.T) {
	resp := ok(nil)
	if resp.Status != "ok" {
		t.Errorf("ok().Status = %q, want %q", resp.Status, "ok")
	}
	if resp.Version != SubsonicVersion {
		t.Errorf("ok().Version = %q, want %q", resp.Version, SubsonicVersion)
	}
	if resp.XMLNS != XMLNamespace {
		t.Errorf("ok().XMLNS = %q, want %q", resp.XMLNS, XMLNamespace)
	}
}

// TestOkAppliesPayloadFunc verifies that the payload function mutates the response.
func TestOkAppliesPayloadFunc(t *testing.T) {
	resp := ok(func(r *SubsonicResponse) {
		r.License = &License{Valid: true}
	})
	if resp.License == nil || !resp.License.Valid {
		t.Errorf("ok() payload not applied: License = %+v", resp.License)
	}
}

// TestOkNilPayload verifies ok(nil) does not panic.
func TestOkNilPayload(t *testing.T) {
	resp := ok(nil)
	if resp.Error != nil {
		t.Errorf("ok(nil).Error should be nil, got %+v", resp.Error)
	}
}

// TestErrRespBuildsCorrectResponse verifies errResp sets status="failed" and the right code.
func TestErrRespBuildsCorrectResponse(t *testing.T) {
	resp := errResp(ErrMissingParam, "missing param")
	if resp.Status != "failed" {
		t.Errorf("errResp().Status = %q, want %q", resp.Status, "failed")
	}
	if resp.Error == nil {
		t.Fatal("errResp().Error is nil")
	}
	if resp.Error.Code != ErrMissingParam {
		t.Errorf("errResp().Error.Code = %d, want %d", resp.Error.Code, ErrMissingParam)
	}
	if resp.Error.Message != "missing param" {
		t.Errorf("errResp().Error.Message = %q, want %q", resp.Error.Message, "missing param")
	}
}

// TestErrRespAllCodes verifies every defined error code round-trips correctly.
func TestErrRespAllCodes(t *testing.T) {
	codes := []int{
		ErrGeneric, ErrMissingParam, ErrWrongVersion, ErrNotAuthenticated,
		ErrWrongCredentials, ErrTokenNotSupported, ErrForbidden, ErrNotFound,
	}
	for _, code := range codes {
		resp := errResp(code, "msg")
		if resp.Error.Code != code {
			t.Errorf("code %d: round-trip got %d", code, resp.Error.Code)
		}
	}
}

// TestSubsonicResponseXMLMarshal verifies the XML output matches the Subsonic
// wire format (element name, xmlns attribute, status attribute).
func TestSubsonicResponseXMLMarshal(t *testing.T) {
	resp := ok(nil)
	data, err := xml.Marshal(resp)
	if err != nil {
		t.Fatalf("xml.Marshal: %v", err)
	}
	s := string(data)

	if !strings.Contains(s, `<subsonic-response`) {
		t.Errorf("XML missing <subsonic-response element; got: %s", s)
	}
	if !strings.Contains(s, `status="ok"`) {
		t.Errorf("XML missing status=\"ok\"; got: %s", s)
	}
	if !strings.Contains(s, XMLNamespace) {
		t.Errorf("XML missing xmlns %q; got: %s", XMLNamespace, s)
	}
}

// TestSubsonicErrorXMLMarshal verifies that error attributes appear in the output.
func TestSubsonicErrorXMLMarshal(t *testing.T) {
	resp := errResp(ErrWrongCredentials, "wrong credentials")
	data, err := xml.Marshal(resp)
	if err != nil {
		t.Fatalf("xml.Marshal: %v", err)
	}
	s := string(data)

	if !strings.Contains(s, `status="failed"`) {
		t.Errorf("XML missing status=\"failed\"; got: %s", s)
	}
	if !strings.Contains(s, `code="40"`) {
		t.Errorf("XML missing code=\"40\"; got: %s", s)
	}
}

// TestRespondXMLDefault verifies respond() writes XML when ?f= is absent.
func TestRespondXMLDefault(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/ping", nil)
	respond(rec, r, ok(nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/xml") {
		t.Errorf("Content-Type: got %q, want text/xml", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<subsonic-response") {
		t.Errorf("body missing <subsonic-response; got: %s", body)
	}
}

// TestRespondJSONFormat verifies respond() writes JSON when ?f=json.
func TestRespondJSONFormat(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/ping?f=json", nil)
	respond(rec, r, ok(nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"subsonic-response"`) {
		t.Errorf("body missing subsonic-response key; got: %s", body)
	}
	resp := parseJSONResponse(t, body)
	if resp.Status != "ok" {
		t.Errorf("JSON status: got %q, want %q", resp.Status, "ok")
	}
}

// TestRespondJSONPFormat verifies respond() wraps JSON in the JSONP callback when ?f=jsonp.
func TestRespondJSONPFormat(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/ping?f=jsonp&callback=myFn", nil)
	respond(rec, r, ok(nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.HasPrefix(body, "myFn(") {
		t.Errorf("JSONP body should start with 'myFn('; got: %s", body)
	}
	if !strings.HasSuffix(strings.TrimSpace(body), ")") {
		t.Errorf("JSONP body should end with ')'; got: %s", body)
	}
}

// TestRespondJSONPDefaultCallback verifies that missing ?callback= falls back to "callback".
func TestRespondJSONPDefaultCallback(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/ping?f=jsonp", nil)
	respond(rec, r, ok(nil))

	body := rec.Body.String()
	if !strings.HasPrefix(body, "callback(") {
		t.Errorf("JSONP default callback: got: %s", body)
	}
}

// TestRespondAlwaysHTTP200 verifies that even error responses use HTTP 200
// (the Subsonic protocol embeds errors in the body, not the HTTP status).
func TestRespondAlwaysHTTP200(t *testing.T) {
	formats := []string{"", "json", "jsonp"}
	for _, f := range formats {
		t.Run("format="+f, func(t *testing.T) {
			rec := httptest.NewRecorder()
			url := "/rest/ping"
			if f != "" {
				url += "?f=" + f
			}
			r := httptest.NewRequest(http.MethodGet, url, nil)
			respond(rec, r, errResp(ErrWrongCredentials, "bad"))
			if rec.Code != http.StatusOK {
				t.Errorf("format=%q: got HTTP %d, want 200", f, rec.Code)
			}
		})
	}
}

// TestRespondJSONErrorResponse verifies error fields appear in JSON body.
func TestRespondJSONErrorResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/ping?f=json", nil)
	respond(rec, r, errResp(ErrForbidden, "permission denied"))

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "failed" {
		t.Errorf("JSON error status: got %q, want %q", resp.Status, "failed")
	}
	if resp.Error == nil {
		t.Fatal("JSON error resp.Error is nil")
	}
	if resp.Error.Code != ErrForbidden {
		t.Errorf("JSON error code: got %d, want %d", resp.Error.Code, ErrForbidden)
	}
}

// ---- system endpoint tests -------------------------------------------------

// TestPingXMLReturns200WithOK verifies ping returns HTTP 200 and status="ok" in XML.
func TestPingXMLReturns200WithOK(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/ping", false)

	h.ping(rec, r)

	if rec.Code != http.StatusOK {
		t.Errorf("ping XML: got HTTP %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `status="ok"`) {
		t.Errorf("ping XML: body missing status=\"ok\"; got: %s", body)
	}
	resp := parseXMLResponse(t, strings.TrimPrefix(body, xml.Header))
	if resp.Status != "ok" {
		t.Errorf("ping XML parsed status: got %q, want %q", resp.Status, "ok")
	}
	if resp.Error != nil {
		t.Errorf("ping XML: unexpected error: %+v", resp.Error)
	}
}

// TestPingJSONReturnsOKStatus verifies ping with ?f=json returns JSON status="ok".
func TestPingJSONReturnsOKStatus(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/ping?f=json", false)

	h.ping(rec, r)

	if rec.Code != http.StatusOK {
		t.Errorf("ping JSON: got HTTP %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"subsonic-response"`) {
		t.Errorf("ping JSON: missing subsonic-response key; got: %s", body)
	}
	resp := parseJSONResponse(t, body)
	if resp.Status != "ok" {
		t.Errorf("ping JSON status: got %q, want %q", resp.Status, "ok")
	}
}

// TestPingNoPayloadFields verifies ping does not set License, Error, or any
// optional field in the response body.
func TestPingNoPayloadFields(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/ping?f=json", false)

	h.ping(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.License != nil {
		t.Errorf("ping should not include license; got: %+v", resp.License)
	}
	if resp.MusicFolders != nil {
		t.Errorf("ping should not include musicFolders; got: %+v", resp.MusicFolders)
	}
}

// TestPingUnauthenticated verifies that ping succeeds even without an auth user
// (ping is the connectivity check; the Subsonic auth middleware is a separate layer).
func TestPingUnauthenticated(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/ping", nil)

	h.ping(rec, r)

	if rec.Code != http.StatusOK {
		t.Errorf("ping unauthenticated: got HTTP %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	resp := parseXMLResponse(t, strings.TrimPrefix(body, xml.Header))
	if resp.Status != "ok" {
		t.Errorf("ping unauthenticated status: got %q, want %q", resp.Status, "ok")
	}
}

// TestPingVersionMatchesConstant verifies the Version field in the response
// matches SubsonicVersion.
func TestPingVersionMatchesConstant(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/ping?f=json", false)

	h.ping(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Version != SubsonicVersion {
		t.Errorf("ping version: got %q, want %q", resp.Version, SubsonicVersion)
	}
}

// TestGetLicenseXMLReturnsValidTrue verifies getLicense XML contains valid="true".
func TestGetLicenseXMLReturnsValidTrue(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getLicense", false)

	h.getLicense(rec, r)

	if rec.Code != http.StatusOK {
		t.Errorf("getLicense XML: got HTTP %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `valid="true"`) {
		t.Errorf("getLicense XML: missing valid=\"true\"; got: %s", body)
	}
}

// TestGetLicenseXMLParsed verifies the License struct has Valid=true after parsing.
func TestGetLicenseXMLParsed(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getLicense", false)

	h.getLicense(rec, r)

	resp := parseXMLResponse(t, strings.TrimPrefix(rec.Body.String(), xml.Header))
	if resp.Status != "ok" {
		t.Errorf("getLicense XML status: got %q, want %q", resp.Status, "ok")
	}
	if resp.License == nil {
		t.Fatal("getLicense XML: License field is nil")
	}
	if !resp.License.Valid {
		t.Errorf("getLicense XML: License.Valid = false, want true")
	}
}

// TestGetLicenseJSONReturnsValid verifies getLicense JSON has license.valid=true.
func TestGetLicenseJSONReturnsValid(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getLicense?f=json", false)

	h.getLicense(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.License == nil {
		t.Fatal("getLicense JSON: License field is nil")
	}
	if !resp.License.Valid {
		t.Errorf("getLicense JSON: License.Valid = false, want true")
	}
}

// TestGetLicenseJSONEmailEmpty verifies getLicense JSON does not expose an email.
func TestGetLicenseJSONEmailEmpty(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getLicense?f=json", false)

	h.getLicense(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.License.Email != "" {
		t.Errorf("getLicense: Email should be empty, got %q", resp.License.Email)
	}
}

// TestGetMusicFoldersEmptyLibrary verifies getMusicFolders returns HTTP 200
// with an empty musicFolders list when no library roots are configured.
func TestGetMusicFoldersEmptyLibrary(t *testing.T) {
	h := newTestHandler(&stubMusicStore{libraries: nil})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getMusicFolders", false)

	h.getMusicFolders(rec, r)

	if rec.Code != http.StatusOK {
		t.Errorf("getMusicFolders empty: got HTTP %d, want 200", rec.Code)
	}
	resp := parseXMLResponse(t, strings.TrimPrefix(rec.Body.String(), xml.Header))
	if resp.Status != "ok" {
		t.Errorf("getMusicFolders empty status: got %q, want %q", resp.Status, "ok")
	}
	if resp.MusicFolders == nil {
		t.Fatal("getMusicFolders empty: MusicFolders field is nil")
	}
	if len(resp.MusicFolders.MusicFolder) != 0 {
		t.Errorf("getMusicFolders empty: expected 0 folders, got %d", len(resp.MusicFolders.MusicFolder))
	}
}

// TestGetMusicFoldersWithLibraries verifies getMusicFolders returns the correct
// folder list when libraries exist.
func TestGetMusicFoldersWithLibraries(t *testing.T) {
	libs := []*model.Library{
		{ID: 1, Name: "Music"},
		{ID: 2, Name: "Podcasts"},
	}
	h := newTestHandler(&stubMusicStore{libraries: libs})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getMusicFolders", false)

	h.getMusicFolders(rec, r)

	resp := parseXMLResponse(t, strings.TrimPrefix(rec.Body.String(), xml.Header))
	if resp.Status != "ok" {
		t.Errorf("getMusicFolders: status %q, want %q", resp.Status, "ok")
	}
	if resp.MusicFolders == nil {
		t.Fatal("getMusicFolders: MusicFolders is nil")
	}
	if len(resp.MusicFolders.MusicFolder) != 2 {
		t.Fatalf("getMusicFolders: got %d folders, want 2", len(resp.MusicFolders.MusicFolder))
	}
	if resp.MusicFolders.MusicFolder[0].Name != "Music" {
		t.Errorf("getMusicFolders: folder[0].Name = %q, want %q",
			resp.MusicFolders.MusicFolder[0].Name, "Music")
	}
	if resp.MusicFolders.MusicFolder[1].ID != 2 {
		t.Errorf("getMusicFolders: folder[1].ID = %d, want 2",
			resp.MusicFolders.MusicFolder[1].ID)
	}
}

// TestGetMusicFoldersJSONFormat verifies getMusicFolders works in JSON format.
func TestGetMusicFoldersJSONFormat(t *testing.T) {
	libs := []*model.Library{{ID: 3, Name: "Radio"}}
	h := newTestHandler(&stubMusicStore{libraries: libs})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getMusicFolders?f=json", false)

	h.getMusicFolders(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.MusicFolders == nil {
		t.Fatal("getMusicFolders JSON: MusicFolders is nil")
	}
	if len(resp.MusicFolders.MusicFolder) != 1 {
		t.Fatalf("getMusicFolders JSON: got %d folders, want 1",
			len(resp.MusicFolders.MusicFolder))
	}
	if resp.MusicFolders.MusicFolder[0].Name != "Radio" {
		t.Errorf("getMusicFolders JSON: Name = %q, want %q",
			resp.MusicFolders.MusicFolder[0].Name, "Radio")
	}
}

// TestGetMusicFoldersUnauthenticated verifies that getMusicFolders returns
// error code 30 (not authenticated) when no user is in context.
func TestGetMusicFoldersUnauthenticated(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getMusicFolders?f=json", nil)

	h.getMusicFolders(rec, r)

	if rec.Code != http.StatusOK {
		t.Errorf("getMusicFolders unauthenticated: got HTTP %d, want 200", rec.Code)
	}
	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "failed" {
		t.Errorf("getMusicFolders unauthenticated status: got %q, want %q", resp.Status, "failed")
	}
	if resp.Error == nil {
		t.Fatal("getMusicFolders unauthenticated: Error is nil")
	}
	if resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getMusicFolders unauthenticated code: got %d, want %d",
			resp.Error.Code, ErrNotAuthenticated)
	}
}

// TestGetMusicFoldersStoreError verifies getMusicFolders returns error code 0
// (generic) when the store returns an error.
func TestGetMusicFoldersStoreError(t *testing.T) {
	h := newTestHandler(&stubMusicStore{libraryErr: errors.New("db down")})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getMusicFolders?f=json", false)

	h.getMusicFolders(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "failed" {
		t.Errorf("getMusicFolders store error status: got %q, want %q", resp.Status, "failed")
	}
	if resp.Error == nil {
		t.Fatal("getMusicFolders store error: Error is nil")
	}
	if resp.Error.Code != ErrGeneric {
		t.Errorf("getMusicFolders store error code: got %d, want %d", resp.Error.Code, ErrGeneric)
	}
}

// ---- unauthenticated request error code tests ------------------------------

// TestGetUserUnauthenticated verifies that getUser returns ErrNotAuthenticated
// (code 30) when no user is in the context.
func TestGetUserUnauthenticated(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getUser?f=json&username=alice", nil)

	h.getUser(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "failed" {
		t.Errorf("getUser unauthenticated status: got %q, want %q", resp.Status, "failed")
	}
	if resp.Error == nil {
		t.Fatal("getUser unauthenticated: Error is nil")
	}
	if resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getUser unauthenticated code: got %d, want %d",
			resp.Error.Code, ErrNotAuthenticated)
	}
}

// TestGetUserMissingParam verifies that getUser returns ErrMissingParam (code 10)
// when the required ?username= parameter is absent.
func TestGetUserMissingParam(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getUser?f=json", false)

	h.getUser(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil {
		t.Fatal("getUser missing param: Error is nil")
	}
	if resp.Error.Code != ErrMissingParam {
		t.Errorf("getUser missing param code: got %d, want %d",
			resp.Error.Code, ErrMissingParam)
	}
}

// TestGetUserForbiddenNonAdmin verifies that a non-admin user cannot fetch
// another user's record and receives ErrForbidden (code 50).
func TestGetUserForbiddenNonAdmin(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getUser?f=json&username=otherusr", false)

	h.getUser(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil {
		t.Fatal("getUser forbidden: Error is nil")
	}
	if resp.Error.Code != ErrForbidden {
		t.Errorf("getUser forbidden code: got %d, want %d", resp.Error.Code, ErrForbidden)
	}
}

// TestGetUsersUnauthenticated verifies that getUsers returns ErrNotAuthenticated
// when no user is in the context.
func TestGetUsersUnauthenticated(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getUsers?f=json", nil)

	h.getUsers(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil {
		t.Fatal("getUsers unauthenticated: Error is nil")
	}
	if resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getUsers unauthenticated code: got %d, want %d",
			resp.Error.Code, ErrNotAuthenticated)
	}
}

// TestGetUsersNonAdminForbidden verifies that a non-admin user receives
// ErrForbidden (code 50) when calling getUsers.
func TestGetUsersNonAdminForbidden(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getUsers?f=json", false)

	h.getUsers(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil {
		t.Fatal("getUsers non-admin: Error is nil")
	}
	if resp.Error.Code != ErrForbidden {
		t.Errorf("getUsers non-admin code: got %d, want %d", resp.Error.Code, ErrForbidden)
	}
}

// ---- parseBoolParam tests --------------------------------------------------

// TestParseBoolParam covers truthy and falsy variations of the Subsonic bool parameter.
func TestParseBoolParam(t *testing.T) {
	truthy := []string{"true", "TRUE", "True", "yes", "YES", "1", "on", "ON", "enable", "enabled", "ENABLED"}
	falsy := []string{"false", "FALSE", "no", "NO", "0", "off", "OFF", "disable", "disabled", "", "  "}

	for _, v := range truthy {
		if !parseBoolParam(v) {
			t.Errorf("parseBoolParam(%q) = false, want true", v)
		}
	}
	for _, v := range falsy {
		if parseBoolParam(v) {
			t.Errorf("parseBoolParam(%q) = true, want false", v)
		}
	}
}

// ---- getScanStatus tests ---------------------------------------------------

// TestGetScanStatusNoScanYet verifies getScanStatus returns scanning=false and count=0
// when no scan has ever run (nil status from store).
func TestGetScanStatusNoScanYet(t *testing.T) {
	h := newTestHandler(&stubMusicStore{scanStatus: nil})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getScanStatus?f=json", false)

	h.getScanStatus(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("getScanStatus no scan: status %q, want ok", resp.Status)
	}
	if resp.ScanStatus == nil {
		t.Fatal("getScanStatus no scan: ScanStatus is nil")
	}
	if resp.ScanStatus.Scanning {
		t.Error("getScanStatus no scan: Scanning should be false")
	}
	if resp.ScanStatus.Count != 0 {
		t.Errorf("getScanStatus no scan: Count = %d, want 0", resp.ScanStatus.Count)
	}
}

// TestGetScanStatusRunning verifies getScanStatus reports scanning=true when a
// scan is in progress.
func TestGetScanStatusRunning(t *testing.T) {
	h := newTestHandler(&stubMusicStore{
		scanStatus: &model.ScanStatus{Status: "running", ScannedFiles: 42},
	})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getScanStatus?f=json", false)

	h.getScanStatus(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if !resp.ScanStatus.Scanning {
		t.Error("getScanStatus running: Scanning should be true")
	}
	if resp.ScanStatus.Count != 42 {
		t.Errorf("getScanStatus running: Count = %d, want 42", resp.ScanStatus.Count)
	}
}

// TestGetScanStatusStoreError verifies getScanStatus returns ErrGeneric when the store fails.
func TestGetScanStatusStoreError(t *testing.T) {
	h := newTestHandler(&stubMusicStore{scanErr: errors.New("db gone")})
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getScanStatus?f=json", false)

	h.getScanStatus(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "failed" {
		t.Errorf("getScanStatus store error: status %q, want failed", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("getScanStatus store error: code = %v, want %d", resp.Error, ErrGeneric)
	}
}

// ---- idempotency tests -----------------------------------------------------

// TestPingIdempotent verifies that calling ping twice produces identical responses.
func TestPingIdempotent(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})

	call := func() string {
		rec := httptest.NewRecorder()
		r := authedRequest(http.MethodGet, "/rest/ping?f=json", false)
		h.ping(rec, r)
		return rec.Body.String()
	}

	first := call()
	second := call()
	if first != second {
		t.Errorf("ping idempotency: responses differ\nfirst:  %s\nsecond: %s", first, second)
	}
}

// TestGetLicenseIdempotent verifies that calling getLicense twice produces identical responses.
func TestGetLicenseIdempotent(t *testing.T) {
	h := newTestHandler(&stubMusicStore{})

	call := func() string {
		rec := httptest.NewRecorder()
		r := authedRequest(http.MethodGet, "/rest/getLicense?f=json", false)
		h.getLicense(rec, r)
		return rec.Body.String()
	}

	first := call()
	second := call()
	if first != second {
		t.Errorf("getLicense idempotency: responses differ\nfirst:  %s\nsecond: %s", first, second)
	}
}
