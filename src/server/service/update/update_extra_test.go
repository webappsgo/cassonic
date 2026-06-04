package update

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- versionParts: gaps in existing coverage ---

// versionParts: pre-release suffix like "1.0.0-beta" — the "-beta" segment
// is non-numeric; strconv.Atoi fails and the component is treated as 0.
func TestVersionParts_PreReleaseSuffix(t *testing.T) {
	got := versionParts("1.0.0-beta")
	if len(got) < 3 {
		t.Fatalf("versionParts(\"1.0.0-beta\"): len %d, want >= 3", len(got))
	}
	if got[0] != 1 || got[1] != 0 {
		t.Errorf("versionParts(\"1.0.0-beta\"): major.minor = %d.%d, want 1.0", got[0], got[1])
	}
	// The patch segment "0-beta" is non-numeric, so it should be 0.
	if got[2] != 0 {
		t.Errorf("versionParts(\"1.0.0-beta\")[2] = %d, want 0 (non-numeric treated as 0)", got[2])
	}
}

// versionParts: v-prefixed pre-release ("v1.2.3-beta") strips v correctly.
func TestVersionParts_VPrefixWithPreRelease(t *testing.T) {
	got := versionParts("v1.2.3-beta")
	if len(got) < 3 {
		t.Fatalf("versionParts(\"v1.2.3-beta\"): len %d, want >= 3", len(got))
	}
	if got[0] != 1 || got[1] != 2 {
		t.Errorf("versionParts(\"v1.2.3-beta\"): major.minor = %d.%d, want 1.2", got[0], got[1])
	}
}

// --- semverLess: additional boundary cases not in existing tests ---

// Pre-release tags are treated as 0 patch — "1.0.0-beta" compares as 1.0.0,
// which is NOT less than "1.0.0".
func TestSemverLess_PreReleaseVsRelease(t *testing.T) {
	if semverLess("1.0.0-beta", "1.0.0") {
		t.Error("semverLess(\"1.0.0-beta\", \"1.0.0\"): got true, want false (pre-release == release numerically)")
	}
}

// "v1.2.3" stripped of v-prefix vs plain "1.2.3" must compare equal.
func TestSemverLess_VPrefixBothEqual(t *testing.T) {
	if semverLess("v1.2.3", "1.2.3") {
		t.Error("semverLess(\"v1.2.3\", \"1.2.3\"): got true, want false")
	}
	if semverLess("1.2.3", "v1.2.3") {
		t.Error("semverLess(\"1.2.3\", \"v1.2.3\"): got true, want false")
	}
}

// Empty string on both sides — both treated as 0, so neither is less.
func TestSemverLess_BothEmpty(t *testing.T) {
	if semverLess("", "") {
		t.Error("semverLess(\"\", \"\"): got true, want false")
	}
}

// Four-component version: extra component beyond patch is compared.
func TestSemverLess_FourComponents(t *testing.T) {
	if !semverLess("1.0.0.0", "1.0.0.1") {
		t.Error("semverLess(\"1.0.0.0\", \"1.0.0.1\"): got false, want true")
	}
	if semverLess("1.0.0.1", "1.0.0.0") {
		t.Error("semverLess(\"1.0.0.1\", \"1.0.0.0\"): got true, want false")
	}
}

// --- IsNewer: additional cases using the public Checker.IsNewer ---

// IsNewer: "v1.2.3" release tag (v-prefixed) is newer than "1.2.2".
func TestIsNewer_VPrefixedReleaseTag(t *testing.T) {
	c := New("1.2.2", log.Default())
	rel := &Release{Version: "v1.2.3"}
	if !c.IsNewer(rel) {
		t.Error("IsNewer(current=1.2.2, release=v1.2.3): got false, want true")
	}
}

// IsNewer: release is a pre-release ("1.0.0-beta") relative to stable "1.0.0"
// — numerically equal so not newer.
func TestIsNewer_PreReleaseSameAsStable(t *testing.T) {
	c := New("1.0.0", log.Default())
	rel := &Release{Version: "1.0.0-beta"}
	if c.IsNewer(rel) {
		t.Error("IsNewer(current=1.0.0, release=1.0.0-beta): got true, want false")
	}
}

// IsNewer: lower patch version is NOT newer.
func TestIsNewer_LowerPatch_ReturnsFalse(t *testing.T) {
	c := New("1.0.5", log.Default())
	rel := &Release{Version: "1.0.3"}
	if c.IsNewer(rel) {
		t.Error("IsNewer(current=1.0.5, release=1.0.3): got true, want false")
	}
}

// IsNewer: lower minor version is NOT newer.
func TestIsNewer_LowerMinor_ReturnsFalse(t *testing.T) {
	c := New("1.3.0", log.Default())
	rel := &Release{Version: "1.1.0"}
	if c.IsNewer(rel) {
		t.Error("IsNewer(current=1.3.0, release=1.1.0): got true, want false")
	}
}

// IsNewer: higher minor version IS newer.
func TestIsNewer_HigherMinor_ReturnsTrue(t *testing.T) {
	c := New("1.0.0", log.Default())
	rel := &Release{Version: "1.1.0"}
	if !c.IsNewer(rel) {
		t.Error("IsNewer(current=1.0.0, release=1.1.0): got false, want true")
	}
}

// IsNewer: same version returns false.
func TestIsNewer_SameVersion_ReturnsFalse(t *testing.T) {
	c := New("2.3.4", log.Default())
	rel := &Release{Version: "2.3.4"}
	if c.IsNewer(rel) {
		t.Error("IsNewer(same version): got true, want false")
	}
}

// --- New() constructor ---

