package scheduler

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// logRotationKeep is how many rotated log files to retain.
const logRotationKeep = 7

// BackupService is the interface for the backup service (avoids import cycle with backup package).
type BackupService interface {
	Backup(ctx context.Context) (string, error)
}

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

// nextSundayAt returns the next Sunday at the given hour in local time.
func nextSundayAt(hour int) time.Time {
	now := time.Now()
	candidate := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	daysUntilSunday := (int(time.Sunday) - int(now.Weekday()) + 7) % 7
	if daysUntilSunday == 0 && candidate.Before(now) {
		daysUntilSunday = 7
	}
	return candidate.AddDate(0, 0, daysUntilSunday)
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

// SSLRenewalJob checks certs 7 days before expiry and triggers renewal.
// Actual renewal is handled by the ssl package on the next startup; this job
// logs a warning when a cert is within the renewal window.
// Schedule: daily at 03:00
func SSLRenewalJob(certDir string, logger *log.Logger) Job {
	return Job{
		Name:     "ssl_renewal",
		Interval: 24 * time.Hour,
		RunAt:    nextOccurrence(3),
		Fn: func(ctx context.Context) error {
			return checkSSLExpiry(certDir, 7*24*time.Hour, logger)
		},
	}
}

// checkSSLExpiry walks certDir looking for cert.pem files and logs a warning
// for any certificate that expires within the given threshold.
func checkSSLExpiry(certDir string, threshold time.Duration, logger *log.Logger) error {
	entries, err := os.ReadDir(certDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ssl_renewal: read cert dir: %w", err)
	}

	now := time.Now()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		certPath := filepath.Join(certDir, entry.Name(), "cert.pem")
		data, err := os.ReadFile(certPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			logger.Printf("[ssl_renewal] read %s: %v", certPath, err)
			continue
		}

		certs, err := x509.ParseCertificates(data)
		if err != nil || len(certs) == 0 {
			logger.Printf("[ssl_renewal] parse %s: %v", certPath, err)
			continue
		}

		expiry := certs[0].NotAfter
		remaining := expiry.Sub(now)
		if remaining <= threshold {
			logger.Printf("[ssl_renewal] SSL renewal needed for %s: expires in %s (%s)",
				entry.Name(), remaining.Round(time.Hour), expiry.Format(time.RFC3339))
		}
	}
	return nil
}

// GeoIPUpdateJob downloads updated GeoIP MMDB files from jsDelivr CDN.
// Downloads to geoipDir using atomic write (.tmp → rename).
// Schedule: weekly Sunday 03:00
func GeoIPUpdateJob(geoipDir string, logger *log.Logger) Job {
	return Job{
		Name:     "geoip_update",
		Interval: 7 * 24 * time.Hour,
		RunAt:    nextSundayAt(3),
		Fn: func(ctx context.Context) error {
			return downloadGeoIPFiles(ctx, geoipDir, logger)
		},
	}
}

// geoIPFile describes one GeoIP database file to download.
type geoIPFile struct {
	name string
	url  string
}

// geoIPFiles lists the GeoIP MMDB files from the sapics/ip-location-db CDN.
var geoIPFiles = []geoIPFile{
	{
		name: "asn.mmdb",
		url:  "https://cdn.jsdelivr.net/npm/@ip-location-db/asn-mmdb/asn.mmdb",
	},
	{
		name: "country.mmdb",
		url:  "https://cdn.jsdelivr.net/npm/@ip-location-db/geo-whois-asn-country-mmdb/geo-whois-asn-country.mmdb",
	},
	{
		name: "city.mmdb",
		url:  "https://cdn.jsdelivr.net/npm/@ip-location-db/dbip-city-mmdb/dbip-city-ipv4.mmdb",
	},
}

// downloadGeoIPFiles downloads all GeoIP database files to geoipDir atomically.
func downloadGeoIPFiles(ctx context.Context, geoipDir string, logger *log.Logger) error {
	if err := os.MkdirAll(geoipDir, 0750); err != nil {
		return fmt.Errorf("geoip_update: mkdir: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Minute}

	for _, f := range geoIPFiles {
		if err := downloadAtomic(ctx, client, f.url, filepath.Join(geoipDir, f.name)); err != nil {
			return fmt.Errorf("geoip_update: %s: %w", f.name, err)
		}
		logger.Printf("[geoip_update] updated %s", f.name)
	}
	return nil
}

// downloadAtomic fetches url and writes to dest via an atomic .tmp → rename.
func downloadAtomic(ctx context.Context, client *http.Client, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}

	tmp := dest + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("write: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close: %w", err)
	}
	return os.Rename(tmp, dest)
}

