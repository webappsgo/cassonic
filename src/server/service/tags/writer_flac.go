package tags

import (
	"fmt"
	"os"
	"strings"

	flac "github.com/go-flac/go-flac"
	"github.com/go-flac/flacvorbis"
)

// vorbisFieldMap maps SongMeta field names to their canonical Vorbis Comment keys.
var vorbisFieldMap = map[string]string{
	"Title":           "TITLE",
	"Artist":          "ARTIST",
	"AlbumArtist":     "ALBUMARTIST",
	"Album":           "ALBUM",
	"TrackNumber":     "TRACKNUMBER",
	"DiscNumber":      "DISCNUMBER",
	"Year":            "DATE",
	"Genre":           "GENRE",
	"Composer":        "COMPOSER",
	"Lyricist":        "LYRICIST",
	"Conductor":       "CONDUCTOR",
	"Comment":         "COMMENT",
	"Lyrics":          "LYRICS",
	"BPM":             "BPM",
	"ReplayGainTrack": "REPLAYGAIN_TRACK_GAIN",
	"ReplayGainAlbum": "REPLAYGAIN_ALBUM_GAIN",
	"MBTrackID":       "MUSICBRAINZ_TRACKID",
	"MBArtistID":      "MUSICBRAINZ_ARTISTID",
	"MBAlbumID":       "MUSICBRAINZ_ALBUMID",
	"MBAlbumArtistID": "MUSICBRAINZ_ALBUMARTISTID",
}

// writeFLAC writes the provided fields to a FLAC file via Vorbis Comments.
// The write is atomic: content is marshalled to a temp file then renamed over the original.
func writeFLAC(path string, fields WritableFields) error {
	ff, err := flac.ParseFile(path)
	if err != nil {
		return fmt.Errorf("tags: parse flac %s: %w", path, err)
	}

	// Find the existing Vorbis Comment block, or prepare to create one.
	vcIdx := -1
	var vc *flacvorbis.MetaDataBlockVorbisComment
	for i, block := range ff.Meta {
		if block.Type == flac.VorbisComment {
			vcIdx = i
			vc, err = flacvorbis.ParseFromMetaDataBlock(*block)
			if err != nil {
				return fmt.Errorf("tags: parse vorbis comment in %s: %w", path, err)
			}
			break
		}
	}
	if vc == nil {
		vc = flacvorbis.New()
	}

	// Remove existing entries for keys we are about to overwrite.
	for field := range fields {
		vcKey, ok := vorbisFieldMap[field]
		if !ok {
			continue
		}
		vc.Comments = filterVorbisComments(vc.Comments, vcKey)
	}

	// Add the new values.
	for field, raw := range fields {
		vcKey, ok := vorbisFieldMap[field]
		if !ok {
			continue
		}
		val := fieldString(raw)
		if addErr := vc.Add(vcKey, val); addErr != nil {
			return fmt.Errorf("tags: add vorbis comment %s in %s: %w", vcKey, path, addErr)
		}
	}

	// Marshal the updated block back into the FLAC metadata slice.
	marshalledBlock := vc.Marshal()
	if vcIdx >= 0 {
		ff.Meta[vcIdx] = &marshalledBlock
	} else {
		ff.Meta = append(ff.Meta, &marshalledBlock)
	}

	// Write atomically: marshal to temp file, then rename over the original.
	tmp := path + ".tags.tmp"
	if err := ff.Save(tmp); err != nil {
		return fmt.Errorf("tags: save flac temp %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("tags: rename flac temp to %s: %w", path, err)
	}

	return nil
}

// filterVorbisComments returns a new comment slice with all entries for key removed.
// Comparison is case-insensitive as per the Vorbis Comment specification.
func filterVorbisComments(comments []string, key string) []string {
	prefix := strings.ToUpper(key) + "="
	out := comments[:0:0]
	for _, c := range comments {
		if !strings.HasPrefix(strings.ToUpper(c), prefix) {
			out = append(out, c)
		}
	}
	return out
}
