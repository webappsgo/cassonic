package api

import (
	"encoding/json"
	"net/http"

	"github.com/local/cassonic/src/server/model"
	cerr "github.com/local/cassonic/src/common/errors"
)

// maskPassword replaces the value of a sensitive password field with "*****".
const maskedPassword = "*****"

// safeIcecastServer returns the server data with the source password masked.
func safeIcecastServer(s *model.IcecastServer) map[string]any {
	return map[string]any{
		"id":          s.ID,
		"name":        s.Name,
		"host":        s.Host,
		"port":        s.Port,
		"protocol":    s.Protocol,
		"source_user": s.SourceUser,
		"source_pass": maskedPassword,
		"enabled":     s.Enabled,
		"created_at":  s.CreatedAt,
		"updated_at":  s.UpdatedAt,
	}
}

// ListIcecastServers returns all Icecast server configurations.
func (h *Handler) ListIcecastServers(w http.ResponseWriter, r *http.Request) {
	servers, err := h.db.Icecast.ListServers(r.Context())
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list servers failed"))
		return
	}

	result := make([]map[string]any, 0, len(servers))
	for _, s := range servers {
		result = append(result, safeIcecastServer(s))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"servers": result,
		"total":   len(result),
	})
}

// createIcecastServerRequest is the body for POST /api/v1/icecast/servers.
type createIcecastServerRequest struct {
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Protocol   string `json:"protocol"`
	SourceUser string `json:"source_user"`
	SourcePass string `json:"source_pass"`
	Enabled    bool   `json:"enabled"`
}

// CreateIcecastServer adds a new Icecast server configuration; admin only.
func (h *Handler) CreateIcecastServer(w http.ResponseWriter, r *http.Request) {
	var req createIcecastServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}
	if req.Name == "" || req.Host == "" {
		writeError(w, r, cerr.BadRequest("name and host are required"))
		return
	}
	if req.Port == 0 {
		req.Port = 8000
	}
	if req.Protocol == "" {
		req.Protocol = "http"
	}
	if req.SourceUser == "" {
		req.SourceUser = "source"
	}

	s := &model.IcecastServer{
		Name:       req.Name,
		Host:       req.Host,
		Port:       req.Port,
		Protocol:   req.Protocol,
		SourceUser: req.SourceUser,
		SourcePass: req.SourcePass,
		Enabled:    req.Enabled,
	}

	id, err := h.db.Icecast.CreateServer(r.Context(), s)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("create server failed"))
		return
	}
	s.ID = id

	writeJSON(w, http.StatusCreated, safeIcecastServer(s))
}

// GetIcecastServer returns a single Icecast server configuration.
func (h *Handler) GetIcecastServer(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid server id"))
		return
	}

	s, err := h.db.Icecast.GetServer(r.Context(), id)
	if err != nil || s == nil {
		writeError(w, r, cerr.NotFound("server not found"))
		return
	}

	writeJSON(w, http.StatusOK, safeIcecastServer(s))
}

// updateIcecastServerRequest is the body for PUT /api/v1/icecast/servers/{id}.
type updateIcecastServerRequest struct {
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       *int   `json:"port"`
	Protocol   string `json:"protocol"`
	SourceUser string `json:"source_user"`
	SourcePass string `json:"source_pass"`
	Enabled    *bool  `json:"enabled"`
}

// UpdateIcecastServer updates an Icecast server configuration; admin only.
func (h *Handler) UpdateIcecastServer(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid server id"))
		return
	}

	s, err := h.db.Icecast.GetServer(r.Context(), id)
	if err != nil || s == nil {
		writeError(w, r, cerr.NotFound("server not found"))
		return
	}

	var req updateIcecastServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}

	if req.Name != "" {
		s.Name = req.Name
	}
	if req.Host != "" {
		s.Host = req.Host
	}
	if req.Port != nil {
		s.Port = *req.Port
	}
	if req.Protocol != "" {
		s.Protocol = req.Protocol
	}
	if req.SourceUser != "" {
		s.SourceUser = req.SourceUser
	}
	if req.SourcePass != "" {
		s.SourcePass = req.SourcePass
	}
	if req.Enabled != nil {
		s.Enabled = *req.Enabled
	}

	if err := h.db.Icecast.UpdateServer(r.Context(), s); err != nil {
		writeError(w, r, cerr.InternalServerError("update server failed"))
		return
	}

	writeJSON(w, http.StatusOK, safeIcecastServer(s))
}

