// Package totp wraps RFC 6238 TOTP generation/validation
// (github.com/pquerna/otp) and QR code rendering
// (github.com/skip2/go-qrcode) for the Server Admin 2FA enrollment flow
// (AI.md PART 17 "TOTP Two-Factor Authentication").
package totp

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	qrcode "github.com/skip2/go-qrcode"
)

// BackupCodeCount is the number of one-time recovery codes generated on
// TOTP setup and regeneration (AI.md PART 17 "Backup codes").
const BackupCodeCount = 10

// Enrollment holds a freshly generated, not-yet-confirmed TOTP secret and its
// presentation material. Never persisted before confirmation — threaded
// through the enrollment confirmation form instead (this codebase's
// stateless-wizard convention).
type Enrollment struct {
	// Secret is the base32-encoded TOTP secret (the "manual entry key").
	Secret string
	// QRCodePNGBase64 is the otpauth:// QR code, PNG-encoded and
	// base64-encoded (no data: URI prefix — the template adds that).
	QRCodePNGBase64 string
	// URI is the otpauth:// provisioning URI encoded into the QR code.
	URI string
}

// Generate creates a new TOTP secret for accountName (the admin's username)
// under the given issuer (the server's display name).
func Generate(issuer, accountName string) (*Enrollment, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
	})
	if err != nil {
		return nil, fmt.Errorf("generate totp secret: %w", err)
	}

	png, err := qrcode.Encode(key.String(), qrcode.Medium, 256)
	if err != nil {
		return nil, fmt.Errorf("encode totp qr code: %w", err)
	}

	return &Enrollment{
		Secret:          key.Secret(),
		QRCodePNGBase64: base64.StdEncoding.EncodeToString(png),
		URI:             key.String(),
	}, nil
}

// Validate reports whether code is a currently valid TOTP code for secret.
func Validate(secret, code string) bool {
	if secret == "" || code == "" {
		return false
	}
	ok, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return false
	}
	return ok
}

// GenerateBackupCodes returns BackupCodeCount fresh recovery codes: raw codes
// to show the admin once, and their SHA-256 hex hashes to persist. Raw codes
// are never stored.
func GenerateBackupCodes() (raw []string, hashed []string, err error) {
	raw = make([]string, BackupCodeCount)
	hashed = make([]string, BackupCodeCount)
	for i := range raw {
		code, err := randomBackupCode()
		if err != nil {
			return nil, nil, fmt.Errorf("generate backup code: %w", err)
		}
		raw[i] = code
		hashed[i] = HashBackupCode(code)
	}
	return raw, hashed, nil
}

// HashBackupCode returns the SHA-256 hex hash of a single backup code, for
// storage and for comparing a submitted code against stored hashes.
func HashBackupCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// randomBackupCode returns a 10-character uppercase hex recovery code
// formatted as XXXXX-XXXXX for readability.
func randomBackupCode() (string, error) {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	s := hex.EncodeToString(buf)
	if len(s) < 10 {
		return "", errors.New("short random read")
	}
	return fmt.Sprintf("%s-%s", s[:5], s[5:10]), nil
}
