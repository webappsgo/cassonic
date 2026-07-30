package ampache

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// TestDispatch_InvalidAction covers the default branch of the action switch.
func TestDispatch_InvalidAction(t *testing.T) {
	h, _ := newTestHandler()
	rec := httptest.NewRecorder()
	h.dispatch(rec, newRequest("bogus_action", nil), true)
	var got xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.ErrorCode != 4701 {
		t.Fatalf("expected 4701 for invalid action, got %+v", got)
	}
}

// TestDispatch_KnownAction spot-checks that dispatch correctly routes at least
// one action per category to its handler (full per-handler coverage lives in
// the other _test.go files; this only proves the switch wiring itself).
func TestDispatch_KnownAction(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.music.artists = []*model.Artist{{ID: 1, Name: "A"}}
	rec := httptest.NewRecorder()
	h.dispatch(rec, newRequest("artists", map[string]string{"auth": token}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	if _, ok := got["artist"]; !ok {
		t.Fatalf("expected dispatch to route to artists handler, got %v", got)
	}
}

// TestDispatch_ActionFromFormValue covers the fallback to r.FormValue when the
// action isn't present in the query string (POST body case).
func TestDispatch_ActionFromFormValue(t *testing.T) {
	h, _ := newTestHandler()
	req := httptest.NewRequest("POST", "/server/json.server.php", nil)
	req.PostForm = map[string][]string{"action": {"bogus"}}
	rec := httptest.NewRecorder()
	h.dispatch(rec, req, true)
	var got xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.ErrorCode != 4701 {
		t.Fatalf("expected 4701, got %+v", got)
	}
}

// TestParam covers the query-then-form fallback helper.
func TestParam(t *testing.T) {
	req := newRequest("artists", map[string]string{"filter": "q"})
	if got := param(req, "filter"); got != "q" {
		t.Fatalf("expected q, got %q", got)
	}
	if got := param(req, "missing"); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

// TestRequireSession covers all three states: missing token, invalid/expired
// token, and a valid token returning the session.
func TestRequireSession(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		h, _ := newTestHandler()
		rec := httptest.NewRecorder()
		session := h.requireSession(rec, newRequest("ping", nil), true)
		if session != nil {
			t.Fatal("expected nil session")
		}
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4700 {
			t.Fatalf("expected 4700, got %+v", got)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		h, _ := newTestHandler()
		rec := httptest.NewRecorder()
		session := h.requireSession(rec, newRequest("ping", map[string]string{"auth": "bogus"}), true)
		if session != nil {
			t.Fatal("expected nil session")
		}
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4700 {
			t.Fatalf("expected 4700, got %+v", got)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		h, _, token := newAuthedHandler(5)
		rec := httptest.NewRecorder()
		session := h.requireSession(rec, newRequest("ping", map[string]string{"auth": token}), true)
		if session == nil || session.UserID != 5 {
			t.Fatalf("expected session for user 5, got %+v", session)
		}
	})
}

// TestRequireAdmin covers: no session, session but store error, non-admin
// user, and admin user.
func TestRequireAdmin(t *testing.T) {
	t.Run("no session", func(t *testing.T) {
		h, _ := newTestHandler()
		rec := httptest.NewRecorder()
		session := h.requireAdmin(rec, newRequest("system_update", nil), true)
		if session != nil {
			t.Fatal("expected nil session")
		}
	})

	t.Run("user lookup error", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.userErr = errors.New("db down")
		rec := httptest.NewRecorder()
		session := h.requireAdmin(rec, newRequest("system_update", map[string]string{"auth": token}), true)
		if session != nil {
			t.Fatal("expected nil session")
		}
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4742 {
			t.Fatalf("expected 4742, got %+v", got)
		}
	})

	t.Run("non-admin user", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, IsAdmin: false}
		rec := httptest.NewRecorder()
		session := h.requireAdmin(rec, newRequest("system_update", map[string]string{"auth": token}), true)
		if session != nil {
			t.Fatal("expected nil session for non-admin")
		}
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4742 {
			t.Fatalf("expected 4742, got %+v", got)
		}
	})

	t.Run("admin user", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, IsAdmin: true}
		rec := httptest.NewRecorder()
		session := h.requireAdmin(rec, newRequest("system_update", map[string]string{"auth": token}), true)
		if session == nil {
			t.Fatal("expected non-nil session for admin")
		}
	})
}
