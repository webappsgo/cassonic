package ampache

import (
	"net/http"
	"strconv"
	"time"

	"github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/store"
)

// handshake authenticates a user via the Ampache passphrase protocol and creates
// a new in-memory session. It returns a HandshakeResp with a session token and
// library counts on success, or an AmpError on failure.
func (h *Handler) handshake(w http.ResponseWriter, r *http.Request, isJSON bool) {
	username := param(r, "user")
	timestamp := param(r, "timestamp")
	auth := param(r, "auth")

	if username == "" || timestamp == "" || auth == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: user, timestamp, and auth are required"))
		return
	}

	ctx := r.Context()
	user, err := middleware.VerifyHandshake(ctx, h.db.Users, h.getPlainPassword, username, auth, timestamp)
	if err != nil {
		respond(w, r, isJSON, errResp(4700, "Access denied: "+err.Error()))
		return
	}

	token := h.sessions.Create(user.ID)
	expire := time.Now().Add(sessionTTL).Format(time.RFC3339)
	now := time.Now().Format(time.RFC3339)

	songCount, albumCount, artistCount, playlistCount := h.libraryCounts(r)

	resp := HandshakeResp{
		Auth:            token,
		API:             ampacheAPIVersion,
		SessionExpire:   expire,
		Update:          now,
		Add:             now,
		Clean:           now,
		Songs:           songCount,
		Artists:         artistCount,
		Albums:          albumCount,
		Playlists:       playlistCount,
		Videos:          0,
		Catalogs:        0,
		Podcasts:        0,
		PodcastEpisodes: 0,
	}

	respond(w, r, isJSON, resp)
}

// goodbye invalidates the session token supplied in the auth parameter.
func (h *Handler) goodbye(w http.ResponseWriter, r *http.Request, isJSON bool) {
	token := param(r, "auth")
	if token == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: auth"))
		return
	}

	session := h.sessions.Get(token)
	username := ""
	if session != nil {
		user, _ := h.db.Users.GetUser(r.Context(), session.UserID)
		if user != nil {
			username = user.Username
		}
	}
	h.sessions.Delete(token)

	respond(w, r, isJSON, okResp("goodbye", map[string]any{
		"username": username,
		"auth":     token,
	}))
}

// ping validates the session and extends its TTL. Returns the server version.
func (h *Handler) ping(w http.ResponseWriter, r *http.Request, isJSON bool) {
	token := param(r, "auth")
	if token == "" {
		respond(w, r, isJSON, map[string]any{
			"version": ampacheAPIVersion,
			"server":  "cassonic",
		})
		return
	}

	session := h.sessions.Get(token)
	if session == nil {
		respond(w, r, isJSON, errResp(4700, "Access denied: invalid or expired session"))
		return
	}

	h.sessions.Extend(token, sessionTTL)

	respond(w, r, isJSON, map[string]any{
		"version":        ampacheAPIVersion,
		"server":         "cassonic",
		"session_expire": time.Now().Add(sessionTTL).Format(time.RFC3339),
	})
}

// checkParameter validates a parameter value and echoes it back.
func (h *Handler) checkParameter(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	parameter := param(r, "parameter")
	input := param(r, "input")
	if parameter == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: parameter"))
		return
	}
	respond(w, r, isJSON, okResp("parameter", map[string]any{
		"name":  parameter,
		"value": input,
	}))
}

// libraryCounts returns song, album, artist, and playlist counts from the store.
// It uses large-limit list calls since the store has no dedicated Count methods.
func (h *Handler) libraryCounts(r *http.Request) (songs, albums, artists, playlists int) {
	ctx := r.Context()

	const bigLimit = 999999

	artistList, err := h.db.Music.ListArtists(ctx, store.ListOpts{Limit: bigLimit})
	if err == nil {
		artists = len(artistList)
	}

	albumList, err := h.db.Music.ListAlbums(ctx, store.ListOpts{Limit: bigLimit})
	if err == nil {
		albums = len(albumList)
	}

	genreList, err := h.db.Music.ListGenres(ctx)
	if err == nil {
		for _, g := range genreList {
			songs += g.SongCount
		}
	}

	playlistList, err := h.db.Playlists.ListPlaylists(ctx, 0)
	if err == nil {
		playlists = len(playlistList)
	}

	return songs, albums, artists, playlists
}

// parseIntParam parses an integer query parameter; returns def on parse failure.
func parseIntParam(r *http.Request, key string, def int) int {
	v := param(r, key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// parseIDParam parses an integer ID parameter; returns 0 on failure.
func parseIDParam(r *http.Request, key string) int64 {
	v := param(r, key)
	if v == "" {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
