package icecast

// Tests cover:
//   - Connect: successful handshake against a fake local TCP server that
//     speaks the Icecast source PUT protocol (verifies request line,
//     Basic Auth header, and that a 200 status is accepted)
//   - Connect: server rejection (non-200 status line) is surfaced as an error
//   - Connect: dial failure (nothing listening) is surfaced as an error
//   - IcecastConn.Write: injects an ICY metadata block at the metaint byte
//     boundary; verified against a real net.Pipe() peer
//   - buildMetadataBlock: length byte, NUL padding, and single-quote escaping
//   - contentTypeForFormat: all known formats plus the default fallback
//   - decryptOrPlaintext: plaintext passthrough, successful "enc:"-prefixed
//     decryption, and error cases (no key, corrupt ciphertext)
//   - EncryptSourcePass: round-trips with decryptOrPlaintext; plaintext
//     passthrough when no key is configured

import (
	"bufio"
	"encoding/base64"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
)

// fakeIcecastServer starts a listener that performs one Icecast source PUT
// handshake and reports the received request line + headers back over reqCh.
func fakeIcecastServer(t *testing.T, statusLine string) (addr string, reqCh <-chan []string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	ch := make(chan []string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		var lines []string
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimRight(line, "\r\n")
			lines = append(lines, line)
			if line == "" {
				break
			}
		}
		ch <- lines
		conn.Write([]byte(statusLine + "\r\n\r\n"))
	}()

	return ln.Addr().String(), ch
}

func TestConnectSuccess(t *testing.T) {
	addr, reqCh := fakeIcecastServer(t, "HTTP/1.0 200 OK")
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	server := &model.IcecastServer{
		Host:       host,
		Port:       port,
		SourceUser: "source",
		SourcePass: "hackme",
	}
	mount := &model.IcecastMount{
		MountPath: "/live",
		Name:      "Live",
		Format:    model.FormatMP3,
		BitRate:   128,
	}

	conn, err := Connect(server, mount, nil)
	if err != nil {
		t.Fatalf("Connect: unexpected error: %v", err)
	}
	defer conn.Close()

	lines := <-reqCh
	if len(lines) == 0 {
		t.Fatal("Connect: server received no request lines")
	}
	if lines[0] != "PUT /live HTTP/1.0" {
		t.Errorf("request line: got %q, want %q", lines[0], "PUT /live HTTP/1.0")
	}

	wantAuth := "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("source:hackme"))
	found := false
	for _, l := range lines {
		if l == wantAuth {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("request headers: missing expected auth header %q; got %v", wantAuth, lines)
	}
}

func TestConnectServerRejects(t *testing.T) {
	addr, _ := fakeIcecastServer(t, "HTTP/1.0 403 Forbidden")
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	server := &model.IcecastServer{Host: host, Port: port, SourceUser: "u", SourcePass: "p"}
	mount := &model.IcecastMount{MountPath: "/live", Format: model.FormatMP3}

	_, err = Connect(server, mount, nil)
	if err == nil {
		t.Fatal("Connect: expected error for non-200 status, got nil")
	}
}

func TestConnectDialFailure(t *testing.T) {
	server := &model.IcecastServer{Host: "127.0.0.1", Port: 1, SourceUser: "u", SourcePass: "p"}
	mount := &model.IcecastMount{MountPath: "/live", Format: model.FormatMP3}

	_, err := Connect(server, mount, nil)
	if err == nil {
		t.Fatal("Connect: expected dial error, got nil")
	}
}

// --- IcecastConn.Write ---

func TestIcecastConnWriteInjectsMetadataAtBoundary(t *testing.T) {
	client, srv := net.Pipe()
	defer client.Close()
	defer srv.Close()

	c := &IcecastConn{conn: client, metaint: 4}
	c.SetMetadata("Song Title")

	// net.Pipe is synchronous and unbuffered: each Write call is matched to
	// exactly the Read call(s) that drain it, so the metadata block
	// (written in a second, separate conn.Write) requires a second Read on
	// the server side to observe.
	readDone := make(chan []byte, 1)
	go func() {
		var got []byte
		buf := make([]byte, 64)
		for len(got) < 5 {
			n, err := srv.Read(buf)
			if n > 0 {
				got = append(got, buf[:n]...)
			}
			if err != nil {
				break
			}
		}
		readDone <- got
	}()

	go func() {
		if _, err := c.Write([]byte("abcd")); err != nil {
			t.Errorf("Write: unexpected error: %v", err)
		}
	}()

	select {
	case got := <-readDone:
		// 4 audio bytes + 1 length byte + metadata block.
		if len(got) < 5 {
			t.Fatalf("Write: server received %d bytes, want at least 5", len(got))
		}
		if string(got[:4]) != "abcd" {
			t.Errorf("Write: audio prefix got %q, want %q", got[:4], "abcd")
		}
		wantLen := (len("StreamTitle='Song Title';StreamUrl='';") + 15) / 16
		if int(got[4]) != wantLen {
			t.Errorf("Write: metadata length byte got %d, want %d", got[4], wantLen)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Write: timed out waiting for server read")
	}
}

// --- buildMetadataBlock ---

func TestBuildMetadataBlock(t *testing.T) {
	tests := []struct {
		name  string
		title string
	}{
		{"empty title", ""},
		{"simple title", "Artist - Title"},
		{"title with single quote", "Guns N' Roses - Song"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := buildMetadataBlock(tt.title)
			if len(block) == 0 {
				t.Fatal("buildMetadataBlock: empty result")
			}
			length := int(block[0])
			if len(block) != 1+length*16 {
				t.Errorf("buildMetadataBlock: block length %d, want %d", len(block), 1+length*16)
			}
			escaped := strings.ReplaceAll(tt.title, "'", "\\'")
			wantContent := "StreamTitle='" + escaped + "';StreamUrl='';"
			gotContent := strings.TrimRight(string(block[1:]), "\x00")
			if gotContent != wantContent {
				t.Errorf("buildMetadataBlock: content got %q, want %q", gotContent, wantContent)
			}
		})
	}
}

// --- contentTypeForFormat ---

func TestContentTypeForFormat(t *testing.T) {
	tests := []struct {
		format model.StreamFormat
		want   string
	}{
		{model.FormatMP3, "audio/mpeg"},
		{model.FormatOGG, "application/ogg"},
		{model.FormatAAC, "audio/aac"},
		{model.FormatOpus, "audio/ogg; codecs=opus"},
		{model.StreamFormat("unknown"), "audio/mpeg"},
	}
	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			got := contentTypeForFormat(tt.format)
			if got != tt.want {
				t.Errorf("contentTypeForFormat(%q): got %q, want %q", tt.format, got, tt.want)
			}
		})
	}
}

