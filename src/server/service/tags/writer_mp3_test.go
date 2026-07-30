package tags

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bogem/id3v2/v2"
)

func TestWriteMP3HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.mp3")
	// bogem/id3v2 constructs a fresh tag when no existing ID3v2 header is
	// found, so an empty file is a valid starting point.
	if err := os.WriteFile(path, []byte{}, 0600); err != nil {
		t.Fatalf("create empty mp3: %v", err)
	}

	fields := WritableFields{
		"Title":           "Test Title",
		"Artist":          "Test Artist",
		"Album":           "Test Album",
		"AlbumArtist":     "Test Album Artist",
		"Year":            "2024",
		"Genre":           "Rock",
		"TrackNumber":     3,
		"DiscNumber":      1,
		"Composer":        "Test Composer",
		"Lyricist":        "Test Lyricist",
		"Conductor":       "Test Conductor",
		"BPM":             120,
		"Comment":         "Test Comment",
		"Lyrics":          "Test Lyrics",
		"MBTrackID":       "track-mbid",
		"MBAlbumID":       "album-mbid",
		"MBArtistID":      "artist-mbid",
		"MBAlbumArtistID": "albumartist-mbid",
	}

	if err := writeMP3(path, fields); err != nil {
		t.Fatalf("writeMP3: unexpected error: %v", err)
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("re-open written mp3: %v", err)
	}
	defer tag.Close()

	if got := tag.Title(); got != "Test Title" {
		t.Errorf("Title: got %q, want %q", got, "Test Title")
	}
	if got := tag.Artist(); got != "Test Artist" {
		t.Errorf("Artist: got %q, want %q", got, "Test Artist")
	}
	if got := tag.Album(); got != "Test Album" {
		t.Errorf("Album: got %q, want %q", got, "Test Album")
	}
	if got := tag.Year(); got != "2024" {
		t.Errorf("Year: got %q, want %q", got, "2024")
	}
	if got := tag.Genre(); got != "Rock" {
		t.Errorf("Genre: got %q, want %q", got, "Rock")
	}
	if got := tag.GetTextFrame("TPE2").Text; got != "Test Album Artist" {
		t.Errorf("TPE2 (AlbumArtist): got %q, want %q", got, "Test Album Artist")
	}
	if got := tag.GetTextFrame("TRCK").Text; got != "3" {
		t.Errorf("TRCK (TrackNumber): got %q, want %q", got, "3")
	}
	if got := tag.GetTextFrame("TPOS").Text; got != "1" {
		t.Errorf("TPOS (DiscNumber): got %q, want %q", got, "1")
	}
	if got := tag.GetTextFrame("TCOM").Text; got != "Test Composer" {
		t.Errorf("TCOM (Composer): got %q, want %q", got, "Test Composer")
	}
	if got := tag.GetTextFrame("TEXT").Text; got != "Test Lyricist" {
		t.Errorf("TEXT (Lyricist): got %q, want %q", got, "Test Lyricist")
	}
	if got := tag.GetTextFrame("TPE3").Text; got != "Test Conductor" {
		t.Errorf("TPE3 (Conductor): got %q, want %q", got, "Test Conductor")
	}
	if got := tag.GetTextFrame("TBPM").Text; got != "120" {
		t.Errorf("TBPM (BPM): got %q, want %q", got, "120")
	}

	comments := tag.GetFrames(tag.CommonID("Comments"))
	if len(comments) != 1 {
		t.Fatalf("Comments: got %d frames, want 1", len(comments))
	}
	if cf, ok := comments[0].(id3v2.CommentFrame); !ok || cf.Text != "Test Comment" {
		t.Errorf("Comments: got %+v, want text %q", comments[0], "Test Comment")
	}

	lyrics := tag.GetFrames(tag.CommonID("Unsynchronised lyrics/text transcription"))
	if len(lyrics) != 1 {
		t.Fatalf("Lyrics: got %d frames, want 1", len(lyrics))
	}
	if lf, ok := lyrics[0].(id3v2.UnsynchronisedLyricsFrame); !ok || lf.Lyrics != "Test Lyrics" {
		t.Errorf("Lyrics: got %+v, want text %q", lyrics[0], "Test Lyrics")
	}

	udtFrames := tag.GetFrames(tag.CommonID("User defined text information frame"))
	found := map[string]string{}
	for _, f := range udtFrames {
		if udtf, ok := f.(id3v2.UserDefinedTextFrame); ok {
			found[udtf.Description] = udtf.Value
		}
	}
	wantUDT := map[string]string{
		"MusicBrainz Track Id":        "track-mbid",
		"MusicBrainz Album Id":        "album-mbid",
		"MusicBrainz Artist Id":       "artist-mbid",
		"MusicBrainz Album Artist Id": "albumartist-mbid",
	}
	for desc, want := range wantUDT {
		if got := found[desc]; got != want {
			t.Errorf("UDT frame %q: got %q, want %q", desc, got, want)
		}
	}
}

func TestWriteMP3IgnoresUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.mp3")
	if err := os.WriteFile(path, []byte{}, 0600); err != nil {
		t.Fatalf("create empty mp3: %v", err)
	}

	if err := writeMP3(path, WritableFields{"NotARealField": "value"}); err != nil {
		t.Fatalf("writeMP3 with unknown field: unexpected error: %v", err)
	}
}

func TestWriteMP3OpenError(t *testing.T) {
	dir := t.TempDir()
	// A path whose parent directory does not exist cannot be opened by id3v2.
	path := filepath.Join(dir, "missing-parent", "song.mp3")

	err := writeMP3(path, WritableFields{"Title": "x"})
	if err == nil {
		t.Fatal("writeMP3 with unopenable path: expected error, got nil")
	}
}
