package scrobble

// Tests cover:
//   - Service.Scrobble: fan-out skips disabled/unverified services, routes by
//     protocol (lastfm vs listenbrainz) to the right backend via a live
//     httptest server, queues a retry entry on submit failure, and does NOT
//     queue on success; also an empty-service-list no-op and a
//     ListServices-error passthrough
//   - Service.Queue: builds a ScrobbleQueueEntry from the given user/service/
//     track and forwards it to EnqueueScrobble, propagating its error
//   - Service.DrainQueue: batches by protocol (50 for lastfm, 1000 for
//     listenbrainz), skips disabled/unverified services, deletes the queue
//     entry on successful submit, increments attempts on failure, continues
//     past a ListPendingByService error for one service, and always calls
//     PurgeStaleQueue with a ~14-day cutoff and maxAttempts=50 even when
//     ListAllEnabledServices returns no services or a submit fails
//   - Service.Verify: routes by protocol, calls SetServiceVerified(true,"")
//     on success and SetServiceVerified(false, err.Error()) on failure
//
// Not covered: the concrete sqliteScrobbleStore (has its own test file) and
// NewService's http.Client timeout value (no observable behavior to assert
// without a real slow server).

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

func testLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

// stubScrobbleStore is a configurable implementation of store.ScrobbleStore.
// All fields default to zero values (no error, empty/nil result) so a test
// under this package only needs to set the fields it actually reads.
type stubScrobbleStore struct {
	mu sync.Mutex

	services    []*model.ScrobbleService
	servicesErr error

	allEnabled    []*model.ScrobbleService
	allEnabledErr error

	createID  int64
	createErr error
	getSvc    *model.ScrobbleService
	getErr    error
	updateErr error
	deleteErr error

	setVerifiedErr   error
	setVerifiedCalls []setVerifiedCall

	enqueueErr error
	enqueued   []*model.ScrobbleQueueEntry

	pendingByService map[int64][]*model.ScrobbleQueueEntry
	pendingErr       map[int64]error
	pendingLimits    map[int64]int

	deleteQueueErr  error
	deletedQueueIDs []int64

	incrementErr   error
	incrementCalls []incrementCall

	purgeErr         error
	purgeCalled      bool
	purgeBefore      time.Time
	purgeMaxAttempts int
}

type setVerifiedCall struct {
	id       int64
	verified bool
	lastErr  string
}

type incrementCall struct {
	id      int64
	lastErr string
}

func (s *stubScrobbleStore) CreateService(ctx context.Context, svc *model.ScrobbleService) (int64, error) {
	return s.createID, s.createErr
}

func (s *stubScrobbleStore) GetService(ctx context.Context, id int64) (*model.ScrobbleService, error) {
	return s.getSvc, s.getErr
}

func (s *stubScrobbleStore) ListServices(ctx context.Context, userID int64) ([]*model.ScrobbleService, error) {
	return s.services, s.servicesErr
}

func (s *stubScrobbleStore) ListAllEnabledServices(ctx context.Context) ([]*model.ScrobbleService, error) {
	return s.allEnabled, s.allEnabledErr
}

func (s *stubScrobbleStore) UpdateService(ctx context.Context, svc *model.ScrobbleService) error {
	return s.updateErr
}

func (s *stubScrobbleStore) DeleteService(ctx context.Context, id int64) error {
	return s.deleteErr
}

func (s *stubScrobbleStore) SetServiceVerified(ctx context.Context, id int64, verified bool, lastErr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setVerifiedCalls = append(s.setVerifiedCalls, setVerifiedCall{id, verified, lastErr})
	return s.setVerifiedErr
}

func (s *stubScrobbleStore) EnqueueScrobble(ctx context.Context, q *model.ScrobbleQueueEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enqueued = append(s.enqueued, q)
	return s.enqueueErr
}

func (s *stubScrobbleStore) ListPendingByService(ctx context.Context, serviceID int64, limit int) ([]*model.ScrobbleQueueEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendingLimits == nil {
		s.pendingLimits = map[int64]int{}
	}
	s.pendingLimits[serviceID] = limit
	if s.pendingErr != nil {
		if err, ok := s.pendingErr[serviceID]; ok {
			return nil, err
		}
	}
	if s.pendingByService == nil {
		return nil, nil
	}
	return s.pendingByService[serviceID], nil
}

func (s *stubScrobbleStore) DeleteQueueEntry(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletedQueueIDs = append(s.deletedQueueIDs, id)
	return s.deleteQueueErr
}

func (s *stubScrobbleStore) IncrementAttempts(ctx context.Context, id int64, lastErr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.incrementCalls = append(s.incrementCalls, incrementCall{id, lastErr})
	return s.incrementErr
}

