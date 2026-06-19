package store

import (
	"context"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
)

// newTestShareStore creates an in-memory SQLite ShareStore for testing.
func newTestShareStore(t *testing.T) ShareStore {
	t.Helper()
	db, err := openDB(":memory:", serverSchema)
	if err != nil {
		t.Fatalf("openDB :memory:: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	s, err := NewShareStore(db)
	if err != nil {
		t.Fatalf("NewShareStore: %v", err)
	}
	return s
}

// sampleShare returns a minimal model.Share for testing.
func sampleShare(userID int64, token string) *model.Share {
	return &model.Share{
		UserID:      userID,
		Token:       token,
		ItemType:    "song",
		ItemID:      42,
		Description: "test share",
	}
}

// TestCreateGetShareRoundtrip verifies that a share created via CreateShare
// can be retrieved by ID with all fields intact.
func TestCreateGetShareRoundtrip(t *testing.T) {
	s := newTestShareStore(t)
	ctx := context.Background()

	sh := sampleShare(1, "abc123")
	id, err := s.CreateShare(ctx, sh)
	if err != nil {
		t.Fatalf("CreateShare: %v", err)
	}
	if id <= 0 {
		t.Fatalf("CreateShare returned non-positive ID: %d", id)
	}

	got, err := s.GetShare(ctx, id)
	if err != nil {
		t.Fatalf("GetShare: %v", err)
	}
	if got == nil {
		t.Fatal("GetShare returned nil for a just-created share")
	}
	if got.UserID != 1 {
		t.Errorf("UserID: got %d, want 1", got.UserID)
	}
	if got.Token != "abc123" {
		t.Errorf("Token: got %q, want abc123", got.Token)
	}
	if got.ItemType != "song" {
		t.Errorf("ItemType: got %q, want song", got.ItemType)
	}
	if got.ItemID != 42 {
		t.Errorf("ItemID: got %d, want 42", got.ItemID)
	}
}

// TestGetShareMissingReturnsNil verifies that GetShare for an unknown ID returns (nil, nil).
func TestGetShareMissingReturnsNil(t *testing.T) {
	s := newTestShareStore(t)
	ctx := context.Background()

	got, err := s.GetShare(ctx, 99999)
	if err != nil {
		t.Fatalf("GetShare (missing): unexpected error: %v", err)
	}
	if got != nil {
		t.Error("GetShare (missing): expected nil, got non-nil")
	}
}

// TestGetShareByToken verifies lookup by token.
func TestGetShareByToken(t *testing.T) {
	s := newTestShareStore(t)
	ctx := context.Background()

	sh := sampleShare(1, "token-xyz")
	id, err := s.CreateShare(ctx, sh)
	if err != nil {
		t.Fatalf("CreateShare: %v", err)
	}

	got, err := s.GetShareByToken(ctx, "token-xyz")
	if err != nil {
		t.Fatalf("GetShareByToken: %v", err)
	}
	if got == nil {
		t.Fatal("GetShareByToken returned nil")
	}
	if got.ID != id {
		t.Errorf("ID: got %d, want %d", got.ID, id)
	}
}

// TestGetShareByTokenMissing verifies GetShareByToken for unknown token returns (nil, nil).
func TestGetShareByTokenMissing(t *testing.T) {
	s := newTestShareStore(t)
	ctx := context.Background()

	got, err := s.GetShareByToken(ctx, "no-such-token")
	if err != nil {
		t.Fatalf("GetShareByToken (missing): unexpected error: %v", err)
	}
	if got != nil {
		t.Error("GetShareByToken (missing): expected nil, got non-nil")
	}
}

// TestListSharesByUser verifies that ListSharesByUser returns only the specified user's shares.
func TestListSharesByUser(t *testing.T) {
	s := newTestShareStore(t)
	ctx := context.Background()

	const user1, user2 = int64(1), int64(2)

	for i, tok := range []string{"t1", "t2", "t3"} {
		sh := sampleShare(user1, tok)
		sh.ItemID = int64(i + 1)
		if _, err := s.CreateShare(ctx, sh); err != nil {
			t.Fatalf("CreateShare(%q): %v", tok, err)
		}
	}
	if _, err := s.CreateShare(ctx, sampleShare(user2, "u2tok")); err != nil {
		t.Fatalf("CreateShare (user2): %v", err)
	}

	user1Shares, err := s.ListSharesByUser(ctx, user1)
	if err != nil {
		t.Fatalf("ListSharesByUser(1): %v", err)
	}
	if len(user1Shares) != 3 {
		t.Errorf("ListSharesByUser(1): got %d, want 3", len(user1Shares))
	}

	user2Shares, err := s.ListSharesByUser(ctx, user2)
	if err != nil {
		t.Fatalf("ListSharesByUser(2): %v", err)
	}
	if len(user2Shares) != 1 {
		t.Errorf("ListSharesByUser(2): got %d, want 1", len(user2Shares))
	}
}

// TestListSharesByUserEmpty verifies ListSharesByUser for a user with no shares returns empty slice.
func TestListSharesByUserEmpty(t *testing.T) {
	s := newTestShareStore(t)
	ctx := context.Background()

	shares, err := s.ListSharesByUser(ctx, 999)
	if err != nil {
		t.Fatalf("ListSharesByUser (empty): %v", err)
	}
	if len(shares) != 0 {
		t.Errorf("ListSharesByUser (empty): got %d, want 0", len(shares))
	}
}

// TestUpdateSharePersistsChanges verifies UpdateShare writes all mutable fields.
func TestUpdateSharePersistsChanges(t *testing.T) {
	s := newTestShareStore(t)
	ctx := context.Background()

	sh := sampleShare(1, "upd-tok")
	id, err := s.CreateShare(ctx, sh)
	if err != nil {
		t.Fatalf("CreateShare: %v", err)
	}

	sh.ID = id
	sh.Description = "updated desc"
	sh.PasswordHash = "sha256:newhash"
	expiry := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	sh.ExpiresAt = expiry
	if err := s.UpdateShare(ctx, sh); err != nil {
		t.Fatalf("UpdateShare: %v", err)
	}

	got, err := s.GetShare(ctx, id)
	if err != nil {
		t.Fatalf("GetShare after update: %v", err)
	}
	if got.Description != "updated desc" {
		t.Errorf("Description after update: got %q, want 'updated desc'", got.Description)
	}
	if got.PasswordHash != "sha256:newhash" {
		t.Errorf("PasswordHash after update: got %q", got.PasswordHash)
	}
}

// TestDeleteShareRemovesRow verifies DeleteShare removes the share.
func TestDeleteShareRemovesRow(t *testing.T) {
	s := newTestShareStore(t)
	ctx := context.Background()

	sh := sampleShare(1, "del-tok")
	id, err := s.CreateShare(ctx, sh)
	if err != nil {
		t.Fatalf("CreateShare: %v", err)
	}

	if err := s.DeleteShare(ctx, id); err != nil {
		t.Fatalf("DeleteShare: %v", err)
	}

	got, err := s.GetShare(ctx, id)
	if err != nil {
		t.Fatalf("GetShare after delete: %v", err)
	}
	if got != nil {
		t.Error("GetShare after delete: expected nil, got non-nil")
	}
}

// TestIncrementViewCountIncreases verifies IncrementViewCount adds 1 to the counter.
func TestIncrementViewCountIncreases(t *testing.T) {
	s := newTestShareStore(t)
	ctx := context.Background()

	sh := sampleShare(1, "view-tok")
	id, err := s.CreateShare(ctx, sh)
	if err != nil {
		t.Fatalf("CreateShare: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := s.IncrementViewCount(ctx, id); err != nil {
			t.Fatalf("IncrementViewCount (pass %d): %v", i+1, err)
		}
	}

	got, err := s.GetShare(ctx, id)
	if err != nil {
		t.Fatalf("GetShare after IncrementViewCount: %v", err)
	}
	if got.ViewCount != 3 {
		t.Errorf("ViewCount: got %d, want 3", got.ViewCount)
	}
}

// TestShareTokenMustBeUnique verifies that inserting two shares with the same token fails.
func TestShareTokenMustBeUnique(t *testing.T) {
	s := newTestShareStore(t)
	ctx := context.Background()

	sh := sampleShare(1, "dup-tok")
	if _, err := s.CreateShare(ctx, sh); err != nil {
		t.Fatalf("CreateShare first: %v", err)
	}

	_, err := s.CreateShare(ctx, sampleShare(2, "dup-tok"))
	if err == nil {
		t.Error("CreateShare with duplicate token: expected error, got nil")
	}
}
