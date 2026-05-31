package ampache

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"time"

	"github.com/local/cassonic/src/server/model"
)

// ampacheAPIVersion is the API version string reported in handshake responses.
const ampacheAPIVersion = "6.0.0"

// sessionTTL is the default Ampache session lifetime.
const sessionTTL = 3 * time.Hour

// AmpError represents an Ampache protocol error.
type AmpError struct {
	XMLName      xml.Name `xml:"error" json:"-"`
	ErrorCode    int      `xml:"errorCode,attr" json:"errorCode"`
	ErrorMessage string   `xml:",chardata" json:"errorMessage"`
}

// HandshakeResp holds handshake response data.
type HandshakeResp struct {
	Auth            string `xml:"auth" json:"auth"`
	API             string `xml:"api" json:"api"`
	SessionExpire   string `xml:"session_expire" json:"session_expire"`
	Update          string `xml:"update" json:"update"`
	Add             string `xml:"add" json:"add"`
	Clean           string `xml:"clean" json:"clean"`
	Songs           int    `xml:"songs" json:"songs"`
	Artists         int    `xml:"artists" json:"artists"`
	Albums          int    `xml:"albums" json:"albums"`
	Playlists       int    `xml:"playlists" json:"playlists"`
	Videos          int    `xml:"videos" json:"videos"`
	Catalogs        int    `xml:"catalogs" json:"catalogs"`
	Podcasts        int    `xml:"podcasts" json:"podcasts"`
	PodcastEpisodes int    `xml:"podcast_episodes" json:"podcast_episodes"`
}

// AmpSong is the Ampache wire representation of a song.
type AmpSong struct {
	XMLName     xml.Name `xml:"song" json:"-"`
	ID          string   `xml:"id,attr" json:"id"`
	Title       string   `xml:"title" json:"title"`
	Name        string   `xml:"name" json:"name"`
	Artist      AmpRef   `xml:"artist" json:"artist"`
	Album       AmpRef   `xml:"album" json:"album"`
	AlbumArtist AmpRef   `xml:"albumartist" json:"albumartist"`
	Disk        int      `xml:"disk" json:"disk"`
	Track       int      `xml:"track" json:"track"`
	Year        int      `xml:"year" json:"year"`
	Genre       []AmpRef `xml:"genre" json:"genre"`
	Duration    int      `xml:"duration" json:"duration"`
	BitRate     int      `xml:"bitrate" json:"bitrate"`
	Rate        int      `xml:"rate" json:"rate"`
	Mode        string   `xml:"mode" json:"mode"`
	Channels    int      `xml:"channels" json:"channels"`
	Mime        string   `xml:"mime" json:"mime"`
	URL         string   `xml:"url" json:"url"`
	Size        int64    `xml:"size" json:"size"`
	MBTrackID   string   `xml:"mbid" json:"mbid"`
	Art         string   `xml:"art" json:"art"`
	Flag        int      `xml:"flag" json:"flag"`
	Preciserating int    `xml:"preciserating" json:"preciserating"`
	Rating      int      `xml:"rating" json:"rating"`
	Composer    string   `xml:"composer" json:"composer"`
	Lyrics      string   `xml:"lyrics" json:"lyrics"`
	Playcount   int      `xml:"playcount" json:"playcount"`
	Catalog     string   `xml:"catalog" json:"catalog"`
	ReplayGain  float64  `xml:"replaygain_track_gain" json:"replaygain_track_gain"`
}

// AmpAlbum is the Ampache wire representation of an album.
type AmpAlbum struct {
	XMLName    xml.Name `xml:"album" json:"-"`
	ID         string   `xml:"id,attr" json:"id"`
	Name       string   `xml:"name" json:"name"`
	Artist     AmpRef   `xml:"artist" json:"artist"`
	Year       int      `xml:"year" json:"year"`
	Genre      []AmpRef `xml:"genre" json:"genre"`
	SongCount  int      `xml:"songcount" json:"songcount"`
	Duration   int      `xml:"time" json:"time"`
	Art        string   `xml:"art" json:"art"`
	MBAlbumID  string   `xml:"mbid" json:"mbid"`
	Rating     int      `xml:"rating" json:"rating"`
	Flag       int      `xml:"flag" json:"flag"`
}