func (s *stubScrobbleStore) PurgeStaleQueue(ctx context.Context, before time.Time, maxAttempts int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeCalled = true
	s.purgeBefore = before
	s.purgeMaxAttempts = maxAttempts
	return s.purgeErr
}

var _ store.ScrobbleStore = (*stubScrobbleStore)(nil)

func newTestService(stub *stubScrobbleStore) *Service {
	return &Service{
		db:     &store.DB{Scrobble: stub},
		logger: testLogger(),
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func sampleTrack() model.ScrobbleTrackData {
	return model.ScrobbleTrackData{
		Artist:    "Test Artist",
		Track:     "Test Track",
		Album:     "Test Album",
		Duration:  200,
		Timestamp: 1690000000,
	}
}

// --- Scrobble ---

func TestScrobble_ListServicesError(t *testing.T) {
	stub := &stubScrobbleStore{servicesErr: errors.New("db down")}
	svc := newTestService(stub)

	err := svc.Scrobble(context.Background(), 1, sampleTrack())
	if err == nil {
		t.Fatal("expected error from ListServices to propagate")
	}
}

func TestScrobble_NoServices(t *testing.T) {
	stub := &stubScrobbleStore{}
	svc := newTestService(stub)

	if err := svc.Scrobble(context.Background(), 1, sampleTrack()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stub.enqueued) != 0 {
		t.Fatalf("expected no queued entries, got %d", len(stub.enqueued))
	}
}

func TestScrobble_SkipsDisabledAndUnverified(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	stub := &stubScrobbleStore{
		services: []*model.ScrobbleService{
			{ID: 1, ServiceType: model.ServiceLastFM, BaseURL: ts.URL, Enabled: false, Verified: true},
			{ID: 2, ServiceType: model.ServiceLastFM, BaseURL: ts.URL, Enabled: true, Verified: false},
		},
	}
	svc := newTestService(stub)

	if err := svc.Scrobble(context.Background(), 1, sampleTrack()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected no HTTP calls for disabled/unverified services, got %d", calls)
	}
	if len(stub.enqueued) != 0 {
		t.Fatalf("expected no queued entries, got %d", len(stub.enqueued))
	}
}

func TestScrobble_SuccessDoesNotQueue(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"scrobbles":{}}`))
	}))
	defer ts.Close()

	stub := &stubScrobbleStore{
		services: []*model.ScrobbleService{
			{ID: 1, ServiceType: model.ServiceLastFM, BaseURL: ts.URL, Enabled: true, Verified: true},
		},
	}
	svc := newTestService(stub)

	if err := svc.Scrobble(context.Background(), 1, sampleTrack()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stub.enqueued) != 0 {
		t.Fatalf("expected no queued entries on success, got %d", len(stub.enqueued))
	}
}

func TestScrobble_FailureQueuesRetry(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}))
	defer ts.Close()

	stub := &stubScrobbleStore{
		services: []*model.ScrobbleService{
			{ID: 42, UserID: 7, ServiceType: model.ServiceLastFM, BaseURL: ts.URL, Enabled: true, Verified: true},
		},
	}
	svc := newTestService(stub)
	track := sampleTrack()

	if err := svc.Scrobble(context.Background(), 7, track); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stub.enqueued) != 1 {
		t.Fatalf("expected 1 queued entry, got %d", len(stub.enqueued))
	}
	entry := stub.enqueued[0]
	if entry.UserID != 7 || entry.ServiceID != 42 {
		t.Errorf("queued entry mismatch: got userID=%d serviceID=%d", entry.UserID, entry.ServiceID)
	}
	if entry.TrackData != track {
		t.Errorf("queued entry track data mismatch: got %+v want %+v", entry.TrackData, track)
	}
}

func TestScrobble_RoutesListenBrainzProtocol(t *testing.T) {
	var gotPath string
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	stub := &stubScrobbleStore{
		services: []*model.ScrobbleService{
			{ID: 1, ServiceType: model.ServiceListenBrainz, BaseURL: ts.URL, TokenEnc: "tok123", Enabled: true, Verified: true},
		},
	}
	svc := newTestService(stub)

	if err := svc.Scrobble(context.Background(), 1, sampleTrack()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/1/submit-listens" {
		t.Errorf("expected ListenBrainz submit path, got %q", gotPath)
	}
	if gotAuth != "Token tok123" {
		t.Errorf("expected token auth header, got %q", gotAuth)
	}
	if len(stub.enqueued) != 0 {
		t.Fatalf("expected no queued entries on success, got %d", len(stub.enqueued))
	}
}

// --- Queue ---

func TestQueue_Success(t *testing.T) {
	stub := &stubScrobbleStore{}
	svc := newTestService(stub)
	track := sampleTrack()

	if err := svc.Queue(context.Background(), 3, 9, track); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stub.enqueued) != 1 {
		t.Fatalf("expected 1 enqueued entry, got %d", len(stub.enqueued))
	}
	entry := stub.enqueued[0]
	if entry.UserID != 3 || entry.ServiceID != 9 || entry.TrackData != track {
		t.Errorf("unexpected entry: %+v", entry)
	}
}

func TestQueue_PropagatesError(t *testing.T) {
	stub := &stubScrobbleStore{enqueueErr: errors.New("insert failed")}
	svc := newTestService(stub)

	err := svc.Queue(context.Background(), 3, 9, sampleTrack())
	if err == nil {
		t.Fatal("expected error from EnqueueScrobble to propagate")
	}
}

// --- DrainQueue ---

func TestDrainQueue_ListAllEnabledError(t *testing.T) {
	stub := &stubScrobbleStore{allEnabledErr: errors.New("boom")}
	svc := newTestService(stub)

	err := svc.DrainQueue(context.Background())
	if err == nil {
		t.Fatal("expected error from ListAllEnabledServices to propagate")
	}
	if stub.purgeCalled {
		t.Error("purge should not run if listing services failed")
	}
}

func TestDrainQueue_NoServicesStillPurges(t *testing.T) {
	stub := &stubScrobbleStore{}
	svc := newTestService(stub)

	if err := svc.DrainQueue(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stub.purgeCalled {
		t.Fatal("expected PurgeStaleQueue to be called even with no services")
	}
	if stub.purgeMaxAttempts != 50 {
		t.Errorf("expected maxAttempts=50, got %d", stub.purgeMaxAttempts)
	}
	wantCutoff := time.Now().Add(-14 * 24 * time.Hour)
	if diff := wantCutoff.Sub(stub.purgeBefore); diff < -time.Minute || diff > time.Minute {
		t.Errorf("purge cutoff not ~14 days ago: got %v want ~%v", stub.purgeBefore, wantCutoff)
	}
}

func TestDrainQueue_SkipsDisabledAndUnverified(t *testing.T) {
	stub := &stubScrobbleStore{
		allEnabled: []*model.ScrobbleService{
			{ID: 1, ServiceType: model.ServiceLastFM, Enabled: false, Verified: true},
			{ID: 2, ServiceType: model.ServiceLastFM, Enabled: true, Verified: false},
		},
	}
	svc := newTestService(stub)

	if err := svc.DrainQueue(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stub.pendingLimits) != 0 {
		t.Errorf("expected no ListPendingByService calls, got %v", stub.pendingLimits)
	}
}

func TestDrainQueue_BatchSizesByProtocol(t *testing.T) {
	stub := &stubScrobbleStore{
		allEnabled: []*model.ScrobbleService{
			{ID: 1, ServiceType: model.ServiceLastFM, Enabled: true, Verified: true},
			{ID: 2, ServiceType: model.ServiceListenBrainz, Enabled: true, Verified: true},
		},
	}
	svc := newTestService(stub)

	if err := svc.DrainQueue(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.pendingLimits[1] != 50 {
		t.Errorf("expected lastfm batch size 50, got %d", stub.pendingLimits[1])
	}
	if stub.pendingLimits[2] != 1000 {
		t.Errorf("expected listenbrainz batch size 1000, got %d", stub.pendingLimits[2])
	}
}

func TestDrainQueue_ListPendingErrorContinuesToNextService(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	stub := &stubScrobbleStore{
		allEnabled: []*model.ScrobbleService{
			{ID: 1, ServiceType: model.ServiceLastFM, Enabled: true, Verified: true},
			{ID: 2, ServiceType: model.ServiceLastFM, BaseURL: ts.URL, Enabled: true, Verified: true},
		},
		pendingErr: map[int64]error{1: errors.New("list failed")},
		pendingByService: map[int64][]*model.ScrobbleQueueEntry{
			2: {{ID: 100, TrackData: sampleTrack()}},
		},
	}
	svc := newTestService(stub)

	if err := svc.DrainQueue(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stub.deletedQueueIDs) != 1 || stub.deletedQueueIDs[0] != 100 {
		t.Errorf("expected entry 100 from service 2 to be processed, got deleted=%v", stub.deletedQueueIDs)
	}
	if !stub.purgeCalled {
		t.Error("expected purge to still run after a per-service list error")
	}
}

func TestDrainQueue_SuccessDeletesEntry(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	stub := &stubScrobbleStore{
		allEnabled: []*model.ScrobbleService{
			{ID: 1, ServiceType: model.ServiceLastFM, BaseURL: ts.URL, Enabled: true, Verified: true},
		},
		pendingByService: map[int64][]*model.ScrobbleQueueEntry{
			1: {{ID: 55, TrackData: sampleTrack()}},
		},
	}
	svc := newTestService(stub)

	if err := svc.DrainQueue(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stub.deletedQueueIDs) != 1 || stub.deletedQueueIDs[0] != 55 {
		t.Fatalf("expected queue entry 55 deleted, got %v", stub.deletedQueueIDs)
	}
	if len(stub.incrementCalls) != 0 {
		t.Errorf("expected no IncrementAttempts calls on success, got %v", stub.incrementCalls)
	}
}

func TestDrainQueue_FailureIncrementsAttempts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	stub := &stubScrobbleStore{
		allEnabled: []*model.ScrobbleService{
			{ID: 1, ServiceType: model.ServiceLastFM, BaseURL: ts.URL, Enabled: true, Verified: true},
		},
		pendingByService: map[int64][]*model.ScrobbleQueueEntry{
			1: {{ID: 77, TrackData: sampleTrack()}},
		},
	}
	svc := newTestService(stub)

	if err := svc.DrainQueue(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stub.deletedQueueIDs) != 0 {
		t.Errorf("expected no deletion on failure, got %v", stub.deletedQueueIDs)
	}
	if len(stub.incrementCalls) != 1 || stub.incrementCalls[0].id != 77 {
		t.Fatalf("expected IncrementAttempts(77, ...), got %v", stub.incrementCalls)
	}
	if stub.incrementCalls[0].lastErr == "" {
		t.Error("expected a non-empty last error message")
	}
}

// --- Verify ---

func TestVerify_LastFMSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"user":{"name":"tester"}}`))
	}))
	defer ts.Close()

	stub := &stubScrobbleStore{}
	svc := newTestService(stub)
	target := &model.ScrobbleService{ID: 5, ServiceType: model.ServiceLastFM, BaseURL: ts.URL, SessionKeyEnc: "sk"}

	if err := svc.Verify(context.Background(), target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stub.setVerifiedCalls) != 1 {
		t.Fatalf("expected 1 SetServiceVerified call, got %d", len(stub.setVerifiedCalls))
	}
	call := stub.setVerifiedCalls[0]
	if call.id != 5 || !call.verified || call.lastErr != "" {
		t.Errorf("unexpected call: %+v", call)
	}
}

