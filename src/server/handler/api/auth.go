package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"

	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	cerr "github.com/local/cassonic/src/common/errors"
)

// argon2id cost parameters for password hashing.
const (
	argon2Memory      = 65536
	argon2Iterations  = 3
	argon2Parallelism = 4
	argon2KeyLen      = 32
	argon2SaltLen     = 16
)

// hashPassword hashes a plaintext password with Argon2id.
// Format: $argon2id$v=19$m=65536,t=3,p=4$<base64salt>$<base64hash>
func hashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2id: generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLen)
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedKey := base64.RawStdEncoding.EncodeToString(key)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory, argon2Iterations, argon2Parallelism, encodedSalt, encodedKey), nil
}

// verifyPassword checks candidate against a stored Argon2id hash using constant-time comparison.
func verifyPassword(password, hash string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("argon2id: unsupported hash format")
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
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
	computed := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(storedKey)))
	return subtle.ConstantTimeCompare(computed, storedKey) == 1, nil
}

// generateToken creates a cryptographically random 32-byte token and returns
// both the raw hex string and its SHA-256 hash hex string.
func generateToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("token: rand: %w", err)
	}
	raw = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return raw, hash, nil
}

// loginRequest is the body for POST /api/v1/auth/login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login authenticates a user and creates a session token.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, r, cerr.BadRequest("username and password are required"))
		return
	}

	user, err := h.db.Users.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("lookup failed"))
		return
	}
	if user == nil {
		writeError(w, r, cerr.Unauthorized("invalid credentials"))
		return
	}
	if !user.IsEnabled {
		writeError(w, r, cerr.Unauthorized("account disabled"))
		return
	}
	if user.IsLocked() {
		writeError(w, r, cerr.Unauthorized("account locked"))
		return
	}

	ok, err := verifyPassword(req.Password, user.PasswordHash)
	if err != nil || !ok {
		_ = h.db.Users.IncrementLoginAttempts(r.Context(), user.ID)
		writeError(w, r, cerr.Unauthorized("invalid credentials"))
		return
	}

	raw, tokenHash, err := generateToken()
	if err != nil {
		writeError(w, r, cerr.InternalServerError("token generation failed"))
		return
	}

	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)
	clientName := r.Header.Get("X-Client-Name")
	if clientName == "" {
		clientName = "web"
	}

	if err := h.db.Users.CreateSession(r.Context(), user.ID, tokenHash, expiresAt, clientName); err != nil {
		writeError(w, r, cerr.InternalServerError("session creation failed"))
		return
	}

	_ = h.db.Users.ResetLoginAttempts(r.Context(), user.ID)
	_ = h.db.Users.UpdateLastLogin(r.Context(), user.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      raw,
		"expires_at": expiresAt.Format(time.RFC3339),
		"user":       safeUser(user),
	})
}

// Logout invalidates the current session token.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeError(w, r, cerr.Unauthorized("missing bearer token"))
		return
	}
	raw := strings.TrimPrefix(authHeader, "Bearer ")
	sum := sha256.Sum256([]byte(raw))
	tokenHash := hex.EncodeToString(sum[:])

	if err := h.db.Users.DeleteSession(r.Context(), tokenHash); err != nil {
		writeError(w, r, cerr.InternalServerError("session deletion failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// createTokenRequest is the body for POST /api/v1/auth/tokens.
type createTokenRequest struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
}

// CreateToken generates a new long-lived API token for the authenticated user.
// The raw token is returned only on creation and never again.
func (h *Handler) CreateToken(w http.ResponseWriter, r *http.Request) {
	u := mw.UserFromContext(r.Context())
	if u == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}
	if req.Name == "" {
		writeError(w, r, cerr.BadRequest("name is required"))
		return
	}

	var expiresAt time.Time
	if req.ExpiresAt != "" {
		var err error
		expiresAt, err = time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			writeError(w, r, cerr.BadRequest("invalid expires_at format; use RFC3339"))
			return
		}
	}

	raw, tokenHash, err := generateToken()
	if err != nil {
		writeError(w, r, cerr.InternalServerError("token generation failed"))
		return
	}

	token := &model.APIToken{
		UserID:    u.ID,
		TokenHash: tokenHash,
		Name:      req.Name,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}

	if err := h.db.Users.CreateAPIToken(r.Context(), token); err != nil {
		writeError(w, r, cerr.InternalServerError("token creation failed"))
		return
	}

	tokens, _ := h.db.Users.ListAPITokens(r.Context(), u.ID)
	var createdID int64
	for _, t := range tokens {
		if t.TokenHash == tokenHash {
			createdID = t.ID
			break
		}
	}

	resp := map[string]any{
		"id":   createdID,
		"token": raw,
		"name": req.Name,
	}
	if !expiresAt.IsZero() {
		resp["expires_at"] = expiresAt.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusCreated, resp)
}

// DeleteToken removes an API token owned by the authenticated user.
func (h *Handler) DeleteToken(w http.ResponseWriter, r *http.Request) {
	u := mw.UserFromContext(r.Context())
	if u == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	tokenID, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid token id"))
		return
	}

	tokens, err := h.db.Users.ListAPITokens(r.Context(), u.ID)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("token lookup failed"))
		return
	}

	found := false
	for _, t := range tokens {
		if t.ID == tokenID {
			found = true
			break
		}
	}
	if !found {
		writeError(w, r, cerr.NotFound("token not found"))
		return
	}

	if err := h.db.Users.DeleteAPIToken(r.Context(), tokenID); err != nil {
		writeError(w, r, cerr.InternalServerError("token deletion failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}
