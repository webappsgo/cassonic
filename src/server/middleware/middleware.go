package middleware

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/local/cassonic/src/server/service/geoip"
)

// statusResponseWriter wraps http.ResponseWriter to capture the status code.
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// newStatusResponseWriter creates a wrapper that defaults to 200 if WriteHeader is never called.
func newStatusResponseWriter(w http.ResponseWriter) *statusResponseWriter {
	return &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

// generateUUIDv4 produces a random UUID v4 string using crypto/rand.
func generateUUIDv4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// RequestIDFromContext retrieves the request ID stored by RequestID middleware.
func RequestIDFromContext(ctx interface{ Value(any) any }) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

// RequestID injects a UUID v4 into every request context and response header.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = generateUUIDv4()
			}
			ctx := r.Context()
			ctx = withValue(ctx, ctxKeyRequestID, id)
			w.Header().Set("X-Request-ID", id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SecurityHeaders sets security-related HTTP response headers.
// Headers are only applied when isProd is true.
func SecurityHeaders(isProd bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isProd {
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.Header().Set("X-Frame-Options", "DENY")
				w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
				w.Header().Set("X-XSS-Protection", "0")
				w.Header().Set("Content-Security-Policy",
					"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; "+
						"img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self'; "+
						"font-src 'self'; frame-ancestors 'none'")

				isHTTPS := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
				if isHTTPS {
					w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// bucket holds the token state for a single rate-limiter key.
type bucket struct {
	tokens     float64
	lastRefill time.Time
}

// RateLimiter is a per-key token-bucket rate limiter.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64
	burst   int
}

// NewRateLimiter creates a RateLimiter with rate tokens/second and burst maximum.
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		burst:   burst,
	}
}

// Allow returns true if the key is within the rate limit, consuming one token.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(rl.burst), lastRefill: now}
		rl.buckets[key] = b
	}

	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}
	b.lastRefill = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Cleanup removes buckets that have been idle for more than 5 minutes.
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-5 * time.Minute)
	for key, b := range rl.buckets {
		if b.lastRefill.Before(cutoff) {
			delete(rl.buckets, key)
		}
	}
}

// Middleware returns a chi-compatible handler that enforces the rate limit.
// The per-request key is the remote IP concatenated with ":" and suffix.
func (rl *RateLimiter) Middleware(suffix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)
			key := ip + ":" + suffix
			if !rl.Allow(key) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IPFilter enforces IP-based allow and block lists.
type IPFilter struct {
	allowList []*net.IPNet
	blockList []*net.IPNet
}

// parseCIDRList converts a slice of CIDR/IP strings into IPNet entries.
func parseCIDRList(list []string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(list))
	for _, entry := range list {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, ipNet, err := net.ParseCIDR(entry)
			if err == nil {
				nets = append(nets, ipNet)
			}
		} else {
			ip := net.ParseIP(entry)
			if ip != nil {
				bits := 32
				if ip.To4() == nil {
					bits = 128
				}
				mask := net.CIDRMask(bits, bits)
				nets = append(nets, &net.IPNet{IP: ip.Mask(mask), Mask: mask})
			}
		}
	}
	return nets
}

