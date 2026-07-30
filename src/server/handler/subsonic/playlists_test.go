package subsonic

// Tests for playlists.go handler methods: getPlaylists, getPlaylist,
// createPlaylist, updatePlaylist, deletePlaylist, search/search2/search3,
// star/unstar, setRating, scrobble, shares (stubbed), bookmarks, play queue,
// and chat.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// ---- stub PlaylistStore ----------------------------------------------------

// stubPlaylistStore implements store.PlaylistStore. Configure the exported
// fields to control lookups; mutation methods record their arguments so
// tests can assert on them.
type stubPlaylistStore struct {
	playlists       []*model.Playlist
	playlistByID    map[int64]*model.Playlist
	entries         map[int64][]*model.PlaylistEntry
	listErr         error
	getErr          error
	createErr       error
	updateErr       error
	deleteErr       error
	entriesErr      error

	createdPlaylist *model.Playlist
	updatedPlaylist *model.Playlist
	deletedID       int64
	setEntries      []int64
	addedEntries    []int64
	removedIndices  []int
}

func (s *stubPlaylistStore) CreatePlaylist(_ context.Context, p *model.Playlist) (int64, error) {
	if s.createErr != nil {
		return 0, s.createErr
	}
	s.createdPlaylist = p
	return 42, nil
}
func (s *stubPlaylistStore) GetPlaylist(_ context.Context, id int64) (*model.Playlist, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.playlistByID != nil {
		return s.playlistByID[id], nil
	}
	return nil, nil
}
func (s *stubPlaylistStore) ListPlaylists(_ context.Context, _ int64) ([]*model.Playlist, error) {
	return s.playlists, s.listErr
}
func (s *stubPlaylistStore) UpdatePlaylist(_ context.Context, p *model.Playlist) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updatedPlaylist = p
	return nil
}
func (s *stubPlaylistStore) DeletePlaylist(_ context.Context, id int64) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deletedID = id
	return nil
}
func (s *stubPlaylistStore) GetPlaylistEntries(_ context.Context, playlistID int64) ([]*model.PlaylistEntry, error) {
	if s.entriesErr != nil {
		return nil, s.entriesErr
	}
	return s.entries[playlistID], nil
}
func (s *stubPlaylistStore) SetPlaylistEntries(_ context.Context, _ int64, songIDs []int64) error {
	s.setEntries = songIDs
	return nil
}
func (s *stubPlaylistStore) AddToPlaylist(_ context.Context, _ int64, songIDs []int64) error {
	s.addedEntries = songIDs
	return nil
}
func (s *stubPlaylistStore) RemoveFromPlaylist(_ context.Context, _ int64, indices []int) error {
	s.removedIndices = indices
	return nil
}

// ---- stub ChatStore ---------------------------------------------------------

type stubChatStore struct {
	messages   []*model.ChatMessage
	getErr     error
	addErr     error
	addedMsg   *model.ChatMessage
}

func (s *stubChatStore) AddMessage(_ context.Context, msg *model.ChatMessage) error {
	if s.addErr != nil {
		return s.addErr
	}
	s.addedMsg = msg
	return nil
}
func (s *stubChatStore) GetMessages(_ context.Context, _ time.Time) ([]*model.ChatMessage, error) {
	return s.messages, s.getErr
}
func (s *stubChatStore) PurgeOldMessages(_ context.Context, _ time.Time) error {
	return errors.New("not implemented")
}

// ---- helper -----------------------------------------------------------------

