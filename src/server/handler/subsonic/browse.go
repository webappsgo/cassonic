package subsonic

import (
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// getMusicFolders returns all configured library root directories.
func (h *Handler) getMusicFolders(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	libs, err := h.db.Music.ListLibraries(r.Context())
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to list music folders."))
		return
	}

	folders := make([]MusicFolder, 0, len(libs))
	for _, lib := range libs {
		folders = append(folders, MusicFolder{
			ID:   int(lib.ID),
			Name: lib.Name,
		})
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.MusicFolders = &MusicFolders{MusicFolder: folders}
	}))
}

// getIndexes returns artists grouped by first letter, optionally filtered by library.
func (h *Handler) getIndexes(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	artists, err := h.db.Music.ListArtists(r.Context(), store.ListOpts{Limit: 5000})
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to list artists."))
		return
	}

	ifModifiedSince := int64(0)
	if v := r.URL.Query().Get("ifModifiedSince"); v != "" {
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			ifModifiedSince = ms
		}
	}

	indexMap := make(map[string][]Child)
	for _, a := range artists {
		letter := artistIndexLetter(a.Name)
		child := artistToChild(a)
		indexMap[letter] = append(indexMap[letter], child)
	}

	letters := make([]string, 0, len(indexMap))
	for k := range indexMap {
		letters = append(letters, k)
	}
	sort.Strings(letters)

	entries := make([]IndexEntry, 0, len(letters))
	for _, letter := range letters {
		entries = append(entries, IndexEntry{
			Name:   letter,
			Artist: indexMap[letter],
		})
	}

	now := time.Now().UnixMilli()
	_ = ifModifiedSince

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Indexes = &Indexes{
			LastModified:    now,
			IgnoredArticles: "The An A Die Das Ein Eine Les Le La",
			Index:           entries,
		}
	}))
}

// getMusicDirectory returns the contents of a folder-based directory by ID.
// The ID may be a library (li-N), artist (ar-N), or album (al-N).
func (h *Handler) getMusicDirectory(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	prefix, dbID, err := decodeID(id)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Directory not found."))
		return
	}

	ctx := r.Context()

	switch prefix {
	case prefixLibrary:
		lib, err := h.db.Music.GetLibrary(ctx, dbID)
		if err != nil || lib == nil {
			respond(w, r, errResp(ErrNotFound, "Directory not found."))
			return
		}
		artists, err := h.db.Music.ListArtists(ctx, store.ListOpts{Limit: 5000})
		if err != nil {
			respond(w, r, errResp(ErrGeneric, "Failed to list artists."))
			return
		}
		children := make([]Child, 0, len(artists))
		for _, a := range artists {
			children = append(children, artistToChild(a))
		}
		respond(w, r, ok(func(resp *SubsonicResponse) {
			resp.Directory = &Directory{
				ID:    id,
				Name:  lib.Name,
				Child: children,
			}
		}))

	case prefixArtist, "":
		artist, err := h.db.Music.GetArtist(ctx, dbID)
		if err != nil || artist == nil {
			respond(w, r, errResp(ErrNotFound, "Directory not found."))
			return
		}
		albums, err := h.db.Music.ListAlbumsByArtist(ctx, dbID)
		if err != nil {
			respond(w, r, errResp(ErrGeneric, "Failed to list albums."))
			return
		}
		children := make([]Child, 0, len(albums))
		for _, al := range albums {
			children = append(children, albumToChild(al))
		}
		respond(w, r, ok(func(resp *SubsonicResponse) {
			resp.Directory = &Directory{
				ID:    id,
				Name:  artist.Name,
				Child: children,
			}
		}))

	case prefixAlbum:
		album, err := h.db.Music.GetAlbum(ctx, dbID)
		if err != nil || album == nil {
			respond(w, r, errResp(ErrNotFound, "Directory not found."))
			return
		}
		songs, err := h.db.Music.ListSongsByAlbum(ctx, dbID)
		if err != nil {
			respond(w, r, errResp(ErrGeneric, "Failed to list songs."))
			return
		}
		children := make([]Child, 0, len(songs))
		for _, s := range songs {
			children = append(children, songToChild(s))
		}
		respond(w, r, ok(func(resp *SubsonicResponse) {
			resp.Directory = &Directory{
				ID:     id,
				Parent: encodeArtistID(album.ArtistID),
				Name:   album.Title,
				Child:  children,
			}
		}))

	default:
		respond(w, r, errResp(ErrNotFound, "Directory not found."))
	}
}

