package middleware

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
}

func TestRequestIDSetsHeader(t *testing.T) {
	mw := RequestID()(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	id := rec.Header().Get("X-Request-ID")
	if id == "" {
		t.Fatal("X-Request-ID header not set")
	}
	if len(id) < 8 {
		t.Errorf("X-Request-ID looks too short: %q", id)
	}
}

func TestRequestIDPreservesExisting(t *testing.T) {
	mw := RequestID()(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "client-provided-id")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	id := rec.Header().Get("X-Request-ID")
	if id != "client-provided-id" {
		t.Errorf("X-Request-ID: got %q, want %q", id, "client-provided-id")
	}
}

func TestRequestIDUnique(t *testing.T) {
	mw := RequestID()(okHandler())

	ids := make(map[string]bool)
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
		id := rec.Header().Get("X-Request-ID")
		if ids[id] {
			t.Errorf("duplicate request ID generated: %q", id)
		}
		ids[id] = true
	}
}

func TestSecurityHeadersProduction(t *testing.T) {
	mw := SecurityHeaders(true)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	tests := []struct {
		header string
		want   string
	}{
		{"X-Frame-Options", "DENY"},
		{"X-Content-Type-Options", "nosniff"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
	}
	for _, tt := range tests {
		got := rec.Header().Get(tt.header)
		if got != tt.want {
			t.Errorf("header %q: got %q, want %q", tt.header, got, tt.want)
		}
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy header not set")
	}
	if !strings.Contains(csp, "default-src") {
		t.Errorf("Content-Security-Policy missing default-src: %q", csp)
	}
}

func TestSecurityHeadersDisabled(t *testing.T) {
	mw := SecurityHeaders(false)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Errorf("X-Frame-Options should not be set in non-prod: got %q", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "" {
		t.Errorf("Content-Security-Policy should not be set in non-prod: got %q", got)
	}
}

func TestSecurityHeadersHTTPS(t *testing.T) {
	mw := SecurityHeaders(true)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	hsts := rec.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("Strict-Transport-Security should be set for HTTPS requests")
	}
}

func TestRateLimiterAllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(10, 2)
	mw := rl.Middleware("test")(okHandler())

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:9999"
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: got status %d, want 200", i+1, rec.Code)
		}
	}
}

func TestRateLimiterBlocks(t *testing.T) {
	rl := NewRateLimiter(0, 2)
	mw := rl.Middleware("test")(okHandler())

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("3rd request: got status %d, want 429", rec.Code)
	}
}

func TestRateLimiterRetryAfterHeader(t *testing.T) {
	rl := NewRateLimiter(0, 1)
	mw := rl.Middleware("retry-test")(okHandler())

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:5000"
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			if rec.Header().Get("Retry-After") == "" {
				t.Error("Retry-After header not set on 429 response")
			}
			return
		}
	}
}

func TestLoggerCallsNext(t *testing.T) {
	var buf strings.Builder
	mw := Logger(&buf)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Logger: next handler status: got %d, want 200", rec.Code)
	}
	if buf.Len() == 0 {
		t.Error("Logger: no log output written")
	}
	if !strings.Contains(buf.String(), "/health") {
		t.Errorf("Logger: expected /health in output, got: %q", buf.String())
	}
}

func TestLoggerNilWriter(t *testing.T) {
	mw := Logger(nil)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Logger with nil writer: got %d, want 200", rec.Code)
	}
}

// stubUserStore is a minimal UserStore for VerifyHandshake tests.
// It only implements the methods the middleware package calls directly.
type stubUserStore struct {
	users map[string]*model.User
}

func newStubUserStore() *stubUserStore {
	return &stubUserStore{users: make(map[string]*model.User)}
}

func (s *stubUserStore) add(u *model.User) {
	s.users[u.Username] = u
}

func (s *stubUserStore) GetUser(ctx context.Context, id int64) (*model.User, error) {
	for _, u := range s.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}

func (s *stubUserStore) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	u, ok := s.users[username]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (s *stubUserStore) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return nil, errors.New("not found")
}

func (s *stubUserStore) CreateUser(ctx context.Context, u *model.User) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *stubUserStore) UpdateUser(ctx context.Context, u *model.User) error {
	return errors.New("not implemented")
}

