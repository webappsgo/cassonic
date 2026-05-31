package service

// SongMeta holds parsed tag data from an audio file.
// It is the data transfer object between the tags sub-package and the scanner.
type SongMeta struct {
	Title       string
	Artist      string
	AlbumArtist string
	Album       string
	TrackNumber int
	DiscNumber  int
	Year        int
	Genre       string
	Composer    string
	Lyricist    string
	Conductor   string
	Comment     string
	Lyrics      string
	BPM         int
	// ReplayGainTrack is the track-level replay gain value in dB
	ReplayGainTrack float64
	// ReplayGainAlbum is the album-level replay gain value in dB
	ReplayGainAlbum float64

	// MusicBrainz IDs
	MBTrackID       string
	MBAlbumID       string
	MBArtistID      string
	MBAlbumArtistID string

	// Duration is the track length in seconds
	Duration int
	// BitRate is the encoding bit rate in kbps
	BitRate int
	// SampleRate is the audio sample rate in Hz
	SampleRate  int
	Channels    int
	Format      string
	ContentType string
	FileSize    int64

	// CoverData holds the raw embedded cover art bytes
	CoverData []byte
	// CoverMime is the MIME type of the embedded cover art
	CoverMime string
}

// TagReader is the interface for reading audio tags from a file.
// It is implemented by the tags sub-package (src/server/service/tags).
type TagReader interface {
	// Read parses tags from the audio file at path and returns a SongMeta.
	Read(path string) (*SongMeta, error)
}
