package ampache

import (
	"encoding/json"
	"encoding/xml"
	"net/http/httptest"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// TestErrResp covers the AmpError constructor and its JSON/XML wire shape.
func TestErrResp(t *testing.T) {
	e := errResp(4701, "Invalid action: bogus")
	if e.ErrorCode != 4701 || e.ErrorMessage != "Invalid action: bogus" {
		t.Fatalf("unexpected AmpError: %+v", e)
	}
}

// TestOkResp covers the success envelope builder.
func TestOkResp(t *testing.T) {
	m := okResp("song", "value")
	if m["song"] != "value" {
		t.Fatalf("okResp: got %v", m)
	}
	if len(m) != 1 {
		t.Fatalf("okResp: expected exactly one key, got %d", len(m))
	}
}

// TestRespondJSON_Error asserts the JSON error envelope shape and that HTTP
// status is always 200, per the Ampache protocol (errors are reported in-body).
func TestRespondJSON_Error(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/server/json.server.php", nil)
	respond(rec, req, true, errResp(4700, "Access denied"))

	if rec.Code != 200 {
		t.Fatalf("expected HTTP 200 even for protocol error, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("unexpected Content-Type: %s", ct)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got["errorCode"] != float64(4700) || got["errorMessage"] != "Access denied" {
		t.Fatalf("unexpected JSON error body: %v", got)
	}
}

// TestRespondJSON_OkEnvelope covers the ordinary success-envelope path.
func TestRespondJSON_OkEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/server/json.server.php", nil)
	respond(rec, req, true, okResp("version", "6.0.0"))

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got["version"] != "6.0.0" {
		t.Fatalf("unexpected JSON body: %v", got)
	}
}

// TestRespondXML_Error asserts the XML error envelope shape: <root><error errorCode=".."/></root>.
func TestRespondXML_Error(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/server/xml.server.php", nil)
	respond(rec, req, false, errResp(4701, "Invalid action: bogus"))

	if rec.Code != 200 {
		t.Fatalf("expected HTTP 200 even for protocol error, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/xml; charset=utf-8" {
		t.Fatalf("unexpected Content-Type: %s", ct)
	}
	var got xmlError
	decodeXMLRoot(t, rec.Body.Bytes(), &got)
	if got.Error.ErrorCode != 4701 || got.Error.ErrorMessage != "Invalid action: bogus" {
		t.Fatalf("unexpected XML error body: %+v", got)
	}
}

// TestRespondXML_OkEnvelope is a regression test for a genuine bug found while
// writing these tests: respondXML previously had no case for map[string]any
// (the type okResp always returns), and encoding/xml cannot marshal a raw
// map[string]interface{} ("xml: unsupported type"). That silently produced a
// truncated/empty XML body for every success response in XML mode — i.e. the
// entire non-JSON Ampache API surface. okEnvelope.MarshalXML fixes this by
// hand-walking the map into named elements under <root>.
func TestRespondXML_OkEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/server/xml.server.php", nil)
	respond(rec, req, false, okResp("success", "station deleted"))

	body := rec.Body.String()
	if body == "" {
		t.Fatal("respondXML produced an empty body for a map[string]any payload")
	}
	var got struct {
		XMLName xml.Name `xml:"root"`
		Success string   `xml:"success"`
	}
	decodeXMLRoot(t, rec.Body.Bytes(), &got)
	if got.Success != "station deleted" {
		t.Fatalf("unexpected XML ok body: %q, full body: %s", got.Success, body)
	}
}

// TestRespondXML_OkEnvelope_MultiKeyAndSlice covers a multi-key envelope with
// a slice-of-struct value (the shape returned by handlers like songs/albums).
func TestRespondXML_OkEnvelope_MultiKeyAndSlice(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/server/xml.server.php", nil)
	payload := map[string]any{
		"count": 2,
		"song":  []AmpSong{{ID: "1", Title: "A"}, {ID: "2", Title: "B"}},
	}
	respond(rec, req, false, payload)

	var got struct {
		XMLName xml.Name  `xml:"root"`
		Count   int       `xml:"count"`
		Songs   []AmpSong `xml:"song"`
	}
	decodeXMLRoot(t, rec.Body.Bytes(), &got)
	if got.Count != 2 || len(got.Songs) != 2 || got.Songs[0].ID != "1" || got.Songs[1].ID != "2" {
		t.Fatalf("unexpected multi-key XML body: %+v, raw: %s", got, rec.Body.String())
	}
}

// TestRespondXML_TypedStruct covers the default branch (a concrete struct
// with its own XMLName, e.g. AmpStats returned directly rather than via okResp).
func TestRespondXML_TypedStruct(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/server/xml.server.php", nil)
	respond(rec, req, false, &AmpStats{Songs: 5, Albums: 2, Artists: 1})

	var got AmpStats
	if err := xml.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("xml.Unmarshal: %v", err)
	}
	if got.Songs != 5 || got.Albums != 2 || got.Artists != 1 {
		t.Fatalf("unexpected AmpStats body: %+v", got)
	}
}

// TestBoolInt covers both branches of the trivial bool-to-int converter.
func TestBoolInt(t *testing.T) {
	if boolInt(true) != 1 {
		t.Fatal("boolInt(true) should be 1")
	}
	if boolInt(false) != 0 {
		t.Fatal("boolInt(false) should be 0")
	}
}

// TestItoa covers positive, negative, and zero boundary values.
func TestItoa(t *testing.T) {
	cases := map[int64]string{
		0:     "0",
		1:     "1",
		42:    "42",
		-7:    "-7",
		12345: "12345",
	}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Errorf("itoa(%d) = %q, want %q", in, got, want)
		}
	}
}

