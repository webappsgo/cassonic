package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// csrfTestConfig returns a CSRFConfig enabled and scoped to the "session" cookie.
func csrfTestConfig() CSRFConfig {
	return CSRFConfig{
		Enabled:           true,
		Secure:            false,
		SessionCookieName: "session",
	}
}

func TestCSRF_IssuesTokenCookieOnFirstRequest(t *testing.T) {
	h := CSRF(csrfTestConfig())(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == csrfCookieName {
			found = true
			if len(c.Value) != csrfTokenLength*2 {
				t.Errorf("csrf cookie value length = %d, want %d", len(c.Value), csrfTokenLength*2)
			}
			if c.SameSite != http.SameSiteStrictMode {
				t.Errorf("csrf cookie SameSite = %v, want Strict", c.SameSite)
			}
		}
	}
	if !found {
		t.Fatal("CSRF middleware did not set csrf_token cookie")
	}
}

func TestCSRF_GetRequestNeverBlocked(t *testing.T) {
	h := CSRF(csrfTestConfig())(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET request status = %d, want 200", rec.Code)
	}
}

func TestCSRF_BearerAuthBypassesValidation(t *testing.T) {
	h := CSRF(csrfTestConfig())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Bearer-authenticated POST status = %d, want 200", rec.Code)
	}
}

func TestCSRF_NoSessionCookieBypassesValidation(t *testing.T) {
	h := CSRF(csrfTestConfig())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("cookie-less POST status = %d, want 200", rec.Code)
	}
}

func TestCSRF_CrossSitePostWithoutTokenIsRejected(t *testing.T) {
	h := CSRF(csrfTestConfig())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site POST without token status = %d, want 403", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(rec.Body.String(), "CSRF_FAILED") {
		t.Errorf("body = %q, want CSRF_FAILED", rec.Body.String())
	}
}

func TestCSRF_SameSiteRequestBypassesValidation(t *testing.T) {
	h := CSRF(csrfTestConfig())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/x", nil)
	req.Host = "cassonic.example"
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
	req.Header.Set("Origin", "https://cassonic.example")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("same-site POST status = %d, want 200", rec.Code)
	}
}

func TestCSRF_MatchingHeaderTokenPasses(t *testing.T) {
	cfg := csrfTestConfig()

	// First request issues a token cookie.
	issue := CSRF(cfg)(okHandler())
	first := httptest.NewRequest(http.MethodGet, "/", nil)
	firstRec := httptest.NewRecorder()
	issue.ServeHTTP(firstRec, first)

	var token string
	for _, c := range firstRec.Result().Cookies() {
		if c.Name == csrfCookieName {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("no csrf token issued")
	}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set(csrfHeaderName, token)
	rec := httptest.NewRecorder()

	CSRF(cfg)(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("matching-token POST status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
}

func TestCSRF_MismatchedHeaderTokenRejected(t *testing.T) {
	h := CSRF(csrfTestConfig())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: strings.Repeat("a", csrfTokenLength*2)})
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set(csrfHeaderName, strings.Repeat("b", csrfTokenLength*2))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("mismatched token POST status = %d, want 403", rec.Code)
	}
}

func TestCSRF_MatchingFormTokenPasses(t *testing.T) {
	token := strings.Repeat("c", csrfTokenLength*2)
	form := url.Values{"csrf_token": {token}}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	CSRF(csrfTestConfig())(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("matching form token POST status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
}

func TestCSRF_ExemptPathBypassesValidation(t *testing.T) {
	cfg := csrfTestConfig()
	cfg.ExemptPaths = []string{"/api/v1/webhooks/*"}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	CSRF(cfg)(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("exempt path POST status = %d, want 200", rec.Code)
	}
}

func TestCSRF_WebSocketUpgradeBypassesValidation(t *testing.T) {
	h := CSRF(csrfTestConfig())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("websocket upgrade POST status = %d, want 200", rec.Code)
	}
}

func TestCSRF_SecFetchSiteCrossSiteRejectsEvenWithSameOrigin(t *testing.T) {
	h := CSRF(csrfTestConfig())(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "cassonic.example"
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
	req.Header.Set("Origin", "https://cassonic.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("Sec-Fetch-Site cross-site POST status = %d, want 403", rec.Code)
	}
}

func TestCSRF_DisabledSkipsEverything(t *testing.T) {
	cfg := csrfTestConfig()
	cfg.Enabled = false
	h := CSRF(cfg)(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("disabled CSRF POST status = %d, want 200", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == csrfCookieName {
			t.Error("disabled CSRF middleware should not issue a token cookie")
		}
	}
}

func TestCSRFTokenFromContext_EmptyWhenUnset(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := CSRFTokenFromContext(req.Context()); got != "" {
		t.Errorf("CSRFTokenFromContext on bare context = %q, want empty", got)
	}
}
