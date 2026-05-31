package ampache

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// playlists returns all playlists visible to the authenticated user.
func (h *Handler) playlists(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	list, err := h.db.Playlists.ListPlaylists(r.Context(), session.UserID)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	base := baseURL(r)
	result := make([]AmpPlaylist, 0, len(list))
	for _, p := range list {
		result = append(result, playlistToAmp(p, session.UserID, base, h))
	}
	respond(w, r, isJSON, okResp("playlist", result))
}

// playlist returns a single playlist by ID (filter parameter).
func (h *Handler) playlist(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (playlist ID)"))
		return
	}

	p, err := h.db.Playlists.GetPlaylist(r.Context(), id)
	if err != nil || p == nil {
		respond(w, r, isJSON, errResp(4704, "Not found"))
		return
	}

	if !p.IsPublic && p.UserID != session.UserID {
		user, _ := h.db.Users.GetUser(r.Context(), session.UserID)
		if user == nil || !user.IsAdmin {
			respond(w, r, isJSON, errResp(4742, "Access denied"))
			return
		}
	}

	respond(w, r, isJSON, playlistToAmp(p, session.UserID, baseURL(r), h))
}

// playlistSongs returns the ordered songs in a playlist identified by filter.
func (h *Handler) playlistSongs(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (playlist ID)"))
		return
	}

	p, err := h.db.Playlists.GetPlaylist(r.Context(), id)
	if err != nil || p == nil {
		respond(w, r, isJSON, errResp(4704, "Not found"))
		return
	}

	if !p.IsPublic && p.UserID != session.UserID {
		user, _ := h.db.Users.GetUser(r.Context(), session.UserID)
		if user == nil || !user.IsAdmin {
			respond(w, r, isJSON, errResp(4742, "Access denied"))
			return
		}
	}

	entries, err := h.db.Playlists.GetPlaylistEntries(r.Context(), id)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	base := baseURL(r)
	result := make([]AmpSong, 0, len(entries))
	for _, e := range entries {
		song, err := h.db.Music.GetSong(r.Context(), e.SongID)
		if err != nil || song == nil {
			continue
		}
		result = append(result, songToAmp(song, base))
	}
	respond(w, r, isJSON, okResp("song", result))
}

// playlistCreate creates a new playlist.
func (h *Handler) playlistCreate(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	name := param(r, "name")
	if name == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: name"))
		return
	}

	plType := param(r, "type")
	isPublic := plType == "public"

	p := &model.Playlist{
		UserID:   session.UserID,
		Name:     name,
		IsPublic: isPublic,
	}

	id, err := h.db.Playlists.CreatePlaylist(r.Context(), p)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to create playlist: "+err.Error()))
		return
	}

	p.ID = id
	respond(w, r, isJSON, playlistToAmp(p, session.UserID, baseURL(r), h))
}

// playlistEdit modifies an existing playlist's metadata and/or entries.
func (h *Handler) playlistEdit(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (playlist ID)"))
		return
	}

	p, err := h.db.Playlists.GetPlaylist(r.Context(), id)
	if err != nil || p == nil {
		respond(w, r, isJSON, errResp(4704, "Not found"))
		return
	}

	if p.UserID != session.UserID {
		user, _ := h.db.Users.GetUser(r.Context(), session.UserID)
		if user == nil || !user.IsAdmin {
			respond(w, r, isJSON, errResp(4742, "Access denied"))
			return
		}
	}

	if name := param(r, "name"); name != "" {
		p.Name = name
	}

	if plType := param(r, "type"); plType != "" {
		p.IsPublic = plType == "public"
	}

	if err := h.db.Playlists.UpdatePlaylist(r.Context(), p); err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to update playlist: "+err.Error()))
		return
	}

	items := param(r, "items")
	if items != "" {
		parts := strings.Split(items, ",")
		songIDs := make([]int64, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			n, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				continue
			}
			songIDs = append(songIDs, n)
		}
		if err := h.db.Playlists.SetPlaylistEntries(r.Context(), id, songIDs); err != nil {
			respond(w, r, isJSON, errResp(4710, "Failed to set entries: "+err.Error()))
			return
		}
	}

	respond(w, r, isJSON, okResp("success", "playlist updated"))
}