// BlocklistUpdateJob downloads IP/domain blocklists.
// Stores in dataDir/security/blocklist.txt using atomic write.
// Schedule: daily at 04:00
func BlocklistUpdateJob(dataDir string, logger *log.Logger) Job {
	return Job{
		Name:     "blocklist_update",
		Interval: 24 * time.Hour,
		RunAt:    nextOccurrence(4),
		Fn: func(ctx context.Context) error {
			return updateBlocklist(ctx, dataDir, logger)
		},
	}
}

// blocklistURL is the public blocklist source.
const blocklistURL = "https://raw.githubusercontent.com/hagezi/dns-blocklists/main/adblock/pro.txt"

// updateBlocklist downloads the blocklist and stores it atomically.
func updateBlocklist(ctx context.Context, dataDir string, logger *log.Logger) error {
	secDir := filepath.Join(dataDir, "security")
	if err := os.MkdirAll(secDir, 0750); err != nil {
		return fmt.Errorf("blocklist_update: mkdir: %w", err)
	}

	dest := filepath.Join(secDir, "blocklist.txt")
	client := &http.Client{Timeout: 10 * time.Minute}

	if err := downloadAtomic(ctx, client, blocklistURL, dest); err != nil {
		return fmt.Errorf("blocklist_update: %w", err)
	}

	logger.Printf("[blocklist_update] updated %s", dest)
	return nil
}

// CVEUpdateJob writes a timestamp to dataDir/security/cve_last_check.
// A full CVE scan is out of scope; this records the last health-check time.
// Schedule: daily at 05:00
func CVEUpdateJob(dataDir string, logger *log.Logger) Job {
	return Job{
		Name:     "cve_update",
		Interval: 24 * time.Hour,
		RunAt:    nextOccurrence(5),
		Fn: func(ctx context.Context) error {
			return runCVECheck(ctx, dataDir, logger)
		},
	}
}

// nvdHealthURL is used to validate NVD API availability (one result).
const nvdHealthURL = "https://services.nvd.nist.gov/rest/json/cves/2.0?resultsPerPage=1"

