package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/gif"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// ErrNoCoverArt is returned when no cover art can be found for a song or album.
var ErrNoCoverArt = errors.New("cover_art: no cover art found")

// coverArtFilenames lists the candidate filenames searched in a song's directory,
// in priority order. Comparisons are case-insensitive.
var coverArtFilenames = []string{
	"cover.jpg", "cover.png",
	"folder.jpg", "folder.png",
	"album.jpg", "album.png",
	"front.jpg", "front.png",
}

// validThumbnailSizes contains the allowed thumbnail dimensions in pixels.
var validThumbnailSizes = []int{64, 300}

// CoverArtService resolves, caches, and serves cover art for songs and albums.
type CoverArtService struct {
	music    store.MusicStore
	thumbDir string
}

// NewCoverArtService creates a CoverArtService.
// thumbDir is the directory where generated thumbnails are cached on disk.
func NewCoverArtService(music store.MusicStore, thumbDir string) *CoverArtService {
	return &CoverArtService{music: music, thumbDir: thumbDir}
}

// GetForSong returns the cover art bytes and MIME type for the given song.
// Resolution priority:
//  1. Embedded art stored in the cover_art table (cover_art_id > 0)
//  2. cover.jpg / cover.png / folder.jpg / … found in the song's directory
//
// Returns ErrNoCoverArt when nothing is found.
func (s *CoverArtService) GetForSong(ctx context.Context, songID int64) ([]byte, string, error) {
	song, err := s.music.GetSong(ctx, songID)
	if err != nil {
		return nil, "", fmt.Errorf("cover_art: get song %d: %w", songID, err)
	}
	if song == nil {
		return nil, "", ErrNoCoverArt
	}

	if song.CoverArtID > 0 {
		data, mime, err := s.fetchFromDB(ctx, song.CoverArtID)
		if err == nil {
			return data, mime, nil
		}
	}

	return findCoverInDir(filepath.Dir(song.Path))
}

// GetForAlbum returns the cover art bytes and MIME type for the given album.
// Resolution priority:
//  1. Embedded art stored in the cover_art table (cover_art_id > 0)
//  2. Delegated to the first song in the album via GetForSong
//
// Returns ErrNoCoverArt when nothing is found.
func (s *CoverArtService) GetForAlbum(ctx context.Context, albumID int64) ([]byte, string, error) {
	album, err := s.music.GetAlbum(ctx, albumID)
	if err != nil {
		return nil, "", fmt.Errorf("cover_art: get album %d: %w", albumID, err)
	}
	if album == nil {
		return nil, "", ErrNoCoverArt
	}

	if album.CoverArtID > 0 {
		data, mime, err := s.fetchFromDB(ctx, album.CoverArtID)
		if err == nil {
			return data, mime, nil
		}
	}

	songs, err := s.music.ListSongsByAlbum(ctx, albumID)
	if err != nil {
		return nil, "", fmt.Errorf("cover_art: list songs for album %d: %w", albumID, err)
	}
	if len(songs) == 0 {
		return nil, "", ErrNoCoverArt
	}

	return s.GetForSong(ctx, songs[0].ID)
}

// GetThumbnail returns a JPEG thumbnail of the specified cover art at the requested size.
// size is snapped to the nearest valid size (64 or 300).
// Thumbnails are cached on disk at {thumbDir}/{coverArtID}_{size}.jpg.
func (s *CoverArtService) GetThumbnail(ctx context.Context, coverArtID int64, size int) ([]byte, string, error) {
	size = snapSize(size)
	cachePath := filepath.Join(s.thumbDir, fmt.Sprintf("%d_%d.jpg", coverArtID, size))

	if data, err := os.ReadFile(cachePath); err == nil {
		return data, "image/jpeg", nil
	}

	origData, _, err := s.fetchFromDB(ctx, coverArtID)
	if err != nil {
		return nil, "", fmt.Errorf("cover_art thumbnail: fetch original: %w", err)
	}

	img, err := decodeImage(origData)
	if err != nil {
		return nil, "", fmt.Errorf("cover_art thumbnail: decode image: %w", err)
	}

	img = resize(img, size)

	thumbBytes, err := encodeJPEG(img)
	if err != nil {
		return nil, "", fmt.Errorf("cover_art thumbnail: encode jpeg: %w", err)
	}

	if err := writeCacheFile(cachePath, thumbBytes); err != nil {
		return nil, "", fmt.Errorf("cover_art thumbnail: write cache: %w", err)
	}

	return thumbBytes, "image/jpeg", nil
}

