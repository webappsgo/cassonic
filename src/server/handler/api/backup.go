// Package api — backup endpoints.
// POST /api/v1/admin/backup             trigger backup (admin only)
// GET  /api/v1/admin/backups            list backups (admin only)
// GET  /api/v1/admin/backups/{filename} download a backup file (admin only)
// POST /api/v1/admin/restore            restore from backup (admin only)
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	cerr "github.com/local/cassonic/src/common/errors"
	"github.com/local/cassonic/src/server/service/backup"
)

// BackupService is the interface the backup handler depends on.
type BackupService interface {
	Backup(ctx context.Context) (string, error)
	Restore(ctx context.Context, path string) error
	List() ([]backup.BackupInfo, error)
}

// backupResponse is the JSON shape returned by a successful Backup call.
type backupResponse struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// TriggerBackup handles POST /api/v1/admin/backup.
// Runs a backup and returns the path and size of the new archive.
func (h *Handler) TriggerBackup(w http.ResponseWriter, r *http.Request) {
	if h.backupSvc == nil {
		writeError(w, r, cerr.ServiceUnavailable("backup service not configured"))
		return
	}

	path, err := h.backupSvc.Backup(r.Context())
	if err != nil {
		writeError(w, r, cerr.InternalServerError("backup failed: "+err.Error()))
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("backup stat: "+err.Error()))
		return
	}

	writeJSON(w, http.StatusCreated, backupResponse{
		Path: path,
		Size: info.Size(),
	})
}

// ListBackups handles GET /api/v1/admin/backups.
func (h *Handler) ListBackups(w http.ResponseWriter, r *http.Request) {
	if h.backupSvc == nil {
		writeError(w, r, cerr.ServiceUnavailable("backup service not configured"))
		return
	}

	infos, err := h.backupSvc.List()
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list backups: "+err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, infos)
}

// DownloadBackup handles GET /api/v1/admin/backups/{filename}.
// Streams the backup file to the client.
func (h *Handler) DownloadBackup(w http.ResponseWriter, r *http.Request) {
	if h.backupSvc == nil {
		writeError(w, r, cerr.ServiceUnavailable("backup service not configured"))
		return
	}

	filename := chi.URLParam(r, "filename")
	if filename == "" || strings.ContainsAny(filename, "/\\") {
		writeError(w, r, cerr.BadRequest("invalid filename"))
		return
	}

	infos, err := h.backupSvc.List()
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list backups: "+err.Error()))
		return
	}

	// Verify the file is a known backup (prevents directory traversal).
	var matchedPath string
	for _, info := range infos {
		if filepath.Base(info.Path) == filename {
			matchedPath = info.Path
			break
		}
	}
	if matchedPath == "" {
		writeError(w, r, cerr.NotFound("backup file not found"))
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, matchedPath)
}

// RestoreBackup handles POST /api/v1/admin/restore.
// Request body: {"path": "cassonic-backup-20240101-120000.tar.gz"}
func (h *Handler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	if h.backupSvc == nil {
		writeError(w, r, cerr.ServiceUnavailable("backup service not configured"))
		return
	}

	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, cerr.BadRequest("invalid request body"))
		return
	}
	if body.Path == "" {
		writeError(w, r, cerr.BadRequest("path is required"))
		return
	}

	// Validate the path is a listed backup.
	infos, err := h.backupSvc.List()
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list backups: "+err.Error()))
		return
	}

	var matchedPath string
	for _, info := range infos {
		if info.Path == body.Path || filepath.Base(info.Path) == body.Path {
			matchedPath = info.Path
			break
		}
	}
	if matchedPath == "" {
		writeError(w, r, cerr.NotFound("backup file not found"))
		return
	}

	if err := h.backupSvc.Restore(r.Context(), matchedPath); err != nil {
		writeError(w, r, cerr.InternalServerError("restore failed: "+err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}
