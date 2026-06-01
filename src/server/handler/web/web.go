// Package web serves the cassonic WebUI using server-side Go templates.
package web

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/argon2"

	"github.com/local/cassonic/src/common/i18n"
	"github.com/local/cassonic/src/config"
	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// Handler serves the cassonic WebUI.
type Handler struct {
	db      *store.DB
	cfg     *config.Config
	version string
	tmpls   *template.Template
	i18n    *i18n.Bundle
}

// PageData is the base data passed to every template.
type PageData struct {
	Title   string
	User    *mw.AuthUser
	Version string
	Flash   string
	Lang    string
	T       func(string) string
}

// NewHandler creates a web Handler backed by the given DB aggregate and config.
func NewHandler(db *store.DB) *Handler {
	return NewHandlerWithConfig(db, config.Defaults(), "dev")
}

// NewHandlerWithConfig creates a web Handler with full configuration.
func NewHandlerWithConfig(db *store.DB, cfg *config.Config, version string) *Handler {
	h := &Handler{
		db:      db,
		cfg:     cfg,
		version: version,
		i18n:    i18n.Default(),
	}
	h.tmpls = h.parseTemplates()
	return h
}

// parseTemplates parses all HTML templates from the embedded filesystem.
func (h *Handler) parseTemplates() *template.Template {
	funcMap := template.FuncMap{
		"formatDuration": formatDuration,
		"formatDate":     formatDate,
		"inc":            func(i int) int { return i + 1 },
		"dec":            func(i int) int { return i - 1 },
		"seq":            makeSeq,
		"formatSize":     formatSize,
		"safeHTML":       func(s string) template.HTML { return template.HTML(s) },
		"not":            templateNot,
		// t is a no-op placeholder; real translation is provided via PageData.T per request.
		"t": func(key string) string { return key },
	}

	tmpl := template.New("").Funcs(funcMap)

	sub, err := fs.Sub(assets, "template")
	if err != nil {
		panic(fmt.Sprintf("web: sub template fs: %v", err))
	}

	tmpl, err = tmpl.ParseFS(sub, "*.html")
	if err != nil {
		panic(fmt.Sprintf("web: parse templates: %v", err))
	}
	return tmpl
}

// staticFS exposes only the static/ subtree for the file server.
var staticFS, _ = fs.Sub(assets, "static")

// Routes returns a chi router for all WebUI endpoints.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/server/about", h.About)
	r.Get("/server/help", h.Help)
	r.Get("/login", h.Login)
	r.Post("/login", h.LoginPost)
	r.Get("/share/{token}", h.Share)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	r.Group(func(r chi.Router) {
		r.Use(h.sessionAuth)
		r.Get("/", h.Home)
		r.Post("/logout", h.Logout)
		r.Get("/library", h.Library)
		r.Get("/artists", h.Artists)
		r.Get("/albums", h.Albums)
		r.Get("/songs", h.Songs)
		r.Get("/genres", h.Genres)
		r.Get("/playlists", h.Playlists)
		r.Get("/playlists/{id}", h.PlaylistDetail)
		r.Get("/album/{id}", h.AlbumDetail)
		r.Get("/artist/{id}", h.ArtistDetail)
		r.Get("/search", h.Search)
		r.Get("/player", h.Player)
		r.Get("/tags/{id}", h.TagEditor)
		r.Get("/icecast", h.Icecast)
		r.Get("/settings", h.Settings)
		r.Get("/upload", h.Upload)
	})

	return r
}

// sessionCookieName is the cookie used to store the web session token.
const sessionCookieName = "cassonic_session"

