// Package urlvars resolves the {proto}, {fqdn}, and {port} request
// variables per AI.md PART 8 "URL & FQDN Detection", gating reverse-proxy
// header trust per AI.md PART 12 "Trusted Proxies". It also implements the
// "Smart FQDN Detection (Live Reload)" domain-learning subsystem and the
// "FQDN Validation Rules" production/development checks.
package urlvars

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"

	"github.com/local/cassonic/src/config"
)

// devTLDs lists the internal/dev-only suffixes blocked in production mode
// and, in development mode, accepted alongside publicly routable names.
var devTLDs = []string{
	".localhost", ".test", ".example", ".invalid",
	".local", ".lan", ".internal", ".home", ".localdomain",
	".home.arpa", ".intranet", ".corp", ".private",
}

// observation is a single resolved fqdn seen via trusted reverse-proxy
// headers, used by the domain-learning subsystem.
type observation struct {
	host string
	at   time.Time
}

// state holds the package-level singleton configuration and live-learning
// state. It is populated once via Init and read on every request.
type state struct {
	mu sync.Mutex

	domains        []string // from server.domain / DOMAIN env, first is primary
	trustedProxies []*net.IPNet
	serverPort     int
	mode           string
	onionAddress   string

	learning     bool
	minSamples   int
	sampleWindow time.Duration
	logChanges   bool

	observations []observation
	baseDomain   string
	wildcard     string
}

var s = &state{mode: "production"}

// alwaysTrusted lists the RFC 1918 / loopback / link-local ranges that are
// always trusted reverse-proxy peers, per AI.md PART 12 "Trusted Proxies".
var alwaysTrusted = mustParseCIDRs([]string{
	"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
	"169.254.0.0/16", "::1/128", "fe80::/10", "fc00::/7",
})

func mustParseCIDRs(cidrs []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// Init configures the package-level resolver from server config. Call once
// at startup after config.Load, and again after any live config reload.
func Init(cfg *config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.domains = nil
	for _, d := range cfg.Server.Domain {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" {
			s.domains = append(s.domains, d)
		}
	}

	s.trustedProxies = mustParseCIDRs(cfg.Server.TrustedProxies)
	s.serverPort = cfg.Server.Port
	s.mode = cfg.Server.Mode

	s.learning = cfg.Server.URLDetection.Learning
	s.minSamples = cfg.Server.URLDetection.MinSamples
	if s.minSamples <= 0 {
		s.minSamples = 3
	}
	window, err := time.ParseDuration(cfg.Server.URLDetection.SampleWindow)
	if err != nil || window <= 0 {
		window = 5 * time.Minute
	}
	s.sampleWindow = window
	s.logChanges = cfg.Server.URLDetection.LogChanges

	s.observations = nil
	s.baseDomain = ""
	s.wildcard = ""
	if len(s.domains) > 0 {
		s.baseDomain = baseDomainOf(s.domains[0])
		s.wildcard = inferWildcardFromList(s.domains)
	}

	for _, d := range s.domains {
		if !validFQDNLocked(d) {
			fmt.Fprintf(os.Stderr, "cassonic: warning: DOMAIN value %q failed FQDN validation for %s mode\n", d, s.mode)
		}
	}
}

// SetOnionAddress records the Tor hidden service address so that requests
// addressed to it resolve {fqdn} via priority 0 (AI.md PART 8, Resolution
// Order), bypassing reverse-proxy headers entirely.
func SetOnionAddress(addr string) {
	s.mu.Lock()
	s.onionAddress = strings.ToLower(strings.TrimSpace(addr))
	s.mu.Unlock()
}

// GetURLVars returns resolved URL variables from the request. Port is the
// empty string for 80/443 (always stripped), per AI.md PART 8.
func GetURLVars(r *http.Request) (proto, fqdn, port string) {
	s.mu.Lock()
	onion := s.onionAddress
	s.mu.Unlock()

	if onion != "" && strings.EqualFold(hostOnly(r.Host), onion) {
		return "http", onion, ""
	}

	trusted := trustedPeer(r)

	proto = resolveProto(r, trusted)
	fqdn = resolveFQDN(r, trusted)
	port = resolvePort(r, trusted, proto)

	if trusted {
		for _, h := range []string{"X-Forwarded-Host", "X-Real-Host", "X-Original-Host"} {
			if r.Header.Get(h) != "" {
				observe(fqdn)
				break
			}
		}
	}

	return proto, fqdn, port
}

