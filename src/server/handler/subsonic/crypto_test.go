package subsonic

// Tests for crypto.go: HashPassword and VerifyPassword (Argon2id).

import (
	"strings"
	"testing"
)

// TestHashPasswordFormat verifies the encoded hash has the expected structure.
func TestHashPasswordFormat(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Fatalf("hash has %d parts, want 6: %q", len(parts), hash)
	}
	if parts[1] != "argon2id" {
		t.Errorf("parts[1] = %q, want argon2id", parts[1])
	}
	if parts[2] != "v=19" {
		t.Errorf("parts[2] = %q, want v=19", parts[2])
	}
}

// TestVerifyPasswordCorrect verifies a hash produced by HashPassword verifies
// successfully against the original password.
func TestVerifyPasswordCorrect(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := VerifyPassword("hunter2", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("VerifyPassword: correct password did not verify")
	}
}

// TestVerifyPasswordIncorrect verifies a wrong password fails verification
// without an error.
func TestVerifyPasswordIncorrect(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := VerifyPassword("wrongpassword", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if ok {
		t.Error("VerifyPassword: wrong password verified successfully")
	}
}

// TestVerifyPasswordMalformedHash verifies malformed hash strings return an error.
func TestVerifyPasswordMalformedHash(t *testing.T) {
	cases := []string{
		"",
		"not-a-hash-at-all",
		"$argon2id$v=19$m=65536,t=3,p=4$onlyonemore",
		"$bcrypt$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			if _, err := VerifyPassword("anything", c); err == nil {
				t.Errorf("VerifyPassword(%q) expected error, got nil", c)
			}
		})
	}
}

// TestHashPasswordUniqueSalt verifies two hashes of the same password differ
// due to random salting.
func TestHashPasswordUniqueSalt(t *testing.T) {
	h1, err := HashPassword("samepassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	h2, err := HashPassword("samepassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if h1 == h2 {
		t.Error("two hashes of the same password should differ due to random salt")
	}
}

// TestHashPasswordEmptyPassword verifies hashing and verifying an empty
// password string works without error.
func TestHashPasswordEmptyPassword(t *testing.T) {
	hash, err := HashPassword("")
	if err != nil {
		t.Fatalf("HashPassword(\"\"): %v", err)
	}
	ok, err := VerifyPassword("", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("VerifyPassword: empty password did not verify against its own hash")
	}
}