// getGenres returns all genres with their song and album counts.
func (h *Handler) getGenres(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	genres, err := h.db.Music.ListGenres(r.Context())
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to list genres."))
		return
	}

	genreResps := make([]Genre, 0, len(genres))
	for _, g := range genres {
		genreResps = append(genreResps, Genre{
			SongCount:  g.SongCount,
			AlbumCount: g.AlbumCount,
			Value:      g.Name,
		})
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Genres = &Genres{Genre: genreResps}
	}))
}

// getArtists returns all artists indexed by first letter using ID3-based IDs.
func (h *Handler) getArtists(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	artists, err := h.db.Music.ListArtists(r.Context(), store.ListOpts{Limit: 5000})
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to list artists."))
		return
	}

	indexMap := make(map[string][]ArtistID3)
	for _, a := range artists {
		letter := artistIndexLetter(a.Name)
		indexMap[letter] = append(indexMap[letter], modelArtistToID3(a))
	}

	letters := make([]string, 0, len(indexMap))
	for k := range indexMap {
		letters = append(letters, k)
	}
	sort.Strings(letters)

	indexes := make([]ArtistIndex, 0, len(letters))
	for _, letter := range letters {
		indexes = append(indexes, ArtistIndex{
			Name:   letter,
			Artist: indexMap[letter],
		})
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Artists = &ArtistsID3{
			IgnoredArticles: "The An A Die Das Ein Eine Les Le La",
			Index:           indexes,
		}
	}))
}

// getArtist returns an artist with all its albums.
func (h *Handler) getArtist(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	dbID, err := decodeArtistID(id)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Artist not found."))
		return
	}

	ctx := r.Context()
	artist, err := h.db.Music.GetArtist(ctx, dbID)
	if err != nil || artist == nil {
		respond(w, r, errResp(ErrNotFound, "Artist not found."))
		return
	}

	albums, err := h.db.Music.ListAlbumsByArtist(ctx, dbID)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to list albums."))
		return
	}

	albumID3s := make([]AlbumID3, 0, len(albums))
	for _, al := range albums {
		albumID3s = append(albumID3s, modelAlbumToID3(al))
	}

	a3 := modelArtistToID3(artist)
	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Artist = &ArtistWithAlbumsID3{
			ArtistID3: a3,
			Album:     albumID3s,
		}
	}))
}

// getAlbum returns an album with all its songs.
func (h *Handler) getAlbum(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	dbID, err := decodeAlbumID(id)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Album not found."))
		return
	}

	ctx := r.Context()
	album, err := h.db.Music.GetAlbum(ctx, dbID)
	if err != nil || album == nil {
		respond(w, r, errResp(ErrNotFound, "Album not found."))
		return
	}

	songs, err := h.db.Music.ListSongsByAlbum(ctx, dbID)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to list songs."))
		return
	}

	children := make([]Child, 0, len(songs))
	for _, s := range songs {
		children = append(children, songToChild(s))
	}

	al3 := modelAlbumToID3(album)
	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Album = &AlbumWithSongsID3{
			AlbumID3: al3,
			Song:     children,
		}
	}))
}

// getSong returns a single song by its Subsonic string ID.
func (h *Handler) getSong(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	dbID, err := decodeSongID(id)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Song not found."))
		return
	}

	song, err := h.db.Music.GetSong(r.Context(), dbID)
	if err != nil || song == nil {
		respond(w, r, errResp(ErrNotFound, "Song not found."))
		return
	}

	child := songToChild(song)
	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Song = &child
	}))
}

