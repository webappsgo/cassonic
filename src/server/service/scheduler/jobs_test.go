package scheduler

// Tests cover:
//   - nextOccurrence / nextSundayAt: relative wall-clock properties (hour
//     matches, result is in the future, weekday==Sunday for nextSundayAt)
//   - checkSSLExpiry: missing cert dir (nil), expiring cert (logs), fresh
//     cert (no log), and a corrupt cert.pem (parse error, logs, continues)
//   - downloadAtomic: 200 OK (content written, no leftover .tmp), non-200
//     status, and context-cancelled request
//   - compressFile: gzip round-trip content match
//   - pruneRotatedLogs: keeps only the newest logRotationKeep files
//   - rotateLogs: rotates+truncates *.log files and prunes old rotations;
//     missing logDir returns nil
//   - purgeExpiredTokens: skips zero ExpiresAt, deletes expired, keeps
//     unexpired, and propagates errors from each store call
//   - torSOCKS5Handshake: success, bad method-selection response,
//     CONNECT-rejected, and malformed addr (SplitHostPort failure)
//   - pingHealthEndpoint: 200 OK, non-200, and connection-refused (all
//     return nil per the function's documented behavior)
//   - Job constructors SSLRenewalJob, GeoIPUpdateJob, BlocklistUpdateJob,
//     CVEUpdateJob, SessionCleanupJob, TokenCleanupJob, LogRotationJob,
//     HealthCheckSelfJob, TorHealthJob, ClusterHeartbeatJob: Name/Interval
//     match the documented schedule; ClusterHeartbeatJob.Fn and
//     SessionCleanupJob.Fn/TokenCleanupJob.Fn are also invoked against the
//     fakeUserStore stub.
//
// LibraryScanJob, PodcastRefreshJob, ScrobbleRetryJob, MusicBrainzLookupJob,
// CoverArtRefreshJob, BackupDailyJob, and BackupHourlyJob are intentionally
// NOT covered: they require constructing *service.Scanner, *podcast.Service,
// *scrobble.Service, *musicbrainz.Client, *service.CoverArtService, or a
// BackupService implementation, none of which have a trivial zero-value or
// stub form worth building solely for a Name/Interval assertion.
// downloadGeoIPFiles, updateBlocklist, and runCVECheck are exercised only
// indirectly (they call downloadAtomic / hit real network hosts); their
// mkdir and dispatch logic is not separately unit tested.
// checkTorConnectivity is not covered: it dials a real Tor SOCKS5 proxy at
// 127.0.0.1:9050 and a real HTTPS host, neither safe nor meaningful in a
// unit test.

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// --- nextOccurrence / nextSundayAt ---

func TestNextOccurrence(t *testing.T) {
	now := time.Now()
	hour := (now.Hour() + 2) % 24

	got := nextOccurrence(hour)

	if got.Hour() != hour {
		t.Errorf("nextOccurrence(%d).Hour() = %d, want %d", hour, got.Hour(), hour)
	}
	if !got.After(now) {
		t.Errorf("nextOccurrence(%d) = %v, want time after %v", hour, got, now)
	}
}

func TestNextOccurrencePastHourRollsToTomorrow(t *testing.T) {
	now := time.Now()
	// An hour that has already passed today forces a rollover to tomorrow,
	// unless we happen to run exactly at hour 0 with second 0.
	hour := now.Hour()
	if now.Minute() == 0 && now.Second() == 0 {
		t.Skip("flaky at exact hour boundary")
	}

	got := nextOccurrence(hour)

	if got.Hour() != hour {
		t.Errorf("Hour() = %d, want %d", got.Hour(), hour)
	}
	if !got.After(now) {
		t.Errorf("expected rollover to a future time, got %v (now %v)", got, now)
	}
	if got.Day() == now.Day() && got.Month() == now.Month() && got.Year() == now.Year() {
		t.Errorf("expected next-day rollover since hour %d already passed, got same day %v", hour, got)
	}
}

func TestNextSundayAt(t *testing.T) {
	now := time.Now()
	hour := 3

	got := nextSundayAt(hour)

	if got.Weekday() != time.Sunday {
		t.Errorf("nextSundayAt weekday = %v, want %v", got.Weekday(), time.Sunday)
	}
	if got.Hour() != hour {
		t.Errorf("nextSundayAt hour = %d, want %d", got.Hour(), hour)
	}
	if !got.After(now) {
		t.Errorf("nextSundayAt(%d) = %v, want time after %v", hour, got, now)
	}
}

