package middleware

import (
	"context"
	"net/http"
)

// contextKey is the unexported integer type used for all context keys in this package.
type contextKey int

const (
	ctxKeyRequestID      contextKey = iota
	ctxKeyUser
	ctxKeyAuthScheme
	ctxKeySubsonicClient
	ctxKeyAmpacheSession
)

// withValue stores a value under key in ctx, returning the new context.
// This thin wrapper keeps context.WithValue calls inside this package.
func withValue(ctx context.Context, key, val any) context.Context {
	return context.WithValue(ctx, key, val)
}

// AuthScheme identifies which authentication method resolved the current request.
type AuthScheme string

const (
	SchemeNative   AuthScheme = "native"
	SchemeSubsonic AuthScheme = "subsonic"
	SchemeAmpache  AuthScheme = "ampache"
)

// AuthUser holds the resolved identity for any supported auth scheme.
type AuthUser struct {
	ID       int64
	Username string
	IsAdmin  bool
	Scheme   AuthScheme
}

// UserFromContext retrieves the authenticated user stored in ctx.
// Returns nil when the request has not been authenticated.
func UserFromContext(ctx context.Context) *AuthUser {
	u, _ := ctx.Value(ctxKeyUser).(*AuthUser)
	return u
}

// WithUser returns a new context that carries u.
func WithUser(ctx context.Context, u *AuthUser) context.Context {
	return withValue(ctx, ctxKeyUser, u)
}

// RequireAuth rejects unauthenticated requests with 401 Unauthorized.
func RequireAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if UserFromContext(r.Context()) == nil {
				w.Header().Set("WWW-Authenticate", `Bearer realm="cassonic"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin rejects requests from authenticated non-admin users with 403 Forbidden.
// It delegates to RequireAuth when no user is present at all.
func RequireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := UserFromContext(r.Context())
			if u == nil {
				w.Header().Set("WWW-Authenticate", `Bearer realm="cassonic"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			if !u.IsAdmin {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SubsonicClientFromContext returns the Subsonic client name stored in ctx.
// Returns an empty string when not set.
func SubsonicClientFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeySubsonicClient).(string)
	return v
}

// Cors sets permissive CORS headers required by the web player and external API clients.
// Preflight OPTIONS requests receive 204 No Content immediately.
func Cors() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Subsonic-Client")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
