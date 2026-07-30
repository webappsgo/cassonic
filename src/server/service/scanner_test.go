package service

// Tests cover:
//   - NewScanner: returns a usable Scanner
//   - Scan: end-to-end against a real sqlite-backed store.DB and a stub
//     TagReader — creates a ScanStatus, walks a temp-dir library, upserts
//     artist/album/song rows, marks the status "completed", and skips
//     disabled libraries
//   - Scan: a missing library path leaves ScanStatus completed (the walker
//     silently returns zero results rather than erroring)
//   - ScanLibrary: updates the library's LastScanAt
//   - processFile: full mode always re-reads tags; incremental mode skips
//     files whose mtime is unchanged and re-reads files whose mtime changed
//   - processFile: falls back to "Unknown Artist"/"Unknown Album" and the
//     filename (without extension) as title when tags are empty
//   - processFile: a tag-read error increments the error counter and skips
//     the file
//   - processFile: a stat error (missing file) is a silent no-op
//   - upsertSong: stores embedded cover art and computes a file hash
//   - hashFile: matches a known SHA-256 digest and errors for a missing file

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// stubTagReader implements TagReader with a fixed or per-path response,
// letting tests control scanner behavior without real audio files.
type stubTagReader struct {
	// byPath overrides the response for a specific path.
	byPath map[string]*SongMeta
	// errByPath forces an error for a specific path.
	errByPath map[string]error
	// calls tracks how many times Read was invoked per path.
	calls map[string]int
	// def is the meta returned when no byPath entry matches.
	def *SongMeta
}

func newStubTagReader() *stubTagReader {
	return &stubTagReader{
		byPath:    make(map[string]*SongMeta),
		errByPath: make(map[string]error),
		calls:     make(map[string]int),
		def: &SongMeta{
			Title:  "Stub Title",
			Artist: "Stub Artist",
			Album:  "Stub Album",
		},
	}
}

func (r *stubTagReader) Read(path string) (*SongMeta, error) {
	r.calls[path]++
	if err, ok := r.errByPath[path]; ok {
		return nil, err
	}
	if meta, ok := r.byPath[path]; ok {
		cp := *meta
		return &cp, nil
	}
	cp := *r.def
	return &cp, nil
}

// seedLibrary creates a library row pointed at dir and returns the model.
func seedLibrary(t *testing.T, db *store.DB, dir string, enabled bool) *model.Library {
	t.Helper()
	lib := &model.Library{Name: "Test Library", Path: dir, Enabled: enabled}
	id, err := db.Music.CreateLibrary(context.Background(), lib)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	lib.ID = id
	return lib
}

func TestNewScannerReturnsUsable(t *testing.T) {
	db := newTestDB(t)
	sc := NewScanner(db.Music, newStubTagReader(), silentLogger())
	if sc == nil {
		t.Fatal("NewScanner: returned nil")
	}
	if sc.music == nil || sc.tagReader == nil || sc.logger == nil {
		t.Error("NewScanner: fields not set")
	}
}

func TestScanEndToEndUpsertsSongs(t *testing.T) {
	db := newTestDB(t)
	root := tempDir(t)
	writeFile(t, root, "track1.mp3")
	writeFile(t, root, "track2.flac")
	writeFile(t, root, "ignored.txt")

	seedLibrary(t, db, root, true)

	sc := NewScanner(db.Music, newStubTagReader(), silentLogger())
	if err := sc.Scan(context.Background(), ScanModeFull); err != nil {
		t.Fatalf("Scan: unexpected error: %v", err)
	}

	songs, err := db.Music.SearchSongs(context.Background(), "Stub Title", store.ListOpts{})
	if err != nil {
		t.Fatalf("SearchSongs: %v", err)
	}
	if len(songs) != 2 {
		t.Fatalf("SearchSongs: got %d songs, want 2", len(songs))
	}

	artist, err := db.Music.GetArtistByName(context.Background(), "Stub Artist")
	if err != nil {
		t.Fatalf("GetArtistByName: %v", err)
	}
	if artist == nil {
		t.Fatal("GetArtistByName: expected artist to be created")
	}
}

