package tags

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewReturnsNonNil(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("New() returned nil")
	}
}

func TestReadNonExistentFile(t *testing.T) {
	r := New()
	_, err := r.Read("/tmp/local/cassonic-nonexistent-file-that-does-not-exist.mp3")
	if err == nil {
		t.Fatal("Read non-existent file: expected error, got nil")
	}
}

func TestReadZeroByteFileDoesNotPanic(t *testing.T) {
	dir := t.TempDir()

	extensions := []string{".mp3", ".flac", ".ogg", ".opus", ".m4a", ".aac", ".wav", ".aiff", ".aif"}

	for _, ext := range extensions {
		ext := ext
		t.Run("zero_byte"+ext, func(t *testing.T) {
			path := filepath.Join(dir, "empty"+ext)

			f, err := os.Create(path)
			if err != nil {
				t.Fatalf("create temp file: %v", err)
			}
			f.Close()

			r := New()
			_, _ = r.Read(path)
		})
	}
}

func TestReadUnsupportedExtension(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "file.xyz")
	if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	r := New()
	_, err := r.Read(path)
	if err == nil {
		t.Fatal("Read unsupported extension: expected error, got nil")
	}
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("Read unsupported extension: got %v, want ErrUnsupportedFormat", err)
	}
}

func TestSupportedExtensions(t *testing.T) {
	supported := []string{".mp3", ".flac", ".ogg", ".opus", ".m4a", ".aac", ".wav", ".aiff", ".aif"}
	unsupported := []string{".xyz", ".txt", ".pdf", ".mid", ".wma", ""}

	dir := t.TempDir()
	r := New()

	for _, ext := range supported {
		path := filepath.Join(dir, "test"+ext)
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("create %s: %v", ext, err)
		}
		f.Close()

		_, err = r.Read(path)
		if errors.Is(err, ErrUnsupportedFormat) {
			t.Errorf("extension %q should be supported, got ErrUnsupportedFormat", ext)
		}
	}

	for _, ext := range unsupported {
		if ext == "" {
			continue
		}
		path := filepath.Join(dir, "test"+ext)
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("create %s: %v", ext, err)
		}
		f.Close()

		_, err = r.Read(path)
		if !errors.Is(err, ErrUnsupportedFormat) {
			t.Errorf("extension %q should be unsupported, got: %v", ext, err)
		}
	}
}

func TestParseBPM(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"120", 120},
		{"  90  ", 90},
		{"0", 0},
		{"", 0},
		{"abc", 0},
		{"200", 200},
	}
	for _, tt := range tests {
		got := parseBPM(tt.input)
		if got != tt.want {
			t.Errorf("parseBPM(%q): got %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseReplayGain(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"-6.54 dB", -6.54},
		{"-6.54 DB", -6.54},
		{"+1.23 dB", 1.23},
		{"0 dB", 0},
		{"-6.54db", -6.54},
		{"", 0},
		{"invalid", 0},
	}
	for _, tt := range tests {
		got := parseReplayGain(tt.input)
		if got != tt.want {
			t.Errorf("parseReplayGain(%q): got %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"hello", false},
		{"", false},
		{"unicode: 日本語", false},
	}
	for _, tt := range tests {
		got := sanitize(tt.input)
		if got == "" && tt.input != "" {
			t.Errorf("sanitize(%q): got empty string", tt.input)
		}
		_ = got
	}

	invalid := string([]byte{0xff, 0xfe})
	got := sanitize(invalid)
	if got == "" {
		t.Error("sanitize invalid UTF-8: got empty string, want replacement")
	}
}