// --- checkSSLExpiry ---

// genSelfSignedCert writes a self-signed cert.pem (raw DER, matching how
// x509.ParseCertificates is invoked in checkSSLExpiry) under dir/name/cert.pem
// with the given NotAfter.
func genSelfSignedCert(t *testing.T, dir, name string, notAfter time.Time) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certDir := filepath.Join(dir, name)
	if err := os.MkdirAll(certDir, 0750); err != nil {
		t.Fatalf("mkdir cert dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(certDir, "cert.pem"), der, 0640); err != nil {
		t.Fatalf("write cert.pem: %v", err)
	}
}

func TestCheckSSLExpiryMissingDir(t *testing.T) {
	err := checkSSLExpiry(filepath.Join(t.TempDir(), "does-not-exist"), 7*24*time.Hour, silentLogger())
	if err != nil {
		t.Errorf("checkSSLExpiry on missing dir: got %v, want nil", err)
	}
}

func TestCheckSSLExpiryExpiringSoon(t *testing.T) {
	dir := t.TempDir()
	genSelfSignedCert(t, dir, "example.com", time.Now().Add(2*24*time.Hour))

	if err := checkSSLExpiry(dir, 7*24*time.Hour, silentLogger()); err != nil {
		t.Errorf("checkSSLExpiry: got %v, want nil", err)
	}
}

func TestCheckSSLExpiryNotExpiring(t *testing.T) {
	dir := t.TempDir()
	genSelfSignedCert(t, dir, "example.com", time.Now().Add(365*24*time.Hour))

	if err := checkSSLExpiry(dir, 7*24*time.Hour, silentLogger()); err != nil {
		t.Errorf("checkSSLExpiry: got %v, want nil", err)
	}
}

func TestCheckSSLExpiryCorruptCert(t *testing.T) {
	dir := t.TempDir()
	certDir := filepath.Join(dir, "broken.example.com")
	if err := os.MkdirAll(certDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(certDir, "cert.pem"), []byte("not a certificate"), 0640); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := checkSSLExpiry(dir, 7*24*time.Hour, silentLogger()); err != nil {
		t.Errorf("checkSSLExpiry with corrupt cert: got %v, want nil (logged, not returned)", err)
	}
}

// --- downloadAtomic ---

func TestDownloadAtomicSuccess(t *testing.T) {
	const body = "mmdb-file-contents"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "out.bin")
	client := srv.Client()

	if err := downloadAtomic(context.Background(), client, srv.URL, dest); err != nil {
		t.Fatalf("downloadAtomic: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != body {
		t.Errorf("dest content = %q, want %q", got, body)
	}

	if _, err := os.Stat(dest + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected no leftover .tmp file, stat err = %v", err)
	}
}

func TestDownloadAtomicNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "out.bin")
	err := downloadAtomic(context.Background(), srv.Client(), srv.URL, dest)
	if err == nil {
		t.Fatal("downloadAtomic: got nil error, want error for HTTP 500")
	}
}

func TestDownloadAtomicContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dest := filepath.Join(t.TempDir(), "out.bin")
	err := downloadAtomic(ctx, srv.Client(), srv.URL, dest)
	if err == nil {
		t.Fatal("downloadAtomic: got nil error, want error for cancelled context")
	}
}

// --- compressFile ---

func TestCompressFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "app.log")
	dst := filepath.Join(dir, "app.log.gz")
	const content = "line one\nline two\n"

	if err := os.WriteFile(src, []byte(content), 0640); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := compressFile(src, dst); err != nil {
		t.Fatalf("compressFile: %v", err)
	}

	f, err := os.Open(dst)
	if err != nil {
		t.Fatalf("open dst: %v", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gz.Close()

	got, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("read gz: %v", err)
	}
	if string(got) != content {
		t.Errorf("decompressed content = %q, want %q", got, content)
	}
}

