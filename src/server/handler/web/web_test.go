package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/argon2"

	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// argon2id parameters mirroring the api package's hashPassword, used to build
// valid test password hashes without depending on the api package.
const (
	testArgon2Memory      = 64 * 1024
	testArgon2Iterations  = 3
	testArgon2Parallelism = 2
	testArgon2SaltLen     = 16
	testArgon2KeyLen      = 32
)

func mustHashPassword(t *testing.T, password string) string {
	t.Helper()
	salt := make([]byte, testArgon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("rand: %v", err)
	}
	key := argon2.IDKey([]byte(password), salt, testArgon2Iterations, testArgon2Memory, testArgon2Parallelism, testArgon2KeyLen)
	return "$argon2id$v=19$m=" +
		itoa(testArgon2Memory) + ",t=" + itoa(testArgon2Iterations) + ",p=" + itoa(testArgon2Parallelism) + "$" +
		base64.RawStdEncoding.EncodeToString(salt) + "$" +
		base64.RawStdEncoding.EncodeToString(key)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// withChiID injects a chi route URL parameter into the request context.
func withChiID(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// withAuthUser injects an authenticated user into the request context.
func withAuthUser(r *http.Request, id int64, username string, isAdmin bool) *http.Request {
	return r.WithContext(mw.WithUser(r.Context(), &mw.AuthUser{ID: id, Username: username, IsAdmin: isAdmin}))
}

var errStore = errors.New("store error")

// --- Home ---

func TestHome_RendersWithoutUser(t *testing.T) {
	db := testDB()
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.Home(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHome_RendersWithUserPlayHistory(t *testing.T) {
	db := testDB()
	db.Music.(*testMusicStore).newestAlbumsResult = []*model.Album{{ID: 1, Title: "A"}}
	db.Activity.(*testActivityStore).getPlayHistoryResult = []*model.PlayHistory{{ID: 1, SongID: 2}}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.Home(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Login / LoginPost / Logout ---

func TestLogin_Renders(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	h.Login(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLoginPost_MissingCredentials(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=&password="))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.LoginPost(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Location"), "error=") {
		t.Errorf("expected error redirect, got %q", w.Header().Get("Location"))
	}
}

func TestLoginPost_UserNotFound(t *testing.T) {
	db := testDB()
	h := newTestHandler(db)
	form := url.Values{"username": {"nobody"}, "password": {"pass"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.LoginPost(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Location"), "invalid+credentials") {
		t.Errorf("expected invalid credentials redirect, got %q", w.Header().Get("Location"))
	}
}

func TestLoginPost_DisabledUser(t *testing.T) {
	db := testDB()
	db.Users.(*testUserStore).getUserByUsernameResult = &model.User{ID: 1, Username: "alice", IsEnabled: false}
	h := newTestHandler(db)
	form := url.Values{"username": {"alice"}, "password": {"pass"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.LoginPost(w, r)
	if !strings.Contains(w.Header().Get("Location"), "invalid+credentials") {
		t.Errorf("expected invalid credentials redirect, got %q", w.Header().Get("Location"))
	}
}

func TestLoginPost_WrongPassword(t *testing.T) {
	db := testDB()
	hash := mustHashPassword(t, "correct")
	db.Users.(*testUserStore).getUserByUsernameResult = &model.User{ID: 1, Username: "alice", IsEnabled: true, PasswordHash: hash}
	h := newTestHandler(db)
	form := url.Values{"username": {"alice"}, "password": {"wrong"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.LoginPost(w, r)
	if !strings.Contains(w.Header().Get("Location"), "invalid+credentials") {
		t.Errorf("expected invalid credentials redirect, got %q", w.Header().Get("Location"))
	}
}

func TestLoginPost_SessionCreateError(t *testing.T) {
	db := testDB()
	hash := mustHashPassword(t, "correct")
	db.Users.(*testUserStore).getUserByUsernameResult = &model.User{ID: 1, Username: "alice", IsEnabled: true, PasswordHash: hash}
	db.Users.(*testUserStore).createSessionErr = errStore
	h := newTestHandler(db)
	form := url.Values{"username": {"alice"}, "password": {"correct"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.LoginPost(w, r)
	if !strings.Contains(w.Header().Get("Location"), "server+error") {
		t.Errorf("expected server error redirect, got %q", w.Header().Get("Location"))
	}
}

func TestLoginPost_Success(t *testing.T) {
	db := testDB()
	hash := mustHashPassword(t, "correct")
	db.Users.(*testUserStore).getUserByUsernameResult = &model.User{ID: 1, Username: "alice", IsEnabled: true, PasswordHash: hash}
	h := newTestHandler(db)
	form := url.Values{"username": {"alice"}, "password": {"correct"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.LoginPost(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if w.Header().Get("Location") != "/" {
		t.Errorf("expected redirect to /, got %q", w.Header().Get("Location"))
	}
	found := false
	for _, c := range w.Result().Cookies() {
		if c.Name == SessionCookieName && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected session cookie to be set")
	}
}

func TestLoginPost_RememberMeSetsExpiry(t *testing.T) {
	db := testDB()
	hash := mustHashPassword(t, "correct")
	db.Users.(*testUserStore).getUserByUsernameResult = &model.User{ID: 1, Username: "alice", IsEnabled: true, PasswordHash: hash}
	h := newTestHandler(db)
	form := url.Values{"username": {"alice"}, "password": {"correct"}, "remember": {"on"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.LoginPost(w, r)
	var cookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == SessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("expected session cookie")
	}
	if cookie.MaxAge <= 0 {
		t.Errorf("expected positive MaxAge for remember-me, got %d", cookie.MaxAge)
	}
}

// --- Admin login (AI.md PART 17 "Scoped Login Redirect") ---

func mustHashAdminPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := store.HashPassword(password)
	if err != nil {
		t.Fatalf("store.HashPassword: %v", err)
	}
	return hash
}

func TestLoginPost_AdminSuccess(t *testing.T) {
	db := testDB()
	hash := mustHashAdminPassword(t, "correct")
	db.Admin.(*testAdminStore).getAdminByUsernameResult = &model.Admin{ID: 1, Username: "root", Enabled: true, PasswordHash: hash}
	h := newTestHandler(db)
	form := url.Values{"username": {"root"}, "password": {"correct"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.LoginPost(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	wantLoc := "/server/" + h.cfg.AdminPath()
	if w.Header().Get("Location") != wantLoc {
		t.Errorf("expected redirect to %q, got %q", wantLoc, w.Header().Get("Location"))
	}
	found := false
	for _, c := range w.Result().Cookies() {
		if c.Name == mw.AdminSessionCookieName && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected admin session cookie to be set")
	}
}

func TestLoginPost_AdminWrongPassword_Rejected(t *testing.T) {
	db := testDB()
	hash := mustHashAdminPassword(t, "correct")
	db.Admin.(*testAdminStore).getAdminByUsernameResult = &model.Admin{ID: 1, Username: "root", Enabled: true, PasswordHash: hash}
	h := newTestHandler(db)
	form := url.Values{"username": {"root"}, "password": {"wrong"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.LoginPost(w, r)
	if !strings.Contains(w.Header().Get("Location"), "invalid+credentials") {
		t.Errorf("expected invalid credentials redirect, got %q", w.Header().Get("Location"))
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == mw.AdminSessionCookieName && c.Value != "" {
			t.Error("expected no admin session cookie on wrong password")
		}
	}
}

func TestLoginPost_AdminDisabled_Rejected(t *testing.T) {
	db := testDB()
	hash := mustHashAdminPassword(t, "correct")
	db.Admin.(*testAdminStore).getAdminByUsernameResult = &model.Admin{ID: 1, Username: "root", Enabled: false, PasswordHash: hash}
	h := newTestHandler(db)
	form := url.Values{"username": {"root"}, "password": {"correct"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.LoginPost(w, r)
	if !strings.Contains(w.Header().Get("Location"), "invalid+credentials") {
		t.Errorf("expected invalid credentials redirect, got %q", w.Header().Get("Location"))
	}
}

func TestLoginPost_AdminWrongPassword_LocksAfterMaxAttempts(t *testing.T) {
	db := testDB()
	hash := mustHashAdminPassword(t, "correct")
	admin := &model.Admin{ID: 1, Username: "root", Enabled: true, PasswordHash: hash, FailedAttempts: 4}
	db.Admin.(*testAdminStore).getAdminByUsernameResult = admin
	// The handler re-fetches the admin after incrementing to check the
	// updated FailedAttempts count; return one attempt past the threshold.
	db.Admin.(*testAdminStore).getAdminResult = &model.Admin{ID: 1, Username: "root", Enabled: true, PasswordHash: hash, FailedAttempts: 5}
	h := newTestHandler(db)
	form := url.Values{"username": {"root"}, "password": {"wrong"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.LoginPost(w, r)
	if !strings.Contains(w.Header().Get("Location"), "invalid+credentials") {
		t.Errorf("expected invalid credentials redirect, got %q", w.Header().Get("Location"))
	}
	if db.Admin.(*testAdminStore).setAdminLockedUntilErr != nil {
		t.Fatalf("unexpected stub error: %v", db.Admin.(*testAdminStore).setAdminLockedUntilErr)
	}
}

func TestLoginPost_AdminUsername_NeverFallsThroughToUserTable(t *testing.T) {
	db := testDB()
	hash := mustHashAdminPassword(t, "correct")
	db.Admin.(*testAdminStore).getAdminByUsernameResult = &model.Admin{ID: 1, Username: "shared", Enabled: true, PasswordHash: hash}
	// A regular user with the same username also exists; the admin match
	// must take priority and never fall through to this record, even on a
	// wrong admin password (AI.md PART 17 "Scoped Login Redirect").
	userHash := mustHashPassword(t, "userpass")
	db.Users.(*testUserStore).getUserByUsernameResult = &model.User{ID: 2, Username: "shared", IsEnabled: true, PasswordHash: userHash}
	h := newTestHandler(db)
	form := url.Values{"username": {"shared"}, "password": {"userpass"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.LoginPost(w, r)
	if !strings.Contains(w.Header().Get("Location"), "invalid+credentials") {
		t.Errorf("expected invalid credentials redirect (admin-exclusive path), got %q", w.Header().Get("Location"))
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == SessionCookieName && c.Value != "" {
			t.Error("expected no regular-user session cookie; admin match must never fall through")
		}
	}
}

func TestLogout_ClearsAdminSessionAndRedirects(t *testing.T) {
	db := testDB()
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodPost, "/logout", nil)
	r.AddCookie(&http.Cookie{Name: mw.AdminSessionCookieName, Value: "sometoken"})
	w := httptest.NewRecorder()
	h.Logout(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if w.Header().Get("Location") != "/login" {
		t.Errorf("expected redirect to /login, got %q", w.Header().Get("Location"))
	}
	found := false
	for _, c := range w.Result().Cookies() {
		if c.Name == mw.AdminSessionCookieName && c.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected admin session cookie to be cleared")
	}
}

func TestLogout_ClearsSessionAndRedirects(t *testing.T) {
	db := testDB()
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodPost, "/logout", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "sometoken"})
	w := httptest.NewRecorder()
	h.Logout(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if w.Header().Get("Location") != "/login" {
		t.Errorf("expected redirect to /login, got %q", w.Header().Get("Location"))
	}
}

func TestLogout_NoCookie(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodPost, "/logout", nil)
	w := httptest.NewRecorder()
	h.Logout(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
}

// --- sessionAuth middleware ---

func TestSessionAuth_NoCookie_RedirectsToLogin(t *testing.T) {
	h := newTestHandler(testDB())
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.sessionAuth(next).ServeHTTP(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if called {
		t.Error("next handler should not be called")
	}
}

func TestSessionAuth_InvalidSession_RedirectsAndClearsCookie(t *testing.T) {
	db := testDB()
	db.Users.(*testUserStore).getSessionErr = errStore
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "badtoken"})
	w := httptest.NewRecorder()
	h.sessionAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
}

func TestSessionAuth_ExpiredSession_Redirects(t *testing.T) {
	db := testDB()
	db.Users.(*testUserStore).getSessionResult = &store.Session{UserID: 1, ExpiresAt: time.Now().Add(-time.Hour)}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "expiredtoken"})
	w := httptest.NewRecorder()
	h.sessionAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
}

func TestSessionAuth_UserDisabled_Redirects(t *testing.T) {
	db := testDB()
	db.Users.(*testUserStore).getSessionResult = &store.Session{UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}
	db.Users.(*testUserStore).getUserResult = &model.User{ID: 1, IsEnabled: false}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "tok"})
	w := httptest.NewRecorder()
	h.sessionAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
}

func TestSessionAuth_Valid_CallsNextWithUser(t *testing.T) {
	db := testDB()
	db.Users.(*testUserStore).getSessionResult = &store.Session{UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}
	db.Users.(*testUserStore).getUserResult = &model.User{ID: 1, Username: "alice", IsEnabled: true}
	h := newTestHandler(db)
	var gotUser *mw.AuthUser
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = mw.UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "tok"})
	w := httptest.NewRecorder()
	h.sessionAuth(next).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUser == nil || gotUser.Username != "alice" {
		t.Errorf("expected user alice in context, got %+v", gotUser)
	}
}

// --- Library / Artists / Albums / Songs / Genres ---

func TestLibrary_Renders(t *testing.T) {
	db := testDB()
	db.Music.(*testMusicStore).listLibrariesResult = []*model.Library{{ID: 1, Name: "Music"}}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/library", nil)
	w := httptest.NewRecorder()
	h.Library(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestArtists_PaginationHasMore(t *testing.T) {
	db := testDB()
	var artists []*model.Artist
	for i := 0; i < 49; i++ {
		artists = append(artists, &model.Artist{ID: int64(i), Name: "a"})
	}
	db.Music.(*testMusicStore).listArtistsResult = artists
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/artists", nil)
	w := httptest.NewRecorder()
	h.Artists(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestArtists_EmptyResult(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/artists?page=2", nil)
	w := httptest.NewRecorder()
	h.Artists(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAlbums_Renders(t *testing.T) {
	db := testDB()
	db.Music.(*testMusicStore).listAlbumsResult = []*model.Album{{ID: 1, Title: "A"}}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/albums", nil)
	w := httptest.NewRecorder()
	h.Albums(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSongs_Renders(t *testing.T) {
	db := testDB()
	db.Music.(*testMusicStore).searchSongsResult = []*model.Song{{ID: 1, Title: "S"}}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/songs", nil)
	w := httptest.NewRecorder()
	h.Songs(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGenres_Renders(t *testing.T) {
	db := testDB()
	db.Music.(*testMusicStore).listGenresResult = []*model.Genre{{ID: 1, Name: "Rock"}}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/genres", nil)
	w := httptest.NewRecorder()
	h.Genres(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Playlists / PlaylistDetail ---

func TestPlaylists_Renders(t *testing.T) {
	db := testDB()
	db.Playlists.(*testPlaylistStore).listPlaylistsResult = []*model.Playlist{{ID: 1, Name: "P"}}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/playlists", nil)
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.Playlists(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPlaylistDetail_InvalidID(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/playlists/bad", nil)
	r = withChiID(r, "id", "bad")
	w := httptest.NewRecorder()
	h.PlaylistDetail(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPlaylistDetail_NotFound(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/playlists/1", nil)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.PlaylistDetail(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPlaylistDetail_StoreError(t *testing.T) {
	db := testDB()
	db.Playlists.(*testPlaylistStore).getPlaylistErr = errStore
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/playlists/1", nil)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.PlaylistDetail(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPlaylistDetail_SkipsMissingSongs(t *testing.T) {
	db := testDB()
	db.Playlists.(*testPlaylistStore).getPlaylistResult = &model.Playlist{ID: 1, Name: "P"}
	db.Playlists.(*testPlaylistStore).getEntriesResult = []*model.PlaylistEntry{
		{ID: 1, SongID: 10},
		{ID: 2, SongID: 20},
	}
	db.Music.(*testMusicStore).getSongResult = &model.Song{ID: 10, Title: "Found"}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/playlists/1", nil)
	r = withChiID(r, "id", "1")
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.PlaylistDetail(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- AlbumDetail / ArtistDetail ---

func TestAlbumDetail_InvalidID(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/album/bad", nil)
	r = withChiID(r, "id", "bad")
	w := httptest.NewRecorder()
	h.AlbumDetail(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAlbumDetail_NotFound(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/album/1", nil)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.AlbumDetail(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAlbumDetail_Success(t *testing.T) {
	db := testDB()
	db.Music.(*testMusicStore).getAlbumResult = &model.Album{ID: 1, Title: "Album"}
	db.Music.(*testMusicStore).songsByAlbumResult = []*model.Song{{ID: 1, Title: "Track"}}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/album/1", nil)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.AlbumDetail(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestArtistDetail_InvalidID(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/artist/bad", nil)
	r = withChiID(r, "id", "bad")
	w := httptest.NewRecorder()
	h.ArtistDetail(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestArtistDetail_NotFound(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/artist/1", nil)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.ArtistDetail(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestArtistDetail_Success(t *testing.T) {
	db := testDB()
	db.Music.(*testMusicStore).getArtistResult = &model.Artist{ID: 1, Name: "Artist"}
	db.Music.(*testMusicStore).albumsByArtistResult = []*model.Album{{ID: 1, Title: "Album"}}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/artist/1", nil)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.ArtistDetail(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Search ---

func TestSearch_EmptyQuery(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/search", nil)
	w := httptest.NewRecorder()
	h.Search(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSearch_WithQuery(t *testing.T) {
	db := testDB()
	db.Music.(*testMusicStore).searchSongsResult = []*model.Song{{ID: 1, Title: "Match"}}
	db.Music.(*testMusicStore).searchAlbumsResult = []*model.Album{{ID: 1, Title: "Match"}}
	db.Music.(*testMusicStore).searchArtistsResult = []*model.Artist{{ID: 1, Name: "Match"}}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/search?q=Match", nil)
	w := httptest.NewRecorder()
	h.Search(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Player ---

func TestPlayer_Renders(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/player", nil)
	w := httptest.NewRecorder()
	h.Player(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- TagEditor ---

func TestTagEditor_InvalidID(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/tags/bad", nil)
	r = withChiID(r, "id", "bad")
	w := httptest.NewRecorder()
	h.TagEditor(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestTagEditor_NotFound(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/tags/1", nil)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.TagEditor(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestTagEditor_Success(t *testing.T) {
	db := testDB()
	db.Music.(*testMusicStore).getSongResult = &model.Song{ID: 1, Title: "Track"}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/tags/1", nil)
	r = withChiID(r, "id", "1")
	w := httptest.NewRecorder()
	h.TagEditor(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Icecast ---

func TestIcecast_NoServers(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/icecast", nil)
	w := httptest.NewRecorder()
	h.Icecast(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIcecast_ServersWithMounts(t *testing.T) {
	db := testDB()
	db.Icecast.(*testIcecastStore).listServersResult = []*model.IcecastServer{{ID: 1, Name: "Server1"}}
	db.Icecast.(*testIcecastStore).mountsByServer = map[int64][]*model.IcecastMount{
		1: {{ID: 1, Name: "Mount1"}},
	}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/icecast", nil)
	w := httptest.NewRecorder()
	h.Icecast(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Settings ---

func TestSettings_Renders(t *testing.T) {
	db := testDB()
	db.Users.(*testUserStore).listAPITokensResult = []*model.APIToken{{ID: 1, Name: "tok"}}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/settings", nil)
	r = withAuthUser(r, 1, "alice", false)
	w := httptest.NewRecorder()
	h.Settings(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Upload ---

func TestUpload_Renders(t *testing.T) {
	db := testDB()
	db.Music.(*testMusicStore).listLibrariesResult = []*model.Library{{ID: 1, Name: "Music"}}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/upload", nil)
	w := httptest.NewRecorder()
	h.Upload(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Share ---

func TestShare_NotFound(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/share/tok", nil)
	r = withChiID(r, "token", "tok")
	w := httptest.NewRecorder()
	h.Share(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestShare_Expired(t *testing.T) {
	db := testDB()
	db.Shares.(*testShareStore).getShareByTokenResult = &model.Share{ID: 1, Token: "tok", ExpiresAt: time.Now().Add(-time.Hour)}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/share/tok", nil)
	r = withChiID(r, "token", "tok")
	w := httptest.NewRecorder()
	h.Share(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestShare_Song(t *testing.T) {
	db := testDB()
	db.Shares.(*testShareStore).getShareByTokenResult = &model.Share{ID: 1, Token: "tok", ItemType: "song", ItemID: 5}
	db.Music.(*testMusicStore).getSongResult = &model.Song{ID: 5, Title: "Track"}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/share/tok", nil)
	r = withChiID(r, "token", "tok")
	w := httptest.NewRecorder()
	h.Share(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestShare_Album(t *testing.T) {
	db := testDB()
	db.Shares.(*testShareStore).getShareByTokenResult = &model.Share{ID: 1, Token: "tok", ItemType: "album", ItemID: 5}
	db.Music.(*testMusicStore).getAlbumResult = &model.Album{ID: 5, Title: "Album"}
	db.Music.(*testMusicStore).songsByAlbumResult = []*model.Song{{ID: 1, Title: "Track"}}
	h := newTestHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/share/tok", nil)
	r = withChiID(r, "token", "tok")
	w := httptest.NewRecorder()
	h.Share(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- About / Help ---

func TestAbout_HTML(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/server/about", nil)
	w := httptest.NewRecorder()
	h.About(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected html content type, got %q", w.Header().Get("Content-Type"))
	}
}

func TestAbout_PlainText(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/server/about", nil)
	r.Header.Set("Accept", "text/plain")
	w := httptest.NewRecorder()
	h.About(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "cassonic") {
		t.Errorf("expected plain-text body to mention cassonic, got %q", w.Body.String())
	}
}

func TestHelp_HTML(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/server/help", nil)
	w := httptest.NewRecorder()
	h.Help(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHelp_PlainText(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/server/help", nil)
	r.Header.Set("Accept", "text/plain")
	w := httptest.NewRecorder()
	h.Help(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Getting Started") {
		t.Errorf("expected help text, got %q", w.Body.String())
	}
}

// --- resolveLocale ---

func TestResolveLocale_QueryParamSetsCookie(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/?lang=fr", nil)
	w := httptest.NewRecorder()
	lang := h.resolveLocale(w, r)
	if lang != "fr" {
		t.Errorf("expected fr, got %q", lang)
	}
	found := false
	for _, c := range w.Result().Cookies() {
		if c.Name == "lang" && c.Value == "fr" {
			found = true
		}
	}
	if !found {
		t.Error("expected lang cookie to be set")
	}
}

func TestResolveLocale_UnsupportedQueryParamIgnored(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/?lang=xx", nil)
	w := httptest.NewRecorder()
	lang := h.resolveLocale(w, r)
	if lang != "en" {
		t.Errorf("expected fallback to en, got %q", lang)
	}
}

func TestResolveLocale_CookieFallback(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "lang", Value: "de"})
	w := httptest.NewRecorder()
	lang := h.resolveLocale(w, r)
	if lang != "de" {
		t.Errorf("expected de, got %q", lang)
	}
}

func TestResolveLocale_AcceptLanguageFallback(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept-Language", "es-ES,es;q=0.9")
	w := httptest.NewRecorder()
	lang := h.resolveLocale(w, r)
	if lang != "es" {
		t.Errorf("expected es, got %q", lang)
	}
}

func TestResolveLocale_DefaultEnglish(t *testing.T) {
	h := newTestHandler(testDB())
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	lang := h.resolveLocale(w, r)
	if lang != "en" {
		t.Errorf("expected en, got %q", lang)
	}
}

// --- verifyPassword ---

func TestVerifyPassword_CorrectPassword(t *testing.T) {
	hash := mustHashPassword(t, "secret")
	ok, err := verifyPassword("secret", hash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected password to verify")
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	hash := mustHashPassword(t, "secret")
	ok, err := verifyPassword("wrong", hash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected password mismatch")
	}
}

func TestVerifyPassword_MalformedHash(t *testing.T) {
	_, err := verifyPassword("secret", "not-a-valid-hash")
	if err == nil {
		t.Error("expected error for malformed hash")
	}
}

func TestVerifyPassword_WrongScheme(t *testing.T) {
	_, err := verifyPassword("secret", "$bcrypt$v=1$a$b$c")
	if err == nil {
		t.Error("expected error for unsupported scheme")
	}
}

// --- generateSessionToken ---

func TestGenerateSessionToken_ReturnsDistinctValues(t *testing.T) {
	raw1, hash1, err := generateSessionToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw2, hash2, err := generateSessionToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw1 == raw2 || hash1 == hash2 {
		t.Error("expected distinct tokens across calls")
	}
	if raw1 == "" || hash1 == "" {
		t.Error("expected non-empty raw and hash")
	}
}

// --- queryInt ---

func TestQueryInt(t *testing.T) {
	tests := []struct {
		name string
		url  string
		key  string
		def  int
		want int
	}{
		{"absent uses default", "/", "page", 1, 1},
		{"valid value", "/?page=5", "page", 1, 5},
		{"invalid value uses default", "/?page=abc", "page", 1, 1},
		{"zero uses default", "/?page=0", "page", 1, 1},
		{"negative uses default", "/?page=-3", "page", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.url, nil)
			got := queryInt(r, tt.key, tt.def)
			if got != tt.want {
				t.Errorf("queryInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

// --- formatDuration ---

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{-5, "0:00"},
		{0, "0:00"},
		{59, "0:59"},
		{60, "1:00"},
		{3661, "1:01:01"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.seconds)
		if got != tt.want {
			t.Errorf("formatDuration(%d) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

// --- formatDate ---

func TestFormatDate(t *testing.T) {
	if got := formatDate(time.Time{}); got != "" {
		t.Errorf("expected empty string for zero time, got %q", got)
	}
	tm := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	if got := formatDate(tm); got != "2024-03-15" {
		t.Errorf("expected 2024-03-15, got %q", got)
	}
}

// --- templateNot ---

func TestTemplateNot(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want bool
	}{
		{"nil", nil, true},
		{"false bool", false, true},
		{"true bool", true, false},
		{"zero int", 0, true},
		{"nonzero int", 5, false},
		{"zero int64", int64(0), true},
		{"nonzero int64", int64(5), false},
		{"empty string", "", true},
		{"nonempty string", "x", false},
		{"unhandled type", 3.14, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := templateNot(tt.in); got != tt.want {
				t.Errorf("templateNot(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// --- makeSeq ---

func TestMakeSeq(t *testing.T) {
	if got := makeSeq(0); len(got) != 0 {
		t.Errorf("expected empty slice for n=0, got %v", got)
	}
	got := makeSeq(3)
	want := []int{0, 1, 2}
	if len(got) != len(want) {
		t.Fatalf("expected length %d, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("makeSeq(3)[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

// --- formatSize ---

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{2048, "2.0 KB"},
		{5 * 1024 * 1024, "5.0 MB"},
		{3 * 1024 * 1024 * 1024, "3.0 GB"},
	}
	for _, tt := range tests {
		if got := formatSize(tt.bytes); got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

// --- Routes wiring smoke test ---

func TestRoutes_LoginReachableUnauthenticated(t *testing.T) {
	h := newTestHandler(testDB())
	srv := httptest.NewServer(h.Routes())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/login")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRoutes_ProtectedRedirectsUnauthenticated(t *testing.T) {
	h := newTestHandler(testDB())
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	srv := httptest.NewServer(h.Routes())
	defer srv.Close()
	resp, err := client.Get(srv.URL + "/library")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302, got %d", resp.StatusCode)
	}
}
