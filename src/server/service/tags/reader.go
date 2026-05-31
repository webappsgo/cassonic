package tags

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bogem/id3v2/v2"
	flac "github.com/go-flac/go-flac"
	"github.com/go-flac/flacvorbis"
	"github.com/dhowden/tag"
	"github.com/local/cassonic/src/server/service"
)

var (
	// ErrUnsupportedFormat is returned when the file extension is not a recognised audio format.
	ErrUnsupportedFormat = errors.New("tags: unsupported audio format")
	// ErrNotWritable is returned when a file cannot be opened for writing.
	ErrNotWritable = errors.New("tags: file is not writable")
)

// maxCoverBytes is the maximum embedded cover art size accepted (20 MiB).
const maxCoverBytes = 20 * 1024 * 1024

// Reader implements service.TagReader using dhowden/tag with format-specific extras.
type Reader struct{}

// New creates a new tag Reader.
func New() *Reader {
	return &Reader{}
}

// Read implements service.TagReader.
// Opens the file, detects format from extension, reads tags, and normalises to SongMeta.
func (r *Reader) Read(path string) (meta *service.SongMeta, readErr error) {
	defer func() {
		if rec := recover(); rec != nil {
			readErr = fmt.Errorf("tags: panic reading %s: %v", path, rec)
		}
	}()

	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".mp3":
		return readMP3(path)
	case ".flac":
		return readFLAC(path)
	case ".ogg":
		return readOGGOpus(path, "ogg")
	case ".opus":
		return readOGGOpus(path, "opus")
	case ".m4a", ".aac":
		return readM4A(path)
	case ".wav":
		return readGeneric(path, "wav", "audio/wav")
	case ".aiff", ".aif":
		return readGeneric(path, "aiff", "audio/aiff")
	default:
		return nil, ErrUnsupportedFormat
	}
}

// sanitize replaces invalid UTF-8 sequences in s with the replacement character.
func sanitize(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "�")
}

// fileSize returns the size of the file at path, or 0 on error.
func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

