package icecast

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/service/crypto"
)

// encPrefix marks a SourcePass value as AES-256-GCM ciphertext produced by
// EncryptSourcePass. Values without the prefix are treated as plaintext,
// which lets an operator hand-edit server.yml without needing to encrypt
// the password themselves.
const encPrefix = "enc:"

// icyMetaInt is the number of audio bytes between ICY metadata blocks.
const icyMetaInt = 8192

// IcecastConn is an open source connection to an Icecast server.
type IcecastConn struct {
	conn   net.Conn
	mount  *model.IcecastMount
	server *model.IcecastServer
	// metaint is the ICY metadata byte interval; always icyMetaInt.
	metaint int
	// bytesSent counts audio bytes written since the connection was established.
	bytesSent int64
	// mu protects currentTitle for concurrent SetMetadata calls.
	mu           sync.RWMutex
	currentTitle string
}

// Connect establishes an Icecast source connection using the HTTP PUT source protocol.
// key is the AES-256 key used to decrypt an "enc:"-prefixed SourcePass; pass
// nil when the source password is known to be stored as plaintext.
func Connect(server *model.IcecastServer, mount *model.IcecastMount, key []byte) (*IcecastConn, error) {
	sourcePass, err := decryptOrPlaintext(server.SourcePass, key)
	if err != nil {
		return nil, fmt.Errorf("icecast connect: %w", err)
	}

	address := net.JoinHostPort(server.Host, fmt.Sprintf("%d", server.Port))
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("icecast connect: dial %s: %w", address, err)
	}

	credentials := base64.StdEncoding.EncodeToString(
		[]byte(server.SourceUser + ":" + sourcePass),
	)

	contentType := contentTypeForFormat(mount.Format)

	var req strings.Builder
	fmt.Fprintf(&req, "PUT %s HTTP/1.0\r\n", mount.MountPath)
	fmt.Fprintf(&req, "Host: %s:%d\r\n", server.Host, server.Port)
	fmt.Fprintf(&req, "Authorization: Basic %s\r\n", credentials)
	fmt.Fprintf(&req, "Content-Type: %s\r\n", contentType)
	fmt.Fprintf(&req, "ice-name: %s\r\n", mount.Name)
	fmt.Fprintf(&req, "ice-description: %s\r\n", mount.Description)
	fmt.Fprintf(&req, "ice-audio-info: bitrate=%d\r\n", mount.BitRate)
	fmt.Fprintf(&req, "icy-metaint: %d\r\n", icyMetaInt)
	fmt.Fprintf(&req, "\r\n")

	if _, err := conn.Write([]byte(req.String())); err != nil {
		conn.Close()
		return nil, fmt.Errorf("icecast connect: write handshake: %w", err)
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("icecast connect: read response: %w", err)
	}
	statusLine = strings.TrimSpace(statusLine)

	if !strings.Contains(statusLine, "200") {
		conn.Close()
		return nil, fmt.Errorf("icecast connect: server rejected connection: %s", statusLine)
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("icecast connect: read headers: %w", err)
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	return &IcecastConn{
		conn:    conn,
		mount:   mount,
		server:  server,
		metaint: icyMetaInt,
	}, nil
}

// SetMetadata updates the ICY stream title sent at the next metadata block boundary.
func (c *IcecastConn) SetMetadata(title string) {
	c.mu.Lock()
	c.currentTitle = title
	c.mu.Unlock()
}

// Write sends audio data to the Icecast server, injecting ICY metadata blocks
// at every metaint byte boundary. Implements io.Writer.
func (c *IcecastConn) Write(p []byte) (n int, err error) {
	for len(p) > 0 {
		remaining := c.metaint - int(c.bytesSent%int64(c.metaint))
		chunk := p
		if len(chunk) > remaining {
			chunk = p[:remaining]
		}

		nn, werr := c.conn.Write(chunk)
		c.bytesSent += int64(nn)
		n += nn
		if werr != nil {
			return n, werr
		}
		p = p[nn:]

		if c.bytesSent%int64(c.metaint) == 0 {
			c.mu.RLock()
			title := c.currentTitle
			c.mu.RUnlock()

			meta := buildMetadataBlock(title)
			if _, werr := c.conn.Write(meta); werr != nil {
				return n, werr
			}
		}
	}
	return n, nil
}

// Close closes the underlying TCP connection.
func (c *IcecastConn) Close() error {
	return c.conn.Close()
}

// buildMetadataBlock builds an ICY metadata block for the given stream title.
// Format: [1-byte length/16][N*16 bytes NUL-padded metadata string]
func buildMetadataBlock(title string) []byte {
	escaped := strings.ReplaceAll(title, "'", "\\'")
	content := fmt.Sprintf("StreamTitle='%s';StreamUrl='';", escaped)
	// The ICY metadata length is a single byte counting 16-byte blocks, so the
	// content cannot exceed 255*16 bytes. Truncate long titles to stay within
	// the limit; otherwise byte(length) overflows and corrupts the block.
	const maxContent = 255 * 16
	if len(content) > maxContent {
		content = content[:maxContent]
	}
	length := (len(content) + 15) / 16
	block := make([]byte, 1+length*16)
	block[0] = byte(length)
	copy(block[1:], content)
	return block
}

// contentTypeForFormat maps a StreamFormat to its MIME type.
func contentTypeForFormat(format model.StreamFormat) string {
	switch format {
	case model.FormatOGG:
		return "application/ogg"
	case model.FormatAAC:
		return "audio/aac"
	case model.FormatOpus:
		return "audio/ogg; codecs=opus"
	default:
		return "audio/mpeg"
	}
}

// decryptOrPlaintext returns the decrypted value if encrypted, or the value as-is.
// AES-256-GCM encrypted values carry an "enc:" prefix; plain values are returned unchanged.
// key is the AES-256 master key; pass nil when encryption has not yet been configured.
// Returns an error when s is "enc:"-prefixed but key is empty or decryption fails,
// rather than silently returning an empty password.
func decryptOrPlaintext(s string, key []byte) (string, error) {
	if !strings.HasPrefix(s, encPrefix) {
		return s, nil
	}
	if len(key) == 0 {
		return "", errors.New("icecast: source password is encrypted but no encryption key is configured")
	}
	plain, err := crypto.Decrypt(key, strings.TrimPrefix(s, encPrefix))
	if err != nil {
		return "", fmt.Errorf("icecast: decrypt source password: %w", err)
	}
	return plain, nil
}

// EncryptSourcePass encrypts plaintext for storage in IcecastServer.SourcePass,
// returning the "enc:"-prefixed ciphertext that decryptOrPlaintext expects.
// key is the AES-256 master key derived the same way as the Subsonic password
// key (crypto.DeriveKey on the server's auth secret); when key is empty the
// value is stored as plaintext, matching decryptOrPlaintext's fallback so a
// server started before secrets are configured never loses the password.
func EncryptSourcePass(key []byte, plaintext string) (string, error) {
	if len(key) == 0 || plaintext == "" {
		return plaintext, nil
	}
	ciphertext, err := crypto.Encrypt(key, plaintext)
	if err != nil {
		return "", fmt.Errorf("icecast: encrypt source password: %w", err)
	}
	return encPrefix + ciphertext, nil
}
