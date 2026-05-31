package api

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/service/ffmpeg"
	cerr "github.com/local/cassonic/src/common/errors"
)

// formatMIME maps a target format string to the appropriate Content-Type.
func formatMIME(format string) string {
	switch format {
	case "mp3":
		return "audio/mpeg"
	case "ogg":
		return "audio/ogg"
	case "opus":
		return "audio/opus"
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	default:
		return "application/octet-stream"
	}
}

// mimeForSong derives the Content-Type from the song's stored content type or file format.
func mimeForSong(song *model.Song) string {
	if song.ContentType != "" {
		return song.ContentType
	}
	switch strings.ToLower(song.FileFormat) {
	case "mp3":
		return "audio/mpeg"
	case "flac":
		return "audio/flac"
	case "ogg":
		return "audio/ogg"
	case "opus":
		return "audio/opus"
	case "aac", "m4a":
		return "audio/aac"
	case "wav":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}

// Stream serves the audio file for a song, optionally transcoding it.
func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
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

	if _, err := os.Stat(song.Path); err != nil {
		writeError(w, r, cerr.NotFound("song file not found on disk"))
		return
	}

	format := strings.ToLower(r.URL.Query().Get("format"))
	maxBitRateStr := r.URL.Query().Get("max_bit_rate")
	maxBitRate := 0
	if maxBitRateStr != "" {
		if v, err := strconv.Atoi(maxBitRateStr); err == nil && v > 0 {
			maxBitRate = v
		}
	}

	needsTranscode := format != "" && format != "original" && format != strings.ToLower(song.FileFormat)
	if maxBitRate > 0 && song.BitRate > maxBitRate {
		needsTranscode = true
	}

	if !needsTranscode || format == "original" || format == "" {
		serveFileDirect(w, r, song)
		go h.registerNowPlaying(auth, song, r.Header.Get("X-Player-Name"))
		return
	}

	if h.ffmpeg == nil {
		writeError(w, r, cerr.ServiceUnavailable("transcoding not available"))
		return
	}

	opts := ffmpeg.TranscodeOpts{
		InputPath:  song.Path,
		Format:     format,
		MaxBitRate: maxBitRate,
	}

	result, err := h.ffmpeg.Transcode(r.Context(), opts)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("transcode failed: "+err.Error()))
		return
	}
	defer result.Close()

	w.Header().Set("Content-Type", formatMIME(format))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	go h.registerNowPlaying(auth, song, r.Header.Get("X-Player-Name"))

	_, _ = io.Copy(w, result)

	go h.recordScrobble(auth.ID, song)
}

// serveFileDirect serves the original audio file with Range request support.
func serveFileDirect(w http.ResponseWriter, r *http.Request, song *model.Song) {
	f, err := os.Open(song.Path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", mimeForSong(song))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.FormatInt(song.FileSize, 10))
	if song.Duration > 0 {
		w.Header().Set("X-Content-Duration", strconv.Itoa(song.Duration))
	}

	http.ServeContent(w, r, filepath.Base(song.Path), time.Time{}, f)
}

// registerNowPlaying records the stream in the now-playing tracker.
func (h *Handler) registerNowPlaying(auth *mw.AuthUser, song *model.Song, playerName string) {
	h.nowPlaying.Register(auth.ID, &NowPlayingInfo{
		UserID:     auth.ID,
		Username:   auth.Username,
		Song:       song,
		StartedAt:  time.Now(),
		PlayerName: playerName,
	})
}

// recordScrobble increments the play count for the song using a background context.
func (h *Handler) recordScrobble(_ int64, song *model.Song) {
	_ = h.db.Music.IncrementPlayCount(context.Background(), song.ID)
}

// Download serves the original audio file as an attachment.
func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
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

	if _, err := os.Stat(song.Path); err != nil {
		writeError(w, r, cerr.NotFound("song file not found on disk"))
		return
	}

	f, err := os.Open(song.Path)
	if err != nil {
		writeError(w, r, cerr.InternalServerError("could not open file"))
		return
	}
	defer f.Close()

	ext := filepath.Ext(song.Path)
	filename := song.Title + ext
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.Header().Set("Content-Type", mimeForSong(song))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.FormatInt(song.FileSize, 10))

	http.ServeContent(w, r, filepath.Base(song.Path), time.Time{}, f)
}

// GetSongCoverArt serves cover art for a song with optional thumbnail sizing.
func (h *Handler) GetSongCoverArt(w http.ResponseWriter, r *http.Request) {
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

	sizeStr := r.URL.Query().Get("size")
	size := 0
	if sizeStr != "" {
		if v, err := strconv.Atoi(sizeStr); err == nil {
			size = v
		}
	}

	var data []byte
	var mimeType string

	if size > 0 && song.CoverArtID > 0 {
		data, mimeType, err = h.coverArt.GetThumbnail(r.Context(), song.CoverArtID, size)
	} else {
		data, mimeType, err = h.coverArt.GetForSong(r.Context(), songID)
	}

	if err != nil {
		writeError(w, r, cerr.NotFound("cover art not found"))
		return
	}

	etag := fmt.Sprintf(`"ca-%d"`, song.CoverArtID)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
