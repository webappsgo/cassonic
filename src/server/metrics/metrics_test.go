package metrics

// Tests cover:
//   - Registration: every exported metric is registered with the default
//     Prometheus registry (verified via a Gather() call) and each is
//     collectable without panicking
//   - Basic API usage for every metric type without panicking: CounterVec
//     (Inc/Add with labels), Gauge (Set/Inc/Dec/Add), Histogram (Observe),
//     Counter (Add)
//   - Deltas rather than absolute values: since these are package-level
//     singletons shared across every test in this binary, assertions use
//     testutil.ToFloat64 before/after a known mutation rather than any
//     absolute value, so test order and parallel test-binary runs cannot
//     make this test flaky
//   - CounterVec labels: distinct label combinations accumulate
//     independently (e.g. Scrobbles{service="lastfm",status="ok"} does not
//     affect Scrobbles{service="lastfm",status="error"})

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestAllMetricsAreRegistered(t *testing.T) {
	// CounterVec collectors only emit a metric family once at least one
	// label combination exists; touch each with a throwaway series so
	// Gather() reports them regardless of what other tests have run.
	HTTPRequests.WithLabelValues("registration-check", "registration-check", "registration-check")
	Scrobbles.WithLabelValues("registration-check", "registration-check")
	AuthAttempts.WithLabelValues("registration-check", "registration-check")
	SchedulerRuns.WithLabelValues("registration-check", "registration-check")

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	want := map[string]bool{
		"cassonic_http_requests_total":   false,
		"cassonic_active_streams":        false,
		"cassonic_library_songs_total":   false,
		"cassonic_library_albums_total":  false,
		"cassonic_library_artists_total": false,
		"cassonic_scan_duration_seconds": false,
		"cassonic_scan_files_total":      false,
		"cassonic_scrobbles_total":       false,
		"cassonic_auth_attempts_total":   false,
		"cassonic_icecast_mounts_active": false,
		"cassonic_scheduler_runs_total":  false,
	}

	for _, fam := range families {
		if fam.GetName() != "" {
			if _, ok := want[fam.GetName()]; ok {
				want[fam.GetName()] = true
			}
		}
	}

	for name, seen := range want {
		if !seen {
			t.Errorf("metric %q was not found in the default gatherer output", name)
		}
	}
}

func TestHTTPRequestsCounterVec(t *testing.T) {
	before := testutil.ToFloat64(HTTPRequests.WithLabelValues("GET", "/api/test", "200"))
	HTTPRequests.WithLabelValues("GET", "/api/test", "200").Inc()
	after := testutil.ToFloat64(HTTPRequests.WithLabelValues("GET", "/api/test", "200"))

	if after != before+1 {
		t.Errorf("HTTPRequests: got delta %v, want 1", after-before)
	}

	// A different label combination must not be affected.
	unrelatedBefore := testutil.ToFloat64(HTTPRequests.WithLabelValues("POST", "/api/other", "500"))
	HTTPRequests.WithLabelValues("GET", "/api/test", "200").Inc()
	unrelatedAfter := testutil.ToFloat64(HTTPRequests.WithLabelValues("POST", "/api/other", "500"))
	if unrelatedAfter != unrelatedBefore {
		t.Errorf("HTTPRequests: unrelated label combination changed: got %v, want unchanged %v", unrelatedAfter, unrelatedBefore)
	}
}

func TestActiveStreamsGauge(t *testing.T) {
	before := testutil.ToFloat64(ActiveStreams)
	ActiveStreams.Inc()
	ActiveStreams.Inc()
	ActiveStreams.Dec()
	after := testutil.ToFloat64(ActiveStreams)

	if after != before+1 {
		t.Errorf("ActiveStreams: got delta %v, want 1", after-before)
	}

	ActiveStreams.Add(5)
	afterAdd := testutil.ToFloat64(ActiveStreams)
	if afterAdd != after+5 {
		t.Errorf("ActiveStreams.Add(5): got delta %v, want 5", afterAdd-after)
	}

	ActiveStreams.Set(42)
	if got := testutil.ToFloat64(ActiveStreams); got != 42 {
		t.Errorf("ActiveStreams.Set(42): got %v, want 42", got)
	}
}