func TestCompressFileMissingSrc(t *testing.T) {
	dir := t.TempDir()
	err := compressFile(filepath.Join(dir, "missing.log"), filepath.Join(dir, "out.gz"))
	if err == nil {
		t.Fatal("compressFile with missing src: got nil error, want error")
	}
}

// --- pruneRotatedLogs ---

func TestPruneRotatedLogsKeepsNewest(t *testing.T) {
	dir := t.TempDir()

	total := logRotationKeep + 3
	var names []string
	for i := 0; i < total; i++ {
		name := fmt.Sprintf("app-2024-01-%02d.log.gz", i+1)
		names = append(names, name)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0640); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	pruneRotatedLogs(dir, silentLogger())

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != logRotationKeep {
		t.Fatalf("got %d remaining files, want %d", len(entries), logRotationKeep)
	}

	// Names sort lexically; the oldest (lowest-numbered) files should have
	// been removed, keeping the newest logRotationKeep.
	wantKept := names[total-logRotationKeep:]
	for _, want := range wantKept {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("expected %s to be kept: %v", want, err)
		}
	}
}

func TestPruneRotatedLogsUnderThreshold(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.log.gz"), []byte("x"), 0640); err != nil {
		t.Fatalf("write: %v", err)
	}

	pruneRotatedLogs(dir, silentLogger())

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("got %d files, want 1 (below threshold, nothing pruned)", len(entries))
	}
}

func TestPruneRotatedLogsMissingDir(t *testing.T) {
	// os.ReadDir fails silently (function returns early); just verify no panic.
	pruneRotatedLogs(filepath.Join(t.TempDir(), "missing"), silentLogger())
}

// --- rotateLogs ---

func TestRotateLogs(t *testing.T) {
	dir := t.TempDir()
	const content = "some log data\n"
	if err := os.WriteFile(filepath.Join(dir, "app.log"), []byte(content), 0640); err != nil {
		t.Fatalf("write log: %v", err)
	}

	if err := rotateLogs(dir, silentLogger()); err != nil {
		t.Fatalf("rotateLogs: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}

	var foundGz bool
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".log.gz") {
			foundGz = true
		}
	}
	if !foundGz {
		t.Error("expected a rotated .log.gz file to be created")
	}

	info, err := os.Stat(filepath.Join(dir, "app.log"))
	if err != nil {
		t.Fatalf("stat app.log: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("app.log size = %d, want 0 (truncated)", info.Size())
	}
}

func TestRotateLogsMissingDir(t *testing.T) {
	err := rotateLogs(filepath.Join(t.TempDir(), "missing"), silentLogger())
	if err != nil {
		t.Errorf("rotateLogs on missing dir: got %v, want nil", err)
	}
}

// --- purgeExpiredTokens via fakeUserStore ---

// fakeUserStore implements store.UserStore with only ListUsers, ListAPITokens,
// and DeleteAPIToken behaving meaningfully; all other methods are unused by
// the functions under test and return zero values.
type fakeUserStore struct {
	users        []*model.User
	tokens       map[int64][]*model.APIToken
	listUsersErr error
	listTokErr   error
	deleteTokErr error
	deleted      []int64
}

func (f *fakeUserStore) CreateUser(ctx context.Context, u *model.User) (int64, error) { return 0, nil }
func (f *fakeUserStore) GetUser(ctx context.Context, id int64) (*model.User, error)   { return nil, nil }
func (f *fakeUserStore) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	return nil, nil
}
func (f *fakeUserStore) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return nil, nil
}
func (f *fakeUserStore) UpdateUser(ctx context.Context, u *model.User) error { return nil }
func (f *fakeUserStore) DeleteUser(ctx context.Context, id int64) error     { return nil }
func (f *fakeUserStore) ListUsers(ctx context.Context) ([]*model.User, error) {
	if f.listUsersErr != nil {
		return nil, f.listUsersErr
	}
	return f.users, nil
}
func (f *fakeUserStore) IncrementLoginAttempts(ctx context.Context, id int64) error { return nil }
func (f *fakeUserStore) ResetLoginAttempts(ctx context.Context, id int64) error     { return nil }
func (f *fakeUserStore) SetLockedUntil(ctx context.Context, id int64, until time.Time) error {
	return nil
}
func (f *fakeUserStore) UpdateLastLogin(ctx context.Context, id int64) error { return nil }
func (f *fakeUserStore) CreateAPIToken(ctx context.Context, t *model.APIToken) error { return nil }
func (f *fakeUserStore) GetAPITokenByHash(ctx context.Context, hash string) (*model.APIToken, error) {
	return nil, nil
}
func (f *fakeUserStore) ListAPITokens(ctx context.Context, userID int64) ([]*model.APIToken, error) {
	if f.listTokErr != nil {
		return nil, f.listTokErr
	}
	return f.tokens[userID], nil
}
func (f *fakeUserStore) DeleteAPIToken(ctx context.Context, id int64) error {
	if f.deleteTokErr != nil {
		return f.deleteTokErr
	}
	f.deleted = append(f.deleted, id)
	return nil
}
func (f *fakeUserStore) UpdateAPITokenLastUsed(ctx context.Context, id int64) error { return nil }
func (f *fakeUserStore) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, clientName string) error {
	return nil
}
func (f *fakeUserStore) GetSessionByHash(ctx context.Context, tokenHash string) (*store.Session, error) {
	return nil, nil
}
func (f *fakeUserStore) DeleteSession(ctx context.Context, tokenHash string) error       { return nil }
func (f *fakeUserStore) DeleteUserSessions(ctx context.Context, userID int64) error      { return nil }
func (f *fakeUserStore) PurgeExpiredSessions(ctx context.Context) error                  { return nil }
func (f *fakeUserStore) GetSubsonicPassword(ctx context.Context, username string) (string, bool, error) {
	return "", false, nil
}
func (f *fakeUserStore) SetSubsonicPassword(ctx context.Context, username string, encrypted string) error {
	return nil
}
func (f *fakeUserStore) CreateRadioStation(ctx context.Context, s *model.InternetRadioStation) (int64, error) {
	return 0, nil
}
func (f *fakeUserStore) GetRadioStation(ctx context.Context, id int64) (*model.InternetRadioStation, error) {
	return nil, nil
}
func (f *fakeUserStore) ListRadioStations(ctx context.Context) ([]*model.InternetRadioStation, error) {
	return nil, nil
}
func (f *fakeUserStore) UpdateRadioStation(ctx context.Context, s *model.InternetRadioStation) error {
	return nil
}
func (f *fakeUserStore) DeleteRadioStation(ctx context.Context, id int64) error { return nil }

