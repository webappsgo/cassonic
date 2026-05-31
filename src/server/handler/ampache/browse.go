package ampache

import (
	"net/http"
	"strings"

	"github.com/local/cassonic/src/server/service"
	"github.com/local/cassonic/src/server/store"
)

// scanModeFull is a local alias for service.ScanModeFull used by scan-triggering actions.
const scanModeFull = service.ScanModeFull

// baseURL extracts the scheme and host from r for use in stream/art URLs.
func baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	return scheme + "://" + r.Host
}

// artists lists all artists with optional search filter, limit, and offset.
func (h *Handler) artists(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	ctx := r.Context()
	filter := param(r, "filter")
	limit := parseIntParam(r, "limit", 500)
	offset := parseIntParam(r, "offset", 0)
	base := baseURL(r)

	var result []AmpArtist
	if filter != "" {
		list, err := h.db.Music.SearchArtists(ctx, filter, store.ListOpts{Limit: limit, Offset: offset})
		if err != nil {
			respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
			return
		}
		for _, a := range list {
			result = append(result, artistToAmp(a, base))
		}
	} else {
		list, err := h.db.Music.ListArtists(ctx, store.ListOpts{Limit: limit, Offset: offset})
		if err != nil {
			respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
			return
		}
		for _, a := range list {
			result = append(result, artistToAmp(a, base))
		}
	}

	if result == nil {
		result = []AmpArtist{}
	}
	respond(w, r, isJSON, okResp("artist", result))
}

// artist returns a single artist by its ID supplied in the filter parameter.
func (h *Handler) artist(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (artist ID)"))
		return
	}

	a, err := h.db.Music.GetArtist(r.Context(), id)
	if err != nil || a == nil {
		respond(w, r, isJSON, errResp(4704, "Not found"))
		return
	}

	respond(w, r, isJSON, artistToAmp(a, baseURL(r)))
}

// artistAlbums returns all albums for the artist whose ID is in the filter parameter.
func (h *Handler) artistAlbums(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (artist ID)"))
		return
	}

	list, err := h.db.Music.ListAlbumsByArtist(r.Context(), id)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	base := baseURL(r)
	result := make([]AmpAlbum, 0, len(list))
	for _, a := range list {
		result = append(result, albumToAmp(a, base))
	}
	respond(w, r, isJSON, okResp("album", result))
}

// artistSongs returns all songs attributed to the artist whose ID is in the filter parameter.
func (h *Handler) artistSongs(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (artist ID)"))
		return
	}

	list, err := h.db.Music.ListSongsByArtist(r.Context(), id)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	base := baseURL(r)
	result := make([]AmpSong, 0, len(list))
	for _, s := range list {
		result = append(result, songToAmp(s, base))
	}
	respond(w, r, isJSON, okResp("song", result))
}

// albums lists albums with optional filter, limit, and offset.
func (h *Handler) albums(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	ctx := r.Context()
	filter := param(r, "filter")
	limit := parseIntParam(r, "limit", 500)
	offset := parseIntParam(r, "offset", 0)
	base := baseURL(r)

	var result []AmpAlbum
	if filter != "" {
		list, err := h.db.Music.SearchAlbums(ctx, filter, store.ListOpts{Limit: limit, Offset: offset})
		if err != nil {
			respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
			return
		}
		for _, a := range list {
			result = append(result, albumToAmp(a, base))
		}
	} else {
		list, err := h.db.Music.ListAlbums(ctx, store.ListOpts{Limit: limit, Offset: offset})
		if err != nil {
			respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
			return
		}
		for _, a := range list {
			result = append(result, albumToAmp(a, base))
		}
	}

	if result == nil {
		result = []AmpAlbum{}
	}
	respond(w, r, isJSON, okResp("album", result))
}

// album returns a single album by ID.
func (h *Handler) album(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (album ID)"))
		return
	}

	a, err := h.db.Music.GetAlbum(r.Context(), id)
	if err != nil || a == nil {
		respond(w, r, isJSON, errResp(4704, "Not found"))
		return
	}

	respond(w, r, isJSON, albumToAmp(a, baseURL(r)))
}

