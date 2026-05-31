package model

import "time"

// Library represents a music library root directory
type Library struct {
	ID         int64     `db:"id"`
	Name       string    `db:"name"`
	Path       string    `db:"path"`
	Enabled    bool      `db:"enabled"`
	LastScanAt time.Time `db:"last_scan_at"`
	SongCount  int       `db:"song_count"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// Artist represents a music artist
type Artist struct {
	ID        int64  `db:"id"`
	Name      string `db:"name"`
	SortName  string `db:"sort_name"`
	AlbumCount int   `db:"album_count"`
	SongCount  int   `db:"song_count"`
	CoverArtID int64 `db:"cover_art_id"`
	Biography  string `db:"biography"`
	// MusicBrainzID is the MusicBrainz artist MBID
	MusicBrainzID string `db:"musicbrainz_id"`
	// UserEdited is true when the user has manually overridden at least one metadata field
	UserEdited bool      `db:"user_edited"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// Album represents a music album
type Album struct {
	ID       int64  `db:"id"`
	Title    string `db:"title"`
	SortTitle string `db:"sort_title"`
	ArtistID int64  `db:"artist_id"`
	// ArtistName is denormalized from the artist row to avoid joins on hot paths
	ArtistName string `db:"artist_name"`
	Year       int    `db:"year"`
	Genre      string `db:"genre"`
	SongCount  int    `db:"song_count"`
	// Duration is the total album length in seconds
	Duration   int   `db:"duration"`
	CoverArtID int64 `db:"cover_art_id"`
	// MusicBrainzID is the MusicBrainz release MBID
	MusicBrainzID string `db:"musicbrainz_id"`
	// UserEdited semantics: true + non-empty field → never overwrite from scan;
	// true + empty field → repopulate from scan
	UserEdited bool      `db:"user_edited"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// Song represents a single audio track
type Song struct {
	ID        int64  `db:"id"`
	LibraryID int64  `db:"library_id"`
	Path      string `db:"path"`
	Title     string `db:"title"`
	SortTitle string `db:"sort_title"`
	ArtistID  int64  `db:"artist_id"`
	ArtistName string `db:"artist_name"`
	AlbumArtistID   int64  `db:"album_artist_id"`
	AlbumArtistName string `db:"album_artist_name"`
	AlbumID    int64  `db:"album_id"`
	AlbumName  string `db:"album_name"`
	TrackNumber int   `db:"track_number"`
	DiscNumber  int   `db:"disc_number"`
	Year        int   `db:"year"`
	Genre       string `db:"genre"`
	// Duration is the track length in seconds
	Duration int `db:"duration"`
	// BitRate is the encoding bit rate in kbps
	BitRate int `db:"bit_rate"`
	// SampleRate is the audio sample rate in Hz
	SampleRate int   `db:"sample_rate"`
	Channels   int   `db:"channels"`
	FileSize   int64 `db:"file_size"`
	ContentType string `db:"content_type"`
	// FileFormat is the container/codec name, e.g. "mp3", "flac", "ogg"
	FileFormat string `db:"file_format"`
	CoverArtID int64  `db:"cover_art_id"`
	// MBTrackID is the MusicBrainz recording MBID
	MBTrackID string `db:"mb_track_id"`
	// MBAlbumID is the MusicBrainz release MBID
	MBAlbumID string `db:"mb_album_id"`
	// MBAlbumArtistID is the MusicBrainz release artist MBID
	MBAlbumArtistID string `db:"mb_album_artist_id"`
	// MBArtistID is the MusicBrainz artist MBID
	MBArtistID  string `db:"mb_artist_id"`
	Composer    string `db:"composer"`
	Lyricist    string `db:"lyricist"`
	Conductor   string `db:"conductor"`
	Comment     string `db:"comment"`
	Lyrics      string `db:"lyrics"`
	BPM         int    `db:"bpm"`
	// ReplayGainTrack is the track-level replay gain value in dB
	ReplayGainTrack float64 `db:"replay_gain_track"`
	// ReplayGainAlbum is the album-level replay gain value in dB
	ReplayGainAlbum float64 `db:"replay_gain_album"`
	// FileHash is the SHA-256 hash of the file content used to detect changes
	FileHash     string    `db:"file_hash"`
	LastModified time.Time `db:"last_modified"`
	// UserEdited semantics: true + non-empty field → never overwrite from scan;
	// true + empty field → repopulate from scan
	UserEdited bool      `db:"user_edited"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// Genre represents a music genre aggregated across the library
type Genre struct {
	ID         int64  `db:"id"`
	Name       string `db:"name"`
	SongCount  int    `db:"song_count"`
	AlbumCount int    `db:"album_count"`
}

// CoverArt represents embedded or externally stored cover art
type CoverArt struct {
	ID int64 `db:"id"`
	// SongID is 0 when the cover art is album-level only
	SongID int64 `db:"song_id"`
	// AlbumID is 0 when the cover art is song-level only
	AlbumID int64 `db:"album_id"`
	// Data holds the raw image bytes when stored in the database
	Data []byte `db:"data"`
	// Path holds the filesystem path when the image is stored on disk
	Path      string    `db:"path"`
	MimeType  string    `db:"mime_type"`
	Width     int       `db:"width"`
	Height    int       `db:"height"`
	CreatedAt time.Time `db:"created_at"`
}