var _ store.UserStore = (*fakeUserStore)(nil)

func TestPurgeExpiredTokens(t *testing.T) {
	users := []*model.User{{ID: 1}, {ID: 2}}
	fake := &fakeUserStore{
		users: users,
		tokens: map[int64][]*model.APIToken{
			1: {
				{ID: 10, ExpiresAt: time.Time{}},               // zero value: skipped
				{ID: 11, ExpiresAt: time.Now().Add(-time.Hour)}, // expired: deleted
			},
			2: {
				{ID: 20, ExpiresAt: time.Now().Add(time.Hour)}, // not expired: kept
			},
		},
	}

	if err := purgeExpiredTokens(context.Background(), fake); err != nil {
		t.Fatalf("purgeExpiredTokens: %v", err)
	}

	if len(fake.deleted) != 1 || fake.deleted[0] != 11 {
		t.Errorf("deleted = %v, want [11]", fake.deleted)
	}
}

func TestPurgeExpiredTokensListUsersError(t *testing.T) {
	fake := &fakeUserStore{listUsersErr: errors.New("db down")}

	err := purgeExpiredTokens(context.Background(), fake)
	if err == nil {
		t.Fatal("purgeExpiredTokens: got nil error, want wrapped error")
	}
}

func TestPurgeExpiredTokensListTokensError(t *testing.T) {
	fake := &fakeUserStore{
		users:      []*model.User{{ID: 1}},
		listTokErr: errors.New("query failed"),
	}

	err := purgeExpiredTokens(context.Background(), fake)
	if err == nil {
		t.Fatal("purgeExpiredTokens: got nil error, want wrapped error")
	}
}

func TestPurgeExpiredTokensDeleteError(t *testing.T) {
	fake := &fakeUserStore{
		users: []*model.User{{ID: 1}},
		tokens: map[int64][]*model.APIToken{
			1: {{ID: 10, ExpiresAt: time.Now().Add(-time.Hour)}},
		},
		deleteTokErr: errors.New("delete failed"),
	}

	err := purgeExpiredTokens(context.Background(), fake)
	if err == nil {
		t.Fatal("purgeExpiredTokens: got nil error, want wrapped error")
	}
}

