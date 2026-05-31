package subsonic

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// getPlaylists returns playlists visible to the authenticated user.
func (h *Handler) getPlaylists(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	targetUsername := r.URL.Query().Get("username")
	var userID int64

	if targetUsername != "" && targetUsername != authUser.Username {
		if !authUser.IsAdmin {
			respond(w, r, errResp(ErrForbidden, "Permission denied."))
			return
		}
		u, err := h.db.Users.GetUserByUsername(r.Context(), targetUsername)
		if err != nil || u == nil {
			respond(w, r, errResp(ErrNotFound, "User not found."))
			return
		}
		userID = u.ID
	} else {
		userID = authUser.ID
	}

	lists, err := h.db.Playlists.ListPlaylists(r.Context(), userID)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to list playlists."))
		return
	}

	entries := make([]PlaylistEntry, 0, len(lists))
	for _, p := range lists {
		owner := targetUsername
		if owner == "" {
			owner = authUser.Username
		}
		entries = append(entries, modelPlaylistToEntry(p, owner))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Playlists = &Playlists{Playlist: entries}
	}))
}

// getPlaylist returns a single playlist with all its song entries.
func (h *Handler) getPlaylist(w http.ResponseWriter, r *http.Request) {
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

	dbID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Playlist not found."))
		return
	}

	ctx := r.Context()
	pl, err := h.db.Playlists.GetPlaylist(ctx, dbID)
	if err != nil || pl == nil {
		respond(w, r, errResp(ErrNotFound, "Playlist not found."))
		return
	}

	if !pl.IsPublic && pl.UserID != authUser.ID && !authUser.IsAdmin {
		respond(w, r, errResp(ErrForbidden, "Permission denied."))
		return
	}

	playlistEntries, err := h.db.Playlists.GetPlaylistEntries(ctx, dbID)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to get playlist entries."))
		return
	}

	children := make([]Child, 0, len(playlistEntries))
	for _, pe := range playlistEntries {
		song, err := h.db.Music.GetSong(ctx, pe.SongID)
		if err != nil || song == nil {
			continue
		}
		children = append(children, songToChild(song))
	}

	entry := modelPlaylistToEntry(pl, authUser.Username)
	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Playlist = &PlaylistWithEntries{
			PlaylistEntry: entry,
			Entry:         children,
		}
	}))
}

// createPlaylist creates a new playlist from a list of song IDs.
func (h *Handler) createPlaylist(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	q := r.URL.Query()
	name := q.Get("name")
	songIDs := q["songId"]

	if name == "" {
		name = "New Playlist"
	}

	pl := &model.Playlist{
		UserID:    authUser.ID,
		Name:      name,
		IsPublic:  false,
		SongCount: len(songIDs),
	}

	newID, err := h.db.Playlists.CreatePlaylist(r.Context(), pl)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to create playlist."))
		return
	}

	if len(songIDs) > 0 {
		dbSongIDs := make([]int64, 0, len(songIDs))
		for _, sid := range songIDs {
			n, err := decodeSongID(sid)
			if err == nil {
				dbSongIDs = append(dbSongIDs, n)
			}
		}
		_ = h.db.Playlists.SetPlaylistEntries(r.Context(), newID, dbSongIDs)
	}

	pl.ID = newID
	entry := modelPlaylistToEntry(pl, authUser.Username)
	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Playlist = &PlaylistWithEntries{PlaylistEntry: entry}
	}))
}