// sessionAuth is chi middleware that authenticates the session cookie.
// On failure it redirects to /login.
func (h *Handler) sessionAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		raw := cookie.Value
		sum := sha256.Sum256([]byte(raw))
		tokenHash := hex.EncodeToString(sum[:])

		session, err := h.db.Users.GetSessionByHash(r.Context(), tokenHash)
		if err != nil || session == nil || session.IsExpired() {
			if session != nil {
				_ = h.db.Users.DeleteSession(r.Context(), tokenHash)
			}
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookieName,
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		user, err := h.db.Users.GetUser(r.Context(), session.UserID)
		if err != nil || user == nil || !user.IsEnabled || user.IsLocked() {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		ctx := mw.WithUser(r.Context(), &mw.AuthUser{
			ID:       user.ID,
			Username: user.Username,
			IsAdmin:  user.IsAdmin,
			Scheme:   mw.SchemeNative,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolveLocale selects the best locale for the request using the priority chain:
// ?lang= query param (writes a cookie) → lang cookie → Accept-Language → "en".
func (h *Handler) resolveLocale(w http.ResponseWriter, r *http.Request) string {
	supported := map[string]bool{
		"en": true, "es": true, "fr": true,
		"de": true, "zh": true, "ja": true, "ar": true,
	}

	if q := r.URL.Query().Get("lang"); q != "" {
		code := strings.ToLower(strings.SplitN(q, "-", 2)[0])
		if supported[code] {
			http.SetCookie(w, &http.Cookie{
				Name:     "lang",
				Value:    code,
				Path:     "/",
				MaxAge:   365 * 24 * 3600,
				HttpOnly: false,
				SameSite: http.SameSiteLaxMode,
			})
			return code
		}
	}

	if c, err := r.Cookie("lang"); err == nil && c.Value != "" {
		code := strings.ToLower(strings.SplitN(c.Value, "-", 2)[0])
		if supported[code] {
			return code
		}
	}

	if al := r.Header.Get("Accept-Language"); al != "" {
		for _, tag := range strings.Split(al, ",") {
			tag = strings.TrimSpace(strings.SplitN(tag, ";", 2)[0])
			code := strings.ToLower(strings.SplitN(tag, "-", 2)[0])
			if supported[code] {
				return code
			}
		}
	}

	return "en"
}

// render executes a named template and writes the result to w.
func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpls.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// base builds a PageData with the authenticated user, version, and i18n translator filled in.
// Note: base does not have access to http.ResponseWriter, so lang-cookie writes from ?lang=
// happen in the render path. Callers that need to set the lang cookie must call resolveLocale
// directly. For most pages base is called after resolveLocale has already run via baseWithLang.
func (h *Handler) base(r *http.Request, title string) PageData {
	lang := "en"
	if c, err := r.Cookie("lang"); err == nil && c.Value != "" {
		lang = strings.ToLower(strings.SplitN(c.Value, "-", 2)[0])
	} else if al := r.Header.Get("Accept-Language"); al != "" {
		for _, tag := range strings.Split(al, ",") {
			tag = strings.TrimSpace(strings.SplitN(tag, ";", 2)[0])
			code := strings.ToLower(strings.SplitN(tag, "-", 2)[0])
			supported := map[string]bool{
				"en": true, "es": true, "fr": true,
				"de": true, "zh": true, "ja": true, "ar": true,
			}
			if supported[code] {
				lang = code
				break
			}
		}
	}
	bundle := h.i18n
	return PageData{
		Title:   title,
		User:    mw.UserFromContext(r.Context()),
		Version: h.version,
		Lang:    lang,
		T:       func(key string) string { return bundle.T(lang, key) },
	}
}

// Home renders the dashboard.
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	pd := h.base(r, "Dashboard — cassonic")

	type dashData struct {
		PageData
		RecentAlbums   []*model.Album
		RecentlyPlayed []*model.PlayHistory
		SongCount      int
		AlbumCount     int
		ArtistCount    int
	}

	recent, _ := h.db.Music.GetNewestAlbums(r.Context(), 12)

	artists, _ := h.db.Music.ListArtists(r.Context(), store.ListOpts{Limit: 1})
	albums, _ := h.db.Music.ListAlbums(r.Context(), store.ListOpts{Limit: 1})
	songs, _ := h.db.Music.SearchSongs(r.Context(), "", store.ListOpts{Limit: 1})

	var played []*model.PlayHistory
	if user := mw.UserFromContext(r.Context()); user != nil {
		played, _ = h.db.Activity.GetPlayHistory(r.Context(), user.ID, 10)
	}

	h.render(w, "index.html", dashData{
		PageData:       pd,
		RecentAlbums:   recent,
		RecentlyPlayed: played,
		SongCount:      len(songs),
		AlbumCount:     len(albums),
		ArtistCount:    len(artists),
	})
}

// Login renders the login form.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	type loginData struct {
		PageData
		Error string
	}
	lang := h.resolveLocale(w, r)
	bundle := h.i18n
	data := loginData{
		PageData: PageData{
			Title:   "Login — cassonic",
			Version: h.version,
			Lang:    lang,
			T:       func(key string) string { return bundle.T(lang, key) },
		},
		Error: r.URL.Query().Get("error"),
	}
	h.render(w, "login.html", data)
}

// argon2id cost parameters matching the API handler.
const (
	argon2Memory      = 65536
	argon2Iterations  = 3
	argon2Parallelism = 4
)

// verifyPassword checks a plaintext password against a stored Argon2id hash.
func verifyPassword(password, hash string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("argon2id: unsupported hash format")
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, fmt.Errorf("argon2id: parse params: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("argon2id: decode salt: %w", err)
	}
	storedKey, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("argon2id: decode key: %w", err)
	}
	computed := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(storedKey)))
	return subtle.ConstantTimeCompare(computed, storedKey) == 1, nil
}

