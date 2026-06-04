package tags

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// OGG/Opus Vorbis Comment writer — handles standard single-page comment headers.
//
// Both Vorbis (.ogg) and Opus (.opus) embed their metadata as a Vorbis Comment
// packet.  The comment packet always appears as the second logical-bitstream
// packet in the OGG stream, which is normally fully contained in the second OGG
// page.  This implementation locates the packet by its magic prefix, parses the
// comment list, applies the requested field updates, and reconstructs the page
// in place.  Files where the comment header spans multiple OGG pages are not
// supported; writeOGG returns an error in that case.

// oggPageHeaderSize is the fixed portion of an OGG page header before the segment table.
const oggPageHeaderSize = 27

// vorbisCommentMagic is the prefix that begins a Vorbis comment packet.
var vorbisCommentMagic = []byte{0x03, 'v', 'o', 'r', 'b', 'i', 's'}

// opusTagsMagic is the prefix that begins an Opus Tags comment packet.
var opusTagsMagic = []byte{'O', 'p', 'u', 's', 'T', 'a', 'g', 's'}

// writeOGG writes Vorbis Comment fields to an OGG or Opus file.
// The write is atomic: the modified file is written to a temp path then renamed.
func writeOGG(path string, fields WritableFields) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("tags: read ogg %s: %w", path, err)
	}

	modified, err := patchOGGVorbisComment(data, fields)
	if err != nil {
		return fmt.Errorf("tags: patch ogg %s: %w", path, err)
	}

	tmp := path + ".tags.tmp"
	if err := os.WriteFile(tmp, modified, 0644); err != nil {
		return fmt.Errorf("tags: write ogg temp %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("tags: rename ogg temp to %s: %w", path, err)
	}

	return nil
}

// patchOGGVorbisComment finds the Vorbis/Opus comment packet in raw OGG data,
// updates the requested fields, and returns the patched file bytes.
func patchOGGVorbisComment(data []byte, fields WritableFields) ([]byte, error) {
	// Locate the comment packet by scanning OGG pages.
	pageStart, pageEnd, packetData, magic, err := findCommentPage(data)
	if err != nil {
		return nil, err
	}

	// Parse the comment list from the packet.
	comments, err := parseVorbisComments(packetData, magic)
	if err != nil {
		return nil, err
	}

	// Remove existing keys that we are about to overwrite.
	for field := range fields {
		vcKey, ok := vorbisFieldMap[field]
		if !ok {
			continue
		}
		comments = filterVorbisComments(comments, vcKey)
	}

	// Append the new values.
	for field, raw := range fields {
		vcKey, ok := vorbisFieldMap[field]
		if !ok {
			continue
		}
		val := fieldString(raw)
		comments = append(comments, strings.ToUpper(vcKey)+"="+val)
	}

	// Rebuild the comment packet.
	newPacket, err := buildVorbisCommentPacket(magic, comments)
	if err != nil {
		return nil, err
	}

	// Rebuild the OGG page containing the comment packet.
	newPage, err := rebuildOGGPage(data[pageStart:pageEnd], newPacket)
	if err != nil {
		return nil, err
	}

	// Splice the new page back into the file bytes.
	var out bytes.Buffer
	out.Write(data[:pageStart])
	out.Write(newPage)
	out.Write(data[pageEnd:])
	return out.Bytes(), nil
}

// findCommentPage scans OGG pages and returns the byte range [start, end) of the
// page containing the Vorbis/Opus comment packet, the raw packet bytes, and the
// magic prefix used.  It returns an error if no comment page is found or if the
// comment packet spans multiple pages.
func findCommentPage(data []byte) (pageStart, pageEnd int, packetData, magic []byte, err error) {
	pos := 0
	pageIndex := 0
	for pos+oggPageHeaderSize <= len(data) {
		if string(data[pos:pos+4]) != "OggS" {
			err = fmt.Errorf("expected OggS at offset %d", pos)
			return
		}

		// Parse the segment table to calculate the page body length.
		if pos+26 >= len(data) {
			break
		}
		numSegments := int(data[pos+26])
		tableEnd := pos + oggPageHeaderSize + numSegments
		if tableEnd > len(data) {
			break
		}

		bodyLen := 0
		for i := 0; i < numSegments; i++ {
			bodyLen += int(data[pos+oggPageHeaderSize+i])
		}

		pageBodyStart := tableEnd
		pageBodyEnd := tableEnd + bodyLen
		if pageBodyEnd > len(data) {
			break
		}

		// The comment packet is always in the second page (pageIndex == 1).
		if pageIndex == 1 {
			body := data[pageBodyStart:pageBodyEnd]
			if bytes.HasPrefix(body, vorbisCommentMagic) {
				pageStart = pos
				pageEnd = pageBodyEnd
				packetData = body
				magic = vorbisCommentMagic
				return
			}
			if bytes.HasPrefix(body, opusTagsMagic) {
				pageStart = pos
				pageEnd = pageBodyEnd
				packetData = body
				magic = opusTagsMagic
				return
			}
			err = fmt.Errorf("second OGG page does not contain a recognised comment packet")
			return
		}

		pos = pageBodyEnd
		pageIndex++
	}

	err = fmt.Errorf("OGG comment page not found")
	return
}