// DeleteIcecastServer removes an Icecast server and its mount points; admin only.
func (h *Handler) DeleteIcecastServer(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid server id"))
		return
	}

	if err := h.db.Icecast.DeleteServer(r.Context(), id); err != nil {
		writeError(w, r, cerr.InternalServerError("delete server failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// ListIcecastMounts returns all Icecast mount point configurations.
func (h *Handler) ListIcecastMounts(w http.ResponseWriter, r *http.Request) {
	mounts, err := h.db.Icecast.ListMounts(r.Context())
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list mounts failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"mounts": mounts,
		"total":  len(mounts),
	})
}

// createIcecastMountRequest is the body for POST /api/v1/icecast/mounts.
type createIcecastMountRequest struct {
	ServerID    int64              `json:"server_id"`
	MountPath   string             `json:"mount_path"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Scope       model.StreamScope  `json:"scope"`
	ArtistID    int64              `json:"artist_id"`
	Genre       string             `json:"genre"`
	Format      model.StreamFormat `json:"format"`
	BitRate     int                `json:"bit_rate"`
	Shuffle     bool               `json:"shuffle"`
	Enabled     bool               `json:"enabled"`
}

// CreateIcecastMount adds a new Icecast mount point; admin only.
func (h *Handler) CreateIcecastMount(w http.ResponseWriter, r *http.Request) {
	var req createIcecastMountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}
	if req.ServerID <= 0 || req.MountPath == "" {
		writeError(w, r, cerr.BadRequest("server_id and mount_path are required"))
		return
	}
	if req.Format == "" {
		req.Format = model.FormatMP3
	}
	if req.BitRate == 0 {
		req.BitRate = 128
	}
	if req.Scope == "" {
		req.Scope = model.ScopeAll
	}

	m := &model.IcecastMount{
		ServerID:    req.ServerID,
		MountPath:   req.MountPath,
		Name:        req.Name,
		Description: req.Description,
		Scope:       req.Scope,
		ArtistID:    req.ArtistID,
		Genre:       req.Genre,
		Format:      req.Format,
		BitRate:     req.BitRate,
		Shuffle:     req.Shuffle,
		Enabled:     req.Enabled,
		Status:      model.StatusDisconnected,
	}

	id, err := h.db.Icecast.CreateMount(r.Context(), m)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("create mount failed"))
		return
	}
	m.ID = id

	writeJSON(w, http.StatusCreated, m)
}

// GetIcecastMount returns a single Icecast mount point.
func (h *Handler) GetIcecastMount(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid mount id"))
		return
	}

	m, err := h.db.Icecast.GetMount(r.Context(), id)
	if err != nil || m == nil {
		writeError(w, r, cerr.NotFound("mount not found"))
		return
	}

	writeJSON(w, http.StatusOK, m)
}

// updateIcecastMountRequest is the body for PUT /api/v1/icecast/mounts/{id}.
type updateIcecastMountRequest struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Scope       model.StreamScope   `json:"scope"`
	ArtistID    *int64              `json:"artist_id"`
	Genre       string              `json:"genre"`
	Format      model.StreamFormat  `json:"format"`
	BitRate     *int                `json:"bit_rate"`
	Shuffle     *bool               `json:"shuffle"`
	Enabled     *bool               `json:"enabled"`
}

// UpdateIcecastMount updates an Icecast mount point configuration; admin only.
func (h *Handler) UpdateIcecastMount(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid mount id"))
		return
	}

	m, err := h.db.Icecast.GetMount(r.Context(), id)
	if err != nil || m == nil {
		writeError(w, r, cerr.NotFound("mount not found"))
		return
	}

	var req updateIcecastMountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}

	if req.Name != "" {
		m.Name = req.Name
	}
	if req.Description != "" {
		m.Description = req.Description
	}
	if req.Scope != "" {
		m.Scope = req.Scope
	}
	if req.ArtistID != nil {
		m.ArtistID = *req.ArtistID
	}
	if req.Genre != "" {
		m.Genre = req.Genre
	}
	if req.Format != "" {
		m.Format = req.Format
	}
	if req.BitRate != nil {
		m.BitRate = *req.BitRate
	}
	if req.Shuffle != nil {
		m.Shuffle = *req.Shuffle
	}
	if req.Enabled != nil {
		m.Enabled = *req.Enabled
	}

	if err := h.db.Icecast.UpdateMount(r.Context(), m); err != nil {
		writeError(w, r, cerr.InternalServerError("update mount failed"))
		return
	}

	writeJSON(w, http.StatusOK, m)
}

// DeleteIcecastMount removes an Icecast mount point; admin only.
func (h *Handler) DeleteIcecastMount(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid mount id"))
		return
	}

	if err := h.db.Icecast.DeleteMount(r.Context(), id); err != nil {
		writeError(w, r, cerr.InternalServerError("delete mount failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// StartIcecastMount enables a mount point and triggers streaming; admin only.
func (h *Handler) StartIcecastMount(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid mount id"))
		return
	}

	m, err := h.db.Icecast.GetMount(r.Context(), id)
	if err != nil || m == nil {
		writeError(w, r, cerr.NotFound("mount not found"))
		return
	}

	m.Enabled = true
	if err := h.db.Icecast.UpdateMount(r.Context(), m); err != nil {
		writeError(w, r, cerr.InternalServerError("enable mount failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "started"})
}

// StopIcecastMount disables a mount point and stops streaming; admin only.
func (h *Handler) StopIcecastMount(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid mount id"))
		return
	}

	m, err := h.db.Icecast.GetMount(r.Context(), id)
	if err != nil || m == nil {
		writeError(w, r, cerr.NotFound("mount not found"))
		return
	}

	m.Enabled = false
	if err := h.db.Icecast.UpdateMount(r.Context(), m); err != nil {
		writeError(w, r, cerr.InternalServerError("disable mount failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "stopped"})
}

// IcecastMountStatus returns the current runtime status of a mount point.
func (h *Handler) IcecastMountStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid mount id"))
		return
	}

	m, err := h.db.Icecast.GetMount(r.Context(), id)
	if err != nil || m == nil {
		writeError(w, r, cerr.NotFound("mount not found"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":           m.ID,
		"status":       m.Status,
		"current_song": m.CurrentSong,
		"last_error":   m.LastError,
		"enabled":      m.Enabled,
	})
}
