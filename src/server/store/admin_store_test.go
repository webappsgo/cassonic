package store

import (
	"context"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
)

// newTestAdminStore creates an in-memory AdminStore spanning two in-memory
// SQLite databases, mirroring the real users.db/server.db split.
func newTestAdminStore(t *testing.T) AdminStore {
	t.Helper()
	usersDB, err := openDB(":memory:", usersSchema)
	if err != nil {
		t.Fatalf("openDB users :memory:: %v", err)
	}
	t.Cleanup(func() { usersDB.Close() })

	serverDB, err := openDB(":memory:", serverSchema)
	if err != nil {
		t.Fatalf("openDB server :memory:: %v", err)
	}
	t.Cleanup(func() { serverDB.Close() })

	return &sqliteAdminStore{usersDB: usersDB, serverDB: serverDB}
}

func sampleAdmin(username, email string) *model.Admin {
	hash, _ := HashPassword("correct horse battery staple")
	return &model.Admin{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Role:         "admin",
		Enabled:      true,
		Source:       "local",
	}
}

func TestCreateGetAdminRoundtrip(t *testing.T) {
	s := newTestAdminStore(t)
	ctx := context.Background()

	a := sampleAdmin("alice", "alice@example.com")
	id, err := s.CreateAdmin(ctx, a)
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}
	if id == 0 {
		t.Fatalf("expected nonzero id")
	}

	got, err := s.GetAdmin(ctx, id)
	if err != nil {
		t.Fatalf("GetAdmin: %v", err)
	}
	if got == nil {
		t.Fatalf("GetAdmin: got nil")
	}
	if got.Username != "alice" || got.Email != "alice@example.com" || got.Role != "admin" || !got.Enabled {
		t.Fatalf("unexpected admin: %+v", got)
	}

	byUsername, err := s.GetAdminByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("GetAdminByUsername: %v", err)
	}
	if byUsername == nil || byUsername.ID != id {
		t.Fatalf("GetAdminByUsername mismatch: %+v", byUsername)
	}

	// CreateAdmin must also provision a default preferences row.
	prefs, err := s.GetAdminPreferences(ctx, id)
	if err != nil {
		t.Fatalf("GetAdminPreferences: %v", err)
	}
	if prefs == nil {
		t.Fatalf("expected default preferences row to exist")
	}
	if prefs.Theme != "dark" || !prefs.EmailSecurity {
		t.Fatalf("unexpected default preferences: %+v", prefs)
	}
}

func TestGetAdminByUsername_NotFound(t *testing.T) {
	s := newTestAdminStore(t)
	got, err := s.GetAdminByUsername(context.Background(), "nobody")
	if err != nil {
		t.Fatalf("GetAdminByUsername: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestUpdateDeleteAdmin(t *testing.T) {
	s := newTestAdminStore(t)
	ctx := context.Background()

	a := sampleAdmin("bob", "bob@example.com")
	id, err := s.CreateAdmin(ctx, a)
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}

	a.ID = id
	a.Role = "readonly"
	a.Enabled = false
	if err := s.UpdateAdmin(ctx, a); err != nil {
		t.Fatalf("UpdateAdmin: %v", err)
	}

	got, err := s.GetAdmin(ctx, id)
	if err != nil {
		t.Fatalf("GetAdmin: %v", err)
	}
	if got.Role != "readonly" || got.Enabled {
		t.Fatalf("update did not persist: %+v", got)
	}

	if err := s.DeleteAdmin(ctx, id); err != nil {
		t.Fatalf("DeleteAdmin: %v", err)
	}
	got, err = s.GetAdmin(ctx, id)
	if err != nil {
		t.Fatalf("GetAdmin after delete: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil after delete, got %+v", got)
	}
}

func TestListAndCountAdmins(t *testing.T) {
	s := newTestAdminStore(t)
	ctx := context.Background()

	if n, err := s.CountAdmins(ctx); err != nil || n != 0 {
		t.Fatalf("CountAdmins: got (%d, %v), want (0, nil)", n, err)
	}

	first, err := s.CreateAdmin(ctx, sampleAdmin("primary", "p@example.com"))
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}
	if _, err := s.CreateAdmin(ctx, sampleAdmin("second", "s@example.com")); err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}

	admins, err := s.ListAdmins(ctx)
	if err != nil {
		t.Fatalf("ListAdmins: %v", err)
	}
	if len(admins) != 2 {
		t.Fatalf("expected 2 admins, got %d", len(admins))
	}
	// Lowest id (the Primary Admin) must come first.
	if admins[0].ID != first {
		t.Fatalf("expected primary admin first, got id %d", admins[0].ID)
	}

	if n, err := s.CountAdmins(ctx); err != nil || n != 2 {
		t.Fatalf("CountAdmins: got (%d, %v), want (2, nil)", n, err)
	}
}

