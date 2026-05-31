package ampache

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/service"
	"github.com/local/cassonic/src/server/store"
)

// Handler holds all Ampache API dependencies.
type Handler struct {
	db               *store.DB
	sessions         *middleware.AmpacheSessionStore
	scanner          *service.Scanner
	coverArt         *service.CoverArtService
	getPlainPassword func(ctx context.Context, username string) (string, bool)
}

// NewHandler creates an Ampache API handler.
func NewHandler(
	db *store.DB,
	sessions *middleware.AmpacheSessionStore,
	scanner *service.Scanner,
	coverArt *service.CoverArtService,
	getPlainPassword func(ctx context.Context, username string) (string, bool),
) *Handler {
	return &Handler{
		db:               db,
		sessions:         sessions,
		scanner:          scanner,
		coverArt:         coverArt,
		getPlainPassword: getPlainPassword,
	}
}

// Routes returns a chi router with GET and POST handlers for both Ampache endpoints.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.AmpacheAuth(h.db.Users, h.sessions))

	r.Get("/server/xml.server.php", func(w http.ResponseWriter, r *http.Request) {
		h.dispatch(w, r, false)
	})
	r.Post("/server/xml.server.php", func(w http.ResponseWriter, r *http.Request) {
		h.dispatch(w, r, false)
	})
	r.Get("/server/json.server.php", func(w http.ResponseWriter, r *http.Request) {
		h.dispatch(w, r, true)
	})
	r.Post("/server/json.server.php", func(w http.ResponseWriter, r *http.Request) {
		h.dispatch(w, r, true)
	})

	return r
}

