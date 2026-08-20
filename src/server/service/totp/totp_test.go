package totp

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestGenerate(t *testing.T) {
	e, err := Generate("cassonic", "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if e.Secret == "" {
		t.Fatalf("expected nonempty secret")
	}
	if e.QRCodePNGBase64 == "" {
		t.Fatalf("expected nonempty QR code")
	}
	if e.URI == "" {
		t.Fatalf("expected nonempty otpauth URI")
	}
}

func TestValidate(t *testing.T) {
	e, err := Generate("cassonic", "alice")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	code, err := totp.GenerateCode(e.Secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if !Validate(e.Secret, code) {
		t.Fatalf("expected valid code to validate")
	}

	if Validate(e.Secret, "000000") {
		t.Fatalf("expected wrong code to fail validation")
	}
	if Validate("", code) {
		t.Fatalf("expected empty secret to fail validation")
	}
	if Validate(e.Secret, "") {
		t.Fatalf("expected empty code to fail validation")
	}
}

func TestGenerateBackupCodes(t *testing.T) {
	raw, hashed, err := GenerateBackupCodes()
	if err != nil {
		t.Fatalf("GenerateBackupCodes: %v", err)
	}
	if len(raw) != BackupCodeCount || len(hashed) != BackupCodeCount {
		t.Fatalf("expected %d codes, got raw=%d hashed=%d", BackupCodeCount, len(raw), len(hashed))
	}

	seen := map[string]bool{}
	for i, code := range raw {
		if seen[code] {
			t.Fatalf("duplicate backup code generated: %s", code)
		}
		seen[code] = true

		if hashed[i] != HashBackupCode(code) {
			t.Fatalf("hash mismatch for backup code %d", i)
		}
	}
}

func TestHashBackupCode(t *testing.T) {
	a := HashBackupCode("ABCDE-12345")
	b := HashBackupCode("ABCDE-12345")
	c := HashBackupCode("different")
	if a != b {
		t.Fatalf("expected deterministic hash")
	}
	if a == c {
		t.Fatalf("expected different codes to hash differently")
	}
}
