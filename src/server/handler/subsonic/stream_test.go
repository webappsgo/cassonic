package subsonic

// Tests for stream.go handler methods: stream, download, hls, getCoverArt,
// getLyrics, getAvatar, getCaptions, and the serveStream/streamDirect pipeline
// (transcoding via streamTranscoded is not exercised since it requires a real
// ffmpeg binary; the h.ffmpeg == nil fallback path is covered instead).

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/service"
	"github.com/local/cassonic/src/server/store"
)

// ---- stub MusicStore extension for stream tests -----------------------------

// stubSongMusicStore extends stubMusicStore to control single-entity lookups
// (GetSong, GetAlbum, GetArtist, GetCoverArt, ListSongsByAlbum) independently,
// since stubMusicStore always returns "not implemented" for these.
type stubSongMusicStore struct {
	stubMusicStore

	song    *model.Song
	songErr error

	album    *model.Album
	albumErr error

	artist    *model.Artist
	artistErr error

	coverArt    *model.CoverArt
	coverArtErr error

	songsByAlbum    []*model.Song
	songsByAlbumErr error

	incrementErr error
}

func (s *stubSongMusicStore) GetSong(_ context.Context, _ int64) (*model.Song, error) {
	return s.song, s.songErr
}
func (s *stubSongMusicStore) GetAlbum(_ context.Context, _ int64) (*model.Album, error) {
	return s.album, s.albumErr
}
func (s *stubSongMusicStore) GetArtist(_ context.Context, _ int64) (*model.Artist, error) {
	return s.artist, s.artistErr
}
func (s *stubSongMusicStore) GetCoverArt(_ context.Context, _ int64) (*model.CoverArt, error) {
	return s.coverArt, s.coverArtErr
}
func (s *stubSongMusicStore) ListSongsByAlbum(_ context.Context, _ int64) ([]*model.Song, error) {
	return s.songsByAlbum, s.songsByAlbumErr
}
func (s *stubSongMusicStore) IncrementPlayCount(_ context.Context, _ int64) error {
	return s.incrementErr
}

// newStreamTestHandler builds a Handler with a stubSongMusicStore and,
// optionally, a real CoverArtService rooted at a scratch thumbnail directory.
func newStreamTestHandler(t *testing.T, music *stubSongMusicStore, users *stubUserStore, withCoverArt bool) *Handler {
	t.Helper()
	if music == nil {
		music = &stubSongMusicStore{}
	}
	if users == nil {
		users = &stubUserStore{}
	}
	db := &store.DB{
		Users:    users,
		Music:    music,
		Activity: &stubActivityStore{},
	}
	h := &Handler{
		db:         db,
		nowPlaying: NewNowPlayingTracker(),
		subsPass:   func(_ context.Context, _ string) (string, bool) { return "", false },
	}
	if withCoverArt {
		h.coverArt = service.NewCoverArtService(music, t.TempDir())
	}
	return h
}

// writeTempSongFile writes content to a temp file and returns its path.
func writeTempSongFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "track.mp3")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTempSongFile: %v", err)
	}
	return path
}

// writeTempCoverFile writes a minimal valid JPEG named cover.jpg into a fresh
// temp directory and returns the directory path.
func writeTempCoverFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cover.jpg"), buf.Bytes(), 0644); err != nil {
		t.Fatalf("writeTempCoverFile: %v", err)
	}
	return dir
}

// ---- stream -----------------------------------------------------------------

func TestStreamUnauthenticated(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, false)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/stream?f=json&id=so-1", nil)

	h.stream(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("stream unauthenticated: got %+v", resp.Error)
	}
}

