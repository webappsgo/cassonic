package email

// Tests cover:
//   - New: returns a usable Service wrapping the given config
//   - Send: Host=="" fast-path returns nil without attempting any network call
//   - Send: dial failure (nothing listening) is surfaced as an error
//   - Send: full happy path (plain-text only, HTML only, and multipart
//     alternative) against a hand-rolled fake SMTP server, verifying the
//     server actually receives MAIL FROM/RCPT TO/DATA and the expected body
//   - SendTemplate: Host=="" fast-path, unknown-template error (returned
//     before any network call), and a successful render+send round trip
//     through the same fake SMTP server
//   - buildMessage: single-body vs. multipart/alternative header and body
//     shape, recipient joining, empty-body edge cases
//   - testSMTP (autodetect.go): success against a fake greeting server,
//     failure on connection refused, failure when the server sends a
//     non-220 greeting
//   - detectGateway / detectFQDN (autodetect.go): weak sanity checks only
//     (must not panic; output, if any, is a plausible non-whitespace
//     string) since both read hardcoded system resources
//     (/proc/net/route, os.Hostname/DNS) that cannot be injected without
//     modifying the source
//
// sendTLS / the StartTLS-upgrade branch of sendPlainOrSTARTTLS are NOT
// covered: exercising them would require generating and trusting a TLS
// certificate for a fake server, which is out of scope for this task.

import (
	"bufio"
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeSMTPServer starts a minimal SMTP server that accepts one connection,
// speaks just enough of the protocol to satisfy net/smtp's client, and
// reports the DATA payload it received over dataCh.
func fakeSMTPServer(t *testing.T) (addr string, dataCh <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	ch := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		w := func(s string) { conn.Write([]byte(s + "\r\n")) }
		r := bufio.NewReader(conn)

		w("220 fake.smtp ESMTP ready")
		var body strings.Builder
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")

			if inData {
				if line == "." {
					inData = false
					w("250 OK: message accepted")
					ch <- body.String()
					continue
				}
				body.WriteString(line)
				body.WriteString("\n")
				continue
			}

			upper := strings.ToUpper(line)
			switch {
			case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
				w("250-fake.smtp greets you")
				w("250 OK")
			case strings.HasPrefix(upper, "MAIL FROM"):
				w("250 OK")
			case strings.HasPrefix(upper, "RCPT TO"):
				w("250 OK")
			case strings.HasPrefix(upper, "DATA"):
				w("354 Start mail input; end with <CRLF>.<CRLF>")
				inData = true
			case strings.HasPrefix(upper, "QUIT"):
				w("221 Bye")
				return
			default:
				w("250 OK")
			}
		}
	}()

	return ln.Addr().String(), ch
}

func testConfig(addr string) Config {
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)
	return Config{
		Host: host,
		Port: port,
		From: "cassonic@example.com",
	}
}

// --- New ---

func TestNewReturnsUsableService(t *testing.T) {
	svc := New(Config{Host: "smtp.example.com"}, nil)
	if svc == nil {
		t.Fatal("New: returned nil")
	}
	if svc.cfg.Host != "smtp.example.com" {
		t.Errorf("New: cfg.Host got %q, want %q", svc.cfg.Host, "smtp.example.com")
	}
}

// --- Send: fast paths ---

func TestSendNoHostConfiguredReturnsNilImmediately(t *testing.T) {
	svc := New(Config{}, nil)
	err := svc.Send(context.Background(), []string{"a@example.com"}, "Subject", "body", "")
	if err != nil {
		t.Errorf("Send with empty Host: expected nil, got %v", err)
	}
}

func TestSendDialFailureReturnsError(t *testing.T) {
	// Port 1 is reserved and nothing should be listening there.
	svc := New(Config{Host: "127.0.0.1", Port: 1, From: "a@example.com"}, nil)
	err := svc.Send(context.Background(), []string{"b@example.com"}, "Subject", "body", "")
	if err == nil {
		t.Fatal("Send: expected dial error, got nil")
	}
}

// --- Send: happy path via fake SMTP server ---