// playlistDelete permanently removes a playlist.
func (h *Handler) playlistDelete(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (playlist ID)"))
		return
	}

	p, err := h.db.Playlists.GetPlaylist(r.Context(), id)
	if err != nil || p == nil {
		respond(w, r, isJSON, errResp(4704, "Not found"))
		return
	}

	if p.UserID != session.UserID {
		user, _ := h.db.Users.GetUser(r.Context(), session.UserID)
		if user == nil || !user.IsAdmin {
			respond(w, r, isJSON, errResp(4742, "Access denied"))
			return
		}
	}

	if err := h.db.Playlists.DeletePlaylist(r.Context(), id); err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to delete playlist: "+err.Error()))
		return
	}

	respond(w, r, isJSON, okResp("success", "playlist deleted"))
}

// playlistAddSong appends a song to a playlist, with optional duplicate check.
func (h *Handler) playlistAddSong(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	playlistID := parseIDParam(r, "filter")
	if playlistID == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (playlist ID)"))
		return
	}

	songID := parseIDParam(r, "song_id")
	if songID == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: song_id"))
		return
	}

	p, err := h.db.Playlists.GetPlaylist(r.Context(), playlistID)
	if err != nil || p == nil {
		respond(w, r, isJSON, errResp(4704, "Playlist not found"))
		return
	}

	if p.UserID != session.UserID {
		user, _ := h.db.Users.GetUser(r.Context(), session.UserID)
		if user == nil || !user.IsAdmin {
			respond(w, r, isJSON, errResp(4742, "Access denied"))
			return
		}
	}

	checkDups := parseIntParam(r, "check", 0)
	if checkDups == 1 {
		entries, err := h.db.Playlists.GetPlaylistEntries(r.Context(), playlistID)
		if err == nil {
			for _, e := range entries {
				if e.SongID == songID {
					respond(w, r, isJSON, okResp("success", "song already in playlist"))
					return
				}
			}
		}
	}

	if err := h.db.Playlists.AddToPlaylist(r.Context(), playlistID, []int64{songID}); err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to add song: "+err.Error()))
		return
	}

	respond(w, r, isJSON, okResp("success", "song added to playlist"))
}

// playlistRemoveSong removes the entry at the given 1-indexed position from a playlist.
func (h *Handler) playlistRemoveSong(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	playlistID := parseIDParam(r, "filter")
	if playlistID == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (playlist ID)"))
		return
	}

	track := parseIntParam(r, "track", 0)
	if track == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: track"))
		return
	}

	p, err := h.db.Playlists.GetPlaylist(r.Context(), playlistID)
	if err != nil || p == nil {
		respond(w, r, isJSON, errResp(4704, "Playlist not found"))
		return
	}

	if p.UserID != session.UserID {
		user, _ := h.db.Users.GetUser(r.Context(), session.UserID)
		if user == nil || !user.IsAdmin {
			respond(w, r, isJSON, errResp(4742, "Access denied"))
			return
		}
	}

	// track is 1-indexed; RemoveFromPlaylist uses 0-indexed positions.
	if err := h.db.Playlists.RemoveFromPlaylist(r.Context(), playlistID, []int{track - 1}); err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to remove song: "+err.Error()))
		return
	}

	respond(w, r, isJSON, okResp("success", "song removed from playlist"))
}

// playlistGenerate creates or returns a dynamically generated playlist.
func (h *Handler) playlistGenerate(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	ctx := r.Context()
	mode := param(r, "mode")
	if mode == "" {
		mode = "random"
	}
	limit := parseIntParam(r, "limit", 25)
	genre := param(r, "genre")

	var songs []*model.Song
	var err error

	switch mode {
	case "random":
		songs, err = h.db.Music.GetRandomSongs(ctx, limit, genre, "", "")
	case "unplayed":
		songs, err = h.db.Music.GetRandomSongs(ctx, limit, genre, "", "")
	case "recent":
		songs, err = h.db.Music.GetRandomSongs(ctx, limit, genre, "", "")
	default:
		songs, err = h.db.Music.GetRandomSongs(ctx, limit, genre, "", "")
	}

	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to generate playlist: "+err.Error()))
		return
	}

	base := baseURL(r)
	result := make([]AmpSong, 0, len(songs))
	for _, s := range songs {
		result = append(result, songToAmp(s, base))
	}
	respond(w, r, isJSON, okResp("song", result))
}

// flag stars or unstars an item for the authenticated user.
func (h *Handler) flag(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	itemType := param(r, "type")
	id := parseIDParam(r, "id")
	flagVal := parseIntParam(r, "flag", 0)

	if itemType == "" || id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: type and id"))
		return
	}

	var err error
	if flagVal == 1 {
		err = h.db.Activity.Star(r.Context(), session.UserID, itemType, id)
	} else {
		err = h.db.Activity.Unstar(r.Context(), session.UserID, itemType, id)
	}

	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to update flag: "+err.Error()))
		return
	}

	respond(w, r, isJSON, okResp("success", "flag updated"))
}

