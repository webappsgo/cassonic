package ampache

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// TestUser covers missing username param, self-lookup allowed for non-admins,
// admin-lookup-of-others allowed, non-admin-lookup-of-others denied, and not found.
func TestUser(t *testing.T) {
	t.Run("missing username", func(t *testing.T) {
		h, _, token := newAuthedHandler(1)
		rec := httptest.NewRecorder()
		h.user(rec, newRequest("user", map[string]string{"auth": token}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4705 {
			t.Fatalf("expected 4705, got %+v", got)
		}
	})

	t.Run("self lookup allowed", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, Username: "alice"}
		ts.users.userByUsername = &model.User{ID: 1, Username: "alice"}
		rec := httptest.NewRecorder()
		h.user(rec, newRequest("user", map[string]string{"auth": token, "username": "alice"}), true)
		var got AmpUser
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.Username != "alice" {
			t.Fatalf("unexpected user: %+v", got)
		}
	})

	t.Run("non-admin lookup of other denied", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, Username: "alice", IsAdmin: false}
		rec := httptest.NewRecorder()
		h.user(rec, newRequest("user", map[string]string{"auth": token, "username": "bob"}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4742 {
			t.Fatalf("expected 4742, got %+v", got)
		}
	})

	t.Run("admin lookup of other allowed", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, Username: "alice", IsAdmin: true}
		ts.users.userByUsername = &model.User{ID: 2, Username: "bob"}
		rec := httptest.NewRecorder()
		h.user(rec, newRequest("user", map[string]string{"auth": token, "username": "bob"}), true)
		var got AmpUser
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.Username != "bob" {
			t.Fatalf("unexpected user: %+v", got)
		}
	})

	t.Run("target not found", func(t *testing.T) {
		h, ts, token := newAuthedHandler(1)
		ts.users.user = &model.User{ID: 1, Username: "alice", IsAdmin: true}
		rec := httptest.NewRecorder()
		h.user(rec, newRequest("user", map[string]string{"auth": token, "username": "ghost"}), true)
		var got xmlErrorJSON
		decodeJSON(t, rec.Body.Bytes(), &got)
		if got.ErrorCode != 4704 {
			t.Fatalf("expected 4704, got %+v", got)
		}
	})
}