// TestSongToAmp covers the model.Song -> AmpSong conversion, including the
// empty-genre edge case (no genre string should produce an empty slice, not nil).
func TestSongToAmp(t *testing.T) {
	s := &model.Song{
		ID: 10, LibraryID: 1, Title: "Track", ArtistID: 2, ArtistName: "Artist",
		AlbumID: 3, AlbumName: "Album", AlbumArtistID: 2, AlbumArtistName: "Artist",
		DiscNumber: 1, TrackNumber: 4, Year: 2020, Genre: "Rock", Duration: 200,
		BitRate: 320, SampleRate: 44100, Channels: 2, ContentType: "audio/mpeg",
		FileSize: 5000, MBTrackID: "mbid-1", Composer: "Comp", Lyrics: "la la",
		ReplayGainTrack: -6.5,
	}
	amp := songToAmp(s, "http://example.com")
	if amp.ID != "10" || amp.Title != "Track" || amp.Name != "Track" {
		t.Fatalf("unexpected song id/title: %+v", amp)
	}
	if amp.Artist.ID != "2" || amp.Artist.Name != "Artist" {
		t.Fatalf("unexpected artist ref: %+v", amp.Artist)
	}
	if len(amp.Genre) != 1 || amp.Genre[0].Name != "Rock" {
		t.Fatalf("unexpected genre: %+v", amp.Genre)
	}
	if amp.URL != "http://example.com/server/json.server.php?action=stream&id=10" {
		t.Fatalf("unexpected stream URL: %s", amp.URL)
	}
	if amp.Art != "http://example.com/server/json.server.php?action=get_art&id=10&type=song" {
		t.Fatalf("unexpected art URL: %s", amp.Art)
	}

	empty := songToAmp(&model.Song{ID: 1}, "")
	if len(empty.Genre) != 0 {
		t.Fatalf("expected empty (non-nil) genre slice, got %+v", empty.Genre)
	}
}

// TestAlbumToAmp covers the model.Album -> AmpAlbum conversion.
func TestAlbumToAmp(t *testing.T) {
	a := &model.Album{ID: 5, Title: "LP", ArtistID: 2, ArtistName: "Artist", Year: 1999, Genre: "Jazz", SongCount: 9, Duration: 3000, MusicBrainzID: "mb-a"}
	amp := albumToAmp(a, "http://x")
	if amp.ID != "5" || amp.Name != "LP" || amp.Artist.ID != "2" {
		t.Fatalf("unexpected album conversion: %+v", amp)
	}
	if len(amp.Genre) != 1 || amp.Genre[0].Name != "Jazz" {
		t.Fatalf("unexpected album genre: %+v", amp.Genre)
	}
	if amp.Art != "http://x/server/json.server.php?action=get_art&id=5&type=album" {
		t.Fatalf("unexpected album art URL: %s", amp.Art)
	}

	empty := albumToAmp(&model.Album{ID: 1}, "")
	if len(empty.Genre) != 0 {
		t.Fatalf("expected empty genre slice, got %+v", empty.Genre)
	}
}

// TestArtistToAmp covers the model.Artist -> AmpArtist conversion.
func TestArtistToAmp(t *testing.T) {
	a := &model.Artist{ID: 7, Name: "Band", AlbumCount: 3, SongCount: 30, Biography: "bio", MusicBrainzID: "mb-r"}
	amp := artistToAmp(a, "http://x")
	if amp.ID != "7" || amp.Name != "Band" || amp.AlbumCount != 3 || amp.SongCount != 30 {
		t.Fatalf("unexpected artist conversion: %+v", amp)
	}
	if amp.Art != "http://x/server/json.server.php?action=get_art&id=7&type=artist" {
		t.Fatalf("unexpected artist art URL: %s", amp.Art)
	}
	if amp.Summary != "bio" || amp.MBID != "mb-r" {
		t.Fatalf("unexpected artist metadata: %+v", amp)
	}
}
