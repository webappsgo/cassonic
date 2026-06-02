package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// stubMusicStore satisfies store.MusicStore with configurable error returns.
// Only ListLibraries is called by Health; all others return an error.
type stubMusicStore struct {
	listLibrariesErr error
}

func (s *stubMusicStore) ListLibraries(ctx context.Context) ([]*model.Library, error) {
	return nil, s.listLibrariesErr
}

func (s *stubMusicStore) CreateLibrary(ctx context.Context, lib *model.Library) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *stubMusicStore) GetLibrary(ctx context.Context, id int64) (*model.Library, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) UpdateLibrary(ctx context.Context, lib *model.Library) error {
	return errors.New("not implemented")
}

func (s *stubMusicStore) DeleteLibrary(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (s *stubMusicStore) UpsertArtist(ctx context.Context, a *model.Artist) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *stubMusicStore) GetArtist(ctx context.Context, id int64) (*model.Artist, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) GetArtistByName(ctx context.Context, name string) (*model.Artist, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) ListArtists(ctx context.Context, opts store.ListOpts) ([]*model.Artist, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) SearchArtists(ctx context.Context, query string, opts store.ListOpts) ([]*model.Artist, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) DeleteArtistsWithNoSongs(ctx context.Context) error {
	return errors.New("not implemented")
}

func (s *stubMusicStore) UpsertAlbum(ctx context.Context, a *model.Album) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *stubMusicStore) GetAlbum(ctx context.Context, id int64) (*model.Album, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) ListAlbums(ctx context.Context, opts store.ListOpts) ([]*model.Album, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) ListAlbumsByArtist(ctx context.Context, artistID int64) ([]*model.Album, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) GetNewestAlbums(ctx context.Context, limit int) ([]*model.Album, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) GetRandomAlbums(ctx context.Context, limit int) ([]*model.Album, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) SearchAlbums(ctx context.Context, query string, opts store.ListOpts) ([]*model.Album, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) DeleteAlbumsWithNoSongs(ctx context.Context) error {
	return errors.New("not implemented")
}

func (s *stubMusicStore) UpsertSong(ctx context.Context, song *model.Song) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *stubMusicStore) GetSong(ctx context.Context, id int64) (*model.Song, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) GetSongByPath(ctx context.Context, path string) (*model.Song, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) ListSongsByAlbum(ctx context.Context, albumID int64) ([]*model.Song, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) ListSongsByArtist(ctx context.Context, artistID int64) ([]*model.Song, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) ListSongsByGenre(ctx context.Context, genre string, opts store.ListOpts) ([]*model.Song, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) GetRandomSongs(ctx context.Context, limit int, genre, fromYear, toYear string) ([]*model.Song, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) SearchSongs(ctx context.Context, query string, opts store.ListOpts) ([]*model.Song, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) MarkSongMissing(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (s *stubMusicStore) DeleteMissingSongs(ctx context.Context) error {
	return errors.New("not implemented")
}

func (s *stubMusicStore) IncrementPlayCount(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (s *stubMusicStore) ListGenres(ctx context.Context) ([]*model.Genre, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) UpsertCoverArt(ctx context.Context, ca *model.CoverArt) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *stubMusicStore) GetCoverArt(ctx context.Context, id int64) (*model.CoverArt, error) {
	return nil, errors.New("not implemented")
}

func (s *stubMusicStore) CreateScanStatus(ctx context.Context, sc *model.ScanStatus) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *stubMusicStore) UpdateScanStatus(ctx context.Context, sc *model.ScanStatus) error {
	return errors.New("not implemented")
}

func (s *stubMusicStore) GetLastScanStatus(ctx context.Context) (*model.ScanStatus, error) {
	return nil, errors.New("not implemented")
}

// stubUserStoreForHealth satisfies store.UserStore with configurable error returns.
// Only ListUsers is called by Health; all others return an error.
type stubUserStoreForHealth struct {
	listUsersErr error
}

func (s *stubUserStoreForHealth) CreateUser(ctx context.Context, u *model.User) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *stubUserStoreForHealth) GetUser(ctx context.Context, id int64) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUserStoreForHealth) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUserStoreForHealth) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUserStoreForHealth) UpdateUser(ctx context.Context, u *model.User) error {
	return errors.New("not implemented")
}

