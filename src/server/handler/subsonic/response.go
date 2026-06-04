package subsonic

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
)

// SubsonicResponse is the root response wrapper for all Subsonic API responses.
type SubsonicResponse struct {
	XMLName xml.Name `xml:"subsonic-response" json:"-"`
	XMLNS   string   `xml:"xmlns,attr,omitempty" json:"-"`
	Status  string   `xml:"status,attr" json:"status"`
	Version string   `xml:"version,attr" json:"version"`

	Error          *SubsonicError         `xml:"error,omitempty" json:"error,omitempty"`
	License        *License               `xml:"license,omitempty" json:"license,omitempty"`
	MusicFolders   *MusicFolders          `xml:"musicFolders,omitempty" json:"musicFolders,omitempty"`
	Indexes        *Indexes               `xml:"indexes,omitempty" json:"indexes,omitempty"`
	Directory      *Directory             `xml:"directory,omitempty" json:"directory,omitempty"`
	Genres         *Genres                `xml:"genres,omitempty" json:"genres,omitempty"`
	Artists        *ArtistsID3            `xml:"artists,omitempty" json:"artists,omitempty"`
	Artist         *ArtistWithAlbumsID3   `xml:"artist,omitempty" json:"artist,omitempty"`
	Album          *AlbumWithSongsID3     `xml:"album,omitempty" json:"album,omitempty"`
	Song           *Child                 `xml:"song,omitempty" json:"song,omitempty"`
	AlbumList      *AlbumList             `xml:"albumList,omitempty" json:"albumList,omitempty"`
	AlbumList2     *AlbumList2            `xml:"albumList2,omitempty" json:"albumList2,omitempty"`
	RandomSongs    *Songs                 `xml:"randomSongs,omitempty" json:"randomSongs,omitempty"`
	SongsByGenre   *Songs                 `xml:"songsByGenre,omitempty" json:"songsByGenre,omitempty"`
	NowPlaying     *NowPlayingResp        `xml:"nowPlaying,omitempty" json:"nowPlaying,omitempty"`
	Starred        *Starred               `xml:"starred,omitempty" json:"starred,omitempty"`
	Starred2       *Starred2              `xml:"starred2,omitempty" json:"starred2,omitempty"`
	SearchResult   *SearchResult          `xml:"searchResult,omitempty" json:"searchResult,omitempty"`
	SearchResult2  *SearchResult2         `xml:"searchResult2,omitempty" json:"searchResult2,omitempty"`
	SearchResult3  *SearchResult3         `xml:"searchResult3,omitempty" json:"searchResult3,omitempty"`
	Playlists      *Playlists             `xml:"playlists,omitempty" json:"playlists,omitempty"`
	Playlist       *PlaylistWithEntries   `xml:"playlist,omitempty" json:"playlist,omitempty"`
	ScanStatus     *ScanStatusResp        `xml:"scanStatus,omitempty" json:"scanStatus,omitempty"`
	User           *UserResp              `xml:"user,omitempty" json:"user,omitempty"`
	Users          *UsersResp             `xml:"users,omitempty" json:"users,omitempty"`
	Shares         *Shares                `xml:"shares,omitempty" json:"shares,omitempty"`
	Share          *ShareEntry            `xml:"share,omitempty" json:"share,omitempty"`
	Bookmarks      *Bookmarks             `xml:"bookmarks,omitempty" json:"bookmarks,omitempty"`
	PlayQueue      *PlayQueueResp         `xml:"playQueue,omitempty" json:"playQueue,omitempty"`
	ChatMessages   *ChatMessages          `xml:"chatMessages,omitempty" json:"chatMessages,omitempty"`
	Podcasts       *PodcastsResp          `xml:"podcasts,omitempty" json:"podcasts,omitempty"`
	NewestPodcasts *NewestPodcasts        `xml:"newestPodcasts,omitempty" json:"newestPodcasts,omitempty"`
	RadioStations  *InternetRadioStations `xml:"internetRadioStations,omitempty" json:"internetRadioStations,omitempty"`
	ArtistInfo     *ArtistInfo            `xml:"artistInfo,omitempty" json:"artistInfo,omitempty"`
	ArtistInfo2    *ArtistInfo2           `xml:"artistInfo2,omitempty" json:"artistInfo2,omitempty"`
	AlbumInfo      *AlbumInfo             `xml:"albumInfo,omitempty" json:"albumInfo,omitempty"`
	SimilarSongs   *SimilarSongs          `xml:"similarSongs,omitempty" json:"similarSongs,omitempty"`
	SimilarSongs2  *SimilarSongs2         `xml:"similarSongs2,omitempty" json:"similarSongs2,omitempty"`
	TopSongs       *TopSongs              `xml:"topSongs,omitempty" json:"topSongs,omitempty"`
	Lyrics         *Lyrics                `xml:"lyrics,omitempty" json:"lyrics,omitempty"`
	Videos              *Videos                   `xml:"videos,omitempty" json:"videos,omitempty"`
	OpenSubsonicExtensions *OpenSubsonicExtensions `xml:"openSubsonicExtensions,omitempty" json:"openSubsonicExtensions,omitempty"`
}