// SaveFromBytes stores raw image bytes in the cover_art table and returns the new ID.
// songID and albumID may be 0 if not applicable.
func (s *CoverArtService) SaveFromBytes(ctx context.Context, data []byte, mime string, songID, albumID int64) (int64, error) {
	img, err := decodeImage(data)
	if err != nil {
		return 0, fmt.Errorf("cover_art save: decode image: %w", err)
	}
	bounds := img.Bounds()

	ca := &model.CoverArt{
		SongID:   songID,
		AlbumID:  albumID,
		Data:     data,
		MimeType: mime,
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
	}

	id, err := s.music.UpsertCoverArt(ctx, ca)
	if err != nil {
		return 0, fmt.Errorf("cover_art save: upsert: %w", err)
	}
	return id, nil
}

// fetchFromDB retrieves raw cover art bytes and MIME type from the database.
func (s *CoverArtService) fetchFromDB(ctx context.Context, id int64) ([]byte, string, error) {
	ca, err := s.music.GetCoverArt(ctx, id)
	if err != nil {
		return nil, "", fmt.Errorf("cover_art: get cover_art %d: %w", id, err)
	}
	if ca == nil {
		return nil, "", ErrNoCoverArt
	}

	if len(ca.Data) > 0 {
		return ca.Data, ca.MimeType, nil
	}

	if ca.Path != "" {
		data, err := os.ReadFile(ca.Path)
		if err != nil {
			return nil, "", fmt.Errorf("cover_art: read file %s: %w", ca.Path, err)
		}
		return data, ca.MimeType, nil
	}

	return nil, "", ErrNoCoverArt
}

// findCoverInDir searches dir for a cover art image file and returns its bytes and MIME type.
func findCoverInDir(dir string) ([]byte, string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, "", ErrNoCoverArt
	}

	existing := make(map[string]string, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			existing[strings.ToLower(e.Name())] = e.Name()
		}
	}

	for _, candidate := range coverArtFilenames {
		realName, ok := existing[candidate]
		if !ok {
			continue
		}
		path := filepath.Join(dir, realName)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if _, err := decodeImage(data); err != nil {
			continue
		}
		mime := mimeFromFilename(realName)
		return data, mime, nil
	}

	return nil, "", ErrNoCoverArt
}

// mimeFromFilename returns an image MIME type based on the file extension.
func mimeFromFilename(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	default:
		return "image/jpeg"
	}
}

// snapSize rounds size up to the nearest valid thumbnail size.
// Sizes greater than 300 are capped at 300.
func snapSize(size int) int {
	for _, v := range validThumbnailSizes {
		if size <= v {
			return v
		}
	}
	return validThumbnailSizes[len(validThumbnailSizes)-1]
}

// writeCacheFile writes data to path atomically by writing a .tmp file and renaming.
func writeCacheFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// resize scales img to fit within maxSize×maxSize while preserving the aspect ratio.
// Uses a BiLinear filter for high-quality downscaling.
func resize(img image.Image, maxSize int) image.Image {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if w <= maxSize && h <= maxSize {
		return img
	}

	var newW, newH int
	if w > h {
		newW = maxSize
		newH = h * maxSize / w
		if newH < 1 {
			newH = 1
		}
	} else {
		newH = maxSize
		newW = w * maxSize / h
		if newW < 1 {
			newW = 1
		}
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
	return dst
}

// decodeImage decodes raw bytes as JPEG, PNG, or GIF.
func decodeImage(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return img, nil
}

// encodeJPEG encodes img as a JPEG at quality 90 and returns the raw bytes.
func encodeJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), nil
}