func TestAdminLoginAttemptsAndLockout(t *testing.T) {
	s := newTestAdminStore(t)
	ctx := context.Background()

	id, err := s.CreateAdmin(ctx, sampleAdmin("carol", "carol@example.com"))
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := s.IncrementAdminLoginAttempts(ctx, id); err != nil {
			t.Fatalf("IncrementAdminLoginAttempts: %v", err)
		}
	}
	got, err := s.GetAdmin(ctx, id)
	if err != nil {
		t.Fatalf("GetAdmin: %v", err)
	}
	if got.FailedAttempts != 3 {
		t.Fatalf("expected 3 failed attempts, got %d", got.FailedAttempts)
	}

	until := time.Now().Add(15 * time.Minute)
	if err := s.SetAdminLockedUntil(ctx, id, until); err != nil {
		t.Fatalf("SetAdminLockedUntil: %v", err)
	}
	got, err = s.GetAdmin(ctx, id)
	if err != nil {
		t.Fatalf("GetAdmin: %v", err)
	}
	if !got.IsLocked() {
		t.Fatalf("expected admin to be locked")
	}

	if err := s.ResetAdminLoginAttempts(ctx, id); err != nil {
		t.Fatalf("ResetAdminLoginAttempts: %v", err)
	}
	got, err = s.GetAdmin(ctx, id)
	if err != nil {
		t.Fatalf("GetAdmin: %v", err)
	}
	if got.FailedAttempts != 0 || got.IsLocked() {
		t.Fatalf("expected reset admin, got %+v", got)
	}

	if err := s.UpdateAdminLastLogin(ctx, id); err != nil {
		t.Fatalf("UpdateAdminLastLogin: %v", err)
	}
	got, err = s.GetAdmin(ctx, id)
	if err != nil {
		t.Fatalf("GetAdmin: %v", err)
	}
	if got.LastLogin.IsZero() {
		t.Fatalf("expected LastLogin to be set")
	}
}

func TestAdminPreferencesUpdate(t *testing.T) {
	s := newTestAdminStore(t)
	ctx := context.Background()

	id, err := s.CreateAdmin(ctx, sampleAdmin("dave", "dave@example.com"))
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}

	prefs, err := s.GetAdminPreferences(ctx, id)
	if err != nil {
		t.Fatalf("GetAdminPreferences: %v", err)
	}
	prefs.Theme = "light"
	prefs.EmailUsers = false
	if err := s.UpdateAdminPreferences(ctx, prefs); err != nil {
		t.Fatalf("UpdateAdminPreferences: %v", err)
	}

	got, err := s.GetAdminPreferences(ctx, id)
	if err != nil {
		t.Fatalf("GetAdminPreferences: %v", err)
	}
	if got.Theme != "light" || got.EmailUsers {
		t.Fatalf("update did not persist: %+v", got)
	}
}

