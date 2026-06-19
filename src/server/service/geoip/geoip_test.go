package geoip

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOpenOptionalEmptyPath verifies that an empty path returns (nil, nil).
func TestOpenOptionalEmptyPath(t *testing.T) {
	db, err := OpenOptional("")
	if err != nil {
		t.Fatalf("OpenOptional empty: unexpected error: %v", err)
	}
	if db != nil {
		t.Error("OpenOptional empty: expected nil DB, got non-nil")
	}
}

// TestOpenOptionalMissingFile verifies that a non-existent path returns (nil, nil).
func TestOpenOptionalMissingFile(t *testing.T) {
	db, err := OpenOptional("/tmp/does-not-exist-cassonic-geoip.mmdb")
	if err != nil {
		t.Fatalf("OpenOptional missing file: unexpected error: %v", err)
	}
	if db != nil {
		t.Error("OpenOptional missing file: expected nil DB, got non-nil")
	}
}

// TestOpenInvalidFile verifies that Open on a non-MaxMind file returns an error.
func TestOpenInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.mmdb")
	if err := os.WriteFile(path, []byte("this is not a maxminddb file"), 0600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	_, err := Open(path)
	if err == nil {
		t.Error("Open on invalid file: expected error, got nil")
	}
}

// TestLookupInvalidIP verifies that Lookup returns an error for an unparseable IP string.
// The IP parsing check happens before any reader call, so a nil reader is safe here.
func TestLookupInvalidIP(t *testing.T) {
	d := &DB{reader: nil}

	_, err := d.Lookup("not-an-ip")
	if err == nil {
		t.Error("Lookup('not-an-ip'): expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid IP") {
		t.Errorf("Lookup error: want 'invalid IP' in message, got: %v", err)
	}
}

// TestLookupEmptyIP verifies that an empty string is rejected as invalid.
func TestLookupEmptyIP(t *testing.T) {
	d := &DB{reader: nil}

	_, err := d.Lookup("")
	if err == nil {
		t.Error("Lookup(''): expected error, got nil")
	}
}