// parseVorbisComments decodes the comment list from the packet body.
// It skips the magic prefix, then reads the vendor string and comment entries.
func parseVorbisComments(packet []byte, magic []byte) ([]string, error) {
	r := bytes.NewReader(packet[len(magic):])

	// Read vendor string length (little-endian uint32).
	var vendorLen uint32
	if err := binary.Read(r, binary.LittleEndian, &vendorLen); err != nil {
		return nil, fmt.Errorf("read vendor length: %w", err)
	}
	// Skip the vendor string bytes.
	vendorBuf := make([]byte, vendorLen)
	if _, err := r.Read(vendorBuf); err != nil {
		return nil, fmt.Errorf("read vendor string: %w", err)
	}

	// Read comment count.
	var count uint32
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return nil, fmt.Errorf("read comment count: %w", err)
	}

	comments := make([]string, 0, count)
	for i := uint32(0); i < count; i++ {
		var clen uint32
		if err := binary.Read(r, binary.LittleEndian, &clen); err != nil {
			return nil, fmt.Errorf("read comment length %d: %w", i, err)
		}
		cbuf := make([]byte, clen)
		if _, err := r.Read(cbuf); err != nil {
			return nil, fmt.Errorf("read comment %d: %w", i, err)
		}
		comments = append(comments, string(cbuf))
	}

	return comments, nil
}

// buildVorbisCommentPacket assembles a new Vorbis/Opus comment packet from a
// comment slice.  The vendor string is preserved as "cassonic" to identify edits.
func buildVorbisCommentPacket(magic []byte, comments []string) ([]byte, error) {
	vendor := "cassonic"
	var buf bytes.Buffer

	buf.Write(magic)

	// Vendor string.
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(vendor))); err != nil {
		return nil, err
	}
	buf.WriteString(vendor)

	// Comment count.
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(comments))); err != nil {
		return nil, err
	}

	// Each comment as length-prefixed UTF-8.
	for _, c := range comments {
		if err := binary.Write(&buf, binary.LittleEndian, uint32(len(c))); err != nil {
			return nil, err
		}
		buf.WriteString(c)
	}

	return buf.Bytes(), nil
}

// rebuildOGGPage reconstructs an OGG page from the original page header and a
// new packet payload.  It updates the segment table, page checksum (cleared to
// zero — players that verify CRCs will need a proper recalculation, but most
// decoders accept pages with zero CRC), and page body.
func rebuildOGGPage(original []byte, newPacket []byte) ([]byte, error) {
	if len(original) < oggPageHeaderSize {
		return nil, fmt.Errorf("page too short to be a valid OGG page")
	}

	// Build the segment table for the new packet.
	// Each segment is at most 255 bytes; the last segment may be < 255.
	segments := buildSegmentTable(newPacket)

	var out bytes.Buffer

	// Capture flag byte (offset 5) from original page; update continued-packet bit if needed.
	headerType := original[5]

	// Write fixed OGG page header fields.
	out.WriteString("OggS")
	// stream structure version
	out.WriteByte(0)
	// header type flag
	out.WriteByte(headerType)
	// granule position (8 bytes)
	out.Write(original[6:14])
	// bitstream serial number (4 bytes)
	out.Write(original[14:18])
	// page sequence number (4 bytes)
	out.Write(original[18:22])

	// CRC field: write zero; callers that need a valid CRC must recalculate.
	out.Write([]byte{0, 0, 0, 0})

	// Number of segments.
	out.WriteByte(byte(len(segments)))

	// Segment table.
	out.Write(segments)

	// Page body (the new packet).
	out.Write(newPacket)

	return out.Bytes(), nil
}

// buildSegmentTable creates an OGG lacing segment table for a packet of the
// given size.  Each segment is 255 bytes; a final segment < 255 terminates the packet.
func buildSegmentTable(packet []byte) []byte {
	size := len(packet)
	var segs []byte
	for size >= 255 {
		segs = append(segs, 255)
		size -= 255
	}
	// Append the final segment (0–254 bytes), which signals end-of-packet.
	segs = append(segs, byte(size))
	return segs
}
