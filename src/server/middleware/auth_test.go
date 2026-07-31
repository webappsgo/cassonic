package middleware

// Tests in this file cover the auth-related middleware that middleware_test.go
// does not already exercise:
//   - auth.go: UserFromContext, WithUser, RequireAuth, RequireAdmin,
//     SubsonicClientFromContext, Cors
//   - auth_ampache.go: AmpacheAuth (no token, unknown token, unknown/locked
//     user, success)
//   - auth_native.go: NativeAuth (no token, session path success/failure/
//     expired, API-token fallback path success/failure), extractBearerToken
//   - auth_subsonic.go: SubsonicAuth (skip, unknown/locked user, token auth,
//     plaintext auth, hex-encoded auth, default no-credentials case),
//     decodeSubsonicPassword, md5Hex, verifyArgon2id, writeSubsonicError
//   - middleware.go: IPFilter (allow list, block list), RateLimiter.Cleanup

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// --- auth.go ---

func TestUserFromContextEmpty(t *testing.T) {
	if u := UserFromContext(context.Background()); u != nil {
		t.Errorf("expected nil user, got %+v", u)
	}
}

func TestWithUserAndUserFromContext(t *testing.T) {
	u := &AuthUser{ID: 1, Username: "alice"}
	ctx := WithUser(context.Background(), u)
	got := UserFromContext(ctx)
	if got == nil || got.Username != "alice" {
		t.Errorf("WithUser/UserFromContext round trip: got %+v", got)
	}
}

func TestSubsonicClientFromContextEmpty(t *testing.T) {
	if c := SubsonicClientFromContext(context.Background()); c != "" {
		t.Errorf("expected empty client, got %q", c)
	}
}

func TestRequireAuthNoUser(t *testing.T) {
	mw := RequireAuth()(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("WWW-Authenticate header not set")
	}
}