// --- decryptOrPlaintext / EncryptSourcePass ---

func TestDecryptOrPlaintextPassthrough(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plaintext passthrough", "hackme", "hackme"},
		{"empty passthrough", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decryptOrPlaintext(tt.in, nil)
			if err != nil {
				t.Fatalf("decryptOrPlaintext(%q): unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("decryptOrPlaintext(%q): got %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDecryptOrPlaintextEncryptedNoKey(t *testing.T) {
	_, err := decryptOrPlaintext("enc:abcd1234", nil)
	if err == nil {
		t.Fatal("decryptOrPlaintext: expected error for encrypted value with no key, got nil")
	}
}

func TestDecryptOrPlaintextCorruptCiphertext(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	_, err := decryptOrPlaintext("enc:not-valid-base64-ciphertext!!", key)
	if err == nil {
		t.Fatal("decryptOrPlaintext: expected error for corrupt ciphertext, got nil")
	}
}

func TestEncryptSourcePassRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")

	stored, err := EncryptSourcePass(key, "hackme")
	if err != nil {
		t.Fatalf("EncryptSourcePass: unexpected error: %v", err)
	}
	if !strings.HasPrefix(stored, "enc:") {
		t.Fatalf("EncryptSourcePass: result %q missing enc: prefix", stored)
	}

	plain, err := decryptOrPlaintext(stored, key)
	if err != nil {
		t.Fatalf("decryptOrPlaintext: unexpected error: %v", err)
	}
	if plain != "hackme" {
		t.Errorf("round trip: got %q, want %q", plain, "hackme")
	}
}

func TestEncryptSourcePassNoKeyPassthrough(t *testing.T) {
	got, err := EncryptSourcePass(nil, "hackme")
	if err != nil {
		t.Fatalf("EncryptSourcePass: unexpected error: %v", err)
	}
	if got != "hackme" {
		t.Errorf("EncryptSourcePass with no key: got %q, want plaintext passthrough %q", got, "hackme")
	}
}

func TestEncryptSourcePassEmptyPassthrough(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	got, err := EncryptSourcePass(key, "")
	if err != nil {
		t.Fatalf("EncryptSourcePass: unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("EncryptSourcePass with empty plaintext: got %q, want empty", got)
	}
}