// dispatch routes the ?action= query parameter to the correct handler method.
// isJSON is true when the request targets the json.server.php endpoint.
func (h *Handler) dispatch(w http.ResponseWriter, r *http.Request, isJSON bool) {
	action := r.URL.Query().Get("action")
	if action == "" {
		action = r.FormValue("action")
	}
	switch action {
	// Session management
	case "handshake":
		h.handshake(w, r, isJSON)
	case "goodbye":
		h.goodbye(w, r, isJSON)
	case "ping":
		h.ping(w, r, isJSON)
	case "check_parameter":
		h.checkParameter(w, r, isJSON)

	// Browsing
	case "artists":
		h.artists(w, r, isJSON)
	case "artist":
		h.artist(w, r, isJSON)
	case "artist_albums":
		h.artistAlbums(w, r, isJSON)
	case "artist_songs":
		h.artistSongs(w, r, isJSON)
	case "albums":
		h.albums(w, r, isJSON)
	case "album":
		h.album(w, r, isJSON)
	case "album_songs":
		h.albumSongs(w, r, isJSON)
	case "songs":
		h.songs(w, r, isJSON)
	case "song":
		h.song(w, r, isJSON)
	case "genres":
		h.genres(w, r, isJSON)
	case "genre":
		h.genre(w, r, isJSON)
	case "genre_songs":
		h.genreSongs(w, r, isJSON)
	case "genre_albums":
		h.genreAlbums(w, r, isJSON)
	case "genre_artists":
		h.genreArtists(w, r, isJSON)
	case "get_indexes":
		h.getIndexes(w, r, isJSON)
	case "stats":
		h.stats(w, r, isJSON)
	case "advanced_search":
		h.advancedSearch(w, r, isJSON)
	case "system_update":
		h.systemUpdate(w, r, isJSON)
	case "catalogs":
		h.catalogs(w, r, isJSON)
	case "catalog":
		h.catalog(w, r, isJSON)
	case "catalog_songs":
		h.catalogSongs(w, r, isJSON)
	case "catalog_albums":
		h.catalogAlbums(w, r, isJSON)
	case "catalog_artists":
		h.catalogArtists(w, r, isJSON)
	case "catalog_action":
		h.catalogAction(w, r, isJSON)
	case "labels":
		h.labels(w, r, isJSON)
	case "label":
		h.label(w, r, isJSON)
	case "label_artists":
		h.labelArtists(w, r, isJSON)

	// Streaming and artwork
	case "stream":
		h.stream(w, r, isJSON)
	case "download":
		h.download(w, r, isJSON)
	case "get_art":
		h.getArt(w, r, isJSON)
	case "update_art":
		h.updateArt(w, r, isJSON)
	case "update_artist_info":
		h.updateArtistInfo(w, r, isJSON)
	case "upload":
		h.upload(w, r, isJSON)

	// Playlists and interactions
	case "playlists":
		h.playlists(w, r, isJSON)
	case "playlist":
		h.playlist(w, r, isJSON)
	case "playlist_songs":
		h.playlistSongs(w, r, isJSON)
	case "playlist_create":
		h.playlistCreate(w, r, isJSON)
	case "playlist_edit":
		h.playlistEdit(w, r, isJSON)
	case "playlist_delete":
		h.playlistDelete(w, r, isJSON)
	case "playlist_add_song":
		h.playlistAddSong(w, r, isJSON)
	case "playlist_remove_song":
		h.playlistRemoveSong(w, r, isJSON)
	case "playlist_generate":
		h.playlistGenerate(w, r, isJSON)
	case "flag":
		h.flag(w, r, isJSON)
	case "rate":
		h.rate(w, r, isJSON)
	case "record_play":
		h.recordPlay(w, r, isJSON)
	case "scrobble":
		h.scrobble(w, r, isJSON)
	case "now_playing":
		h.nowPlaying(w, r, isJSON)
	case "get_similar":
		h.getSimilar(w, r, isJSON)
	case "shares":
		h.shares(w, r, isJSON)
	case "share":
		h.share(w, r, isJSON)
	case "share_create":
		h.shareCreate(w, r, isJSON)
	case "share_edit":
		h.shareEdit(w, r, isJSON)
	case "share_delete":
		h.shareDelete(w, r, isJSON)
	case "bookmarks":
		h.bookmarks(w, r, isJSON)
	case "bookmark_create":
		h.bookmarkCreate(w, r, isJSON)
	case "bookmark_edit":
		h.bookmarkEdit(w, r, isJSON)
	case "bookmark_delete":
		h.bookmarkDelete(w, r, isJSON)
	case "get_bookmark":
		h.getBookmark(w, r, isJSON)
	case "deleted_songs":
		h.deletedSongs(w, r, isJSON)
	case "deleted_video":
		h.deletedVideo(w, r, isJSON)
	case "deleted_podcast_episodes":
		h.deletedPodcastEpisodes(w, r, isJSON)

	// User management
	case "user":
		h.user(w, r, isJSON)
	case "users":
		h.users(w, r, isJSON)
	case "user_create":
		h.userCreate(w, r, isJSON)
	case "user_edit":
		h.userEdit(w, r, isJSON)
	case "user_delete":
		h.userDelete(w, r, isJSON)
	case "user_preferences":
		h.userPreferences(w, r, isJSON)
	case "user_preference":
		h.userPreference(w, r, isJSON)
	case "system_preferences":
		h.systemPreferences(w, r, isJSON)
	case "system_preference":
		h.systemPreference(w, r, isJSON)
	case "preference_create":
		h.preferenceCreate(w, r, isJSON)
	case "preference_edit":
		h.preferenceEdit(w, r, isJSON)
	case "preference_delete":
		h.preferenceDelete(w, r, isJSON)
	case "toggle_follow":
		h.toggleFollow(w, r, isJSON)
	case "last_shouts":
		h.lastShouts(w, r, isJSON)
	case "timeline":
		h.timeline(w, r, isJSON)
	case "friends_timeline":
		h.friendsTimeline(w, r, isJSON)

	// Podcasts
	case "podcasts":
		h.podcasts(w, r, isJSON)
	case "podcast":
		h.podcast(w, r, isJSON)
	case "podcast_create":
		h.podcastCreate(w, r, isJSON)
	case "podcast_edit":
		h.podcastEdit(w, r, isJSON)
	case "podcast_delete":
		h.podcastDelete(w, r, isJSON)
	case "podcast_episodes":
		h.podcastEpisodes(w, r, isJSON)
	case "podcast_episode":
		h.podcastEpisode(w, r, isJSON)
	case "podcast_episode_delete":
		h.podcastEpisodeDelete(w, r, isJSON)
	case "update_podcast":
		h.updatePodcast(w, r, isJSON)

	// Internet radio
	case "live_streams":
		h.liveStreams(w, r, isJSON)
	case "live_stream":
		h.liveStream(w, r, isJSON)
	case "live_stream_create":
		h.liveStreamCreate(w, r, isJSON)
	case "live_stream_edit":
		h.liveStreamEdit(w, r, isJSON)
	case "live_stream_delete":
		h.liveStreamDelete(w, r, isJSON)

	default:
		respond(w, r, isJSON, errResp(4701, "Invalid action: "+action))
	}
}

// param returns the first non-empty value from the query string or form body.
func param(r *http.Request, key string) string {
	v := r.URL.Query().Get(key)
	if v == "" {
		v = r.FormValue(key)
	}
	return v
}

// requireSession validates the auth token and returns the session, or writes an
// error response and returns nil.
func (h *Handler) requireSession(w http.ResponseWriter, r *http.Request, isJSON bool) *middleware.AmpacheSession {
	token := param(r, "auth")
	if token == "" {
		respond(w, r, isJSON, errResp(4700, "Access denied: missing auth token"))
		return nil
	}
	session := h.sessions.Get(token)
	if session == nil {
		respond(w, r, isJSON, errResp(4700, "Access denied: invalid or expired session"))
		return nil
	}
	return session
}

// requireAdmin validates the session and checks that the user is an admin.
// Returns the session on success, nil on failure (response already written).
func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request, isJSON bool) *middleware.AmpacheSession {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return nil
	}
	user, err := h.db.Users.GetUser(r.Context(), session.UserID)
	if err != nil || user == nil || !user.IsAdmin {
		respond(w, r, isJSON, errResp(4742, "Failed access check: admin required"))
		return nil
	}
	return session
}