func TestRequireAuthWithUser(t *testing.T) {
	mw := RequireAuth()(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := WithUser(req.Context(), &AuthUser{ID: 1})
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestRequireAdminNoUser(t *testing.T) {
	mw := RequireAdmin()(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestRequireAdminNonAdmin(t *testing.T) {
	mw := RequireAdmin()(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := WithUser(req.Context(), &AuthUser{ID: 1, IsAdmin: false})
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestRequireAdminIsAdmin(t *testing.T) {
	mw := RequireAdmin()(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := WithUser(req.Context(), &AuthUser{ID: 1, IsAdmin: true})
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestCorsPreflight(t *testing.T) {
	mw := Cors()(okHandler())
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Access-Control-Allow-Origin not set to *")
	}
}

func TestCorsNonPreflight(t *testing.T) {
	mw := Cors()(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("Access-Control-Allow-Methods not set")
	}
}

// --- fakeAuthStore: extends stubUserStore with configurable session/token lookups ---

type fakeAuthStore struct {
	*stubUserStore
	session        *store.Session
	sessionErr     error
	deleteSessionN int
	apiToken       *model.APIToken
	apiTokenErr    error
}

func newFakeAuthStore() *fakeAuthStore {
	return &fakeAuthStore{stubUserStore: newStubUserStore()}
}

func (f *fakeAuthStore) GetSessionByHash(_ context.Context, _ string) (*store.Session, error) {
	return f.session, f.sessionErr
}

func (f *fakeAuthStore) DeleteSession(_ context.Context, _ string) error {
	f.deleteSessionN++
	return nil
}

func (f *fakeAuthStore) GetAPITokenByHash(_ context.Context, _ string) (*model.APIToken, error) {
	return f.apiToken, f.apiTokenErr
}

// --- auth_ampache.go: AmpacheAuth ---

func TestAmpacheAuthNoToken(t *testing.T) {
	users := newStubUserStore()
	sessions := NewAmpacheSessionStore(time.Hour)
	mw := AmpacheAuth(users, sessions)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestAmpacheAuthUnknownToken(t *testing.T) {
	users := newStubUserStore()
	sessions := NewAmpacheSessionStore(time.Hour)
	mw := AmpacheAuth(users, sessions)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/?auth=nope", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (pass-through)", rec.Code)
	}
}

func TestAmpacheAuthUnknownUser(t *testing.T) {
	users := newStubUserStore()
	sessions := NewAmpacheSessionStore(time.Hour)
	token := sessions.Create(999)
	mw := AmpacheAuth(users, sessions)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/?auth="+token, nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (pass-through)", rec.Code)
	}
}

func TestAmpacheAuthLockedUser(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{ID: 1, Username: "locked", IsEnabled: true, LockedUntil: time.Now().Add(time.Hour)})
	sessions := NewAmpacheSessionStore(time.Hour)
	token := sessions.Create(1)
	mw := AmpacheAuth(users, sessions)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/?auth="+token, nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestAmpacheAuthSuccess(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{ID: 1, Username: "alice", IsEnabled: true, IsAdmin: true})
	sessions := NewAmpacheSessionStore(time.Hour)
	token := sessions.Create(1)

	var gotUser *AuthUser
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mw := AmpacheAuth(users, sessions)(next)

	req := httptest.NewRequest(http.MethodGet, "/?auth="+token, nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if gotUser == nil || gotUser.Username != "alice" || gotUser.Scheme != SchemeAmpache {
		t.Errorf("context user: got %+v", gotUser)
	}
}

// --- auth_native.go: extractBearerToken ---

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(r *http.Request)
		wantTk string
	}{
		{"Bearer prefix", func(r *http.Request) { r.Header.Set("Authorization", "Bearer abc123") }, "abc123"},
		{"lowercase bearer", func(r *http.Request) { r.Header.Set("Authorization", "bearer xyz789") }, "xyz789"},
		{"malformed short header", func(r *http.Request) { r.Header.Set("Authorization", "Bearer") }, ""},
		{"non-bearer scheme", func(r *http.Request) { r.Header.Set("Authorization", "Basic abc123") }, ""},
		{"query token fallback", func(r *http.Request) {}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tt.setup(req)
			got := extractBearerToken(req)
			if got != tt.wantTk {
				t.Errorf("extractBearerToken: got %q, want %q", got, tt.wantTk)
			}
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/?token=qtok", nil)
	if got := extractBearerToken(req); got != "qtok" {
		t.Errorf("query token: got %q, want %q", got, "qtok")
	}
}

// --- auth_native.go: NativeAuth ---

func TestNativeAuthNoToken(t *testing.T) {
	f := newFakeAuthStore()
	mw := NativeAuth(f)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestNativeAuthSessionExpired(t *testing.T) {
	f := newFakeAuthStore()
	f.session = &store.Session{UserID: 1, ExpiresAt: time.Now().Add(-time.Hour)}
	mw := NativeAuth(f)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (pass-through)", rec.Code)
	}
	if f.deleteSessionN != 1 {
		t.Errorf("expected stale session deleted once, got %d calls", f.deleteSessionN)
	}
}

func TestNativeAuthSessionUserLocked(t *testing.T) {
	f := newFakeAuthStore()
	f.add(&model.User{ID: 5, Username: "locked", IsEnabled: true, LockedUntil: time.Now().Add(time.Hour)})
	f.session = &store.Session{UserID: 5, ExpiresAt: time.Now().Add(time.Hour)}
	mw := NativeAuth(f)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestNativeAuthSessionSuccess(t *testing.T) {
	f := newFakeAuthStore()
	f.add(&model.User{ID: 5, Username: "bob", IsEnabled: true})
	f.session = &store.Session{UserID: 5, ExpiresAt: time.Now().Add(time.Hour)}

	var gotUser *AuthUser
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mw := NativeAuth(f)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if gotUser == nil || gotUser.Username != "bob" || gotUser.Scheme != SchemeNative {
		t.Errorf("context user: got %+v", gotUser)
	}
}

func TestNativeAuthSessionUserNotFound(t *testing.T) {
	f := newFakeAuthStore()
	f.session = &store.Session{UserID: 999, ExpiresAt: time.Now().Add(time.Hour)}
	mw := NativeAuth(f)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (pass-through)", rec.Code)
	}
}

func TestNativeAuthAPITokenExpired(t *testing.T) {
	f := newFakeAuthStore()
	f.apiToken = &model.APIToken{ID: 1, UserID: 1, ExpiresAt: time.Now().Add(-time.Hour)}
	mw := NativeAuth(f)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (pass-through)", rec.Code)
	}
}

func TestNativeAuthAPITokenUserLocked(t *testing.T) {
	f := newFakeAuthStore()
	f.add(&model.User{ID: 2, Username: "locked2", IsEnabled: true, LockedUntil: time.Now().Add(time.Hour)})
	f.apiToken = &model.APIToken{ID: 1, UserID: 2}
	mw := NativeAuth(f)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestNativeAuthAPITokenSuccess(t *testing.T) {
	f := newFakeAuthStore()
	f.add(&model.User{ID: 3, Username: "carol", IsEnabled: true})
	f.apiToken = &model.APIToken{ID: 1, UserID: 3}

	var gotUser *AuthUser
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mw := NativeAuth(f)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if gotUser == nil || gotUser.Username != "carol" {
		t.Errorf("context user: got %+v", gotUser)
	}
}

func TestNativeAuthAPITokenNotFound(t *testing.T) {
	f := newFakeAuthStore()
	mw := NativeAuth(f)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (pass-through)", rec.Code)
	}
}

func TestNativeAuthAPITokenUserNotFound(t *testing.T) {
	f := newFakeAuthStore()
	f.apiToken = &model.APIToken{ID: 1, UserID: 999}
	mw := NativeAuth(f)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (pass-through)", rec.Code)
	}
}

// --- auth_subsonic.go ---

// argon2Hash builds a valid argon2id hash string for the given plaintext using
// fixed test parameters, matching the format verifyArgon2id expects.
func argon2Hash(t *testing.T, plaintext string) string {
	t.Helper()
	salt := []byte("0123456789abcdef")
	memory := uint32(64 * 1024)
	iterations := uint32(1)
	parallelism := uint8(1)
	key := argon2.IDKey([]byte(plaintext), salt, iterations, memory, parallelism, 32)
	return "$argon2id$v=19$m=" + strconv.Itoa(int(memory)) + ",t=" + strconv.Itoa(int(iterations)) +
		",p=" + strconv.Itoa(int(parallelism)) +
		"$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key)
}

func TestSubsonicAuthSkipNoUsername(t *testing.T) {
	users := newStubUserStore()
	mw := SubsonicAuth(users, nil)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestSubsonicAuthUnknownUserXML(t *testing.T) {
	users := newStubUserStore()
	mw := SubsonicAuth(users, nil)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/?u=nobody&p=pw", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (subsonic errors are HTTP 200)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `status="failed"`) {
		t.Errorf("expected failed status in XML body, got: %s", rec.Body.String())
	}
	var parsed struct {
		XMLName xml.Name `xml:"subsonic-response"`
		Error   struct {
			Code int `xml:"code,attr"`
		} `xml:"error"`
	}
	if err := xml.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("xml.Unmarshal: %v", err)
	}
	if parsed.Error.Code != subsonicErrorWrongCredentials {
		t.Errorf("error code: got %d, want %d", parsed.Error.Code, subsonicErrorWrongCredentials)
	}
}

func TestSubsonicAuthUnknownUserJSON(t *testing.T) {
	users := newStubUserStore()
	mw := SubsonicAuth(users, nil)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/?u=nobody&p=pw&f=json", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
	if !strings.Contains(rec.Body.String(), `"status":"failed"`) {
		t.Errorf("expected failed status in JSON body, got: %s", rec.Body.String())
	}
}

func TestSubsonicAuthLockedUser(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{ID: 1, Username: "locked", IsEnabled: true, LockedUntil: time.Now().Add(time.Hour)})
	mw := SubsonicAuth(users, nil)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/?u=locked&p=pw", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `status="failed"`) {
		t.Error("expected failed status for locked user")
	}
}