// generateSessionToken creates a 32-byte random hex session token.
func generateSessionToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("token: rand: %w", err)
	}
	raw = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return raw, hash, nil
}

// LoginPost handles the login form submission.
func (h *Handler) LoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?error=bad+request", http.StatusFound)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	remember := r.FormValue("remember") == "on"

	if username == "" || password == "" {
		http.Redirect(w, r, "/login?error=username+and+password+required", http.StatusFound)
		return
	}

	user, err := h.db.Users.GetUserByUsername(r.Context(), username)
	if err != nil || user == nil || !user.IsEnabled || user.IsLocked() {
		http.Redirect(w, r, "/login?error=invalid+credentials", http.StatusFound)
		return
	}

	ok, err := verifyPassword(password, user.PasswordHash)
	if err != nil || !ok {
		_ = h.db.Users.IncrementLoginAttempts(r.Context(), user.ID)
		http.Redirect(w, r, "/login?error=invalid+credentials", http.StatusFound)
		return
	}

	raw, tokenHash, err := generateSessionToken()
	if err != nil {
		http.Redirect(w, r, "/login?error=server+error", http.StatusFound)
		return
	}

	duration := time.Duration(h.cfg.Auth.SessionDuration) * time.Hour
	if remember {
		duration = 30 * 24 * time.Hour
	}
	expiresAt := time.Now().UTC().Add(duration)

	if err := h.db.Users.CreateSession(r.Context(), user.ID, tokenHash, expiresAt, "web"); err != nil {
		http.Redirect(w, r, "/login?error=server+error", http.StatusFound)
		return
	}

	_ = h.db.Users.ResetLoginAttempts(r.Context(), user.ID)
	_ = h.db.Users.UpdateLastLogin(r.Context(), user.ID)

	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    raw,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if remember {
		cookie.Expires = expiresAt
		cookie.MaxAge = int(duration.Seconds())
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusFound)
}

// Logout clears the session cookie and destroys the server-side session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && cookie.Value != "" {
		sum := sha256.Sum256([]byte(cookie.Value))
		tokenHash := hex.EncodeToString(sum[:])
		_ = h.db.Users.DeleteSession(r.Context(), tokenHash)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/login", http.StatusFound)
}

// Library renders the library management page.
func (h *Handler) Library(w http.ResponseWriter, r *http.Request) {
	type libData struct {
		PageData
		Libraries []*model.Library
		LastScan  *model.ScanStatus
	}

	libs, _ := h.db.Music.ListLibraries(r.Context())
	scan, _ := h.db.Music.GetLastScanStatus(r.Context())

	h.render(w, "library.html", libData{
		PageData:  h.base(r, "Library — cassonic"),
		Libraries: libs,
		LastScan:  scan,
	})
}

// Artists renders the artists browser.
func (h *Handler) Artists(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	const perPage = 48
	offset := (page - 1) * perPage

	type artistsData struct {
		PageData
		Artists  []*model.Artist
		Page     int
		HasMore  bool
	}

	artists, _ := h.db.Music.ListArtists(r.Context(), store.ListOpts{
		Offset: offset,
		Limit:  perPage + 1,
		SortBy: "name",
	})

	hasMore := len(artists) > perPage
	if hasMore {
		artists = artists[:perPage]
	}

	h.render(w, "artists.html", artistsData{
		PageData: h.base(r, "Artists — cassonic"),
		Artists:  artists,
		Page:     page,
		HasMore:  hasMore,
	})
}

