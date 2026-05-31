package email

import (
	"bufio"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// AutoDetectSMTP tries common SMTP host/port combinations and returns the first working one.
// Returns ("", 0, false) if none found.
// Called on first run when smtp.host is empty in config.
func AutoDetectSMTP() (host string, port int, ok bool) {
	candidates := []string{
		"127.0.0.1",
		"172.17.0.1",
		detectGateway(),
		detectFQDN(),
	}
	ports := []int{587, 465, 25}

	for _, h := range candidates {
		if h == "" {
			continue
		}
		for _, p := range ports {
			if testSMTP(h, p) {
				return h, p, true
			}
		}
	}
	return "", 0, false
}

// testSMTP tries an SMTP EHLO handshake to host:port with a 2-second timeout.
func testSMTP(host string, port int) bool {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return false
	}
	defer c.Close()
	return true
}

// detectGateway parses /proc/net/route on Linux to find the default IPv4 gateway.
// Returns an empty string on non-Linux systems or on any error.
func detectGateway() string {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Skip header line.
	scanner.Scan()
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		// Destination == 00000000 is the default route.
		if fields[1] != "00000000" {
			continue
		}
		// Gateway field is little-endian hex.
		gw := fields[2]
		if len(gw) != 8 {
			continue
		}
		var b [4]byte
		for i := 0; i < 4; i++ {
			var v uint8
			if _, err := fmt.Sscanf(gw[i*2:i*2+2], "%02X", &v); err != nil {
				break
			}
			b[3-i] = v
		}
		return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
	}
	return ""
}

// detectFQDN attempts to determine the fully qualified domain name of the local host.
// Falls back to the short hostname on any lookup error.
func detectFQDN() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	addrs, err := net.LookupHost(hostname)
	if err != nil || len(addrs) == 0 {
		return hostname
	}
	names, err := net.LookupAddr(addrs[0])
	if err != nil || len(names) == 0 {
		return hostname
	}
	return strings.TrimSuffix(names[0], ".")
}
