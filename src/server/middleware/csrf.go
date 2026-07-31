package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

// csrfCookieName is the double-submit CSRF token cookie (PART 16 -> "CSRF Protection").
const csrfCookieName = "csrf_token"

// csrfHeaderName is the request header carrying the CSRF token for JSON/XHR requests.
const csrfHeaderName = "X-CSRF-Token"

// csrfFieldName is the form field carrying the CSRF token for native HTML form submits.
const csrfFieldName = "csrf_token"

// csrfTokenLength is the byte length of a generated CSRF token (32 bytes per spec).
const csrfTokenLength = 32

// CSRFConfig configures the CSRF double-submit cookie protection middleware.
type CSRFConfig struct {
	// Enabled toggles CSRF validation. Set false only for API-only deployments
	// with no browser forms at all.
	Enabled bool
	// Secure marks the CSRF cookie Secure; should be true when serving HTTPS.
	Secure bool
	// SessionCookieName is the session cookie whose presence marks a request as
	// cookie-authenticated; CSRF validation only applies to such requests.
	SessionCookieName string
	// ExemptPaths are operator-declared glob patterns (path.Match syntax)
	// exempt from validation, e.g. OAuth callbacks and webhook receivers.
	ExemptPaths []string
}

// CSRF returns middleware implementing the double-submit cookie CSRF pattern
// (PART 16 -> "CSRF Protection"). It issues a per-browser CSRF token cookie on
// every response and validates state-changing (POST/PUT/PATCH/DELETE) requests
// that authenticate via session cookie and originate cross-site, per the
// "When CSRF Validation Runs" bypass rules.
func CSRF(cfg CSRFConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			token, isNew := csrfTokenFromRequest(r)
			if isNew {
				http.SetCookie(w, &http.Cookie{
					Name:     csrfCookieName,
					Value:    token,
					Path:     "/",
					HttpOnly: false,
					Secure:   cfg.Secure,
					SameSite: http.SameSiteStrictMode,
				})
				// Make the freshly issued token visible to r.Cookie() for the
				// rest of this request (e.g. the WebUI template renderer),
				// since http.SetCookie only affects the outgoing response.
				r.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
			}
			r = r.WithContext(withValue(r.Context(), ctxKeyCSRFToken, token))

			if csrfValidationRequired(r, cfg) && !csrfValidate(r, token) {
				log.Printf("security.csrf_failure ip=%s path=%s method=%s", extractIP(r), r.URL.Path, r.Method)
				writeCSRFFailure(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CSRFTokenFromContext retrieves the CSRF token issued for this request, for
// embedding into server-rendered forms as a hidden csrf_token input.
func CSRFTokenFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyCSRFToken).(string)
	return v
}

// ResetCSRFCookie clears the CSRF token cookie so the next request issues a
// fresh token. Call on login, logout, and privilege change to prevent
// session fixation of the CSRF token (PART 16 -> "CSRF Protection" ->
// "Implementation Rules").
func ResetCSRFCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// csrfTokenFromRequest returns the CSRF token to use for this request: the
// existing cookie value if present and well-formed, otherwise a freshly
// generated token (isNew == true, meaning the caller must set the cookie).
func csrfTokenFromRequest(r *http.Request) (token string, isNew bool) {
	if c, err := r.Cookie(csrfCookieName); err == nil && len(c.Value) == csrfTokenLength*2 {
		return c.Value, false
	}
	return generateCSRFToken(), true
}

// generateCSRFToken returns a new random 32-byte CSRF token, hex-encoded.
func generateCSRFToken() string {
	b := make([]byte, csrfTokenLength)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// csrfMethodRequiresValidation reports whether r's HTTP method can change
// state; read-only methods (GET, HEAD, OPTIONS) are safe per RFC 9110.
func csrfMethodRequiresValidation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// csrfHasBearerAuth reports whether r carries a Bearer or API-token
// credential. Such credentials are never auto-attached by browsers, so
// cross-site forgery has no vector and CSRF validation does not apply.
func csrfHasBearerAuth(r *http.Request) bool {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return true
	}
	return r.Header.Get("X-API-Token") != ""
}

// csrfIsCrossSite reports whether r's Sec-Fetch-Site header, or its Origin
// (falling back to Referer when Origin is absent), indicates a cross-site or
// unknown source. An unknown source is treated as cross-site (safe default).
func csrfIsCrossSite(r *http.Request) bool {
	if sfs := r.Header.Get("Sec-Fetch-Site"); sfs == "cross-site" {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Header.Get("Referer")
	}
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return true
	}
	return !strings.EqualFold(u.Host, r.Host)
}

// csrfExempt reports whether path matches one of the operator-declared exempt
// path glob patterns (web.csrf.exempt_paths).
func csrfExempt(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if ok, err := filepath.Match(pattern, path); err == nil && ok {
			return true
		}
	}
	return false
}

// csrfValidationRequired reports whether r must pass CSRF token validation,
// per the "When CSRF Validation Runs" / bypass rules (PART 16 -> "CSRF Protection").
func csrfValidationRequired(r *http.Request, cfg CSRFConfig) bool {
	if !csrfMethodRequiresValidation(r.Method) {
		return false
	}
	if csrfHasBearerAuth(r) {
		return false
	}
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}
	if csrfExempt(r.URL.Path, cfg.ExemptPaths) {
		return false
	}
	if _, err := r.Cookie(cfg.SessionCookieName); err != nil {
		// No session cookie: the request is not cookie-authenticated, so
		// there is no auto-attached credential to forge.
		return false
	}
	return csrfIsCrossSite(r)
}

// csrfValidate compares the CSRF cookie token against the value submitted in
// the X-CSRF-Token header (JSON/XHR requests) or, if absent, the csrf_token
// form field (native HTML form submits) — the double-submit cookie pattern.
func csrfValidate(r *http.Request, cookieToken string) bool {
	submitted := r.Header.Get(csrfHeaderName)
	if submitted == "" {
		submitted = r.PostFormValue(csrfFieldName)
	}
	if submitted == "" || cookieToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(submitted), []byte(cookieToken)) == 1
}

// writeCSRFFailure writes the canonical 403 CSRF failure response
// (PART 16 -> "CSRF Protection" -> "Implementation Rules").
func writeCSRFFailure(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      false,
		"error":   "CSRF_FAILED",
		"message": "CSRF token validation failed",
	})
}
