package ssl

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeSelfSignedCert generates a self-signed ECDSA certificate and writes
// both the cert and key to dir, returning their paths.
func writeSelfSignedCert(t *testing.T, dir string) (certFile, keyFile string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")

	cf, err := os.Create(certFile)
	if err != nil {
		t.Fatalf("create cert file: %v", err)
	}
	if err := pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		t.Fatalf("encode cert pem: %v", err)
	}
	cf.Close()

	kf, err := os.Create(keyFile)
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	if err := pem.Encode(kf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		t.Fatalf("encode key pem: %v", err)
	}
	kf.Close()

	return certFile, keyFile
}

// --- New() disabled path ---

// New with Enabled=false must return nil (SSL is disabled).
func TestNew_Disabled_ReturnsNil(t *testing.T) {
	m := New(Config{Enabled: false})
	if m != nil {
		t.Errorf("New(Enabled=false): got non-nil Manager, want nil")
	}
}

// --- TLSConfig() disabled path ---

// TLSConfig on a nil Manager returns nil (method is nil-safe).
func TestTLSConfig_NilManager_ReturnsNil(t *testing.T) {
	var m *Manager
	if tc := m.TLSConfig(); tc != nil {
		t.Errorf("(*Manager)(nil).TLSConfig(): got non-nil *tls.Config, want nil")
	}
}

// TLSConfig on a Manager created with Enabled=false is nil — covered by
// TestNew_Disabled_ReturnsNil, but we also verify via the zero value path.
func TestTLSConfig_Disabled_IsNil(t *testing.T) {
	m := New(Config{Enabled: false})
	// New returns nil, so TLSConfig() on it must return nil.
	if tc := m.TLSConfig(); tc != nil {
		t.Errorf("disabled Manager.TLSConfig(): got non-nil, want nil")
	}
}

// --- CertFiles() disabled path ---

// CertFiles on a nil Manager returns ("", "").
func TestCertFiles_NilManager_ReturnsBoth_Empty(t *testing.T) {
	var m *Manager
	cert, key := m.CertFiles()
	if cert != "" || key != "" {
		t.Errorf("(*Manager)(nil).CertFiles(): got (%q, %q), want (\"\", \"\")", cert, key)
	}
}

// --- New() with Enabled=true and empty Domain ---

// New with Enabled=true but no Domain and no LocalCert/LocalKey returns a Manager
// that has no autocert configured. TLSConfig still returns a non-nil config,
// but CertFiles returns empty strings (no local certs either).
func TestNew_EnabledNoDomain_NoLocalCert_SignalsMisconfiguration(t *testing.T) {
	m := New(Config{Enabled: true})
	if m == nil {
		t.Fatal("New(Enabled=true, no Domain): got nil Manager, want non-nil")
	}

	// TLSConfig must still return a valid (if incomplete) config — not panic.
	tc := m.TLSConfig()
	if tc == nil {
		t.Error("Manager with no domain/certs: TLSConfig() returned nil, want non-nil")
	}

	// CertFiles must signal there are no local certs.
	cert, key := m.CertFiles()
	if cert != "" || key != "" {
		t.Errorf("Manager with no domain/certs: CertFiles() = (%q, %q), want (\"\", \"\")", cert, key)
	}
}

// --- TLSConfig() with local cert/key ---

// TLSConfig with valid LocalCert/LocalKey returns a non-nil *tls.Config.
func TestTLSConfig_LocalCert_ReturnsNonNil(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeSelfSignedCert(t, dir)

	m := New(Config{
		Enabled:   true,
		LocalCert: certFile,
		LocalKey:  keyFile,
	})
	if m == nil {
		t.Fatal("New with LocalCert/LocalKey: got nil Manager")
	}

	tc := m.TLSConfig()
	if tc == nil {
		t.Fatal("TLSConfig() with local cert: got nil *tls.Config")
	}
}

// TLSConfig with local certs must set MinVersion = tls.VersionTLS12.
func TestTLSConfig_LocalCert_MinVersionIsTLS12(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeSelfSignedCert(t, dir)

	m := New(Config{
		Enabled:   true,
		LocalCert: certFile,
		LocalKey:  keyFile,
	})

	tc := m.TLSConfig()
	if tc == nil {
		t.Fatal("TLSConfig() returned nil")
	}
	if tc.MinVersion != tls.VersionTLS12 {
		t.Errorf("TLSConfig().MinVersion = 0x%04x, want 0x%04x (TLS 1.2)",
			tc.MinVersion, tls.VersionTLS12)
	}
}