// New never returns nil regardless of inputs.
func TestNew_NeverReturnsNil(t *testing.T) {
	cases := []string{"", "dev", "1.0.0", "v1.2.3-beta"}
	for _, v := range cases {
		c := New(v, log.Default())
		if c == nil {
			t.Errorf("New(%q, logger): returned nil Checker", v)
		}
	}
}

// New stores the version so IsNewer compares against it correctly.
func TestNew_StoresCurrentVersion(t *testing.T) {
	c := New("5.0.0", log.Default())
	rel := &Release{Version: "4.9.9"}
	if c.IsNewer(rel) {
		t.Error("New(5.0.0): IsNewer(4.9.9) = true, want false (stored version must be 5.0.0)")
	}
}

// --- CheckLatest: HTTP interaction via httptest ---

// CheckLatest parses a well-formed GitHub Releases API response correctly.
func TestCheckLatest_WellFormedResponse(t *testing.T) {
	payload := map[string]interface{}{
		"tag_name":     "v2.1.0",
		"published_at": "2026-01-15T10:00:00Z",
		"assets": []map[string]interface{}{
			{"browser_download_url": "https://example.com/cassonic-linux-amd64", "name": "cassonic-linux-amd64"},
		},
	}
	body, _ := json.Marshal(payload)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	c := New("1.0.0", log.Default())
	c.httpClient = srv.Client()

	// Point the checker at the test server by temporarily replacing the URL constant
	// is not possible (it's a package-level const), so we exercise CheckLatest
	// indirectly by confirming the HTTP client is wired up correctly. Instead, we
	// call CheckLatest with the real URL replaced via the test server URL injected
	// through a custom http.Client that redirects all requests.
	c.httpClient = &http.Client{
		Transport: &redirectTransport{base: srv.URL},
		Timeout:   5 * time.Second,
	}

	rel, err := c.CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest: unexpected error: %v", err)
	}
	if rel.Version != "2.1.0" {
		t.Errorf("CheckLatest: Version = %q, want \"2.1.0\"", rel.Version)
	}
	if rel.DownloadURL != "https://example.com/cassonic-linux-amd64" {
		t.Errorf("CheckLatest: DownloadURL = %q, want the asset URL", rel.DownloadURL)
	}
	wantTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	if !rel.PublishedAt.Equal(wantTime) {
		t.Errorf("CheckLatest: PublishedAt = %v, want %v", rel.PublishedAt, wantTime)
	}
}

// CheckLatest strips the "v" prefix from tag_name.
func TestCheckLatest_StripVPrefix(t *testing.T) {
	payload := map[string]interface{}{
		"tag_name": "v3.0.0",
		"assets":   []interface{}{},
	}
	body, _ := json.Marshal(payload)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	c := New("1.0.0", log.Default())
	c.httpClient = &http.Client{
		Transport: &redirectTransport{base: srv.URL},
		Timeout:   5 * time.Second,
	}

	rel, err := c.CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest: %v", err)
	}
	if rel.Version != "3.0.0" {
		t.Errorf("CheckLatest: Version = %q, want \"3.0.0\" (v prefix stripped)", rel.Version)
	}
}

// CheckLatest returns an error when the server responds with a non-200 status.
func TestCheckLatest_NonOKStatus_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()

	c := New("1.0.0", log.Default())
	c.httpClient = &http.Client{
		Transport: &redirectTransport{base: srv.URL},
		Timeout:   5 * time.Second,
	}

	_, err := c.CheckLatest(context.Background())
	if err == nil {
		t.Error("CheckLatest with HTTP 404: expected error, got nil")
	}
}

// CheckLatest returns an error when the response body is invalid JSON.
func TestCheckLatest_InvalidJSON_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	c := New("1.0.0", log.Default())
	c.httpClient = &http.Client{
		Transport: &redirectTransport{base: srv.URL},
		Timeout:   5 * time.Second,
	}

	_, err := c.CheckLatest(context.Background())
	if err == nil {
		t.Error("CheckLatest with invalid JSON: expected error, got nil")
	}
}

// CheckLatest handles missing assets gracefully (DownloadURL is empty string).
func TestCheckLatest_NoAssets_DownloadURLEmpty(t *testing.T) {
	payload := map[string]interface{}{
		"tag_name": "v1.0.0",
		"assets":   []interface{}{},
	}
	body, _ := json.Marshal(payload)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	c := New("0.9.0", log.Default())
	c.httpClient = &http.Client{
		Transport: &redirectTransport{base: srv.URL},
		Timeout:   5 * time.Second,
	}

	rel, err := c.CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest with no assets: %v", err)
	}
	if rel.DownloadURL != "" {
		t.Errorf("CheckLatest with no assets: DownloadURL = %q, want \"\"", rel.DownloadURL)
	}
}

// CheckLatest respects context cancellation.
func TestCheckLatest_ContextCancelled_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow server — the context will cancel before this returns.
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New("1.0.0", log.Default())
	c.httpClient = &http.Client{
		Transport: &redirectTransport{base: srv.URL},
		Timeout:   10 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.CheckLatest(ctx)
	if err == nil {
		t.Error("CheckLatest with cancelled context: expected error, got nil")
	}
}

// redirectTransport rewrites every request's host to the given base URL,
// allowing us to redirect CheckLatest (which has a hardcoded GitHub URL) to
// the test server without modifying package internals.
type redirectTransport struct {
	base string
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	target, err := http.NewRequest(req.Method, rt.base, req.Body)
	if err != nil {
		return nil, err
	}
	clone.URL = target.URL
	clone.Host = target.Host
	return http.DefaultTransport.RoundTrip(clone)
}