func TestSubsonicAuthDefaultNoCredentials(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{ID: 1, Username: "dave", IsEnabled: true})
	mw := SubsonicAuth(users, nil)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/?u=dave", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `status="failed"`) {
		t.Error("expected failed status when no auth params supplied")
	}
}

func TestSubsonicAuthTokenSuccess(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{ID: 1, Username: "eve", IsEnabled: true})
	getSubsonicPassword := func(_ context.Context, _ string) (string, bool) { return "subpass", true }

	var gotUser *AuthUser
	var gotClient string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		gotClient = SubsonicClientFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mw := SubsonicAuth(users, getSubsonicPassword)(next)

	salt := "salt123"
	sum := md5.Sum([]byte("subpass" + salt))
	token := hex.EncodeToString(sum[:])

	req := httptest.NewRequest(http.MethodGet, "/?u=eve&t="+token+"&s="+salt+"&c=myclient", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if gotUser == nil || gotUser.Username != "eve" || gotUser.Scheme != SchemeSubsonic {
		t.Errorf("context user: got %+v", gotUser)
	}
	if gotClient != "myclient" {
		t.Errorf("client: got %q, want %q", gotClient, "myclient")
	}
}

func TestSubsonicAuthTokenWrongToken(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{ID: 1, Username: "frank", IsEnabled: true})
	getSubsonicPassword := func(_ context.Context, _ string) (string, bool) { return "subpass", true }
	mw := SubsonicAuth(users, getSubsonicPassword)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/?u=frank&t=deadbeef&s=salt123", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `status="failed"`) {
		t.Error("expected failed status for wrong token")
	}
}

func TestSubsonicAuthTokenNoSubsonicPassword(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{ID: 1, Username: "grace", IsEnabled: true})
	getSubsonicPassword := func(_ context.Context, _ string) (string, bool) { return "", false }
	mw := SubsonicAuth(users, getSubsonicPassword)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/?u=grace&t=abc&s=salt", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	var parsed struct {
		XMLName xml.Name `xml:"subsonic-response"`
		Error   struct {
			Code int `xml:"code,attr"`
		} `xml:"error"`
	}
	if err := xml.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("xml.Unmarshal: %v", err)
	}
	if parsed.Error.Code != subsonicErrorTokenAuthNotSupported {
		t.Errorf("error code: got %d, want %d", parsed.Error.Code, subsonicErrorTokenAuthNotSupported)
	}
}

