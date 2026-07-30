package tags

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteNonExistentFileReturnsErrNotWritable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.mp3")

	err := Write(path, WritableFields{"Title": "x"})
	if !errors.Is(err, ErrNotWritable) {
		t.Errorf("Write nonexistent file: got %v, want ErrNotWritable", err)
	}
}

func TestWriteUnwritablePathReturnsErrNotWritable(t *testing.T) {
	dir := t.TempDir()
	// A path inside a non-existent parent directory can never be opened for writing.
	path := filepath.Join(dir, "missing-parent", "song.mp3")

	err := Write(path, WritableFields{"Title": "x"})
	if !errors.Is(err, ErrNotWritable) {
		t.Errorf("Write path with missing parent: got %v, want ErrNotWritable", err)
	}
}

func TestWriteUnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.xyz")
	if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	err := Write(path, WritableFields{"Title": "x"})
	if err == nil {
		t.Fatal("Write unsupported extension: expected error, got nil")
	}
	if errors.Is(err, ErrNotWritable) {
		t.Error("Write unsupported extension: got ErrNotWritable, want format error")
	}
}

func TestWriteDispatchesOnLowercasedExtension(t *testing.T) {
	// An uppercase .MP3 extension must still dispatch to writeMP3, not fall
	// through to the "unsupported format" branch.
	dir := t.TempDir()
	path := filepath.Join(dir, "song.MP3")
	if err := os.WriteFile(path, []byte{}, 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	err := Write(path, WritableFields{"Title": "Upper"})
	if err != nil {
		t.Errorf("Write uppercase extension: unexpected error: %v", err)
	}
}