// SubsonicError carries the numeric error code and human-readable message.
type SubsonicError struct {
	Code    int    `xml:"code,attr" json:"code"`
	Message string `xml:"message,attr" json:"message"`
}

// Subsonic error codes as defined in the Subsonic API specification.
const (
	ErrGeneric              = 0
	ErrMissingParam         = 10
	ErrWrongVersion         = 20
	ErrNotAuthenticated     = 30
	ErrWrongCredentials     = 40
	ErrTokenNotSupported    = 41
	ErrForbidden            = 50
	ErrNotFound             = 70
)

// Child represents a song or directory entry in Subsonic browse and search responses.
type Child struct {
	ID               string `xml:"id,attr" json:"id"`
	Parent           string `xml:"parent,attr,omitempty" json:"parent,omitempty"`
	IsDir            bool   `xml:"isDir,attr" json:"isDir"`
	Title            string `xml:"title,attr" json:"title"`
	Album            string `xml:"album,attr,omitempty" json:"album,omitempty"`
	Artist           string `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	Track            int    `xml:"track,attr,omitempty" json:"track,omitempty"`
	Year             int    `xml:"year,attr,omitempty" json:"year,omitempty"`
	Genre            string `xml:"genre,attr,omitempty" json:"genre,omitempty"`
	CoverArt         string `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Size             int64  `xml:"size,attr,omitempty" json:"size,omitempty"`
	ContentType      string `xml:"contentType,attr,omitempty" json:"contentType,omitempty"`
	Suffix           string `xml:"suffix,attr,omitempty" json:"suffix,omitempty"`
	Duration         int    `xml:"duration,attr,omitempty" json:"duration,omitempty"`
	BitRate          int    `xml:"bitRate,attr,omitempty" json:"bitRate,omitempty"`
	Path             string `xml:"path,attr,omitempty" json:"path,omitempty"`
	PlayCount        int    `xml:"playCount,attr,omitempty" json:"playCount,omitempty"`
	Starred          string `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	AlbumID          string `xml:"albumId,attr,omitempty" json:"albumId,omitempty"`
	ArtistID         string `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	Type             string `xml:"type,attr,omitempty" json:"type,omitempty"`
	UserRating       int    `xml:"userRating,attr,omitempty" json:"userRating,omitempty"`
	Composer         string `xml:"composer,attr,omitempty" json:"composer,omitempty"`
	DiscNumber       int    `xml:"discNumber,attr,omitempty" json:"discNumber,omitempty"`
	BookmarkPosition int64  `xml:"bookmarkPosition,attr,omitempty" json:"bookmarkPosition,omitempty"`
}