func TestLibraryGauges(t *testing.T) {
	gauges := map[string]prometheus.Gauge{
		"LibrarySongs":   LibrarySongs,
		"LibraryAlbums":  LibraryAlbums,
		"LibraryArtists": LibraryArtists,
	}
	for name, g := range gauges {
		t.Run(name, func(t *testing.T) {
			g.Set(123)
			if got := testutil.ToFloat64(g); got != 123 {
				t.Errorf("%s.Set(123): got %v, want 123", name, got)
			}
		})
	}
}

func TestScanDurationHistogramObserve(t *testing.T) {
	// Histograms have no single "current value" to diff and comparing raw
	// dto.Metric sample counts would require importing the client_model
	// package directly (which go.mod does not currently declare as a
	// direct dependency). Instead, verify Observe does not panic and that
	// the metric remains collectable by the default gatherer afterward.
	ScanDuration.Observe(1.5)
	ScanDuration.Observe(0.001)

	if n := testutil.CollectAndCount(ScanDuration); n != 1 {
		t.Errorf("ScanDuration: CollectAndCount got %d, want 1", n)
	}
}

func TestScanFilesTotalCounter(t *testing.T) {
	before := testutil.ToFloat64(ScanFilesTotal)
	ScanFilesTotal.Add(3)
	after := testutil.ToFloat64(ScanFilesTotal)
	if after != before+3 {
		t.Errorf("ScanFilesTotal: got delta %v, want 3", after-before)
	}
}

func TestScrobblesCounterVecLabelsAreIndependent(t *testing.T) {
	beforeOK := testutil.ToFloat64(Scrobbles.WithLabelValues("lastfm", "ok"))
	beforeErr := testutil.ToFloat64(Scrobbles.WithLabelValues("lastfm", "error"))

	Scrobbles.WithLabelValues("lastfm", "ok").Inc()

	afterOK := testutil.ToFloat64(Scrobbles.WithLabelValues("lastfm", "ok"))
	afterErr := testutil.ToFloat64(Scrobbles.WithLabelValues("lastfm", "error"))

	if afterOK != beforeOK+1 {
		t.Errorf("Scrobbles{ok}: got delta %v, want 1", afterOK-beforeOK)
	}
	if afterErr != beforeErr {
		t.Errorf("Scrobbles{error}: got %v, want unchanged %v", afterErr, beforeErr)
	}
}

func TestAuthAttemptsCounterVec(t *testing.T) {
	before := testutil.ToFloat64(AuthAttempts.WithLabelValues("basic", "success"))
	AuthAttempts.WithLabelValues("basic", "success").Inc()
	after := testutil.ToFloat64(AuthAttempts.WithLabelValues("basic", "success"))
	if after != before+1 {
		t.Errorf("AuthAttempts: got delta %v, want 1", after-before)
	}
}

func TestIcecastMountsActiveGauge(t *testing.T) {
	IcecastMountsActive.Set(2)
	if got := testutil.ToFloat64(IcecastMountsActive); got != 2 {
		t.Errorf("IcecastMountsActive.Set(2): got %v, want 2", got)
	}
	IcecastMountsActive.Dec()
	if got := testutil.ToFloat64(IcecastMountsActive); got != 1 {
		t.Errorf("IcecastMountsActive.Dec(): got %v, want 1", got)
	}
}

func TestSchedulerRunsCounterVec(t *testing.T) {
	before := testutil.ToFloat64(SchedulerRuns.WithLabelValues("backup", "success"))
	SchedulerRuns.WithLabelValues("backup", "success").Inc()
	after := testutil.ToFloat64(SchedulerRuns.WithLabelValues("backup", "success"))
	if after != before+1 {
		t.Errorf("SchedulerRuns: got delta %v, want 1", after-before)
	}
}
