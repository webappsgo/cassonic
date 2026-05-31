package api

import (
	"net/http"
	"strconv"

	"github.com/local/cassonic/src/server/store"
	cerr "github.com/local/cassonic/src/common/errors"
)

// ListArtists returns a paginated list of artists with optional search and sort.
func (h *Handler) ListArtists(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	q := r.URL.Query().Get("search")
	sortBy := r.URL.Query().Get("sort")
	desc := r.URL.Query().Get("order") == "desc"

	opts := store.ListOpts{
		Limit:  limit,
		Offset: offset,
		SortBy: sortBy,
		Desc:   desc,
	}

	var artists interface{}
	var err error

	if q != "" {
		artists, err = h.db.Music.SearchArtists(r.Context(), q, opts)
	} else {
		artists, err = h.db.Music.ListArtists(r.Context(), opts)
	}
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list artists failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"artists": artists,
	})
}

// GetArtist returns a single artist with their albums.
func (h *Handler) GetArtist(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid artist id"))
		return
	}

	artist, err := h.db.Music.GetArtist(r.Context(), id)
	if err != nil || artist == nil {
		writeError(w, r, cerr.NotFound("artist not found"))
		return
	}

	albums, err := h.db.Music.ListAlbumsByArtist(r.Context(), id)
	if err != nil {
		albums = nil
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"artist": artist,
		"albums": albums,
	})
}

// ListAlbums returns a paginated list of albums with optional filters.
func (h *Handler) ListAlbums(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	sortBy := r.URL.Query().Get("sort")
	desc := r.URL.Query().Get("order") == "desc"

	opts := store.ListOpts{
		Limit:  limit,
		Offset: offset,
		SortBy: sortBy,
		Desc:   desc,
	}

	q := r.URL.Query().Get("search")
	artistIDStr := r.URL.Query().Get("artist_id")

	if q != "" {
		albums, err := h.db.Music.SearchAlbums(r.Context(), q, opts)
		if err != nil {
			writeError(w, r, cerr.InternalServerError("search albums failed"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"albums": albums})
		return
	}

	if artistIDStr != "" {
		artistID, err := strconv.ParseInt(artistIDStr, 10, 64)
		if err != nil || artistID <= 0 {
			writeError(w, r, cerr.BadRequest("invalid artist_id"))
			return
		}
		albums, err := h.db.Music.ListAlbumsByArtist(r.Context(), artistID)
		if err != nil {
			writeError(w, r, cerr.InternalServerError("list albums failed"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"albums": albums})
		return
	}

	albums, err := h.db.Music.ListAlbums(r.Context(), opts)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list albums failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"albums": albums})
}

// GetAlbum returns a single album with its songs.
func (h *Handler) GetAlbum(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid album id"))
		return
	}

	album, err := h.db.Music.GetAlbum(r.Context(), id)
	if err != nil || album == nil {
		writeError(w, r, cerr.NotFound("album not found"))
		return
	}

	songs, err := h.db.Music.ListSongsByAlbum(r.Context(), id)
	if err != nil {
		songs = nil
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"album": album,
		"songs": songs,
	})
}

// ListSongs returns a paginated list of songs with optional filters.
func (h *Handler) ListSongs(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	opts := store.ListOpts{Limit: limit, Offset: offset}

	q := r.URL.Query().Get("search")
	albumIDStr := r.URL.Query().Get("album_id")
	artistIDStr := r.URL.Query().Get("artist_id")
	genre := r.URL.Query().Get("genre")

	if q != "" {
		songs, err := h.db.Music.SearchSongs(r.Context(), q, opts)
		if err != nil {
			writeError(w, r, cerr.InternalServerError("search songs failed"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"songs": songs})
		return
	}

	if albumIDStr != "" {
		albumID, err := strconv.ParseInt(albumIDStr, 10, 64)
		if err != nil || albumID <= 0 {
			writeError(w, r, cerr.BadRequest("invalid album_id"))
			return
		}
		songs, err := h.db.Music.ListSongsByAlbum(r.Context(), albumID)
		if err != nil {
			writeError(w, r, cerr.InternalServerError("list songs failed"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"songs": songs})
		return
	}

	if artistIDStr != "" {
		artistID, err := strconv.ParseInt(artistIDStr, 10, 64)
		if err != nil || artistID <= 0 {
			writeError(w, r, cerr.BadRequest("invalid artist_id"))
			return
		}
		songs, err := h.db.Music.ListSongsByArtist(r.Context(), artistID)
		if err != nil {
			writeError(w, r, cerr.InternalServerError("list songs failed"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"songs": songs})
		return
	}

	if genre != "" {
		songs, err := h.db.Music.ListSongsByGenre(r.Context(), genre, opts)
		if err != nil {
			writeError(w, r, cerr.InternalServerError("list songs by genre failed"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"songs": songs})
		return
	}

	songs, err := h.db.Music.SearchSongs(r.Context(), "", opts)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list songs failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"songs": songs})
}

// GetSong returns a single song by ID.
func (h *Handler) GetSong(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid song id"))
		return
	}

	song, err := h.db.Music.GetSong(r.Context(), id)
	if err != nil || song == nil {
		writeError(w, r, cerr.NotFound("song not found"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"song": song})
}

// ListGenres returns all genres with their song and album counts.
func (h *Handler) ListGenres(w http.ResponseWriter, r *http.Request) {
	genres, err := h.db.Music.ListGenres(r.Context())
	if err != nil {
		writeError(w, r, cerr.InternalServerError("list genres failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"genres": genres,
		"total":  len(genres),
	})
}

// Search performs a unified search across artists, albums, and songs.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, r, cerr.BadRequest("q parameter is required"))
		return
	}

	limit, offset := parsePagination(r)
	opts := store.ListOpts{Limit: limit, Offset: offset}

	artists, err := h.db.Music.SearchArtists(r.Context(), q, opts)
	if err != nil {
		artists = nil
	}

	albums, err := h.db.Music.SearchAlbums(r.Context(), q, opts)
	if err != nil {
		albums = nil
	}

	songs, err := h.db.Music.SearchSongs(r.Context(), q, opts)
	if err != nil {
		songs = nil
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"artists": artists,
		"albums":  albums,
		"songs":   songs,
	})
}
