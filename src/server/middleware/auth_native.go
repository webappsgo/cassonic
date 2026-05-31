package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/local/cassonic/src/server/store"
)

// NativeAuth authenticates requests using Bearer tokens.
// Extracts a token from the Authorization: Bearer <token> header, falling back
// to the ?token= query parameter. On success the resolved AuthUser is stored in
// the request context. On failure the request continues unauthenticated so that
// RequireAuth() can enforce the policy at the route level.
func NativeAuth(users store.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			raw := sha256.Sum256([]byte(token))
			hashHex := hex.EncodeToString(raw[:])

			ctx := r.Context()

			// Attempt session lookup first.
			session, err := users.GetSessionByHash(ctx, hashHex)
			if err == nil && session != nil {
				if session.IsExpired() {
					// Delete stale session; continue without auth.
					_ = users.DeleteSession(ctx, hashHex)
					next.ServeHTTP(w, r)
					return
				}

				user, err := users.GetUser(ctx, session.UserID)
				if err != nil || user == nil {
					next.ServeHTTP(w, r)
					return
				}

				if user.IsLocked() || !user.IsEnabled {
					w.Header().Set("WWW-Authenticate", `Bearer realm="cassonic"`)
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				_ = users.UpdateLastLogin(ctx, user.ID)

				ctx = WithUser(ctx, &AuthUser{
					ID:       user.ID,
					Username: user.Username,
					IsAdmin:  user.IsAdmin,
					Scheme:   SchemeNative,
				})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Fall through to API token lookup.
			apiToken, err := users.GetAPITokenByHash(ctx, hashHex)
			if err != nil || apiToken == nil {
				next.ServeHTTP(w, r)
				return
			}

			if apiToken.IsExpired() {
				next.ServeHTTP(w, r)
				return
			}

			user, err := users.GetUser(ctx, apiToken.UserID)
			if err != nil || user == nil {
				next.ServeHTTP(w, r)
				return
			}

			if user.IsLocked() || !user.IsEnabled {
				w.Header().Set("WWW-Authenticate", `Bearer realm="cassonic"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			_ = users.UpdateAPITokenLastUsed(ctx, apiToken.ID)

			ctx = WithUser(ctx, &AuthUser{
				ID:       user.ID,
				Username: user.Username,
				IsAdmin:  user.IsAdmin,
				Scheme:   SchemeNative,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractBearerToken returns the raw token from Authorization: Bearer <token>,
// falling back to the ?token= query parameter. Returns an empty string when
// neither source provides a value. The comparison against "bearer" uses a
// case-insensitive prefix check per RFC 7235.
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		const prefix = "bearer "
		if len(authHeader) > len(prefix) && strings.EqualFold(authHeader[:len(prefix)], prefix) {
			candidate := strings.TrimSpace(authHeader[len(prefix):])
			if candidate != "" {
				// Verify the extracted token matches the header value via constant-time
				// comparison to avoid any timing side-channel on this extraction path.
				expected := []byte(authHeader[len(prefix):])
				got := []byte(candidate)
				if subtle.ConstantTimeCompare(expected, got) == 1 {
					return candidate
				}
			}
		}
	}

	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}

	return ""
}
