// Package metrics defines all Prometheus metrics exported by cassonic.
// All metric names are prefixed with cassonic_ per PART 21 of the spec.
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// HTTPRequests counts every HTTP request handled by cassonic, labelled by method, path, and status.
	HTTPRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cassonic_http_requests_total",
		Help: "Total HTTP requests handled by cassonic.",
	}, []string{"method", "path", "status"})

	// ActiveStreams is the number of currently active audio streams.
	ActiveStreams = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cassonic_active_streams",
		Help: "Number of currently active audio streams.",
	})

	// LibrarySongs is the total number of songs in the library.
	LibrarySongs = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cassonic_library_songs_total",
		Help: "Total songs in the library.",
	})

	// LibraryAlbums is the total number of albums in the library.
	LibraryAlbums = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cassonic_library_albums_total",
		Help: "Total albums in the library.",
	})

	// LibraryArtists is the total number of artists in the library.
	LibraryArtists = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cassonic_library_artists_total",
		Help: "Total artists in the library.",
	})

	// ScanDuration measures library scan duration in seconds.
	ScanDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "cassonic_scan_duration_seconds",
		Help:    "Library scan duration in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	// ScanFilesTotal counts total files processed across all library scans.
	ScanFilesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cassonic_scan_files_total",
		Help: "Total files processed across all library scans.",
	})

	// Scrobbles counts scrobble attempts by service and outcome.
	Scrobbles = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cassonic_scrobbles_total",
		Help: "Total scrobble attempts by service and outcome.",
	}, []string{"service", "status"})

	// AuthAttempts counts authentication attempts by scheme and result.
	AuthAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cassonic_auth_attempts_total",
		Help: "Authentication attempts by scheme and result.",
	}, []string{"scheme", "result"})

	// IcecastMountsActive is the number of active Icecast mounts.
	IcecastMountsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cassonic_icecast_mounts_active",
		Help: "Number of active Icecast mounts.",
	})

	// SchedulerRuns counts scheduler job runs by name and outcome.
	SchedulerRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cassonic_scheduler_runs_total",
		Help: "Scheduler job runs by name and outcome.",
	}, []string{"job", "status"})
)

func init() {
	prometheus.MustRegister(
		HTTPRequests,
		ActiveStreams,
		LibrarySongs, LibraryAlbums, LibraryArtists,
		ScanDuration, ScanFilesTotal,
		Scrobbles,
		AuthAttempts,
		IcecastMountsActive,
		SchedulerRuns,
	)
}