// runCVECheck pings the NVD API and records the last-check timestamp.
func runCVECheck(ctx context.Context, dataDir string, logger *log.Logger) error {
	secDir := filepath.Join(dataDir, "security")
	if err := os.MkdirAll(secDir, 0750); err != nil {
		return fmt.Errorf("cve_update: mkdir: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, nvdHealthURL, nil)
	if err != nil {
		return fmt.Errorf("cve_update: build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Printf("[cve_update] NVD API unreachable: %v", err)
	} else {
		resp.Body.Close()
		logger.Printf("[cve_update] NVD API responded: HTTP %d", resp.StatusCode)
	}

	stamp := filepath.Join(secDir, "cve_last_check")
	if writeErr := os.WriteFile(stamp, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0640); writeErr != nil {
		return fmt.Errorf("cve_update: write timestamp: %w", writeErr)
	}
	return nil
}

// SessionCleanupJob removes expired sessions from the users DB.
// Schedule: every 15 minutes
func SessionCleanupJob(users store.UserStore) Job {
	return Job{
		Name:     "session_cleanup",
		Interval: 15 * time.Minute,
		Fn: func(ctx context.Context) error {
			return users.PurgeExpiredSessions(ctx)
		},
	}
}

// TokenCleanupJob removes expired API tokens from the users DB.
// Schedule: every 15 minutes
func TokenCleanupJob(users store.UserStore) Job {
	return Job{
		Name:     "token_cleanup",
		Interval: 15 * time.Minute,
		Fn: func(ctx context.Context) error {
			return purgeExpiredTokens(ctx, users)
		},
	}
}

// purgeExpiredTokens lists all users and removes expired API tokens.
// The store does not expose a bulk delete, so we iterate per user.
func purgeExpiredTokens(ctx context.Context, users store.UserStore) error {
	all, err := users.ListUsers(ctx)
	if err != nil {
		return fmt.Errorf("token_cleanup: list users: %w", err)
	}
	for _, u := range all {
		tokens, err := users.ListAPITokens(ctx, u.ID)
		if err != nil {
			return fmt.Errorf("token_cleanup: list tokens for user %d: %w", u.ID, err)
		}
		for _, t := range tokens {
			if t.ExpiresAt.IsZero() {
				continue
			}
			if time.Now().After(t.ExpiresAt) {
				if delErr := users.DeleteAPIToken(ctx, t.ID); delErr != nil {
					return fmt.Errorf("token_cleanup: delete token %d: %w", t.ID, delErr)
				}
			}
		}
	}
	return nil
}

// LogRotationJob rotates and gzips log files in logDir.
// Renames {name}.log → {name}-YYYY-MM-DD.log.gz, keeps last logRotationKeep rotated files.
// Schedule: daily at 00:00
func LogRotationJob(logDir string, logger *log.Logger) Job {
	return Job{
		Name:     "log_rotation",
		Interval: 24 * time.Hour,
		RunAt:    nextOccurrence(0),
		Fn: func(ctx context.Context) error {
			return rotateLogs(logDir, logger)
		},
	}
}

// rotateLogs rotates all *.log files in logDir by compressing them with gzip.
func rotateLogs(logDir string, logger *log.Logger) error {
	entries, err := os.ReadDir(logDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("log_rotation: read dir: %w", err)
	}

	stamp := time.Now().Format("2006-01-02")

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}

		src := filepath.Join(logDir, entry.Name())
		base := strings.TrimSuffix(entry.Name(), ".log")
		dst := filepath.Join(logDir, base+"-"+stamp+".log.gz")

		if err := compressFile(src, dst); err != nil {
			logger.Printf("[log_rotation] compress %s: %v", src, err)
			continue
		}

		if err := os.Truncate(src, 0); err != nil {
			logger.Printf("[log_rotation] truncate %s: %v", src, err)
		}

		logger.Printf("[log_rotation] rotated %s → %s", src, dst)
	}

	pruneRotatedLogs(logDir, logger)
	return nil
}

// compressFile gzip-compresses src into dst.
func compressFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return fmt.Errorf("create dst: %w", err)
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	if _, err := io.Copy(gz, in); err != nil {
		return fmt.Errorf("compress: %w", err)
	}
	return gz.Close()
}

// pruneRotatedLogs deletes the oldest rotated log files beyond logRotationKeep.
func pruneRotatedLogs(logDir string, logger *log.Logger) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return
	}

	var rotated []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".log.gz") {
			rotated = append(rotated, filepath.Join(logDir, e.Name()))
		}
	}

	sort.Strings(rotated)

	if len(rotated) <= logRotationKeep {
		return
	}

	for _, path := range rotated[:len(rotated)-logRotationKeep] {
		if err := os.Remove(path); err != nil {
			logger.Printf("[log_rotation] prune %s: %v", path, err)
		}
	}
}

// BackupDailyJob runs a full backup to backupDir.
// Keeps last 7 daily backups.
// Schedule: daily at 02:00
func BackupDailyJob(backupSvc BackupService, logger *log.Logger) Job {
	return Job{
		Name:     "backup_daily",
		Interval: 24 * time.Hour,
		RunAt:    nextOccurrence(2),
		Fn: func(ctx context.Context) error {
			path, err := backupSvc.Backup(ctx)
			if err != nil {
				return fmt.Errorf("backup_daily: %w", err)
			}
			logger.Printf("[backup_daily] completed: %s", path)
			return nil
		},
	}
}

// BackupHourlyJob runs an incremental backup every hour.
// Only register this job when explicitly configured by the operator.
func BackupHourlyJob(backupSvc BackupService, logger *log.Logger) Job {
	return Job{
		Name:     "backup_hourly",
		Interval: time.Hour,
		Fn: func(ctx context.Context) error {
			path, err := backupSvc.Backup(ctx)
			if err != nil {
				return fmt.Errorf("backup_hourly: %w", err)
			}
			logger.Printf("[backup_hourly] completed: %s", path)
			return nil
		},
	}
}

