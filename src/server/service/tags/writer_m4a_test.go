package tags

// Tests cover:
//   - writeM4A: always returns a descriptive "not supported" error regardless
//     of input, since M4A atom-tree writing is not implemented.
//
// The happy-write path is intentionally NOT covered because there is no
// happy path: writeM4A has no working implementation to exercise, and any
// future replacement (real atom writing) would need a valid MP4 container
// fixture at that time.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteM4AAlwaysErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "song.m4a")

	err := writeM4A(path, WritableFields{"Title": "x"})
	if err == nil {
		t.Fatal("writeM4A: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "song.m4a") {
		t.Errorf("writeM4A error should mention the filename: %v", err)
	}
}

func TestWriteM4AViaWriteDispatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.m4a")
	if err := os.WriteFile(path, []byte{}, 0600); err != nil {
		t.Fatalf("create empty m4a: %v", err)
	}

	if err := Write(path, WritableFields{"Title": "x"}); err == nil {
		t.Fatal("Write on .m4a: expected error, got nil")
	}
}
