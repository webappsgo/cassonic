package subsonic

// Tests for ID encode/decode helpers in ids.go: encodeSongID, encodeAlbumID,
// encodeArtistID, encodePodcastID, encodeRadioID, decodeID and the typed
// decode* wrappers.

import "testing"

// TestEncodeIDs verifies each encode* helper produces the correctly prefixed string.
func TestEncodeIDs(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"song", encodeSongID(42), "so-42"},
		{"album", encodeAlbumID(7), "al-7"},
		{"artist", encodeArtistID(1), "ar-1"},
		{"podcast", encodePodcastID(9), "pc-9"},
		{"radio", encodeRadioID(3), "ra-3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

// TestDecodeIDPrefixes verifies decodeID recognizes every known prefix and
// returns the correct numeric ID.
func TestDecodeIDPrefixes(t *testing.T) {
	tests := []struct {
		id         string
		wantPrefix string
		wantID     int64
	}{
		{"so-42", prefixSong, 42},
		{"al-7", prefixAlbum, 7},
		{"ar-1", prefixArtist, 1},
		{"li-5", prefixLibrary, 5},
		{"pc-9", prefixPodcast, 9},
		{"ep-11", prefixEpisode, 11},
		{"sh-3", prefixShare, 3},
		{"ra-2", prefixRadio, 2},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			prefix, dbID, err := decodeID(tt.id)
			if err != nil {
				t.Fatalf("decodeID(%q) error: %v", tt.id, err)
			}
			if prefix != tt.wantPrefix {
				t.Errorf("prefix = %q, want %q", prefix, tt.wantPrefix)
			}
			if dbID != tt.wantID {
				t.Errorf("dbID = %d, want %d", dbID, tt.wantID)
			}
		})
	}
}

// TestDecodeIDLegacyBareInteger verifies decodeID falls back to parsing a
// bare integer for legacy clients that send unprefixed numeric IDs.
func TestDecodeIDLegacyBareInteger(t *testing.T) {
	prefix, dbID, err := decodeID("123")
	if err != nil {
		t.Fatalf("decodeID(\"123\") error: %v", err)
	}
	if prefix != "" {
		t.Errorf("prefix = %q, want empty", prefix)
	}
	if dbID != 123 {
		t.Errorf("dbID = %d, want 123", dbID)
	}
}

// TestDecodeIDMalformed verifies decodeID returns an error for unparsable input.
func TestDecodeIDMalformed(t *testing.T) {
	cases := []string{"so-abc", "not-a-number", "", "al-"}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			if _, _, err := decodeID(c); err == nil {
				t.Errorf("decodeID(%q) expected error, got nil", c)
			}
		})
	}
}

// TestDecodeTypedWrappers verifies each typed decode* helper round-trips
// through the corresponding encode* helper.
func TestDecodeTypedWrappers(t *testing.T) {
	if got, err := decodeSongID(encodeSongID(11)); err != nil || got != 11 {
		t.Errorf("decodeSongID: got %d, err %v, want 11, nil", got, err)
	}
	if got, err := decodeAlbumID(encodeAlbumID(22)); err != nil || got != 22 {
		t.Errorf("decodeAlbumID: got %d, err %v, want 22, nil", got, err)
	}
	if got, err := decodeArtistID(encodeArtistID(33)); err != nil || got != 33 {
		t.Errorf("decodeArtistID: got %d, err %v, want 33, nil", got, err)
	}
	if got, err := decodePodcastID(encodePodcastID(44)); err != nil || got != 44 {
		t.Errorf("decodePodcastID: got %d, err %v, want 44, nil", got, err)
	}
	if got, err := decodeRadioID(encodeRadioID(55)); err != nil || got != 55 {
		t.Errorf("decodeRadioID: got %d, err %v, want 55, nil", got, err)
	}
}

// TestDecodeTypedWrappersError verifies typed decode* helpers propagate errors
// from malformed input.
func TestDecodeTypedWrappersError(t *testing.T) {
	if _, err := decodeSongID("bad"); err == nil {
		t.Error("decodeSongID(\"bad\") expected error")
	}
	if _, err := decodeAlbumID("bad"); err == nil {
		t.Error("decodeAlbumID(\"bad\") expected error")
	}
	if _, err := decodeArtistID("bad"); err == nil {
		t.Error("decodeArtistID(\"bad\") expected error")
	}
	if _, err := decodePodcastID("bad"); err == nil {
		t.Error("decodePodcastID(\"bad\") expected error")
	}
	if _, err := decodeRadioID("bad"); err == nil {
		t.Error("decodeRadioID(\"bad\") expected error")
	}
}