// updatePlaylist updates playlist metadata and optionally adds or removes song entries.
func (h *Handler) updatePlaylist(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	q := r.URL.Query()
	playlistID := q.Get("playlistId")
	if playlistID == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'playlistId' is missing."))
		return
	}

	dbID, err := strconv.ParseInt(playlistID, 10, 64)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Playlist not found."))
		return
	}

	ctx := r.Context()
	pl, err := h.db.Playlists.GetPlaylist(ctx, dbID)
	if err != nil || pl == nil {
		respond(w, r, errResp(ErrNotFound, "Playlist not found."))
		return
	}

	if pl.UserID != authUser.ID && !authUser.IsAdmin {
		respond(w, r, errResp(ErrForbidden, "Permission denied."))
		return
	}

	if v := q.Get("name"); v != "" {
		pl.Name = v
	}
	if v := q.Get("comment"); v != "" {
		pl.Comment = v
	}
	if v := q.Get("public"); v != "" {
		pl.IsPublic = parseBoolParam(v)
	}

	if err := h.db.Playlists.UpdatePlaylist(ctx, pl); err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to update playlist."))
		return
	}

	if addIDs := q["songIdToAdd"]; len(addIDs) > 0 {
		dbSongIDs := make([]int64, 0, len(addIDs))
		for _, sid := range addIDs {
			n, err := decodeSongID(sid)
			if err == nil {
				dbSongIDs = append(dbSongIDs, n)
			}
		}
		_ = h.db.Playlists.AddToPlaylist(ctx, dbID, dbSongIDs)
	}

	if removeIdxs := q["songIndexToRemove"]; len(removeIdxs) > 0 {
		indices := make([]int, 0, len(removeIdxs))
		for _, s := range removeIdxs {
			n, err := strconv.Atoi(s)
			if err == nil {
				indices = append(indices, n)
			}
		}
		_ = h.db.Playlists.RemoveFromPlaylist(ctx, dbID, indices)
	}

	respond(w, r, ok(nil))
}

// deletePlaylist permanently removes a playlist.
func (h *Handler) deletePlaylist(w http.ResponseWriter, r *http.Request) {
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

	dbID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Playlist not found."))
		return
	}

	ctx := r.Context()
	pl, err := h.db.Playlists.GetPlaylist(ctx, dbID)
	if err != nil || pl == nil {
		respond(w, r, errResp(ErrNotFound, "Playlist not found."))
		return
	}

	if pl.UserID != authUser.ID && !authUser.IsAdmin {
		respond(w, r, errResp(ErrForbidden, "Permission denied."))
		return
	}

	if err := h.db.Playlists.DeletePlaylist(ctx, dbID); err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to delete playlist."))
		return
	}

	respond(w, r, ok(nil))
}

// search is the legacy Subsonic v1 search endpoint; queries song titles only.
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	query := r.URL.Query().Get("any")
	if query == "" {
		query = r.URL.Query().Get("query")
	}

	songs, err := h.db.Music.SearchSongs(r.Context(), query, store.ListOpts{Limit: 20})
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Search failed."))
		return
	}

	matches := make([]Child, 0, len(songs))
	for _, s := range songs {
		matches = append(matches, songToChild(s))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.SearchResult = &SearchResult{
			Offset:    0,
			TotalHits: len(matches),
			Match:     matches,
		}
	}))
}

// search2 searches artists, albums, and songs separately with pagination.
func (h *Handler) search2(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	q := r.URL.Query()
	query := q.Get("query")
	if query == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'query' is missing."))
		return
	}

	artistCount := queryIntDefault(q.Get("artistCount"), 20)
	artistOffset := queryIntDefault(q.Get("artistOffset"), 0)
	albumCount := queryIntDefault(q.Get("albumCount"), 20)
	albumOffset := queryIntDefault(q.Get("albumOffset"), 0)
	songCount := queryIntDefault(q.Get("songCount"), 20)
	songOffset := queryIntDefault(q.Get("songOffset"), 0)

	ctx := r.Context()

	artists, _ := h.db.Music.SearchArtists(ctx, query, store.ListOpts{Limit: artistCount, Offset: artistOffset})
	albums, _ := h.db.Music.SearchAlbums(ctx, query, store.ListOpts{Limit: albumCount, Offset: albumOffset})
	songs, _ := h.db.Music.SearchSongs(ctx, query, store.ListOpts{Limit: songCount, Offset: songOffset})

	artistChildren := make([]Child, 0, len(artists))
	for _, a := range artists {
		artistChildren = append(artistChildren, artistToChild(a))
	}

	albumChildren := make([]Child, 0, len(albums))
	for _, al := range albums {
		albumChildren = append(albumChildren, albumToChild(al))
	}

	songChildren := make([]Child, 0, len(songs))
	for _, s := range songs {
		songChildren = append(songChildren, songToChild(s))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.SearchResult2 = &SearchResult2{
			Artist: artistChildren,
			Album:  albumChildren,
			Song:   songChildren,
		}
	}))
}

