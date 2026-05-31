package service

import (
	"context"

	"github.com/local/cassonic/src/server/store"
)

// RefreshAll iterates over all albums in the music store and warms the thumbnail
// cache for each at the standard sizes. Existing cache files are regenerated when
// the underlying cover art has changed because GetThumbnail writes through the cache.
func (s *CoverArtService) RefreshAll(ctx context.Context) error {
	albums, err := s.music.ListAlbums(ctx, store.ListOpts{Limit: 0})
	if err != nil {
		return err
	}

	thumbnailSizes := validThumbnailSizes

	for _, album := range albums {
		if album.CoverArtID == 0 {
			continue
		}

		for _, size := range thumbnailSizes {
			if _, _, err := s.GetThumbnail(ctx, album.CoverArtID, size); err != nil {
				continue
			}
		}
	}

	return nil
}
