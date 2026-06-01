package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	cerr "github.com/local/cassonic/src/common/errors"
	mw "github.com/local/cassonic/src/server/middleware"
)

// Build info variables set at compile time via -ldflags.
var (
	Version   = "dev"
	CommitID  = "unknown"
	BuildDate = "unknown"
)

// Health returns a public health check response with content negotiation.
// Accepts: application/json (default), text/plain, text/html.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	serverDBStatus := "ok"
	usersDBStatus := "ok"

	if _, err := h.db.Music.ListLibraries(r.Context()); err != nil {
		serverDBStatus = "error"
	}
	if _, err := h.db.Users.ListUsers(r.Context()); err != nil {
		usersDBStatus = "error"
	}

	overallStatus := "healthy"
	if serverDBStatus != "ok" || usersDBStatus != "ok" {
		overallStatus = "degraded"
	}

	switch mw.AcceptedFormat(r) {
	case "plain":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "status: %s\nversion: %s\ndb.server: %s\ndb.users: %s\n",
			overallStatus, Version, serverDBStatus, usersDBStatus)
	case "html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w,
			"<html><body><p>Status: %s</p><p>Version: %s</p><p>DB server: %s</p><p>DB users: %s</p></body></html>",
			overallStatus, Version, serverDBStatus, usersDBStatus)
	default:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":     true,
			"status": overallStatus,
			"version": Version,
			"db": map[string]any{
				"server": serverDBStatus,
				"users":  usersDBStatus,
			},
		})
	}
}

// Version returns the public version information with content negotiation.
// Accepts: application/json (default), text/plain, text/html.
func (h *Handler) Version(w http.ResponseWriter, r *http.Request) {
	switch mw.AcceptedFormat(r) {
	case "plain":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "version: %s\ncommit: %s\nbuilt: %s\n", Version, CommitID, BuildDate)
	case "html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w,
			"<html><body><p>Version: %s</p><p>Commit: %s</p><p>Built: %s</p></body></html>",
			Version, CommitID, BuildDate)
	default:
		writeJSON(w, http.StatusOK, map[string]any{
			"version": Version,
			"commit":  CommitID,
			"built":   BuildDate,
		})
	}
}

// Metrics delegates to the Prometheus handler; admin only.
func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	if r == nil {
		writeError(w, r, cerr.InternalServerError("invalid request"))
		return
	}

	promhttp.Handler().ServeHTTP(w, r)
}