func TestAdminSessionLifecycle(t *testing.T) {
	s := newTestAdminStore(t)
	ctx := context.Background()

	adminID, err := s.CreateAdmin(ctx, sampleAdmin("erin", "erin@example.com"))
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}

	sess := &model.AdminSession{
		TokenHash: "deadbeef",
		AdminID:   adminID,
		IP:        "127.0.0.1",
		UserAgent: "test-agent",
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}
	if err := s.CreateAdminSession(ctx, sess); err != nil {
		t.Fatalf("CreateAdminSession: %v", err)
	}

	got, err := s.GetAdminSessionByHash(ctx, "deadbeef")
	if err != nil {
		t.Fatalf("GetAdminSessionByHash: %v", err)
	}
	if got == nil || got.AdminID != adminID {
		t.Fatalf("unexpected session: %+v", got)
	}
	if got.IsExpired() {
		t.Fatalf("expected session not expired")
	}

	if err := s.DeleteAdminSession(ctx, "deadbeef"); err != nil {
		t.Fatalf("DeleteAdminSession: %v", err)
	}
	got, err = s.GetAdminSessionByHash(ctx, "deadbeef")
	if err != nil {
		t.Fatalf("GetAdminSessionByHash after delete: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil after delete, got %+v", got)
	}
}

func TestDeleteAdminSessionsAndPurgeExpired(t *testing.T) {
	s := newTestAdminStore(t)
	ctx := context.Background()

	adminID, err := s.CreateAdmin(ctx, sampleAdmin("frank", "frank@example.com"))
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}

	live := &model.AdminSession{TokenHash: "live", AdminID: adminID, ExpiresAt: time.Now().Add(time.Hour)}
	expired := &model.AdminSession{TokenHash: "expired", AdminID: adminID, ExpiresAt: time.Now().Add(-time.Hour)}
	if err := s.CreateAdminSession(ctx, live); err != nil {
		t.Fatalf("CreateAdminSession live: %v", err)
	}
	if err := s.CreateAdminSession(ctx, expired); err != nil {
		t.Fatalf("CreateAdminSession expired: %v", err)
	}

	if err := s.PurgeExpiredAdminSessions(ctx); err != nil {
		t.Fatalf("PurgeExpiredAdminSessions: %v", err)
	}
	if got, err := s.GetAdminSessionByHash(ctx, "expired"); err != nil || got != nil {
		t.Fatalf("expected expired session purged, got (%+v, %v)", got, err)
	}
	if got, err := s.GetAdminSessionByHash(ctx, "live"); err != nil || got == nil {
		t.Fatalf("expected live session to survive purge, got (%+v, %v)", got, err)
	}

	if err := s.DeleteAdminSessions(ctx, adminID); err != nil {
		t.Fatalf("DeleteAdminSessions: %v", err)
	}
	if got, err := s.GetAdminSessionByHash(ctx, "live"); err != nil || got != nil {
		t.Fatalf("expected all sessions deleted, got (%+v, %v)", got, err)
	}
}

func TestAuditLogAppendAndList(t *testing.T) {
	s := newTestAdminStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		e := &model.AuditEntry{
			Level:     "info",
			Category:  "auth",
			Action:    "login",
			ActorType: "admin",
			ActorID:   "1",
			ActorIP:   "127.0.0.1",
			Detail:    "{}",
			Success:   true,
		}
		if err := s.AppendAuditEntry(ctx, e); err != nil {
			t.Fatalf("AppendAuditEntry: %v", err)
		}
	}

	entries, err := s.ListAuditEntries(ctx, 2)
	if err != nil {
		t.Fatalf("ListAuditEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (limit), got %d", len(entries))
	}
	if entries[0].ID <= entries[1].ID {
		t.Fatalf("expected newest-first ordering, got ids %d, %d", entries[0].ID, entries[1].ID)
	}
}

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("s3cr3t-passw0rd")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	ok, err := VerifyPassword(hash, "s3cr3t-passw0rd")
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Fatalf("expected password to verify")
	}

	ok, err = VerifyPassword(hash, "wrong-password")
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if ok {
		t.Fatalf("expected wrong password to fail verification")
	}

	if NeedsRehash(hash) {
		t.Fatalf("freshly hashed password should not need rehash")
	}
	if !NeedsRehash("$argon2id$v=19$m=1024,t=1,p=1$c2FsdA$aGFzaA") {
		t.Fatalf("weak-parameter hash should need rehash")
	}
	if !NeedsRehash("not-a-valid-hash") {
		t.Fatalf("malformed hash should need rehash")
	}
}
