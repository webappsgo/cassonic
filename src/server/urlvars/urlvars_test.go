package urlvars

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/local/cassonic/src/config"
)

func testConfig() *config.Config {
	cfg := config.Defaults()
	cfg.Server.Port = 4533
	return cfg
}

// testConfigNoPort omits Server.Port so port resolution falls through to the
// proto-default (priority 4), which is what most of these tests exercise.
func testConfigNoPort() *config.Config {
	cfg := config.Defaults()
	cfg.Server.Port = 0
	return cfg
}

func newRequest(remoteAddr string, headers map[string]string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "http://example.invalid/", nil)
	r.RemoteAddr = remoteAddr
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestGetURLVarsFallsBackToHostnameThenLocalhost(t *testing.T) {
	cfg := testConfig()
	Init(cfg)

	r := newRequest("203.0.113.5:12345", nil)
	proto, fqdn, port := GetURLVars(r)

	if proto != "http" {
		t.Errorf("proto = %q, want http", proto)
	}
	// Priority 3 (os.Hostname()) wins whenever the environment has one; only
	// an environment with no hostname at all reaches priority 7 (localhost).
	wantFQDN := "localhost"
	if hn, err := os.Hostname(); err == nil && hn != "" {
		wantFQDN = hn
	}
	if fqdn != wantFQDN {
		t.Errorf("fqdn = %q, want %q (no DOMAIN, no proxy trust)", fqdn, wantFQDN)
	}
	if port != "4533" {
		t.Errorf("port = %q, want 4533 (server listen port, priority 3)", port)
	}
}

func TestGetURLVarsTrustedProxyHeaders(t *testing.T) {
	cfg := testConfigNoPort()
	Init(cfg)

	r := newRequest("127.0.0.1:55555", map[string]string{
		"X-Forwarded-Host":  "app.example.com",
		"X-Forwarded-Proto": "https",
	})
	proto, fqdn, port := GetURLVars(r)

	if proto != "https" {
		t.Errorf("proto = %q, want https", proto)
	}
	if fqdn != "app.example.com" {
		t.Errorf("fqdn = %q, want app.example.com", fqdn)
	}
	if port != "" {
		t.Errorf("port = %q, want empty (443 stripped for https)", port)
	}
}

func TestGetURLVarsIgnoresHeadersFromUntrustedPeer(t *testing.T) {
	cfg := testConfig()
	Init(cfg)

	r := newRequest("203.0.113.5:12345", map[string]string{
		"X-Forwarded-Host":  "attacker.example.com",
		"X-Forwarded-Proto": "https",
	})
	_, fqdn, _ := GetURLVars(r)

	if fqdn == "attacker.example.com" {
		t.Errorf("fqdn resolved from an untrusted peer's headers, want the header ignored")
	}
}

func TestGetURLVarsDomainEnvTakesPriorityOverHostnameFallback(t *testing.T) {
	cfg := testConfig()
	cfg.Server.Domain = []string{"myapp.com", "www.myapp.com"}
	Init(cfg)

	r := newRequest("203.0.113.5:12345", nil)
	_, fqdn, _ := GetURLVars(r)

	if fqdn != "myapp.com" {
		t.Errorf("fqdn = %q, want myapp.com (primary DOMAIN entry)", fqdn)
	}
	if base := GetBaseDomain(); base != "myapp.com" {
		t.Errorf("GetBaseDomain() = %q, want myapp.com", base)
	}
	if wc := GetWildcardDomain(); wc != "*.myapp.com" {
		t.Errorf("GetWildcardDomain() = %q, want *.myapp.com", wc)
	}
}

func TestGetURLVarsProxyHeadersOutrankDomainEnv(t *testing.T) {
	cfg := testConfig()
	cfg.Server.Domain = []string{"myapp.com"}
	Init(cfg)

	r := newRequest("127.0.0.1:1", map[string]string{"X-Forwarded-Host": "override.myapp.com"})
	_, fqdn, _ := GetURLVars(r)

	if fqdn != "override.myapp.com" {
		t.Errorf("fqdn = %q, want override.myapp.com (reverse-proxy header outranks DOMAIN)", fqdn)
	}
}

func TestGetURLVarsOnionMatchBypassesEverything(t *testing.T) {
	cfg := testConfig()
	cfg.Server.Domain = []string{"myapp.com"}
	Init(cfg)
	SetOnionAddress("abc123.onion")
	defer SetOnionAddress("")

	r := newRequest("127.0.0.1:1", map[string]string{
		"X-Forwarded-Host":  "override.myapp.com",
		"X-Forwarded-Proto": "https",
	})
	r.Host = "abc123.onion"
	proto, fqdn, port := GetURLVars(r)

	if proto != "http" || fqdn != "abc123.onion" || port != "" {
		t.Errorf("got proto=%q fqdn=%q port=%q, want http/abc123.onion/\"\"", proto, fqdn, port)
	}
}

func TestBuildURLStripsDefaultPorts(t *testing.T) {
	cfg := testConfigNoPort()
	cfg.Server.Domain = []string{"myapp.com"}
	Init(cfg)

	r := newRequest("203.0.113.5:1", nil)
	got := BuildURL(r, "/rest/")
	want := "http://myapp.com/rest/"
	if got != want {
		t.Errorf("BuildURL() = %q, want %q", got, want)
	}
}

func TestValidFQDNProductionRules(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"api.example.com", true},
		{"my.server.domain.co.uk", true},
		{"localhost", false},
		{"dev.local", false},
		{"app.test", false},
		{"192.168.1.1", false},
		{"myhost", false},
	}
	for _, c := range cases {
		if got := ValidFQDN(c.host, true); got != c.want {
			t.Errorf("ValidFQDN(%q, prod=true) = %v, want %v", c.host, got, c.want)
		}
	}
}

func TestValidFQDNDevelopmentRules(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"api.example.com", true},
		{"localhost", true},
		{"dev.local", true},
		{"app.test", true},
		{"192.168.1.1", false},
		{"devbox", false},
	}
	for _, c := range cases {
		if got := ValidFQDN(c.host, false); got != c.want {
			t.Errorf("ValidFQDN(%q, prod=false) = %v, want %v", c.host, got, c.want)
		}
	}
}

func TestTrustedProxiesConfigExtendsTrust(t *testing.T) {
	cfg := testConfig()
	cfg.Server.TrustedProxies = []string{"203.0.113.0/24"}
	Init(cfg)

	r := newRequest("203.0.113.5:1", map[string]string{"X-Forwarded-Host": "app.example.com"})
	_, fqdn, _ := GetURLVars(r)

	if fqdn != "app.example.com" {
		t.Errorf("fqdn = %q, want app.example.com (peer in configured trusted_proxies)", fqdn)
	}
}

func TestObserveInfersWildcardAfterMinSamples(t *testing.T) {
	cfg := testConfig()
	cfg.Server.URLDetection.MinSamples = 2
	Init(cfg)

	hosts := []string{"www.myapp.com", "myapp.com"}
	for _, h := range hosts {
		r := newRequest("127.0.0.1:1", map[string]string{"X-Forwarded-Host": h})
		GetURLVars(r)
	}

	if base := GetBaseDomain(); base != "myapp.com" {
		t.Errorf("GetBaseDomain() = %q, want myapp.com", base)
	}
	if wc := GetWildcardDomain(); wc != "*.myapp.com" {
		t.Errorf("GetWildcardDomain() = %q, want *.myapp.com", wc)
	}
}
