package tags

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// buildOGGPage assembles a single minimal OGG page with the given sequence
// number and body. numSegments and the segment table assume body fits in a
// single lacing value (<255 bytes), which is sufficient for these tests.
func buildOGGPage(seq uint32, body []byte) []byte {
	var out bytes.Buffer
	out.WriteString("OggS")
	out.WriteByte(0) // stream structure version
	out.WriteByte(2) // header type flag (2 = beginning-of-stream on page 0; harmless on others for this test)
	out.Write(make([]byte, 8))       // granule position
	binary.Write(&out, binary.LittleEndian, uint32(1)) // serial number
	binary.Write(&out, binary.LittleEndian, seq)       // page sequence number
	out.Write(make([]byte, 4))       // CRC (unused by the parser under test)
	out.WriteByte(1)                 // number of segments
	out.WriteByte(byte(len(body)))   // segment table: single segment
	out.Write(body)
	return out.Bytes()
}

// buildVorbisCommentPacketBytes mirrors the on-disk layout produced by
// buildVorbisCommentPacket, used here to construct fixture input independent
// of the code under test.
func buildVorbisCommentPacketBytes(magic []byte, vendor string, comments []string) []byte {
	var buf bytes.Buffer
	buf.Write(magic)
	binary.Write(&buf, binary.LittleEndian, uint32(len(vendor)))
	buf.WriteString(vendor)
	binary.Write(&buf, binary.LittleEndian, uint32(len(comments)))
	for _, c := range comments {
		binary.Write(&buf, binary.LittleEndian, uint32(len(c)))
		buf.WriteString(c)
	}
	return buf.Bytes()
}

// minimalOGGVorbisBytes builds a two-page OGG stream: an arbitrary first
// page (standing in for the identification header, which this writer never
// inspects) and a second page containing a Vorbis comment packet.
func minimalOGGVorbisBytes(comments []string) []byte {
	page0 := buildOGGPage(0, []byte{1, 2, 3, 4})
	packet := buildVorbisCommentPacketBytes(vorbisCommentMagic, "cassonic-test", comments)
	page1 := buildOGGPage(1, packet)
	return append(page0, page1...)
}

func TestWriteOGGHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.ogg")
	if err := os.WriteFile(path, minimalOGGVorbisBytes(nil), 0600); err != nil {
		t.Fatalf("create minimal ogg: %v", err)
	}

	fields := WritableFields{
		"Title":  "Test Title",
		"Artist": "Test Artist",
		"BPM":    140,
	}

	if err := writeOGG(path, fields); err != nil {
		t.Fatalf("writeOGG: unexpected error: %v", err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written ogg: %v", err)
	}

	_, _, packetData, magic, err := findCommentPage(out)
	if err != nil {
		t.Fatalf("re-find comment page: %v", err)
	}
	if !bytes.Equal(magic, vorbisCommentMagic) {
		t.Fatalf("magic mismatch after write")
	}
	comments, err := parseVorbisComments(packetData, magic)
	if err != nil {
		t.Fatalf("re-parse vorbis comments: %v", err)
	}

	want := map[string]bool{
		"TITLE=Test Title":  true,
		"ARTIST=Test Artist": true,
		"BPM=140":           true,
	}
	for _, c := range comments {
		delete(want, c)
	}
	if len(want) != 0 {
		t.Errorf("missing expected comments after write: %v (got %v)", want, comments)
	}
}

func TestWriteOGGOpusMagic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.opus")
	if err := os.WriteFile(path, minimalOGGVorbisBytes(nil), 0600); err != nil {
		t.Fatalf("create minimal ogg: %v", err)
	}
	// Rebuild using OpusTags magic instead of Vorbis magic for this format.
	page0 := buildOGGPage(0, []byte{1, 2, 3, 4})
	packet := buildVorbisCommentPacketBytes(opusTagsMagic, "cassonic-test", nil)
	page1 := buildOGGPage(1, packet)
	if err := os.WriteFile(path, append(page0, page1...), 0600); err != nil {
		t.Fatalf("rewrite as opus: %v", err)
	}

	if err := writeOGG(path, WritableFields{"Title": "Opus Title"}); err != nil {
		t.Fatalf("writeOGG on opus file: unexpected error: %v", err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written opus: %v", err)
	}
	_, _, packetData, magic, err := findCommentPage(out)
	if err != nil {
		t.Fatalf("re-find comment page: %v", err)
	}
	if !bytes.Equal(magic, opusTagsMagic) {
		t.Errorf("expected OpusTags magic to be preserved, got %q", magic)
	}
	comments, err := parseVorbisComments(packetData, magic)
	if err != nil {
		t.Fatalf("re-parse vorbis comments: %v", err)
	}
	found := false
	for _, c := range comments {
		if c == "TITLE=Opus Title" {
			found = true
		}
	}
	if !found {
		t.Errorf("TITLE comment not found in %v", comments)
	}
}

func TestWriteOGGReadError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.ogg")

	err := writeOGG(path, WritableFields{"Title": "x"})
	if err == nil {
		t.Fatal("writeOGG on nonexistent file: expected error, got nil")
	}
}

func TestWriteOGGNoCommentPageFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.ogg")
	// Only one page, so the required second (comment) page never appears.
	page0 := buildOGGPage(0, []byte{1, 2, 3, 4})
	if err := os.WriteFile(path, page0, 0600); err != nil {
		t.Fatalf("create single-page ogg: %v", err)
	}

	err := writeOGG(path, WritableFields{"Title": "x"})
	if err == nil {
		t.Fatal("writeOGG with no comment page: expected error, got nil")
	}
}

func TestWriteOGGSecondPageNotAComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.ogg")
	page0 := buildOGGPage(0, []byte{1, 2, 3, 4})
	page1 := buildOGGPage(1, []byte("not a comment packet"))
	if err := os.WriteFile(path, append(page0, page1...), 0600); err != nil {
		t.Fatalf("create malformed ogg: %v", err)
	}

	err := writeOGG(path, WritableFields{"Title": "x"})
	if err == nil {
		t.Fatal("writeOGG with non-comment second page: expected error, got nil")
	}
}

func TestWriteOGGGarbageData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.ogg")
	if err := os.WriteFile(path, []byte("this is not an ogg file at all"), 0600); err != nil {
		t.Fatalf("create garbage file: %v", err)
	}

	err := writeOGG(path, WritableFields{"Title": "x"})
	if err == nil {
		t.Fatal("writeOGG on garbage data: expected error, got nil")
	}
}

func TestBuildSegmentTable(t *testing.T) {
	tests := []struct {
		size int
		want []byte
	}{
		{0, []byte{0}},
		{10, []byte{10}},
		{255, []byte{255, 0}},
		{300, []byte{255, 45}},
	}
	for _, tt := range tests {
		got := buildSegmentTable(make([]byte, tt.size))
		if !bytes.Equal(got, tt.want) {
			t.Errorf("buildSegmentTable(size=%d): got %v, want %v", tt.size, got, tt.want)
		}
	}
}