// Albums renders the albums browser.
func (h *Handler) Albums(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	const perPage = 48
	offset := (page - 1) * perPage

	type albumsData struct {
		PageData
		Albums  []*model.Album
		Page    int
		HasMore bool
	}

	albums, _ := h.db.Music.ListAlbums(r.Context(), store.ListOpts{
		Offset: offset,
		Limit:  perPage + 1,
		SortBy: "title",
	})

	hasMore := len(albums) > perPage
	if hasMore {
		albums = albums[:perPage]
	}

	h.render(w, "albums.html", albumsData{
		PageData: h.base(r, "Albums — cassonic"),
		Albums:   albums,
		Page:     page,
		HasMore:  hasMore,
	})
}

// Songs renders the songs browser.
func (h *Handler) Songs(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	const perPage = 100
	offset := (page - 1) * perPage

	type songsData struct {
		PageData
		Songs   []*model.Song
		Page    int
		HasMore bool
	}

	songs, _ := h.db.Music.SearchSongs(r.Context(), "", store.ListOpts{
		Offset: offset,
		Limit:  perPage + 1,
		SortBy: "title",
	})

	hasMore := len(songs) > perPage
	if hasMore {
		songs = songs[:perPage]
	}

	h.render(w, "songs.html", songsData{
		PageData: h.base(r, "Songs — cassonic"),
		Songs:    songs,
		Page:     page,
		HasMore:  hasMore,
	})
}

// Genres renders the genre browser.
func (h *Handler) Genres(w http.ResponseWriter, r *http.Request) {
	type genresData struct {
		PageData
		Genres []*model.Genre
	}

	genres, _ := h.db.Music.ListGenres(r.Context())

	h.render(w, "genres.html", genresData{
		PageData: h.base(r, "Genres — cassonic"),
		Genres:   genres,
	})
}

// Playlists renders the playlist list.
func (h *Handler) Playlists(w http.ResponseWriter, r *http.Request) {
	user := mw.UserFromContext(r.Context())

	type playlistsData struct {
		PageData
		Playlists []*model.Playlist
		UserID    int64
	}

	pls, _ := h.db.Playlists.ListPlaylists(r.Context(), user.ID)

	h.render(w, "playlists.html", playlistsData{
		PageData:  h.base(r, "Playlists — cassonic"),
		Playlists: pls,
		UserID:    user.ID,
	})
}

// PlaylistDetail renders a single playlist.
func (h *Handler) PlaylistDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	pl, err := h.db.Playlists.GetPlaylist(r.Context(), id)
	if err != nil || pl == nil {
		http.NotFound(w, r)
		return
	}

	entries, _ := h.db.Playlists.GetPlaylistEntries(r.Context(), id)

	type songEntry struct {
		Entry *model.PlaylistEntry
		Song  *model.Song
	}

	type playlistData struct {
		PageData
		Playlist *model.Playlist
		Entries  []songEntry
		UserID   int64
	}

	var songEntries []songEntry
	for _, e := range entries {
		s, err := h.db.Music.GetSong(r.Context(), e.SongID)
		if err != nil || s == nil {
			continue
		}
		songEntries = append(songEntries, songEntry{Entry: e, Song: s})
	}

	user := mw.UserFromContext(r.Context())
	h.render(w, "playlist.html", playlistData{
		PageData: h.base(r, pl.Name+" — cassonic"),
		Playlist: pl,
		Entries:  songEntries,
		UserID:   user.ID,
	})
}

// AlbumDetail renders a single album with its tracklist.
func (h *Handler) AlbumDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	album, err := h.db.Music.GetAlbum(r.Context(), id)
	if err != nil || album == nil {
		http.NotFound(w, r)
		return
	}

	songs, _ := h.db.Music.ListSongsByAlbum(r.Context(), id)

	type albumData struct {
		PageData
		Album *model.Album
		Songs []*model.Song
	}

	h.render(w, "album.html", albumData{
		PageData: h.base(r, album.Title+" — cassonic"),
		Album:    album,
		Songs:    songs,
	})
}

// ArtistDetail renders a single artist with their discography.
func (h *Handler) ArtistDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	artist, err := h.db.Music.GetArtist(r.Context(), id)
	if err != nil || artist == nil {
		http.NotFound(w, r)
		return
	}

	albums, _ := h.db.Music.ListAlbumsByArtist(r.Context(), id)

	type artistData struct {
		PageData
		Artist *model.Artist
		Albums []*model.Album
	}

	h.render(w, "artist.html", artistData{
		PageData: h.base(r, artist.Name+" — cassonic"),
		Artist:   artist,
		Albums:   albums,
	})
}

