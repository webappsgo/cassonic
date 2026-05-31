package scheduler

import (
	"context"
	"time"

	"github.com/local/cassonic/src/server/service"
	"github.com/local/cassonic/src/server/service/musicbrainz"
	"github.com/local/cassonic/src/server/service/podcast"
	"github.com/local/cassonic/src/server/service/scrobble"
	"github.com/local/cassonic/src/server/store"
)

// mbidBatchSize is the maximum number of songs enriched per MusicBrainz job run.
const mbidBatchSize = 100

// mbidCandidatePool is how many songs are fetched to find those missing MBIDs.
// Larger than mbidBatchSize to compensate for songs that already have all IDs.
const mbidCandidatePool = 500

// nextOccurrence returns the next wall-clock time at which the given hour (local time)
// will occur. If that time is already in the past for today, the next-day occurrence
// is returned.
func nextOccurrence(hour int) time.Time {
	now := time.Now()
	candidate := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if candidate.Before(now) {
		candidate = candidate.Add(24 * time.Hour)
	}
	return candidate
}

// LibraryScanJob returns a Job that runs an incremental library scan every 24 hours.
func LibraryScanJob(scanner *service.Scanner) Job {
	return Job{
		Name:     "library_scan",
		Interval: 24 * time.Hour,
		Fn: func(ctx context.Context) error {
			return scanner.Scan(ctx, service.ScanModeIncremental)
		},
	}
}

// PodcastRefreshJob returns a Job that refreshes all podcast RSS feeds every 4 hours.
func PodcastRefreshJob(pod *podcast.Service) Job {
	return Job{
		Name:     "podcast_refresh",
		Interval: 4 * time.Hour,
		Fn: func(ctx context.Context) error {
			return pod.RefreshAll(ctx)
		},
	}
}

// ScrobbleRetryJob returns a Job that drains the scrobble retry queue every 30 minutes.
func ScrobbleRetryJob(scr *scrobble.Service) Job {
	return Job{
		Name:     "scrobble_retry",
		Interval: 30 * time.Minute,
		Fn: func(ctx context.Context) error {
			return scr.DrainQueue(ctx)
		},
	}
}

// MusicBrainzLookupJob returns a Job that fills empty MBID fields on songs every 24 hours,
// scheduled to begin at the next 02:00 local time to avoid peak usage periods.
// At most mbidBatchSize songs are processed per run to respect the 1 req/s rate limit.
func MusicBrainzLookupJob(mb *musicbrainz.Client, music store.MusicStore) Job {
	return Job{
		Name:     "musicbrainz_lookup",
		Interval: 24 * time.Hour,
		RunAt:    nextOccurrence(2),
		Fn: func(ctx context.Context) error {
			songs, err := music.SearchSongs(ctx, "", store.ListOpts{Limit: mbidCandidatePool})
			if err != nil {
				return err
			}

			processed := 0
			for _, song := range songs {
				if processed >= mbidBatchSize {
					break
				}
				if song.MBTrackID != "" && song.MBArtistID != "" &&
					song.MBAlbumID != "" && song.MBAlbumArtistID != "" {
					continue
				}

				changed, err := mb.FillSongMBIDs(ctx, song)
				if err != nil {
					return err
				}
				if changed {
					if _, upsertErr := music.UpsertSong(ctx, song); upsertErr != nil {
						return upsertErr
					}
				}
				processed++
			}
			return nil
		},
	}
}

// CoverArtRefreshJob returns a Job that refreshes cached cover art thumbnails every 48 hours.
func CoverArtRefreshJob(coverArt *service.CoverArtService) Job {
	return Job{
		Name:     "cover_art_refresh",
		Interval: 48 * time.Hour,
		Fn: func(ctx context.Context) error {
			return coverArt.RefreshAll(ctx)
		},
	}
}
