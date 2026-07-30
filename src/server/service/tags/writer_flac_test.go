package tags

import (
	"os"
	"path/filepath"
	"testing"

	flac "github.com/go-flac/go-flac"
	"github.com/go-flac/flacvorbis"
)

// minimalFLACBytes builds the smallest byte sequence go-flac/go-flac will
// accept: the "fLaC" magic, a single (final) STREAMINFO metadata block filled
// with zeros, and a trailing 4-byte stub that satisfies the frame sync-code
// check in readFLACStream (0xFF followed by the top six bits 111110). No
// real audio data is required because the library never decodes frames for
// tag editing — it only re-emits whatever bytes follow the metadata blocks.
func minimalFLACBytes() []byte {
	out := []byte("fLaC")
	// STREAMINFO block header: type 0, final bit set, 34-byte body.
	out = append(out, 0x80, 0x00, 0x00, 0x22)
	out = append(out, make([]byte, 34)...)
	// Frame sync stub so readFLACStream's magic check passes.
	out = append(out, 0xFF, 0xF8, 0x00, 0x00)
	return out
}

func TestWriteFLACHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.flac")
	if err := os.WriteFile(path, minimalFLACBytes(), 0600); err != nil {
		t.Fatalf("create minimal flac: %v", err)
	}

	fields := WritableFields{
		"Title":     "Test Title",
		"Artist":    "Test Artist",
		"Album":     "Test Album",
		"Year":      "2024",
		"BPM":       128,
		"MBTrackID": "track-mbid",
	}

	if err := writeFLAC(path, fields); err != nil {
		t.Fatalf("writeFLAC: unexpected error: %v", err)
	}

	ff, err := flac.ParseFile(path)
	if err != nil {
		t.Fatalf("re-parse written flac: %v", err)
	}

	var vc *flacvorbis.MetaDataBlockVorbisComment
	for _, block := range ff.Meta {
		if block.Type == flac.VorbisComment {
			vc, err = flacvorbis.ParseFromMetaDataBlock(*block)
			if err != nil {
				t.Fatalf("parse vorbis comment: %v", err)
			}
			break
		}
	}
	if vc == nil {
		t.Fatal("no VorbisComment block found after write")
	}

	want := map[string]string{
		"TITLE":     "Test Title",
		"ARTIST":    "Test Artist",
		"ALBUM":     "Test Album",
		"DATE":      "2024",
		"BPM":       "128",
		"MUSICBRAINZ_TRACKID": "track-mbid",
	}
	for key, wantVal := range want {
		gotVals, err := vc.Get(key)
		if err != nil || len(gotVals) == 0 {
			t.Errorf("vorbis comment %q: not found (err=%v)", key, err)
			continue
		}
		if gotVals[0] != wantVal {
			t.Errorf("vorbis comment %q: got %q, want %q", key, gotVals[0], wantVal)
		}
	}
}

func TestWriteFLACOverwritesExistingComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.flac")
	if err := os.WriteFile(path, minimalFLACBytes(), 0600); err != nil {
		t.Fatalf("create minimal flac: %v", err)
	}

	if err := writeFLAC(path, WritableFields{"Title": "First"}); err != nil {
		t.Fatalf("first writeFLAC: unexpected error: %v", err)
	}
	if err := writeFLAC(path, WritableFields{"Title": "Second"}); err != nil {
		t.Fatalf("second writeFLAC: unexpected error: %v", err)
	}

	ff, err := flac.ParseFile(path)
	if err != nil {
		t.Fatalf("re-parse written flac: %v", err)
	}
	var vc *flacvorbis.MetaDataBlockVorbisComment
	for _, block := range ff.Meta {
		if block.Type == flac.VorbisComment {
			vc, err = flacvorbis.ParseFromMetaDataBlock(*block)
			if err != nil {
				t.Fatalf("parse vorbis comment: %v", err)
			}
			break
		}
	}
	if vc == nil {
		t.Fatal("no VorbisComment block found after write")
	}
	got, err := vc.Get("TITLE")
	if err != nil || len(got) != 1 || got[0] != "Second" {
		t.Errorf("TITLE after overwrite: got %v (err=%v), want single value %q", got, err, "Second")
	}
}

func TestWriteFLACParseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.flac")
	if err := os.WriteFile(path, []byte("not a flac file"), 0600); err != nil {
		t.Fatalf("create garbage file: %v", err)
	}

	err := writeFLAC(path, WritableFields{"Title": "x"})
	if err == nil {
		t.Fatal("writeFLAC on garbage data: expected error, got nil")
	}
}

func TestFilterVorbisComments(t *testing.T) {
	comments := []string{"TITLE=Old", "ARTIST=Someone", "title=Duplicate"}
	got := filterVorbisComments(comments, "TITLE")
	want := []string{"ARTIST=Someone"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("filterVorbisComments: got %v, want %v", got, want)
	}
}