// newPlaylistsTestHandler builds a Handler with all stub stores wired,
// including PlaylistStore and ChatStore which the default test helpers omit.
func newPlaylistsTestHandler(music *stubMusicStore, users *stubUserStore, activity *stubActivityStore, playlists *stubPlaylistStore, chat *stubChatStore) *Handler {
	if music == nil {
		music = &stubMusicStore{}
	}
	if users == nil {
		users = &stubUserStore{}
	}
	if activity == nil {
		activity = &stubActivityStore{}
	}
	if playlists == nil {
		playlists = &stubPlaylistStore{}
	}
	if chat == nil {
		chat = &stubChatStore{}
	}
	db := &store.DB{
		Users:     users,
		Music:     music,
		Activity:  activity,
		Playlists: playlists,
		Chat:      chat,
	}
	return &Handler{
		db:         db,
		nowPlaying: NewNowPlayingTracker(),
		subsPass:   func(_ context.Context, _ string) (string, bool) { return "", false },
	}
}

// ---- getPlaylists -----------------------------------------------------------

func TestGetPlaylistsUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getPlaylists?f=json", nil)

	h.getPlaylists(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getPlaylists unauthenticated: got %+v", resp.Error)
	}
}

func TestGetPlaylistsOwn(t *testing.T) {
	pls := &stubPlaylistStore{
		playlists: []*model.Playlist{
			{ID: 1, UserID: 1, Name: "Faves", SongCount: 3},
		},
	}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getPlaylists?f=json", false)

	h.getPlaylists(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Fatalf("getPlaylists: status %q, want ok", resp.Status)
	}
	if resp.Playlists == nil || len(resp.Playlists.Playlist) != 1 {
		t.Fatalf("getPlaylists: got %+v, want 1 playlist", resp.Playlists)
	}
	if resp.Playlists.Playlist[0].Owner != "testuser" {
		t.Errorf("getPlaylists: owner = %q, want testuser", resp.Playlists.Playlist[0].Owner)
	}
}