// HealthCheckSelfJob pings GET /api/v1/health on localhost and logs the result.
// Schedule: every 5 minutes
func HealthCheckSelfJob(port int, logger *log.Logger) Job {
	return Job{
		Name:     "healthcheck_self",
		Interval: 5 * time.Minute,
		Fn: func(ctx context.Context) error {
			return pingHealthEndpoint(ctx, port, logger)
		},
	}
}

// pingHealthEndpoint makes a GET request to the server's own health endpoint.
func pingHealthEndpoint(ctx context.Context, port int, logger *log.Logger) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/health", port)
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("healthcheck_self: build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Printf("[healthcheck_self] WARN: health check failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Printf("[healthcheck_self] WARN: health endpoint returned HTTP %d", resp.StatusCode)
	} else {
		logger.Printf("[healthcheck_self] OK (HTTP %d)", resp.StatusCode)
	}
	return nil
}

// TorHealthJob checks Tor connectivity via the Tor SOCKS5 proxy at 127.0.0.1:9050.
// Logs a warning if connectivity fails. Safe to run when Tor is not running.
// Schedule: every 10 minutes
func TorHealthJob(logger *log.Logger) Job {
	return Job{
		Name:     "tor_health",
		Interval: 10 * time.Minute,
		Fn: func(ctx context.Context) error {
			checkTorConnectivity(ctx, logger)
			return nil
		},
	}
}

// checkTorConnectivity tests reachability of check.torproject.org via the Tor SOCKS5 proxy.
func checkTorConnectivity(ctx context.Context, logger *log.Logger) {
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, "tcp", "127.0.0.1:9050")
			if err != nil {
				return nil, fmt.Errorf("tor socks5: %w", err)
			}
			if err := torSOCKS5Handshake(conn, addr); err != nil {
				conn.Close()
				return nil, err
			}
			return conn, nil
		},
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://check.torproject.org/api/ip", nil)
	if err != nil {
		logger.Printf("[tor_health] build request: %v", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Printf("[tor_health] WARN: Tor connectivity check failed: %v", err)
		return
	}
	defer resp.Body.Close()
	logger.Printf("[tor_health] OK: check.torproject.org responded HTTP %d", resp.StatusCode)
}

// torSOCKS5Handshake performs a minimal SOCKS5 CONNECT handshake to addr ("host:port").
func torSOCKS5Handshake(conn net.Conn, addr string) error {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("split host:port: %w", err)
	}

	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return fmt.Errorf("parse port: %w", err)
	}

	// Greeting: VER=5, NMETHODS=1, METHOD=0 (no auth)
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return fmt.Errorf("socks5 greeting: %w", err)
	}

	// Server method selection: expect VER=5, METHOD=0
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("socks5 method response: %w", err)
	}
	if buf[0] != 0x05 || buf[1] != 0x00 {
		return fmt.Errorf("socks5: unexpected method response: %x", buf)
	}

	// CONNECT request: VER, CMD=CONNECT, RSV, ATYP=DOMAINNAME, len, host, port
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	req = append(req, []byte(host)...)
	req = append(req, byte(port>>8), byte(port&0xFF))
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("socks5 connect: %w", err)
	}

	// Read CONNECT response header (at least 10 bytes)
	hdr := make([]byte, 10)
	if _, err := io.ReadFull(conn, hdr); err != nil {
		return fmt.Errorf("socks5 connect response: %w", err)
	}
	if hdr[1] != 0x00 {
		return fmt.Errorf("socks5 connect rejected: code %d", hdr[1])
	}
	return nil
}

// ClusterHeartbeatJob is a no-op placeholder for single-node deployments.
// In cluster mode (future), this would update a heartbeat record in server.db.
// Schedule: every 30 seconds
func ClusterHeartbeatJob(db *store.DB, logger *log.Logger) Job {
	return Job{
		Name:     "cluster_heartbeat",
		Interval: 30 * time.Second,
		Fn: func(ctx context.Context) error {
			// Single-node: no-op. Cluster heartbeat logic will be implemented
			// when clustering support is added.
			return nil
		},
	}
}
