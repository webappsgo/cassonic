package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/service"
	"github.com/local/cassonic/src/server/service/ffmpeg"
	"github.com/local/cassonic/src/server/service/tags"
	"github.com/local/cassonic/src/server/store"
	cerr "github.com/local/cassonic/src/common/errors"
)

// Handler holds all native API dependencies.
type Handler struct {
	db         *store.DB
	scanner    *service.Scanner
	coverArt   *service.CoverArtService
	ffmpeg     *ffmpeg.Manager
	tagReader  *tags.Reader
	nowPlaying *NowPlayingTracker
	backupSvc  BackupService
}

// NowPlayingInfo holds metadata for one active native API stream.
type NowPlayingInfo struct {
	UserID     int64
	Username   string
	Song       *model.Song
	StartedAt  time.Time
	PlayerName string
}

// NowPlayingTracker tracks active streams.
type NowPlayingTracker struct {
	mu      sync.RWMutex
	streams map[int64]*NowPlayingInfo
}

// NewNowPlayingTracker creates an empty NowPlayingTracker.
func NewNowPlayingTracker() *NowPlayingTracker {
	return &NowPlayingTracker{
		streams: make(map[int64]*NowPlayingInfo),
	}
}

// Register records or replaces the active stream for the given user.
func (t *NowPlayingTracker) Register(userID int64, info *NowPlayingInfo) {
	t.mu.Lock()
	t.streams[userID] = info
	t.mu.Unlock()
}

// Unregister removes the active stream entry for the given user.
func (t *NowPlayingTracker) Unregister(userID int64) {
	t.mu.Lock()
	delete(t.streams, userID)
	t.mu.Unlock()
}

// All returns a snapshot of all active stream entries.
func (t *NowPlayingTracker) All() []*NowPlayingInfo {
	t.mu.RLock()
	result := make([]*NowPlayingInfo, 0, len(t.streams))
	for _, info := range t.streams {
		cp := *info
		result = append(result, &cp)
	}
	t.mu.RUnlock()
	return result
}

// ForUser returns the active stream entry for a specific user, or nil if none.
func (t *NowPlayingTracker) ForUser(userID int64) *NowPlayingInfo {
	t.mu.RLock()
	info := t.streams[userID]
	t.mu.RUnlock()
	if info == nil {
		return nil
	}
	cp := *info
	return &cp
}

// NewHandler creates a Handler with all required dependencies.
func NewHandler(
	db *store.DB,
	scanner *service.Scanner,
	coverArt *service.CoverArtService,
	ff *ffmpeg.Manager,
	tagReader *tags.Reader,
) *Handler {
	return &Handler{
		db:         db,
		scanner:    scanner,
		coverArt:   coverArt,
		ffmpeg:     ff,
		tagReader:  tagReader,
		nowPlaying: NewNowPlayingTracker(),
	}
}

// WithBackupService attaches a backup service to the handler.
func (h *Handler) WithBackupService(svc BackupService) *Handler {
	h.backupSvc = svc
	return h
}