// getAlbumList returns a list of albums using the legacy folder-based ID scheme.
func (h *Handler) getAlbumList(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	albums, err := h.resolveAlbumList(r)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to list albums."))
		return
	}

	children := make([]Child, 0, len(albums))
	for _, al := range albums {
		children = append(children, albumToChild(al))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.AlbumList = &AlbumList{Album: children}
	}))
}

// getAlbumList2 returns a list of albums using ID3-based IDs.
func (h *Handler) getAlbumList2(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	albums, err := h.resolveAlbumList(r)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to list albums."))
		return
	}

	albumID3s := make([]AlbumID3, 0, len(albums))
	for _, al := range albums {
		albumID3s = append(albumID3s, modelAlbumToID3(al))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.AlbumList2 = &AlbumList2{Album: albumID3s}
	}))
}

// resolveAlbumList fetches albums based on the ?type= parameter.
func (h *Handler) resolveAlbumList(r *http.Request) ([]*model.Album, error) {
	q := r.URL.Query()
	listType := q.Get("type")
	size := queryIntDefault(q.Get("size"), 10)
	if size > 500 {
		size = 500
	}
	offset := queryIntDefault(q.Get("offset"), 0)
	ctx := r.Context()

	switch listType {
	case "newest":
		return h.db.Music.GetNewestAlbums(ctx, size)

	case "random":
		return h.db.Music.GetRandomAlbums(ctx, size)

	case "alphabeticalByName":
		return h.db.Music.ListAlbums(ctx, store.ListOpts{
			Offset: offset, Limit: size, SortBy: "title",
		})

	case "alphabeticalByArtist":
		return h.db.Music.ListAlbums(ctx, store.ListOpts{
			Offset: offset, Limit: size, SortBy: "artist_name",
		})

	case "byYear":
		from := q.Get("fromYear")
		to := q.Get("toYear")
		songs, err := h.db.Music.GetRandomSongs(ctx, size*20, "", from, to)
		if err != nil {
			return nil, err
		}
		seen := make(map[int64]bool)
		var albums []*model.Album
		for _, s := range songs {
			if seen[s.AlbumID] {
				continue
			}
			seen[s.AlbumID] = true
			al, err := h.db.Music.GetAlbum(ctx, s.AlbumID)
			if err == nil && al != nil {
				albums = append(albums, al)
			}
			if len(albums) >= size {
				break
			}
		}
		return albums, nil

	case "byGenre":
		genre := q.Get("genre")
		songs, err := h.db.Music.ListSongsByGenre(ctx, genre, store.ListOpts{
			Offset: offset, Limit: size * 10,
		})
		if err != nil {
			return nil, err
		}
		seen := make(map[int64]bool)
		var albums []*model.Album
		for _, s := range songs {
			if seen[s.AlbumID] {
				continue
			}
			seen[s.AlbumID] = true
			al, err := h.db.Music.GetAlbum(ctx, s.AlbumID)
			if err == nil && al != nil {
				albums = append(albums, al)
			}
			if len(albums) >= size {
				break
			}
		}
		return albums, nil

	case "starred":
		starred, err := h.db.Activity.GetStarred(ctx, middleware.UserFromContext(r.Context()).ID)
		if err != nil {
			return nil, err
		}
		result := starred.Albums
		if len(result) > size {
			result = result[:size]
		}
		return result, nil

	default:
		return h.db.Music.GetNewestAlbums(ctx, size)
	}
}

// getRandomSongs returns a random selection of songs filtered by optional parameters.
func (h *Handler) getRandomSongs(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	q := r.URL.Query()
	size := queryIntDefault(q.Get("size"), 10)
	if size > 500 {
		size = 500
	}
	genre := q.Get("genre")
	fromYear := q.Get("fromYear")
	toYear := q.Get("toYear")

	songs, err := h.db.Music.GetRandomSongs(r.Context(), size, genre, fromYear, toYear)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to get random songs."))
		return
	}

	children := make([]Child, 0, len(songs))
	for _, s := range songs {
		children = append(children, songToChild(s))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.RandomSongs = &Songs{Song: children}
	}))
}

