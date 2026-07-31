package service

// Tests cover:
//   - NewCoverArtService: returns a usable service
//   - GetForSong: embedded DB art takes priority; falls back to a directory
//     file when no embedded art exists; ErrNoCoverArt when neither exists
//   - GetForAlbum: embedded DB art takes priority; delegates to the first
//     song's cover when no embedded art exists; ErrNoCoverArt for an empty
//     album or unknown album ID
//   - GetThumbnail: generates and caches a resized JPEG, then serves from
//     the on-disk cache on a second call; size snapping
//   - SaveFromBytes: decodes and stores raw image bytes, recording width and
//     height
//   - findCoverInDir: case-insensitive filename matching, priority order,
//     skips undecodable files, ErrNoCoverArt when directory has no match
//   - mimeFromFilename: known and unknown extensions
//   - snapSize: rounds up to the nearest valid size, caps at the largest
//   - writeCacheFile: creates parent directories and writes atomically
//   - resize: no-op when already within bounds, scales down preserving
//     aspect ratio
//   - decodeImage / encodeJPEG: round-trip a generated image

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// makeJPEG returns encoded JPEG bytes for a solid-color w×h image.
func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 50, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("makeJPEG: encode: %v", err)
	}
	return buf.Bytes()
}

// makePNG returns encoded PNG bytes for a solid-color w×h image.
func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 200, B: 10, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("makePNG: encode: %v", err)
	}
	return buf.Bytes()
}

func TestNewCoverArtServiceReturnsUsable(t *testing.T) {
	db := newTestDB(t)
	svc := NewCoverArtService(db.Music, tempDir(t))
	if svc == nil {
		t.Fatal("NewCoverArtService: returned nil")
	}
}