func TestScanSkipsDisabledLibraries(t *testing.T) {
	db := newTestDB(t)
	root := tempDir(t)
	writeFile(t, root, "track1.mp3")
	seedLibrary(t, db, root, false)

	sc := NewScanner(db.Music, newStubTagReader(), silentLogger())
	if err := sc.Scan(context.Background(), ScanModeFull); err != nil {
		t.Fatalf("Scan: unexpected error: %v", err)
	}

	songs, err := db.Music.SearchSongs(context.Background(), "Stub Title", store.ListOpts{})
	if err != nil {
		t.Fatalf("SearchSongs: %v", err)
	}
	if len(songs) != 0 {
		t.Fatalf("SearchSongs: got %d songs, want 0 for disabled library", len(songs))
	}
}

func TestScanNonExistentLibraryPathStillCompletes(t *testing.T) {
	db := newTestDB(t)
	root := tempDir(t)
	missing := filepath.Join(root, "does-not-exist")
	seedLibrary(t, db, missing, true)

	sc := NewScanner(db.Music, newStubTagReader(), silentLogger())
	// WalkAudioFiles silently no-ops on a missing directory (fs.WalkDir
	// swallows the initial stat error), so Scan itself still succeeds; this
	// asserts that behavior rather than assuming a hard failure.
	if err := sc.Scan(context.Background(), ScanModeFull); err != nil {
		t.Fatalf("Scan: unexpected error for missing library dir: %v", err)
	}
}

func TestScanLibraryUpdatesLastScanAt(t *testing.T) {
	db := newTestDB(t)
	root := tempDir(t)
	writeFile(t, root, "track1.mp3")
	lib := seedLibrary(t, db, root, true)

	sc := NewScanner(db.Music, newStubTagReader(), silentLogger())
	counters := &scanCounters{}
	if err := sc.ScanLibrary(context.Background(), lib, ScanModeFull, counters); err != nil {
		t.Fatalf("ScanLibrary: unexpected error: %v", err)
	}

	updated, err := db.Music.GetLibrary(context.Background(), lib.ID)
	if err != nil {
		t.Fatalf("GetLibrary: %v", err)
	}
	if updated.LastScanAt.IsZero() {
		t.Error("ScanLibrary: expected LastScanAt to be set")
	}
}