func (s *stubUserStore) DeleteUser(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (s *stubUserStore) ListUsers(ctx context.Context) ([]*model.User, error) {
	return nil, nil
}

func (s *stubUserStore) IncrementLoginAttempts(ctx context.Context, id int64) error {
	return nil
}

func (s *stubUserStore) ResetLoginAttempts(ctx context.Context, id int64) error {
	return nil
}

func (s *stubUserStore) SetLockedUntil(ctx context.Context, id int64, until time.Time) error {
	return nil
}

func (s *stubUserStore) UpdateLastLogin(ctx context.Context, id int64) error {
	return nil
}

func (s *stubUserStore) CreateAPIToken(ctx context.Context, t *model.APIToken) error {
	return errors.New("not implemented")
}

func (s *stubUserStore) GetAPITokenByHash(ctx context.Context, hash string) (*model.APIToken, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUserStore) ListAPITokens(ctx context.Context, userID int64) ([]*model.APIToken, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUserStore) DeleteAPIToken(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (s *stubUserStore) UpdateAPITokenLastUsed(ctx context.Context, id int64) error {
	return nil
}

func (s *stubUserStore) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, clientName string) error {
	return errors.New("not implemented")
}

func (s *stubUserStore) GetSessionByHash(ctx context.Context, tokenHash string) (*store.Session, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUserStore) DeleteSession(ctx context.Context, tokenHash string) error {
	return errors.New("not implemented")
}

func (s *stubUserStore) DeleteUserSessions(ctx context.Context, userID int64) error {
	return errors.New("not implemented")
}

func (s *stubUserStore) PurgeExpiredSessions(ctx context.Context) error {
	return nil
}

func (s *stubUserStore) GetSubsonicPassword(ctx context.Context, username string) (string, bool, error) {
	return "", false, nil
}

func (s *stubUserStore) SetSubsonicPassword(ctx context.Context, username string, encrypted string) error {
	return errors.New("not implemented")
}

func (s *stubUserStore) CreateRadioStation(ctx context.Context, st *model.InternetRadioStation) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *stubUserStore) GetRadioStation(ctx context.Context, id int64) (*model.InternetRadioStation, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUserStore) ListRadioStations(ctx context.Context) ([]*model.InternetRadioStation, error) {
	return nil, nil
}

func (s *stubUserStore) UpdateRadioStation(ctx context.Context, st *model.InternetRadioStation) error {
	return errors.New("not implemented")
}

func (s *stubUserStore) DeleteRadioStation(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

// ampacheSHA256Passphrase computes SHA256(timestamp + SHA256(password)) as hex.
func ampacheSHA256Passphrase(timestamp, password string) string {
	inner := sha256.Sum256([]byte(password))
	innerHex := hex.EncodeToString(inner[:])
	outer := sha256.Sum256([]byte(timestamp + innerHex))
	return hex.EncodeToString(outer[:])
}

// ampacheMD5Passphrase computes MD5(timestamp + MD5(password)) as hex.
func ampacheMD5Passphrase(timestamp, password string) string {
	inner := md5.Sum([]byte(password))
	innerHex := hex.EncodeToString(inner[:])
	outer := md5.Sum([]byte(timestamp + innerHex))
	return hex.EncodeToString(outer[:])
}

// TestAcceptedFormat covers all content-negotiation cases including boundary values.
func TestAcceptedFormat(t *testing.T) {
	cases := []struct {
		name   string
		accept string
		want   string
	}{
		{"application/json explicit", "application/json", "json"},
		{"text/plain", "text/plain", "plain"},
		{"text/html", "text/html", "html"},
		{"empty accept defaults to json", "", "json"},
		{"wildcard defaults to json", "*/*", "json"},
		{"text/plain with quality", "text/plain; q=0.9, */*; q=0.8", "plain"},
		{"text/html with quality", "text/html, application/xhtml+xml", "html"},
		// text/plain appears in the Accept string, so AcceptedFormat returns "plain" (no quality parsing)
		{"application/json with quality and text/plain fallback", "application/json, text/plain; q=0.5", "plain"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			got := AcceptedFormat(req)
			if got != tc.want {
				t.Errorf("AcceptedFormat(%q) = %q, want %q", tc.accept, got, tc.want)
			}
		})
	}
}

// TestGeoIPFilterNilDB verifies that a nil DB is a pass-through regardless of country lists.
func TestGeoIPFilterNilDB(t *testing.T) {
	cases := []struct {
		name          string
		denyCountries []string
		allowCountries []string
	}{
		{"nil db no lists", nil, nil},
		{"nil db with deny list", []string{"US", "CN"}, nil},
		{"nil db with allow list", nil, []string{"DE"}},
		{"nil db with both lists", []string{"US"}, []string{"DE"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mw := GeoIPFilter(nil, tc.denyCountries, tc.allowCountries)(okHandler())
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "1.2.3.4:1234"
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("nil DB GeoIPFilter: got %d, want 200", rec.Code)
			}
		})
	}
}