func TestGetForSongEmbeddedTakesPriority(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	svc := NewCoverArtService(db.Music, tempDir(t))

	songDir := tempDir(t)
	path := writeFile(t, songDir, "track.mp3")
	// Also drop a directory-level cover that should be ignored in favor of
	// the embedded art.
	if err := os.WriteFile(filepath.Join(songDir, "cover.jpg"), makeJPEG(t, 4, 4), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	libID := seedMinimalLibrary(t, db)

	caID, err := db.Music.UpsertCoverArt(ctx, &model.CoverArt{Data: makeJPEG(t, 8, 8), MimeType: "image/jpeg"})
	if err != nil {
		t.Fatalf("UpsertCoverArt: %v", err)
	}

	songID, err := db.Music.UpsertSong(ctx, &model.Song{LibraryID: libID, Path: path, Title: "t", CoverArtID: caID})
	if err != nil {
		t.Fatalf("UpsertSong: %v", err)
	}

	data, mime, err := svc.GetForSong(ctx, songID)
	if err != nil {
		t.Fatalf("GetForSong: unexpected error: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("mime: got %q, want image/jpeg", mime)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data")
	}
}

func TestGetForSongFallsBackToDirectoryFile(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	svc := NewCoverArtService(db.Music, tempDir(t))

	songDir := tempDir(t)
	path := writeFile(t, songDir, "track.mp3")
	if err := os.WriteFile(filepath.Join(songDir, "folder.png"), makePNG(t, 4, 4), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	libID := seedMinimalLibrary(t, db)

	songID, err := db.Music.UpsertSong(ctx, &model.Song{LibraryID: libID, Path: path, Title: "t"})
	if err != nil {
		t.Fatalf("UpsertSong: %v", err)
	}

	data, mime, err := svc.GetForSong(ctx, songID)
	if err != nil {
		t.Fatalf("GetForSong: unexpected error: %v", err)
	}
	if mime != "image/png" {
		t.Errorf("mime: got %q, want image/png", mime)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data")
	}
}

func TestGetForSongNoCoverReturnsErrNoCoverArt(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	svc := NewCoverArtService(db.Music, tempDir(t))

	libID := seedMinimalLibrary(t, db)

	songDir := tempDir(t)
	path := writeFile(t, songDir, "track.mp3")
	songID, err := db.Music.UpsertSong(ctx, &model.Song{LibraryID: libID, Path: path, Title: "t"})
	if err != nil {
		t.Fatalf("UpsertSong: %v", err)
	}

	_, _, err = svc.GetForSong(ctx, songID)
	if !errors.Is(err, ErrNoCoverArt) {
		t.Errorf("GetForSong: got err %v, want ErrNoCoverArt", err)
	}
}

func TestGetForSongUnknownIDReturnsErrNoCoverArt(t *testing.T) {
	db := newTestDB(t)
	svc := NewCoverArtService(db.Music, tempDir(t))

	_, _, err := svc.GetForSong(context.Background(), 999999)
	if !errors.Is(err, ErrNoCoverArt) {
		t.Errorf("GetForSong: got err %v, want ErrNoCoverArt", err)
	}
}

func TestGetForAlbumEmbeddedTakesPriority(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	svc := NewCoverArtService(db.Music, tempDir(t))

	artistID := seedArtist(t, db, "Artist")

	caID, err := db.Music.UpsertCoverArt(ctx, &model.CoverArt{Data: makeJPEG(t, 8, 8), MimeType: "image/jpeg"})
	if err != nil {
		t.Fatalf("UpsertCoverArt: %v", err)
	}
	albumID, err := db.Music.UpsertAlbum(ctx, &model.Album{Title: "Album", ArtistID: artistID, CoverArtID: caID})
	if err != nil {
		t.Fatalf("UpsertAlbum: %v", err)
	}

	data, mime, err := svc.GetForAlbum(ctx, albumID)
	if err != nil {
		t.Fatalf("GetForAlbum: unexpected error: %v", err)
	}
	if mime != "image/jpeg" || len(data) == 0 {
		t.Errorf("GetForAlbum: got mime=%q len=%d", mime, len(data))
	}
}

func TestGetForAlbumDelegatesToFirstSong(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	svc := NewCoverArtService(db.Music, tempDir(t))

	artistID := seedArtist(t, db, "Artist")
	libID := seedMinimalLibrary(t, db)

	albumID, err := db.Music.UpsertAlbum(ctx, &model.Album{Title: "Album", ArtistID: artistID})
	if err != nil {
		t.Fatalf("UpsertAlbum: %v", err)
	}

	songDir := tempDir(t)
	path := writeFile(t, songDir, "track.mp3")
	if err := os.WriteFile(filepath.Join(songDir, "album.jpg"), makeJPEG(t, 4, 4), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := db.Music.UpsertSong(ctx, &model.Song{LibraryID: libID, Path: path, Title: "t", AlbumID: albumID}); err != nil {
		t.Fatalf("UpsertSong: %v", err)
	}

	data, mime, err := svc.GetForAlbum(ctx, albumID)
	if err != nil {
		t.Fatalf("GetForAlbum: unexpected error: %v", err)
	}
	if mime != "image/jpeg" || len(data) == 0 {
		t.Errorf("GetForAlbum: got mime=%q len=%d", mime, len(data))
	}
}

func TestGetForAlbumEmptyAlbumReturnsErrNoCoverArt(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	svc := NewCoverArtService(db.Music, tempDir(t))

	artistID := seedArtist(t, db, "Artist")

	albumID, err := db.Music.UpsertAlbum(ctx, &model.Album{Title: "Empty Album", ArtistID: artistID})
	if err != nil {
		t.Fatalf("UpsertAlbum: %v", err)
	}

	_, _, err = svc.GetForAlbum(ctx, albumID)
	if !errors.Is(err, ErrNoCoverArt) {
		t.Errorf("GetForAlbum: got err %v, want ErrNoCoverArt", err)
	}
}

func TestGetForAlbumUnknownIDReturnsErrNoCoverArt(t *testing.T) {
	db := newTestDB(t)
	svc := NewCoverArtService(db.Music, tempDir(t))

	_, _, err := svc.GetForAlbum(context.Background(), 999999)
	if !errors.Is(err, ErrNoCoverArt) {
		t.Errorf("GetForAlbum: got err %v, want ErrNoCoverArt", err)
	}
}

func TestGetThumbnailGeneratesAndCaches(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	thumbDir := tempDir(t)
	svc := NewCoverArtService(db.Music, thumbDir)

	caID, err := db.Music.UpsertCoverArt(ctx, &model.CoverArt{Data: makeJPEG(t, 500, 500), MimeType: "image/jpeg"})
	if err != nil {
		t.Fatalf("UpsertCoverArt: %v", err)
	}

	data, mime, err := svc.GetThumbnail(ctx, caID, 64)
	if err != nil {
		t.Fatalf("GetThumbnail: unexpected error: %v", err)
	}
	if mime != "image/jpeg" || len(data) == 0 {
		t.Fatalf("GetThumbnail: got mime=%q len=%d", mime, len(data))
	}

	cachePath := filepath.Join(thumbDir, "1_64.jpg")
	if _, err := os.Stat(cachePath); err != nil {
		t.Errorf("expected cache file at %s: %v", cachePath, err)
	}

	// Second call should serve straight from the cache file.
	data2, mime2, err := svc.GetThumbnail(ctx, caID, 64)
	if err != nil {
		t.Fatalf("GetThumbnail (cached): unexpected error: %v", err)
	}
	if mime2 != "image/jpeg" || !bytes.Equal(data, data2) {
		t.Errorf("GetThumbnail (cached): result differs from first call")
	}
}

func TestGetThumbnailUnknownCoverArtIDErrors(t *testing.T) {
	db := newTestDB(t)
	svc := NewCoverArtService(db.Music, tempDir(t))

	_, _, err := svc.GetThumbnail(context.Background(), 999999, 64)
	if err == nil {
		t.Fatal("GetThumbnail: expected error for unknown cover art ID")
	}
}

func TestSaveFromBytesStoresDimensions(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	svc := NewCoverArtService(db.Music, tempDir(t))

	id, err := svc.SaveFromBytes(ctx, makeJPEG(t, 12, 20), "image/jpeg", 0, 0)
	if err != nil {
		t.Fatalf("SaveFromBytes: unexpected error: %v", err)
	}
	if id == 0 {
		t.Fatal("SaveFromBytes: expected non-zero ID")
	}

	ca, err := db.Music.GetCoverArt(ctx, id)
	if err != nil {
		t.Fatalf("GetCoverArt: %v", err)
	}
	if ca.Width != 12 || ca.Height != 20 {
		t.Errorf("dimensions: got %dx%d, want 12x20", ca.Width, ca.Height)
	}
}

func TestSaveFromBytesInvalidDataErrors(t *testing.T) {
	db := newTestDB(t)
	svc := NewCoverArtService(db.Music, tempDir(t))

	_, err := svc.SaveFromBytes(context.Background(), []byte("not an image"), "image/jpeg", 0, 0)
	if err == nil {
		t.Fatal("SaveFromBytes: expected error for invalid image data")
	}
}

func TestFindCoverInDir(t *testing.T) {
	t.Run("case insensitive priority match", func(t *testing.T) {
		dir := tempDir(t)
		if err := os.WriteFile(filepath.Join(dir, "FOLDER.JPG"), makeJPEG(t, 4, 4), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "front.png"), makePNG(t, 4, 4), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		data, mime, err := findCoverInDir(dir)
		if err != nil {
			t.Fatalf("findCoverInDir: unexpected error: %v", err)
		}
		if mime != "image/jpeg" || len(data) == 0 {
			// folder.jpg ranks before front.png in coverArtFilenames.
			t.Errorf("findCoverInDir: got mime=%q, want image/jpeg (folder.jpg priority)", mime)
		}
	})

	t.Run("skips undecodable file and falls through", func(t *testing.T) {
		dir := tempDir(t)
		if err := os.WriteFile(filepath.Join(dir, "cover.jpg"), []byte("not an image"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "folder.png"), makePNG(t, 4, 4), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		_, mime, err := findCoverInDir(dir)
		if err != nil {
			t.Fatalf("findCoverInDir: unexpected error: %v", err)
		}
		if mime != "image/png" {
			t.Errorf("findCoverInDir: got mime=%q, want image/png (fell through past undecodable cover.jpg)", mime)
		}
	})

	t.Run("no match returns ErrNoCoverArt", func(t *testing.T) {
		dir := tempDir(t)
		if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		_, _, err := findCoverInDir(dir)
		if !errors.Is(err, ErrNoCoverArt) {
			t.Errorf("findCoverInDir: got err %v, want ErrNoCoverArt", err)
		}
	})

	t.Run("non-existent directory returns ErrNoCoverArt", func(t *testing.T) {
		_, _, err := findCoverInDir(filepath.Join(tempDir(t), "missing"))
		if !errors.Is(err, ErrNoCoverArt) {
			t.Errorf("findCoverInDir: got err %v, want ErrNoCoverArt", err)
		}
	})
}

func TestMimeFromFilename(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"cover.jpg", "image/jpeg"},
		{"cover.JPEG", "image/jpeg"},
		{"cover.png", "image/png"},
		{"cover.gif", "image/gif"},
		{"cover.bmp", "image/jpeg"},
		{"cover", "image/jpeg"},
	}
	for _, tt := range tests {
		if got := mimeFromFilename(tt.name); got != tt.want {
			t.Errorf("mimeFromFilename(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestSnapSize(t *testing.T) {
	tests := []struct {
		in   int
		want int
	}{
		{0, 64},
		{1, 64},
		{64, 64},
		{65, 300},
		{300, 300},
		{1000, 300},
	}
	for _, tt := range tests {
		if got := snapSize(tt.in); got != tt.want {
			t.Errorf("snapSize(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestWriteCacheFileCreatesParentDirs(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "nested", "sub", "file.jpg")

	if err := writeCacheFile(path, []byte("data")); err != nil {
		t.Fatalf("writeCacheFile: unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "data" {
		t.Errorf("writeCacheFile: got %q, want %q", got, "data")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("writeCacheFile: expected .tmp file to be renamed away")
	}
}

func TestResize(t *testing.T) {
	small := image.NewRGBA(image.Rect(0, 0, 10, 10))
	if got := resize(small, 64); got.Bounds().Dx() != 10 || got.Bounds().Dy() != 10 {
		t.Errorf("resize: expected no-op for image within bounds, got %dx%d", got.Bounds().Dx(), got.Bounds().Dy())
	}

	wide := image.NewRGBA(image.Rect(0, 0, 400, 200))
	got := resize(wide, 100)
	if got.Bounds().Dx() != 100 {
		t.Errorf("resize: width got %d, want 100", got.Bounds().Dx())
	}
	if got.Bounds().Dy() != 50 {
		t.Errorf("resize: height got %d, want 50 (aspect preserved)", got.Bounds().Dy())
	}

	tall := image.NewRGBA(image.Rect(0, 0, 100, 400))
	got = resize(tall, 100)
	if got.Bounds().Dy() != 100 || got.Bounds().Dx() != 25 {
		t.Errorf("resize: got %dx%d, want 25x100", got.Bounds().Dx(), got.Bounds().Dy())
	}
}

func TestDecodeAndEncodeImageRoundTrip(t *testing.T) {
	original := makeJPEG(t, 16, 16)

	img, err := decodeImage(original)
	if err != nil {
		t.Fatalf("decodeImage: unexpected error: %v", err)
	}

	out, err := encodeJPEG(img)
	if err != nil {
		t.Fatalf("encodeJPEG: unexpected error: %v", err)
	}
	if len(out) == 0 {
		t.Error("encodeJPEG: expected non-empty output")
	}

	if _, err := decodeImage([]byte("garbage")); err == nil {
		t.Error("decodeImage: expected error for garbage input")
	}
}