// ArtistID3 represents an artist in ID3-based browse responses.
type ArtistID3 struct {
	ID         string `xml:"id,attr" json:"id"`
	Name       string `xml:"name,attr" json:"name"`
	CoverArt   string `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	AlbumCount int    `xml:"albumCount,attr" json:"albumCount"`
	Starred    string `xml:"starred,attr,omitempty" json:"starred,omitempty"`
}

// AlbumID3 represents an album in ID3-based browse responses.
type AlbumID3 struct {
	ID        string `xml:"id,attr" json:"id"`
	Name      string `xml:"name,attr" json:"name"`
	Artist    string `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	ArtistID  string `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	CoverArt  string `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	SongCount int    `xml:"songCount,attr" json:"songCount"`
	Duration  int    `xml:"duration,attr" json:"duration"`
	Year      int    `xml:"year,attr,omitempty" json:"year,omitempty"`
	Genre     string `xml:"genre,attr,omitempty" json:"genre,omitempty"`
	Starred   string `xml:"starred,attr,omitempty" json:"starred,omitempty"`
}

// License describes the server license status; always valid for open-source builds.
type License struct {
	Valid          bool   `xml:"valid,attr" json:"valid"`
	Email          string `xml:"email,attr,omitempty" json:"email,omitempty"`
	LicenseExpires string `xml:"licenseExpires,attr,omitempty" json:"licenseExpires,omitempty"`
}

// MusicFolders is the container for music folder (library) entries.
type MusicFolders struct {
	MusicFolder []MusicFolder `xml:"musicFolder" json:"musicFolder"`
}

// MusicFolder represents one library root directory.
type MusicFolder struct {
	ID   int    `xml:"id,attr" json:"id"`
	Name string `xml:"name,attr" json:"name"`
}

// Indexes groups artists by their first letter for fast index browsing.
type Indexes struct {
	LastModified        int64          `xml:"lastModified,attr" json:"lastModified"`
	IgnoredArticles     string         `xml:"ignoredArticles,attr" json:"ignoredArticles"`
	Index               []IndexEntry   `xml:"index,omitempty" json:"index,omitempty"`
	Child               []Child        `xml:"child,omitempty" json:"child,omitempty"`
}

// IndexEntry is one letter-group within an Indexes response.
type IndexEntry struct {
	Name   string  `xml:"name,attr" json:"name"`
	Artist []Child `xml:"artist" json:"artist"`
}

// ArtistsID3 groups ID3 artists by first letter.
type ArtistsID3 struct {
	IgnoredArticles string           `xml:"ignoredArticles,attr" json:"ignoredArticles"`
	Index           []ArtistIndex    `xml:"index,omitempty" json:"index,omitempty"`
}

// ArtistIndex groups ID3 artists under one letter.
type ArtistIndex struct {
	Name   string      `xml:"name,attr" json:"name"`
	Artist []ArtistID3 `xml:"artist" json:"artist"`
}

// ArtistWithAlbumsID3 is an artist with its full album list.
type ArtistWithAlbumsID3 struct {
	ArtistID3
	Album []AlbumID3 `xml:"album,omitempty" json:"album,omitempty"`
}

// AlbumWithSongsID3 is an album with its full song list.
type AlbumWithSongsID3 struct {
	AlbumID3
	Song []Child `xml:"song,omitempty" json:"song,omitempty"`
}

