package tags

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WritableFields specifies which fields to write to an audio file.
// Only fields present in the map are written; the key is the SongMeta field name.
type WritableFields map[string]any

// Write writes the specified fields to an audio file.
// Returns ErrNotWritable if the file cannot be opened for writing.
// All writes are atomic: the new content is written to a temp file, then os.Rename replaces the original.
func Write(path string, fields WritableFields) error {
	// Check write permission before dispatching to format-specific writers.
	perm, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return ErrNotWritable
	}
	perm.Close()

	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".mp3":
		return writeMP3(path, fields)
	case ".flac":
		return writeFLAC(path, fields)
	case ".ogg", ".opus":
		return writeOGG(path, fields)
	case ".m4a", ".aac":
		return writeM4A(path, fields)
	default:
		return fmt.Errorf("tags: unsupported format for writing: %s", ext)
	}
}