// parseBPM converts a BPM string to an integer, returning 0 on failure.
func parseBPM(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

// parseReplayGain parses a ReplayGain string like "-6.54 dB" and returns the float value.
func parseReplayGain(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(strings.ToLower(s), " db")
	s = strings.TrimSuffix(s, "db")
	s = strings.TrimSpace(s)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// extractCover returns cover data and MIME type from a dhowden/tag Picture, enforcing the size limit.
func extractCover(pic *tag.Picture) ([]byte, string) {
	if pic == nil {
		return nil, ""
	}
	if len(pic.Data) > maxCoverBytes {
		return nil, ""
	}
	return pic.Data, sanitize(pic.MIMEType)
}

// baseMetaFromTag fills a SongMeta from the dhowden Metadata interface fields common to all formats.
func baseMetaFromTag(m tag.Metadata) *service.SongMeta {
	trackNum, _ := m.Track()
	discNum, _ := m.Disc()

	cover, coverMime := extractCover(m.Picture())

	meta := &service.SongMeta{
		Title:       sanitize(m.Title()),
		Artist:      sanitize(m.Artist()),
		AlbumArtist: sanitize(m.AlbumArtist()),
		Album:       sanitize(m.Album()),
		TrackNumber: trackNum,
		DiscNumber:  discNum,
		Year:        m.Year(),
		Genre:       sanitize(m.Genre()),
		Composer:    sanitize(m.Composer()),
		Lyrics:      sanitize(m.Lyrics()),
		Comment:     sanitize(m.Comment()),
		CoverData:   cover,
		CoverMime:   coverMime,
	}

	return meta
}

// readMP3 reads tags from an MP3 file using dhowden/tag for base fields and bogem/id3v2 for extras.
func readMP3(path string) (*service.SongMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("tags: open %s: %w", path, err)
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil && !errors.Is(err, tag.ErrNoTagsFound) {
		return nil, fmt.Errorf("tags: read %s: %w", path, err)
	}

	var meta *service.SongMeta
	if m != nil {
		meta = baseMetaFromTag(m)
	} else {
		meta = &service.SongMeta{}
	}

	meta.Format = "mp3"
	meta.ContentType = "audio/mpeg"
	meta.FileSize = fileSize(path)

	enrichMP3(path, meta)

	return meta, nil
}

// enrichMP3 uses bogem/id3v2 to extract frames that dhowden/tag does not surface directly:
// TXXX (MusicBrainz IDs, ReplayGain), USLT (lyrics), TCOM, TEXT, TPE3, TBPM, COMM.
func enrichMP3(path string, meta *service.SongMeta) {
	defer func() {
		recover()
	}()

	t, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return
	}
	defer t.Close()

	// Extract simple text frames only when the base reader returned empty strings.
	if meta.Composer == "" {
		meta.Composer = sanitize(t.GetTextFrame(t.CommonID("Composer")).Text)
	}
	if meta.Lyrics == "" {
		if frames := t.GetFrames(t.CommonID("Unsynchronised lyrics/text transcription")); len(frames) > 0 {
			if uf, ok := frames[0].(id3v2.UnsynchronisedLyricsFrame); ok {
				meta.Lyrics = sanitize(uf.Lyrics)
			}
		}
	}
	if meta.Comment == "" {
		if frames := t.GetFrames(t.CommonID("Comments")); len(frames) > 0 {
			if cf, ok := frames[0].(id3v2.CommentFrame); ok {
				meta.Comment = sanitize(cf.Text)
			}
		}
	}

	// Extract BPM from TBPM text frame.
	bpmText := t.GetTextFrame("TBPM").Text
	if bpmText != "" {
		meta.BPM = parseBPM(bpmText)
	}

	// Lyricist frame TEXT (TPE3 for conductor, TEXT for lyricist).
	if meta.Lyricist == "" {
		meta.Lyricist = sanitize(t.GetTextFrame("TEXT").Text)
	}
	if meta.Conductor == "" {
		meta.Conductor = sanitize(t.GetTextFrame("TPE3").Text)
	}

	// Iterate TXXX frames for MusicBrainz IDs and ReplayGain values.
	for _, framer := range t.GetFrames("TXXX") {
		udtf, ok := framer.(id3v2.UserDefinedTextFrame)
		if !ok {
			continue
		}
		desc := strings.TrimSpace(udtf.Description)
		val := sanitize(strings.TrimSpace(udtf.Value))
		switch desc {
		case "MusicBrainz Track Id":
			meta.MBTrackID = val
		case "MusicBrainz Artist Id":
			meta.MBArtistID = val
		case "MusicBrainz Album Id":
			meta.MBAlbumID = val
		case "MusicBrainz Album Artist Id":
			meta.MBAlbumArtistID = val
		case "REPLAYGAIN_TRACK_GAIN":
			meta.ReplayGainTrack = parseReplayGain(val)
		case "REPLAYGAIN_ALBUM_GAIN":
			meta.ReplayGainAlbum = parseReplayGain(val)
		}
	}
}

// readFLAC reads tags from a FLAC file using dhowden/tag for base fields and go-flac for Vorbis extras.
func readFLAC(path string) (*service.SongMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("tags: open %s: %w", path, err)
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil && !errors.Is(err, tag.ErrNoTagsFound) {
		return nil, fmt.Errorf("tags: read %s: %w", path, err)
	}

	var meta *service.SongMeta
	if m != nil {
		meta = baseMetaFromTag(m)
	} else {
		meta = &service.SongMeta{}
	}

	meta.Format = "flac"
	meta.ContentType = "audio/flac"
	meta.FileSize = fileSize(path)

	enrichFLAC(path, meta)

	return meta, nil
}

// enrichFLAC reads Vorbis Comments from a FLAC file to fill MusicBrainz IDs and extra fields.
func enrichFLAC(path string, meta *service.SongMeta) {
	defer func() {
		recover()
	}()

	ff, err := flac.ParseFile(path)
	if err != nil {
		return
	}

	var vc *flacvorbis.MetaDataBlockVorbisComment
	for _, block := range ff.Meta {
		if block.Type == flac.VorbisComment {
			vc, err = flacvorbis.ParseFromMetaDataBlock(*block)
			if err != nil {
				return
			}
			break
		}
	}
	if vc == nil {
		return
	}

	// Helper to get the first value for a Vorbis comment key.
	get := func(key string) string {
		vals, err := vc.Get(key)
		if err != nil || len(vals) == 0 {
			return ""
		}
		return sanitize(vals[0])
	}

	if v := get("MUSICBRAINZ_TRACKID"); v != "" {
		meta.MBTrackID = v
	}
	if v := get("MUSICBRAINZ_ARTISTID"); v != "" {
		meta.MBArtistID = v
	}
	if v := get("MUSICBRAINZ_ALBUMID"); v != "" {
		meta.MBAlbumID = v
	}
	if v := get("MUSICBRAINZ_ALBUMARTISTID"); v != "" {
		meta.MBAlbumArtistID = v
	}
	if meta.Lyrics == "" {
		meta.Lyrics = get("LYRICS")
	}
	if meta.Composer == "" {
		meta.Composer = get("COMPOSER")
	}
	if meta.Lyricist == "" {
		meta.Lyricist = get("LYRICIST")
	}
	if meta.Conductor == "" {
		meta.Conductor = get("CONDUCTOR")
	}
	if meta.BPM == 0 {
		meta.BPM = parseBPM(get("BPM"))
	}
	if meta.ReplayGainTrack == 0 {
		meta.ReplayGainTrack = parseReplayGain(get("REPLAYGAIN_TRACK_GAIN"))
	}
	if meta.ReplayGainAlbum == 0 {
		meta.ReplayGainAlbum = parseReplayGain(get("REPLAYGAIN_ALBUM_GAIN"))
	}
}

// readOGGOpus reads tags from an OGG or Opus file using dhowden/tag for all fields.
// Basic Vorbis Comments (title, artist, album, etc.) are already surfaced by dhowden/tag.
// MusicBrainz IDs are extracted from the Raw() map.
func readOGGOpus(path string, format string) (*service.SongMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("tags: open %s: %w", path, err)
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil && !errors.Is(err, tag.ErrNoTagsFound) {
		return nil, fmt.Errorf("tags: read %s: %w", path, err)
	}

	var meta *service.SongMeta
	if m != nil {
		meta = baseMetaFromTag(m)
		// Extract extras from the Raw Vorbis Comment map.
		enrichFromVorbisRaw(m.Raw(), meta)
	} else {
		meta = &service.SongMeta{}
	}

	meta.Format = format
	if format == "opus" {
		meta.ContentType = "audio/ogg; codecs=opus"
	} else {
		meta.ContentType = "audio/ogg"
	}
	meta.FileSize = fileSize(path)

	return meta, nil
}

// enrichFromVorbisRaw extracts MusicBrainz IDs and extra Vorbis Comment fields
// from the dhowden/tag Raw() map (which uses lowercase keys for Vorbis format).
func enrichFromVorbisRaw(raw map[string]interface{}, meta *service.SongMeta) {
	get := func(key string) string {
		if v, ok := raw[key]; ok {
			if s, ok := v.(string); ok {
				return sanitize(s)
			}
		}
		return ""
	}

	if v := get("musicbrainz_trackid"); v != "" {
		meta.MBTrackID = v
	}
	if v := get("musicbrainz_artistid"); v != "" {
		meta.MBArtistID = v
	}
	if v := get("musicbrainz_albumid"); v != "" {
		meta.MBAlbumID = v
	}
	if v := get("musicbrainz_albumartistid"); v != "" {
		meta.MBAlbumArtistID = v
	}
	if meta.Lyrics == "" {
		meta.Lyrics = get("lyrics")
	}
	if meta.Lyricist == "" {
		meta.Lyricist = get("lyricist")
	}
	if meta.Conductor == "" {
		meta.Conductor = get("conductor")
	}
	if meta.BPM == 0 {
		meta.BPM = parseBPM(get("bpm"))
	}
	if meta.ReplayGainTrack == 0 {
		meta.ReplayGainTrack = parseReplayGain(get("replaygain_track_gain"))
	}
	if meta.ReplayGainAlbum == 0 {
		meta.ReplayGainAlbum = parseReplayGain(get("replaygain_album_gain"))
	}
}

// readM4A reads tags from an M4A/AAC file using dhowden/tag.
// MusicBrainz IDs are extracted from the Raw() map using iTunes freeform atom names.
func readM4A(path string) (*service.SongMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("tags: open %s: %w", path, err)
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil && !errors.Is(err, tag.ErrNoTagsFound) {
		return nil, fmt.Errorf("tags: read %s: %w", path, err)
	}

	var meta *service.SongMeta
	if m != nil {
		meta = baseMetaFromTag(m)
		enrichFromM4ARaw(m.Raw(), meta)
	} else {
		meta = &service.SongMeta{}
	}

	meta.Format = "m4a"
	meta.ContentType = "audio/mp4"
	meta.FileSize = fileSize(path)

	return meta, nil
}

// enrichFromM4ARaw extracts MusicBrainz IDs from iTunes freeform atoms in the Raw() map.
// dhowden/tag stores MP4 free-form atoms with the key "----:com.apple.iTunes:<name>".
func enrichFromM4ARaw(raw map[string]interface{}, meta *service.SongMeta) {
	get := func(key string) string {
		if v, ok := raw[key]; ok {
			if s, ok := v.(string); ok {
				return sanitize(s)
			}
		}
		return ""
	}

	if v := get("----:com.apple.iTunes:MusicBrainz Track Id"); v != "" {
		meta.MBTrackID = v
	}
	if v := get("----:com.apple.iTunes:MusicBrainz Artist Id"); v != "" {
		meta.MBArtistID = v
	}
	if v := get("----:com.apple.iTunes:MusicBrainz Album Id"); v != "" {
		meta.MBAlbumID = v
	}
	if v := get("----:com.apple.iTunes:MusicBrainz Album Artist Id"); v != "" {
		meta.MBAlbumArtistID = v
	}
}

// readGeneric reads tags from a WAV or AIFF file using dhowden/tag.
func readGeneric(path string, format string, contentType string) (*service.SongMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("tags: open %s: %w", path, err)
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil && !errors.Is(err, tag.ErrNoTagsFound) {
		return nil, fmt.Errorf("tags: read %s: %w", path, err)
	}

	var meta *service.SongMeta
	if m != nil {
		meta = baseMetaFromTag(m)
	} else {
		meta = &service.SongMeta{}
	}

	meta.Format = format
	meta.ContentType = contentType
	meta.FileSize = fileSize(path)

	return meta, nil
}
