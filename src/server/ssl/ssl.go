// Package ssl manages TLS configuration for the cassonic server.
// It supports Let's Encrypt via autocert and local cert/key file pairs.
// TLS 1.2 is the minimum supported version per PART 15.
package ssl

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/crypto/acme/autocert"
)

// Config describes TLS settings for the server.
type Config struct {
	// Enabled controls whether TLS is active.
	Enabled bool

	// Domain is the FQDN used for Let's Encrypt certificate issuance.
	// Required when LocalCert/LocalKey are not set.
	Domain string

	// Email is the contact address for ACME registration.
	Email string

	// CertDir is the directory where autocert caches its certificates.
	// Defaults to {config_dir}/ssl/letsencrypt/.
	CertDir string

	// LocalCert is the path to a manually provisioned PEM certificate file.
	LocalCert string

	// LocalKey is the path to the matching PEM private key file.
	LocalKey string
}

// Manager holds a configured TLS setup and the optional autocert manager.
type Manager struct {
	cfg  Config
	auto *autocert.Manager
}

// New creates a Manager from cfg. Returns nil when SSL is disabled.
func New(cfg Config) *Manager {
	if !cfg.Enabled {
		return nil
	}
	m := &Manager{cfg: cfg}

	// Prefer local cert/key when both are provided.
	if cfg.LocalCert != "" && cfg.LocalKey != "" {
		return m
	}

	// Fall back to Let's Encrypt autocert when a domain is provided.
	if cfg.Domain != "" {
		cacheDir := cfg.CertDir
		if cacheDir == "" {
			cacheDir = filepath.Join(os.TempDir(), "cassonic-ssl-cache")
		}
		m.auto = &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(cfg.Domain),
			Cache:      autocert.DirCache(cacheDir),
			Email:      cfg.Email,
		}
	}

	return m
}

// TLSConfig returns a *tls.Config suitable for http.Server.
// Returns nil when the receiver is nil (SSL disabled).
func (m *Manager) TLSConfig() *tls.Config {
	if m == nil {
		return nil
	}

	// Build a safe cipher-suite list by filtering the Go default set.
	safe := safeCipherSuites()

	if m.auto != nil {
		tc := m.auto.TLSConfig()
		tc.MinVersion = tls.VersionTLS12
		tc.CipherSuites = safe
		return tc
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		CipherSuites: safe,
	}
}

// CertFiles returns the local certificate and key paths when local certs are
// configured. Returns empty strings when autocert is in use.
func (m *Manager) CertFiles() (certFile, keyFile string) {
	if m == nil {
		return "", ""
	}
	if m.auto != nil {
		return "", ""
	}
	return m.cfg.LocalCert, m.cfg.LocalKey
}

// HTTPHandler wraps h so that plain-HTTP requests are redirected to HTTPS.
// Returns h unchanged when the receiver is nil (SSL disabled).
func (m *Manager) HTTPHandler(h http.Handler) http.Handler {
	if m == nil {
		return h
	}
	if m.auto != nil {
		return m.auto.HTTPHandler(h)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := fmt.Sprintf("https://%s%s", r.Host, r.RequestURI)
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}

// safeCipherSuites returns the IDs from tls.CipherSuites() — Go's list of
// non-insecure suites — which excludes RC4, 3DES, and export ciphers.
func safeCipherSuites() []uint16 {
	suites := tls.CipherSuites()
	ids := make([]uint16, 0, len(suites))
	for _, s := range suites {
		ids = append(ids, s.ID)
	}
	return ids
}