// Search renders the search results page.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	type searchData struct {
		PageData
		Query   string
		Songs   []*model.Song
		Albums  []*model.Album
		Artists []*model.Artist
	}

	data := searchData{
		PageData: h.base(r, "Search — cassonic"),
		Query:    q,
	}

	if q != "" {
		opts := store.ListOpts{Limit: 20}
		data.Songs, _ = h.db.Music.SearchSongs(r.Context(), q, opts)
		data.Albums, _ = h.db.Music.SearchAlbums(r.Context(), q, opts)
		data.Artists, _ = h.db.Music.SearchArtists(r.Context(), q, opts)
	}

	h.render(w, "search.html", data)
}

// Player renders the full-screen now-playing view.
func (h *Handler) Player(w http.ResponseWriter, r *http.Request) {
	h.render(w, "player.html", h.base(r, "Now Playing — cassonic"))
}

// TagEditor renders the metadata editor for a song.
func (h *Handler) TagEditor(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	song, err := h.db.Music.GetSong(r.Context(), id)
	if err != nil || song == nil {
		http.NotFound(w, r)
		return
	}

	type tagsData struct {
		PageData
		Song *model.Song
	}

	h.render(w, "tags.html", tagsData{
		PageData: h.base(r, "Edit Tags — cassonic"),
		Song:     song,
	})
}

// Icecast renders the Icecast server management page.
func (h *Handler) Icecast(w http.ResponseWriter, r *http.Request) {
	type serverWithMounts struct {
		Server *model.IcecastServer
		Mounts []*model.IcecastMount
	}

	type icecastData struct {
		PageData
		Servers []serverWithMounts
	}

	servers, _ := h.db.Icecast.ListServers(r.Context())

	var serverData []serverWithMounts
	for _, s := range servers {
		mounts, _ := h.db.Icecast.ListMountsByServer(r.Context(), s.ID)
		serverData = append(serverData, serverWithMounts{Server: s, Mounts: mounts})
	}

	h.render(w, "icecast.html", icecastData{
		PageData: h.base(r, "Icecast — cassonic"),
		Servers:  serverData,
	})
}

// Settings renders the settings page.
func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) {
	user := mw.UserFromContext(r.Context())

	type settingsData struct {
		PageData
		Tokens    []*model.APIToken
		Libraries []*model.Library
		Cfg       *config.Config
	}

	tokens, _ := h.db.Users.ListAPITokens(r.Context(), user.ID)
	libs, _ := h.db.Music.ListLibraries(r.Context())

	h.render(w, "settings.html", settingsData{
		PageData:  h.base(r, "Settings — cassonic"),
		Tokens:    tokens,
		Libraries: libs,
		Cfg:       h.cfg,
	})
}

// Share renders the public share page (no authentication required).
func (h *Handler) Share(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	share, err := h.db.Shares.GetShareByToken(r.Context(), token)
	if err != nil || share == nil || share.IsExpired() {
		http.NotFound(w, r)
		return
	}

	_ = h.db.Shares.IncrementViewCount(r.Context(), share.ID)

	type shareData struct {
		PageData
		Share   *model.Share
		Songs   []*model.Song
		Album   *model.Album
	}

	lang := h.resolveLocale(w, r)
	bundle := h.i18n
	data := shareData{
		PageData: PageData{
			Title:   "Shared — cassonic",
			Version: h.version,
			Lang:    lang,
			T:       func(key string) string { return bundle.T(lang, key) },
		},
		Share: share,
	}

	switch share.ItemType {
	case "song":
		s, err := h.db.Music.GetSong(r.Context(), share.ItemID)
		if err == nil && s != nil {
			data.Songs = []*model.Song{s}
		}
	case "album":
		a, err := h.db.Music.GetAlbum(r.Context(), share.ItemID)
		if err == nil && a != nil {
			data.Album = a
			data.Songs, _ = h.db.Music.ListSongsByAlbum(r.Context(), share.ItemID)
		}
	}

	h.render(w, "share.html", data)
}

// aboutDescription is the project description shown on the About page.
const aboutDescription = "cassonic is a self-hosted music streaming server — a full-featured, drop-in replacement for Airsonic, Subsonic, Libresonic, Ampache, and kPlaylist. Every existing Subsonic and Ampache client works without reconfiguration."

