package api

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	cerr "github.com/local/cassonic/src/common/errors"
)

// Build info variables set at compile time via -ldflags.
var (
	Version   = "dev"
	CommitID  = "unknown"
	BuildDate = "unknown"
)

// Health returns a public health check response.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	serverDBStatus := "ok"
	usersDBStatus := "ok"

	if _, err := h.db.Music.ListLibraries(r.Context()); err != nil {
		serverDBStatus = "error"
	}
	if _, err := h.db.Users.ListUsers(r.Context()); err != nil {
		usersDBStatus = "error"
	}

	overallStatus := "ok"
	if serverDBStatus != "ok" || usersDBStatus != "ok" {
		overallStatus = "degraded"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  overallStatus,
		"version": Version,
		"db": map[string]any{
			"server": serverDBStatus,
			"users":  usersDBStatus,
		},
	})
}

// Version returns the public version information.
func (h *Handler) Version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version": Version,
		"commit":  CommitID,
		"built":   BuildDate,
	})
}

// Metrics delegates to the Prometheus handler; admin only.
func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	if r == nil {
		writeError(w, r, cerr.InternalServerError("invalid request"))
		return
	}

	promhttp.Handler().ServeHTTP(w, r)
}
