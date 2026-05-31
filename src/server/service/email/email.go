// Package email provides SMTP email delivery with plain-text and HTML support.
package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
	"text/template"
	"time"
)

// Config holds SMTP connection parameters. A zero-value Config (empty Host) disables sending.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// TLS uses an implicit TLS connection (port 465 style).
	TLS bool
	// StartTLS upgrades a plain connection to TLS via STARTTLS (port 587 style).
	StartTLS bool
}

// Service sends email via the configured SMTP server.
type Service struct {
	cfg    Config
	logger *log.Logger
}

// New creates an email Service with the given configuration.
func New(cfg Config, logger *log.Logger) *Service {
	return &Service{cfg: cfg, logger: logger}
}

// builtinTemplates contains the built-in email template strings keyed by name.
// Each template receives a map[string]string of substitution values.
var builtinTemplates = map[string]string{
	"password_reset": "Reset your cassonic password: {{.link}}",

	"new_device_login": "New login from {{.ip}} at {{.time}}",

	"share_notification": "{{.user}} shared music with you: {{.link}}",

	"scan_complete": "Library scan complete: {{.added}} added, {{.updated}} updated",
}

// Send delivers a plain-text or HTML email to the given recipients.
// If both textBody and htmlBody are provided, a multipart/alternative message is sent.
// Returns nil immediately when no SMTP host is configured.
func (s *Service) Send(_ context.Context, to []string, subject, textBody, htmlBody string) error {
	if s.cfg.Host == "" {
		return nil
	}

	msg := buildMessage(s.cfg.From, to, subject, textBody, htmlBody)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	if s.cfg.TLS {
		return s.sendTLS(addr, to, msg)
	}
	return s.sendPlainOrSTARTTLS(addr, to, msg)
}

// SendTemplate renders a named built-in template with data and sends the result.
// data must be a map[string]string matching the template's placeholder keys.
// Returns nil immediately when no SMTP host is configured.
func (s *Service) SendTemplate(ctx context.Context, to []string, subject, tmplName string, data any) error {
	if s.cfg.Host == "" {
		return nil
	}

	tmplStr, ok := builtinTemplates[tmplName]
	if !ok {
		return fmt.Errorf("email: unknown template %q", tmplName)
	}

	tmpl, err := template.New(tmplName).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("email: parse template %q: %w", tmplName, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("email: render template %q: %w", tmplName, err)
	}

	return s.Send(ctx, to, subject, buf.String(), "")
}

// sendTLS opens an implicit TLS (port 465) connection and sends msg.
func (s *Service) sendTLS(addr string, to []string, msg []byte) error {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         s.cfg.Host,
		MinVersion:         tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("email: tls dial %s: %w", addr, err)
	}

	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return fmt.Errorf("email: smtp client: %w", err)
	}
	defer client.Close()

	if err := s.authAndSend(client, to, msg); err != nil {
		return err
	}
	return client.Quit()
}

// sendPlainOrSTARTTLS opens a plain TCP connection, optionally upgrades via STARTTLS,
// and sends msg.
func (s *Service) sendPlainOrSTARTTLS(addr string, to []string, msg []byte) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = s.cfg.Host
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("email: dial %s: %w", addr, err)
	}
	defer client.Close()

	if s.cfg.StartTLS {
		tlsCfg := &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		}
		if err := client.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("email: starttls: %w", err)
		}
	}

	if err := s.authAndSend(client, to, msg); err != nil {
		return err
	}
	return client.Quit()
}

// authAndSend authenticates (if credentials are set) and delivers the message.
func (s *Service) authAndSend(client *smtp.Client, to []string, msg []byte) error {
	if s.cfg.Username != "" {
		auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("email: auth: %w", err)
		}
	}

	if err := client.Mail(s.cfg.From); err != nil {
		return fmt.Errorf("email: MAIL FROM: %w", err)
	}

	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("email: RCPT TO %s: %w", addr, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("email: DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("email: write body: %w", err)
	}
	return w.Close()
}

// buildMessage constructs a raw RFC 5322 message.
// When both textBody and htmlBody are non-empty, a multipart/alternative body is produced.
// When only one body is provided, a plain Content-Type header is used.
func buildMessage(from string, to []string, subject, textBody, htmlBody string) []byte {
	var buf bytes.Buffer

	headers := map[string]string{
		"From":         from,
		"To":           strings.Join(to, ", "),
		"Subject":      subject,
		"Date":         time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 -0700"),
		"MIME-Version": "1.0",
	}

	if htmlBody != "" && textBody != "" {
		boundary := "cassonic_mime_boundary"
		headers["Content-Type"] = fmt.Sprintf(`multipart/alternative; boundary="%s"`, boundary)

		for k, v := range headers {
			fmt.Fprintf(&buf, "%s: %s\r\n", k, v)
		}
		buf.WriteString("\r\n")

		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
		buf.WriteString(textBody)
		buf.WriteString("\r\n")

		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		buf.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
		buf.WriteString(htmlBody)
		buf.WriteString("\r\n")

		fmt.Fprintf(&buf, "--%s--\r\n", boundary)
	} else if htmlBody != "" {
		headers["Content-Type"] = "text/html; charset=utf-8"
		for k, v := range headers {
			fmt.Fprintf(&buf, "%s: %s\r\n", k, v)
		}
		buf.WriteString("\r\n")
		buf.WriteString(htmlBody)
	} else {
		headers["Content-Type"] = "text/plain; charset=utf-8"
		for k, v := range headers {
			fmt.Fprintf(&buf, "%s: %s\r\n", k, v)
		}
		buf.WriteString("\r\n")
		buf.WriteString(textBody)
	}

	return buf.Bytes()
}