func TestStreamMissingID(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/stream?f=json", false)

	h.stream(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("stream missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestStreamInvalidID(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/stream?f=json&id=!!!", false)

	h.stream(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("stream invalid id: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestStreamSongNotFound(t *testing.T) {
	music := &stubSongMusicStore{songErr: errors.New("not found")}
	h := newStreamTestHandler(t, music, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/stream?f=json&id=so-1", false)

	h.stream(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("stream song not found: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestStreamSuccessDirect(t *testing.T) {
	path := writeTempSongFile(t, "fake-audio-bytes")
	music := &stubSongMusicStore{song: &model.Song{
		ID: 1, Title: "Track", ArtistName: "Artist", AlbumName: "Album",
		Path: path, ContentType: "audio/mpeg", BitRate: 128,
	}}
	h := newStreamTestHandler(t, music, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/stream?id=so-1", false)

	h.stream(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("stream: status %d, want 200", rec.Code)
	}
	if rec.Body.String() != "fake-audio-bytes" {
		t.Errorf("stream: body %q, want fake-audio-bytes", rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "audio/mpeg" {
		t.Errorf("stream: Content-Type = %q, want audio/mpeg", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Errorf("stream: Accept-Ranges = %q, want bytes", rec.Header().Get("Accept-Ranges"))
	}
}

func TestStreamFileNotAccessible(t *testing.T) {
	music := &stubSongMusicStore{song: &model.Song{ID: 1, Path: "/nonexistent/path/track.mp3"}}
	h := newStreamTestHandler(t, music, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/stream?id=so-1", false)

	h.stream(rec, r)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("stream missing file: status %d, want 500", rec.Code)
	}
}

// ---- download -----------------------------------------------------------------

func TestDownloadUnauthenticated(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, false)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/download?f=json&id=so-1", nil)

	h.download(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("download unauthenticated: got %+v", resp.Error)
	}
}

func TestDownloadForbiddenNonAdminNoPermission(t *testing.T) {
	users := &stubUserStore{byUsername: &model.User{ID: 1, Username: "testuser", CanDownload: false}}
	h := newStreamTestHandler(t, nil, users, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/download?f=json&id=so-1", false)

	h.download(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrForbidden {
		t.Errorf("download forbidden: got %+v, want code %d", resp.Error, ErrForbidden)
	}
}

func TestDownloadMissingID(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/download?f=json", true)

	h.download(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("download missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestDownloadInvalidID(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/download?f=json&id=!!!", true)

	h.download(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("download invalid id: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestDownloadSongNotFound(t *testing.T) {
	music := &stubSongMusicStore{songErr: errors.New("not found")}
	h := newStreamTestHandler(t, music, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/download?f=json&id=so-1", true)

	h.download(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("download song not found: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestDownloadFileNotAccessible(t *testing.T) {
	music := &stubSongMusicStore{song: &model.Song{ID: 1, Path: "/nonexistent/path/track.mp3"}}
	h := newStreamTestHandler(t, music, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/download?f=json&id=so-1", true)

	h.download(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrGeneric {
		t.Errorf("download inaccessible file: got %+v, want code %d", resp.Error, ErrGeneric)
	}
}

func TestDownloadSuccessAdmin(t *testing.T) {
	path := writeTempSongFile(t, "download-bytes")
	music := &stubSongMusicStore{song: &model.Song{
		ID: 1, Title: "Track", Path: path, ContentType: "audio/mpeg",
	}}
	h := newStreamTestHandler(t, music, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/download?id=so-1", true)

	h.download(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("download: status %d, want 200", rec.Code)
	}
	if rec.Body.String() != "download-bytes" {
		t.Errorf("download: body %q", rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "Track.mp3") {
		t.Errorf("download: Content-Disposition = %q", rec.Header().Get("Content-Disposition"))
	}
}

func TestDownloadSuccessNonAdminWithPermission(t *testing.T) {
	path := writeTempSongFile(t, "download-bytes")
	music := &stubSongMusicStore{song: &model.Song{ID: 1, Title: "Track", Path: path, ContentType: "audio/mpeg"}}
	users := &stubUserStore{byUsername: &model.User{ID: 1, Username: "testuser", CanDownload: true}}
	h := newStreamTestHandler(t, music, users, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/download?id=so-1", false)

	h.download(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("download: status %d, want 200", rec.Code)
	}
}

// ---- hls ------------------------------------------------------------------

func TestHLSUnauthenticated(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, false)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/hls?f=json&id=so-1", nil)

	h.hls(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("hls unauthenticated: got %+v", resp.Error)
	}
}

func TestHLSMissingID(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/hls?f=json", false)

	h.hls(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrMissingParam {
		t.Errorf("hls missing id: got %+v, want code %d", resp.Error, ErrMissingParam)
	}
}

func TestHLSSongNotFound(t *testing.T) {
	music := &stubSongMusicStore{songErr: errors.New("not found")}
	h := newStreamTestHandler(t, music, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/hls?f=json&id=so-1", false)

	h.hls(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("hls song not found: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

func TestHLSSuccess(t *testing.T) {
	music := &stubSongMusicStore{song: &model.Song{ID: 1, Duration: 180}}
	h := newStreamTestHandler(t, music, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/hls?id=so-1&u=testuser", false)

	h.hls(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("hls: status %d, want 200", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/vnd.apple.mpegurl" {
		t.Errorf("hls: Content-Type = %q", rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if !strings.Contains(body, "#EXTM3U") || !strings.Contains(body, "#EXT-X-TARGETDURATION:180") {
		t.Errorf("hls: body missing expected fields: %q", body)
	}
	if !strings.Contains(body, "/rest/stream?id=so-1") {
		t.Errorf("hls: body missing stream URL: %q", body)
	}
}

// ---- getCoverArt ----------------------------------------------------------

func TestGetCoverArtUnauthenticated(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, true)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getCoverArt?id=al-1", nil)

	h.getCoverArt(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("getCoverArt unauthenticated: status %d, want 401", rec.Code)
	}
}

func TestGetCoverArtMissingID(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, true)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getCoverArt", false)

	h.getCoverArt(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getCoverArt missing id: status %d, want 404", rec.Code)
	}
}

func TestGetCoverArtInvalidID(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, true)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getCoverArt?id=!!!", false)

	h.getCoverArt(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getCoverArt invalid id: status %d, want 404", rec.Code)
	}
}

func TestGetCoverArtAlbumNotFound(t *testing.T) {
	music := &stubSongMusicStore{album: nil}
	h := newStreamTestHandler(t, music, nil, true)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getCoverArt?id=al-1", false)

	h.getCoverArt(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getCoverArt album not found: status %d, want 404", rec.Code)
	}
}

func TestGetCoverArtAlbumStoreError(t *testing.T) {
	music := &stubSongMusicStore{albumErr: errors.New("db error")}
	h := newStreamTestHandler(t, music, nil, true)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getCoverArt?id=al-1", false)

	h.getCoverArt(rec, r)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("getCoverArt album store error: status %d, want 500", rec.Code)
	}
}

func TestGetCoverArtSongSuccess(t *testing.T) {
	dir := writeTempCoverFile(t)
	music := &stubSongMusicStore{song: &model.Song{ID: 1, Path: filepath.Join(dir, "track.mp3")}}
	h := newStreamTestHandler(t, music, nil, true)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getCoverArt?id=so-1", false)

	h.getCoverArt(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("getCoverArt song: status %d, want 200", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("getCoverArt song: Content-Type = %q", rec.Header().Get("Content-Type"))
	}
	if rec.Body.Len() == 0 {
		t.Error("getCoverArt song: empty body")
	}
}

func TestGetCoverArtSongNoCover(t *testing.T) {
	dir := t.TempDir()
	music := &stubSongMusicStore{song: &model.Song{ID: 1, Path: filepath.Join(dir, "track.mp3")}}
	h := newStreamTestHandler(t, music, nil, true)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getCoverArt?id=so-1", false)

	h.getCoverArt(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getCoverArt no cover: status %d, want 404", rec.Code)
	}
}

func TestGetCoverArtArtistNotFound(t *testing.T) {
	music := &stubSongMusicStore{artistErr: errors.New("not found")}
	h := newStreamTestHandler(t, music, nil, true)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getCoverArt?id=ar-1", false)

	h.getCoverArt(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("getCoverArt artist not found: status %d, want 404", rec.Code)
	}
}

// ---- getLyrics --------------------------------------------------------------

func TestGetLyricsUnauthenticated(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, false)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getLyrics?f=json", nil)

	h.getLyrics(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotAuthenticated {
		t.Errorf("getLyrics unauthenticated: got %+v", resp.Error)
	}
}

func TestGetLyricsNoArtistOrTitle(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getLyrics?f=json", false)

	h.getLyrics(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Status != "ok" {
		t.Fatalf("getLyrics: status %q, want ok", resp.Status)
	}
	if resp.Lyrics == nil || resp.Lyrics.Value != "" {
		t.Errorf("getLyrics: got %+v, want empty lyrics", resp.Lyrics)
	}
}

// ---- getAvatar --------------------------------------------------------------

func TestGetAvatarUnauthenticated(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, false)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getAvatar", nil)

	h.getAvatar(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("getAvatar unauthenticated: status %d, want 401", rec.Code)
	}
}

func TestGetAvatarSuccess(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, false)
	rec := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/rest/getAvatar", false)

	h.getAvatar(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("getAvatar: status %d, want 200", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "image/svg+xml" {
		t.Errorf("getAvatar: Content-Type = %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "<svg") {
		t.Errorf("getAvatar: body doesn't contain svg: %q", rec.Body.String())
	}
}

// ---- getCaptions -----------------------------------------------------------

func TestGetCaptionsAlwaysNotFound(t *testing.T) {
	h := newStreamTestHandler(t, nil, nil, false)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rest/getCaptions?f=json", nil)

	h.getCaptions(rec, r)

	resp := parseJSONResponse(t, rec.Body.String())
	if resp.Error == nil || resp.Error.Code != ErrNotFound {
		t.Errorf("getCaptions: got %+v, want code %d", resp.Error, ErrNotFound)
	}
}

// ---- formatToMIME -----------------------------------------------------------

func TestFormatToMIME(t *testing.T) {
	cases := map[string]string{
		"mp3":     "audio/mpeg",
		"MP3":     "audio/mpeg",
		"ogg":     "audio/ogg",
		"opus":    "audio/ogg; codecs=opus",
		"aac":     "audio/aac",
		"flac":    "audio/flac",
		"unknown": "audio/mpeg",
		"":        "audio/mpeg",
	}
	for format, want := range cases {
		if got := formatToMIME(format); got != want {
			t.Errorf("formatToMIME(%q) = %q, want %q", format, got, want)
		}
	}
}