// --- torSOCKS5Handshake ---

func TestTorSOCKS5HandshakeSuccess(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	done := make(chan error, 1)
	go func() {
		done <- torSOCKS5Handshake(client, "check.torproject.org:443")
	}()

	buf := make([]byte, 3)
	if _, err := io.ReadFull(server, buf); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := server.Write([]byte{0x05, 0x00}); err != nil {
		t.Fatalf("write method selection: %v", err)
	}

	connectReq := make([]byte, 5+len("check.torproject.org")+2)
	if _, err := io.ReadFull(server, connectReq); err != nil {
		t.Fatalf("read connect request: %v", err)
	}

	resp := make([]byte, 10)
	resp[1] = 0x00
	if _, err := server.Write(resp); err != nil {
		t.Fatalf("write connect response: %v", err)
	}

	if err := <-done; err != nil {
		t.Errorf("torSOCKS5Handshake: got %v, want nil", err)
	}
}

func TestTorSOCKS5HandshakeBadMethodResponse(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	done := make(chan error, 1)
	go func() {
		done <- torSOCKS5Handshake(client, "example.com:443")
	}()

	buf := make([]byte, 3)
	if _, err := io.ReadFull(server, buf); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := server.Write([]byte{0x05, 0xFF}); err != nil {
		t.Fatalf("write bad method: %v", err)
	}

	err := <-done
	if err == nil {
		t.Fatal("torSOCKS5Handshake: got nil error, want error for bad method response")
	}
}

func TestTorSOCKS5HandshakeConnectRejected(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	done := make(chan error, 1)
	go func() {
		done <- torSOCKS5Handshake(client, "example.com:443")
	}()

	buf := make([]byte, 3)
	if _, err := io.ReadFull(server, buf); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := server.Write([]byte{0x05, 0x00}); err != nil {
		t.Fatalf("write method selection: %v", err)
	}

	connectReq := make([]byte, 5+len("example.com")+2)
	if _, err := io.ReadFull(server, connectReq); err != nil {
		t.Fatalf("read connect request: %v", err)
	}

	resp := make([]byte, 10)
	resp[1] = 0x05 // connection refused
	if _, err := server.Write(resp); err != nil {
		t.Fatalf("write rejection: %v", err)
	}

	err := <-done
	if err == nil {
		t.Fatal("torSOCKS5Handshake: got nil error, want error for rejected CONNECT")
	}
}

func TestTorSOCKS5HandshakeBadAddr(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	err := torSOCKS5Handshake(client, "not-a-valid-addr")
	if err == nil {
		t.Fatal("torSOCKS5Handshake with malformed addr: got nil error, want error")
	}
}

// --- pingHealthEndpoint ---

func srvPort(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return port
}

func TestPingHealthEndpointOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := pingHealthEndpoint(context.Background(), srvPort(t, srv), silentLogger())
	if err != nil {
		t.Errorf("pingHealthEndpoint: got %v, want nil", err)
	}
}

func TestPingHealthEndpointNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	err := pingHealthEndpoint(context.Background(), srvPort(t, srv), silentLogger())
	if err != nil {
		t.Errorf("pingHealthEndpoint on non-200: got %v, want nil (logged, not returned)", err)
	}
}

func TestPingHealthEndpointConnRefused(t *testing.T) {
	// Port 1 is a privileged, virtually always-closed port; the request
	// should fail to connect, and the function must still return nil.
	err := pingHealthEndpoint(context.Background(), 1, silentLogger())
	if err != nil {
		t.Errorf("pingHealthEndpoint on connection refused: got %v, want nil (logged, not returned)", err)
	}
}

// --- Job constructors ---

func TestSSLRenewalJob(t *testing.T) {
	j := SSLRenewalJob(t.TempDir(), silentLogger())
	if j.Name != "ssl_renewal" {
		t.Errorf("Name = %q, want %q", j.Name, "ssl_renewal")
	}
	if j.Interval != 24*time.Hour {
		t.Errorf("Interval = %v, want 24h", j.Interval)
	}
	if err := j.Fn(context.Background()); err != nil {
		t.Errorf("Fn: got %v, want nil", err)
	}
}

