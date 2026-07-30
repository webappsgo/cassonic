// Package ampache tests exercise the Ampache API handlers directly (bypassing
// the chi router and auth middleware, matching the convention used by the
// subsonic and api handler packages): construct a *Handler backed by stub
// stores, build an *http.Request, invoke the handler method, and assert on
// the decoded JSON or XML response body.
package ampache

import (
	"context"
	"crypto/md5" //nolint:gosec // legacy Ampache handshake compatibility, not used for security
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/service"
	"github.com/local/cassonic/src/server/store"
)

// testStores bundles all the stub stores backing a *store.DB for a test,
// so callers can configure exactly the fields the handler under test reads.
type testStores struct {
	music     *stubMusicStore
	users     *stubUserStore
	activity  *stubActivityStore
	playlists *stubPlaylistStore
	shares    *stubShareStore
}

// newTestHandler builds an ampache Handler wired to fresh stub stores.
// The scanner is constructed with a nil TagReader: Scanner.Scan only touches
// the tag reader for libraries with Enabled == true, and stubMusicStore's
// ListLibraries defaults to an empty slice, so this is safe unless a test
// explicitly configures enabled libraries.
func newTestHandler() (*Handler, *testStores) {
	ts := &testStores{
		music:     &stubMusicStore{},
		users:     &stubUserStore{},
		activity:  &stubActivityStore{},
		playlists: &stubPlaylistStore{},
		shares:    &stubShareStore{},
	}
	db := &store.DB{
		Music:     ts.music,
		Users:     ts.users,
		Activity:  ts.activity,
		Playlists: ts.playlists,
		Shares:    ts.shares,
	}
	sessions := middleware.NewAmpacheSessionStore(sessionTTL)
	scanner := service.NewScanner(ts.music, nil, log.New(io.Discard, "", 0))
	coverArt := service.NewCoverArtService(ts.music, "")
	getPlainPassword := func(ctx context.Context, username string) (string, bool) {
		return "", false
	}
	h := NewHandler(db, sessions, scanner, coverArt, getPlainPassword)
	return h, ts
}

// newAuthedHandler returns a handler plus a valid session token for userID,
// created directly through the real AmpacheSessionStore infrastructure.
func newAuthedHandler(userID int64) (*Handler, *testStores, string) {
	h, ts := newTestHandler()
	token := h.sessions.Create(userID)
	return h, ts, token
}

// newRequest builds a GET request against the ampache endpoint with the given
// query parameters (which always includes "action").
func newRequest(action string, params map[string]string) *http.Request {
	q := url.Values{}
	q.Set("action", action)
	for k, v := range params {
		q.Set(k, v)
	}
	target := "/server/xml.server.php?" + q.Encode()
	return httptest.NewRequest(http.MethodGet, target, nil)
}

// decodeXMLRoot unmarshals a <root>...</root> XML response body into v.
func decodeXMLRoot(t *testing.T, body []byte, v any) {
	t.Helper()
	if err := xml.Unmarshal(body, v); err != nil {
		t.Fatalf("xml.Unmarshal: %v\nbody: %s", err, body)
	}
}

// xmlError is used to decode a <root><error errorCode="..">msg</error></root> body.
type xmlError struct {
	XMLName xml.Name `xml:"root"`
	Error   AmpError `xml:"error"`
}

// decodeJSON unmarshals a JSON response body into v.
func decodeJSON(t *testing.T, body []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("json.Unmarshal: %v\nbody: %s", err, body)
	}
}

// xmlErrorJSON decodes the JSON error envelope shape: {"errorCode":.., "errorMessage":..}.
type xmlErrorJSON struct {
	ErrorCode    int    `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

// sha256Passphrase computes the Ampache SHA-256 handshake passphrase:
// HASH(timestamp + HASH(password)).
func sha256Passphrase(password, timestamp string) string {
	inner := sha256.Sum256([]byte(password))
	outer := sha256.Sum256([]byte(timestamp + hex.EncodeToString(inner[:])))
	return hex.EncodeToString(outer[:])
}

// md5Passphrase computes the legacy Ampache MD5 handshake passphrase:
// HASH(timestamp + HASH(password)).
func md5Passphrase(password, timestamp string) string { //nolint:gosec // legacy Ampache compatibility
	inner := md5.Sum([]byte(password))
	outer := md5.Sum([]byte(timestamp + hex.EncodeToString(inner[:])))
	return hex.EncodeToString(outer[:])
}

// nowTimestamp returns the current unix timestamp as a decimal string.
func nowTimestamp() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}