func (s *stubUserStoreForHealth) DeleteUser(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (s *stubUserStoreForHealth) ListUsers(ctx context.Context) ([]*model.User, error) {
	return nil, s.listUsersErr
}

func (s *stubUserStoreForHealth) IncrementLoginAttempts(ctx context.Context, id int64) error {
	return nil
}

func (s *stubUserStoreForHealth) ResetLoginAttempts(ctx context.Context, id int64) error {
	return nil
}

func (s *stubUserStoreForHealth) SetLockedUntil(ctx context.Context, id int64, until time.Time) error {
	return nil
}

func (s *stubUserStoreForHealth) UpdateLastLogin(ctx context.Context, id int64) error {
	return nil
}

func (s *stubUserStoreForHealth) CreateAPIToken(ctx context.Context, t *model.APIToken) error {
	return errors.New("not implemented")
}

func (s *stubUserStoreForHealth) GetAPITokenByHash(ctx context.Context, hash string) (*model.APIToken, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUserStoreForHealth) ListAPITokens(ctx context.Context, userID int64) ([]*model.APIToken, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUserStoreForHealth) DeleteAPIToken(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (s *stubUserStoreForHealth) UpdateAPITokenLastUsed(ctx context.Context, id int64) error {
	return nil
}

func (s *stubUserStoreForHealth) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, clientName string) error {
	return errors.New("not implemented")
}

func (s *stubUserStoreForHealth) GetSessionByHash(ctx context.Context, tokenHash string) (*store.Session, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUserStoreForHealth) DeleteSession(ctx context.Context, tokenHash string) error {
	return errors.New("not implemented")
}

func (s *stubUserStoreForHealth) DeleteUserSessions(ctx context.Context, userID int64) error {
	return errors.New("not implemented")
}

func (s *stubUserStoreForHealth) PurgeExpiredSessions(ctx context.Context) error {
	return nil
}

func (s *stubUserStoreForHealth) GetSubsonicPassword(ctx context.Context, username string) (string, bool, error) {
	return "", false, nil
}

func (s *stubUserStoreForHealth) SetSubsonicPassword(ctx context.Context, username string, encrypted string) error {
	return errors.New("not implemented")
}

func (s *stubUserStoreForHealth) CreateRadioStation(ctx context.Context, st *model.InternetRadioStation) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *stubUserStoreForHealth) GetRadioStation(ctx context.Context, id int64) (*model.InternetRadioStation, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUserStoreForHealth) ListRadioStations(ctx context.Context) ([]*model.InternetRadioStation, error) {
	return nil, nil
}

func (s *stubUserStoreForHealth) UpdateRadioStation(ctx context.Context, st *model.InternetRadioStation) error {
	return errors.New("not implemented")
}

func (s *stubUserStoreForHealth) DeleteRadioStation(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

// newHealthyDB returns a *store.DB whose Music and Users stores succeed on health checks.
func newHealthyDB() *store.DB {
	return &store.DB{
		Music: &stubMusicStore{listLibrariesErr: nil},
		Users: &stubUserStoreForHealth{listUsersErr: nil},
	}
}

// newDegradedDB returns a *store.DB whose Music and Users stores both fail.
func newDegradedDB() *store.DB {
	return &store.DB{
		Music: &stubMusicStore{listLibrariesErr: errors.New("db down")},
		Users: &stubUserStoreForHealth{listUsersErr: errors.New("db down")},
	}
}

// newHealthHandler constructs a Handler backed by the given stub DB with
// no optional services (scanner, coverArt, ffmpeg, tagReader are all nil).
func newHealthHandler(db *store.DB) *Handler {
	return &Handler{db: db, nowPlaying: NewNowPlayingTracker()}
}

// TestHealthJSONDefault verifies that GET /api/v1/health with no Accept header
// returns application/json with ok:true and a status field.
func TestHealthJSONDefault(t *testing.T) {
	h := newHealthHandler(newHealthyDB())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if ok, _ := body["ok"].(bool); !ok {
		t.Errorf("JSON body: ok field is not true; got %v", body["ok"])
	}
	if _, hasStatus := body["status"]; !hasStatus {
		t.Error("JSON body: missing 'status' field")
	}
}

// TestHealthJSONExplicit verifies that Accept: application/json also returns JSON.
func TestHealthJSONExplicit(t *testing.T) {
	h := newHealthHandler(newHealthyDB())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if ok, _ := body["ok"].(bool); !ok {
		t.Errorf("JSON ok field: got %v, want true", body["ok"])
	}
}

// TestHealthPlainText verifies that Accept: text/plain returns plain text containing "ok".
func TestHealthPlainText(t *testing.T) {
	h := newHealthHandler(newHealthyDB())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type: got %q, want text/plain", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "status:") {
		t.Errorf("plain text body missing 'status:'; got: %q", body)
	}
}

// TestHealthHTML verifies that Accept: text/html returns an HTML page containing "Status".
func TestHealthHTML(t *testing.T) {
	h := newHealthHandler(newHealthyDB())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type: got %q, want text/html", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Status") {
		t.Errorf("HTML body missing 'Status'; got: %q", body)
	}
	if !strings.Contains(body, "<html") {
		t.Errorf("HTML body does not contain <html tag; got: %q", body)
	}
}