// Directory represents a browse-by-folder directory listing.
type Directory struct {
	ID        string  `xml:"id,attr" json:"id"`
	Parent    string  `xml:"parent,attr,omitempty" json:"parent,omitempty"`
	Name      string  `xml:"name,attr" json:"name"`
	Starred   string  `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	PlayCount int     `xml:"playCount,attr,omitempty" json:"playCount,omitempty"`
	Child     []Child `xml:"child,omitempty" json:"child,omitempty"`
}

// Genre holds one genre with its content counts.
type Genre struct {
	SongCount  int    `xml:"songCount,attr" json:"songCount"`
	AlbumCount int    `xml:"albumCount,attr" json:"albumCount"`
	Value      string `xml:",chardata" json:"value"`
}

// Genres is the container for genre entries.
type Genres struct {
	Genre []Genre `xml:"genre" json:"genre"`
}

// AlbumList is the container for legacy (non-ID3) album list responses.
type AlbumList struct {
	Album []Child `xml:"album,omitempty" json:"album,omitempty"`
}

// AlbumList2 is the container for ID3-based album list responses.
type AlbumList2 struct {
	Album []AlbumID3 `xml:"album,omitempty" json:"album,omitempty"`
}

// Songs is a generic container for a list of Child entries.
type Songs struct {
	Song []Child `xml:"song,omitempty" json:"song,omitempty"`
}

// NowPlayingResp is the container for active stream entries.
type NowPlayingResp struct {
	Entry []NowPlayingEntryResp `xml:"entry,omitempty" json:"entry,omitempty"`
}

// NowPlayingEntryResp extends Child with streaming session metadata for API responses.
type NowPlayingEntryResp struct {
	Child
	Username   string `xml:"username,attr" json:"username"`
	MinutesAgo int    `xml:"minutesAgo,attr" json:"minutesAgo"`
	PlayerID   int    `xml:"playerId,attr" json:"playerId"`
	PlayerName string `xml:"playerName,attr,omitempty" json:"playerName,omitempty"`
}

// Starred groups starred songs, albums, and artists (legacy folder-based IDs).
type Starred struct {
	Artist []Child   `xml:"artist,omitempty" json:"artist,omitempty"`
	Album  []Child   `xml:"album,omitempty" json:"album,omitempty"`
	Song   []Child   `xml:"song,omitempty" json:"song,omitempty"`
}

// Starred2 groups starred songs, albums, and artists (ID3-based IDs).
type Starred2 struct {
	Artist []ArtistID3 `xml:"artist,omitempty" json:"artist,omitempty"`
	Album  []AlbumID3  `xml:"album,omitempty" json:"album,omitempty"`
	Song   []Child     `xml:"song,omitempty" json:"song,omitempty"`
}

// SearchResult is the legacy v1 search response.
type SearchResult struct {
	Offset      int     `xml:"offset,attr" json:"offset"`
	TotalHits   int     `xml:"totalHits,attr" json:"totalHits"`
	Match       []Child `xml:"match,omitempty" json:"match,omitempty"`
}

// SearchResult2 is the search2 response with separate result categories.
type SearchResult2 struct {
	Artist []Child `xml:"artist,omitempty" json:"artist,omitempty"`
	Album  []Child `xml:"album,omitempty" json:"album,omitempty"`
	Song   []Child `xml:"song,omitempty" json:"song,omitempty"`
}

// SearchResult3 is the search3 response with ID3-based result categories.
type SearchResult3 struct {
	Artist []ArtistID3 `xml:"artist,omitempty" json:"artist,omitempty"`
	Album  []AlbumID3  `xml:"album,omitempty" json:"album,omitempty"`
	Song   []Child     `xml:"song,omitempty" json:"song,omitempty"`
}

// Playlists is the container for playlist summary entries.
type Playlists struct {
	Playlist []PlaylistEntry `xml:"playlist,omitempty" json:"playlist,omitempty"`
}

// PlaylistEntry is one playlist in the playlists listing.
type PlaylistEntry struct {
	ID        string `xml:"id,attr" json:"id"`
	Name      string `xml:"name,attr" json:"name"`
	Comment   string `xml:"comment,attr,omitempty" json:"comment,omitempty"`
	Owner     string `xml:"owner,attr,omitempty" json:"owner,omitempty"`
	Public    bool   `xml:"public,attr" json:"public"`
	SongCount int    `xml:"songCount,attr" json:"songCount"`
	Duration  int    `xml:"duration,attr" json:"duration"`
	CoverArt  string `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Created   string `xml:"created,attr,omitempty" json:"created,omitempty"`
	Changed   string `xml:"changed,attr,omitempty" json:"changed,omitempty"`
}

// PlaylistWithEntries is a full playlist with its song entries.
type PlaylistWithEntries struct {
	PlaylistEntry
	Entry []Child `xml:"entry,omitempty" json:"entry,omitempty"`
}

// ScanStatusResp describes the current or most recent scan operation.
type ScanStatusResp struct {
	Scanning bool  `xml:"scanning,attr" json:"scanning"`
	Count    int64 `xml:"count,attr" json:"count"`
}

