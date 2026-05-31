package subsonic

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2idParams holds the fixed Argon2id cost parameters used for all new hashes.
const (
	argon2Memory      = 65536
	argon2Iterations  = 3
	argon2Parallelism = 4
	argon2KeyLen      = 32
	argon2SaltLen     = 16
)

// HashPassword hashes password using Argon2id and returns the encoded hash string.
// Format: $argon2id$v=19$m=65536,t=3,p=4$<base64salt>$<base64hash>
func HashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2id: generate salt: %w", err)
	}

	key := argon2.IDKey(
		[]byte(password),
		salt,
		argon2Iterations,
		argon2Memory,
		argon2Parallelism,
		argon2KeyLen,
	)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedKey := base64.RawStdEncoding.EncodeToString(key)

	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory, argon2Iterations, argon2Parallelism,
		encodedSalt, encodedKey,
	), nil
}

// VerifyPassword checks candidate against the stored Argon2id hash using
// constant-time comparison. Returns (true, nil) on match.
func VerifyPassword(candidate, hash string) (bool, error) {
	parts := strings.Split(hash, "$")
	// Expected: ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("argon2id: unsupported hash format")
	}

	var memory, iterations uint32
	var parallelism uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
	if err != nil {
		return false, fmt.Errorf("argon2id: parse params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("argon2id: decode salt: %w", err)
	}

	storedKey, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("argon2id: decode key: %w", err)
	}

	computed := argon2.IDKey(
		[]byte(candidate),
		salt,
		iterations,
		memory,
		parallelism,
		uint32(len(storedKey)),
	)

	return subtle.ConstantTimeCompare(computed, storedKey) == 1, nil
}
