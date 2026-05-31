package scrobble

import (
	"testing"
)

func TestLastfmSign(t *testing.T) {
	tests := []struct {
		name      string
		params    map[string]string
		apiSecret string
		wantMD5   string
	}{
		{
			name: "basic known signature",
			params: map[string]string{
				"method":  "track.scrobble",
				"api_key": "abc123",
				"artist":  "Cher",
				"track":   "Believe",
			},
			apiSecret: "secret",
			wantMD5:   "60d2ec4ccbd60352fa237baf9fba97ac",
		},
		{
			name: "format and callback excluded",
			params: map[string]string{
				"method":   "track.scrobble",
				"api_key":  "abc123",
				"artist":   "Cher",
				"track":    "Believe",
				"format":   "json",
				"callback": "cb",
			},
			apiSecret: "secret",
			wantMD5:   "60d2ec4ccbd60352fa237baf9fba97ac",
		},
		{
			name: "single param",
			params: map[string]string{
				"method": "auth.getSession",
			},
			apiSecret: "mysecret",
			wantMD5:   "cb20b36c4c22481795f0c92fba0be150",
		},
		{
			name:      "empty params",
			params:    map[string]string{},
			apiSecret: "secret",
			wantMD5:   "5ebe2294ecd0e0f08eab7690d2a6ee69",
		},
		{
			name: "only excluded params",
			params: map[string]string{
				"format":   "json",
				"callback": "cb",
			},
			apiSecret: "secret",
			wantMD5:   "5ebe2294ecd0e0f08eab7690d2a6ee69",
		},
		{
			name: "alphabetical ordering enforced",
			params: map[string]string{
				"z_param": "last",
				"a_param": "first",
				"m_param": "middle",
			},
			apiSecret: "s",
			wantMD5:   "7a604c76418788935e50504f14c89a3e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastfmSign(tt.params, tt.apiSecret)
			if got != tt.wantMD5 {
				t.Errorf("lastfmSign(%v, %q): got %q, want %q", tt.params, tt.apiSecret, got, tt.wantMD5)
			}
		})
	}
}

func TestLastfmSignExcludesFormatAndCallback(t *testing.T) {
	withExcluded := map[string]string{
		"method":   "track.scrobble",
		"api_key":  "key123",
		"format":   "json",
		"callback": "myfunc",
	}
	withoutExcluded := map[string]string{
		"method":  "track.scrobble",
		"api_key": "key123",
	}

	got := lastfmSign(withExcluded, "secret")
	want := lastfmSign(withoutExcluded, "secret")

	if got != want {
		t.Errorf("format/callback should be excluded from signature: got %q, want %q", got, want)
	}
}

func TestLastfmSignDeterministic(t *testing.T) {
	params := map[string]string{
		"method":  "track.love",
		"api_key": "key",
		"artist":  "Artist",
		"track":   "Track",
	}
	first := lastfmSign(params, "s")
	second := lastfmSign(params, "s")
	if first != second {
		t.Errorf("lastfmSign not deterministic: got %q then %q", first, second)
	}
}

func TestLastfmSignLowercaseHex(t *testing.T) {
	params := map[string]string{"method": "test"}
	got := lastfmSign(params, "secret")

	for _, ch := range got {
		if ch >= 'A' && ch <= 'F' {
			t.Errorf("signature contains uppercase hex characters: %q", got)
		}
	}
	if len(got) != 32 {
		t.Errorf("MD5 hex should be 32 characters, got %d: %q", len(got), got)
	}
}