// UserResp is the Subsonic representation of a user account.
type UserResp struct {
	Username            string `xml:"username,attr" json:"username"`
	Email               string `xml:"email,attr,omitempty" json:"email,omitempty"`
	ScrobblingEnabled   bool   `xml:"scrobblingEnabled,attr" json:"scrobblingEnabled"`
	MaxBitRate          int    `xml:"maxBitRate,attr,omitempty" json:"maxBitRate,omitempty"`
	AdminRole           bool   `xml:"adminRole,attr" json:"adminRole"`
	SettingsRole        bool   `xml:"settingsRole,attr" json:"settingsRole"`
	DownloadRole        bool   `xml:"downloadRole,attr" json:"downloadRole"`
	UploadRole          bool   `xml:"uploadRole,attr" json:"uploadRole"`
	PlaylistRole        bool   `xml:"playlistRole,attr" json:"playlistRole"`
	CoverArtRole        bool   `xml:"coverArtRole,attr" json:"coverArtRole"`
	CommentRole         bool   `xml:"commentRole,attr" json:"commentRole"`
	PodcastRole         bool   `xml:"podcastRole,attr" json:"podcastRole"`
	StreamRole          bool   `xml:"streamRole,attr" json:"streamRole"`
	JukeboxRole         bool   `xml:"jukeboxRole,attr" json:"jukeboxRole"`
	ShareRole           bool   `xml:"shareRole,attr" json:"shareRole"`
	VideoConversionRole bool   `xml:"videoConversionRole,attr" json:"videoConversionRole"`
	Folder              []int  `xml:"folder,omitempty" json:"folder,omitempty"`
}

// UsersResp is the container for multiple user entries.
type UsersResp struct {
	User []UserResp `xml:"user" json:"user"`
}

// Shares is the container for share entries.
type Shares struct {
	Share []ShareEntry `xml:"share,omitempty" json:"share,omitempty"`
}

// ShareEntry represents one public share link.
type ShareEntry struct {
	ID          string  `xml:"id,attr" json:"id"`
	URL         string  `xml:"url,attr" json:"url"`
	Description string  `xml:"description,attr,omitempty" json:"description,omitempty"`
	Username    string  `xml:"username,attr" json:"username"`
	Created     string  `xml:"created,attr,omitempty" json:"created,omitempty"`
	Expires     string  `xml:"expires,attr,omitempty" json:"expires,omitempty"`
	ViewCount   int     `xml:"viewCount,attr" json:"viewCount"`
	Entry       []Child `xml:"entry,omitempty" json:"entry,omitempty"`
}

// Bookmarks is the container for bookmark entries.
type Bookmarks struct {
	Bookmark []BookmarkEntry `xml:"bookmark,omitempty" json:"bookmark,omitempty"`
}

// BookmarkEntry represents one saved playback position.
type BookmarkEntry struct {
	Position int64  `xml:"position,attr" json:"position"`
	Username string `xml:"username,attr" json:"username"`
	Comment  string `xml:"comment,attr,omitempty" json:"comment,omitempty"`
	Created  string `xml:"created,attr,omitempty" json:"created,omitempty"`
	Changed  string `xml:"changed,attr,omitempty" json:"changed,omitempty"`
	Entry    *Child `xml:"entry,omitempty" json:"entry,omitempty"`
}

// PlayQueueResp represents the user's current cross-client play queue.
type PlayQueueResp struct {
	Current   string  `xml:"current,attr,omitempty" json:"current,omitempty"`
	Position  int64   `xml:"position,attr,omitempty" json:"position,omitempty"`
	Username  string  `xml:"username,attr" json:"username"`
	Changed   string  `xml:"changed,attr,omitempty" json:"changed,omitempty"`
	ChangedBy string  `xml:"changedBy,attr,omitempty" json:"changedBy,omitempty"`
	Entry     []Child `xml:"entry,omitempty" json:"entry,omitempty"`
}

// ChatMessages is the container for chat message entries.
type ChatMessages struct {
	ChatMessage []ChatMessage `xml:"chatMessage,omitempty" json:"chatMessage,omitempty"`
}

