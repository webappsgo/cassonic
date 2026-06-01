// Package crypto provides AES-256-GCM encrypt/decrypt helpers for reversible
// credential storage (e.g. subsonic passwords that must be recovered as plaintext).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

// DeriveKey derives a 32-byte AES key from the given secret using Argon2id.
// The salt is deterministic: the first 16 bytes of the secret itself.
// Using a fixed salt is intentional here — the secret is high-entropy and
// the goal is a per-server-instance deterministic key, not a password hash.
func DeriveKey(secret []byte) []byte {
	salt := make([]byte, 16)
	copy(salt, secret)
	return argon2.IDKey(secret, salt, 1, 64*1024, 4, 32)
}

// Encrypt encrypts plaintext using AES-256-GCM with the given 32-byte key.
// Returns a base64-encoded ciphertext with the random nonce prepended.
func Encrypt(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext using the given key.
func Decrypt(key []byte, ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, cipherData := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