// AmpArtist is the Ampache wire representation of an artist.
type AmpArtist struct {
	XMLName    xml.Name `xml:"artist" json:"-"`
	ID         string   `xml:"id,attr" json:"id"`
	Name       string   `xml:"name" json:"name"`
	AlbumCount int      `xml:"albumcount" json:"albumcount"`
	SongCount  int      `xml:"songcount" json:"songcount"`
	Art        string   `xml:"art" json:"art"`
	Summary    string   `xml:"summary" json:"summary"`
	MBID       string   `xml:"mbid" json:"mbid"`
	Rating     int      `xml:"rating" json:"rating"`
	Flag       int      `xml:"flag" json:"flag"`
}

// AmpRef is a lightweight reference to another entity used inside compound types.
type AmpRef struct {
	ID   string `xml:"id,attr" json:"id"`
	Name string `xml:",chardata" json:"name"`
}

// AmpGenre is the Ampache wire representation of a genre (also used as label).
type AmpGenre struct {
	XMLName    xml.Name `xml:"genre" json:"-"`
	ID         string   `xml:"id,attr" json:"id"`
	Name       string   `xml:"name" json:"name"`
	SongCount  int      `xml:"songs" json:"songs"`
	AlbumCount int      `xml:"albums" json:"albums"`
	ArtistCount int     `xml:"artists" json:"artists"`
}

// AmpCatalog is the Ampache wire representation of a music library.
type AmpCatalog struct {
	XMLName   xml.Name `xml:"catalog" json:"-"`
	ID        string   `xml:"id,attr" json:"id"`
	Name      string   `xml:"name" json:"name"`
	Type      string   `xml:"catalog_type" json:"catalog_type"`
	LastUpdate string  `xml:"last_update" json:"last_update"`
	LastAdd    string  `xml:"last_add" json:"last_add"`
	LastClean  string  `xml:"last_clean" json:"last_clean"`
	Enabled    int     `xml:"enabled" json:"enabled"`
	Path       string  `xml:"path" json:"path"`
}

// AmpUser is the Ampache wire representation of a user account.
type AmpUser struct {
	XMLName     xml.Name `xml:"user" json:"-"`
	ID          string   `xml:"id,attr" json:"id"`
	Username    string   `xml:"username" json:"username"`
	Auth        string   `xml:"auth" json:"auth"`
	Email       string   `xml:"email" json:"email"`
	Access      int      `xml:"access" json:"access"`
	FullName    string   `xml:"fullname" json:"fullname"`
	CanDownload int      `xml:"download" json:"download"`
	CanUpload   int      `xml:"upload" json:"upload"`
	Disabled    int      `xml:"disabled" json:"disabled"`
}

// AmpPreference is the Ampache wire representation of a user or server preference.
type AmpPreference struct {
	XMLName     xml.Name `xml:"preference" json:"-"`
	ID          string   `xml:"id,attr" json:"id"`
	Name        string   `xml:"name" json:"name"`
	Value       string   `xml:"value" json:"value"`
	Description string   `xml:"description" json:"description"`
	Level       int      `xml:"level" json:"level"`
	Type        string   `xml:"type" json:"type"`
	Category    string   `xml:"category" json:"category"`
}

// AmpShare is the Ampache wire representation of a share link.
type AmpShare struct {
	XMLName     xml.Name `xml:"share" json:"-"`
	ID          string   `xml:"id,attr" json:"id"`
	Name        string   `xml:"name" json:"name"`
	Owner       string   `xml:"owner" json:"owner"`
	AllowStream int      `xml:"allow_stream" json:"allow_stream"`
	AllowDownload int    `xml:"allow_download" json:"allow_download"`
	Expire      string   `xml:"expire" json:"expire"`
	PublicURL   string   `xml:"public_url" json:"public_url"`
	Creation    string   `xml:"creation" json:"creation"`
	LastVisit   string   `xml:"lastvisit" json:"lastvisit"`
	ObjectType  string   `xml:"object_type" json:"object_type"`
	ObjectID    string   `xml:"object_id" json:"object_id"`
}

// AmpBookmark is the Ampache wire representation of a playback bookmark.
type AmpBookmark struct {
	XMLName    xml.Name `xml:"bookmark" json:"-"`
	ID         string   `xml:"id,attr" json:"id"`
	Owner      string   `xml:"owner" json:"owner"`
	ObjectType string   `xml:"object_type" json:"object_type"`
	ObjectID   string   `xml:"object_id" json:"object_id"`
	Position   int64    `xml:"position" json:"position"`
	Comment    string   `xml:"comment" json:"comment"`
	Creation   string   `xml:"creation" json:"creation"`
	Update     string   `xml:"update" json:"update"`
}