// ChatMessage represents one chat message.
type ChatMessage struct {
	Username string `xml:"username,attr" json:"username"`
	Time     int64  `xml:"time,attr" json:"time"`
	Message  string `xml:"message,attr" json:"message"`
}

// PodcastsResp is the container for podcast channel entries.
type PodcastsResp struct {
	Channel []PodcastChannelResp `xml:"channel,omitempty" json:"channel,omitempty"`
}

// PodcastChannelResp represents one podcast subscription.
type PodcastChannelResp struct {
	ID             string               `xml:"id,attr" json:"id"`
	URL            string               `xml:"url,attr" json:"url"`
	Title          string               `xml:"title,attr,omitempty" json:"title,omitempty"`
	Description    string               `xml:"description,attr,omitempty" json:"description,omitempty"`
	CoverArt       string               `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	OriginalImageURL string             `xml:"originalImageUrl,attr,omitempty" json:"originalImageUrl,omitempty"`
	Status         string               `xml:"status,attr" json:"status"`
	ErrorMessage   string               `xml:"errorMessage,attr,omitempty" json:"errorMessage,omitempty"`
	Episode        []PodcastEpisodeResp `xml:"episode,omitempty" json:"episode,omitempty"`
}

// PodcastEpisodeResp represents one podcast episode.
type PodcastEpisodeResp struct {
	ID          string `xml:"id,attr" json:"id"`
	ChannelID   string `xml:"channelId,attr" json:"channelId"`
	StreamID    string `xml:"streamId,attr,omitempty" json:"streamId,omitempty"`
	Title       string `xml:"title,attr,omitempty" json:"title,omitempty"`
	Description string `xml:"description,attr,omitempty" json:"description,omitempty"`
	Status      string `xml:"status,attr" json:"status"`
	PublishDate string `xml:"publishDate,attr,omitempty" json:"publishDate,omitempty"`
	Year        int    `xml:"year,attr,omitempty" json:"year,omitempty"`
	Genre       string `xml:"genre,attr,omitempty" json:"genre,omitempty"`
	CoverArt    string `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Size        int64  `xml:"size,attr,omitempty" json:"size,omitempty"`
	ContentType string `xml:"contentType,attr,omitempty" json:"contentType,omitempty"`
	Suffix      string `xml:"suffix,attr,omitempty" json:"suffix,omitempty"`
	Duration    int    `xml:"duration,attr,omitempty" json:"duration,omitempty"`
	BitRate     int    `xml:"bitRate,attr,omitempty" json:"bitRate,omitempty"`
	IsDir       bool   `xml:"isDir,attr" json:"isDir"`
	Parent      string `xml:"parent,attr,omitempty" json:"parent,omitempty"`
}

// NewestPodcasts is the container for the most recent podcast episodes.
type NewestPodcasts struct {
	Episode []PodcastEpisodeResp `xml:"episode,omitempty" json:"episode,omitempty"`
}

// InternetRadioStations is the container for internet radio station entries.
type InternetRadioStations struct {
	InternetRadioStation []InternetRadioStation `xml:"internetRadioStation,omitempty" json:"internetRadioStation,omitempty"`
}

// InternetRadioStation represents one internet radio stream.
type InternetRadioStation struct {
	ID          string `xml:"id,attr" json:"id"`
	Name        string `xml:"name,attr" json:"name"`
	StreamURL   string `xml:"streamUrl,attr" json:"streamUrl"`
	HomepageURL string `xml:"homepageUrl,attr,omitempty" json:"homepageUrl,omitempty"`
}

// ArtistInfo holds supplemental artist information from the music library metadata.
type ArtistInfo struct {
	Biography      string      `xml:"biography,omitempty" json:"biography,omitempty"`
	MusicBrainzID  string      `xml:"musicBrainzId,omitempty" json:"musicBrainzId,omitempty"`
	SmallImageURL  string      `xml:"smallImageUrl,omitempty" json:"smallImageUrl,omitempty"`
	MediumImageURL string      `xml:"mediumImageUrl,omitempty" json:"mediumImageUrl,omitempty"`
	LargeImageURL  string      `xml:"largeImageUrl,omitempty" json:"largeImageUrl,omitempty"`
	SimilarArtist  []ArtistID3 `xml:"similarArtist,omitempty" json:"similarArtist,omitempty"`
}