// aboutFeatures lists the key features shown on the About page.
var aboutFeatures = []string{
	"Full Subsonic REST API compatibility (v1.1.0–1.16.1)",
	"Ampache API v5 and v6 compatibility (XML and JSON)",
	"Built-in tag editor (ID3v2, MP4, Vorbis, FLAC, APE, WMA, WAV — 7 formats)",
	"MusicBrainz ID auto-lookup (opt-in, never overwrites user-edited fields)",
	"Multi-server multi-mount Icecast relay streaming (by track, artist, or genre)",
	"Multi-service audio scrobbling (Last.fm, ListenBrainz, Libre.fm, GNU FM, Maloja, and custom servers)",
	"Podcast support (RSS fetch, episode download, playback)",
	"Public share links with optional expiry and password",
	"Audio file upload (per-user permission, admin-configurable)",
	"Mobile-first WebUI with built-in music player",
	"Tor hidden service support (auto-enabled when tor binary is present)",
	"Built-in scheduler, GeoIP filtering, Prometheus metrics, backup, and auto-update",
}

// About renders the /server/about page with project description and feature list.
// When Accept: text/plain is sent, a plain-text summary is returned instead.
func (h *Handler) About(w http.ResponseWriter, r *http.Request) {
	if mw.AcceptedFormat(r) == "plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "cassonic — About\n\n%s\n\nFeatures:\n", aboutDescription)
		for _, f := range aboutFeatures {
			fmt.Fprintf(w, "  - %s\n", f)
		}
		return
	}

	type aboutData struct {
		PageData
		Description string
		Features    []string
	}
	lang := h.resolveLocale(w, r)
	bundle := h.i18n
	h.render(w, "about.html", aboutData{
		PageData: PageData{
			Title:   "About cassonic",
			Version: h.version,
			Lang:    lang,
			T:       func(key string) string { return bundle.T(lang, key) },
		},
		Description: aboutDescription,
		Features:    aboutFeatures,
	})
}

// Help renders the /server/help page with getting-started guide and real API examples.
// When Accept: text/plain is sent, a plain-text summary is returned instead.
func (h *Handler) Help(w http.ResponseWriter, r *http.Request) {
	port := 4533
	if h.cfg != nil && h.cfg.Server.Port != 0 {
		port = h.cfg.Server.Port
	}

	if mw.AcceptedFormat(r) == "plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "cassonic — Help\n\nGetting Started:\n")
		fmt.Fprintf(w, "  Server runs on port %d by default.\n", port)
		fmt.Fprintf(w, "  Subsonic API base: http://localhost:%d/rest/\n", port)
		fmt.Fprintf(w, "  Ampache API base:  http://localhost:%d/ampache/server/xml.server.php\n", port)
		fmt.Fprintf(w, "  Admin panel:       http://localhost:%d/admin\n", port)
		fmt.Fprintf(w, "  Health check:      http://localhost:%d/health\n", port)
		return
	}

	type helpData struct {
		PageData
		Port int
	}
	lang := h.resolveLocale(w, r)
	bundle := h.i18n
	h.render(w, "help.html", helpData{
		PageData: PageData{
			Title:   "Help — cassonic",
			Version: h.version,
			Lang:    lang,
			T:       func(key string) string { return bundle.T(lang, key) },
		},
		Port: port,
	})
}

// Upload renders the file upload page.
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	type uploadData struct {
		PageData
		Libraries []*model.Library
	}

	libs, _ := h.db.Music.ListLibraries(r.Context())

	h.render(w, "upload.html", uploadData{
		PageData:  h.base(r, "Upload — cassonic"),
		Libraries: libs,
	})
}

// queryInt reads an integer query parameter, returning def on error or absence.
func queryInt(r *http.Request, key string, def int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return def
	}
	return v
}

// formatDuration formats a duration in seconds as M:SS or H:MM:SS.
func formatDuration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// formatDate formats a time.Time as YYYY-MM-DD.
func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// templateNot negates the truthiness of any value, mirroring Go template semantics.
func templateNot(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case bool:
		return !val
	case int:
		return val == 0
	case int64:
		return val == 0
	case string:
		return val == ""
	default:
		return false
	}
}

// makeSeq returns a slice of ints from 0 to n-1.
func makeSeq(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}

// formatSize formats a byte count as a human-readable string.
func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/gb)
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/mb)
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/kb)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