func TestGetPlaylistsOtherUserForbiddenNonAdmin(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getPlaylists?f=json&username=someoneelse", false)

	h.getPlaylists(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrForbidden {
		t.Errorf("getPlaylists cross-user non-admin: got %+v, want code %d", resp.Error, ErrForbidden)
	}
}

func TestGetPlaylistsOtherUserAdminUserNotFound(t *testing.T) {
	h := newPlaylistsTestHandler(nil, &stubUserStore{}, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getPlaylists?f=json&username=someoneelse", true)

	h.getPlaylists(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("getPlaylists cross-user unknown user: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestGetPlaylistsOtherUserAdminSuccess(t *testing.T) {
	users := &stubUserStore{byUsername: &model.User{ID: 2, Username: "someoneelse"}}
	pls := &stubPlaylistStore{playlists: []*model.Playlist{{ID: 5, UserID: 2, Name: "Shared"}}}
	h := newPlaylistsTestHandler(nil, users, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getPlaylists?f=json&username=someoneelse", true)

	h.getPlaylists(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Fatalf("getPlaylists admin cross-user: status %q, want ok", resp.Status)
	}
	if len(resp.Playlists.Playlist) != 1 || resp.Playlists.Playlist[0].Owner != "someoneelse" {
		t.Errorf("getPlaylists admin cross-user: got %+v", resp.Playlists.Playlist)
	}
}

func TestGetPlaylistsStoreError(t *testing.T) {
	pls := &stubPlaylistStore{listErr: errors.New("db down")}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getPlaylists?f=json", false)

	h.getPlaylists(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("getPlaylists store error: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

// ---- getPlaylist -------------------------------------------------------------

func TestGetPlaylistUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getPlaylist?f=json&id=1", nil)

	h.getPlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getPlaylist unauthenticated: got %+v", resp.Error)
	}
}

func TestGetPlaylistMissingID(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getPlaylist?f=json", false)

	h.getPlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("getPlaylist missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestGetPlaylistInvalidID(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getPlaylist?f=json&id=notanumber", false)

	h.getPlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("getPlaylist invalid id: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestGetPlaylistNotFound(t *testing.T) {
	pls := &stubPlaylistStore{playlistByID: map[int64]*model.Playlist{}}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getPlaylist?f=json&id=99", false)

	h.getPlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("getPlaylist not found: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestGetPlaylistPrivateForbidden(t *testing.T) {
	pls := &stubPlaylistStore{playlistByID: map[int64]*model.Playlist{
		1: {ID: 1, UserID: 2, IsPublic: false},
	}}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getPlaylist?f=json&id=1", false)

	h.getPlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrForbidden {
		t.Errorf("getPlaylist private other user: got %+v, want code %d", resp.Error, ErrForbidden)
	}
}

func TestGetPlaylistSuccessWithEntries(t *testing.T) {
	music := &stubMusicStore{}
	pls := &stubPlaylistStore{
		playlistByID: map[int64]*model.Playlist{
			1: {ID: 1, UserID: 1, Name: "My List", IsPublic: true},
		},
		entries: map[int64][]*model.PlaylistEntry{
			1: {{SongID: 10, Position: 0}},
		},
	}
	h := newPlaylistsTestHandler(music, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getPlaylist?f=json&id=1", false)

	h.getPlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Fatalf("getPlaylist: status %q, want ok, err %+v", resp.Status, resp.Error)
	}
	if resp.Playlist == nil {
		t.Fatal("getPlaylist: Playlist is nil")
	}
	// GetSong always fails on the empty stub, so the song entry is skipped.
	if len(resp.Playlist.Entry) != 0 {
		t.Errorf("getPlaylist: got %d entries, want 0 (song lookup fails on stub)", len(resp.Playlist.Entry))
	}
}

// ---- createPlaylist ----------------------------------------------------------

func TestCreatePlaylistUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/createPlaylist?f=json&name=Test", nil)

	h.createPlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("createPlaylist unauthenticated: got %+v", resp.Error)
	}
}

func TestCreatePlaylistDefaultName(t *testing.T) {
	pls := &stubPlaylistStore{}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/createPlaylist?f=json", false)

	h.createPlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Fatalf("createPlaylist: status %q, want ok", resp.Status)
	}
	if pls.createdPlaylist == nil || pls.createdPlaylist.Name != "New Playlist" {
		t.Errorf("createPlaylist: got %+v, want default name", pls.createdPlaylist)
	}
}

func TestCreatePlaylistWithSongs(t *testing.T) {
	pls := &stubPlaylistStore{}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/createPlaylist?f=json&name=Mix&songId=so-1&songId=so-2", false)

	h.createPlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Fatalf("createPlaylist: status %q, want ok", resp.Status)
	}
	if len(pls.setEntries) != 2 {
		t.Errorf("createPlaylist: got %d entries set, want 2", len(pls.setEntries))
	}
}

func TestCreatePlaylistStoreError(t *testing.T) {
	pls := &stubPlaylistStore{createErr: errors.New("db down")}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/createPlaylist?f=json&name=Mix", false)

	h.createPlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("createPlaylist store error: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

// ---- updatePlaylist -----------------------------------------------------------

func TestUpdatePlaylistUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/updatePlaylist?f=json&playlistId=1", nil)

	h.updatePlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("updatePlaylist unauthenticated: got %+v", resp.Error)
	}
}

func TestUpdatePlaylistMissingID(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/updatePlaylist?f=json", false)

	h.updatePlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("updatePlaylist missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestUpdatePlaylistNotFound(t *testing.T) {
	pls := &stubPlaylistStore{playlistByID: map[int64]*model.Playlist{}}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/updatePlaylist?f=json&playlistId=1", false)

	h.updatePlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("updatePlaylist not found: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestUpdatePlaylistForbiddenNonOwner(t *testing.T) {
	pls := &stubPlaylistStore{playlistByID: map[int64]*model.Playlist{
		1: {ID: 1, UserID: 2},
	}}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/updatePlaylist?f=json&playlistId=1", false)

	h.updatePlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrForbidden {
		t.Errorf("updatePlaylist non-owner: got %+v, want code %d", resp.Error, ErrForbidden)
	}
}

func TestUpdatePlaylistSuccess(t *testing.T) {
	pls := &stubPlaylistStore{playlistByID: map[int64]*model.Playlist{
		1: {ID: 1, UserID: 1, Name: "Old"},
	}}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	target := "/rest/updatePlaylist?f=json&playlistId=1&name=New&comment=Hi&public=true" +
		"&songIdToAdd=so-1&songIndexToRemove=0"
	r := authedRequest(http.MethodGet, target, false)

	h.updatePlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Fatalf("updatePlaylist: status %q, want ok, err %+v", resp.Status, resp.Error)
	}
	if pls.updatedPlaylist == nil || pls.updatedPlaylist.Name != "New" || pls.updatedPlaylist.Comment != "Hi" || !pls.updatedPlaylist.IsPublic {
		t.Errorf("updatePlaylist: got %+v", pls.updatedPlaylist)
	}
	if len(pls.addedEntries) != 1 {
		t.Errorf("updatePlaylist: got %d added entries, want 1", len(pls.addedEntries))
	}
	if len(pls.removedIndices) != 1 || pls.removedIndices[0] != 0 {
		t.Errorf("updatePlaylist: got %v removed indices, want [0]", pls.removedIndices)
	}
}

func TestUpdatePlaylistStoreError(t *testing.T) {
	pls := &stubPlaylistStore{
		playlistByID: map[int64]*model.Playlist{1: {ID: 1, UserID: 1}},
		updateErr:    errors.New("db down"),
	}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/updatePlaylist?f=json&playlistId=1&name=New", false)

	h.updatePlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("updatePlaylist store error: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

// ---- deletePlaylist -----------------------------------------------------------

func TestDeletePlaylistUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/deletePlaylist?f=json&id=1", nil)

	h.deletePlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("deletePlaylist unauthenticated: got %+v", resp.Error)
	}
}

func TestDeletePlaylistMissingID(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deletePlaylist?f=json", false)

	h.deletePlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("deletePlaylist missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestDeletePlaylistInvalidID(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deletePlaylist?f=json&id=abc", false)

	h.deletePlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("deletePlaylist invalid id: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestDeletePlaylistNotFound(t *testing.T) {
	pls := &stubPlaylistStore{playlistByID: map[int64]*model.Playlist{}}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deletePlaylist?f=json&id=1", false)

	h.deletePlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("deletePlaylist not found: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestDeletePlaylistForbiddenNonOwner(t *testing.T) {
	pls := &stubPlaylistStore{playlistByID: map[int64]*model.Playlist{
		1: {ID: 1, UserID: 2},
	}}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deletePlaylist?f=json&id=1", false)

	h.deletePlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrForbidden {
		t.Errorf("deletePlaylist non-owner: got %+v, want code %d", resp.Error, ErrForbidden)
	}
}

func TestDeletePlaylistAdminOverride(t *testing.T) {
	pls := &stubPlaylistStore{playlistByID: map[int64]*model.Playlist{
		1: {ID: 1, UserID: 2},
	}}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deletePlaylist?f=json&id=1", true)

	h.deletePlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Fatalf("deletePlaylist admin: status %q, want ok, err %+v", resp.Status, resp.Error)
	}
	if pls.deletedID != 1 {
		t.Errorf("deletePlaylist admin: deletedID = %d, want 1", pls.deletedID)
	}
}

func TestDeletePlaylistStoreError(t *testing.T) {
	pls := &stubPlaylistStore{
		playlistByID: map[int64]*model.Playlist{1: {ID: 1, UserID: 1}},
		deleteErr:    errors.New("db down"),
	}
	h := newPlaylistsTestHandler(nil, nil, nil, pls, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deletePlaylist?f=json&id=1", false)

	h.deletePlaylist(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("deletePlaylist store error: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

// ---- search / search2 / search3 ------------------------------------------

func TestSearchUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/search?f=json&any=x", nil)

	h.search(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("search unauthenticated: got %+v", resp.Error)
	}
}

func TestSearchFallsBackToQueryParam(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/search?f=json&query=hello", false)

	h.search(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Fatalf("search: status %q, want ok", resp.Status)
	}
	if resp.SearchResult == nil {
		t.Fatal("search: SearchResult is nil")
	}
}

func TestSearch2MissingQuery(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/search2?f=json", false)

	h.search2(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("search2 missing query: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestSearch2Success(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/search2?f=json&query=hello", false)

	h.search2(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" || resp.SearchResult2 == nil {
		t.Fatalf("search2: status %q, SearchResult2 %+v", resp.Status, resp.SearchResult2)
	}
}

func TestSearch3MissingQuery(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/search3?f=json", false)

	h.search3(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("search3 missing query: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestSearch3Success(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/search3?f=json&query=hello", false)

	h.search3(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" || resp.SearchResult3 == nil {
		t.Fatalf("search3: status %q, SearchResult3 %+v", resp.Status, resp.SearchResult3)
	}
}

// ---- star / unstar -----------------------------------------------------------

func TestStarUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/star?f=json&id=so-1", nil)

	h.star(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("star unauthenticated: got %+v", resp.Error)
	}
}

func TestStarAllIDKinds(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	target := "/rest/star?f=json&id=so-1&albumId=al-2&artistId=ar-3"
	r := authedRequest(http.MethodGet, target, false)

	h.star(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("star: status %q, want ok", resp.Status)
	}
}

func TestUnstarUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/unstar?f=json&id=so-1", nil)

	h.unstar(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("unstar unauthenticated: got %+v", resp.Error)
	}
}

func TestUnstarAllIDKinds(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	target := "/rest/unstar?f=json&id=so-1&albumId=al-2&artistId=ar-3"
	r := authedRequest(http.MethodGet, target, false)

	h.unstar(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("unstar: status %q, want ok", resp.Status)
	}
}

// ---- setRating ----------------------------------------------------------------

func TestSetRatingUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/setRating?f=json&id=so-1&rating=3", nil)

	h.setRating(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("setRating unauthenticated: got %+v", resp.Error)
	}
}

func TestSetRatingMissingID(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/setRating?f=json&rating=3", false)

	h.setRating(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("setRating missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestSetRatingMissingRating(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/setRating?f=json&id=so-1", false)

	h.setRating(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("setRating missing rating: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestSetRatingOutOfRange(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/setRating?f=json&id=so-1&rating=9", false)

	h.setRating(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("setRating out of range: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

func TestSetRatingInvalidID(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/setRating?f=json&id=!!!&rating=3", false)

	h.setRating(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("setRating invalid id: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestSetRatingZeroClears(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/setRating?f=json&id=so-1&rating=0", false)

	h.setRating(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("setRating rating=0: status %q, want ok", resp.Status)
	}
}

func TestSetRatingAlbum(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/setRating?f=json&id=al-1&rating=5", false)

	h.setRating(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("setRating album: status %q, want ok", resp.Status)
	}
}

// ---- scrobble ------------------------------------------------------------------

func TestScrobbleUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/scrobble?f=json&id=so-1", nil)

	h.scrobble(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("scrobble unauthenticated: got %+v", resp.Error)
	}
}

func TestScrobbleMissingID(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/scrobble?f=json", false)

	h.scrobble(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("scrobble missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestScrobbleInvalidID(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/scrobble?f=json&id=!!!", false)

	h.scrobble(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("scrobble invalid id: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestScrobbleNonSubmission(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/scrobble?f=json&id=so-1&submission=false", false)

	h.scrobble(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("scrobble non-submission: status %q, want ok", resp.Status)
	}
}

func TestScrobbleWithExplicitTime(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/scrobble?f=json&id=so-1&time=1000000", false)

	h.scrobble(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("scrobble with time: status %q, want ok", resp.Status)
	}
}

// ---- shares (stubbed, always ok) -----------------------------------------------

func TestGetSharesUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getShares?f=json", nil)

	h.getShares(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getShares unauthenticated: got %+v", resp.Error)
	}
}

func TestGetSharesSuccess(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getShares?f=json", false)

	h.getShares(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" || resp.Shares == nil {
		t.Errorf("getShares: status %q, Shares %+v", resp.Status, resp.Shares)
	}
}

func TestCreateShareUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/createShare?f=json", nil)

	h.createShare(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("createShare unauthenticated: got %+v", resp.Error)
	}
}

func TestCreateShareSuccess(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/createShare?f=json", false)

	h.createShare(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" || resp.Shares == nil {
		t.Errorf("createShare: status %q, Shares %+v", resp.Status, resp.Shares)
	}
}

func TestUpdateShareUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/updateShare?f=json", nil)

	h.updateShare(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("updateShare unauthenticated: got %+v", resp.Error)
	}
}

func TestUpdateShareSuccess(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/updateShare?f=json", false)

	h.updateShare(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("updateShare: status %q, want ok", resp.Status)
	}
}

func TestDeleteShareUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/deleteShare?f=json", nil)

	h.deleteShare(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("deleteShare unauthenticated: got %+v", resp.Error)
	}
}

func TestDeleteShareSuccess(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deleteShare?f=json", false)

	h.deleteShare(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("deleteShare: status %q, want ok", resp.Status)
	}
}

// ---- bookmarks --------------------------------------------------------------

func TestGetBookmarksUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getBookmarks?f=json", nil)

	h.getBookmarks(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getBookmarks unauthenticated: got %+v", resp.Error)
	}
}

func TestGetBookmarksStoreError(t *testing.T) {
	activity := &stubActivityStore{}
	h := newPlaylistsTestHandler(nil, nil, activity, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getBookmarks?f=json", false)

	h.getBookmarks(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("getBookmarks: status %q, want ok", resp.Status)
	}
	if resp.Bookmarks == nil {
		t.Fatal("getBookmarks: Bookmarks is nil")
	}
}

func TestCreateBookmarkUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/createBookmark?f=json&id=so-1&position=1000", nil)

	h.createBookmark(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("createBookmark unauthenticated: got %+v", resp.Error)
	}
}

func TestCreateBookmarkMissingID(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/createBookmark?f=json&position=1000", false)

	h.createBookmark(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("createBookmark missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestCreateBookmarkMissingPosition(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/createBookmark?f=json&id=so-1", false)

	h.createBookmark(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("createBookmark missing position: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestCreateBookmarkInvalidPosition(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/createBookmark?f=json&id=so-1&position=notanumber", false)

	h.createBookmark(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("createBookmark invalid position: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

func TestCreateBookmarkInvalidID(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/createBookmark?f=json&id=!!!&position=1000", false)

	h.createBookmark(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("createBookmark invalid id: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestCreateBookmarkSuccess(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/createBookmark?f=json&id=so-1&position=1000&comment=hi", false)

	h.createBookmark(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("createBookmark: status %q, want ok, err %+v", resp.Status, resp.Error)
	}
}

func TestDeleteBookmarkUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/deleteBookmark?f=json&id=so-1", nil)

	h.deleteBookmark(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("deleteBookmark unauthenticated: got %+v", resp.Error)
	}
}

func TestDeleteBookmarkMissingID(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deleteBookmark?f=json", false)

	h.deleteBookmark(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("deleteBookmark missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestDeleteBookmarkInvalidID(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deleteBookmark?f=json&id=!!!", false)

	h.deleteBookmark(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("deleteBookmark invalid id: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestDeleteBookmarkSuccess(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/deleteBookmark?f=json&id=so-1", false)

	h.deleteBookmark(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("deleteBookmark: status %q, want ok", resp.Status)
	}
}

// ---- play queue ---------------------------------------------------------------

func TestGetPlayQueueUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getPlayQueue?f=json", nil)

	h.getPlayQueue(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getPlayQueue unauthenticated: got %+v", resp.Error)
	}
}

func TestGetPlayQueueEmpty(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getPlayQueue?f=json", false)

	h.getPlayQueue(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Fatalf("getPlayQueue: status %q, want ok", resp.Status)
	}
	if resp.PlayQueue == nil || resp.PlayQueue.Username != "testuser" {
		t.Errorf("getPlayQueue empty: got %+v", resp.PlayQueue)
	}
}

func TestSavePlayQueueUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/savePlayQueue?f=json", nil)

	h.savePlayQueue(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("savePlayQueue unauthenticated: got %+v", resp.Error)
	}
}

func TestSavePlayQueueSuccess(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	target := "/rest/savePlayQueue?f=json&id=so-1&id=so-2&current=so-1&position=5000"
	r := authedRequest(http.MethodGet, target, false)

	h.savePlayQueue(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Errorf("savePlayQueue: status %q, want ok, err %+v", resp.Status, resp.Error)
	}
}

// ---- chat -----------------------------------------------------------------------

func TestGetChatMessagesUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getChatMessages?f=json", nil)

	h.getChatMessages(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getChatMessages unauthenticated: got %+v", resp.Error)
	}
}

func TestGetChatMessagesSuccess(t *testing.T) {
	chat := &stubChatStore{messages: []*model.ChatMessage{
		{Username: "u1", Message: "hi", CreatedAt: time.Now()},
	}}
	h := newPlaylistsTestHandler(nil, nil, nil, nil, chat)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getChatMessages?f=json&since=0", false)

	h.getChatMessages(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Fatalf("getChatMessages: status %q, want ok", resp.Status)
	}
	if resp.ChatMessages == nil || len(resp.ChatMessages.ChatMessage) != 1 {
		t.Errorf("getChatMessages: got %+v", resp.ChatMessages)
	}
}

func TestGetChatMessagesStoreError(t *testing.T) {
	chat := &stubChatStore{getErr: errors.New("db down")}
	h := newPlaylistsTestHandler(nil, nil, nil, nil, chat)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getChatMessages?f=json", false)

	h.getChatMessages(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("getChatMessages store error: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

func TestAddChatMessageUnauthenticated(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/addChatMessage?f=json&message=hi", nil)

	h.addChatMessage(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("addChatMessage unauthenticated: got %+v", resp.Error)
	}
}

func TestAddChatMessageMissingMessage(t *testing.T) {
	h := newPlaylistsTestHandler(nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/addChatMessage?f=json", false)

	h.addChatMessage(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("addChatMessage missing message: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestAddChatMessageStoreError(t *testing.T) {
	chat := &stubChatStore{addErr: errors.New("db down")}
	h := newPlaylistsTestHandler(nil, nil, nil, nil, chat)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/addChatMessage?f=json&message=hi", false)

	h.addChatMessage(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("addChatMessage store error: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

func TestAddChatMessageSuccess(t *testing.T) {
	chat := &stubChatStore{}
	h := newPlaylistsTestHandler(nil, nil, nil, nil, chat)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/addChatMessage?f=json&message=hello", false)

	h.addChatMessage(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Fatalf("addChatMessage: status %q, want ok", resp.Status)
	}
	if chat.addedMsg == nil || chat.addedMsg.Message != "hello" {
		t.Errorf("addChatMessage: got %+v", chat.addedMsg)
	}
}

// ---- modelPlaylistToEntry -------------------------------------------------------

func TestModelPlaylistToEntry(t *testing.T) {
	p := &model.Playlist{ID: 7, Name: "Test", Comment: "c", IsPublic: true, SongCount: 2, Duration: 300}
	entry := modelPlaylistToEntry(p, "owner1")
	if entry.ID != "7" || entry.Name != "Test" || entry.Owner != "owner1" || !entry.Public || entry.SongCount != 2 {
		t.Errorf("modelPlaylistToEntry: got %+v", entry)
	}
}
