package store

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters for admin password hashing. Kept local to this file so
// admin auth (PART 17) does not depend on, or refactor, the unrelated
// Argon2id call sites used for Subsonic/Ampache user passwords elsewhere in
// the codebase.
const (
	adminHashMemory      = 65536
	adminHashIterations  = 3
	adminHashParallelism = 4
	adminHashKeyLen      = 32
	adminHashSaltLen     = 16
)

// HashPassword produces an Argon2id hash of the given plaintext password.
// Format: $argon2id$v=19$m=65536,t=3,p=4$<base64salt>$<base64hash>
func HashPassword(password string) (string, error) {
	salt := make([]byte, adminHashSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("hash password: generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, adminHashIterations, adminHashMemory, adminHashParallelism, adminHashKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		adminHashMemory, adminHashIterations, adminHashParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword checks a candidate plaintext password against a stored
// Argon2id hash of the form
// $argon2id$v=19$m=<mem>,t=<time>,p=<threads>$<base64salt>$<base64hash>.
func VerifyPassword(hash, password string) (bool, error) {
	parts := strings.Split(hash, "$")
	// Expected split: ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("verify password: unsupported hash format")
	}

	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, fmt.Errorf("verify password: invalid argon2id params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("verify password: invalid argon2id salt: %w", err)
	}
	storedKey, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("verify password: invalid argon2id key: %w", err)
	}

	computed := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(storedKey)))
	return subtle.ConstantTimeCompare(computed, storedKey) == 1, nil
}

// NeedsRehash reports whether a stored hash uses weaker-than-current Argon2id
// parameters and should be re-hashed on the next successful login.
func NeedsRehash(hash string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return true
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return true
	}
	return memory < adminHashMemory || iterations < adminHashIterations || parallelism < adminHashParallelism
}
