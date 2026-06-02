package crypto

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// TestDeriveKey covers: 32-byte output length, determinism for same input,
// and divergence for different secrets.
func TestDeriveKey(t *testing.T) {
	secret := []byte("super-secret-key-for-testing-1234")
	key := DeriveKey(secret)

	if len(key) != 32 {
		t.Fatalf("DeriveKey: got %d bytes, want 32", len(key))
	}

	// Same input must always produce the same output (deterministic salt).
	key2 := DeriveKey(secret)
	if !bytes.Equal(key, key2) {
		t.Error("DeriveKey: not deterministic — two calls with same secret returned different keys")
	}

	// Different secret must produce a different key.
	other := DeriveKey([]byte("completely-different-secret-9999"))
	if bytes.Equal(key, other) {
		t.Error("DeriveKey: different secrets produced identical keys")
	}
}

// TestDeriveKeyShortSecret ensures DeriveKey handles a secret shorter than
// the 16-byte salt window without panicking or crashing.
func TestDeriveKeyShortSecret(t *testing.T) {
	key := DeriveKey([]byte("short"))
	if len(key) != 32 {
		t.Fatalf("DeriveKey (short secret): got %d bytes, want 32", len(key))
	}
}

// TestDeriveKeyEmptySecret ensures an empty secret is handled safely.
func TestDeriveKeyEmptySecret(t *testing.T) {
	key := DeriveKey([]byte{})
	if len(key) != 32 {
		t.Fatalf("DeriveKey (empty secret): got %d bytes, want 32", len(key))
	}
}

// TestEncryptDecryptRoundtrip verifies that plaintext survives Encrypt→Decrypt unchanged.
func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := DeriveKey([]byte("roundtrip-test-key-padding-xxxxx"))

	cases := []string{
		"hello world",
		"",
		"unicode: 日本語",
		strings.Repeat("x", 1024),
	}

	for _, plain := range cases {
		ct, err := Encrypt(key, plain)
		if err != nil {
			t.Fatalf("Encrypt(%q): unexpected error: %v", plain, err)
		}

		got, err := Decrypt(key, ct)
		if err != nil {
			t.Fatalf("Decrypt(%q): unexpected error: %v", plain, err)
		}

		if got != plain {
			t.Errorf("roundtrip mismatch: got %q, want %q", got, plain)
		}
	}
}

// TestEncryptProducesUniqueCiphertexts checks that each Encrypt call uses a fresh
// nonce so the same plaintext yields different ciphertexts.
func TestEncryptProducesUniqueCiphertexts(t *testing.T) {
	key := DeriveKey([]byte("nonce-uniqueness-test-key-xxxxxxx"))
	plain := "same plaintext every time"

	ct1, err := Encrypt(key, plain)
	if err != nil {
		t.Fatalf("Encrypt call 1: %v", err)
	}
	ct2, err := Encrypt(key, plain)
	if err != nil {
		t.Fatalf("Encrypt call 2: %v", err)
	}

	if ct1 == ct2 {
		t.Error("Encrypt: two calls with the same plaintext returned identical ciphertext — nonce is not random")
	}
}

// TestDecryptTamperedCiphertext verifies that flipping a byte in the GCM tag
// region causes authentication failure.
func TestDecryptTamperedCiphertext(t *testing.T) {
	key := DeriveKey([]byte("tamper-test-key-padding-xxxxxxxxx"))

	ct, err := Encrypt(key, "sensitive data")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(ct)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}

	// Flip a byte in the middle of the payload (past the 12-byte nonce).
	mid := len(raw) / 2
	raw[mid] ^= 0xFF

	tampered := base64.StdEncoding.EncodeToString(raw)

	_, err = Decrypt(key, tampered)
	if err == nil {
		t.Error("Decrypt tampered ciphertext: expected error, got nil")
	}
}

// TestDecryptTruncatedCiphertext verifies that a payload shorter than the nonce
// size is rejected with an error.
func TestDecryptTruncatedCiphertext(t *testing.T) {
	key := DeriveKey([]byte("truncate-test-key-padding-xxxxxxx"))

	// 5 bytes is shorter than the 12-byte GCM nonce.
	short := base64.StdEncoding.EncodeToString([]byte("short"))

	_, err := Decrypt(key, short)
	if err == nil {
		t.Error("Decrypt truncated ciphertext: expected error, got nil")
	}
}

// TestDecryptInvalidBase64 verifies that malformed base64 input is rejected.
func TestDecryptInvalidBase64(t *testing.T) {
	key := DeriveKey([]byte("base64-test-key-padding-xxxxxxxxx"))

	_, err := Decrypt(key, "!!!not-valid-base64!!!")
	if err == nil {
		t.Error("Decrypt invalid base64: expected error, got nil")
	}
}

// TestDecryptWrongKey verifies that a ciphertext encrypted with one key cannot
// be decrypted with a different key.
func TestDecryptWrongKey(t *testing.T) {
	key1 := DeriveKey([]byte("key-one-padding-xxxxxxxxxxxxxxxxx"))
	key2 := DeriveKey([]byte("key-two-padding-xxxxxxxxxxxxxxxxx"))

	ct, err := Encrypt(key1, "secret payload")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = Decrypt(key2, ct)
	if err == nil {
		t.Error("Decrypt with wrong key: expected error, got nil")
	}
}

// TestEncryptInvalidKeySize verifies that passing a key of the wrong length
// returns an error rather than panicking (AES requires 16, 24, or 32 bytes).
func TestEncryptInvalidKeySize(t *testing.T) {
	_, err := Encrypt([]byte("tooshort"), "data")
	if err == nil {
		t.Error("Encrypt with 8-byte key: expected error, got nil")
	}
}

// TestDecryptEmptyCiphertext checks that an empty string input returns an error.
func TestDecryptEmptyCiphertext(t *testing.T) {
	key := DeriveKey([]byte("empty-cipher-test-key-xxxxxxxxxxx"))

	// Empty string decodes to zero bytes, which is less than nonce size.
	_, err := Decrypt(key, "")
	if err == nil {
		t.Error("Decrypt empty ciphertext: expected error, got nil")
	}
}