// BuildURL constructs a full URL for path with automatic port stripping.
// :80 and :443 are never included.
func BuildURL(r *http.Request, path string) string {
	proto, fqdn, port := GetURLVars(r)
	if port == "" {
		return fmt.Sprintf("%s://%s%s", proto, fqdn, path)
	}
	return fmt.Sprintf("%s://%s:%s%s", proto, fqdn, port, path)
}

// ResolvedFQDN returns the best-known {fqdn} outside of any HTTP request
// context (startup banners, default email from-addresses, and similar).
// It follows the same chain as GetURLVars from priority 2 (DOMAIN) onward,
// since reverse-proxy headers require a live request to read.
func ResolvedFQDN() string {
	s.mu.Lock()
	domains := s.domains
	s.mu.Unlock()
	if len(domains) > 0 {
		return domains[0]
	}
	if hn, err := os.Hostname(); err == nil && hn != "" {
		return hn
	}
	if hn := os.Getenv("HOSTNAME"); hn != "" {
		return hn
	}
	if ip := firstPublicIP(6); ip != "" {
		return ip
	}
	if ip := firstPublicIP(4); ip != "" {
		return ip
	}
	return "localhost"
}

// GetBaseDomain returns the inferred base domain, e.g. "myapp.com" even when
// accessed via "www.myapp.com".
func GetBaseDomain() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.baseDomain != "" {
		return s.baseDomain
	}
	if len(s.domains) > 0 {
		return s.domains[0]
	}
	return ""
}

// GetWildcardDomain returns the inferred wildcard domain, e.g. "*.myapp.com",
// or the empty string if no wildcard pattern was detected.
func GetWildcardDomain() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wildcard
}

// ValidFQDN reports whether host is an acceptable {fqdn} value for the given
// mode, per AI.md PART 8 "FQDN Validation Rules".
func ValidFQDN(host string, prodMode bool) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	if net.ParseIP(host) != nil {
		return false
	}
	if strings.HasSuffix(host, ".onion") || strings.HasSuffix(host, ".i2p") || strings.HasSuffix(host, ".exit") {
		return true
	}
	if host == "localhost" {
		return !prodMode
	}

	isDevTLD := false
	for _, tld := range devTLDs {
		if strings.HasSuffix(host, tld) {
			isDevTLD = true
			break
		}
	}

	if prodMode {
		if isDevTLD || !strings.Contains(host, ".") {
			return false
		}
		_, err := publicsuffix.EffectiveTLDPlusOne(host)
		return err == nil
	}

	return strings.Contains(host, ".")
}

// validFQDNLocked calls ValidFQDN using the current package mode. Callers
// must hold s.mu.
func validFQDNLocked(host string) bool {
	return ValidFQDN(host, s.mode != "development")
}

