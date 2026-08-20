package store

import (
	"context"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
)

func TestGetTOTPSecret_NotFound(t *testing.T) {
	s := newTestAdminStore(t)
	got, err := s.GetTOTPSecret(context.Background(), "admin", 1)
	if err != nil {
		t.Fatalf("GetTOTPSecret: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestEnableGetDisableTOTP(t *testing.T) {
	s := newTestAdminStore(t)
	ctx := context.Background()

	adminID, err := s.CreateAdmin(ctx, sampleAdmin("gina", "gina@example.com"))
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}

	err = s.EnableTOTP(ctx, &model.TOTPSecret{
		UserType:    "admin",
		UserID:      adminID,
		Secret:      "encrypted-secret",
		BackupCodes: `["hash1","hash2"]`,
	})
	if err != nil {
		t.Fatalf("EnableTOTP: %v", err)
	}

	got, err := s.GetTOTPSecret(ctx, "admin", adminID)
	if err != nil {
		t.Fatalf("GetTOTPSecret: %v", err)
	}
	if got == nil {
		t.Fatalf("expected TOTP secret row")
	}
	if !got.Enabled || got.Secret != "encrypted-secret" || got.BackupCodes != `["hash1","hash2"]` {
		t.Fatalf("unexpected totp secret: %+v", got)
	}

	// Re-enabling (re-enrollment) must replace the secret and backup codes.
	err = s.EnableTOTP(ctx, &model.TOTPSecret{
		UserType:    "admin",
		UserID:      adminID,
		Secret:      "new-encrypted-secret",
		BackupCodes: `["hash3"]`,
	})
	if err != nil {
		t.Fatalf("EnableTOTP (replace): %v", err)
	}
	got, err = s.GetTOTPSecret(ctx, "admin", adminID)
	if err != nil {
		t.Fatalf("GetTOTPSecret: %v", err)
	}
	if got.Secret != "new-encrypted-secret" || got.BackupCodes != `["hash3"]` {
		t.Fatalf("expected replaced secret/codes, got %+v", got)
	}

	if err := s.TouchTOTPLastUsed(ctx, "admin", adminID); err != nil {
		t.Fatalf("TouchTOTPLastUsed: %v", err)
	}
	got, err = s.GetTOTPSecret(ctx, "admin", adminID)
	if err != nil {
		t.Fatalf("GetTOTPSecret: %v", err)
	}
	if got.LastUsed.IsZero() {
		t.Fatalf("expected LastUsed to be set")
	}

	if err := s.UpdateTOTPBackupCodes(ctx, "admin", adminID, `["hash4"]`); err != nil {
		t.Fatalf("UpdateTOTPBackupCodes: %v", err)
	}
	got, err = s.GetTOTPSecret(ctx, "admin", adminID)
	if err != nil {
		t.Fatalf("GetTOTPSecret: %v", err)
	}
	if got.BackupCodes != `["hash4"]` {
		t.Fatalf("expected updated backup codes, got %+v", got)
	}

	if err := s.DisableTOTP(ctx, "admin", adminID); err != nil {
		t.Fatalf("DisableTOTP: %v", err)
	}
	got, err = s.GetTOTPSecret(ctx, "admin", adminID)
	if err != nil {
		t.Fatalf("GetTOTPSecret after disable: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil after disable, got %+v", got)
	}
}

func TestAdminMFAChallengeLifecycle(t *testing.T) {
	s := newTestAdminStore(t)
	ctx := context.Background()

	adminID, err := s.CreateAdmin(ctx, sampleAdmin("hank", "hank@example.com"))
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}

	c := &model.AdminMFAChallenge{
		TokenHash: "challenge-hash",
		AdminID:   adminID,
		IP:        "127.0.0.1",
		UserAgent: "test-agent",
		Remember:  true,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	if err := s.CreateAdminMFAChallenge(ctx, c); err != nil {
		t.Fatalf("CreateAdminMFAChallenge: %v", err)
	}

	got, err := s.GetAdminMFAChallengeByHash(ctx, "challenge-hash")
	if err != nil {
		t.Fatalf("GetAdminMFAChallengeByHash: %v", err)
	}
	if got == nil || got.AdminID != adminID || !got.Remember {
		t.Fatalf("unexpected challenge: %+v", got)
	}
	if got.IsExpired() {
		t.Fatalf("expected challenge not expired")
	}

	if err := s.DeleteAdminMFAChallenge(ctx, "challenge-hash"); err != nil {
		t.Fatalf("DeleteAdminMFAChallenge: %v", err)
	}
	got, err = s.GetAdminMFAChallengeByHash(ctx, "challenge-hash")
	if err != nil {
		t.Fatalf("GetAdminMFAChallengeByHash after delete: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil after delete, got %+v", got)
	}
}

func TestPurgeExpiredAdminMFAChallenges(t *testing.T) {
	s := newTestAdminStore(t)
	ctx := context.Background()

	adminID, err := s.CreateAdmin(ctx, sampleAdmin("ivan", "ivan@example.com"))
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}

	live := &model.AdminMFAChallenge{TokenHash: "live", AdminID: adminID, ExpiresAt: time.Now().Add(time.Hour)}
	expired := &model.AdminMFAChallenge{TokenHash: "expired", AdminID: adminID, ExpiresAt: time.Now().Add(-time.Hour)}
	if err := s.CreateAdminMFAChallenge(ctx, live); err != nil {
		t.Fatalf("CreateAdminMFAChallenge live: %v", err)
	}
	if err := s.CreateAdminMFAChallenge(ctx, expired); err != nil {
		t.Fatalf("CreateAdminMFAChallenge expired: %v", err)
	}

	if err := s.PurgeExpiredAdminMFAChallenges(ctx); err != nil {
		t.Fatalf("PurgeExpiredAdminMFAChallenges: %v", err)
	}
	if got, err := s.GetAdminMFAChallengeByHash(ctx, "expired"); err != nil || got != nil {
		t.Fatalf("expected expired challenge purged, got (%+v, %v)", got, err)
	}
	if got, err := s.GetAdminMFAChallengeByHash(ctx, "live"); err != nil || got == nil {
		t.Fatalf("expected live challenge to survive purge, got (%+v, %v)", got, err)
	}
}
