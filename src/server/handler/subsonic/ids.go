package subsonic

import (
	"fmt"
	"strconv"
	"strings"
)

// ID prefix constants used to encode DB integer IDs as Subsonic string IDs.
const (
	prefixSong    = "so-"
	prefixAlbum   = "al-"
	prefixArtist  = "ar-"
	prefixLibrary = "li-"
	prefixPodcast = "pc-"
	prefixEpisode = "ep-"
	prefixShare   = "sh-"
	prefixRadio   = "ra-"
)

// encodeSongID encodes a song DB ID as a Subsonic string ID.
func encodeSongID(id int64) string {
	return fmt.Sprintf("%s%d", prefixSong, id)
}

// encodeAlbumID encodes an album DB ID as a Subsonic string ID.
func encodeAlbumID(id int64) string {
	return fmt.Sprintf("%s%d", prefixAlbum, id)
}

// encodeArtistID encodes an artist DB ID as a Subsonic string ID.
func encodeArtistID(id int64) string {
	return fmt.Sprintf("%s%d", prefixArtist, id)
}

// encodeLibraryID encodes a library DB ID as a Subsonic string ID.
func encodeLibraryID(id int64) string {
	return fmt.Sprintf("%s%d", prefixLibrary, id)
}

// encodePodcastID encodes a podcast channel DB ID as a Subsonic string ID.
func encodePodcastID(id int64) string {
	return fmt.Sprintf("%s%d", prefixPodcast, id)
}

// encodeEpisodeID encodes a podcast episode DB ID as a Subsonic string ID.
func encodeEpisodeID(id int64) string {
	return fmt.Sprintf("%s%d", prefixEpisode, id)
}

// encodeShareID encodes a share DB ID as a Subsonic string ID.
func encodeShareID(id int64) string {
	return fmt.Sprintf("%s%d", prefixShare, id)
}

// encodeRadioID encodes a radio station DB ID as a Subsonic string ID.
func encodeRadioID(id int64) string {
	return fmt.Sprintf("%s%d", prefixRadio, id)
}

// decodeID parses a prefixed Subsonic string ID and returns the numeric DB ID
// and the recognized prefix string. Returns an error for malformed input.
func decodeID(id string) (prefix string, dbID int64, err error) {
	for _, p := range []string{
		prefixSong, prefixAlbum, prefixArtist, prefixLibrary,
		prefixPodcast, prefixEpisode, prefixShare, prefixRadio,
	} {
		if strings.HasPrefix(id, p) {
			raw := id[len(p):]
			n, parseErr := strconv.ParseInt(raw, 10, 64)
			if parseErr != nil {
				return "", 0, fmt.Errorf("decode id %q: %w", id, parseErr)
			}
			return p, n, nil
		}
	}
	// Fallback: try to parse as a bare integer for legacy clients.
	n, parseErr := strconv.ParseInt(id, 10, 64)
	if parseErr != nil {
		return "", 0, fmt.Errorf("decode id %q: unrecognized format", id)
	}
	return "", n, nil
}

// decodeSongID extracts a song DB ID from a Subsonic string ID.
func decodeSongID(id string) (int64, error) {
	_, n, err := decodeID(id)
	return n, err
}

// decodeAlbumID extracts an album DB ID from a Subsonic string ID.
func decodeAlbumID(id string) (int64, error) {
	_, n, err := decodeID(id)
	return n, err
}

// decodeArtistID extracts an artist DB ID from a Subsonic string ID.
func decodeArtistID(id string) (int64, error) {
	_, n, err := decodeID(id)
	return n, err
}

// decodePodcastID extracts a podcast channel DB ID from a Subsonic string ID.
func decodePodcastID(id string) (int64, error) {
	_, n, err := decodeID(id)
	return n, err
}

// decodeEpisodeID extracts a podcast episode DB ID from a Subsonic string ID.
func decodeEpisodeID(id string) (int64, error) {
	_, n, err := decodeID(id)
	return n, err
}

// decodeShareID extracts a share DB ID from a Subsonic string ID.
func decodeShareID(id string) (int64, error) {
	_, n, err := decodeID(id)
	return n, err
}

// decodeRadioID extracts a radio station DB ID from a Subsonic string ID.
func decodeRadioID(id string) (int64, error) {
	_, n, err := decodeID(id)
	return n, err
}
