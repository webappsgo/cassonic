package middleware

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// ampacheHandshakeMaxAge is the maximum drift allowed between the client-supplied
// timestamp and the server clock when verifying a handshake passphrase.
const ampacheHandshakeMaxAge = 300 * time.Second

// AmpacheSession holds an active in-memory Ampache session token with its owner
// and expiry. Sessions are not persisted to SQLite; a server restart clears all
// Ampache sessions (per the Ampache spec).
type AmpacheSession struct {
	Token  string
	UserID int64
	Expiry time.Time
}

// AmpacheSessionStore is a thread-safe in-memory store for Ampache sessions.
type AmpacheSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*AmpacheSession
	ttl      time.Duration
}

// NewAmpacheSessionStore returns an AmpacheSessionStore with the given session TTL.
func NewAmpacheSessionStore(ttl time.Duration) *AmpacheSessionStore {
	return &AmpacheSessionStore{
		sessions: make(map[string]*AmpacheSession),
		ttl:      ttl,
	}
}

// Create generates a random 32-byte hex session token, stores the session, and
// returns the token string.
func (s *AmpacheSessionStore) Create(userID int64) string {
	var raw [32]byte
	_, _ = rand.Read(raw[:])
	token := hex.EncodeToString(raw[:])

	session := &AmpacheSession{
		Token:  token,
		UserID: userID,
		Expiry: time.Now().Add(s.ttl),
	}

	s.mu.Lock()
	s.sessions[token] = session
	s.mu.Unlock()

	return token
}

// Get retrieves the session for the given token. Returns nil when the token is
// unknown or the session has expired.
func (s *AmpacheSessionStore) Get(token string) *AmpacheSession {
	s.mu.RLock()
	session, ok := s.sessions[token]
	s.mu.RUnlock()

	if !ok {
		return nil
	}
	if time.Now().After(session.Expiry) {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		return nil
	}
	return session
}

// Delete removes a session immediately (used by the Ampache goodbye action).
func (s *AmpacheSessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// Extend resets the expiry of the named session to now + ttl.
// If the session does not exist, Extend is a no-op.
func (s *AmpacheSessionStore) Extend(token string, ttl time.Duration) {
	s.mu.Lock()
	if session, ok := s.sessions[token]; ok {
		session.Expiry = time.Now().Add(ttl)
	}
	s.mu.Unlock()
}

// Cleanup removes all sessions whose Expiry is in the past.
func (s *AmpacheSessionStore) Cleanup() {
	now := time.Now()
	s.mu.Lock()
	for token, session := range s.sessions {
		if now.After(session.Expiry) {
			delete(s.sessions, token)
		}
	}
	s.mu.Unlock()
}

// AmpacheAuth validates Ampache session tokens on every non-handshake request.
// The session token is read from the ?auth= query parameter. On success the
// resolved AuthUser is set in the request context. On failure the chain continues
// without auth so that RequireAuth() can enforce access control at the route level.
func AmpacheAuth(users store.UserStore, sessions *AmpacheSessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("auth")
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			session := sessions.Get(token)
			if session == nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			user, err := users.GetUser(ctx, session.UserID)
			if err != nil || user == nil {
				next.ServeHTTP(w, r)
				return
			}

			if user.IsLocked() || !user.IsEnabled {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			ctx = WithUser(ctx, &AuthUser{
				ID:       user.ID,
				Username: user.Username,
				IsAdmin:  user.IsAdmin,
				Scheme:   SchemeAmpache,
			})
			ctx = withValue(ctx, ctxKeyAmpacheSession, session)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// VerifyHandshake verifies an Ampache handshake and returns the authenticated
// user when the passphrase is valid. It accepts both SHA-256 (preferred) and
// MD5 (legacy) passphrase formats and rejects timestamps that differ from the
// server clock by more than 300 seconds.
//
// SHA-256 format: passphrase = SHA256(timestamp + SHA256(password))
// MD5 format:     passphrase = MD5(timestamp  + MD5(password))
//
// getPlainPassword returns the reversibly-stored plaintext password for a
// username. It returns ("", false) when the user has no such password available.
func VerifyHandshake(
	ctx context.Context,
	users store.UserStore,
	getPlainPassword func(ctx context.Context, username string) (string, bool),
	username, passphrase, timestamp string,
) (*model.User, error) {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	drift := time.Since(time.Unix(ts, 0))
	if drift < 0 {
		drift = -drift
	}
	if drift > ampacheHandshakeMaxAge {
		return nil, fmt.Errorf("timestamp expired: drift %s exceeds %s", drift, ampacheHandshakeMaxAge)
	}

	user, err := users.GetUserByUsername(ctx, username)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}

	if user.IsLocked() || !user.IsEnabled {
		return nil, fmt.Errorf("account locked or disabled")
	}

	plainPassword, ok := getPlainPassword(ctx, username)
	if !ok || plainPassword == "" {
		return nil, fmt.Errorf("no reversible password available for Ampache auth")
	}

	// Try SHA-256 first, then fall back to MD5 legacy.
	if verifyAmpacheSHA256(passphrase, timestamp, plainPassword) {
		return user, nil
	}
	if verifyAmpacheMD5(passphrase, timestamp, plainPassword) {
		return user, nil
	}

	return nil, fmt.Errorf("invalid passphrase")
}

// verifyAmpacheSHA256 checks the SHA-256 handshake passphrase.
// expected = SHA256(timestamp + SHA256(password))
func verifyAmpacheSHA256(passphrase, timestamp, password string) bool {
	innerSum := sha256.Sum256([]byte(password))
	inner := hex.EncodeToString(innerSum[:])
	outerSum := sha256.Sum256([]byte(timestamp + inner))
	expected := hex.EncodeToString(outerSum[:])
	return subtle.ConstantTimeCompare([]byte(expected), []byte(passphrase)) == 1
}

// verifyAmpacheMD5 checks the MD5 legacy handshake passphrase.
// expected = MD5(timestamp + MD5(password))
func verifyAmpacheMD5(passphrase, timestamp, password string) bool {
	innerSum := md5.Sum([]byte(password))
	inner := hex.EncodeToString(innerSum[:])
	outerSum := md5.Sum([]byte(timestamp + inner))
	expected := hex.EncodeToString(outerSum[:])
	return subtle.ConstantTimeCompare([]byte(expected), []byte(passphrase)) == 1
}
