package subsonic

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/local/cassonic/src/server/service"
	"github.com/local/cassonic/src/server/service/ffmpeg"
	"github.com/local/cassonic/src/server/store"
)

// SubsonicVersion is the Subsonic REST API version this handler implements.
const SubsonicVersion = "1.16.1"

// XMLNamespace is the XML namespace declared on all subsonic-response elements.
const XMLNamespace = "http://subsonic.org/restapi"

// Handler holds all dependencies for the Subsonic API layer.
type Handler struct {
	db          *store.DB
	scanner     *service.Scanner
	coverArt    *service.CoverArtService
	ffmpeg      *ffmpeg.Manager
	nowPlaying  *NowPlayingTracker
	subsPass    func(ctx context.Context, username string) (string, bool)
}

// NewHandler creates a Subsonic API handler with all required dependencies.
func NewHandler(
	db *store.DB,
	scanner *service.Scanner,
	coverArt *service.CoverArtService,
	ff *ffmpeg.Manager,
	subsPass func(ctx context.Context, username string) (string, bool),
) *Handler {
	return &Handler{
		db:         db,
		scanner:    scanner,
		coverArt:   coverArt,
		ffmpeg:     ff,
		nowPlaying: NewNowPlayingTracker(),
		subsPass:   subsPass,
	}
}

// NowPlayingTracker returns the handler's NowPlayingTracker for use by the native API SSE layer.
func (h *Handler) NowPlayingTrackerRef() *NowPlayingTracker {
	return h.nowPlaying
}

// Routes returns a chi router with all Subsonic REST API endpoints mounted.
// Each endpoint is registered both with and without the .view suffix.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()

	// Mount all actions under /rest/ — dispatch strips .view suffix.
	r.Get("/rest/{action}", h.dispatch)
	r.Post("/rest/{action}", h.dispatch)
	r.Get("/rest/{action}.view", h.dispatchView)
	r.Post("/rest/{action}.view", h.dispatchView)

	return r
}

// dispatchView strips the .view suffix from the URL path and re-dispatches.
func (h *Handler) dispatchView(w http.ResponseWriter, r *http.Request) {
	action := chi.URLParam(r, "action")
	action = strings.TrimSuffix(action, ".view")
	h.route(w, r, action)
}

// dispatch reads the action directly from the URL parameter.
func (h *Handler) dispatch(w http.ResponseWriter, r *http.Request) {
	action := chi.URLParam(r, "action")
	h.route(w, r, action)
}

// route maps an action name to the appropriate handler method.
func (h *Handler) route(w http.ResponseWriter, r *http.Request, action string) {
	switch action {
	// System
	case "ping":
		h.ping(w, r)
	case "getLicense":
		h.getLicense(w, r)
	case "getScanStatus":
		h.getScanStatus(w, r)
	case "startScan":
		h.startScan(w, r)
	case "getUser":
		h.getUser(w, r)
	case "getUsers":
		h.getUsers(w, r)
	case "createUser":
		h.createUser(w, r)
	case "updateUser":
		h.updateUser(w, r)
	case "deleteUser":
		h.deleteUser(w, r)
	case "changePassword":
		h.changePassword(w, r)

	// Browse
	case "getMusicFolders":
		h.getMusicFolders(w, r)
	case "getIndexes":
		h.getIndexes(w, r)
	case "getMusicDirectory":
		h.getMusicDirectory(w, r)
	case "getGenres":
		h.getGenres(w, r)
	case "getArtists":
		h.getArtists(w, r)
	case "getArtist":
		h.getArtist(w, r)
	case "getAlbum":
		h.getAlbum(w, r)
	case "getSong":
		h.getSong(w, r)
	case "getAlbumList":
		h.getAlbumList(w, r)
	case "getAlbumList2":
		h.getAlbumList2(w, r)
	case "getRandomSongs":
		h.getRandomSongs(w, r)
	case "getSongsByGenre":
		h.getSongsByGenre(w, r)
	case "getStarred":
		h.getStarred(w, r)
	case "getStarred2":
		h.getStarred2(w, r)
	case "getNowPlaying":
		h.getNowPlaying(w, r)
	case "getVideos":
		h.getVideos(w, r)
	case "getVideoInfo":
		h.getVideoInfo(w, r)
	case "getArtistInfo":
		h.getArtistInfo(w, r)
	case "getArtistInfo2":
		h.getArtistInfo2(w, r)
	case "getAlbumInfo":
		h.getAlbumInfo(w, r)
	case "getAlbumInfo2":
		h.getAlbumInfo2(w, r)
	case "getSimilarSongs":
		h.getSimilarSongs(w, r)
	case "getSimilarSongs2":
		h.getSimilarSongs2(w, r)
	case "getTopSongs":
		h.getTopSongs(w, r)

	// Stream
	case "stream":
		h.stream(w, r)
	case "download":
		h.download(w, r)
	case "hls":
		h.hls(w, r)
	case "getCoverArt":
		h.getCoverArt(w, r)
	case "getLyrics":
		h.getLyrics(w, r)
	case "getAvatar":
		h.getAvatar(w, r)
	case "getCaptions":
		h.getCaptions(w, r)

	// Playlists and interaction
	case "getPlaylists":
		h.getPlaylists(w, r)
	case "getPlaylist":
		h.getPlaylist(w, r)
	case "createPlaylist":
		h.createPlaylist(w, r)
	case "updatePlaylist":
		h.updatePlaylist(w, r)
	case "deletePlaylist":
		h.deletePlaylist(w, r)
	case "search":
		h.search(w, r)
	case "search2":
		h.search2(w, r)
	case "search3":
		h.search3(w, r)
	case "star":
		h.star(w, r)
	case "unstar":
		h.unstar(w, r)
	case "setRating":
		h.setRating(w, r)
	case "scrobble":
		h.scrobble(w, r)
	case "getShares":
		h.getShares(w, r)
	case "createShare":
		h.createShare(w, r)
	case "updateShare":
		h.updateShare(w, r)
	case "deleteShare":
		h.deleteShare(w, r)
	case "getBookmarks":
		h.getBookmarks(w, r)
	case "createBookmark":
		h.createBookmark(w, r)
	case "deleteBookmark":
		h.deleteBookmark(w, r)
	case "getPlayQueue":
		h.getPlayQueue(w, r)
	case "savePlayQueue":
		h.savePlayQueue(w, r)
	case "getChatMessages":
		h.getChatMessages(w, r)
	case "addChatMessage":
		h.addChatMessage(w, r)

	// Podcast
	case "getPodcasts":
		h.getPodcasts(w, r)
	case "getNewestPodcasts":
		h.getNewestPodcasts(w, r)
	case "refreshPodcasts":
		h.refreshPodcasts(w, r)
	case "createPodcastChannel":
		h.createPodcastChannel(w, r)
	case "deletePodcastChannel":
		h.deletePodcastChannel(w, r)
	case "deletePodcastEpisode":
		h.deletePodcastEpisode(w, r)
	case "downloadPodcastEpisode":
		h.downloadPodcastEpisode(w, r)

	// Internet radio
	case "getInternetRadioStations":
		h.getInternetRadioStations(w, r)
	case "createInternetRadioStation":
		h.createInternetRadioStation(w, r)
	case "updateInternetRadioStation":
		h.updateInternetRadioStation(w, r)
	case "deleteInternetRadioStation":
		h.deleteInternetRadioStation(w, r)

	// Jukebox
	case "jukeboxControl":
		h.jukeboxControl(w, r)

	default:
		respond(w, r, errResp(ErrNotFound, "Unknown action: "+action))
	}
}