func TestSubsonicAuthPlaintextSuccess(t *testing.T) {
	users := newStubUserStore()
	hash := argon2Hash(t, "mypassword")
	users.add(&model.User{ID: 1, Username: "heidi", IsEnabled: true, PasswordHash: hash})

	var gotUser *AuthUser
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mw := SubsonicAuth(users, nil)(next)

	req := httptest.NewRequest(http.MethodGet, "/?u=heidi&p=mypassword", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if gotUser == nil || gotUser.Username != "heidi" {
		t.Errorf("context user: got %+v", gotUser)
	}
}

func TestSubsonicAuthPlaintextWrongPassword(t *testing.T) {
	users := newStubUserStore()
	hash := argon2Hash(t, "mypassword")
	users.add(&model.User{ID: 1, Username: "ivan", IsEnabled: true, PasswordHash: hash})
	mw := SubsonicAuth(users, nil)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/?u=ivan&p=wrongpassword", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `status="failed"`) {
		t.Error("expected failed status for wrong plaintext password")
	}
}

func TestSubsonicAuthHexEncodedSuccess(t *testing.T) {
	users := newStubUserStore()
	hash := argon2Hash(t, "hexpass")
	users.add(&model.User{ID: 1, Username: "judy", IsEnabled: true, PasswordHash: hash})
	mw := SubsonicAuth(users, nil)(okHandler())

	encoded := hex.EncodeToString([]byte("hexpass"))
	req := httptest.NewRequest(http.MethodGet, "/?u=judy&p=enc:"+encoded, nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `status="failed"`) {
		t.Errorf("expected success, got: %s", rec.Body.String())
	}
}

func TestSubsonicAuthHexEncodedInvalid(t *testing.T) {
	users := newStubUserStore()
	hash := argon2Hash(t, "hexpass")
	users.add(&model.User{ID: 1, Username: "karl", IsEnabled: true, PasswordHash: hash})
	mw := SubsonicAuth(users, nil)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/?u=karl&p=enc:zzznothex", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `status="failed"`) {
		t.Error("expected failed status for invalid hex-encoded password")
	}
}

func TestSubsonicAuthMalformedHash(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{ID: 1, Username: "leo", IsEnabled: true, PasswordHash: "not-a-valid-hash"})
	mw := SubsonicAuth(users, nil)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/?u=leo&p=anything", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `status="failed"`) {
		t.Error("expected failed status for malformed hash")
	}
}

// --- middleware.go: IPFilter ---

func TestIPFilterAllowListPermits(t *testing.T) {
	f := NewIPFilter([]string{"10.0.0.0/8"}, nil)
	mw := f.Middleware()(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestIPFilterAllowListDenies(t *testing.T) {
	f := NewIPFilter([]string{"10.0.0.0/8"}, nil)
	mw := f.Middleware()(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "8.8.8.8:1234"
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestIPFilterBlockListDenies(t *testing.T) {
	f := NewIPFilter(nil, []string{"8.8.8.8"})
	mw := f.Middleware()(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "8.8.8.8:1234"
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestIPFilterBlockListPermitsOthers(t *testing.T) {
	f := NewIPFilter(nil, []string{"8.8.8.8"})
	mw := f.Middleware()(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestIPFilterNoLists(t *testing.T) {
	f := NewIPFilter(nil, nil)
	mw := f.Middleware()(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestIPFilterIgnoresInvalidEntries(t *testing.T) {
	f := NewIPFilter([]string{"not-an-ip", "", "192.168.0.0/16"}, nil)
	mw := f.Middleware()(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (valid CIDR should still match)", rec.Code)
	}
}

func TestIPFilterSingleIPEntry(t *testing.T) {
	f := NewIPFilter([]string{"203.0.113.5"}, nil)
	mw := f.Middleware()(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:1234"
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

// --- middleware.go: RateLimiter.Cleanup ---

func TestRateLimiterCleanupRemovesIdleBuckets(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	rl.Allow("stale-key")

	rl.mu.Lock()
	rl.buckets["stale-key"].lastRefill = time.Now().Add(-10 * time.Minute)
	rl.mu.Unlock()

	rl.Cleanup()

	rl.mu.Lock()
	_, ok := rl.buckets["stale-key"]
	rl.mu.Unlock()

	if ok {
		t.Error("expected stale bucket to be removed by Cleanup")
	}
}

func TestRateLimiterCleanupKeepsFreshBuckets(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	rl.Allow("fresh-key")

	rl.Cleanup()

	rl.mu.Lock()
	_, ok := rl.buckets["fresh-key"]
	rl.mu.Unlock()

	if !ok {
		t.Error("expected fresh bucket to survive Cleanup")
	}
}