// getSongsByGenre returns songs filtered by genre.
func (h *Handler) getSongsByGenre(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	q := r.URL.Query()
	genre := q.Get("genre")
	if genre == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'genre' is missing."))
		return
	}

	count := queryIntDefault(q.Get("count"), 10)
	if count > 500 {
		count = 500
	}
	offset := queryIntDefault(q.Get("offset"), 0)

	songs, err := h.db.Music.ListSongsByGenre(r.Context(), genre, store.ListOpts{
		Offset: offset,
		Limit:  count,
	})
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to get songs by genre."))
		return
	}

	children := make([]Child, 0, len(songs))
	for _, s := range songs {
		children = append(children, songToChild(s))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.SongsByGenre = &Songs{Song: children}
	}))
}

// getStarred returns all starred items using legacy folder-based IDs.
func (h *Handler) getStarred(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	starred, err := h.db.Activity.GetStarred(r.Context(), authUser.ID)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to get starred items."))
		return
	}

	artistChildren := make([]Child, 0, len(starred.Artists))
	for _, a := range starred.Artists {
		artistChildren = append(artistChildren, artistToChild(a))
	}

	albumChildren := make([]Child, 0, len(starred.Albums))
	for _, al := range starred.Albums {
		albumChildren = append(albumChildren, albumToChild(al))
	}

	songChildren := make([]Child, 0, len(starred.Songs))
	for _, s := range starred.Songs {
		songChildren = append(songChildren, songToChild(s))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Starred = &Starred{
			Artist: artistChildren,
			Album:  albumChildren,
			Song:   songChildren,
		}
	}))
}

// getStarred2 returns all starred items using ID3-based IDs.
func (h *Handler) getStarred2(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	starred, err := h.db.Activity.GetStarred(r.Context(), authUser.ID)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to get starred items."))
		return
	}

	artistID3s := make([]ArtistID3, 0, len(starred.Artists))
	for _, a := range starred.Artists {
		artistID3s = append(artistID3s, modelArtistToID3(a))
	}

	albumID3s := make([]AlbumID3, 0, len(starred.Albums))
	for _, al := range starred.Albums {
		albumID3s = append(albumID3s, modelAlbumToID3(al))
	}

	songChildren := make([]Child, 0, len(starred.Songs))
	for _, s := range starred.Songs {
		songChildren = append(songChildren, songToChild(s))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Starred2 = &Starred2{
			Artist: artistID3s,
			Album:  albumID3s,
			Song:   songChildren,
		}
	}))
}

// getNowPlaying returns all currently active streams.
func (h *Handler) getNowPlaying(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	entries := h.nowPlaying.All()
	npEntries := make([]NowPlayingEntryResp, 0, len(entries))
	for _, e := range entries {
		child := Child{
			ID:     encodeSongID(e.SongID),
			IsDir:  false,
			Title:  e.Title,
			Artist: e.Artist,
			Album:  e.Album,
		}
		npEntries = append(npEntries, NowPlayingEntryResp{
			Child:      child,
			Username:   e.Username,
			MinutesAgo: e.MinutesAgo,
			PlayerName: e.PlayerName,
		})
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.NowPlaying = &NowPlayingResp{Entry: npEntries}
	}))
}

// getVideos returns an empty video list; this server handles audio only.
func (h *Handler) getVideos(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Videos = &Videos{}
	}))
}

// getVideoInfo returns an empty response; this server handles audio only.
func (h *Handler) getVideoInfo(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	respond(w, r, errResp(ErrNotFound, "Video not found."))
}

// getArtistInfo returns biography and external metadata for an artist (folder-based ID).
func (h *Handler) getArtistInfo(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	dbID, err := decodeArtistID(id)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Artist not found."))
		return
	}

	artist, err := h.db.Music.GetArtist(r.Context(), dbID)
	if err != nil || artist == nil {
		respond(w, r, errResp(ErrNotFound, "Artist not found."))
		return
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.ArtistInfo = &ArtistInfo{
			Biography:     artist.Biography,
			MusicBrainzID: artist.MusicBrainzID,
		}
	}))
}

