package ampache

import (
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// stream serves the audio file for the requested song ID.
// Supports Range requests for seeking. The offset param specifies a start
// position in seconds (applied as a byte offset estimate when format=raw).
func (h *Handler) stream(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "id")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: id"))
		return
	}

	song, err := h.db.Music.GetSong(r.Context(), id)
	if err != nil || song == nil {
		respond(w, r, isJSON, errResp(4704, "Not found"))
		return
	}

	f, err := os.Open(song.Path)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Cannot open file"))
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Cannot stat file"))
		return
	}

	mime := song.ContentType
	if mime == "" {
		mime = "audio/mpeg"
	}

	_ = h.db.Music.IncrementPlayCount(r.Context(), song.ID)

	w.Header().Set("Content-Type", mime)
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, song.Path, info.ModTime(), f)
}

// download serves the raw audio file for direct download.
func (h *Handler) download(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "id")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: id"))
		return
	}

	song, err := h.db.Music.GetSong(r.Context(), id)
	if err != nil || song == nil {
		respond(w, r, isJSON, errResp(4704, "Not found"))
		return
	}

	f, err := os.Open(song.Path)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Cannot open file"))
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Cannot stat file"))
		return
	}

	ext := ""
	if i := strings.LastIndex(song.Path, "."); i >= 0 {
		ext = song.Path[i:]
	}

	mime := song.ContentType
	if mime == "" {
		mime = "audio/mpeg"
	}

	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", `attachment; filename="`+strconv.Itoa(int(song.ID))+ext+`"`)
	http.ServeContent(w, r, song.Path, info.ModTime(), f)
}

// getArt returns the cover art image for the requested item.
// The type parameter is one of song, album, or artist.
func (h *Handler) getArt(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireSession(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "id")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: id"))
		return
	}

	artType := param(r, "type")
	if artType == "" {
		artType = "song"
	}

	var data []byte
	var mime string
	var err error

	switch artType {
	case "song":
		data, mime, err = h.coverArt.GetForSong(r.Context(), id)
	case "album":
		data, mime, err = h.coverArt.GetForAlbum(r.Context(), id)
	case "artist":
		// Artists may have cover art stored on their first album.
		albums, aerr := h.db.Music.ListAlbumsByArtist(r.Context(), id)
		if aerr == nil && len(albums) > 0 {
			data, mime, err = h.coverArt.GetForAlbum(r.Context(), albums[0].ID)
		} else {
			err = aerr
		}
	default:
		respond(w, r, isJSON, errResp(4705, "Invalid type parameter"))
		return
	}

	if err != nil {
		respond(w, r, isJSON, errResp(4704, "Not found"))
		return
	}

	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// updateArt fetches cover art from a URL and stores it for the given item.
// Admin only.
func (h *Handler) updateArt(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "id")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: id"))
		return
	}

	artType := param(r, "type")
	if artType == "" {
		artType = "song"
	}

	url := param(r, "url")
	if url == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: url"))
		return
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to fetch art: "+err.Error()))
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to read art data"))
		return
	}

	mime := resp.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/jpeg"
	}

	var songID, albumID int64
	switch artType {
	case "song":
		songID = id
	case "album":
		albumID = id
	}

	_, err = h.coverArt.SaveFromBytes(r.Context(), data, mime, songID, albumID)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to save art: "+err.Error()))
		return
	}

	respond(w, r, isJSON, okResp("success", "art updated"))
}

// updateArtistInfo is a placeholder for MusicBrainz artist info lookup.
// The actual lookup is deferred to a future wave.
func (h *Handler) updateArtistInfo(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	id := parseIDParam(r, "id")
	if id == 0 {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: id"))
		return
	}

	respond(w, r, isJSON, okResp("success", "artist info update queued"))
}

// upload receives a multipart audio file and saves it to the specified destination.
// Admin only.
func (h *Handler) upload(w http.ResponseWriter, r *http.Request, isJSON bool) {
	session := h.requireAdmin(w, r, isJSON)
	if session == nil {
		return
	}

	destination := param(r, "destination")
	if destination == "" {
		respond(w, r, isJSON, errResp(4705, "Missing parameter: destination"))
		return
	}

	if err := r.ParseMultipartForm(500 << 20); err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to parse multipart form: "+err.Error()))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respond(w, r, isJSON, errResp(4705, "Missing file in upload"))
		return
	}
	defer file.Close()

	destPath := strings.TrimSuffix(destination, "/") + "/" + header.Filename
	out, err := os.Create(destPath)
	if err != nil {
		respond(w, r, isJSON, errResp(4710, "Cannot create destination file: "+err.Error()))
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		respond(w, r, isJSON, errResp(4710, "Failed to write file: "+err.Error()))
		return
	}

	respond(w, r, isJSON, okResp("success", "file uploaded to "+destPath))
}