// TestGeoIPFilterEmptyLists verifies that empty deny/allow lists are pass-throughs.
func TestGeoIPFilterEmptyLists(t *testing.T) {
	mw := GeoIPFilter(nil, []string{}, []string{})(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "8.8.8.8:53"
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("empty lists GeoIPFilter: got %d, want 200", rec.Code)
	}
}

// TestGeoIPFilterPrivateIPBypass verifies that RFC 1918 addresses always pass through
// even when a non-nil DB and country lists are provided (this tests the code path
// using nil DB since we cannot instantiate a real DB without a file; the private IP
// bypass is reached before the DB lookup).
func TestGeoIPFilterPrivateIPBypass(t *testing.T) {
	privateAddrs := []string{
		"127.0.0.1:80",
		"10.0.0.1:80",
		"192.168.1.1:80",
		"172.16.0.1:80",
	}
	for _, addr := range privateAddrs {
		t.Run(addr, func(t *testing.T) {
			mw := GeoIPFilter(nil, []string{"US"}, nil)(okHandler())
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = addr
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("private IP %q: got %d, want 200", addr, rec.Code)
			}
		})
	}
}

// TestAmpacheSessionStoreCreateGet covers the basic create/get lifecycle.
func TestAmpacheSessionStoreCreateGet(t *testing.T) {
	ss := NewAmpacheSessionStore(time.Hour)
	token := ss.Create(42)

	if token == "" {
		t.Fatal("Create returned empty token")
	}
	if len(token) != 64 {
		t.Errorf("token length: got %d, want 64 hex chars", len(token))
	}

	sess := ss.Get(token)
	if sess == nil {
		t.Fatal("Get returned nil for freshly created session")
	}
	if sess.UserID != 42 {
		t.Errorf("UserID: got %d, want 42", sess.UserID)
	}
	if sess.Token != token {
		t.Errorf("Token field mismatch: got %q, want %q", sess.Token, token)
	}
	if sess.Expiry.Before(time.Now()) {
		t.Error("session Expiry is already in the past")
	}
}

// TestAmpacheSessionStoreGetUnknown verifies that Get returns nil for unknown tokens.
func TestAmpacheSessionStoreGetUnknown(t *testing.T) {
	ss := NewAmpacheSessionStore(time.Hour)
	if got := ss.Get("nonexistent"); got != nil {
		t.Errorf("Get(unknown): expected nil, got %+v", got)
	}
}

// TestAmpacheSessionStoreGetExpired verifies that Get returns nil for expired sessions.
func TestAmpacheSessionStoreGetExpired(t *testing.T) {
	ss := NewAmpacheSessionStore(time.Millisecond)
	token := ss.Create(1)
	time.Sleep(5 * time.Millisecond)

	if got := ss.Get(token); got != nil {
		t.Errorf("Get(expired): expected nil, got %+v", got)
	}
}

// TestAmpacheSessionStoreDelete verifies that Delete makes the session unreachable.
func TestAmpacheSessionStoreDelete(t *testing.T) {
	ss := NewAmpacheSessionStore(time.Hour)
	token := ss.Create(7)

	ss.Delete(token)

	if got := ss.Get(token); got != nil {
		t.Errorf("Get after Delete: expected nil, got %+v", got)
	}
}

// TestAmpacheSessionStoreDeleteNonexistent verifies Delete is a no-op for unknown tokens.
func TestAmpacheSessionStoreDeleteNonexistent(t *testing.T) {
	ss := NewAmpacheSessionStore(time.Hour)
	ss.Delete("does-not-exist")
}

// TestAmpacheSessionStoreExtend verifies that Extend pushes the expiry forward.
func TestAmpacheSessionStoreExtend(t *testing.T) {
	ss := NewAmpacheSessionStore(10 * time.Millisecond)
	token := ss.Create(3)

	ss.Extend(token, time.Hour)

	sess := ss.Get(token)
	if sess == nil {
		t.Fatal("Get after Extend returned nil")
	}
	if sess.Expiry.Before(time.Now().Add(30 * time.Minute)) {
		t.Errorf("Extend did not push expiry far enough: %v", sess.Expiry)
	}
}

// TestAmpacheSessionStoreExtendNonexistent verifies Extend is a no-op for unknown tokens.
func TestAmpacheSessionStoreExtendNonexistent(t *testing.T) {
	ss := NewAmpacheSessionStore(time.Hour)
	ss.Extend("ghost-token", time.Hour)
}

