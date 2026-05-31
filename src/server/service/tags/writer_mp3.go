package tags

import (
	"fmt"
	"os"

	"github.com/bogem/id3v2/v2"
)

// writeMP3 writes the provided fields to an MP3 file using ID3v2.4 tags.
// The write is atomic: bogem/id3v2 Save() writes to a temp file and renames it.
func writeMP3(path string, fields WritableFields) error {
	t, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("tags: open mp3 %s: %w", path, err)
	}
	defer t.Close()

	// ID3v2.4 uses UTF-8; set it as the default encoding for all frames.
	t.SetDefaultEncoding(id3v2.EncodingUTF8)
	t.SetVersion(4)

	enc := id3v2.EncodingUTF8

	for field, raw := range fields {
		val := fieldString(raw)
		switch field {
		case "Title":
			t.SetTitle(val)
		case "Artist":
			t.SetArtist(val)
		case "Album":
			t.SetAlbum(val)
		case "AlbumArtist":
			t.AddTextFrame("TPE2", enc, val)
		case "Year":
			t.SetYear(val)
		case "Genre":
			t.SetGenre(val)
		case "TrackNumber":
			t.AddTextFrame("TRCK", enc, val)
		case "DiscNumber":
			t.AddTextFrame("TPOS", enc, val)
		case "Composer":
			t.AddTextFrame("TCOM", enc, val)
		case "Lyricist":
			t.AddTextFrame("TEXT", enc, val)
		case "Conductor":
			t.AddTextFrame("TPE3", enc, val)
		case "BPM":
			t.AddTextFrame("TBPM", enc, val)
		case "Comment":
			t.AddCommentFrame(id3v2.CommentFrame{
				Encoding:    enc,
				Language:    "eng",
				Description: "",
				Text:        val,
			})
		case "Lyrics":
			t.AddUnsynchronisedLyricsFrame(id3v2.UnsynchronisedLyricsFrame{
				Encoding:          enc,
				Language:          "eng",
				ContentDescriptor: "",
				Lyrics:            val,
			})
		case "MBTrackID":
			t.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{
				Encoding:    enc,
				Description: "MusicBrainz Track Id",
				Value:       val,
			})
		case "MBAlbumID":
			t.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{
				Encoding:    enc,
				Description: "MusicBrainz Album Id",
				Value:       val,
			})
		case "MBArtistID":
			t.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{
				Encoding:    enc,
				Description: "MusicBrainz Artist Id",
				Value:       val,
			})
		case "MBAlbumArtistID":
			t.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{
				Encoding:    enc,
				Description: "MusicBrainz Album Artist Id",
				Value:       val,
			})
		}
	}

	// Save() writes to a temp file named path+"-id3v2" and renames it atomically.
	if err := t.Save(); err != nil {
		return fmt.Errorf("tags: save mp3 %s: %w", path, err)
	}

	// Remove any leftover temp file on unexpected failure paths.
	_ = os.Remove(path + "-id3v2")

	return nil
}