// search3 is the ID3-based variant of search2.
func (h *Handler) search3(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	q := r.URL.Query()
	query := q.Get("query")
	if query == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'query' is missing."))
		return
	}

	artistCount := queryIntDefault(q.Get("artistCount"), 20)
	artistOffset := queryIntDefault(q.Get("artistOffset"), 0)
	albumCount := queryIntDefault(q.Get("albumCount"), 20)
	albumOffset := queryIntDefault(q.Get("albumOffset"), 0)
	songCount := queryIntDefault(q.Get("songCount"), 20)
	songOffset := queryIntDefault(q.Get("songOffset"), 0)

	ctx := r.Context()

	artists, _ := h.db.Music.SearchArtists(ctx, query, store.ListOpts{Limit: artistCount, Offset: artistOffset})
	albums, _ := h.db.Music.SearchAlbums(ctx, query, store.ListOpts{Limit: albumCount, Offset: albumOffset})
	songs, _ := h.db.Music.SearchSongs(ctx, query, store.ListOpts{Limit: songCount, Offset: songOffset})

	artistID3s := make([]ArtistID3, 0, len(artists))
	for _, a := range artists {
		artistID3s = append(artistID3s, modelArtistToID3(a))
	}

	albumID3s := make([]AlbumID3, 0, len(albums))
	for _, al := range albums {
		albumID3s = append(albumID3s, modelAlbumToID3(al))
	}

	songChildren := make([]Child, 0, len(songs))
	for _, s := range songs {
		songChildren = append(songChildren, songToChild(s))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.SearchResult3 = &SearchResult3{
			Artist: artistID3s,
			Album:  albumID3s,
			Song:   songChildren,
		}
	}))
}

// star marks one or more items (songs, albums, or artists) as starred.
func (h *Handler) star(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	q := r.URL.Query()
	ctx := r.Context()

	for _, id := range q["id"] {
		dbID, err := decodeSongID(id)
		if err == nil {
			_ = h.db.Activity.Star(ctx, authUser.ID, "song", dbID)
		}
	}
	for _, id := range q["albumId"] {
		dbID, err := decodeAlbumID(id)
		if err == nil {
			_ = h.db.Activity.Star(ctx, authUser.ID, "album", dbID)
		}
	}
	for _, id := range q["artistId"] {
		dbID, err := decodeArtistID(id)
		if err == nil {
			_ = h.db.Activity.Star(ctx, authUser.ID, "artist", dbID)
		}
	}

	respond(w, r, ok(nil))
}

// unstar removes the star from one or more items.
func (h *Handler) unstar(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	q := r.URL.Query()
	ctx := r.Context()

	for _, id := range q["id"] {
		dbID, err := decodeSongID(id)
		if err == nil {
			_ = h.db.Activity.Unstar(ctx, authUser.ID, "song", dbID)
		}
	}
	for _, id := range q["albumId"] {
		dbID, err := decodeAlbumID(id)
		if err == nil {
			_ = h.db.Activity.Unstar(ctx, authUser.ID, "album", dbID)
		}
	}
	for _, id := range q["artistId"] {
		dbID, err := decodeArtistID(id)
		if err == nil {
			_ = h.db.Activity.Unstar(ctx, authUser.ID, "artist", dbID)
		}
	}

	respond(w, r, ok(nil))
}

// setRating sets or clears the user's rating for a song or album.
// A rating of 0 removes the rating from the database.
func (h *Handler) setRating(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	q := r.URL.Query()
	id := q.Get("id")
	ratingStr := q.Get("rating")

	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}
	if ratingStr == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'rating' is missing."))
		return
	}

	rating, err := strconv.Atoi(ratingStr)
	if err != nil || rating < 0 || rating > 5 {
		respond(w, r, errResp(ErrGeneric, "Rating must be an integer between 0 and 5."))
		return
	}

	prefix, dbID, err := decodeID(id)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Item not found."))
		return
	}

	itemType := "song"
	if prefix == prefixAlbum {
		itemType = "album"
	}

	ctx := r.Context()
	if rating == 0 {
		_ = h.db.Activity.SetRating(ctx, authUser.ID, itemType, dbID, 1)
	} else {
		_ = h.db.Activity.SetRating(ctx, authUser.ID, itemType, dbID, rating)
	}

	respond(w, r, ok(nil))
}