// rate sets a 0–5 star rating for an item.
func (h *Handler) rate(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	itemType := param(r, "type")
	id := parseIDParam(r, "id")
	rating := parseIntParam(r, "rating", 0)

	if itemType == "" || id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: type and id"))
		return
	}

	if rating < 0 || rating > 5 {
		respond(w, r, isJSON, errResp(4710, "Rating must be 0-5"))
		return
	}

	if err := h.db.Activity.SetRating(r.Context(), session.UserID, itemType, id, rating); err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to set rating: "+err.Error()))
		return
	}

	respond(w, r, isJSON, okResp("success", "rating updated"))
}

// recordPlay records a manual play event for the authenticated user.
func (h *Handler) recordPlay(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	songID := parseIDParam(r, "id")
	if songID == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: id"))
		return
	}

	client := param(r, "client")
	if client == "" {
		client = "ampache"
	}

	dateStr := param(r, "date")
	playedAt := time.Now()
	if dateStr != "" {
		if ts, err := strconv.ParseInt(dateStr, 10, 64); err == nil {
			playedAt = time.Unix(ts, 0)
		}
	}

	h2 := &model.PlayHistory{
		UserID:     session.UserID,
		SongID:     songID,
		PlayedAt:   playedAt,
		ClientName: client,
	}

	if err := h.db.Activity.RecordPlay(r.Context(), h2); err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to record play: "+err.Error()))
		return
	}

	_ = h.db.Music.IncrementPlayCount(r.Context(), songID)

	respond(w, r, isJSON, okResp("success", "play recorded"))
}

// scrobble records a play based on song metadata (for external scrobble clients).
func (h *Handler) scrobble(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	songTitle := param(r, "song")
	artistName := param(r, "artist")
	client := param(r, "client")
	if client == "" {
		client = "ampache"
	}

	dateStr := param(r, "date")
	playedAt := time.Now()
	if dateStr != "" {
		if ts, err := strconv.ParseInt(dateStr, 10, 64); err == nil {
			playedAt = time.Unix(ts, 0)
		}
	}

	ctx := r.Context()

	songs, err := h.db.Music.SearchSongs(ctx, songTitle, store.ListOpts{Limit: 20})
	if err == nil {
		for _, s := range songs {
			if strings.EqualFold(s.Title, songTitle) && strings.EqualFold(s.ArtistName, artistName) {
				h2 := &model.PlayHistory{
					UserID:     session.UserID,
					SongID:     s.ID,
					PlayedAt:   playedAt,
					ClientName: client,
					Scrobbled:  true,
				}
				_ = h.db.Activity.RecordPlay(ctx, h2)
				_ = h.db.Music.IncrementPlayCount(ctx, s.ID)
				break
			}
		}
	}

	respond(w, r, isJSON, okResp("success", "scrobble recorded"))
}

// nowPlaying returns the list of currently active streams. This implementation
// returns an empty list as per-session stream tracking is not implemented.
func (h *Handler) nowPlaying(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("now_playing", []AmpNowPlaying{}))
}

// getSimilar returns songs with the same genre as the given song or artist.
func (h *Handler) getSimilar(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "id")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: id"))
		return
	}

	itemType := param(r, "type")
	if itemType == "" {
		itemType = "song"
	}

	limit := parseIntParam(r, "limit", 10)
	ctx := r.Context()
	base := baseURL(r)

	var genre string

	switch itemType {
	case "song":
		song, err := h.db.Music.GetSong(ctx, id)
		if err != nil || song == nil {
			respond(w, r, isJSON, errResp(4704, "Not found"))
			return
		}
		genre = song.Genre

	case "artist":
		songs, err := h.db.Music.ListSongsByArtist(ctx, id)
		if err == nil && len(songs) > 0 {
			genre = songs[0].Genre
		}
	}

	similar, err := h.db.Music.GetRandomSongs(ctx, limit, genre, "", "")
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	result := make([]AmpSong, 0, len(similar))
	for _, s := range similar {
		if s.ID == id {
			continue
		}
		result = append(result, songToAmp(s, base))
	}
	respond(w, r, isJSON, okResp("song", result))
}

// shares returns all shares visible to the authenticated user.
func (h *Handler) shares(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("share", []AmpShare{}))
}