// TestHealthDegradedStatus verifies that when a DB call fails the status is "degraded".
func TestHealthDegradedStatus(t *testing.T) {
	h := newHealthHandler(newDegradedDB())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if status, _ := body["status"].(string); status != "degraded" {
		t.Errorf("status: got %q, want 'degraded'", status)
	}
}

// TestHealthDegradedPlainText verifies the plain-text response when the DB fails.
func TestHealthDegradedPlainText(t *testing.T) {
	h := newHealthHandler(newDegradedDB())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "degraded") {
		t.Errorf("plain text degraded: expected 'degraded' in body; got: %q", body)
	}
}

// TestVersionJSONDefault verifies that GET /api/v1/version with no Accept header
// returns JSON with a "version" field.
func TestVersionJSONDefault(t *testing.T) {
	Version = "1.2.3"
	h := newHealthHandler(newHealthyDB())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()

	h.Version(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}

	// Version uses writeJSON which wraps in {"ok":true,"data":{...}}
	var envelope map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("JSON body missing 'data' map; got: %v", envelope)
	}
	if ver, _ := data["version"].(string); ver == "" {
		t.Errorf("data.version is empty; full body: %v", envelope)
	}
}

// TestVersionJSONExplicit verifies Accept: application/json also returns JSON.
func TestVersionJSONExplicit(t *testing.T) {
	Version = "2.0.0"
	h := newHealthHandler(newHealthyDB())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	h.Version(rec, req)

	var envelope map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("JSON body missing 'data' map; got: %v", envelope)
	}
	if ver, _ := data["version"].(string); ver != "2.0.0" {
		t.Errorf("version: got %q, want 2.0.0", ver)
	}
}

// TestVersionPlainText verifies that Accept: text/plain returns plain text containing "version:".
func TestVersionPlainText(t *testing.T) {
	Version = "3.4.5"
	h := newHealthHandler(newHealthyDB())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()

	h.Version(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type: got %q, want text/plain", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "version:") {
		t.Errorf("plain text body missing 'version:'; got: %q", body)
	}
	if !strings.Contains(body, "3.4.5") {
		t.Errorf("plain text body missing version value '3.4.5'; got: %q", body)
	}
}

// TestVersionHTML verifies that Accept: text/html returns an HTML page containing "Version".
func TestVersionHTML(t *testing.T) {
	Version = "4.5.6"
	h := newHealthHandler(newHealthyDB())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	h.Version(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type: got %q, want text/html", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Version") {
		t.Errorf("HTML body missing 'Version'; got: %q", body)
	}
	if !strings.Contains(body, "4.5.6") {
		t.Errorf("HTML body missing version '4.5.6'; got: %q", body)
	}
}

// TestHealthWildcardAccept verifies that Accept: */* falls through to JSON.
func TestHealthWildcardAccept(t *testing.T) {
	h := newHealthHandler(newHealthyDB())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Accept", "*/*")
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type for */*: got %q, want application/json", ct)
	}
}

// TestHealthMusicDBErrorOnly verifies status is "degraded" when only the music DB fails.
func TestHealthMusicDBErrorOnly(t *testing.T) {
	db := &store.DB{
		Music: &stubMusicStore{listLibrariesErr: errors.New("music db down")},
		Users: &stubUserStoreForHealth{listUsersErr: nil},
	}
	h := newHealthHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if status, _ := body["status"].(string); status != "degraded" {
		t.Errorf("status with music DB failure: got %q, want 'degraded'", status)
	}
	dbInfo, _ := body["db"].(map[string]any)
	if dbInfo == nil {
		t.Fatal("JSON body missing 'db' field")
	}
	if sv, _ := dbInfo["server"].(string); sv != "error" {
		t.Errorf("db.server: got %q, want 'error'", sv)
	}
	if uv, _ := dbInfo["users"].(string); uv != "ok" {
		t.Errorf("db.users: got %q, want 'ok'", uv)
	}
}

// TestHealthUsersDBErrorOnly verifies status is "degraded" when only the users DB fails.
func TestHealthUsersDBErrorOnly(t *testing.T) {
	db := &store.DB{
		Music: &stubMusicStore{listLibrariesErr: nil},
		Users: &stubUserStoreForHealth{listUsersErr: errors.New("users db down")},
	}
	h := newHealthHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if status, _ := body["status"].(string); status != "degraded" {
		t.Errorf("status with users DB failure: got %q, want 'degraded'", status)
	}
	dbInfo, _ := body["db"].(map[string]any)
	if dbInfo == nil {
		t.Fatal("JSON body missing 'db' field")
	}
	if sv, _ := dbInfo["server"].(string); sv != "ok" {
		t.Errorf("db.server: got %q, want 'ok'", sv)
	}
	if uv, _ := dbInfo["users"].(string); uv != "error" {
		t.Errorf("db.users: got %q, want 'error'", uv)
	}
}
