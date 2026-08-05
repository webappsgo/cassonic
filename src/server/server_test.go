// server_test.go - tests for server.go: server construction (New), router
// wiring (buildRouter), the With* fluent setters, and the small pure handler
// builders that don't require a live listener (healthz, version, autodiscover,
// sitemap, graphql, openapi, metrics). Uses httptest per the project's
// no-live-server test guidance, and the stub-store embedding pattern already
// used by src/server/handler/*/*_test.go.
package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/local/cassonic/src/config"
	"github.com/local/cassonic/src/server/service"
	"github.com/local/cassonic/src/server/service/crypto"
	"github.com/local/cassonic/src/server/service/tags"
	"github.com/local/cassonic/src/server/store"
)

// stubUserStore embeds store.UserStore (nil); server construction and route
// registration never call user-store methods directly, only middleware
// closures do at request time, and no test here exercises authenticated routes.
type stubUserStore struct {
	store.UserStore
}

// stubMusicStore embeds store.MusicStore (nil) for the same reason.
type stubMusicStore struct {
	store.MusicStore
}

// newTestServer builds a fully-wired *Server against stub stores, suitable
// for exercising route registration and the unauthenticated handlers below.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.Defaults()
	cfg.Server.Port = 4533

	db := &store.DB{
		Users: &stubUserStore{},
		Music: &stubMusicStore{},
	}

	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	music := &stubMusicStore{}
	scanner := service.NewScanner(music, tags.New(), logger)
	coverArt := service.NewCoverArtService(music, t.TempDir())

	return New(cfg, t.TempDir()+"/server.yml", db, scanner, coverArt, nil, tags.New())
}

func TestNew_ReturnsConfiguredServer(t *testing.T) {
	srv := newTestServer(t)
	if srv == nil {
		t.Fatal("New returned nil")
	}
	if srv.http == nil {
		t.Fatal("New did not initialize the internal http.Server")
	}
	if srv.http.Handler == nil {
		t.Fatal("New did not attach a router as the http.Server handler")
	}
}

