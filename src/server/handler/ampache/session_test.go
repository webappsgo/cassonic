package ampache

import (
	"context"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
)

// TestHandshake_MissingParams covers the 4705 error path for each required
// parameter (user, timestamp, auth) being absent.
func TestHandshake_MissingParams(t *testing.T) {
	cases := []map[string]string{
		{"timestamp": "1", "auth": "a"},
		{"user": "bob", "auth": "a"},
		{"user": "bob", "timestamp": "1"},
		{},
	}
	for _, params := range cases {
		h, _ := newTestHandler()
		rec := httptest.NewRecorder()
		req := newRequest("handshake", params)
		h.handshake(rec, req, true)

		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4705 {
			t.Errorf("params=%v: expected 4705, got %d (%s)", params, got.ErrorCode, got.ErrorMessage)
		}
	}
}

// TestHandshake_SHA256Success exercises the full happy path: a client computes
// HASH(timestamp + HASH(password)) with SHA-256, and the handler must create a
// session and return library counts.
func TestHandshake_SHA256Success(t *testing.T) {
	h, ts := newTestHandler()
	ts.users.userByUsername = &model.User{ID: 1, Username: "alice", IsEnabled: true}
	h.getPlainPassword = func(ctx context.Context, username string) (string, bool) {
		return "secret", true
	}
	ts.music.artists = []*model.Artist{{ID: 1}, {ID: 2}}
	ts.music.albums = []*model.Album{{ID: 1}}
	ts.music.genres = []*model.Genre{{ID: 1, SongCount: 5}}
	ts.playlists.playlists = []*model.Playlist{{ID: 1}}

	ts_ := nowTimestamp()
	pass := sha256Passphrase("secret", ts_)

	rec := httptest.NewRecorder()
	req := newRequest("handshake", map[string]string{"user": "alice", "timestamp": ts_, "auth": pass})
	h.handshake(rec, req, true)

	var resp HandshakeResp
	decodeJSON(t, rec.Body.Bytes(), &resp)
	if resp.Auth == "" {
		t.Fatal("expected a non-empty session token")
	}
	if resp.API != ampacheAPIVersion {
		t.Fatalf("unexpected API version: %s", resp.API)
	}
	if resp.Artists != 2 || resp.Albums != 1 || resp.Playlists != 1 || resp.Songs != 5 {
		t.Fatalf("unexpected library counts: %+v", resp)
	}
	if h.sessions.Get(resp.Auth) == nil {
		t.Fatal("expected a session to be created for the returned token")
	}
}

// TestHandshake_MD5LegacySuccess covers the legacy MD5 handshake fallback,
// required for compatibility with older Ampache clients.
func TestHandshake_MD5LegacySuccess(t *testing.T) {
	h, ts := newTestHandler()
	ts.users.userByUsername = &model.User{ID: 1, Username: "alice", IsEnabled: true}
	h.getPlainPassword = func(ctx context.Context, username string) (string, bool) {
		return "secret", true
	}

	ts_ := nowTimestamp()
	pass := md5Passphrase("secret", ts_)

	rec := httptest.NewRecorder()
	req := newRequest("handshake", map[string]string{"user": "alice", "timestamp": ts_, "auth": pass})
	h.handshake(rec, req, true)

	var resp HandshakeResp
	decodeJSON(t, rec.Body.Bytes(), &resp)
	if resp.Auth == "" {
		t.Fatal("expected a non-empty session token for valid MD5 legacy passphrase")
	}
}

// TestHandshake_WrongPassword covers the 4700 error path when the passphrase
// does not match either SHA-256 or MD5 derivations of the real password.
func TestHandshake_WrongPassword(t *testing.T) {
	h, ts := newTestHandler()
	ts.users.userByUsername = &model.User{ID: 1, Username: "alice", IsEnabled: true}
	h.getPlainPassword = func(ctx context.Context, username string) (string, bool) {
		return "secret", true
	}

	rec := httptest.NewRecorder()
	req := newRequest("handshake", map[string]string{"user": "alice", "timestamp": nowTimestamp(), "auth": "not-a-valid-hash"})
	h.handshake(rec, req, true)

	var got xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.ErrorCode != 4700 {
		t.Fatalf("expected 4700, got %d (%s)", got.ErrorCode, got.ErrorMessage)
	}
}