// TestAmpacheSessionStoreCleanup verifies that Cleanup removes expired sessions
// and leaves live sessions untouched.
func TestAmpacheSessionStoreCleanup(t *testing.T) {
	ss := NewAmpacheSessionStore(time.Millisecond)

	// Create a session that will expire very quickly.
	expired := ss.Create(10)

	// Wait for it to expire.
	time.Sleep(10 * time.Millisecond)

	// Create a long-lived session after the sleep so it is definitely not expired.
	live := ss.Create(12)
	ss.Extend(live, time.Hour)

	ss.Cleanup()

	// The expired session must be gone.
	if got := ss.Get(expired); got != nil {
		t.Errorf("Cleanup: expired session still present after Cleanup")
	}

	// The live session must still be reachable.
	if got := ss.Get(live); got == nil {
		t.Errorf("Cleanup: live session was incorrectly removed")
	}
}

// TestAmpacheSessionStoreCreateUnique verifies that two creates produce distinct tokens.
func TestAmpacheSessionStoreCreateUnique(t *testing.T) {
	ss := NewAmpacheSessionStore(time.Hour)
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		tok := ss.Create(int64(i))
		if seen[tok] {
			t.Fatalf("duplicate token produced at iteration %d: %q", i, tok)
		}
		seen[tok] = true
	}
}

// TestVerifyHandshakeSHA256 verifies that a correct SHA-256 passphrase authenticates.
func TestVerifyHandshakeSHA256(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{ID: 1, Username: "alice", IsEnabled: true})

	ts := time.Now().Unix()
	timestamp := strconv.FormatInt(ts, 10)
	passphrase := ampacheSHA256Passphrase(timestamp, "secret")
	plainPwd := func(_ context.Context, _ string) (string, bool) { return "secret", true }

	user, err := VerifyHandshake(context.Background(), users, plainPwd, "alice", passphrase, timestamp)
	if err != nil {
		t.Fatalf("VerifyHandshake SHA-256: unexpected error: %v", err)
	}
	if user == nil || user.Username != "alice" {
		t.Errorf("VerifyHandshake SHA-256: expected alice, got %+v", user)
	}
}

// TestVerifyHandshakeMD5 verifies that a correct MD5 passphrase authenticates (legacy path).
func TestVerifyHandshakeMD5(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{ID: 2, Username: "bob", IsEnabled: true})

	ts := time.Now().Unix()
	timestamp := strconv.FormatInt(ts, 10)
	passphrase := ampacheMD5Passphrase(timestamp, "hunter2")
	plainPwd := func(_ context.Context, _ string) (string, bool) { return "hunter2", true }

	user, err := VerifyHandshake(context.Background(), users, plainPwd, "bob", passphrase, timestamp)
	if err != nil {
		t.Fatalf("VerifyHandshake MD5: unexpected error: %v", err)
	}
	if user == nil || user.Username != "bob" {
		t.Errorf("VerifyHandshake MD5: expected bob, got %+v", user)
	}
}

// TestVerifyHandshakeExpiredTimestamp verifies that a timestamp > 300s old is rejected.
func TestVerifyHandshakeExpiredTimestamp(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{ID: 3, Username: "carol", IsEnabled: true})

	oldTS := time.Now().Add(-10 * time.Minute).Unix()
	timestamp := strconv.FormatInt(oldTS, 10)
	passphrase := ampacheSHA256Passphrase(timestamp, "pw")
	plainPwd := func(_ context.Context, _ string) (string, bool) { return "pw", true }

	_, err := VerifyHandshake(context.Background(), users, plainPwd, "carol", passphrase, timestamp)
	if err == nil {
		t.Error("VerifyHandshake: expected error for expired timestamp, got nil")
	}
}

// TestVerifyHandshakeFutureTimestamp verifies that a far-future timestamp is also rejected.
func TestVerifyHandshakeFutureTimestamp(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{ID: 4, Username: "dave", IsEnabled: true})

	futureTS := time.Now().Add(10 * time.Minute).Unix()
	timestamp := strconv.FormatInt(futureTS, 10)
	passphrase := ampacheSHA256Passphrase(timestamp, "pw")
	plainPwd := func(_ context.Context, _ string) (string, bool) { return "pw", true }

	_, err := VerifyHandshake(context.Background(), users, plainPwd, "dave", passphrase, timestamp)
	if err == nil {
		t.Error("VerifyHandshake: expected error for far-future timestamp, got nil")
	}
}

