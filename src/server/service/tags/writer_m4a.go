package tags

import (
	"fmt"
	"path/filepath"
)

// writeM4A writes tags to an M4A/AAC file.
// M4A atom writing requires complex atom tree manipulation. This implementation
// returns an error until a suitable pure-Go M4A writer is integrated.
func writeM4A(path string, fields WritableFields) error {
	return fmt.Errorf("m4a tag writing is not supported for %s: use mp3 or flac format for full tag editing support", filepath.Base(path))
}
