package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
}

func TestRequestIDSetsHeader(t *testing.T) {
	mw := RequestID()(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	id := rec.Header().Get("X-Request-ID")
	if id == "" {
		t.Fatal("X-Request-ID header not set")
	}
	if len(id) < 8 {
		t.Errorf("X-Request-ID looks too short: %q", id)
	}
}

func TestRequestIDPreservesExisting(t *testing.T) {
	mw := RequestID()(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "client-provided-id")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	id := rec.Header().Get("X-Request-ID")
	if id != "client-provided-id" {
		t.Errorf("X-Request-ID: got %q, want %q", id, "client-provided-id")
	}
}

func TestRequestIDUnique(t *testing.T) {
	mw := RequestID()(okHandler())

	ids := make(map[string]bool)
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
		id := rec.Header().Get("X-Request-ID")
		if ids[id] {
			t.Errorf("duplicate request ID generated: %q", id)
		}
		ids[id] = true
	}
}

func TestSecurityHeadersProduction(t *testing.T) {
	mw := SecurityHeaders(true)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	tests := []struct {
		header string
		want   string
	}{
		{"X-Frame-Options", "DENY"},
		{"X-Content-Type-Options", "nosniff"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
	}
	for _, tt := range tests {
		got := rec.Header().Get(tt.header)
		if got != tt.want {
			t.Errorf("header %q: got %q, want %q", tt.header, got, tt.want)
		}
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy header not set")
	}
	if !strings.Contains(csp, "default-src") {
		t.Errorf("Content-Security-Policy missing default-src: %q", csp)
	}
}

func TestSecurityHeadersDisabled(t *testing.T) {
	mw := SecurityHeaders(false)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Errorf("X-Frame-Options should not be set in non-prod: got %q", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "" {
		t.Errorf("Content-Security-Policy should not be set in non-prod: got %q", got)
	}
}

func TestSecurityHeadersHTTPS(t *testing.T) {
	mw := SecurityHeaders(true)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	hsts := rec.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("Strict-Transport-Security should be set for HTTPS requests")
	}
}

func TestRateLimiterAllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(10, 2)
	mw := rl.Middleware("test")(okHandler())

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:9999"
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: got status %d, want 200", i+1, rec.Code)
		}
	}
}

func TestRateLimiterBlocks(t *testing.T) {
	rl := NewRateLimiter(0, 2)
	mw := rl.Middleware("test")(okHandler())

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("3rd request: got status %d, want 429", rec.Code)
	}
}

func TestRateLimiterRetryAfterHeader(t *testing.T) {
	rl := NewRateLimiter(0, 1)
	mw := rl.Middleware("retry-test")(okHandler())

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:5000"
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			if rec.Header().Get("Retry-After") == "" {
				t.Error("Retry-After header not set on 429 response")
			}
			return
		}
	}
}

func TestLoggerCallsNext(t *testing.T) {
	var buf strings.Builder
	mw := Logger(&buf)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Logger: next handler status: got %d, want 200", rec.Code)
	}
	if buf.Len() == 0 {
		t.Error("Logger: no log output written")
	}
	if !strings.Contains(buf.String(), "/health") {
		t.Errorf("Logger: expected /health in output, got: %q", buf.String())
	}
}

func TestLoggerNilWriter(t *testing.T) {
	mw := Logger(nil)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Logger with nil writer: got %d, want 200", rec.Code)
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(r *http.Request)
		wantIP  string
	}{
		{
			name:   "remote addr only",
			setup:  func(r *http.Request) { r.RemoteAddr = "1.2.3.4:5678" },
			wantIP: "1.2.3.4",
		},
		{
			name: "X-Forwarded-For single",
			setup: func(r *http.Request) {
				r.RemoteAddr = "127.0.0.1:80"
				r.Header.Set("X-Forwarded-For", "5.6.7.8")
			},
			wantIP: "5.6.7.8",
		},
		{
			name: "X-Forwarded-For multi",
			setup: func(r *http.Request) {
				r.RemoteAddr = "127.0.0.1:80"
				r.Header.Set("X-Forwarded-For", "5.6.7.8, 10.0.0.1")
			},
			wantIP: "5.6.7.8",
		},
		{
			name: "X-Real-IP",
			setup: func(r *http.Request) {
				r.RemoteAddr = "127.0.0.1:80"
				r.Header.Set("X-Real-IP", "9.8.7.6")
			},
			wantIP: "9.8.7.6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tt.setup(req)
			got := extractIP(req)
			if got != tt.wantIP {
				t.Errorf("extractIP: got %q, want %q", got, tt.wantIP)
			}
		})
	}
}
