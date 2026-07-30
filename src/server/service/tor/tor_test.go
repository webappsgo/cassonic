package tor

// Tests cover:
//   - New: returns a usable Service with the given keyPath and initial
//     Addr()=="" state
//   - Addr: returns "" before Start has ever run
//   - Stop: no-op (returns nil) when the service was never started
//   - Start: the cached-address early-return branch (s.t != nil), exercised
//     via direct same-package struct construction so no real Tor binary or
//     network is required
//   - Start: the key-loading branch that runs BEFORE binetор.Start is
//     invoked (os.ReadFile + hex.DecodeString + length check). The build
//     environment has no "tor" executable on PATH, so bine's
//     exec.Cmd.Start() fails immediately (no network access is attempted)
//     and Start() returns an error right after logging whether the key was
//     loaded or was corrupt/absent. A short context timeout bounds the call
//     in case a "tor" binary IS present on some other machine running these
//     tests, so the suite never hangs either way.
//   - saveKey: round trip (hex-encoded 64-byte key on disk, 0600
//     permissions, parent directory created on demand), that saveKey
//     overwrites a previous key file, and its two filesystem error paths
//     (MkdirAll failing because a path component is a plain file, and
//     WriteFile failing because keyPath itself is a directory)
//
// NOT covered, and why:
//   - Start()'s success path from binetор.Start(...) onward (t.Listen,
//     onion.ID/onion.Key handling, the "hidden service running at %s" log
//     line): requires a real Tor binary that successfully bootstraps a
//     circuit and publishes a v3 onion descriptor to the live Tor network.
//     *bine/tor.Tor and *bine/tor.OnionService are concrete structs with no
//     interface seam in this package, so they cannot be faked/mocked
//     in-process; only a real subprocess + real network can drive that
//     code.
//   - Stop()'s real teardown path (s.onion.Close() / s.t.Close(), the error
//     wrapping "tor: close onion: %w" / "tor: close tor: %w", and the
//     first-error-wins ordering when both Close calls fail): same reason —
//     s.onion and s.t are only ever non-nil after a real bine Tor/onion
//     service was constructed by the library itself; there is no
//     constructor seam in this package to hand it a fake with a controlled
//     Close() error.

import (
	"context"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	binetор "github.com/cretz/bine/tor"
	"github.com/cretz/bine/torutil/ed25519"
)

// tempDir creates a temp directory under /tmp/local/cassonic-XXXXXX and
// registers cleanup, matching repo convention.
func tempDir(t *testing.T) string {
	t.Helper()
	base := "/tmp/local"
	if err := os.MkdirAll(base, 0750); err != nil {
		t.Fatalf("tempDir: mkdir %s: %v", base, err)
	}
	dir, err := os.MkdirTemp(base, "cassonic-")
	if err != nil {
		t.Fatalf("tempDir: mkdirtemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// --- New / Addr ---

func TestNewReturnsUsableService(t *testing.T) {
	dir := tempDir(t)
	keyPath := filepath.Join(dir, "onion.key")

	svc := New(keyPath, nil)
	if svc == nil {
		t.Fatal("New: returned nil")
	}
	if svc.keyPath != keyPath {
		t.Errorf("New: keyPath got %q, want %q", svc.keyPath, keyPath)
	}
	if svc.Addr() != "" {
		t.Errorf("Addr: got %q, want empty string before Start", svc.Addr())
	}
}

// --- Stop: no-op ---

func TestStopNeverStartedIsNoOp(t *testing.T) {
	svc := New(filepath.Join(tempDir(t), "onion.key"), nil)
	if err := svc.Stop(); err != nil {
		t.Errorf("Stop: expected nil for never-started service, got %v", err)
	}
	// Idempotent: calling Stop again must still be a safe no-op.
	if err := svc.Stop(); err != nil {
		t.Errorf("Stop (second call): expected nil, got %v", err)
	}
}

// --- Start: cached-address early return ---

func TestStartReturnsCachedAddrWithoutTouchingTor(t *testing.T) {
	// s.t is non-nil (a zero-value *bine/tor.Tor, never actually started)
	// so Start must take the early-return branch and hand back s.addr
	// without dereferencing or calling anything on s.t.
	svc := &Service{
		t:    &binetор.Tor{},
		addr: "abcdefghijklmnop.onion",
	}

	addr, err := svc.Start(context.Background(), 8080)
	if err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}
	if addr != "abcdefghijklmnop.onion" {
		t.Errorf("Start: got %q, want cached %q", addr, "abcdefghijklmnop.onion")
	}
	if svc.Addr() != "abcdefghijklmnop.onion" {
		t.Errorf("Addr after Start: got %q, want %q", svc.Addr(), "abcdefghijklmnop.onion")
	}
}

// --- saveKey ---

func TestSaveKeyRoundTrip(t *testing.T) {
	dir := tempDir(t)
	keyPath := filepath.Join(dir, "nested", "onion.key")

	svc := &Service{keyPath: keyPath}

	priv := make(ed25519.PrivateKey, 64)
	for i := range priv {
		priv[i] = byte(i)
	}

	if err := svc.saveKey(priv); err != nil {
		t.Fatalf("saveKey: %v", err)
	}

	raw, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	decoded, err := hex.DecodeString(string(raw))
	if err != nil {
		t.Fatalf("hex.DecodeString: %v", err)
	}
	if len(decoded) != 64 {
		t.Fatalf("saveKey: decoded key length %d, want 64", len(decoded))
	}
	for i, b := range decoded {
		if b != byte(i) {
			t.Fatalf("saveKey: byte %d mismatch: got %d, want %d", i, b, byte(i))
		}
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(keyPath)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("saveKey: perm got %v, want %v", info.Mode().Perm(), os.FileMode(0600))
		}
	}
}

func TestSaveKeyOverwritesExisting(t *testing.T) {
	dir := tempDir(t)
	keyPath := filepath.Join(dir, "onion.key")
	svc := &Service{keyPath: keyPath}

	first := make(ed25519.PrivateKey, 64)
	for i := range first {
		first[i] = 1
	}
	second := make(ed25519.PrivateKey, 64)
	for i := range second {
		second[i] = 2
	}

	if err := svc.saveKey(first); err != nil {
		t.Fatalf("saveKey (first): %v", err)
	}
	if err := svc.saveKey(second); err != nil {
		t.Fatalf("saveKey (second): %v", err)
	}

	raw, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	decoded, err := hex.DecodeString(string(raw))
	if err != nil {
		t.Fatalf("hex.DecodeString: %v", err)
	}
	for i, b := range decoded {
		if b != 2 {
			t.Fatalf("saveKey: byte %d got %d, want overwritten value 2", i, b)
		}
	}
}

// --- saveKey: filesystem error paths ---

func TestSaveKeyMkdirAllErrorWhenParentIsFile(t *testing.T) {
	dir := tempDir(t)
	// Create a plain file where saveKey needs a directory.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0644); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}
	keyPath := filepath.Join(blocker, "nested", "onion.key")
	svc := &Service{keyPath: keyPath}

	priv := make(ed25519.PrivateKey, 64)
	if err := svc.saveKey(priv); err == nil {
		t.Fatal("saveKey: expected error when a path component is a file, got nil")
	}
}

func TestSaveKeyWriteFileErrorWhenKeyPathIsDir(t *testing.T) {
	dir := tempDir(t)
	// keyPath itself is a directory, so os.WriteFile must fail.
	keyPath := filepath.Join(dir, "onion.key")
	if err := os.MkdirAll(keyPath, 0750); err != nil {
		t.Fatalf("setup MkdirAll: %v", err)
	}
	svc := &Service{keyPath: keyPath}

	priv := make(ed25519.PrivateKey, 64)
	if err := svc.saveKey(priv); err == nil {
		t.Fatal("saveKey: expected error when keyPath is a directory, got nil")
	}
}

// --- Start: key-loading branch (runs before binetор.Start) ---

// startWithTimeout calls Start with a short-lived context so the call
// returns quickly regardless of whether binetор.Start fails immediately
// (no "tor" executable, the expected case in the build environment) or
// attempts real network access (bounded by the timeout as a safety net).
func startWithTimeout(t *testing.T, svc *Service) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return svc.Start(ctx, 8080)
}

