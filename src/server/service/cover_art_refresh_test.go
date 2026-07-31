package service

// Tests cover:
//   - RefreshAll: warms thumbnails for albums that have cover art, skips
//     albums with CoverArtID == 0, and does not error when GetThumbnail
//     fails for one album (e.g. a stale/deleted cover_art row)
//   - RefreshAll: no albums is a no-op that returns nil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

func TestRefreshAllWarmsThumbnailsForAlbumsWithCoverArt(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	thumbDir := tempDir(t)
	svc := NewCoverArtService(db.Music, thumbDir)

	artistID := seedArtist(t, db, "Artist")

	caID, err := db.Music.UpsertCoverArt(ctx, &model.CoverArt{Data: makeJPEG(t, 32, 32), MimeType: "image/jpeg"})
	if err != nil {
		t.Fatalf("UpsertCoverArt: %v", err)
	}
	if _, err := db.Music.UpsertAlbum(ctx, &model.Album{Title: "With Cover", ArtistID: artistID, CoverArtID: caID}); err != nil {
		t.Fatalf("UpsertAlbum: %v", err)
	}
	// An album with no cover art should be skipped entirely (no lookup error).
	if _, err := db.Music.UpsertAlbum(ctx, &model.Album{Title: "No Cover", ArtistID: artistID}); err != nil {
		t.Fatalf("UpsertAlbum: %v", err)
	}

	if err := svc.RefreshAll(ctx); err != nil {
		t.Fatalf("RefreshAll: unexpected error: %v", err)
	}

	for _, size := range validThumbnailSizes {
		cachePath := filepath.Join(thumbDir, fmt.Sprintf("%d_%d.jpg", caID, size))
		if _, err := os.Stat(cachePath); err != nil {
			t.Errorf("expected thumbnail cache file for size %d at %s: %v", size, cachePath, err)
		}
	}
}

func TestRefreshAllNoAlbumsIsNoOp(t *testing.T) {
	db := newTestDB(t)
	svc := NewCoverArtService(db.Music, tempDir(t))

	if err := svc.RefreshAll(context.Background()); err != nil {
		t.Fatalf("RefreshAll: unexpected error for empty library: %v", err)
	}
}

func TestRefreshAllContinuesPastThumbnailError(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	svc := NewCoverArtService(db.Music, tempDir(t))

	artistID := seedArtist(t, db, "Artist")

	// CoverArtID points at a row that doesn't exist, so GetThumbnail will
	// fail for every size; RefreshAll must swallow that and still return nil.
	if _, err := db.Music.UpsertAlbum(ctx, &model.Album{Title: "Stale Cover", ArtistID: artistID, CoverArtID: 999999}); err != nil {
		t.Fatalf("UpsertAlbum: %v", err)
	}

	if err := svc.RefreshAll(ctx); err != nil {
		t.Fatalf("RefreshAll: unexpected error: %v", err)
	}
}