// TestUsers covers admin-only listing and the store-error path.
func TestUsers(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.users.user = &model.User{ID: 1, IsAdmin: true}
	ts.users.users = []*model.User{{ID: 1, Username: "alice"}, {ID: 2, Username: "bob"}}
	rec := httptest.NewRecorder()
	h.users(rec, newRequest("users", map[string]string{"auth": token}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	list, ok := got["user"].([]any)
	if !ok || len(list) != 2 {
		t.Fatalf("expected 2 users, got %v", got)
	}

	ts.users.usersErr = errors.New("x")
	rec2 := httptest.NewRecorder()
	h.users(rec2, newRequest("users", map[string]string{"auth": token}), true)
	var errGot xmlErrorJSON
	decodeJSON(t, rec2.Body.Bytes(), &errGot)
	if errGot.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", errGot)
	}
}

// TestUserCreate covers admin gating, missing params, and success including
// the group=0 admin-flag and disable=1 mappings.
func TestUserCreate(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.users.user = &model.User{ID: 1, IsAdmin: true}

	rec := httptest.NewRecorder()
	h.userCreate(rec, newRequest("user_create", map[string]string{"auth": token}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	ts.users.createUserID = 7
	rec2 := httptest.NewRecorder()
	h.userCreate(rec2, newRequest("user_create", map[string]string{
		"auth": token, "username": "carol", "password": "pw", "email": "c@x.com", "group": "0", "disable": "1",
	}), true)
	var got AmpUser
	decodeJSON(t, rec2.Body.Bytes(), &got)
	if got.ID != "7" || got.Username != "carol" || got.Access != 100 || got.Disabled != 1 {
		t.Fatalf("unexpected created user: %+v", got)
	}

	ts.users.createUserErr = errors.New("x")
	rec3 := httptest.NewRecorder()
	h.userCreate(rec3, newRequest("user_create", map[string]string{
		"auth": token, "username": "dave", "password": "pw",
	}), true)
	var errGot xmlErrorJSON
	decodeJSON(t, rec3.Body.Bytes(), &errGot)
	if errGot.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", errGot)
	}
}

// TestUserEdit covers missing username, not found, and partial field update
// including the group/disable sentinel-value handling (-1 means "unset").
func TestUserEdit(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.users.user = &model.User{ID: 1, IsAdmin: true}

	rec := httptest.NewRecorder()
	h.userEdit(rec, newRequest("user_edit", map[string]string{"auth": token}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	rec2 := httptest.NewRecorder()
	h.userEdit(rec2, newRequest("user_edit", map[string]string{"auth": token, "username": "ghost"}), true)
	var notFound xmlErrorJSON
	decodeJSON(t, rec2.Body.Bytes(), &notFound)
	if notFound.ErrorCode != 4704 {
		t.Fatalf("expected 4704, got %+v", notFound)
	}

	ts.users.userByUsername = &model.User{ID: 2, Username: "bob", Email: "old@x.com", IsAdmin: false, IsEnabled: true}
	rec3 := httptest.NewRecorder()
	h.userEdit(rec3, newRequest("user_edit", map[string]string{
		"auth": token, "username": "bob", "email": "new@x.com", "group": "0", "disable": "1",
	}), true)
	var got AmpUser
	decodeJSON(t, rec3.Body.Bytes(), &got)
	if got.Email != "new@x.com" || got.Access != 100 || got.Disabled != 1 {
		t.Fatalf("unexpected edited user: %+v", got)
	}

	ts.users.updateUserErr = errors.New("x")
	rec4 := httptest.NewRecorder()
	h.userEdit(rec4, newRequest("user_edit", map[string]string{"auth": token, "username": "bob"}), true)
	var errGot xmlErrorJSON
	decodeJSON(t, rec4.Body.Bytes(), &errGot)
	if errGot.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", errGot)
	}
}

// TestUserDelete covers missing username, not found, success, and store error.
func TestUserDelete(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.users.user = &model.User{ID: 1, IsAdmin: true}

	rec := httptest.NewRecorder()
	h.userDelete(rec, newRequest("user_delete", map[string]string{"auth": token}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	rec2 := httptest.NewRecorder()
	h.userDelete(rec2, newRequest("user_delete", map[string]string{"auth": token, "username": "ghost"}), true)
	var notFound xmlErrorJSON
	decodeJSON(t, rec2.Body.Bytes(), &notFound)
	if notFound.ErrorCode != 4704 {
		t.Fatalf("expected 4704, got %+v", notFound)
	}

	ts.users.userByUsername = &model.User{ID: 2, Username: "bob"}
	rec3 := httptest.NewRecorder()
	h.userDelete(rec3, newRequest("user_delete", map[string]string{"auth": token, "username": "bob"}), true)
	var got map[string]any
	decodeJSON(t, rec3.Body.Bytes(), &got)
	if got["success"] == nil {
		t.Fatalf("expected success key, got %v", got)
	}

	ts.users.deleteUserErr = errors.New("x")
	rec4 := httptest.NewRecorder()
	h.userDelete(rec4, newRequest("user_delete", map[string]string{"auth": token, "username": "bob"}), true)
	var errGot xmlErrorJSON
	decodeJSON(t, rec4.Body.Bytes(), &errGot)
	if errGot.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", errGot)
	}
}

// TestUserPreferences covers listing and the not-found path.
func TestUserPreferences(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.users.user = &model.User{ID: 1, Theme: "dark", Language: "en"}
	rec := httptest.NewRecorder()
	h.userPreferences(rec, newRequest("user_preferences", map[string]string{"auth": token}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	list, ok := got["preference"].([]any)
	if !ok || len(list) == 0 {
		t.Fatalf("expected non-empty preference list, got %v", got)
	}

	h2, _, token2 := newAuthedHandler(1)
	rec2 := httptest.NewRecorder()
	h2.userPreferences(rec2, newRequest("user_preferences", map[string]string{"auth": token2}), true)
	var notFound xmlErrorJSON
	decodeJSON(t, rec2.Body.Bytes(), &notFound)
	if notFound.ErrorCode != 4704 {
		t.Fatalf("expected 4704 when user store empty, got %+v", notFound)
	}
}

// TestUserPreference covers missing filter, found, and not found by name.
func TestUserPreference(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.users.user = &model.User{ID: 1, Theme: "dark", Language: "en"}

	rec := httptest.NewRecorder()
	h.userPreference(rec, newRequest("user_preference", map[string]string{"auth": token}), true)
	var missing xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &missing)
	if missing.ErrorCode != 4705 {
		t.Fatalf("expected 4705, got %+v", missing)
	}

	rec2 := httptest.NewRecorder()
	h.userPreference(rec2, newRequest("user_preference", map[string]string{"auth": token, "filter": "lang"}), true)
	var got AmpPreference
	decodeJSON(t, rec2.Body.Bytes(), &got)
	if got.Name != "lang" || got.Value != "en" {
		t.Fatalf("unexpected preference: %+v", got)
	}

	rec3 := httptest.NewRecorder()
	h.userPreference(rec3, newRequest("user_preference", map[string]string{"auth": token, "filter": "nope"}), true)
	var notFound xmlErrorJSON
	decodeJSON(t, rec3.Body.Bytes(), &notFound)
	if notFound.ErrorCode != 4704 {
		t.Fatalf("expected 4704, got %+v", notFound)
	}
}

// TestSystemPreferences covers admin-only listing.
func TestSystemPreferences(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.users.user = &model.User{ID: 1, IsAdmin: true}
	rec := httptest.NewRecorder()
	h.systemPreferences(rec, newRequest("system_preferences", map[string]string{"auth": token}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	list, ok := got["preference"].([]any)
	if !ok || len(list) == 0 {
		t.Fatalf("expected non-empty preference list, got %v", got)
	}
}

// TestSystemPreference covers the fixed not-found response.
func TestSystemPreference(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.users.user = &model.User{ID: 1, IsAdmin: true}
	rec := httptest.NewRecorder()
	h.systemPreference(rec, newRequest("system_preference", map[string]string{"auth": token}), true)
	var got xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.ErrorCode != 4704 {
		t.Fatalf("expected 4704, got %+v", got)
	}
}

// TestPreferenceReadOnlyEndpoints covers create/edit/delete all returning the
// fixed read-only error, admin-gated.
func TestPreferenceReadOnlyEndpoints(t *testing.T) {
	h, ts, token := newAuthedHandler(1)
	ts.users.user = &model.User{ID: 1, IsAdmin: true}

	rec := httptest.NewRecorder()
	h.preferenceCreate(rec, newRequest("preference_create", map[string]string{"auth": token}), true)
	var got xmlErrorJSON
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", got)
	}

	rec2 := httptest.NewRecorder()
	h.preferenceEdit(rec2, newRequest("preference_edit", map[string]string{"auth": token}), true)
	var got2 xmlErrorJSON
	decodeJSON(t, rec2.Body.Bytes(), &got2)
	if got2.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", got2)
	}

	rec3 := httptest.NewRecorder()
	h.preferenceDelete(rec3, newRequest("preference_delete", map[string]string{"auth": token}), true)
	var got3 xmlErrorJSON
	decodeJSON(t, rec3.Body.Bytes(), &got3)
	if got3.ErrorCode != 4710 {
		t.Fatalf("expected 4710, got %+v", got3)
	}
}

// TestSocialStubEndpoints covers toggleFollow, lastShouts, timeline,
// friendsTimeline: all are documented no-op stubs behind session auth.
func TestSocialStubEndpoints(t *testing.T) {
	h, _, token := newAuthedHandler(1)

	rec := httptest.NewRecorder()
	h.toggleFollow(rec, newRequest("toggle_follow", map[string]string{"auth": token}), true)
	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got["success"] == nil {
		t.Fatalf("expected success key, got %v", got)
	}

	rec2 := httptest.NewRecorder()
	h.lastShouts(rec2, newRequest("last_shouts", map[string]string{"auth": token}), true)
	var got2 map[string]any
	decodeJSON(t, rec2.Body.Bytes(), &got2)
	if list, ok := got2["shout"].([]any); !ok || len(list) != 0 {
		t.Fatalf("expected empty shout list, got %v", got2)
	}

	rec3 := httptest.NewRecorder()
	h.timeline(rec3, newRequest("timeline", map[string]string{"auth": token}), true)
	var got3 map[string]any
	decodeJSON(t, rec3.Body.Bytes(), &got3)
	if list, ok := got3["activity"].([]any); !ok || len(list) != 0 {
		t.Fatalf("expected empty activity list, got %v", got3)
	}

	rec4 := httptest.NewRecorder()
	h.friendsTimeline(rec4, newRequest("friends_timeline", map[string]string{"auth": token}), true)
	var got4 map[string]any
	decodeJSON(t, rec4.Body.Bytes(), &got4)
	if list, ok := got4["activity"].([]any); !ok || len(list) != 0 {
		t.Fatalf("expected empty activity list, got %v", got4)
	}
}

// TestHashPasswordArgon2id covers the format and that two hashes of the same
// password differ (random salt) while both remain well-formed.
func TestHashPasswordArgon2id(t *testing.T) {
	h1, err := hashPasswordArgon2id("secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h2, err := hashPasswordArgon2id("secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h1 == h2 {
		t.Fatal("expected distinct hashes due to random salt")
	}
	if h1[:9] != "$argon2id" {
		t.Fatalf("unexpected hash prefix: %q", h1)
	}
}