// ArtistInfo2 is the ID3-based variant of ArtistInfo.
type ArtistInfo2 struct {
	ArtistInfo
}

// AlbumInfo holds supplemental album information.
type AlbumInfo struct {
	Notes          string `xml:"notes,omitempty" json:"notes,omitempty"`
	MusicBrainzID  string `xml:"musicBrainzId,omitempty" json:"musicBrainzId,omitempty"`
	LastFmURL      string `xml:"lastFmUrl,omitempty" json:"lastFmUrl,omitempty"`
	SmallImageURL  string `xml:"smallImageUrl,omitempty" json:"smallImageUrl,omitempty"`
	MediumImageURL string `xml:"mediumImageUrl,omitempty" json:"mediumImageUrl,omitempty"`
	LargeImageURL  string `xml:"largeImageUrl,omitempty" json:"largeImageUrl,omitempty"`
}

// SimilarSongs groups songs with a similar genre for the legacy folder-based endpoint.
type SimilarSongs struct {
	Song []Child `xml:"song,omitempty" json:"song,omitempty"`
}

// SimilarSongs2 groups songs with a similar genre for the ID3-based endpoint.
type SimilarSongs2 struct {
	Song []Child `xml:"song,omitempty" json:"song,omitempty"`
}

// TopSongs groups the most-played songs for a given artist.
type TopSongs struct {
	Song []Child `xml:"song,omitempty" json:"song,omitempty"`
}

// Lyrics holds song lyric text returned by getLyrics.
type Lyrics struct {
	Artist string `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	Title  string `xml:"title,attr,omitempty" json:"title,omitempty"`
	Value  string `xml:",chardata" json:"value,omitempty"`
}

// Videos is the container for video entries; always empty for audio-only servers.
type Videos struct {
	Video []Child `xml:"video,omitempty" json:"video,omitempty"`
}

// OpenSubsonicExtensions lists the OpenSubsonic extensions this server supports.
type OpenSubsonicExtensions struct {
	Extension []OpenSubsonicExtension `xml:"extension,omitempty" json:"extension,omitempty"`
}

// OpenSubsonicExtension is a single named extension with its supported version.
type OpenSubsonicExtension struct {
	Name     string `xml:"name,attr" json:"name"`
	Versions []int  `xml:"versions,omitempty" json:"versions"`
}

// jsonWrapper is the outer key used in all JSON Subsonic responses.
type jsonWrapper struct {
	SubsonicResponse *SubsonicResponse `json:"subsonic-response"`
}

// ok constructs a successful SubsonicResponse and applies the given payload function.
func ok(payload func(*SubsonicResponse)) *SubsonicResponse {
	r := &SubsonicResponse{
		XMLNS:   XMLNamespace,
		Status:  "ok",
		Version: SubsonicVersion,
	}
	if payload != nil {
		payload(r)
	}
	return r
}

// errResp constructs an error SubsonicResponse with the given Subsonic error code.
func errResp(code int, msg string) *SubsonicResponse {
	return &SubsonicResponse{
		XMLNS:   XMLNamespace,
		Status:  "failed",
		Version: SubsonicVersion,
		Error:   &SubsonicError{Code: code, Message: msg},
	}
}

// respond serializes resp as XML or JSON based on the ?f= query parameter.
// All Subsonic responses use HTTP 200 regardless of error status.
func respond(w http.ResponseWriter, r *http.Request, resp *SubsonicResponse) {
	format := r.URL.Query().Get("f")
	callback := r.URL.Query().Get("callback")

	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(jsonWrapper{SubsonicResponse: resp})

	case "jsonp":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(jsonWrapper{SubsonicResponse: resp})
		if callback == "" {
			callback = "callback"
		}
		_, _ = fmt.Fprintf(w, "%s(%s)", callback, data)

	default:
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, xml.Header)
		_ = xml.NewEncoder(w).Encode(resp)
	}
}