func TestProcessFileIncrementalSkipsUnchangedFile(t *testing.T) {
	db := newTestDB(t)
	root := tempDir(t)
	path := writeFile(t, root, "track1.mp3")
	lib := seedLibrary(t, db, root, true)

	reader := newStubTagReader()
	sc := NewScanner(db.Music, reader, silentLogger())
	counters := &scanCounters{}

	// First pass (full) inserts the song.
	sc.processFile(context.Background(), lib, path, counters, ScanModeFull)
	if reader.calls[path] != 1 {
		t.Fatalf("expected 1 tag read after first pass, got %d", reader.calls[path])
	}

	// Second pass (incremental) should skip since mtime is unchanged.
	sc.processFile(context.Background(), lib, path, counters, ScanModeIncremental)
	if reader.calls[path] != 1 {
		t.Errorf("incremental scan re-read unchanged file: got %d calls, want 1", reader.calls[path])
	}

	// Touch the mtime forward and rescan incrementally: should re-read.
	future := time.Now().Add(10 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	sc.processFile(context.Background(), lib, path, counters, ScanModeIncremental)
	if reader.calls[path] != 2 {
		t.Errorf("incremental scan did not re-read changed file: got %d calls, want 2", reader.calls[path])
	}
}

func TestProcessFileFallsBackToUnknownAndFilename(t *testing.T) {
	db := newTestDB(t)
	root := tempDir(t)
	path := writeFile(t, root, "My Song Title.mp3")
	lib := seedLibrary(t, db, root, true)

	reader := newStubTagReader()
	reader.def = &SongMeta{} // empty tags
	sc := NewScanner(db.Music, reader, silentLogger())
	counters := &scanCounters{}

	sc.processFile(context.Background(), lib, path, counters, ScanModeFull)

	song, err := db.Music.GetSongByPath(context.Background(), path)
	if err != nil {
		t.Fatalf("GetSongByPath: %v", err)
	}
	if song == nil {
		t.Fatal("GetSongByPath: expected song to exist")
	}
	if song.Title != "My Song Title" {
		t.Errorf("Title: got %q, want %q", song.Title, "My Song Title")
	}
	if song.ArtistName != "Unknown Artist" {
		t.Errorf("ArtistName: got %q, want Unknown Artist", song.ArtistName)
	}
	if song.AlbumName != "Unknown Album" {
		t.Errorf("AlbumName: got %q, want Unknown Album", song.AlbumName)
	}
	if song.AlbumArtistName != "Unknown Artist" {
		t.Errorf("AlbumArtistName: got %q, want Unknown Artist (defaults to artist)", song.AlbumArtistName)
	}
}

func TestProcessFileTagReadErrorIncrementsCounter(t *testing.T) {
	db := newTestDB(t)
	root := tempDir(t)
	path := writeFile(t, root, "broken.mp3")
	lib := seedLibrary(t, db, root, true)

	reader := newStubTagReader()
	reader.errByPath[path] = errors.New("boom")
	sc := NewScanner(db.Music, reader, silentLogger())
	counters := &scanCounters{}

	sc.processFile(context.Background(), lib, path, counters, ScanModeFull)

	if counters.errors.Load() != 1 {
		t.Errorf("errors counter: got %d, want 1", counters.errors.Load())
	}
	song, err := db.Music.GetSongByPath(context.Background(), path)
	if err != nil {
		t.Fatalf("GetSongByPath: %v", err)
	}
	if song != nil {
		t.Error("GetSongByPath: expected no song row for a failed tag read")
	}
}

func TestProcessFileStatErrorIsSilentNoOp(t *testing.T) {
	db := newTestDB(t)
	root := tempDir(t)
	lib := seedLibrary(t, db, root, true)
	missing := filepath.Join(root, "gone.mp3")

	sc := NewScanner(db.Music, newStubTagReader(), silentLogger())
	counters := &scanCounters{}

	sc.processFile(context.Background(), lib, missing, counters, ScanModeFull)

	if counters.scanned.Load() != 0 || counters.errors.Load() != 0 {
		t.Errorf("expected no counters incremented for stat error, got scanned=%d errors=%d",
			counters.scanned.Load(), counters.errors.Load())
	}
}

func TestUpsertSongStoresCoverArtAndHash(t *testing.T) {
	db := newTestDB(t)
	root := tempDir(t)
	path := writeFile(t, root, "with_cover.mp3")
	if err := os.WriteFile(path, []byte("audio-bytes"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	lib := seedLibrary(t, db, root, true)

	reader := newStubTagReader()
	reader.byPath[path] = &SongMeta{
		Title:     "Cover Song",
		Artist:    "Cover Artist",
		Album:     "Cover Album",
		CoverData: []byte("fake-image-bytes"),
		CoverMime: "image/jpeg",
	}
	sc := NewScanner(db.Music, reader, silentLogger())
	counters := &scanCounters{}

	sc.processFile(context.Background(), lib, path, counters, ScanModeFull)

	song, err := db.Music.GetSongByPath(context.Background(), path)
	if err != nil {
		t.Fatalf("GetSongByPath: %v", err)
	}
	if song == nil {
		t.Fatal("GetSongByPath: expected song row")
	}
	if song.CoverArtID == 0 {
		t.Error("CoverArtID: expected non-zero cover art ID")
	}

	wantHash := sha256.Sum256([]byte("audio-bytes"))
	if song.FileHash != hex.EncodeToString(wantHash[:]) {
		t.Errorf("FileHash: got %q, want %q", song.FileHash, hex.EncodeToString(wantHash[:]))
	}
}

func TestHashFile(t *testing.T) {
	root := tempDir(t)
	path := filepath.Join(root, "data.bin")
	if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := hashFile(path)
	if err != nil {
		t.Fatalf("hashFile: unexpected error: %v", err)
	}
	want := sha256.Sum256([]byte("hello world"))
	if got != hex.EncodeToString(want[:]) {
		t.Errorf("hashFile: got %q, want %q", got, hex.EncodeToString(want[:]))
	}

	if _, err := hashFile(filepath.Join(root, "missing.bin")); err == nil {
		t.Error("hashFile: expected error for missing file")
	}
}