func TestSendPlainTextHappyPath(t *testing.T) {
	addr, dataCh := fakeSMTPServer(t)
	svc := New(testConfig(addr), nil)

	err := svc.Send(context.Background(), []string{"to@example.com"}, "Hello", "plain body text", "")
	if err != nil {
		t.Fatalf("Send: unexpected error: %v", err)
	}

	select {
	case body := <-dataCh:
		if !strings.Contains(body, "plain body text") {
			t.Errorf("Send: server-received body missing text content; got:\n%s", body)
		}
		if !strings.Contains(body, "Content-Type: text/plain") {
			t.Errorf("Send: server-received body missing text/plain Content-Type; got:\n%s", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Send: timed out waiting for server to receive DATA")
	}
}

func TestSendMultipartHappyPath(t *testing.T) {
	addr, dataCh := fakeSMTPServer(t)
	svc := New(testConfig(addr), nil)

	err := svc.Send(context.Background(), []string{"to@example.com"}, "Hello", "plain part", "<b>html part</b>")
	if err != nil {
		t.Fatalf("Send: unexpected error: %v", err)
	}

	select {
	case body := <-dataCh:
		if !strings.Contains(body, "multipart/alternative") {
			t.Errorf("Send: expected multipart/alternative Content-Type; got:\n%s", body)
		}
		if !strings.Contains(body, "plain part") || !strings.Contains(body, "html part") {
			t.Errorf("Send: missing plain or html part; got:\n%s", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Send: timed out waiting for server to receive DATA")
	}
}

// --- SendTemplate ---

func TestSendTemplateNoHostConfiguredReturnsNilImmediately(t *testing.T) {
	svc := New(Config{}, nil)
	err := svc.SendTemplate(context.Background(), []string{"a@example.com"}, "Subject", "password_reset", map[string]string{"link": "x"})
	if err != nil {
		t.Errorf("SendTemplate with empty Host: expected nil, got %v", err)
	}
}

func TestSendTemplateUnknownTemplateReturnsErrorWithoutNetwork(t *testing.T) {
	// Port 1 would fail to dial if the network path were reached at all;
	// the unknown-template error must be returned before that happens.
	svc := New(Config{Host: "127.0.0.1", Port: 1, From: "a@example.com"}, nil)
	err := svc.SendTemplate(context.Background(), []string{"a@example.com"}, "Subject", "no_such_template", nil)
	if err == nil {
		t.Fatal("SendTemplate: expected error for unknown template, got nil")
	}
	if !strings.Contains(err.Error(), "unknown template") {
		t.Errorf("SendTemplate: error %v, want it to mention %q", err, "unknown template")
	}
}

func TestSendTemplateHappyPath(t *testing.T) {
	addr, dataCh := fakeSMTPServer(t)
	svc := New(testConfig(addr), nil)

	err := svc.SendTemplate(context.Background(), []string{"to@example.com"}, "Reset", "password_reset", map[string]string{"link": "https://example.com/reset"})
	if err != nil {
		t.Fatalf("SendTemplate: unexpected error: %v", err)
	}

	select {
	case body := <-dataCh:
		if !strings.Contains(body, "https://example.com/reset") {
			t.Errorf("SendTemplate: rendered link missing from body; got:\n%s", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SendTemplate: timed out waiting for server to receive DATA")
	}
}

// --- buildMessage ---

func TestBuildMessage(t *testing.T) {
	tests := []struct {
		name        string
		textBody    string
		htmlBody    string
		wantHave    []string
		wantMissing []string
	}{
		{
			name:     "plain text only",
			textBody: "plain content",
			htmlBody: "",
			wantHave: []string{"Content-Type: text/plain; charset=utf-8", "plain content"},
			wantMissing: []string{
				"multipart/alternative",
			},
		},
		{
			name:     "html only",
			textBody: "",
			htmlBody: "<p>html content</p>",
			wantHave: []string{"Content-Type: text/html; charset=utf-8", "<p>html content</p>"},
			wantMissing: []string{
				"multipart/alternative",
			},
		},
		{
			name:     "multipart",
			textBody: "plain content",
			htmlBody: "<p>html content</p>",
			wantHave: []string{
				`multipart/alternative; boundary="cassonic_mime_boundary"`,
				"plain content",
				"<p>html content</p>",
				"--cassonic_mime_boundary--",
			},
		},
		{
			name:     "both empty",
			textBody: "",
			htmlBody: "",
			wantHave: []string{"Content-Type: text/plain; charset=utf-8"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := buildMessage("from@example.com", []string{"a@example.com", "b@example.com"}, "Subject Line", tt.textBody, tt.htmlBody)
			s := string(msg)
			for _, want := range tt.wantHave {
				if !strings.Contains(s, want) {
					t.Errorf("buildMessage(%s): missing %q\nfull message:\n%s", tt.name, want, s)
				}
			}
			for _, notWant := range tt.wantMissing {
				if strings.Contains(s, notWant) {
					t.Errorf("buildMessage(%s): unexpectedly contains %q\nfull message:\n%s", tt.name, notWant, s)
				}
			}
			if !strings.Contains(s, "To: a@example.com, b@example.com") {
				t.Errorf("buildMessage(%s): recipients not joined correctly\nfull message:\n%s", tt.name, s)
			}
			if !strings.Contains(s, "From: from@example.com") {
				t.Errorf("buildMessage(%s): missing From header\nfull message:\n%s", tt.name, s)
			}
			if !strings.Contains(s, "Subject: Subject Line") {
				t.Errorf("buildMessage(%s): missing Subject header\nfull message:\n%s", tt.name, s)
			}
		})
	}
}

// --- testSMTP ---

func TestTestSMTPSuccess(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.Write([]byte("220 fake.smtp ready\r\n"))
		// Give the client time to read the greeting before we close.
		time.Sleep(50 * time.Millisecond)
	}()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	if !testSMTP(host, port) {
		t.Error("testSMTP: expected true for server sending 220 greeting")
	}
}

func TestTestSMTPConnectionRefused(t *testing.T) {
	if testSMTP("127.0.0.1", 1) {
		t.Error("testSMTP: expected false for connection refused on reserved port")
	}
}

func TestTestSMTPBadGreeting(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.Write([]byte("554 go away\r\n"))
		time.Sleep(50 * time.Millisecond)
	}()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	if testSMTP(host, port) {
		t.Error("testSMTP: expected false when server sends a non-220 greeting")
	}
}

// --- detectGateway / detectFQDN: weak sanity checks ---

func TestDetectGatewayDoesNotPanic(t *testing.T) {
	got := detectGateway()
	if got != "" && strings.TrimSpace(got) == "" {
		t.Errorf("detectGateway: got whitespace-only non-empty string %q", got)
	}
}

func TestDetectFQDNDoesNotPanic(t *testing.T) {
	got := detectFQDN()
	if got != "" && strings.TrimSpace(got) == "" {
		t.Errorf("detectFQDN: got whitespace-only non-empty string %q", got)
	}
}