// TestHandshake_UserNotFound covers the 4700 error path when the username
// does not resolve to any user.
func TestHandshake_UserNotFound(t *testing.T) {
	h, _ := newTestHandler()
	rec := httptest.NewRecorder()
	req := newRequest("handshake", map[string]string{"user": "ghost", "timestamp": nowTimestamp(), "auth": "x"})
	h.handshake(rec, req, true)

	var got xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.ErrorCode != 4700 {
		t.Fatalf("expected 4700 for unknown user, got %d", got.ErrorCode)
	}
}

// TestHandshake_ExpiredTimestamp covers the timestamp-drift boundary: a
// timestamp older than ampacheHandshakeMaxAge (300s) must be rejected even
// with an otherwise-correct passphrase.
func TestHandshake_ExpiredTimestamp(t *testing.T) {
	h, ts := newTestHandler()
	ts.users.userByUsername = &model.User{ID: 1, Username: "alice", IsEnabled: true}
	h.getPlainPassword = func(ctx context.Context, username string) (string, bool) {
		return "secret", true
	}

	old := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	pass := sha256Passphrase("secret", old)

	rec := httptest.NewRecorder()
	req := newRequest("handshake", map[string]string{"user": "alice", "timestamp": old, "auth": pass})
	h.handshake(rec, req, true)

	var got xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.ErrorCode != 4700 {
		t.Fatalf("expected 4700 for expired timestamp, got %d (%s)", got.ErrorCode, got.ErrorMessage)
	}
}

// TestHandshake_DisabledAccount covers accounts that are disabled or locked.
func TestHandshake_DisabledAccount(t *testing.T) {
	h, ts := newTestHandler()
	ts.users.userByUsername = &model.User{ID: 1, Username: "alice", IsEnabled: false}
	h.getPlainPassword = func(ctx context.Context, username string) (string, bool) {
		return "secret", true
	}
	tsNow := nowTimestamp()
	pass := sha256Passphrase("secret", tsNow)

	rec := httptest.NewRecorder()
	req := newRequest("handshake", map[string]string{"user": "alice", "timestamp": tsNow, "auth": pass})
	h.handshake(rec, req, true)

	var got xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.ErrorCode != 4700 {
		t.Fatalf("expected 4700 for disabled account, got %d", got.ErrorCode)
	}
}

// TestGoodbye covers the missing-auth error path and the success path,
// including invalidating the token so a subsequent lookup fails.
func TestGoodbye(t *testing.T) {
	t.Run("missing auth", func(t *testing.T) {
		h, _ := newTestHandler()
		rec := httptest.NewRecorder()
		req := newRequest("goodbye", nil)
		h.goodbye(rec, req, true)

		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4705 {
			t.Fatalf("expected 4705, got %d", got.ErrorCode)
		}
	})

	t.Run("success invalidates session", func(t *testing.T) {
		h, ts, _ := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, Username: "alice"}
		token := h.sessions.Create(1)

		rec := httptest.NewRecorder()
		req := newRequest("goodbye", map[string]string{"auth": token})
		h.goodbye(rec, req, true)

		if rec.Code != 200 {
			t.Fatalf("expected HTTP 200, got %d", rec.Code)
		}
		if h.sessions.Get(token) != nil {
			t.Fatal("expected session to be deleted after goodbye")
		}
	})

	t.Run("unknown token still returns ok with empty username", func(t *testing.T) {
		h, _ := newTestHandler()
		rec := httptest.NewRecorder()
		req := newRequest("goodbye", map[string]string{"auth": "bogus-token"})
		h.goodbye(rec, req, true)

		if rec.Code != 200 {
			t.Fatalf("expected HTTP 200, got %d", rec.Code)
		}
	})
}

