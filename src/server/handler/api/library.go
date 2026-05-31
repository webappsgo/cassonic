package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/service"
	cerr "github.com/local/cassonic/src/common/errors"
)

// ListLibraries returns all configured music libraries.
func (h *Handler) ListLibraries(w http.ResponseWriter, r *http.Request) {
	libs, err := h.db.Music.ListLibraries(r.Context())
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list libraries failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"libraries": libs,
		"total":     len(libs),
	})
}

// createLibraryRequest is the body for POST /api/v1/libraries.
type createLibraryRequest struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// CreateLibrary adds a new music library root; admin only.
func (h *Handler) CreateLibrary(w http.ResponseWriter, r *http.Request) {
	var req createLibraryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}
	if req.Name == "" || req.Path == "" {
		writeError(w, r, cerr.BadRequest("name and path are required"))
		return
	}

	if _, err := os.Stat(req.Path); err != nil {
		writeError(w, r, cerr.BadRequest("path does not exist or is not accessible"))
		return
	}

	lib := &model.Library{
		Name:    req.Name,
		Path:    req.Path,
		Enabled: true,
	}

	id, err := h.db.Music.CreateLibrary(r.Context(), lib)
	if err != nil {
		writeError(w, r, cerr.Conflict("library creation failed: "+err.Error()))
		return
	}
	lib.ID = id

	writeJSON(w, http.StatusCreated, lib)
}

// GetLibrary returns a single library by ID.
func (h *Handler) GetLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid library id"))
		return
	}

	lib, err := h.db.Music.GetLibrary(r.Context(), id)
	if err != nil || lib == nil {
		writeError(w, r, cerr.NotFound("library not found"))
		return
	}

	writeJSON(w, http.StatusOK, lib)
}

// updateLibraryRequest is the body for PUT /api/v1/libraries/{id}.
type updateLibraryRequest struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Enabled *bool  `json:"enabled"`
}

// UpdateLibrary updates a library's name, path, or enabled flag; admin only.
func (h *Handler) UpdateLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid library id"))
		return
	}

	lib, err := h.db.Music.GetLibrary(r.Context(), id)
	if err != nil || lib == nil {
		writeError(w, r, cerr.NotFound("library not found"))
		return
	}

	var req updateLibraryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}

	if req.Name != "" {
		lib.Name = req.Name
	}
	if req.Path != "" {
		if _, err := os.Stat(req.Path); err != nil {
			writeError(w, r, cerr.BadRequest("path does not exist or is not accessible"))
			return
		}
		lib.Path = req.Path
	}
	if req.Enabled != nil {
		lib.Enabled = *req.Enabled
	}

	if err := h.db.Music.UpdateLibrary(r.Context(), lib); err != nil {
		writeError(w, r, cerr.InternalServerError("update failed"))
		return
	}

	writeJSON(w, http.StatusOK, lib)
}

// DeleteLibrary removes a library and all its scanned songs; admin only.
func (h *Handler) DeleteLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid library id"))
		return
	}

	if err := h.db.Music.DeleteLibrary(r.Context(), id); err != nil {
		writeError(w, r, cerr.InternalServerError("delete failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// ScanLibrary triggers an asynchronous library scan; admin only.
func (h *Handler) ScanLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid library id"))
		return
	}

	lib, err := h.db.Music.GetLibrary(r.Context(), id)
	if err != nil || lib == nil {
		writeError(w, r, cerr.NotFound("library not found"))
		return
	}

	scanStatus := &model.ScanStatus{
		Status: "running",
	}
	scanID, err := h.db.Music.CreateScanStatus(r.Context(), scanStatus)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("scan initiation failed"))
		return
	}

	go func() {
		if h.scanner != nil {
			_ = h.scanner.Scan(context.Background(), service.ScanModeIncremental)
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]any{
		"scan_id": scanID,
	})
}

