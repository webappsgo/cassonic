package model

import (
	"testing"
	"time"
)

func TestUserIsLocked(t *testing.T) {
	t.Run("zero LockedUntil is not locked", func(t *testing.T) {
		u := &User{}
		if u.IsLocked() {
			t.Error("User with zero LockedUntil should not be locked")
		}
	})
	t.Run("LockedUntil in past is not locked", func(t *testing.T) {
		u := &User{LockedUntil: time.Now().Add(-time.Minute)}
		if u.IsLocked() {
			t.Error("User with past LockedUntil should not be locked")
		}
	})
	t.Run("LockedUntil in future is locked", func(t *testing.T) {
		u := &User{LockedUntil: time.Now().Add(time.Hour)}
		if !u.IsLocked() {
			t.Error("User with future LockedUntil should be locked")
		}
	})
}

func TestAPITokenIsExpired(t *testing.T) {
	t.Run("zero ExpiresAt never expires", func(t *testing.T) {
		tok := &APIToken{}
		if tok.IsExpired() {
			t.Error("APIToken with zero ExpiresAt should never be expired")
		}
	})
	t.Run("ExpiresAt in past is expired", func(t *testing.T) {
		tok := &APIToken{ExpiresAt: time.Now().Add(-time.Minute)}
		if !tok.IsExpired() {
			t.Error("APIToken with past ExpiresAt should be expired")
		}
	})
	t.Run("ExpiresAt in future is not expired", func(t *testing.T) {
		tok := &APIToken{ExpiresAt: time.Now().Add(time.Hour)}
		if tok.IsExpired() {
			t.Error("APIToken with future ExpiresAt should not be expired")
		}
	})
}

func TestShareIsExpired(t *testing.T) {
	t.Run("zero ExpiresAt never expires", func(t *testing.T) {
		s := &Share{}
		if s.IsExpired() {
			t.Error("Share with zero ExpiresAt should never be expired")
		}
	})
	t.Run("ExpiresAt in past is expired", func(t *testing.T) {
		s := &Share{ExpiresAt: time.Now().Add(-time.Minute)}
		if !s.IsExpired() {
			t.Error("Share with past ExpiresAt should be expired")
		}
	})
	t.Run("ExpiresAt in future is not expired", func(t *testing.T) {
		s := &Share{ExpiresAt: time.Now().Add(time.Hour)}
		if s.IsExpired() {
			t.Error("Share with future ExpiresAt should not be expired")
		}
	})
}

func TestShareHasPassword(t *testing.T) {
	t.Run("empty PasswordHash has no password", func(t *testing.T) {
		s := &Share{}
		if s.HasPassword() {
			t.Error("Share with empty PasswordHash should not have a password")
		}
	})
	t.Run("non-empty PasswordHash has password", func(t *testing.T) {
		s := &Share{PasswordHash: "sha256:abc123"}
		if !s.HasPassword() {
			t.Error("Share with non-empty PasswordHash should have a password")
		}
	})
}

func TestServiceTypeProtocol(t *testing.T) {
	tests := []struct {
		svc      ServiceType
		wantProto string
	}{
		{ServiceLastFM, "lastfm"},
		{ServiceLibreFM, "lastfm"},
		{ServiceGnuFM, "lastfm"},
		{ServiceCustomLastFM, "lastfm"},
		{ServiceListenBrainz, "listenbrainz"},
		{ServiceMaloja, "listenbrainz"},
		{ServiceCustomListenBrainz, "listenbrainz"},
	}
	for _, tt := range tests {
		t.Run(string(tt.svc), func(t *testing.T) {
			if got := tt.svc.Protocol(); got != tt.wantProto {
				t.Errorf("Protocol() = %q, want %q", got, tt.wantProto)
			}
		})
	}
}

func TestServiceTypeBaseURL(t *testing.T) {
	tests := []struct {
		svc     ServiceType
		wantURL string
	}{
		{ServiceLastFM, "https://ws.audioscrobbler.com/2.0/"},
		{ServiceLibreFM, "https://libre.fm/2.0/"},
		{ServiceGnuFM, "https://gnufm.libresource.org/2.0/"},
		{ServiceListenBrainz, "https://api.listenbrainz.org"},
		{ServiceMaloja, ""},
		{ServiceCustomLastFM, ""},
		{ServiceCustomListenBrainz, ""},
	}
	for _, tt := range tests {
		t.Run(string(tt.svc), func(t *testing.T) {
			if got := tt.svc.BaseURL(); got != tt.wantURL {
				t.Errorf("BaseURL() = %q, want %q", got, tt.wantURL)
			}
		})
	}
}