// TestPing covers all three branches: no token, invalid token, and a valid
// token (which must extend the session TTL).
func TestPing(t *testing.T) {
	t.Run("no token returns bare version", func(t *testing.T) {
		h, _ := newTestHandler()
		rec := httptest.NewRecorder()
		req := newRequest("ping", nil)
		h.ping(rec, req, true)

		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got["version"] != ampacheAPIVersion {
			t.Fatalf("unexpected ping body: %v", got)
		}
		if _, hasExpire := got["session_expire"]; hasExpire {
			t.Fatal("session_expire should not be present without a token")
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		h, _ := newTestHandler()
		rec := httptest.NewRecorder()
		req := newRequest("ping", map[string]string{"auth": "bogus"})
		h.ping(rec, req, true)

		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4700 {
			t.Fatalf("expected 4700 for invalid token, got %d", got.ErrorCode)
		}
	})

	t.Run("valid token extends session", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		req := newRequest("ping", map[string]string{"auth": token})
		h.ping(rec, req, true)

		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got["session_expire"] == nil || got["session_expire"] == "" {
			t.Fatalf("expected session_expire to be set, got %v", got)
		}
	})
}

// TestCheckParameter covers missing session, missing parameter, and success.
func TestCheckParameter(t *testing.T) {
	t.Run("missing session", func(t *testing.T) {
		h, _ := newTestHandler()
		rec := httptest.NewRecorder()
		req := newRequest("check_parameter", map[string]string{"parameter": "x"})
		h.checkParameter(rec, req, true)

		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4700 {
			t.Fatalf("expected 4700, got %d", got.ErrorCode)
		}
	})

	t.Run("missing parameter", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		req := newRequest("check_parameter", map[string]string{"auth": token})
		h.checkParameter(rec, req, true)

		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4705 {
			t.Fatalf("expected 4705, got %d", got.ErrorCode)
		}
	})

	t.Run("success echoes value", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		req := newRequest("check_parameter", map[string]string{"auth": token, "parameter": "limit", "input": "50"})
		h.checkParameter(rec, req, true)

		var got map[string]any
		decodeJSON(t, rec.Body.Bytes(), &got)
		wrapped, ok := got["parameter"].(map[string]any)
		if !ok {
			t.Fatalf("unexpected body: %v", got)
		}
		if wrapped["name"] != "limit" || wrapped["value"] != "50" {
			t.Fatalf("unexpected echoed parameter: %v", wrapped)
		}
	})
}

// TestParseIntParam covers valid, missing, and unparsable values.
func TestParseIntParam(t *testing.T) {
	req := newRequest("noop", map[string]string{"limit": "25", "bad": "notanumber"})
	if got := parseIntParam(req, "limit", 10); got != 25 {
		t.Errorf("parseIntParam(limit) = %d, want 25", got)
	}
	if got := parseIntParam(req, "missing", 10); got != 10 {
		t.Errorf("parseIntParam(missing) = %d, want default 10", got)
	}
	if got := parseIntParam(req, "bad", 10); got != 10 {
		t.Errorf("parseIntParam(bad) = %d, want default 10 on parse failure", got)
	}
}

// TestParseIDParam covers valid, missing, and unparsable values.
func TestParseIDParam(t *testing.T) {
	req := newRequest("noop", map[string]string{"id": "42", "bad": "nope"})
	if got := parseIDParam(req, "id"); got != 42 {
		t.Errorf("parseIDParam(id) = %d, want 42", got)
	}
	if got := parseIDParam(req, "missing"); got != 0 {
		t.Errorf("parseIDParam(missing) = %d, want 0", got)
	}
	if got := parseIDParam(req, "bad"); got != 0 {
		t.Errorf("parseIDParam(bad) = %d, want 0 on parse failure", got)
	}
}

// TestVerifyHandshake_NoReversiblePassword is a light sanity check that the
// package correctly propagates a "no reversible password" failure from
// middleware.VerifyHandshake as a 4700 handshake error.
func TestVerifyHandshake_NoReversiblePassword(t *testing.T) {
	h, ts := newTestHandler()
	ts.users.userByUsername = &model.User{ID: 1, Username: "alice", IsEnabled: true}
	h.getPlainPassword = func(ctx context.Context, username string) (string, bool) {
		return "", false
	}
	tsNow := nowTimestamp()
	pass := sha256Passphrase("secret", tsNow)

	rec := httptest.NewRecorder()
	req := newRequest("handshake", map[string]string{"user": "alice", "timestamp": tsNow, "auth": pass})
	h.handshake(rec, req, true)

	var got xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.ErrorCode != 4700 {
		t.Fatalf("expected 4700, got %d", got.ErrorCode)
	}
}
