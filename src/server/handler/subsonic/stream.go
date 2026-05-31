package subsonic

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/service"
	"github.com/local/cassonic/src/server/service/ffmpeg"
	"github.com/local/cassonic/src/server/store"
)

// stream serves an audio file, optionally transcoded, and records the play.
func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
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

	ctx := r.Context()
	song, err := h.db.Music.GetSong(ctx, dbID)
	if err != nil || song == nil {
		respond(w, r, errResp(ErrNotFound, "Song not found."))
		return
	}

	q := r.URL.Query()
	maxBitRate := queryIntDefault(q.Get("maxBitRate"), 0)
	format := q.Get("format")
	timeOffset := queryIntDefault(q.Get("timeOffset"), 0)

	_ = h.db.Music.IncrementPlayCount(ctx, song.ID)
	_ = h.db.Activity.RecordPlay(ctx, &model.PlayHistory{
		UserID:     authUser.ID,
		SongID:     song.ID,
		ClientName: middleware.SubsonicClientFromContext(ctx),
	})

	clientName := middleware.SubsonicClientFromContext(ctx)
	h.nowPlaying.Register(&NowPlayingEntry{
		UserID:     authUser.ID,
		Username:   authUser.Username,
		SongID:     song.ID,
		Title:      song.Title,
		Artist:     song.ArtistName,
		Album:      song.AlbumName,
		PlayerName: clientName,
	})

	h.serveStream(w, r, song, maxBitRate, format, timeOffset)
}

// download serves the original audio file as an attachment download.
func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	if !authUser.IsAdmin {
		u, err := h.db.Users.GetUserByUsername(r.Context(), authUser.Username)
		if err != nil || u == nil || !u.CanDownload {
			respond(w, r, errResp(ErrForbidden, "Download permission denied."))
			return
		}
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

	f, err := os.Open(song.Path)
	if err != nil {
		respond(w, r, errResp(ErrGeneric, "File not accessible."))
		return
	}
	defer f.Close()

	ext := filepath.Ext(song.Path)
	filename := song.Title + ext
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", song.ContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

// hls generates a minimal HLS playlist pointing at the stream endpoint.
func (h *Handler) hls(w http.ResponseWriter, r *http.Request) {
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

	streamURL := fmt.Sprintf("%s/rest/stream?id=%s&u=%s",
		r.URL.Scheme+"://"+r.Host,
		id,
		r.URL.Query().Get("u"),
	)

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "#EXTM3U\n")
	fmt.Fprintf(w, "#EXT-X-VERSION:3\n")
	fmt.Fprintf(w, "#EXT-X-TARGETDURATION:%d\n", song.Duration)
	fmt.Fprintf(w, "#EXT-X-MEDIA-SEQUENCE:0\n")
	fmt.Fprintf(w, "#EXTINF:%d,\n", song.Duration)
	fmt.Fprintf(w, "%s\n", streamURL)
	fmt.Fprintf(w, "#EXT-X-ENDLIST\n")
}