// TestBuildRouter_PublicEndpoints exercises every route mounted before auth
// that buildRouter registers directly (health, version, static, sitemap,
// docs, openapi, graphql) using httptest.NewRecorder — no live listener.
func TestBuildRouter_PublicEndpoints(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{"healthz html", http.MethodGet, "/server/healthz", http.StatusOK, "OK"},
		{"health json", http.MethodGet, "/health", http.StatusOK, `"status":"ok"`},
		{"api v1 health", http.MethodGet, "/api/v1/health", http.StatusOK, `"status":"ok"`},
		{"version json", http.MethodGet, "/version", http.StatusOK, `"version"`},
		{"api version alias", http.MethodGet, "/api/version", http.StatusOK, `"version"`},
		{"autodiscover", http.MethodGet, "/api/v1/autodiscover", http.StatusOK, `"subsonic_url"`},
		{"openapi spec", http.MethodGet, "/api/v1/openapi.json", http.StatusOK, `"openapi": "3.0.3"`},
		{"sitemap", http.MethodGet, "/sitemap.xml", http.StatusOK, "<urlset"},
		{"favicon redirect", http.MethodGet, "/favicon.ico", http.StatusMovedPermanently, ""},
		{"docs redirect", http.MethodGet, "/api/docs", http.StatusMovedPermanently, ""},
		{"metrics unauthenticated by default", http.MethodGet, "/metrics", http.StatusOK, ""},
		{"graphql get", http.MethodGet, "/graphql/", http.StatusOK, `"__typename":"Query"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			srv.http.Handler.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("%s %s: status = %d, want %d (body: %s)", tt.method, tt.path, rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("%s %s: body = %q, want to contain %q", tt.method, tt.path, rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestVersionJSON_Fields(t *testing.T) {
	origV, origC, origB := Version, CommitID, BuildDate
	t.Cleanup(func() { Version, CommitID, BuildDate = origV, origC, origB })
	Version, CommitID, BuildDate = "9.9.9", "deadbeef", "2026-07-30"

	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, req)

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /version response: %v", err)
	}
	if body["version"] != "9.9.9" || body["commit"] != "deadbeef" || body["buildDate"] != "2026-07-30" {
		t.Errorf("/version response = %+v, want version/commit/buildDate to match package vars", body)
	}
}

func TestAutodiscoverJSON_UsesBaseURL(t *testing.T) {
	srv := newTestServer(t)
	srv.cfg.Server.BaseURL = "/music"

	req := httptest.NewRequest(http.MethodGet, "/api/v1/autodiscover", nil)
	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /api/v1/autodiscover response: %v", err)
	}
	if body["base_url"] != "/music" {
		t.Errorf("autodiscover base_url = %v, want /music", body["base_url"])
	}
	if body["server"] != "cassonic" {
		t.Errorf("autodiscover server = %v, want cassonic", body["server"])
	}
}

func TestSitemapHandler_HTTPScheme(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	req.Host = "example.test"
	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "http://example.test/") {
		t.Errorf("sitemap should use http scheme and request host when TLS is nil, got: %s", body)
	}
	if strings.Contains(body, "https://") {
		t.Errorf("sitemap should not use https scheme for a plain request, got: %s", body)
	}
}

// --- With* fluent setters --------------------------------------------------

func TestWithSSL_AttachesManagerAndReturnsSelf(t *testing.T) {
	srv := newTestServer(t)
	if got := srv.WithSSL(nil); got != srv {
		t.Error("WithSSL should return the same *Server for chaining")
	}
	if srv.sslMgr != nil {
		t.Error("WithSSL(nil) should leave sslMgr nil")
	}
}

func TestWithBackupService_AttachesAndReturnsSelf(t *testing.T) {
	srv := newTestServer(t)
	if got := srv.WithBackupService(nil); got != srv {
		t.Error("WithBackupService should return the same *Server for chaining")
	}
	if srv.backupSvc != nil {
		t.Error("WithBackupService(nil) should leave backupSvc nil")
	}
}

func TestWithScheduler_AttachesAndReturnsSelf(t *testing.T) {
	srv := newTestServer(t)
	if got := srv.WithScheduler(nil); got != srv {
		t.Error("WithScheduler should return the same *Server for chaining")
	}
	if srv.sched != nil {
		t.Error("WithScheduler(nil) should leave sched nil")
	}
}

func TestWithGeoIP_RebuildsRouterAndStoresFilters(t *testing.T) {
	srv := newTestServer(t)
	oldHandler := srv.http.Handler

	deny := []string{"CN", "RU"}
	allow := []string{"US"}
	got := srv.WithGeoIP(nil, deny, allow)

	if got != srv {
		t.Error("WithGeoIP should return the same *Server for chaining")
	}
	if len(srv.denyCountries) != 2 || srv.denyCountries[0] != "CN" {
		t.Errorf("WithGeoIP denyCountries = %v, want %v", srv.denyCountries, deny)
	}
	if len(srv.allowCountries) != 1 || srv.allowCountries[0] != "US" {
		t.Errorf("WithGeoIP allowCountries = %v, want %v", srv.allowCountries, allow)
	}
	// buildRouter is called again to pick up the new GeoIP middleware — the
	// handler value must change (new chi.Mux instance) even though behavior
	// for a request without geoipDB set is unaffected.
	if srv.http.Handler == oldHandler {
		t.Error("WithGeoIP should rebuild the router (new Handler instance)")
	}

	// The rebuilt router must still serve the routes registered in buildRouter.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("rebuilt router /health status = %d, want 200", rec.Code)
	}
}

// --- metricsHandler token auth ---------------------------------------------

func TestMetricsHandler_TokenAuth(t *testing.T) {
	srv := newTestServer(t)
	srv.MetricsToken = "s3cret"
	srv.http.Handler = srv.buildRouter()

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"no token", "", http.StatusUnauthorized},
		{"wrong token", "Bearer nope", http.StatusUnauthorized},
		{"correct token", "Bearer s3cret", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			srv.http.Handler.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("%s: status = %d, want %d", tt.name, rec.Code, tt.wantStatus)
			}
		})
	}
}

// --- getSubsonicPassword ---------------------------------------------------

// subsonicPasswordUserStore is a UserStore stub that returns a configured
// encrypted subsonic password (or error/not-found) from GetSubsonicPassword.
type subsonicPasswordUserStore struct {
	store.UserStore
	enc string
	ok  bool
	err error
}

func (s *subsonicPasswordUserStore) GetSubsonicPassword(ctx context.Context, username string) (string, bool, error) {
	return s.enc, s.ok, s.err
}

func TestGetSubsonicPassword_NotSet(t *testing.T) {
	srv := newTestServer(t)
	srv.db.Users = &subsonicPasswordUserStore{ok: false}

	_, ok := srv.getSubsonicPassword(context.Background(), "nouser")
	if ok {
		t.Error("getSubsonicPassword with no stored password should return ok=false")
	}
}

func TestGetSubsonicPassword_StoreError(t *testing.T) {
	srv := newTestServer(t)
	srv.db.Users = &subsonicPasswordUserStore{ok: true, err: context.DeadlineExceeded}

	_, ok := srv.getSubsonicPassword(context.Background(), "erruser")
	if ok {
		t.Error("getSubsonicPassword with a store error should return ok=false")
	}
}

func TestGetSubsonicPassword_CorruptCiphertext(t *testing.T) {
	srv := newTestServer(t)
	srv.db.Users = &subsonicPasswordUserStore{ok: true, enc: "not-valid-ciphertext"}

	_, ok := srv.getSubsonicPassword(context.Background(), "baduser")
	if ok {
		t.Error("getSubsonicPassword with undecryptable ciphertext should return ok=false")
	}
}

func TestGetSubsonicPassword_RoundTrip(t *testing.T) {
	srv := newTestServer(t)
	enc, err := crypto.Encrypt(srv.subsonicKey, "hunter2")
	if err != nil {
		t.Fatalf("crypto.Encrypt: %v", err)
	}
	srv.db.Users = &subsonicPasswordUserStore{ok: true, enc: enc}

	plain, ok := srv.getSubsonicPassword(context.Background(), "gooduser")
	if !ok {
		t.Fatal("getSubsonicPassword: expected ok=true for a validly encrypted password")
	}
	if plain != "hunter2" {
		t.Errorf("getSubsonicPassword decrypted = %q, want %q", plain, "hunter2")
	}
}