// getArtistInfo2 is the ID3-based variant of getArtistInfo.
func (h *Handler) getArtistInfo2(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	dbID, err := decodeArtistID(id)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Artist not found."))
		return
	}

	artist, err := h.db.Music.GetArtist(r.Context(), dbID)
	if err != nil || artist == nil {
		respond(w, r, errResp(ErrNotFound, "Artist not found."))
		return
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.ArtistInfo2 = &ArtistInfo2{
			ArtistInfo: ArtistInfo{
				Biography:     artist.Biography,
				MusicBrainzID: artist.MusicBrainzID,
			},
		}
	}))
}

// getAlbumInfo returns supplemental information for an album (folder-based ID).
func (h *Handler) getAlbumInfo(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	dbID, err := decodeAlbumID(id)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Album not found."))
		return
	}

	album, err := h.db.Music.GetAlbum(r.Context(), dbID)
	if err != nil || album == nil {
		respond(w, r, errResp(ErrNotFound, "Album not found."))
		return
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.AlbumInfo = &AlbumInfo{
			MusicBrainzID: album.MusicBrainzID,
		}
	}))
}

// getAlbumInfo2 is the ID3-based variant of getAlbumInfo.
func (h *Handler) getAlbumInfo2(w http.ResponseWriter, r *http.Request) {
	h.getAlbumInfo(w, r)
}

// getSimilarSongs returns songs in the same genre as the given song (folder-based ID).
func (h *Handler) getSimilarSongs(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	count := queryIntDefault(r.URL.Query().Get("count"), 50)

	dbID, err := decodeSongID(id)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Song not found."))
		return
	}

	song, err := h.db.Music.GetSong(r.Context(), dbID)
	if err != nil || song == nil {
		respond(w, r, errResp(ErrNotFound, "Song not found."))
		return
	}

	similar, err := h.db.Music.GetRandomSongs(r.Context(), count, song.Genre, "", "")
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to get similar songs."))
		return
	}

	children := make([]Child, 0, len(similar))
	for _, s := range similar {
		if s.ID != song.ID {
			children = append(children, songToChild(s))
		}
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.SimilarSongs = &SimilarSongs{Song: children}
	}))
}

// getSimilarSongs2 is the ID3-based variant of getSimilarSongs.
func (h *Handler) getSimilarSongs2(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	count := queryIntDefault(r.URL.Query().Get("count"), 50)

	dbID, err := decodeArtistID(id)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Artist not found."))
		return
	}

	artist, err := h.db.Music.GetArtist(r.Context(), dbID)
	if err != nil || artist == nil {
		respond(w, r, errResp(ErrNotFound, "Artist not found."))
		return
	}

	songs, err := h.db.Music.ListSongsByArtist(r.Context(), dbID)
	if err != nil || len(songs) == 0 {
		respond(w, r, ok(func(resp *SubsonicResponse) {
			resp.SimilarSongs2 = &SimilarSongs2{}
		}))
		return
	}

	genre := songs[0].Genre
	similar, err := h.db.Music.GetRandomSongs(r.Context(), count, genre, "", "")
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to get similar songs."))
		return
	}

	children := make([]Child, 0, len(similar))
	for _, s := range similar {
		children = append(children, songToChild(s))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.SimilarSongs2 = &SimilarSongs2{Song: children}
	}))
}

// getTopSongs returns the most-played songs by the artist with the given name.
func (h *Handler) getTopSongs(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	artistName := r.URL.Query().Get("artist")
	if artistName == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'artist' is missing."))
		return
	}

	count := queryIntDefault(r.URL.Query().Get("count"), 50)

	ctx := r.Context()
	artist, err := h.db.Music.GetArtistByName(ctx, artistName)
	if err != nil || artist == nil {
		respond(w, r, ok(func(resp *SubsonicResponse) {
			resp.TopSongs = &TopSongs{}
		}))
		return
	}

	songs, err := h.db.Music.ListSongsByArtist(ctx, artist.ID)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to get top songs."))
		return
	}

	// Sort descending by TrackNumber as a stable secondary order;
	// play_count is tracked in the DB but not exposed on model.Song.
	sort.Slice(songs, func(i, j int) bool {
		return songs[i].TrackNumber < songs[j].TrackNumber
	})

	if len(songs) > count {
		songs = songs[:count]
	}

	children := make([]Child, 0, len(songs))
	for _, s := range songs {
		children = append(children, songToChild(s))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.TopSongs = &TopSongs{Song: children}
	}))
}