// TLSConfig cipher suite list must be non-empty (safe suites applied).
func TestTLSConfig_LocalCert_CipherSuitesNonEmpty(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeSelfSignedCert(t, dir)

	m := New(Config{
		Enabled:   true,
		LocalCert: certFile,
		LocalKey:  keyFile,
	})

	tc := m.TLSConfig()
	if tc == nil {
		t.Fatal("TLSConfig() returned nil")
	}
	if len(tc.CipherSuites) == 0 {
		t.Error("TLSConfig().CipherSuites is empty, want at least one safe suite")
	}
}

// --- CertFiles() with local cert/key ---

// CertFiles returns the configured paths when local certs are set.
func TestCertFiles_LocalCert_ReturnsPaths(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeSelfSignedCert(t, dir)

	m := New(Config{
		Enabled:   true,
		LocalCert: certFile,
		LocalKey:  keyFile,
	})

	gotCert, gotKey := m.CertFiles()
	if gotCert != certFile {
		t.Errorf("CertFiles() cert = %q, want %q", gotCert, certFile)
	}
	if gotKey != keyFile {
		t.Errorf("CertFiles() key = %q, want %q", gotKey, keyFile)
	}
}

// CertFiles returns ("", "") when autocert is configured (domain path).
func TestCertFiles_AutocertDomain_ReturnsEmpty(t *testing.T) {
	m := New(Config{
		Enabled: true,
		Domain:  "example.com",
		CertDir: t.TempDir(),
	})
	if m == nil {
		t.Fatal("New with Domain: got nil Manager")
	}

	cert, key := m.CertFiles()
	if cert != "" || key != "" {
		t.Errorf("CertFiles() with autocert: got (%q, %q), want (\"\", \"\")", cert, key)
	}
}

// --- autocert path: TLSConfig MinVersion ---

// TLSConfig on an autocert Manager must also enforce TLS 1.2 minimum.
func TestTLSConfig_Autocert_MinVersionIsTLS12(t *testing.T) {
	m := New(Config{
		Enabled: true,
		Domain:  "example.com",
		CertDir: t.TempDir(),
	})
	if m == nil {
		t.Fatal("New with Domain: got nil Manager")
	}

	tc := m.TLSConfig()
	if tc == nil {
		t.Fatal("TLSConfig() with autocert: got nil")
	}
	if tc.MinVersion != tls.VersionTLS12 {
		t.Errorf("autocert TLSConfig().MinVersion = 0x%04x, want 0x%04x (TLS 1.2)",
			tc.MinVersion, tls.VersionTLS12)
	}
}

// --- LocalCert takes precedence over Domain ---

// When both LocalCert/LocalKey and Domain are provided, local cert wins:
// CertFiles returns paths and auto is nil.
func TestNew_LocalCertTakesPrecedenceOverDomain(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeSelfSignedCert(t, dir)

	m := New(Config{
		Enabled:   true,
		Domain:    "example.com",
		LocalCert: certFile,
		LocalKey:  keyFile,
	})
	if m == nil {
		t.Fatal("New: got nil Manager")
	}

	gotCert, gotKey := m.CertFiles()
	if gotCert != certFile || gotKey != keyFile {
		t.Errorf("CertFiles() = (%q, %q), want (%q, %q)", gotCert, gotKey, certFile, keyFile)
	}
}

// --- safeCipherSuites ---

// safeCipherSuites must return only IDs present in tls.CipherSuites().
func TestSafeCipherSuites_ContainsOnlySecureSuites(t *testing.T) {
	secure := tls.CipherSuites()
	secureIDs := make(map[uint16]bool, len(secure))
	for _, s := range secure {
		secureIDs[s.ID] = true
	}

	got := safeCipherSuites()
	if len(got) == 0 {
		t.Fatal("safeCipherSuites() returned empty slice")
	}
	for _, id := range got {
		if !secureIDs[id] {
			t.Errorf("safeCipherSuites() contains ID 0x%04x which is not in tls.CipherSuites()", id)
		}
	}
}

// safeCipherSuites must NOT include IDs from tls.InsecureCipherSuites().
func TestSafeCipherSuites_ExcludesInsecureSuites(t *testing.T) {
	insecure := tls.InsecureCipherSuites()
	insecureIDs := make(map[uint16]bool, len(insecure))
	for _, s := range insecure {
		insecureIDs[s.ID] = true
	}

	for _, id := range safeCipherSuites() {
		if insecureIDs[id] {
			t.Errorf("safeCipherSuites() contains insecure suite ID 0x%04x", id)
		}
	}
}

// --- HTTPHandler ---

// sentinelHandler is a named struct so it can be compared by pointer identity.
type sentinelHandler struct{}

func (sentinelHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

// HTTPHandler on a nil Manager returns the original handler unchanged (same pointer).
func TestHTTPHandler_NilManager_ReturnsOriginal(t *testing.T) {
	var m *Manager
	original := sentinelHandler{}
	got := m.HTTPHandler(original)
	if got != original {
		t.Error("(*Manager)(nil).HTTPHandler(h): returned different handler, want same")
	}
}