// AmpPlaylist is the Ampache wire representation of a playlist.
type AmpPlaylist struct {
	XMLName   xml.Name `xml:"playlist" json:"-"`
	ID        string   `xml:"id,attr" json:"id"`
	Name      string   `xml:"name" json:"name"`
	Owner     string   `xml:"owner" json:"owner"`
	Items     int      `xml:"items" json:"items"`
	Type      string   `xml:"type" json:"type"`
	Duration  int      `xml:"duration" json:"duration"`
	Art       string   `xml:"art" json:"art"`
}

// AmpPodcast is the Ampache wire representation of a podcast channel.
type AmpPodcast struct {
	XMLName      xml.Name      `xml:"podcast" json:"-"`
	ID           string        `xml:"id,attr" json:"id"`
	Name         string        `xml:"name" json:"name"`
	Description  string        `xml:"description" json:"description"`
	Language     string        `xml:"language" json:"language"`
	Copyright    string        `xml:"copyright" json:"copyright"`
	FeedURL      string        `xml:"feed_url" json:"feed_url"`
	Generator    string        `xml:"generator" json:"generator"`
	Website      string        `xml:"website" json:"website"`
	Build        string        `xml:"build" json:"build"`
	Status       string        `xml:"status" json:"status"`
	Episodes     []AmpPodcastEpisode `xml:"episode,omitempty" json:"episode,omitempty"`
}

// AmpPodcastEpisode is the Ampache wire representation of a podcast episode.
type AmpPodcastEpisode struct {
	XMLName     xml.Name `xml:"podcast_episode" json:"-"`
	ID          string   `xml:"id,attr" json:"id"`
	Title       string   `xml:"title" json:"title"`
	Name        string   `xml:"name" json:"name"`
	Description string   `xml:"description" json:"description"`
	Category    string   `xml:"category" json:"category"`
	Author      string   `xml:"author" json:"author"`
	Website     string   `xml:"website" json:"website"`
	PubDate     string   `xml:"pubdate" json:"pubdate"`
	State       string   `xml:"state" json:"state"`
	FileSize    int64    `xml:"filelength" json:"filelength"`
	Duration    int      `xml:"time" json:"time"`
	Mime        string   `xml:"mime" json:"mime"`
	URL         string   `xml:"url" json:"url"`
	Podcast     AmpRef   `xml:"podcast" json:"podcast"`
}

// AmpLiveStream is the Ampache wire representation of an internet radio station.
type AmpLiveStream struct {
	XMLName   xml.Name `xml:"live_stream" json:"-"`
	ID        string   `xml:"id,attr" json:"id"`
	Name      string   `xml:"name" json:"name"`
	Codec     string   `xml:"codec" json:"codec"`
	BitRate   int      `xml:"bitrate" json:"bitrate"`
	Sampling  int      `xml:"sampling" json:"sampling"`
	URL       string   `xml:"url" json:"url"`
	SiteURL   string   `xml:"site_url" json:"site_url"`
	IsPublic  int      `xml:"is_public" json:"is_public"`
	Catalog   int      `xml:"catalog" json:"catalog"`
}

// AmpNowPlaying is the Ampache wire representation of a current stream.
type AmpNowPlaying struct {
	XMLName   xml.Name `xml:"song" json:"-"`
	ID        string   `xml:"id,attr" json:"id"`
	Title     string   `xml:"title" json:"title"`
	Artist    AmpRef   `xml:"artist" json:"artist"`
	Album     AmpRef   `xml:"album" json:"album"`
	Client    string   `xml:"client" json:"client"`
	Expire    int      `xml:"expire" json:"expire"`
	UserID    string   `xml:"user_id" json:"user_id"`
	Username  string   `xml:"username" json:"username"`
}

// AmpStats is the Ampache wire representation of server statistics.
type AmpStats struct {
	XMLName  xml.Name `xml:"stats" json:"-"`
	Songs    int      `xml:"songs" json:"songs"`
	Albums   int      `xml:"albums" json:"albums"`
	Artists  int      `xml:"artists" json:"artists"`
	Genres   int      `xml:"tags" json:"tags"`
	Catalogs int      `xml:"catalogs" json:"catalogs"`
}

// ampacheRoot wraps any payload for XML marshaling under a <root> element.
type ampacheRoot struct {
	XMLName xml.Name
	Payload any
}

// ampacheXMLWrapper is a typed XML marshaling envelope for a known payload type.
type ampacheXMLWrapper struct {
	XMLName xml.Name
	Data    []byte `xml:",innerxml"`
}