func TestStartLogsCorruptKeyFile(t *testing.T) {
	dir := tempDir(t)
	keyPath := filepath.Join(dir, "onion.key")
	if err := os.WriteFile(keyPath, []byte("not-valid-hex!!"), 0600); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}

	var buf strings.Builder
	svc := &Service{keyPath: keyPath, logger: log.New(&buf, "", 0)}

	if _, err := startWithTimeout(t, svc); err == nil {
		t.Fatal("Start: expected error (no real Tor binary/network available), got nil")
	}
	if !strings.Contains(buf.String(), "key file corrupt, generating new key") {
		t.Errorf("Start: expected log to mention corrupt key file, got %q", buf.String())
	}
}

func TestStartLogsCorruptKeyFileWrongLength(t *testing.T) {
	dir := tempDir(t)
	keyPath := filepath.Join(dir, "onion.key")
	// Valid hex, but not 64 decoded bytes.
	if err := os.WriteFile(keyPath, []byte(hex.EncodeToString([]byte("short"))), 0600); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}

	var buf strings.Builder
	svc := &Service{keyPath: keyPath, logger: log.New(&buf, "", 0)}

	if _, err := startWithTimeout(t, svc); err == nil {
		t.Fatal("Start: expected error (no real Tor binary/network available), got nil")
	}
	if !strings.Contains(buf.String(), "key file corrupt, generating new key") {
		t.Errorf("Start: expected log to mention corrupt key file, got %q", buf.String())
	}
}

func TestStartLogsLoadedKey(t *testing.T) {
	dir := tempDir(t)
	keyPath := filepath.Join(dir, "onion.key")

	priv := make(ed25519.PrivateKey, 64)
	for i := range priv {
		priv[i] = byte(i)
	}
	if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(priv)), 0600); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}

	var buf strings.Builder
	svc := &Service{keyPath: keyPath, logger: log.New(&buf, "", 0)}

	if _, err := startWithTimeout(t, svc); err == nil {
		t.Fatal("Start: expected error (no real Tor binary/network available), got nil")
	}
	if !strings.Contains(buf.String(), "loaded key from "+keyPath) {
		t.Errorf("Start: expected log to mention loaded key from %q, got %q", keyPath, buf.String())
	}
}

func TestStartMissingKeyFileSkipsLoad(t *testing.T) {
	dir := tempDir(t)
	// keyPath does not exist: os.ReadFile fails, so neither the
	// "loaded key" nor "corrupt" branch should log anything about the key.
	keyPath := filepath.Join(dir, "does-not-exist.key")

	var buf strings.Builder
	svc := &Service{keyPath: keyPath, logger: log.New(&buf, "", 0)}

	if _, err := startWithTimeout(t, svc); err == nil {
		t.Fatal("Start: expected error (no real Tor binary/network available), got nil")
	}
	if strings.Contains(buf.String(), "loaded key from") || strings.Contains(buf.String(), "key file corrupt") {
		t.Errorf("Start: expected no key-related log line for a missing key file, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "starting Tor process") {
		t.Errorf("Start: expected log to reach the tor-start line, got %q", buf.String())
	}
}
