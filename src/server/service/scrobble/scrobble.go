package scrobble

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// Service handles scrobbling to multiple backends concurrently.
type Service struct {
	db     *store.DB
	client *http.Client
	logger *log.Logger
}

// NewService creates a new scrobbling service with a 30-second HTTP timeout.
func NewService(db *store.DB, logger *log.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Scrobble sends a play event to all enabled and verified services for the given user.
// All configured services are called concurrently; failures are independent and queued for retry.
func (s *Service) Scrobble(ctx context.Context, userID int64, track model.ScrobbleTrackData) error {
	svcs, err := s.db.Scrobble.ListServices(ctx, userID)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, svc := range svcs {
		if !svc.Enabled || !svc.Verified {
			continue
		}
		wg.Add(1)
		go func(svc *model.ScrobbleService) {
			defer wg.Done()
			var submitErr error
			switch svc.ServiceType.Protocol() {
			case "listenbrainz":
				submitErr = s.scrobbleListenBrainz(ctx, svc, track)
			default:
				submitErr = s.scrobbleLastFM(ctx, svc, track)
			}
			if submitErr != nil {
				s.logger.Printf("scrobble: service %d (%s) failed: %v; queuing for retry", svc.ID, svc.ServiceType, submitErr)
				if qErr := s.Queue(ctx, userID, svc.ID, track); qErr != nil {
					s.logger.Printf("scrobble: failed to queue retry for service %d: %v", svc.ID, qErr)
				}
			}
		}(svc)
	}
	wg.Wait()
	return nil
}

// Queue adds a scrobble to the retry queue for later delivery.
func (s *Service) Queue(ctx context.Context, userID int64, serviceID int64, track model.ScrobbleTrackData) error {
	entry := &model.ScrobbleQueueEntry{
		UserID:    userID,
		ServiceID: serviceID,
		TrackData: track,
	}
	return s.db.Scrobble.EnqueueScrobble(ctx, entry)
}

// DrainQueue processes all pending scrobbles grouped by service.
// Called by the scheduler every 30 minutes.
// Last.fm protocol services are batched up to 50; ListenBrainz up to 1000.
func (s *Service) DrainQueue(ctx context.Context) error {
	svcs, err := s.db.Scrobble.ListAllEnabledServices(ctx)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-14 * 24 * time.Hour)

	for _, svc := range svcs {
		if !svc.Enabled || !svc.Verified {
			continue
		}

		batchSize := 50
		if svc.ServiceType.Protocol() == "listenbrainz" {
			batchSize = 1000
		}

		entries, err := s.db.Scrobble.ListPendingByService(ctx, svc.ID, batchSize)
		if err != nil {
			s.logger.Printf("scrobble: drain queue list for service %d: %v", svc.ID, err)
			continue
		}

		for _, entry := range entries {
			var submitErr error
			switch svc.ServiceType.Protocol() {
			case "listenbrainz":
				submitErr = s.scrobbleListenBrainz(ctx, svc, entry.TrackData)
			default:
				submitErr = s.scrobbleLastFM(ctx, svc, entry.TrackData)
			}

			if submitErr == nil {
				if delErr := s.db.Scrobble.DeleteQueueEntry(ctx, entry.ID); delErr != nil {
					s.logger.Printf("scrobble: drain delete queue entry %d: %v", entry.ID, delErr)
				}
			} else {
				s.logger.Printf("scrobble: drain service %d entry %d failed: %v", svc.ID, entry.ID, submitErr)
				if incErr := s.db.Scrobble.IncrementAttempts(ctx, entry.ID, submitErr.Error()); incErr != nil {
					s.logger.Printf("scrobble: drain increment attempts %d: %v", entry.ID, incErr)
				}
			}
		}
	}

	if err := s.db.Scrobble.PurgeStaleQueue(ctx, cutoff, 50); err != nil {
		s.logger.Printf("scrobble: purge stale queue: %v", err)
	}

	return nil
}

// Verify checks whether a scrobble service configuration is valid and working.
// Updates the service record with verified=true/false and last_error.
func (s *Service) Verify(ctx context.Context, svc *model.ScrobbleService) error {
	var err error
	switch svc.ServiceType.Protocol() {
	case "listenbrainz":
		err = s.verifyListenBrainz(ctx, svc)
	default:
		err = s.verifyLastFM(ctx, svc)
	}

	if err != nil {
		if setErr := s.db.Scrobble.SetServiceVerified(ctx, svc.ID, false, err.Error()); setErr != nil {
			s.logger.Printf("scrobble: verify set unverified for service %d: %v", svc.ID, setErr)
		}
		return err
	}

	if setErr := s.db.Scrobble.SetServiceVerified(ctx, svc.ID, true, ""); setErr != nil {
		s.logger.Printf("scrobble: verify set verified for service %d: %v", svc.ID, setErr)
	}
	return nil
}