// matchesAny reports whether ip is covered by any network in the list.
func matchesAny(ip net.IP, list []*net.IPNet) bool {
	for _, n := range list {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// NewIPFilter constructs an IPFilter from string lists of CIDRs or IPs.
func NewIPFilter(allowList, blockList []string) *IPFilter {
	return &IPFilter{
		allowList: parseCIDRList(allowList),
		blockList: parseCIDRList(blockList),
	}
}

// Middleware returns a chi-compatible middleware enforcing the allow/block lists.
// When allowList is non-empty, only matching IPs are permitted.
// When blockList is non-empty, matching IPs are rejected.
// allowList takes precedence over blockList when both are set.
func (f *IPFilter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawIP := extractIP(r)
			ip := net.ParseIP(rawIP)

			if len(f.allowList) > 0 {
				if ip == nil || !matchesAny(ip, f.allowList) {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			if len(f.blockList) > 0 && ip != nil && matchesAny(ip, f.blockList) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractIP returns the best available client IP from the request.
// It prefers X-Forwarded-For (first entry) then X-Real-IP, falling back to RemoteAddr.
func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		if candidate := strings.TrimSpace(parts[0]); candidate != "" {
			return candidate
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rfc1918 holds the private IPv4 ranges that should never be GeoIP-blocked.
var rfc1918 = []*net.IPNet{
	mustParseCIDR("127.0.0.0/8"),
	mustParseCIDR("10.0.0.0/8"),
	mustParseCIDR("172.16.0.0/12"),
	mustParseCIDR("192.168.0.0/16"),
	mustParseCIDR("::1/128"),
	mustParseCIDR("fc00::/7"),
}

// mustParseCIDR panics on invalid CIDR — only called with compile-time constants.
func mustParseCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic("middleware: mustParseCIDR: " + s + ": " + err.Error())
	}
	return n
}

// isPrivateIP reports whether ip is an RFC 1918 / loopback address.
func isPrivateIP(ip net.IP) bool {
	for _, n := range rfc1918 {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// GeoIPFilter returns middleware that blocks or allows requests based on country code.
// db may be nil — middleware becomes a pass-through.
// denyCountries and allowCountries are ISO 3166-1 alpha-2 codes (upper-case).
// When both are set, allowCountries takes precedence per spec.
func GeoIPFilter(db *geoip.DB, denyCountries, allowCountries []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if db == nil || (len(denyCountries) == 0 && len(allowCountries) == 0) {
				next.ServeHTTP(w, r)
				return
			}

			rawIP := extractIP(r)
			ip := net.ParseIP(rawIP)

			if ip == nil || isPrivateIP(ip) {
				next.ServeHTTP(w, r)
				return
			}

			loc, err := db.Lookup(rawIP)
			if err != nil || loc == nil {
				next.ServeHTTP(w, r)
				return
			}

			code := strings.ToUpper(loc.CountryCode)

			if len(allowCountries) > 0 {
				for _, c := range allowCountries {
					if strings.ToUpper(c) == code {
						next.ServeHTTP(w, r)
						return
					}
				}
				geoipDeny(w)
				return
			}

			for _, c := range denyCountries {
				if strings.ToUpper(c) == code {
					geoipDeny(w)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// geoipDeny writes a 403 JSON response for GeoIP-blocked requests.
func geoipDeny(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      false,
		"error":   "FORBIDDEN",
		"message": "Access denied from your region",
	})
}

// Logger returns a middleware that writes a structured log line for each request.
// If w is nil, output goes to os.Stdout.
func Logger(w io.Writer) func(http.Handler) http.Handler {
	if w == nil {
		w = os.Stdout
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := newStatusResponseWriter(rw)
			next.ServeHTTP(sw, r)
			duration := time.Since(start)
			reqID := RequestIDFromContext(r.Context())
			fmt.Fprintf(w, "%s %s %s %d %s req_id=%s\n",
				start.UTC().Format(time.RFC3339),
				r.Method,
				r.URL.Path,
				sw.statusCode,
				formatDuration(duration),
				reqID,
			)
		})
	}
}

// formatDuration renders a duration as a human-readable string with ms suffix.
func formatDuration(d time.Duration) string {
	ms := d.Milliseconds()
	return fmt.Sprintf("%dms", ms)
}

// AcceptedFormat returns the best response format for the request.
// For API routes: "json" (default) or "plain" (if Accept: text/plain).
// For web routes: "html" (default) or "plain" (if Accept: text/plain).
func AcceptedFormat(r *http.Request) string {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/plain") {
		return "plain"
	}
	if strings.Contains(accept, "text/html") {
		return "html"
	}
	return "json"
}