// albumSongs returns all songs in the album whose ID is in the filter parameter.
func (h *Handler) albumSongs(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (album ID)"))
		return
	}

	list, err := h.db.Music.ListSongsByAlbum(r.Context(), id)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	base := baseURL(r)
	result := make([]AmpSong, 0, len(list))
	for _, s := range list {
		result = append(result, songToAmp(s, base))
	}
	respond(w, r, isJSON, okResp("song", result))
}

// songs lists songs with optional filter, limit, and offset.
func (h *Handler) songs(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	ctx := r.Context()
	filter := param(r, "filter")
	limit := parseIntParam(r, "limit", 500)
	offset := parseIntParam(r, "offset", 0)
	base := baseURL(r)

	var result []AmpSong
	if filter != "" {
		list, err := h.db.Music.SearchSongs(ctx, filter, store.ListOpts{Limit: limit, Offset: offset})
		if err != nil {
			respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
			return
		}
		for _, s := range list {
			result = append(result, songToAmp(s, base))
		}
	} else {
		list, err := h.db.Music.GetRandomSongs(ctx, limit, "", "", "")
		if err != nil {
			respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
			return
		}
		_ = offset
		for _, s := range list {
			result = append(result, songToAmp(s, base))
		}
	}

	if result == nil {
		result = []AmpSong{}
	}
	respond(w, r, isJSON, okResp("song", result))
}

// song returns a single song by ID supplied in the filter parameter.
func (h *Handler) song(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (song ID)"))
		return
	}

	s, err := h.db.Music.GetSong(r.Context(), id)
	if err != nil || s == nil {
		respond(w, r, isJSON, errResp(4704, "Not found"))
		return
	}

	respond(w, r, isJSON, songToAmp(s, baseURL(r)))
}

// genres lists all genres with song and album counts.
func (h *Handler) genres(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	list, err := h.db.Music.ListGenres(r.Context())
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	result := make([]AmpGenre, 0, len(list))
	for i, g := range list {
		result = append(result, AmpGenre{
			ID:         itoa(int64(i + 1)),
			Name:       g.Name,
			SongCount:  g.SongCount,
			AlbumCount: g.AlbumCount,
		})
	}
	respond(w, r, isJSON, okResp("genre", result))
}

// genre returns a single genre by its name supplied in the filter parameter.
func (h *Handler) genre(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	filter := param(r, "filter")
	if filter == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (genre name)"))
		return
	}

	list, err := h.db.Music.ListGenres(r.Context())
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	for i, g := range list {
		if strings.EqualFold(g.Name, filter) {
			respond(w, r, isJSON, AmpGenre{
				ID:         itoa(int64(i + 1)),
				Name:       g.Name,
				SongCount:  g.SongCount,
				AlbumCount: g.AlbumCount,
			})
			return
		}
	}
	respond(w, r, isJSON, errResp(4704, "Not found"))
}

// genreSongs returns songs in the genre named by the filter parameter.
func (h *Handler) genreSongs(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	filter := param(r, "filter")
	if filter == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (genre name)"))
		return
	}

	limit := parseIntParam(r, "limit", 500)
	offset := parseIntParam(r, "offset", 0)

	list, err := h.db.Music.ListSongsByGenre(r.Context(), filter, store.ListOpts{Limit: limit, Offset: offset})
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	base := baseURL(r)
	result := make([]AmpSong, 0, len(list))
	for _, s := range list {
		result = append(result, songToAmp(s, base))
	}
	respond(w, r, isJSON, okResp("song", result))
}

// genreAlbums returns albums in the genre named by the filter parameter.
func (h *Handler) genreAlbums(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	filter := param(r, "filter")
	if filter == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (genre name)"))
		return
	}

	list, err := h.db.Music.SearchAlbums(r.Context(), "", store.ListOpts{Limit: 999})
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	base := baseURL(r)
	result := make([]AmpAlbum, 0)
	for _, a := range list {
		if strings.EqualFold(a.Genre, filter) {
			result = append(result, albumToAmp(a, base))
		}
	}
	respond(w, r, isJSON, okResp("album", result))
}

