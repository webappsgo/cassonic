// Package tor provides a Tor hidden service that forwards to the local HTTP port.
// It uses the bine library to manage the Tor process and onion service lifecycle.
package tor

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	binetор "github.com/cretz/bine/tor"
	"github.com/cretz/bine/torutil/ed25519"
)

// Service manages a Tor hidden service instance.
type Service struct {
	mu      sync.Mutex
	t       *binetор.Tor
	onion   *binetор.OnionService
	keyPath string
	logger  *log.Logger
	addr    string
}

// New creates a Tor service. keyPath is where the ed25519 key is persisted across restarts.
func New(keyPath string, logger *log.Logger) *Service {
	return &Service{
		keyPath: keyPath,
		logger:  logger,
	}
}

// Start starts Tor and publishes a hidden service pointing at localPort.
// If a key file exists at keyPath it is loaded (same .onion address across restarts).
// The key is saved to keyPath (mode 0600) after the first run.
// Returns the .onion address on success.
func (s *Service) Start(ctx context.Context, localPort int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.t != nil {
		return s.addr, nil
	}

	var keyPair ed25519.KeyPair

	if raw, err := os.ReadFile(s.keyPath); err == nil {
		// Decode the persisted hex-encoded 64-byte private key.
		privBytes, decErr := hex.DecodeString(string(raw))
		if decErr == nil && len(privBytes) == 64 {
			keyPair = ed25519.PrivateKey(privBytes).KeyPair()
			s.logger.Printf("tor: loaded key from %s", s.keyPath)
		} else {
			s.logger.Printf("tor: key file corrupt, generating new key")
		}
	}

	s.logger.Printf("tor: starting Tor process...")
	t, err := binetор.Start(ctx, &binetор.StartConf{
		EnableNetwork: true,
	})
	if err != nil {
		return "", fmt.Errorf("tor: start: %w", err)
	}

	listenConf := &binetор.ListenConf{
		RemotePorts: []int{80},
		LocalPort:   localPort,
		Version3:    true,
	}
	if keyPair != nil {
		listenConf.Key = keyPair
	}

	s.logger.Printf("tor: publishing onion service (this may take a minute)...")
	onion, err := t.Listen(ctx, listenConf)
	if err != nil {
		_ = t.Close()
		return "", fmt.Errorf("tor: listen: %w", err)
	}

	// Persist the key when it was newly generated.
	if keyPair == nil && onion.Key != nil {
		if kp, ok := onion.Key.(ed25519.KeyPair); ok {
			if err := s.saveKey(kp.PrivateKey()); err != nil {
				s.logger.Printf("tor: warning: could not save key: %v", err)
			}
		}
	}

	s.t = t
	s.onion = onion
	s.addr = fmt.Sprintf("%s.onion", onion.ID)

	s.logger.Printf("tor: hidden service running at %s", s.addr)
	return s.addr, nil
}

// Stop gracefully shuts down the onion service and Tor process.
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.t == nil {
		return nil
	}

	var firstErr error
	if s.onion != nil {
		if err := s.onion.Close(); err != nil {
			firstErr = fmt.Errorf("tor: close onion: %w", err)
		}
		s.onion = nil
	}
	if err := s.t.Close(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("tor: close tor: %w", err)
	}
	s.t = nil
	s.addr = ""
	return firstErr
}

// Addr returns the current .onion address, or an empty string if not started.
func (s *Service) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// saveKey writes the 64-byte ed25519 private key as hex to keyPath with mode 0600.
func (s *Service) saveKey(priv ed25519.PrivateKey) error {
	if err := os.MkdirAll(filepath.Dir(s.keyPath), 0750); err != nil {
		return err
	}
	encoded := hex.EncodeToString([]byte(priv))
	return os.WriteFile(s.keyPath, []byte(encoded), 0600)
}
