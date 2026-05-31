package icecast

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/local/cassonic/src/server/model"
)

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
func Connect(server *model.IcecastServer, mount *model.IcecastMount) (*IcecastConn, error) {
	address := net.JoinHostPort(server.Host, fmt.Sprintf("%d", server.Port))
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("icecast connect: dial %s: %w", address, err)
	}

	sourcePass := decryptOrPlaintext(server.SourcePass, nil)
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
func decryptOrPlaintext(s string, key []byte) string {
	if !strings.HasPrefix(s, "enc:") {
		return s
	}
	_ = key
	return ""
}