// genreArtists returns artists whose songs include the genre named by filter.
func (h *Handler) genreArtists(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	filter := param(r, "filter")
	if filter == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (genre name)"))
		return
	}

	songs, err := h.db.Music.ListSongsByGenre(r.Context(), filter, store.ListOpts{Limit: 999999})
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	base := baseURL(r)
	seen := make(map[int64]bool)
	result := make([]AmpArtist, 0)
	for _, s := range songs {
		if seen[s.ArtistID] {
			continue
		}
		seen[s.ArtistID] = true
		a, err := h.db.Music.GetArtist(r.Context(), s.ArtistID)
		if err == nil && a != nil {
			result = append(result, artistToAmp(a, base))
		}
	}
	respond(w, r, isJSON, okResp("artist", result))
}

// getIndexes returns all artists, albums, songs, and playlists as an index structure.
func (h *Handler) getIndexes(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	ctx := r.Context()
	base := baseURL(r)

	artists, err := h.db.Music.ListArtists(ctx, store.ListOpts{Limit: 999999})
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	ampArtists := make([]AmpArtist, 0, len(artists))
	for _, a := range artists {
		ampArtists = append(ampArtists, artistToAmp(a, base))
	}

	respond(w, r, isJSON, okResp("artist", ampArtists))
}

// stats returns aggregate server statistics.
func (h *Handler) stats(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	ctx := r.Context()

	artists, _ := h.db.Music.ListArtists(ctx, store.ListOpts{Limit: 999999})
	albums, _ := h.db.Music.ListAlbums(ctx, store.ListOpts{Limit: 999999})
	genres, _ := h.db.Music.ListGenres(ctx)
	libs, _ := h.db.Music.ListLibraries(ctx)

	songCount := 0
	for _, g := range genres {
		songCount += g.SongCount
	}

	respond(w, r, isJSON, AmpStats{
		Songs:    songCount,
		Albums:   len(albums),
		Artists:  len(artists),
		Genres:   len(genres),
		Catalogs: len(libs),
	})
}