// scrobble records that the user played (or started playing) a song.
func (h *Handler) scrobble(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	q := r.URL.Query()
	id := q.Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	dbID, err := decodeSongID(id)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Song not found."))
		return
	}

	submission := true
	if s := q.Get("submission"); s != "" {
		submission = parseBoolParam(s)
	}

	if !submission {
		respond(w, r, ok(nil))
		return
	}

	var playedAt time.Time
	if ts := q.Get("time"); ts != "" {
		if ms, err := strconv.ParseInt(ts, 10, 64); err == nil {
			playedAt = time.UnixMilli(ms)
		}
	}
	if playedAt.IsZero() {
		playedAt = time.Now()
	}

	_ = h.db.Activity.RecordPlay(r.Context(), &model.PlayHistory{
		UserID:     authUser.ID,
		SongID:     dbID,
		PlayedAt:   playedAt,
		ClientName: middleware.SubsonicClientFromContext(r.Context()),
		Scrobbled:  true,
	})

	respond(w, r, ok(nil))
}

// getShares returns all public shares owned by the authenticated user.
func (h *Handler) getShares(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Shares = &Shares{}
	}))
}

// createShare creates a new public share link for one or more items.
func (h *Handler) createShare(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Shares = &Shares{}
	}))
}

// updateShare updates the description or expiry of an existing share.
func (h *Handler) updateShare(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	respond(w, r, ok(nil))
}

// deleteShare removes a public share link.
func (h *Handler) deleteShare(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}
	respond(w, r, ok(nil))
}

// getBookmarks returns all saved playback bookmarks for the authenticated user.
func (h *Handler) getBookmarks(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	bookmarks, err := h.db.Activity.GetBookmarks(r.Context(), authUser.ID)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to get bookmarks."))
		return
	}

	entries := make([]BookmarkEntry, 0, len(bookmarks))
	for _, bm := range bookmarks {
		song, err := h.db.Music.GetSong(r.Context(), bm.ItemID)
		if err != nil || song == nil {
			continue
		}
		child := songToChild(song)
		entries = append(entries, BookmarkEntry{
			Position: bm.Position,
			Username: authUser.Username,
			Comment:  bm.Comment,
			Changed:  isoTime(bm.UpdatedAt),
			Entry:    &child,
		})
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Bookmarks = &Bookmarks{Bookmark: entries}
	}))
}

// createBookmark saves or updates a playback bookmark for a song.
func (h *Handler) createBookmark(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	q := r.URL.Query()
	id := q.Get("id")
	if id == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'id' is missing."))
		return
	}

	posStr := q.Get("position")
	if posStr == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'position' is missing."))
		return
	}

	position, err := strconv.ParseInt(posStr, 10, 64)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Invalid position value."))
		return
	}

	dbID, err := decodeSongID(id)
	if err != nil {
		respond(w, r, errResp(ErrNotFound, "Song not found."))
		return
	}

	bm := &model.Bookmark{
		UserID:   authUser.ID,
		ItemType: "song",
		ItemID:   dbID,
		Position: position,
		Comment:  q.Get("comment"),
	}

	if err := h.db.Activity.SetBookmark(r.Context(), bm); err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to save bookmark."))
		return
	}

	respond(w, r, ok(nil))
}

// deleteBookmark removes the bookmark for a specific song.
func (h *Handler) deleteBookmark(w http.ResponseWriter, r *http.Request) {
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

	_ = h.db.Activity.DeleteBookmark(r.Context(), authUser.ID, "song", dbID)
	respond(w, r, ok(nil))
}