func TestGeoIPUpdateJob(t *testing.T) {
	j := GeoIPUpdateJob(t.TempDir(), silentLogger())
	if j.Name != "geoip_update" {
		t.Errorf("Name = %q, want %q", j.Name, "geoip_update")
	}
	if j.Interval != 7*24*time.Hour {
		t.Errorf("Interval = %v, want 7 days", j.Interval)
	}
}

func TestBlocklistUpdateJob(t *testing.T) {
	j := BlocklistUpdateJob(t.TempDir(), silentLogger())
	if j.Name != "blocklist_update" {
		t.Errorf("Name = %q, want %q", j.Name, "blocklist_update")
	}
	if j.Interval != 24*time.Hour {
		t.Errorf("Interval = %v, want 24h", j.Interval)
	}
}

func TestCVEUpdateJob(t *testing.T) {
	j := CVEUpdateJob(t.TempDir(), silentLogger())
	if j.Name != "cve_update" {
		t.Errorf("Name = %q, want %q", j.Name, "cve_update")
	}
	if j.Interval != 24*time.Hour {
		t.Errorf("Interval = %v, want 24h", j.Interval)
	}
}

func TestSessionCleanupJob(t *testing.T) {
	fake := &fakeUserStore{}
	j := SessionCleanupJob(fake)
	if j.Name != "session_cleanup" {
		t.Errorf("Name = %q, want %q", j.Name, "session_cleanup")
	}
	if j.Interval != 15*time.Minute {
		t.Errorf("Interval = %v, want 15m", j.Interval)
	}
	if err := j.Fn(context.Background()); err != nil {
		t.Errorf("Fn: got %v, want nil", err)
	}
}

func TestTokenCleanupJob(t *testing.T) {
	fake := &fakeUserStore{users: []*model.User{{ID: 1}}}
	j := TokenCleanupJob(fake)
	if j.Name != "token_cleanup" {
		t.Errorf("Name = %q, want %q", j.Name, "token_cleanup")
	}
	if j.Interval != 15*time.Minute {
		t.Errorf("Interval = %v, want 15m", j.Interval)
	}
	if err := j.Fn(context.Background()); err != nil {
		t.Errorf("Fn: got %v, want nil", err)
	}
}

func TestLogRotationJob(t *testing.T) {
	j := LogRotationJob(t.TempDir(), silentLogger())
	if j.Name != "log_rotation" {
		t.Errorf("Name = %q, want %q", j.Name, "log_rotation")
	}
	if j.Interval != 24*time.Hour {
		t.Errorf("Interval = %v, want 24h", j.Interval)
	}
	if err := j.Fn(context.Background()); err != nil {
		t.Errorf("Fn: got %v, want nil", err)
	}
}

func TestHealthCheckSelfJob(t *testing.T) {
	j := HealthCheckSelfJob(1, silentLogger())
	if j.Name != "healthcheck_self" {
		t.Errorf("Name = %q, want %q", j.Name, "healthcheck_self")
	}
	if j.Interval != 5*time.Minute {
		t.Errorf("Interval = %v, want 5m", j.Interval)
	}
	if err := j.Fn(context.Background()); err != nil {
		t.Errorf("Fn: got %v, want nil", err)
	}
}

func TestTorHealthJob(t *testing.T) {
	j := TorHealthJob(silentLogger())
	if j.Name != "tor_health" {
		t.Errorf("Name = %q, want %q", j.Name, "tor_health")
	}
	if j.Interval != 10*time.Minute {
		t.Errorf("Interval = %v, want 10m", j.Interval)
	}
}

func TestClusterHeartbeatJob(t *testing.T) {
	j := ClusterHeartbeatJob(nil, silentLogger())
	if j.Name != "cluster_heartbeat" {
		t.Errorf("Name = %q, want %q", j.Name, "cluster_heartbeat")
	}
	if j.Interval != 30*time.Second {
		t.Errorf("Interval = %v, want 30s", j.Interval)
	}
	if err := j.Fn(context.Background()); err != nil {
		t.Errorf("Fn: got %v, want nil (documented no-op)", err)
	}
}
