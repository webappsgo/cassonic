package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"

	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/service/tags"
	cerr "github.com/local/cassonic/src/common/errors"
)

// songTagFields maps a SongMeta field name to its value for the JSON response.
func songTagFields(path string, h *Handler) map[string]any {
	fields := map[string]any{}
	if h.tagReader == nil {
		return fields
	}
	meta, err := h.tagReader.Read(path)
	if err != nil {
		return fields
	}
	fields["title"] = meta.Title
	fields["artist"] = meta.Artist
	fields["album_artist"] = meta.AlbumArtist
	fields["album"] = meta.Album
	fields["track_number"] = meta.TrackNumber
	fields["disc_number"] = meta.DiscNumber
	fields["year"] = meta.Year
	fields["genre"] = meta.Genre
	fields["composer"] = meta.Composer
	fields["lyricist"] = meta.Lyricist
	fields["conductor"] = meta.Conductor
	fields["comment"] = meta.Comment
	fields["lyrics"] = meta.Lyrics
	fields["bpm"] = meta.BPM
	fields["replay_gain_track"] = meta.ReplayGainTrack
	fields["replay_gain_album"] = meta.ReplayGainAlbum
	fields["mb_track_id"] = meta.MBTrackID
	fields["mb_album_id"] = meta.MBAlbumID
	fields["mb_artist_id"] = meta.MBArtistID
	fields["mb_album_artist_id"] = meta.MBAlbumArtistID
	return fields
}

// isWritable returns true when the file at path can be opened for writing.
func isWritable(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// GetTags returns all tag fields for a song plus a writable flag.
func (h *Handler) GetTags(w http.ResponseWriter, r *http.Request) {
	songID, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid song id"))
		return
	}

	song, err := h.db.Music.GetSong(r.Context(), songID)
	if err != nil || song == nil {
		writeError(w, r, cerr.NotFound("song not found"))
		return
	}

	writable := isWritable(song.Path)
	tagData := songTagFields(song.Path, h)

	writeJSON(w, http.StatusOK, map[string]any{
		"tags":     tagData,
		"writable": writable,
	})
}

// PatchTags writes a partial set of tag fields to the audio file.
func (h *Handler) PatchTags(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	songID, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid song id"))
		return
	}

	song, err := h.db.Music.GetSong(r.Context(), songID)
	if err != nil || song == nil {
		writeError(w, r, cerr.NotFound("song not found"))
		return
	}

	if !isWritable(song.Path) {
		writeError(w, r, cerr.Forbidden("file is not writable"))
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, r, cerr.BadRequest("could not read request body"))
		return
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, r, cerr.BadRequest("invalid JSON body"))
		return
	}

	fields := tags.WritableFields{}
	for k, v := range raw {
		fields[k] = v
	}

	if err := tags.Write(song.Path, fields); err != nil {
		if err == tags.ErrNotWritable {
			writeError(w, r, cerr.Forbidden("file is not writable"))
			return
		}
		writeError(w, r, cerr.InternalServerError("tag write failed: "+err.Error()))
		return
	}

	song.UserEdited = true
	if _, err := h.db.Music.UpsertSong(r.Context(), song); err != nil {
		writeError(w, r, cerr.InternalServerError("db update failed"))
		return
	}

	tagData := songTagFields(song.Path, h)
	writeJSON(w, http.StatusOK, map[string]any{"tags": tagData})
}

// TagsWritable reports whether the song file can be written to.
func (h *Handler) TagsWritable(w http.ResponseWriter, r *http.Request) {
	songID, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid song id"))
		return
	}

	song, err := h.db.Music.GetSong(r.Context(), songID)
	if err != nil || song == nil {
		writeError(w, r, cerr.NotFound("song not found"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"writable": isWritable(song.Path)})
}

// UploadSongCoverArt receives a multipart image and stores it as the song's cover art.
func (h *Handler) UploadSongCoverArt(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	songID, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid song id"))
		return
	}

	song, err := h.db.Music.GetSong(r.Context(), songID)
	if err != nil || song == nil {
		writeError(w, r, cerr.NotFound("song not found"))
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		writeError(w, r, cerr.BadRequest("invalid multipart form"))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, r, cerr.BadRequest("missing file field"))
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil {
		writeError(w, r, cerr.InternalServerError("could not read uploaded file"))
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	coverID, err := h.coverArt.SaveFromBytes(r.Context(), data, mimeType, songID, song.AlbumID)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("cover art save failed"))
		return
	}

	song.CoverArtID = coverID
	if _, err := h.db.Music.UpsertSong(r.Context(), song); err != nil {
		writeError(w, r, cerr.InternalServerError("db update failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"cover_art_id": coverID})
}

// DeleteSongCoverArt clears the cover art association for a song.
func (h *Handler) DeleteSongCoverArt(w http.ResponseWriter, r *http.Request) {
	auth := mw.UserFromContext(r.Context())
	if auth == nil {
		writeError(w, r, cerr.Unauthorized("not authenticated"))
		return
	}

	songID, err := parseID(r, "id")
	if err != nil {
		writeError(w, r, cerr.BadRequest("invalid song id"))
		return
	}

	song, err := h.db.Music.GetSong(r.Context(), songID)
	if err != nil || song == nil {
		writeError(w, r, cerr.NotFound("song not found"))
		return
	}

	song.CoverArtID = 0
	if _, err := h.db.Music.UpsertSong(r.Context(), song); err != nil {
		writeError(w, r, cerr.InternalServerError("db update failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}