// share returns a single share by ID.
func (h *Handler) share(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, errResp(4704, "Not found"))
}

// shareCreate creates a new share link.
func (h *Handler) shareCreate(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("success", "shares not implemented"))
}

// shareEdit modifies an existing share.
func (h *Handler) shareEdit(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("success", "shares not implemented"))
}

// shareDelete removes a share.
func (h *Handler) shareDelete(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("success", "shares not implemented"))
}

// bookmarks returns all bookmarks for the authenticated user.
func (h *Handler) bookmarks(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	list, err := h.db.Activity.GetBookmarks(r.Context(), session.UserID)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	result := make([]AmpBookmark, 0, len(list))
	for _, b := range list {
		result = append(result, bookmarkToAmp(b, ""))
	}
	respond(w, r, isJSON, okResp("bookmark", result))
}

// bookmarkCreate creates or updates a bookmark.
func (h *Handler) bookmarkCreate(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (item ID)"))
		return
	}

	objectType := param(r, "type")
	if objectType == "" {
		objectType = "song"
	}

	position := int64(parseIntParam(r, "position", 0))
	comment := param(r, "comment")

	b := &model.Bookmark{
		UserID:   session.UserID,
		ItemType: objectType,
		ItemID:   id,
		Position: position,
		Comment:  comment,
	}

	if err := h.db.Activity.SetBookmark(r.Context(), b); err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to save bookmark: "+err.Error()))
		return
	}

	respond(w, r, isJSON, bookmarkToAmp(b, ""))
}

// bookmarkEdit updates an existing bookmark.
func (h *Handler) bookmarkEdit(w http.ResponseWriter, r *http.Request, isJSON bool) {
	h.bookmarkCreate(w, r, isJSON)
}

// bookmarkDelete removes a bookmark.
func (h *Handler) bookmarkDelete(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (item ID)"))
		return
	}

	objectType := param(r, "type")
	if objectType == "" {
		objectType = "song"
	}

	if err := h.db.Activity.DeleteBookmark(r.Context(), session.UserID, objectType, id); err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to delete bookmark: "+err.Error()))
		return
	}

	respond(w, r, isJSON, okResp("success", "bookmark deleted"))
}

// getBookmark returns the bookmark for a specific item.
func (h *Handler) getBookmark(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "filter")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: filter (item ID)"))
		return
	}

	objectType := param(r, "type")
	if objectType == "" {
		objectType = "song"
	}

	list, err := h.db.Activity.GetBookmarks(r.Context(), session.UserID)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Bad request: "+err.Error()))
		return
	}

	for _, b := range list {
		if b.ItemType == objectType && b.ItemID == id {
			respond(w, r, isJSON, bookmarkToAmp(b, ""))
			return
		}
	}

	respond(w, r, isJSON, errResp(4704, "Bookmark not found"))
}

// deletedSongs returns an empty list (deleted songs are purged from the DB).
func (h *Handler) deletedSongs(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("song", []AmpSong{}))
}

// deletedVideo returns an empty list (video not supported).
func (h *Handler) deletedVideo(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("video", []map[string]any{}))
}

// deletedPodcastEpisodes returns episodes with deleted status.
func (h *Handler) deletedPodcastEpisodes(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}
	respond(w, r, isJSON, okResp("podcast_episode", []AmpPodcastEpisode{}))
}

// playlistToAmp converts a model.Playlist to the Ampache wire type.
// The owner username is left empty when h is nil or the lookup fails.
func playlistToAmp(p *model.Playlist, _ int64, base string, _ *Handler) AmpPlaylist {
	plType := "private"
	if p.IsPublic {
		plType = "public"
	}
	return AmpPlaylist{
		ID:       itoa(p.ID),
		Name:     p.Name,
		Owner:    "",
		Items:    p.SongCount,
		Type:     plType,
		Duration: p.Duration,
		Art:      base + "/server/json.server.php?action=get_art&id=" + itoa(p.ID) + "&type=playlist",
	}
}

// bookmarkToAmp converts a model.Bookmark to the Ampache wire type.
func bookmarkToAmp(b *model.Bookmark, username string) AmpBookmark {
	return AmpBookmark{
		ID:         itoa(b.ID),
		Owner:      username,
		ObjectType: b.ItemType,
		ObjectID:   itoa(b.ItemID),
		Position:   b.Position,
		Comment:    b.Comment,
		Update:     b.UpdatedAt.Format(time.RFC3339),
	}
}