// advancedSearch parses rule arrays and performs text-based search across songs,
// albums, and artists. Supports rule types: title, artist, album. Operators:
// contain, start, end, is, is_not.
func (h *Handler) advancedSearch(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	ctx := r.Context()
	base := baseURL(r)
	searchType := param(r, "type")
	if searchType == "" {
		searchType = "song"
	}
	limit := parseIntParam(r, "limit", 100)
	offset := parseIntParam(r, "offset", 0)

	q := r.URL.Query()

	type rule struct {
		ruleType string
		operator string
		input    string
	}

	var rules []rule
	for i := 1; ; i++ {
		ruleKey := "rule_" + itoa(int64(i))
		ruleType := q.Get(ruleKey)
		if ruleType == "" {
			break
		}
		op := q.Get(ruleKey + "_operator")
		inp := q.Get(ruleKey + "_input")
		rules = append(rules, rule{ruleType: ruleType, operator: op, input: inp})
	}

	applyOp := func(value, op, input string) bool {
		switch op {
		case "0", "contain", "":
			return strings.Contains(strings.ToLower(value), strings.ToLower(input))
		case "1", "start":
			return strings.HasPrefix(strings.ToLower(value), strings.ToLower(input))
		case "2", "end":
			return strings.HasSuffix(strings.ToLower(value), strings.ToLower(input))
		case "3", "is":
			return strings.EqualFold(value, input)
		case "4", "is_not":
			return !strings.EqualFold(value, input)
		default:
			return strings.Contains(strings.ToLower(value), strings.ToLower(input))
		}
	}

	switch searchType {
	case "song":
		var query string
		for _, rl := range rules {
			if rl.ruleType == "title" || rl.ruleType == "artist" || rl.ruleType == "album" {
				query = rl.input
				break
			}
		}
		list, err := h.db.Music.SearchSongs(ctx, query, store.ListOpts{Limit: limit, Offset: offset})
		if err != nil {
			respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
			return
		}
		result := make([]AmpSong, 0)
		for _, s := range list {
			match := true
			for _, rl := range rules {
				var fieldVal string
				switch rl.ruleType {
				case "title":
					fieldVal = s.Title
				case "artist":
					fieldVal = s.ArtistName
				case "album":
					fieldVal = s.AlbumName
				}
				if !applyOp(fieldVal, rl.operator, rl.input) {
					match = false
					break
				}
			}
			if match {
				result = append(result, songToAmp(s, base))
			}
		}
		respond(w, r, isJSON, okResp("song", result))

	case "album":
		var query string
		for _, rl := range rules {
			if rl.ruleType == "title" || rl.ruleType == "album" {
				query = rl.input
				break
			}
		}
		list, err := h.db.Music.SearchAlbums(ctx, query, store.ListOpts{Limit: limit, Offset: offset})
		if err != nil {
			respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
			return
		}
		result := make([]AmpAlbum, 0)
		for _, a := range list {
			match := true
			for _, rl := range rules {
				var fieldVal string
				switch rl.ruleType {
				case "title", "album":
					fieldVal = a.Title
				case "artist":
					fieldVal = a.ArtistName
				}
				if !applyOp(fieldVal, rl.operator, rl.input) {
					match = false
					break
				}
			}
			if match {
				result = append(result, albumToAmp(a, base))
			}
		}
		respond(w, r, isJSON, okResp("album", result))

	case "artist":
		var query string
		for _, rl := range rules {
			if rl.ruleType == "artist" || rl.ruleType == "title" {
				query = rl.input
				break
			}
		}
		list, err := h.db.Music.SearchArtists(ctx, query, store.ListOpts{Limit: limit, Offset: offset})
		if err != nil {
			respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
			return
		}
		result := make([]AmpArtist, 0)
		for _, a := range list {
			match := true
			for _, rl := range rules {
				var fieldVal string
				switch rl.ruleType {
				case "artist", "title":
					fieldVal = a.Name
				}
				if !applyOp(fieldVal, rl.operator, rl.input) {
					match = false
					break
				}
			}
			if match {
				result = append(result, artistToAmp(a, base))
			}
		}
		respond(w, r, isJSON, okResp("artist", result))

	default:
		respond(w, r, isJSON, errResp(4710, "Bad request: unsupported type"))
	}
}

// systemUpdate triggers a library scan. Admin only.
func (h *Handler) systemUpdate(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	go func() {
		_ = h.scanner.Scan(r.Context(), scanModeFull)
	}()

	respond(w, r, isJSON, okResp("success", "scan triggered"))
}

// catalogs returns all configured music library directories as Ampache catalogs.
func (h *Handler) catalogs(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	libs, err := h.db.Music.ListLibraries(r.Context())
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	result := make([]AmpCatalog, 0, len(libs))
	for _, lib := range libs {
		enabled := 0
		if lib.Enabled {
			enabled = 1
		}
		result = append(result, AmpCatalog{
			ID:         itoa(lib.ID),
			Name:       lib.Name,
			Type:       "local",
			LastUpdate: lib.LastScanAt.Format("2006-01-02T15:04:05Z07:00"),
			LastAdd:    lib.LastScanAt.Format("2006-01-02T15:04:05Z07:00"),
			LastClean:  lib.LastScanAt.Format("2006-01-02T15:04:05Z07:00"),
			Enabled:    enabled,
			Path:       lib.Path,
		})
	}
	respond(w, r, isJSON, okResp("catalog", result))
}