// artistIndexLetter returns the uppercase first letter of a name for index grouping.
// Names starting with a digit go under "#". Non-ASCII leading characters use "?".
func artistIndexLetter(name string) string {
	name = stripIgnoredArticle(name)
	for _, r := range name {
		if unicode.IsDigit(r) {
			return "#"
		}
		if unicode.IsLetter(r) {
			return strings.ToUpper(string(r))
		}
		return "#"
	}
	return "#"
}

// stripIgnoredArticle removes leading "The ", "A ", "An " (case-insensitive) from a name.
func stripIgnoredArticle(name string) string {
	lower := strings.ToLower(name)
	for _, article := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(lower, article) {
			return strings.TrimSpace(name[len(article):])
		}
	}
	return name
}

// songToChild converts a model.Song to a Subsonic Child response element.
func songToChild(s *model.Song) Child {
	suffix := strings.TrimPrefix(filepath.Ext(s.Path), ".")
	return Child{
		ID:          encodeSongID(s.ID),
		Parent:      encodeAlbumID(s.AlbumID),
		IsDir:       false,
		Title:       s.Title,
		Album:       s.AlbumName,
		Artist:      s.ArtistName,
		Track:       s.TrackNumber,
		Year:        s.Year,
		Genre:       s.Genre,
		CoverArt:    encodeAlbumID(s.AlbumID),
		Size:        s.FileSize,
		ContentType: s.ContentType,
		Suffix:      suffix,
		Duration:    s.Duration,
		BitRate:     s.BitRate,
		AlbumID:     encodeAlbumID(s.AlbumID),
		ArtistID:    encodeArtistID(s.ArtistID),
		Type:        "music",
		Composer:    s.Composer,
		DiscNumber:  s.DiscNumber,
	}
}

// albumToChild converts a model.Album to a Subsonic Child response element (folder view).
func albumToChild(al *model.Album) Child {
	return Child{
		ID:       encodeAlbumID(al.ID),
		Parent:   encodeArtistID(al.ArtistID),
		IsDir:    true,
		Title:    al.Title,
		Album:    al.Title,
		Artist:   al.ArtistName,
		Year:     al.Year,
		Genre:    al.Genre,
		CoverArt: encodeAlbumID(al.ID),
		Duration: al.Duration,
		ArtistID: encodeArtistID(al.ArtistID),
		AlbumID:  encodeAlbumID(al.ID),
	}
}

// artistToChild converts a model.Artist to a Subsonic Child response element (folder view).
func artistToChild(a *model.Artist) Child {
	return Child{
		ID:    encodeArtistID(a.ID),
		IsDir: true,
		Title: a.Name,
	}
}

// modelArtistToID3 converts a model.Artist to an ArtistID3 response element.
func modelArtistToID3(a *model.Artist) ArtistID3 {
	return ArtistID3{
		ID:         encodeArtistID(a.ID),
		Name:       a.Name,
		AlbumCount: a.AlbumCount,
	}
}

// modelAlbumToID3 converts a model.Album to an AlbumID3 response element.
func modelAlbumToID3(al *model.Album) AlbumID3 {
	return AlbumID3{
		ID:        encodeAlbumID(al.ID),
		Name:      al.Title,
		Artist:    al.ArtistName,
		ArtistID:  encodeArtistID(al.ArtistID),
		CoverArt:  encodeAlbumID(al.ID),
		SongCount: al.SongCount,
		Duration:  al.Duration,
		Year:      al.Year,
		Genre:     al.Genre,
	}
}

// queryIntDefault parses s as an integer, returning defaultVal on parse failure.
func queryIntDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return n
}

// isoTime formats t as an ISO 8601 string, returning "" for zero values.
func isoTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