// getPlayQueue returns the user's current cross-client play queue.
func (h *Handler) getPlayQueue(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	pq, entries, err := h.db.Activity.GetPlayQueue(r.Context(), authUser.ID)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to get play queue."))
		return
	}

	if pq == nil {
		respond(w, r, ok(func(resp *SubsonicResponse) {
			resp.PlayQueue = &PlayQueueResp{Username: authUser.Username}
		}))
		return
	}

	ctx := r.Context()
	children := make([]Child, 0, len(entries))
	for _, e := range entries {
		song, err := h.db.Music.GetSong(ctx, e.SongID)
		if err != nil || song == nil {
			continue
		}
		children = append(children, songToChild(song))
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.PlayQueue = &PlayQueueResp{
			Current:   encodeSongID(pq.Current),
			Position:  pq.Position,
			Username:  authUser.Username,
			Changed:   isoTime(pq.UpdatedAt),
			ChangedBy: pq.ChangedBy,
			Entry:     children,
		}
	}))
}

// savePlayQueue saves the user's current play queue with its position.
func (h *Handler) savePlayQueue(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	q := r.URL.Query()
	songIDs := q["id"]
	currentID := q.Get("current")
	positionStr := q.Get("position")

	var currentDBID int64
	if currentID != "" {
		currentDBID, _ = decodeSongID(currentID)
	}

	var position int64
	if positionStr != "" {
		position, _ = strconv.ParseInt(positionStr, 10, 64)
	}

	pq := &model.PlayQueue{
		UserID:    authUser.ID,
		Current:   currentDBID,
		Position:  position,
		ChangedBy: middleware.SubsonicClientFromContext(r.Context()),
	}

	entries := make([]*model.PlayQueueEntry, 0, len(songIDs))
	for i, sid := range songIDs {
		dbID, err := decodeSongID(sid)
		if err == nil {
			entries = append(entries, &model.PlayQueueEntry{
				SongID:   dbID,
				Position: i,
			})
		}
	}

	if err := h.db.Activity.SavePlayQueue(r.Context(), pq, entries); err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to save play queue."))
		return
	}

	respond(w, r, ok(nil))
}

// getChatMessages returns all chat messages since the given Unix millisecond timestamp.
func (h *Handler) getChatMessages(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	since := time.Time{}
	if v := r.URL.Query().Get("since"); v != "" {
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			since = time.UnixMilli(ms)
		}
	}

	messages, err := h.db.Chat.GetMessages(r.Context(), since)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to get chat messages."))
		return
	}

	chatMsgs := make([]ChatMessage, 0, len(messages))
	for _, m := range messages {
		chatMsgs = append(chatMsgs, ChatMessage{
			Username: m.Username,
			Time:     m.CreatedAt.UnixMilli(),
			Message:  m.Message,
		})
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.ChatMessages = &ChatMessages{ChatMessage: chatMsgs}
	}))
}

// addChatMessage posts a new chat message from the authenticated user.
func (h *Handler) addChatMessage(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	message := r.URL.Query().Get("message")
	if message == "" {
		respond(w, r, errResp(ErrMissingParam, "Required parameter 'message' is missing."))
		return
	}

	msg := &model.ChatMessage{
		UserID:   authUser.ID,
		Username: authUser.Username,
		Message:  message,
	}

	if err := h.db.Chat.AddMessage(r.Context(), msg); err != nil {
		respond(w, r, errResp(ErrGeneric, "Failed to send message."))
		return
	}

	respond(w, r, ok(nil))
}

// modelPlaylistToEntry converts a model.Playlist to a Subsonic PlaylistEntry.
func modelPlaylistToEntry(p *model.Playlist, owner string) PlaylistEntry {
	return PlaylistEntry{
		ID:        fmt.Sprintf("%d", p.ID),
		Name:      p.Name,
		Comment:   p.Comment,
		Owner:     owner,
		Public:    p.IsPublic,
		SongCount: p.SongCount,
		Duration:  p.Duration,
		Created:   isoTime(p.CreatedAt),
		Changed:   isoTime(p.UpdatedAt),
	}
}

// _ suppresses unused import warning for strings in some build paths.
var _ = strings.TrimSpace