// catalog returns a single catalog by ID.
func (h *Handler) catalog(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (catalog ID)"))
		return
	}

	lib, err := h.db.Music.GetLibrary(r.Context(), id)
	if err != nil || lib == nil {
		respond(w, r, isJSON, errResp(4704, "Not found"))
		return
	}

	enabled := 0
	if lib.Enabled {
		enabled = 1
	}
	respond(w, r, isJSON, AmpCatalog{
		ID:         itoa(lib.ID),
		Name:       lib.Name,
		Type:       "local",
		LastUpdate: lib.LastScanAt.Format("2006-01-02T15:04:05Z07:00"),
		LastAdd:    lib.LastScanAt.Format("2006-01-02T15:04:05Z07:00"),
		LastClean:  lib.LastScanAt.Format("2006-01-02T15:04:05Z07:00"),
		Enabled:    enabled,
		Path:       lib.Path,
	})
}

// catalogSongs returns songs belonging to the catalog (library) identified by filter.
func (h *Handler) catalogSongs(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (catalog ID)"))
		return
	}

	limit := parseIntParam(r, "limit", 500)
	offset := parseIntParam(r, "offset", 0)

	all, err := h.db.Music.SearchSongs(r.Context(), "", store.ListOpts{Limit: limit + offset, Offset: 0})
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	base := baseURL(r)
	result := make([]AmpSong, 0)
	count := 0
	for _, s := range all {
		if s.LibraryID != id {
			continue
		}
		if count < offset {
			count++
			continue
		}
		if len(result) >= limit {
			break
		}
		result = append(result, songToAmp(s, base))
		count++
	}
	respond(w, r, isJSON, okResp("song", result))
}

// catalogAlbums returns albums in the catalog identified by filter.
func (h *Handler) catalogAlbums(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (catalog ID)"))
		return
	}

	limit := parseIntParam(r, "limit", 500)
	offset := parseIntParam(r, "offset", 0)

	songs, err := h.db.Music.SearchSongs(r.Context(), "", store.ListOpts{Limit: 999999})
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	base := baseURL(r)
	seen := make(map[int64]bool)
	result := make([]AmpAlbum, 0)
	count := 0
	for _, s := range songs {
		if s.LibraryID != id {
			continue
		}
		if seen[s.AlbumID] {
			continue
		}
		seen[s.AlbumID] = true
		if count < offset {
			count++
			continue
		}
		if len(result) >= limit {
			break
		}
		album, err := h.db.Music.GetAlbum(r.Context(), s.AlbumID)
		if err == nil && album != nil {
			result = append(result, albumToAmp(album, base))
		}
		count++
	}
	respond(w, r, isJSON, okResp("album", result))
}

// catalogArtists returns artists in the catalog identified by filter.
func (h *Handler) catalogArtists(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (catalog ID)"))
		return
	}

	songs, err := h.db.Music.SearchSongs(r.Context(), "", store.ListOpts{Limit: 999999})
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	base := baseURL(r)
	seen := make(map[int64]bool)
	result := make([]AmpArtist, 0)
	for _, s := range songs {
		if s.LibraryID != id {
			continue
		}
		if seen[s.ArtistID] {
			continue
		}
		seen[s.ArtistID] = true
		artist, err := h.db.Music.GetArtist(r.Context(), s.ArtistID)
		if err == nil && artist != nil {
			result = append(result, artistToAmp(artist, base))
		}
	}
	respond(w, r, isJSON, okResp("artist", result))
}

// catalogAction triggers a scan action on the specified catalog.
func (h *Handler) catalogAction(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	go func() {
		_ = h.scanner.Scan(r.Context(), scanModeFull)
	}()

	respond(w, r, isJSON, okResp("success", "catalog action triggered"))
}

// labels lists genres as Ampache labels (genres are the closest mapping).
func (h *Handler) labels(w http.ResponseWriter, r *http.Request, isJSON bool) {
	h.genres(w, r, isJSON)
}

// label returns a single label (genre) by filter.
func (h *Handler) label(w http.ResponseWriter, r *http.Request, isJSON bool) {
	h.genre(w, r, isJSON)
}

// labelArtists returns artists for the label (genre) named by filter.
func (h *Handler) labelArtists(w http.ResponseWriter, r *http.Request, isJSON bool) {
	h.genreArtists(w, r, isJSON)
}