func TestVerify_LastFMFailure(t *testing.T) {
	stub := &stubScrobbleStore{}
	svc := newTestService(stub)
	// No SessionKeyEnc set: verifyLastFM fails without an HTTP call.
	target := &model.ScrobbleService{ID: 6, ServiceType: model.ServiceLastFM}

	err := svc.Verify(context.Background(), target)
	if err == nil {
		t.Fatal("expected verify error for missing session key")
	}
	if len(stub.setVerifiedCalls) != 1 {
		t.Fatalf("expected 1 SetServiceVerified call, got %d", len(stub.setVerifiedCalls))
	}
	call := stub.setVerifiedCalls[0]
	if call.id != 6 || call.verified || call.lastErr == "" {
		t.Errorf("unexpected call: %+v", call)
	}
}

func TestVerify_ListenBrainzSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":200,"message":"Token valid.","user_name":"tester"}`))
	}))
	defer ts.Close()

	stub := &stubScrobbleStore{}
	svc := newTestService(stub)
	target := &model.ScrobbleService{ID: 8, ServiceType: model.ServiceListenBrainz, BaseURL: ts.URL, TokenEnc: "tok"}

	if err := svc.Verify(context.Background(), target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := stub.setVerifiedCalls[0]
	if !call.verified {
		t.Errorf("expected verified=true, got %+v", call)
	}
}

func TestVerify_ListenBrainzFailureSetsUnverified(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":401,"message":"Invalid token."}`))
	}))
	defer ts.Close()

	stub := &stubScrobbleStore{}
	svc := newTestService(stub)
	target := &model.ScrobbleService{ID: 9, ServiceType: model.ServiceListenBrainz, BaseURL: ts.URL, TokenEnc: "tok"}

	err := svc.Verify(context.Background(), target)
	if err == nil {
		t.Fatal("expected error for rejected token")
	}
	call := stub.setVerifiedCalls[0]
	if call.verified || call.lastErr == "" {
		t.Errorf("unexpected call: %+v", call)
	}
}

func TestVerify_SetServiceVerifiedErrorDoesNotOverrideOriginalError(t *testing.T) {
	stub := &stubScrobbleStore{setVerifiedErr: errors.New("db write failed")}
	svc := newTestService(stub)
	target := &model.ScrobbleService{ID: 1, ServiceType: model.ServiceLastFM}

	err := svc.Verify(context.Background(), target)
	if err == nil {
		t.Fatal("expected the original verify error, not the SetServiceVerified error")
	}
}