// errResp builds an AmpError with the given code and message.
func errResp(code int, msg string) *AmpError {
	return &AmpError{ErrorCode: code, ErrorMessage: msg}
}

// respond serializes v as JSON or XML and writes HTTP 200.
// Ampache protocol: HTTP 200 even for errors.
func respond(w http.ResponseWriter, r *http.Request, isJSON bool, v any) {
	if isJSON {
		respondJSON(w, r, v)
	} else {
		respondXML(w, r, v)
	}
}

// respondJSON serializes v as a JSON object and writes HTTP 200.
func respondJSON(w http.ResponseWriter, _ *http.Request, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// respondXML serializes v wrapped in <root> and writes HTTP 200.
func respondXML(w http.ResponseWriter, _ *http.Request, v any) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	switch payload := v.(type) {
	case *AmpError:
		_ = xml.NewEncoder(w).Encode(struct {
			XMLName xml.Name `xml:"root"`
			Error   *AmpError `xml:"error"`
		}{Error: payload})
	case map[string]any:
		_ = xml.NewEncoder(w).Encode(struct {
			XMLName xml.Name `xml:"root"`
			Data    map[string]any
		}{Data: payload})
	default:
		_ = xml.NewEncoder(w).Encode(v)
	}
}

// okResp returns a standard Ampache success envelope for simple responses.
func okResp(key string, value any) map[string]any {
	return map[string]any{key: value}
}

// boolInt converts a bool to 1 or 0.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// songToAmp converts a model.Song to the Ampache wire type.
// baseURL is the server base URL used to build stream and art URLs.
func songToAmp(s *model.Song, baseURL string) AmpSong {
	genre := []AmpRef{}
	if s.Genre != "" {
		genre = []AmpRef{{ID: "0", Name: s.Genre}}
	}
	return AmpSong{
		ID:          itoa(s.ID),
		Title:       s.Title,
		Name:        s.Title,
		Artist:      AmpRef{ID: itoa(s.ArtistID), Name: s.ArtistName},
		Album:       AmpRef{ID: itoa(s.AlbumID), Name: s.AlbumName},
		AlbumArtist: AmpRef{ID: itoa(s.AlbumArtistID), Name: s.AlbumArtistName},
		Disk:        s.DiscNumber,
		Track:       s.TrackNumber,
		Year:        s.Year,
		Genre:       genre,
		Duration:    s.Duration,
		BitRate:     s.BitRate,
		Rate:        s.SampleRate,
		Mode:        "vbr",
		Channels:    s.Channels,
		Mime:        s.ContentType,
		URL:         baseURL + "/server/json.server.php?action=stream&id=" + itoa(s.ID),
		Size:        s.FileSize,
		MBTrackID:   s.MBTrackID,
		Art:         baseURL + "/server/json.server.php?action=get_art&id=" + itoa(s.ID) + "&type=song",
		Composer:    s.Composer,
		Lyrics:      s.Lyrics,
		Catalog:     itoa(s.LibraryID),
		ReplayGain:  s.ReplayGainTrack,
	}
}

// albumToAmp converts a model.Album to the Ampache wire type.
func albumToAmp(a *model.Album, baseURL string) AmpAlbum {
	genre := []AmpRef{}
	if a.Genre != "" {
		genre = []AmpRef{{ID: "0", Name: a.Genre}}
	}
	return AmpAlbum{
		ID:        itoa(a.ID),
		Name:      a.Title,
		Artist:    AmpRef{ID: itoa(a.ArtistID), Name: a.ArtistName},
		Year:      a.Year,
		Genre:     genre,
		SongCount: a.SongCount,
		Duration:  a.Duration,
		Art:       baseURL + "/server/json.server.php?action=get_art&id=" + itoa(a.ID) + "&type=album",
		MBAlbumID: a.MusicBrainzID,
	}
}

// artistToAmp converts a model.Artist to the Ampache wire type.
func artistToAmp(a *model.Artist, baseURL string) AmpArtist {
	return AmpArtist{
		ID:         itoa(a.ID),
		Name:       a.Name,
		AlbumCount: a.AlbumCount,
		SongCount:  a.SongCount,
		Art:        baseURL + "/server/json.server.php?action=get_art&id=" + itoa(a.ID) + "&type=artist",
		Summary:    a.Biography,
		MBID:       a.MusicBrainzID,
	}
}

// itoa converts an int64 to its string representation.
func itoa(n int64) string {
	return strconv64(n)
}

// strconv64 is a minimal int64-to-string helper that avoids importing strconv at package level.
func strconv64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
