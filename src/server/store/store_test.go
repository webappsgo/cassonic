package store

import (
	"context"
	"strings"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// newTestUserStore creates an in-memory SQLite UserStore with the full schema applied.
// Tests that call this helper operate on a private DB and cannot interfere with each other.
func newTestUserStore(t *testing.T) UserStore {
	t.Helper()
	db, err := openDB(":memory:", usersSchema)
	if err != nil {
		t.Fatalf("openDB :memory:: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &sqliteUserStore{db: db}
}

// sampleUser returns a minimal, valid model.User suitable for insertion.
func sampleUser(username, email string) *model.User {
	return &model.User{
		Username:     username,
		Email:        email,
		PasswordHash: "argon2id$...",
		DisplayName:  "Test User",
		IsEnabled:    true,
		Language:     "en",
		Theme:        "dark",
		CanDownload:  true,
		CanShare:     true,
		CanComment:   true,
		CanPodcast:   true,
	}
}

// TestCreateGetUserRoundtrip verifies that a user inserted via CreateUser can be
// retrieved by its ID with all fields intact.
func TestCreateGetUserRoundtrip(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	u := sampleUser("alice", "alice@example.com")
	id, err := s.CreateUser(ctx, u)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if id <= 0 {
		t.Fatalf("CreateUser returned non-positive ID: %d", id)
	}

	got, err := s.GetUser(ctx, id)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got == nil {
		t.Fatal("GetUser returned nil for a just-created user")
	}

	if got.Username != u.Username {
		t.Errorf("Username: got %q, want %q", got.Username, u.Username)
	}
	if got.Email != u.Email {
		t.Errorf("Email: got %q, want %q", got.Email, u.Email)
	}
	if got.PasswordHash != u.PasswordHash {
		t.Errorf("PasswordHash: got %q, want %q", got.PasswordHash, u.PasswordHash)
	}
	if got.DisplayName != u.DisplayName {
		t.Errorf("DisplayName: got %q, want %q", got.DisplayName, u.DisplayName)
	}
	if got.IsEnabled != u.IsEnabled {
		t.Errorf("IsEnabled: got %v, want %v", got.IsEnabled, u.IsEnabled)
	}
}

// TestGetUserByUsername verifies lookup by username returns the correct user.
func TestGetUserByUsername(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	u := sampleUser("bob", "bob@example.com")
	id, err := s.CreateUser(ctx, u)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := s.GetUserByUsername(ctx, "bob")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if got == nil {
		t.Fatal("GetUserByUsername returned nil")
	}
	if got.ID != id {
		t.Errorf("ID: got %d, want %d", got.ID, id)
	}
	if got.Username != "bob" {
		t.Errorf("Username: got %q, want %q", got.Username, "bob")
	}
}

// TestGetUserByUsernameMissing verifies that querying a non-existent username returns nil, nil.
func TestGetUserByUsernameMissing(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	got, err := s.GetUserByUsername(ctx, "nobody")
	if err != nil {
		t.Fatalf("GetUserByUsername (missing): unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("GetUserByUsername (missing): got non-nil user, want nil")
	}
}

// TestGetUserByEmail verifies lookup by email address.
func TestGetUserByEmail(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	u := sampleUser("carol", "carol@example.com")
	id, err := s.CreateUser(ctx, u)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := s.GetUserByEmail(ctx, "carol@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if got == nil {
		t.Fatal("GetUserByEmail returned nil")
	}
	if got.ID != id {
		t.Errorf("ID: got %d, want %d", got.ID, id)
	}
}

// TestGetUserByEmailMissing verifies that querying a non-existent email returns nil, nil.
func TestGetUserByEmailMissing(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	got, err := s.GetUserByEmail(ctx, "ghost@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail (missing): unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("GetUserByEmail (missing): got non-nil user, want nil")
	}
}

// TestUpdateUser verifies that UpdateUser persists a changed display_name.
func TestUpdateUser(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	u := sampleUser("dave", "dave@example.com")
	id, err := s.CreateUser(ctx, u)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Mutate display name and write back.
	u.ID = id
	u.DisplayName = "David Updated"
	if err := s.UpdateUser(ctx, u); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	got, err := s.GetUser(ctx, id)
	if err != nil {
		t.Fatalf("GetUser after update: %v", err)
	}
	if got == nil {
		t.Fatal("GetUser after update returned nil")
	}
	if got.DisplayName != "David Updated" {
		t.Errorf("DisplayName after update: got %q, want %q", got.DisplayName, "David Updated")
	}
}

// TestDeleteUser verifies that DeleteUser removes the user; subsequent GetUser
// returns nil without error.
func TestDeleteUser(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	u := sampleUser("eve", "eve@example.com")
	id, err := s.CreateUser(ctx, u)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := s.DeleteUser(ctx, id); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	got, err := s.GetUser(ctx, id)
	if err != nil {
		t.Fatalf("GetUser after delete: unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("GetUser after delete: got non-nil user, want nil")
	}
}

// TestListUsers verifies that ListUsers returns all inserted users ordered by username.
func TestListUsers(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	names := []string{"zara", "alice", "mike"}
	for _, n := range names {
		if _, err := s.CreateUser(ctx, sampleUser(n, n+"@example.com")); err != nil {
			t.Fatalf("CreateUser(%q): %v", n, err)
		}
	}

	users, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != len(names) {
		t.Fatalf("ListUsers: got %d users, want %d", len(users), len(names))
	}

	// Expect alphabetical order: alice, mike, zara.
	want := []string{"alice", "mike", "zara"}
	for i, u := range users {
		if u.Username != want[i] {
			t.Errorf("ListUsers[%d]: got %q, want %q", i, u.Username, want[i])
		}
	}
}

// TestListUsersEmpty verifies that ListUsers on an empty store returns a nil or
// empty slice without error.
func TestListUsersEmpty(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	users, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers (empty): %v", err)
	}
	if len(users) != 0 {
		t.Errorf("ListUsers (empty): got %d users, want 0", len(users))
	}
}

// TestGetSubsonicPasswordNotSet verifies the documented "not set" contract:
// returns ("", false, nil) when subsonic_password is blank.
func TestGetSubsonicPasswordNotSet(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	u := sampleUser("frank", "frank@example.com")
	if _, err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	enc, ok, err := s.GetSubsonicPassword(ctx, "frank")
	if err != nil {
		t.Fatalf("GetSubsonicPassword: %v", err)
	}
	if ok {
		t.Error("GetSubsonicPassword: ok=true for user with no subsonic password set")
	}
	if enc != "" {
		t.Errorf("GetSubsonicPassword: got %q, want empty string", enc)
	}
}

// TestSetGetSubsonicPasswordRoundtrip verifies that SetSubsonicPassword stores the
// value and GetSubsonicPassword retrieves it exactly.
func TestSetGetSubsonicPasswordRoundtrip(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	u := sampleUser("grace", "grace@example.com")
	if _, err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	const encrypted = "AES256GCM:base64ciphertext=="
	if err := s.SetSubsonicPassword(ctx, "grace", encrypted); err != nil {
		t.Fatalf("SetSubsonicPassword: %v", err)
	}

	got, ok, err := s.GetSubsonicPassword(ctx, "grace")
	if err != nil {
		t.Fatalf("GetSubsonicPassword: %v", err)
	}
	if !ok {
		t.Error("GetSubsonicPassword: ok=false after SetSubsonicPassword")
	}
	if got != encrypted {
		t.Errorf("GetSubsonicPassword: got %q, want %q", got, encrypted)
	}
}

// TestDuplicateUsernameReturnsError verifies that inserting two users with the same
// username returns an error on the second insert.
func TestDuplicateUsernameReturnsError(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	u1 := sampleUser("heidi", "heidi@example.com")
	if _, err := s.CreateUser(ctx, u1); err != nil {
		t.Fatalf("CreateUser first: %v", err)
	}

	u2 := sampleUser("heidi", "heidi-alt@example.com")
	_, err := s.CreateUser(ctx, u2)
	if err == nil {
		t.Error("CreateUser duplicate username: expected error, got nil")
	}
}

// TestDuplicateEmailReturnsError verifies that inserting two users with the same
// email returns an error on the second insert.
func TestDuplicateEmailReturnsError(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	u1 := sampleUser("ivan", "shared@example.com")
	if _, err := s.CreateUser(ctx, u1); err != nil {
		t.Fatalf("CreateUser first: %v", err)
	}

	u2 := sampleUser("judy", "shared@example.com")
	_, err := s.CreateUser(ctx, u2)
	if err == nil {
		t.Error("CreateUser duplicate email: expected error, got nil")
	}
}

// TestGetUserUnknownIDReturnsNil verifies that GetUser for a non-existent ID
// returns (nil, nil) — no error, just absence.
func TestGetUserUnknownIDReturnsNil(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	got, err := s.GetUser(ctx, 99999)
	if err != nil {
		t.Fatalf("GetUser (unknown ID): unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("GetUser (unknown ID): got non-nil user, want nil")
	}
}

// TestDeleteNonExistentUserNoError verifies that deleting a user that does not
// exist does not return an error (idempotent delete).
func TestDeleteNonExistentUserNoError(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	if err := s.DeleteUser(ctx, 99999); err != nil {
		t.Errorf("DeleteUser (non-existent): expected no error, got %v", err)
	}
}

// TestUpdateUserIdempotent verifies that calling UpdateUser twice with the same
// data leaves the record in the correct state.
func TestUpdateUserIdempotent(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	u := sampleUser("kate", "kate@example.com")
	id, err := s.CreateUser(ctx, u)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	u.ID = id
	u.DisplayName = "Katherine"

	for i := 0; i < 2; i++ {
		if err := s.UpdateUser(ctx, u); err != nil {
			t.Fatalf("UpdateUser (pass %d): %v", i+1, err)
		}
	}

	got, err := s.GetUser(ctx, id)
	if err != nil {
		t.Fatalf("GetUser after idempotent update: %v", err)
	}
	if got.DisplayName != "Katherine" {
		t.Errorf("DisplayName after idempotent update: got %q, want %q", got.DisplayName, "Katherine")
	}
}

// TestSubsonicPasswordMissingUser verifies GetSubsonicPassword returns ("", false, nil)
// for a username that does not exist in the database.
func TestSubsonicPasswordMissingUser(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	enc, ok, err := s.GetSubsonicPassword(ctx, "nobody")
	if err != nil {
		t.Fatalf("GetSubsonicPassword (missing user): unexpected error: %v", err)
	}
	if ok {
		t.Error("GetSubsonicPassword (missing user): ok=true, want false")
	}
	if enc != "" {
		t.Errorf("GetSubsonicPassword (missing user): got %q, want empty", enc)
	}
}

// TestSetSubsonicPasswordOverwrite verifies that calling SetSubsonicPassword a second
// time replaces the previous value.
func TestSetSubsonicPasswordOverwrite(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	u := sampleUser("leo", "leo@example.com")
	if _, err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := s.SetSubsonicPassword(ctx, "leo", "first-value"); err != nil {
		t.Fatalf("SetSubsonicPassword (first): %v", err)
	}
	if err := s.SetSubsonicPassword(ctx, "leo", "second-value"); err != nil {
		t.Fatalf("SetSubsonicPassword (second): %v", err)
	}

	got, ok, err := s.GetSubsonicPassword(ctx, "leo")
	if err != nil {
		t.Fatalf("GetSubsonicPassword after overwrite: %v", err)
	}
	if !ok {
		t.Error("GetSubsonicPassword after overwrite: ok=false")
	}
	if got != "second-value" {
		t.Errorf("GetSubsonicPassword after overwrite: got %q, want %q", got, "second-value")
	}
}

// TestListUsersOrderedAlphabetically inserts users in reverse order and confirms
// ListUsers always returns them sorted by username ascending.
func TestListUsersOrderedAlphabetically(t *testing.T) {
	s := newTestUserStore(t)
	ctx := context.Background()

	insertOrder := []string{"zoe", "anne", "mary", "bob"}
	for _, name := range insertOrder {
		if _, err := s.CreateUser(ctx, sampleUser(name, name+"@example.com")); err != nil {
			t.Fatalf("CreateUser(%q): %v", name, err)
		}
	}

	users, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}

	for i := 1; i < len(users); i++ {
		if strings.ToLower(users[i-1].Username) > strings.ToLower(users[i].Username) {
			t.Errorf("ListUsers: not sorted at index %d: %q > %q",
				i, users[i-1].Username, users[i].Username)
		}
	}
}