// Routes builds and returns the chi router for all /api/v1 endpoints.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(mw.Cors())

	r.Post("/api/v1/auth/login", h.Login)
	r.With(mw.RequireAuth()).Post("/api/v1/auth/logout", h.Logout)
	r.With(mw.RequireAuth()).Post("/api/v1/auth/tokens", h.CreateToken)
	r.With(mw.RequireAuth()).Delete("/api/v1/auth/tokens/{id}", h.DeleteToken)

	r.With(mw.RequireAdmin()).Get("/api/v1/users", h.ListUsers)
	r.With(mw.RequireAdmin()).Post("/api/v1/users", h.CreateUser)
	r.With(mw.RequireAuth()).Get("/api/v1/users/me", h.GetMe)
	r.With(mw.RequireAuth()).Put("/api/v1/users/me", h.UpdateMe)
	r.With(mw.RequireAuth()).Get("/api/v1/users/{id}", h.GetUser)
	r.With(mw.RequireAdmin()).Put("/api/v1/users/{id}", h.UpdateUser)
	r.With(mw.RequireAdmin()).Delete("/api/v1/users/{id}", h.DeleteUser)
	r.With(mw.RequireAuth()).Post("/api/v1/users/{id}/password", h.ChangePassword)

	r.With(mw.RequireAuth()).Get("/api/v1/libraries", h.ListLibraries)
	r.With(mw.RequireAdmin()).Post("/api/v1/libraries", h.CreateLibrary)
	r.With(mw.RequireAuth()).Get("/api/v1/libraries/{id}", h.GetLibrary)
	r.With(mw.RequireAdmin()).Put("/api/v1/libraries/{id}", h.UpdateLibrary)
	r.With(mw.RequireAdmin()).Delete("/api/v1/libraries/{id}", h.DeleteLibrary)
	r.With(mw.RequireAdmin()).Post("/api/v1/libraries/{id}/scan", h.ScanLibrary)

	r.With(mw.RequireAuth()).Get("/api/v1/artists", h.ListArtists)
	r.With(mw.RequireAuth()).Get("/api/v1/artists/{id}", h.GetArtist)
	r.With(mw.RequireAuth()).Get("/api/v1/albums", h.ListAlbums)
	r.With(mw.RequireAuth()).Get("/api/v1/albums/{id}", h.GetAlbum)
	r.With(mw.RequireAuth()).Get("/api/v1/songs", h.ListSongs)
	r.With(mw.RequireAuth()).Get("/api/v1/songs/{id}", h.GetSong)
	r.With(mw.RequireAuth()).Get("/api/v1/genres", h.ListGenres)
	r.With(mw.RequireAuth()).Get("/api/v1/search", h.Search)

	r.With(mw.RequireAuth()).Get("/api/v1/songs/{id}/stream", h.Stream)
	r.With(mw.RequireAuth()).Get("/api/v1/songs/{id}/download", h.Download)
	r.With(mw.RequireAuth()).Get("/api/v1/songs/{id}/tags", h.GetTags)
	r.With(mw.RequireAuth()).Patch("/api/v1/songs/{id}/tags", h.PatchTags)
	r.With(mw.RequireAuth()).Get("/api/v1/songs/{id}/tags/writable", h.TagsWritable)
	r.With(mw.RequireAuth()).Get("/api/v1/songs/{id}/cover-art", h.GetSongCoverArt)
	r.With(mw.RequireAuth()).Post("/api/v1/songs/{id}/cover-art", h.UploadSongCoverArt)
	r.With(mw.RequireAuth()).Delete("/api/v1/songs/{id}/cover-art", h.DeleteSongCoverArt)
	r.With(mw.RequireAuth()).Post("/api/v1/songs/{id}/stars", h.StarSong)
	r.With(mw.RequireAuth()).Delete("/api/v1/songs/{id}/stars", h.UnstarSong)
	r.With(mw.RequireAuth()).Patch("/api/v1/songs/{id}/rating", h.RateSong)
	r.With(mw.RequireAuth()).Post("/api/v1/songs/{id}/scrobbles", h.Scrobble)

	r.With(mw.RequireAuth()).Get("/api/v1/playlists", h.ListPlaylists)
	r.With(mw.RequireAuth()).Post("/api/v1/playlists", h.CreatePlaylist)
	r.With(mw.RequireAuth()).Get("/api/v1/playlists/{id}", h.GetPlaylist)
	r.With(mw.RequireAuth()).Put("/api/v1/playlists/{id}", h.UpdatePlaylist)
	r.With(mw.RequireAuth()).Delete("/api/v1/playlists/{id}", h.DeletePlaylist)
	r.With(mw.RequireAuth()).Get("/api/v1/playlists/{id}/songs", h.GetPlaylistSongs)
	r.With(mw.RequireAuth()).Post("/api/v1/playlists/{id}/songs", h.AddPlaylistSongs)
	r.With(mw.RequireAuth()).Delete("/api/v1/playlists/{id}/songs/{songId}", h.RemovePlaylistSong)

	r.With(mw.RequireAuth()).Get("/api/v1/play-queues", h.GetPlayQueue)
	r.With(mw.RequireAuth()).Put("/api/v1/play-queues", h.SavePlayQueue)
	r.With(mw.RequireAuth()).Get("/api/v1/bookmarks", h.ListBookmarks)
	r.With(mw.RequireAuth()).Post("/api/v1/bookmarks", h.CreateBookmark)
	r.With(mw.RequireAuth()).Put("/api/v1/bookmarks/{id}", h.UpdateBookmark)
	r.With(mw.RequireAuth()).Delete("/api/v1/bookmarks/{id}", h.DeleteBookmark)

	r.With(mw.RequireAuth()).Get("/api/v1/now-playing", h.NowPlaying)
	r.With(mw.RequireAuth()).Get("/api/v1/events/now-playing", h.NowPlayingSSE)

	r.With(mw.RequireAuth()).Get("/api/v1/shares", h.ListShares)
	r.With(mw.RequireAuth()).Post("/api/v1/shares", h.CreateShare)
	r.Get("/api/v1/shares/{token}", h.GetShare)
	r.With(mw.RequireAuth()).Put("/api/v1/shares/{id}", h.UpdateShare)
	r.With(mw.RequireAuth()).Delete("/api/v1/shares/{id}", h.DeleteShare)

	r.With(mw.RequireAuth()).Get("/api/v1/podcasts", h.ListPodcasts)
	r.With(mw.RequireAdmin()).Post("/api/v1/podcasts", h.CreatePodcast)
	r.With(mw.RequireAuth()).Get("/api/v1/podcasts/{id}", h.GetPodcast)
	r.With(mw.RequireAdmin()).Put("/api/v1/podcasts/{id}", h.UpdatePodcast)
	r.With(mw.RequireAdmin()).Delete("/api/v1/podcasts/{id}", h.DeletePodcast)
	r.With(mw.RequireAuth()).Get("/api/v1/podcasts/{id}/episodes", h.ListPodcastEpisodes)
	r.With(mw.RequireAuth()).Get("/api/v1/podcasts/episodes/{id}", h.GetPodcastEpisode)
	r.With(mw.RequireAdmin()).Post("/api/v1/podcasts/episodes/{id}/download", h.DownloadPodcastEpisode)
	r.With(mw.RequireAdmin()).Delete("/api/v1/podcasts/episodes/{id}", h.DeletePodcastEpisode)

	r.With(mw.RequireAuth()).Get("/api/v1/icecast/servers", h.ListIcecastServers)
	r.With(mw.RequireAdmin()).Post("/api/v1/icecast/servers", h.CreateIcecastServer)
	r.With(mw.RequireAuth()).Get("/api/v1/icecast/servers/{id}", h.GetIcecastServer)
	r.With(mw.RequireAdmin()).Put("/api/v1/icecast/servers/{id}", h.UpdateIcecastServer)
	r.With(mw.RequireAdmin()).Delete("/api/v1/icecast/servers/{id}", h.DeleteIcecastServer)
	r.With(mw.RequireAuth()).Get("/api/v1/icecast/mounts", h.ListIcecastMounts)
	r.With(mw.RequireAdmin()).Post("/api/v1/icecast/mounts", h.CreateIcecastMount)
	r.With(mw.RequireAuth()).Get("/api/v1/icecast/mounts/{id}", h.GetIcecastMount)
	r.With(mw.RequireAdmin()).Put("/api/v1/icecast/mounts/{id}", h.UpdateIcecastMount)
	r.With(mw.RequireAdmin()).Delete("/api/v1/icecast/mounts/{id}", h.DeleteIcecastMount)
	r.With(mw.RequireAdmin()).Post("/api/v1/icecast/mounts/{id}/start", h.StartIcecastMount)
	r.With(mw.RequireAdmin()).Post("/api/v1/icecast/mounts/{id}/stop", h.StopIcecastMount)
	r.With(mw.RequireAuth()).Get("/api/v1/icecast/mounts/{id}/status", h.IcecastMountStatus)

	r.With(mw.RequireAdmin()).Get("/api/v1/metrics", h.Metrics)
	r.Get("/api/v1/health", h.Health)
	r.Get("/api/v1/version", h.Version)

	r.With(mw.RequireAdmin()).Post("/api/v1/admin/backup", h.TriggerBackup)
	r.With(mw.RequireAdmin()).Get("/api/v1/admin/backups", h.ListBackups)
	r.With(mw.RequireAdmin()).Get("/api/v1/admin/backups/{filename}", h.DownloadBackup)
	r.With(mw.RequireAdmin()).Post("/api/v1/admin/restore", h.RestoreBackup)

	return r
}

// writeJSON writes a success envelope {"ok":true,"data":{...}} with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(map[string]any{
		"ok":   true,
		"data": data,
	})
}

// writeError writes an RFC 7807 problem detail response.
func writeError(w http.ResponseWriter, r *http.Request, prob *cerr.Problem) {
	prob.Instance = r.URL.Path
	prob.WriteJSON(w)
}

// parsePagination extracts limit and offset from query parameters.
// Default limit is 50; maximum is 500.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 500 {
		limit = 500
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

// parseID parses a chi URL parameter as int64, returning an error on failure.
func parseID(r *http.Request, key string) (int64, error) {
	raw := chi.URLParam(r, key)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, cerr.BadRequest("invalid id: " + raw)
	}
	return id, nil
}