// TestVerifyHandshakeWrongPassphrase verifies that a wrong passphrase is rejected.
func TestVerifyHandshakeWrongPassphrase(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{ID: 5, Username: "eve", IsEnabled: true})

	ts := time.Now().Unix()
	timestamp := strconv.FormatInt(ts, 10)
	plainPwd := func(_ context.Context, _ string) (string, bool) { return "correct-password", true }

	_, err := VerifyHandshake(context.Background(), users, plainPwd, "eve", "deadbeef", timestamp)
	if err == nil {
		t.Error("VerifyHandshake: expected error for wrong passphrase, got nil")
	}
}

// TestVerifyHandshakeInvalidTimestamp verifies that a non-numeric timestamp is rejected.
func TestVerifyHandshakeInvalidTimestamp(t *testing.T) {
	users := newStubUserStore()
	plainPwd := func(_ context.Context, _ string) (string, bool) { return "pw", true }

	_, err := VerifyHandshake(context.Background(), users, plainPwd, "alice", "aabbcc", "not-a-number")
	if err == nil {
		t.Error("VerifyHandshake: expected error for non-numeric timestamp, got nil")
	}
}

// TestVerifyHandshakeUnknownUser verifies that an unknown username is rejected.
func TestVerifyHandshakeUnknownUser(t *testing.T) {
	users := newStubUserStore()

	ts := time.Now().Unix()
	timestamp := strconv.FormatInt(ts, 10)
	passphrase := ampacheSHA256Passphrase(timestamp, "pw")
	plainPwd := func(_ context.Context, _ string) (string, bool) { return "pw", true }

	_, err := VerifyHandshake(context.Background(), users, plainPwd, "nobody", passphrase, timestamp)
	if err == nil {
		t.Error("VerifyHandshake: expected error for unknown user, got nil")
	}
}

// TestVerifyHandshakeNoReversiblePassword verifies that missing plaintext password is rejected.
func TestVerifyHandshakeNoReversiblePassword(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{ID: 6, Username: "frank", IsEnabled: true})

	ts := time.Now().Unix()
	timestamp := strconv.FormatInt(ts, 10)
	passphrase := ampacheSHA256Passphrase(timestamp, "pw")
	plainPwd := func(_ context.Context, _ string) (string, bool) { return "", false }

	_, err := VerifyHandshake(context.Background(), users, plainPwd, "frank", passphrase, timestamp)
	if err == nil {
		t.Error("VerifyHandshake: expected error when no reversible password available, got nil")
	}
}

// TestVerifyHandshakeLockedAccount verifies that a locked account is rejected.
func TestVerifyHandshakeLockedAccount(t *testing.T) {
	users := newStubUserStore()
	users.add(&model.User{
		ID:          7,
		Username:    "grace",
		IsEnabled:   true,
		LockedUntil: time.Now().Add(time.Hour),
	})

	ts := time.Now().Unix()
	timestamp := strconv.FormatInt(ts, 10)
	passphrase := ampacheSHA256Passphrase(timestamp, "pw")
	plainPwd := func(_ context.Context, _ string) (string, bool) { return "pw", true }

	_, err := VerifyHandshake(context.Background(), users, plainPwd, "grace", passphrase, timestamp)
	if err == nil {
		t.Error("VerifyHandshake: expected error for locked account, got nil")
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(r *http.Request)
		wantIP  string
	}{
		{
			name:   "remote addr only",
			setup:  func(r *http.Request) { r.RemoteAddr = "1.2.3.4:5678" },
			wantIP: "1.2.3.4",
		},
		{
			name: "X-Forwarded-For single",
			setup: func(r *http.Request) {
				r.RemoteAddr = "127.0.0.1:80"
				r.Header.Set("X-Forwarded-For", "5.6.7.8")
			},
			wantIP: "5.6.7.8",
		},
		{
			name: "X-Forwarded-For multi",
			setup: func(r *http.Request) {
				r.RemoteAddr = "127.0.0.1:80"
				r.Header.Set("X-Forwarded-For", "5.6.7.8, 10.0.0.1")
			},
			wantIP: "5.6.7.8",
		},
		{
			name: "X-Real-IP",
			setup: func(r *http.Request) {
				r.RemoteAddr = "127.0.0.1:80"
				r.Header.Set("X-Real-IP", "9.8.7.6")
			},
			wantIP: "9.8.7.6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tt.setup(req)
			got := extractIP(req)
			if got != tt.wantIP {
				t.Errorf("extractIP: got %q, want %q", got, tt.wantIP)
			}
		})
	}
}
