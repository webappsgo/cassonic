package middleware

import (
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/local/cassonic/src/server/store"
)

// subsonicVersion is the Subsonic API version reported in all error responses.
const subsonicVersion = "1.16.1"

// subsonicErrorWrongCredentials is the standard error code for bad credentials.
const subsonicErrorWrongCredentials = 40

// subsonicErrorTokenAuthNotSupported is used when token auth is requested but
// the user has no Subsonic password configured.
const subsonicErrorTokenAuthNotSupported = 41

// subsonicXMLError is the XML envelope for a Subsonic failure response.
type subsonicXMLError struct {
	XMLName xml.Name          `xml:"subsonic-response"`
	Status  string            `xml:"status,attr"`
	Version string            `xml:"version,attr"`
	Error   subsonicErrorElem `xml:"error"`
}

// subsonicErrorElem carries the numeric code and human-readable message.
type subsonicErrorElem struct {
	Code    int    `xml:"code,attr"`
	Message string `xml:"message,attr"`
}

// subsonicJSONError mirrors the XML envelope structure for JSON responses.
type subsonicJSONError struct {
	SubsonicResponse struct {
		Status  string `json:"status"`
		Version string `json:"version"`
		Error   struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"subsonic-response"`
}

// SubsonicAuth authenticates requests using the Subsonic API authentication
// scheme. Three modes are supported:
//
//  1. Token auth (Subsonic 1.13.0+): ?u=username&t=md5(subsonicPassword+salt)&s=salt
//  2. Plaintext: ?u=username&p=password (verified against Argon2id hash)
//  3. Hex-encoded plaintext: ?u=username&p=enc:hexhexhex (decoded, then treated as plaintext)
//
// getSubsonicPassword returns the stored plaintext Subsonic password for a
// username. It returns ("", false) when the user has no Subsonic password set.
// On auth failure a Subsonic-format XML or JSON error is written and the handler
// chain is terminated. The middleware is skipped entirely when no Subsonic auth
// parameters are present.
func SubsonicAuth(
	users store.UserStore,
	getSubsonicPassword func(ctx context.Context, username string) (string, bool),
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			username := q.Get("u")

			// Skip this middleware when no Subsonic auth parameters are present.
			if username == "" {
				next.ServeHTTP(w, r)
				return
			}

			format := q.Get("f")
			clientName := q.Get("c")
			saltParam := q.Get("s")
			tokenParam := q.Get("t")
			passParam := q.Get("p")

			ctx := r.Context()

			user, err := users.GetUserByUsername(ctx, username)
			if err != nil || user == nil {
				writeSubsonicError(w, format, subsonicErrorWrongCredentials, "Wrong username or password.")
				return
			}

			if user.IsLocked() || !user.IsEnabled {
				writeSubsonicError(w, format, subsonicErrorWrongCredentials, "Wrong username or password.")
				return
			}

			var authenticated bool

			switch {
			// Token auth mode: t and s params present.
			case tokenParam != "" && saltParam != "":
				subsonicPass, ok := getSubsonicPassword(ctx, username)
				if !ok || subsonicPass == "" {
					writeSubsonicError(w, format, subsonicErrorTokenAuthNotSupported,
						"Token authentication not supported. Set a Subsonic password in your profile.")
					return
				}
				expected := md5Hex(subsonicPass + saltParam)
				authenticated = subtle.ConstantTimeCompare([]byte(expected), []byte(tokenParam)) == 1

			// Plaintext or hex-encoded password mode.
			case passParam != "":
				plaintext, err := decodeSubsonicPassword(passParam)
				if err != nil {
					writeSubsonicError(w, format, subsonicErrorWrongCredentials, "Wrong username or password.")
					return
				}
				authenticated, err = verifyArgon2id(user.PasswordHash, plaintext)
				if err != nil {
					writeSubsonicError(w, format, subsonicErrorWrongCredentials, "Wrong username or password.")
					return
				}

			default:
				writeSubsonicError(w, format, subsonicErrorWrongCredentials, "Wrong username or password.")
				return
			}

			if !authenticated {
				writeSubsonicError(w, format, subsonicErrorWrongCredentials, "Wrong username or password.")
				return
			}

			_ = users.UpdateLastLogin(ctx, user.ID)

			ctx = WithUser(ctx, &AuthUser{
				ID:       user.ID,
				Username: user.Username,
				IsAdmin:  user.IsAdmin,
				Scheme:   SchemeSubsonic,
			})
			ctx = withValue(ctx, ctxKeySubsonicClient, clientName)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// decodeSubsonicPassword handles both the enc:hexhexhex prefix form and plain
// UTF-8 passwords.
func decodeSubsonicPassword(p string) (string, error) {
	const encPrefix = "enc:"
	if strings.HasPrefix(p, encPrefix) {
		raw, err := hex.DecodeString(p[len(encPrefix):])
		if err != nil {
			return "", fmt.Errorf("invalid hex-encoded password: %w", err)
		}
		return string(raw), nil
	}
	return p, nil
}

// md5Hex returns the lowercase hex-encoded MD5 digest of s.
func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// verifyArgon2id checks a candidate plaintext password against a stored Argon2id
// hash of the form $argon2id$v=19$m=<mem>,t=<time>,p=<threads>$<base64salt>$<base64hash>.
func verifyArgon2id(hash, plaintext string) (bool, error) {
	parts := strings.Split(hash, "$")
	// Expected split: ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("unsupported hash format")
	}

	var memory uint32
	var iterations uint32
	var parallelism uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
	if err != nil {
		return false, fmt.Errorf("invalid argon2id params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("invalid argon2id salt: %w", err)
	}

	storedKey, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("invalid argon2id key: %w", err)
	}

	computed := argon2.IDKey([]byte(plaintext), salt, iterations, memory, parallelism, uint32(len(storedKey)))
	match := subtle.ConstantTimeCompare(computed, storedKey) == 1
	return match, nil
}

// writeSubsonicError writes an error response in the correct format.
// format=="json" produces JSON; anything else produces XML.
func writeSubsonicError(w http.ResponseWriter, format string, code int, message string) {
	if format == "json" {
		var resp subsonicJSONError
		resp.SubsonicResponse.Status = "failed"
		resp.SubsonicResponse.Version = subsonicVersion
		resp.SubsonicResponse.Error.Code = code
		resp.SubsonicResponse.Error.Message = message
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	resp := subsonicXMLError{
		Status:  "failed",
		Version: subsonicVersion,
		Error: subsonicErrorElem{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, xml.Header)
	_ = xml.NewEncoder(w).Encode(resp)
}