// resolveProto resolves {proto} per the AI.md PART 8 priority order.
func resolveProto(r *http.Request, trusted bool) string {
	if trusted {
		if v := r.Header.Get("X-Forwarded-Proto"); v != "" {
			return strings.ToLower(strings.TrimSpace(strings.Split(v, ",")[0]))
		}
		if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Ssl")), "on") {
			return "https"
		}
		if v := strings.TrimSpace(r.Header.Get("X-Url-Scheme")); v != "" {
			return strings.ToLower(v)
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// resolveFQDN resolves {fqdn} per the AI.md PART 8 priority order (skipping
// priority 0, the Tor .onion match, which GetURLVars handles separately).
func resolveFQDN(r *http.Request, trusted bool) string {
	if trusted {
		for _, h := range []string{"X-Forwarded-Host", "X-Real-Host", "X-Original-Host"} {
			if v := strings.TrimSpace(r.Header.Get(h)); v != "" {
				return hostOnly(v)
			}
		}
	}
	return ResolvedFQDN()
}

// resolvePort resolves {port} per the AI.md PART 8 priority order, returning
// the empty string when the port is the default for proto (80/443).
func resolvePort(r *http.Request, trusted bool, proto string) string {
	var p string
	if trusted {
		p = strings.TrimSpace(r.Header.Get("X-Forwarded-Port"))
	}
	if p == "" {
		if _, hp, err := net.SplitHostPort(r.Host); err == nil && hp != "" {
			p = hp
		}
	}
	if p == "" {
		s.mu.Lock()
		sp := s.serverPort
		s.mu.Unlock()
		if sp != 0 {
			p = strconv.Itoa(sp)
		}
	}
	if p == "" {
		if proto == "https" {
			p = "443"
		} else {
			p = "80"
		}
	}
	if (p == "80" && proto == "http") || (p == "443" && proto == "https") {
		return ""
	}
	return p
}

// hostOnly strips any :port suffix from a Host-style value.
func hostOnly(h string) string {
	h = strings.TrimSpace(h)
	if host, _, err := net.SplitHostPort(h); err == nil {
		return host
	}
	return h
}

// trustedPeer reports whether the immediate TCP peer (never a header-rewritten
// address) is inside the trusted-proxy set, per AI.md PART 12.
func trustedPeer(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	s.mu.Lock()
	extra := s.trustedProxies
	s.mu.Unlock()

	for _, n := range alwaysTrusted {
		if n.Contains(ip) {
			return true
		}
	}
	for _, n := range extra {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// firstPublicIP returns the first global-unicast, non-private local address
// of the given IP version (4 or 6), or the empty string if none is found.
func firstPublicIP(version int) string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP
		isV4 := ip.To4() != nil
		if version == 4 && !isV4 {
			continue
		}
		if version == 6 && isV4 {
			continue
		}
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			continue
		}
		return ip.String()
	}
	return ""
}

// baseDomainOf returns the registrable base domain of host (e.g.
// "my.server.domain.co.uk" -> "domain.co.uk"), or host itself if it is an IP
// or has no recognized public suffix.
func baseDomainOf(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || net.ParseIP(host) != nil {
		return host
	}
	base, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return host
	}
	return base
}

// inferWildcardFromList infers a wildcard pattern from an explicit DOMAIN
// list: if a later entry shares the primary entry's base domain, the base
// domain is presumed to serve all subdomains.
func inferWildcardFromList(domains []string) string {
	if len(domains) < 2 {
		return ""
	}
	base := baseDomainOf(domains[0])
	for _, d := range domains[1:] {
		if d != base && baseDomainOf(d) == base {
			return "*." + base
		}
	}
	return ""
}

// observe records a trusted-header-derived fqdn observation and, once
// min_samples is reached within sample_window, (re)infers the base/wildcard
// domain. It is a no-op when learning is disabled or DOMAIN is explicitly
// set (AI.md PART 8: "Skip learning | If DOMAIN set, no need to learn").
func observe(host string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.learning || len(s.domains) > 0 || host == "" {
		return
	}

	now := time.Now()
	s.observations = append(s.observations, observation{host: host, at: now})

	cutoff := now.Add(-s.sampleWindow)
	pruned := s.observations[:0]
	for _, o := range s.observations {
		if o.at.After(cutoff) {
			pruned = append(pruned, o)
		}
	}
	s.observations = pruned

	if len(s.observations) < s.minSamples {
		return
	}

	baseCounts := map[string]int{}
	for _, o := range s.observations {
		baseCounts[baseDomainOf(o.host)]++
	}
	var topBase string
	var topCount int
	for b, c := range baseCounts {
		if c > topCount {
			topBase, topCount = b, c
		}
	}

	distinct := map[string]bool{}
	for _, o := range s.observations {
		if baseDomainOf(o.host) == topBase {
			distinct[o.host] = true
		}
	}

	newWildcard := ""
	if len(distinct) > 1 {
		newWildcard = "*." + topBase
	}

	if s.logChanges && topBase != s.baseDomain {
		fmt.Printf("cassonic: fqdn: base domain detected: %s\n", topBase)
	}
	s.baseDomain = topBase
	s.wildcard = newWildcard
}