// getCoverArt serves cover art bytes for a song, album, or artist.
func (h *Handler) getCoverArt(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	size := queryIntDefault(r.URL.Query().Get("size"), 0)

	prefix, dbID, err := decodeID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ctx := r.Context()

	var data []byte
	var mime string

	switch prefix {
	case prefixAlbum, "":
		if size > 0 {
			data, mime, err = h.coverArt.GetThumbnail(ctx, dbID, size)
			if err != nil {
				data, mime, err = h.coverArt.GetForAlbum(ctx, dbID)
			}
		} else {
			data, mime, err = h.coverArt.GetForAlbum(ctx, dbID)
		}

	case prefixSong:
		if size > 0 {
			data, mime, err = h.coverArt.GetThumbnail(ctx, dbID, size)
			if err != nil {
				data, mime, err = h.coverArt.GetForSong(ctx, dbID)
			}
		} else {
			data, mime, err = h.coverArt.GetForSong(ctx, dbID)
		}

	case prefixArtist:
		artist, aerr := h.db.Music.GetArtist(ctx, dbID)
		if aerr != nil || artist == nil {
			http.NotFound(w, r)
			return
		}
		data, mime, err = h.coverArt.GetForAlbum(ctx, dbID)

	default:
		http.NotFound(w, r)
		return
	}

	if err != nil {
		if errors.Is(err, service.ErrNoCoverArt) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", mime)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// getLyrics returns lyrics for a song identified by artist and title params, or by song ID.
func (h *Handler) getLyrics(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		respond(w, r, errResp(ErrNotAuthenticated, "Not authenticated."))
		return
	}

	q := r.URL.Query()
	artist := q.Get("artist")
	title := q.Get("title")

	var lyrics string
	var lArtist, lTitle string

	if artist != "" && title != "" {
		songs, err := h.db.Music.SearchSongs(r.Context(), title, store.ListOpts{Limit: 1})
		if err == nil && len(songs) > 0 {
			for _, s := range songs {
				if strings.EqualFold(s.ArtistName, artist) {
					lyrics = s.Lyrics
					lArtist = s.ArtistName
					lTitle = s.Title
					break
				}
			}
		}
	}

	respond(w, r, ok(func(resp *SubsonicResponse) {
		resp.Lyrics = &Lyrics{
			Artist: lArtist,
			Title:  lTitle,
			Value:  lyrics,
		}
	}))
}

// getAvatar returns a default SVG avatar; custom avatars are not supported.
func (h *Handler) getAvatar(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if authUser == nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 40">` +
		`<circle cx="20" cy="20" r="20" fill="#555"/>` +
		`<circle cx="20" cy="16" r="7" fill="#ccc"/>` +
		`<ellipse cx="20" cy="36" rx="12" ry="8" fill="#ccc"/>` +
		`</svg>`

	w.Header().Set("Content-Type", "image/svg+xml")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, svg)
}

// getCaptions always returns not-found; caption/subtitle tracks are not supported.
func (h *Handler) getCaptions(w http.ResponseWriter, r *http.Request) {
	respond(w, r, errResp(ErrNotFound, "Captions not found."))
}

// serveStream pipes an audio file to the response, transcoding if requested.
func (h *Handler) serveStream(w http.ResponseWriter, r *http.Request, song *model.Song, maxBitRate int, format string, timeOffset int) {
	needsTranscode := false
	if h.ffmpeg != nil && format != "" && format != "raw" {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(song.Path), "."))
		if format != ext {
			needsTranscode = true
		}
	}

	if h.ffmpeg != nil && (needsTranscode || maxBitRate > 0) && maxBitRate != song.BitRate {
		h.streamTranscoded(w, r, song, maxBitRate, format, timeOffset)
		return
	}

	h.streamDirect(w, r, song)
}

// streamDirect serves the original audio file without any processing.
func (h *Handler) streamDirect(w http.ResponseWriter, r *http.Request, song *model.Song) {
	f, err := os.Open(song.Path)
	if err != nil {
		http.Error(w, "File not accessible", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", song.ContentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

// streamTranscoded pipes the audio through ffmpeg before sending to the client.
func (h *Handler) streamTranscoded(w http.ResponseWriter, r *http.Request, song *model.Song, maxBitRate int, format string, timeOffset int) {
	if h.ffmpeg == nil {
		h.streamDirect(w, r, song)
		return
	}

	opts := ffmpeg.TranscodeOpts{
		InputPath:   song.Path,
		Format:      format,
		MaxBitRate:  maxBitRate,
		StartOffset: timeOffset,
	}

	result, err := h.ffmpeg.Transcode(r.Context(), opts)
	if err != nil {
		http.Error(w, "Transcoding failed", http.StatusInternalServerError)
		return
	}
	defer result.Close()

	contentType := formatToMIME(format)
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, result)
}

// formatToMIME returns the MIME type for a given audio format string.
func formatToMIME(format string) string {
	switch strings.ToLower(format) {
	case "mp3":
		return "audio/mpeg"
	case "ogg":
		return "audio/ogg"
	case "opus":
		return "audio/ogg; codecs=opus"
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	default:
		return "audio/mpeg"
	}
}

